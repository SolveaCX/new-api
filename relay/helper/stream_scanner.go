package helper

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/bytedance/gopkg/util/gopool"

	"github.com/gin-gonic/gin"
)

const (
	InitialScannerBufferSize    = 64 << 10 // 64KB (64*1024)
	DefaultMaxScannerBufferSize = 64 << 20 // 64MB (64*1024*1024) default SSE buffer size
	DefaultPingInterval         = 10 * time.Second
)

func getScannerBufferSize() int {
	if constant.StreamScannerMaxBufferMB > 0 {
		return constant.StreamScannerMaxBufferMB << 20
	}
	return DefaultMaxScannerBufferSize
}

func NewStreamScanner(reader io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, InitialScannerBufferSize), getScannerBufferSize())
	return scanner
}

func copyCodexSSEHeaders(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) {
	if c == nil || c.Writer == nil || resp == nil || info == nil || info.ApiType != constant.APITypeCodex {
		return
	}
	for _, name := range []string{"X-Reasoning-Included", "X-Codex-Turn-State"} {
		values := resp.Header.Values(name)
		if !service.ShouldCopyUpstreamHeader(c, name, values) {
			continue
		}
		for _, value := range values {
			if value != "" {
				c.Writer.Header().Add(name, value)
			}
		}
	}
}

// StreamDataGate runs before a data event is handed to the provider handler.
// It returns ready=false while the caller wants the event held back, ready=true
// when the held events may be replayed, and a non-nil error when the stream
// must terminate before anything is written downstream.
type StreamDataGate func(data string) (ready bool, err error)

// StreamDataGateEnd is called after the upstream stream reaches a natural
// [DONE] or EOF boundary while the gate is still closed. It lets a gated
// protocol decide whether buffered events are safe to replay at end-of-stream
// (for example, a short but otherwise valid stream without [DONE]).
type StreamDataGateEnd func() (ready bool, err error)

func StreamScannerHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, dataHandler func(data string, sr *StreamResult)) {
	_ = streamScannerHandler(c, resp, info, nil, nil, dataHandler)
}

// StreamScannerHandlerWithDataGate is the opt-in variant for protocols that
// need to validate an initial event before committing SSE headers. The normal
// StreamScannerHandler path is unchanged. Events held by the gate are replayed
// in order once it returns ready=true; timeout and ping lifecycles still start
// immediately when this function is called.
func StreamScannerHandlerWithDataGate(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, gate StreamDataGate, dataHandler func(data string, sr *StreamResult)) error {
	return streamScannerHandler(c, resp, info, gate, nil, dataHandler)
}

// StreamScannerHandlerWithDataGateAndEnd is the gated variant for protocols
// that need an explicit end-of-stream decision before replaying buffered
// events. StreamScannerHandlerWithDataGate remains available for callers that
// do not need end finalization.
func StreamScannerHandlerWithDataGateAndEnd(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, gate StreamDataGate, gateEnd StreamDataGateEnd, dataHandler func(data string, sr *StreamResult)) error {
	return streamScannerHandler(c, resp, info, gate, gateEnd, dataHandler)
}

type streamDataEvent struct {
	data string
	end  bool
}

func streamScannerHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, gate StreamDataGate, gateEnd StreamDataGateEnd, dataHandler func(data string, sr *StreamResult)) (resultErr error) {

	if resp == nil || dataHandler == nil {
		return nil
	}

	// 无条件新建 StreamStatus
	info.StreamStatus = relaycommon.NewStreamStatus()

	// Ensure the upstream body is closed exactly once. A gated preflight error
	// closes it immediately to interrupt a scanner blocked in Read; the defer
	// below remains the fallback for every other exit path.
	var bodyCloseOnce sync.Once
	closeBody := func() {
		bodyCloseOnce.Do(func() {
			if resp.Body != nil {
				_ = resp.Body.Close()
			}
		})
	}
	defer closeBody()

	streamingTimeout := time.Duration(constant.StreamingTimeout) * time.Second

	var (
		stopChan        = make(chan bool, 3) // 增加缓冲区避免阻塞
		scanner         = NewStreamScanner(resp.Body)
		ticker          = time.NewTicker(streamingTimeout)
		pingTicker      *time.Ticker
		writeMutex      sync.Mutex     // Mutex to protect concurrent writes
		wg              sync.WaitGroup // 用于等待所有 goroutine 退出
		streamStart     = make(chan struct{})
		streamStartOnce sync.Once
		gateAccepted    atomic.Bool
	)
	startStream := func() {
		SetEventStreamHeaders(c)
	}
	releaseStream := func() {
		streamStartOnce.Do(func() {
			close(streamStart)
		})
	}
	var gateErrorChan chan error
	if gate != nil {
		gateErrorChan = make(chan error, 1)
	}

	// First-response watchdog: catches "alive but silent" upstream where
	// keep-alive comments/blank lines keep the per-chunk ticker reset
	// indefinitely. Disabled (nil channel) when StreamingFirstResponseTimeout
	// is <= 0 or when info already has a first response time (rare retry case).
	var (
		frtTimer    *time.Timer
		frtTimerC   <-chan time.Time
		frtStopOnce sync.Once
	)
	if constant.StreamingFirstResponseTimeout > 0 && !info.HasSendResponse() {
		frtTimer = time.NewTimer(time.Duration(constant.StreamingFirstResponseTimeout) * time.Second)
		frtTimerC = frtTimer.C
	}
	stopFRTTimer := func() {
		if frtTimer == nil {
			return
		}
		frtStopOnce.Do(func() {
			if !frtTimer.Stop() {
				select {
				case <-frtTimer.C:
				default:
				}
			}
		})
	}
	openGate := func() {
		if gate == nil || !gateAccepted.CompareAndSwap(false, true) {
			return
		}
		// The scanner can receive several prelude events before the data handler
		// opens the gate. Treat the gate transition as the first accepted response
		// and start a fresh idle window from that point.
		info.SetFirstResponseTime()
		stopFRTTimer()
		ticker.Reset(streamingTimeout)
		startStream()
	}

	generalSettings := operation_setting.GetGeneralSetting()
	pingEnabled := generalSettings.PingIntervalEnabled && !info.DisablePing
	pingInterval := time.Duration(generalSettings.PingIntervalSeconds) * time.Second
	if pingInterval <= 0 {
		pingInterval = DefaultPingInterval
	}

	if pingEnabled {
		pingTicker = time.NewTicker(pingInterval)
	}

	logger.LogDebug(c, "relay timeout seconds: %d", common.RelayTimeout)
	logger.LogDebug(c, "relay max idle conns: %d", common.RelayMaxIdleConns)
	logger.LogDebug(c, "relay max idle conns per host: %d", common.RelayMaxIdleConnsPerHost)
	logger.LogDebug(c, "streaming timeout seconds: %d", int64(streamingTimeout.Seconds()))
	logger.LogDebug(c, "ping interval seconds: %d", int64(pingInterval.Seconds()))

	// 改进资源清理，确保所有 goroutine 正确退出
	defer func() {
		// 通知所有 goroutine 停止
		common.SafeSendBool(stopChan, true)

		ticker.Stop()
		stopFRTTimer()
		if pingTicker != nil {
			pingTicker.Stop()
		}

		// 等待所有 goroutine 退出，最多等待5秒
		done := make(chan struct{})
		gopool.Go(func() {
			wg.Wait()
			close(done)
		})

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			logger.LogError(c, "timeout waiting for goroutines to exit")
		}
		if gateErrorChan != nil {
			select {
			case resultErr = <-gateErrorChan:
			default:
			}
		}

		close(stopChan)
	}()

	scanner.Split(bufio.ScanLines)
	copyCodexSSEHeaders(c, resp, info)
	if gate == nil {
		startStream()
		releaseStream()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		if resultErr != nil {
			cancel()
			closeBody()
		}
	}()

	ctx = context.WithValue(ctx, "stop_chan", stopChan)

	// Handle ping data sending with improved error handling
	if pingEnabled && pingTicker != nil {
		wg.Add(1)
		gopool.Go(func() {
			defer func() {
				wg.Done()
				if r := recover(); r != nil {
					logger.LogError(c, fmt.Sprintf("ping goroutine panic: %v", r))
					info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonPanic, fmt.Errorf("ping panic: %v", r))
					common.SafeSendBool(stopChan, true)
				}
				logger.LogDebug(c, "ping goroutine exited")
			}()

			// A gated stream must not let a keep-alive commit the response before
			// the first upstream event has been validated. The regular path has
			// streamStart closed already, so its behavior is unchanged.
			select {
			case <-streamStart:
			case <-ctx.Done():
				return
			case <-stopChan:
				return
			case <-c.Request.Context().Done():
				return
			}

			// 添加超时保护，防止 goroutine 无限运行
			maxPingDuration := 30 * time.Minute // 最大 ping 持续时间
			pingTimeout := time.NewTimer(maxPingDuration)
			defer pingTimeout.Stop()

			for {
				select {
				case <-pingTicker.C:
					// 使用超时机制防止写操作阻塞
					done := make(chan error, 1)
					gopool.Go(func() {
						writeMutex.Lock()
						defer writeMutex.Unlock()
						done <- PingData(c)
					})

					select {
					case err := <-done:
						if err != nil {
							logger.LogError(c, "ping data error: "+err.Error())
							info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonPingFail, err)
							return
						}
						logger.LogDebug(c, "ping data sent")
					case <-time.After(10 * time.Second):
						logger.LogError(c, "ping data send timeout")
						info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonPingFail, fmt.Errorf("ping send timeout"))
						return
					case <-ctx.Done():
						return
					case <-stopChan:
						return
					}
				case <-ctx.Done():
					return
				case <-stopChan:
					return
				case <-c.Request.Context().Done():
					// 监听客户端断开连接
					return
				case <-pingTimeout.C:
					logger.LogError(c, "ping goroutine max duration reached")
					return
				}
			}
		})
	}

	dataChan := make(chan streamDataEvent, 10)

	wg.Add(1)
	gopool.Go(func() {
		defer func() {
			wg.Done()
			if r := recover(); r != nil {
				logger.LogError(c, fmt.Sprintf("data handler goroutine panic: %v", r))
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonPanic, fmt.Errorf("handler panic: %v", r))
			}
			common.SafeSendBool(stopChan, true)
		}()
		sr := newStreamResult(info.StreamStatus)
		pending := make([]string, 0, 4)
		gateOpen := gate == nil
		dispatch := func(data string) bool {
			sr.reset()
			writeMutex.Lock()
			dataHandler(data, sr)
			writeMutex.Unlock()
			return sr.IsStopped()
		}
		for event := range dataChan {
			if event.end {
				if !gateOpen && gateEnd != nil {
					ready, gateErr := gateEnd()
					if gateErr != nil {
						closeBody()
						if gateErrorChan != nil {
							gateErrorChan <- gateErr
						}
						sr.reset()
						sr.Stop(gateErr)
						return
					}
					if ready {
						gateOpen = true
						openGate()
						for _, pendingData := range pending {
							if dispatch(pendingData) {
								return
							}
						}
						pending = nil
						releaseStream()
					}
				}
				return
			}
			data := event.data
			if !gateOpen {
				ready, gateErr := gate(data)
				if gateErr != nil {
					closeBody()
					if gateErrorChan != nil {
						gateErrorChan <- gateErr
					}
					sr.reset()
					sr.Stop(gateErr)
					return
				}
				pending = append(pending, data)
				if !ready {
					continue
				}
				gateOpen = true
				openGate()
				for _, pendingData := range pending {
					if dispatch(pendingData) {
						return
					}
				}
				pending = nil
				releaseStream()
				continue
			}
			if dispatch(data) {
				return
			}
		}
	})

	// Scanner goroutine with improved error handling
	wg.Add(1)
	common.RelayCtxGo(ctx, func() {
		defer func() {
			close(dataChan)
			wg.Done()
			if r := recover(); r != nil {
				logger.LogError(c, fmt.Sprintf("scanner goroutine panic: %v", r))
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonPanic, fmt.Errorf("scanner panic: %v", r))
			}
			common.SafeSendBool(stopChan, true)
			logger.LogDebug(c, "scanner goroutine exited")
		}()

		for scanner.Scan() {
			// 检查是否需要停止
			select {
			case <-stopChan:
				return
			case <-ctx.Done():
				return
			case <-c.Request.Context().Done():
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, c.Request.Context().Err())
				return
			default:
			}

			ticker.Reset(streamingTimeout)
			data := scanner.Text()
			logger.LogDebug(c, "stream scanner data: %s", data)

			if len(data) < 6 {
				continue
			}
			if data[:5] != "data:" && data[:6] != "[DONE]" {
				continue
			}
			data = data[5:]
			data = strings.TrimSpace(data)
			if data == "" {
				continue
			}
			if !strings.HasPrefix(data, "[DONE]") {
				if gate == nil || gateAccepted.Load() {
					info.SetFirstResponseTime()
					// First accepted data event arrived; stand down the FRT watchdog.
					stopFRTTimer()
				}
				info.ReceivedResponseCount++

				select {
				case dataChan <- streamDataEvent{data: data}:
				case <-ctx.Done():
					return
				case <-stopChan:
					return
				}
			} else {
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonDone, nil)
				logger.LogDebug(c, "received [DONE], stopping scanner")
				if gate != nil && !gateAccepted.Load() {
					select {
					case dataChan <- streamDataEvent{end: true}:
					case <-ctx.Done():
					case <-stopChan:
					}
				}
				return
			}
		}

		if err := scanner.Err(); err != nil {
			if err != io.EOF {
				logger.LogError(c, "scanner error: "+err.Error())
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonScannerErr, err)
			}
		}
		info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonEOF, nil)
		if gate != nil && !gateAccepted.Load() {
			select {
			case dataChan <- streamDataEvent{end: true}:
			case <-ctx.Done():
			case <-stopChan:
			}
		}
	})

	// 主循环等待完成或超时
	select {
	case gateErr := <-gateErrorChan:
		if gateErr != nil {
			resultErr = gateErr
		}
	case <-ticker.C:
		info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonTimeout, nil)
	case <-frtTimerC:
		// FRT watchdog fired before first upstream data event. Re-check to
		// avoid the race where data arrived but the goroutine hadn't yet
		// stopped this timer; only treat as FRT timeout when truly silent.
		if !info.HasSendResponse() {
			info.StreamStatus.SetEndReason(
				relaycommon.StreamEndReasonFirstResponseTimeout,
				fmt.Errorf("upstream did not send first response token within %ds", constant.StreamingFirstResponseTimeout),
			)
		}
	case <-stopChan:
		// EndReason already set by the goroutine that triggered stopChan
	case <-c.Request.Context().Done():
		info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, c.Request.Context().Err())
	}

	if info.StreamStatus.IsNormalEnd() && !info.StreamStatus.HasErrors() {
		logger.LogInfo(c, fmt.Sprintf("stream ended: %s", info.StreamStatus.Summary()))
	} else {
		logger.LogError(c, fmt.Sprintf("stream ended: %s, received=%d", info.StreamStatus.Summary(), info.ReceivedResponseCount))
	}
	return resultErr
}
