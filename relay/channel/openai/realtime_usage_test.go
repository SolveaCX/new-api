package openai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestMergeRealtimeUsagePreservesBillingProvenance(t *testing.T) {
	total := &dto.RealtimeUsage{
		TotalTokens:          10,
		LocalEstimatedTokens: 3,
		FinalResponseCount:   1,
	}
	usage := &dto.RealtimeUsage{
		TotalTokens:          25,
		InputTokens:          11,
		OutputTokens:         14,
		LocalEstimatedTokens: 7,
		FinalResponseCount:   2,
	}
	usage.InputTokenDetails.CachedTokens = 4
	usage.InputTokenDetails.TextTokens = 5
	usage.InputTokenDetails.AudioTokens = 2
	usage.OutputTokenDetails.TextTokens = 8
	usage.OutputTokenDetails.AudioTokens = 6

	mergeRealtimeUsage(total, usage)

	require.Equal(t, 35, total.TotalTokens)
	require.Equal(t, 11, total.InputTokens)
	require.Equal(t, 14, total.OutputTokens)
	require.Equal(t, 4, total.InputTokenDetails.CachedTokens)
	require.Equal(t, 5, total.InputTokenDetails.TextTokens)
	require.Equal(t, 2, total.InputTokenDetails.AudioTokens)
	require.Equal(t, 8, total.OutputTokenDetails.TextTokens)
	require.Equal(t, 6, total.OutputTokenDetails.AudioTokens)
	require.Equal(t, 10, total.LocalEstimatedTokens)
	require.Equal(t, 3, total.FinalResponseCount)
}

func TestOpenaiRealtimeHandlerJoinsPeerReaderBeforeFinalUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	clientHandler, clientPeer := newRealtimeWebsocketPair(t)
	targetHandler, targetPeer := newRealtimeWebsocketPair(t)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/realtime", nil)
	info := &relaycommon.RelayInfo{
		ClientWs: clientHandler,
		TargetWs: targetHandler,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "realtime-race-test",
		},
		UsePrice: true,
	}

	type result struct {
		err   error
		usage *dto.RealtimeUsage
	}
	resultCh := make(chan result, 1)
	go func() {
		apiErr, usage := OpenaiRealtimeHandler(ctx, info)
		var err error
		if apiErr != nil {
			err = apiErr
		}
		resultCh <- result{err: err, usage: usage}
	}()
	clientPayload, err := common.Marshal(dto.RealtimeEvent{
		Type: dto.RealtimeEventConversationItemCreated,
		Item: &dto.RealtimeItem{
			Type: "message",
			Content: []dto.RealtimeContent{{
				Type: "input_text",
				Text: "concurrent client estimate",
			}},
		},
	})
	require.NoError(t, err)
	deltaPayload, err := common.Marshal(dto.RealtimeEvent{
		Type:  dto.RealtimeEventResponseFunctionCallArgumentsDelta,
		Delta: "concurrent server estimate",
	})
	require.NoError(t, err)
	donePayload, err := common.Marshal(dto.RealtimeEvent{
		Type:     dto.RealtimeEventTypeResponseDone,
		Response: &dto.RealtimeResponse{},
	})
	require.NoError(t, err)

	var writers sync.WaitGroup
	closeTarget := make(chan struct{})
	writers.Add(2)
	go func() {
		defer writers.Done()
		for i := 0; i < 500; i++ {
			if err := clientPeer.WriteMessage(websocket.TextMessage, clientPayload); err != nil {
				return
			}
			time.Sleep(50 * time.Microsecond)
		}
	}()
	go func() {
		defer writers.Done()
		for i := 0; i < 50; i++ {
			if err := targetPeer.WriteMessage(websocket.TextMessage, deltaPayload); err != nil {
				return
			}
		}
		_ = targetPeer.WriteMessage(websocket.TextMessage, donePayload)
		<-closeTarget
		_ = targetPeer.Close()
	}()
	require.NoError(t, clientPeer.SetReadDeadline(time.Now().Add(3*time.Second)))
	for {
		_, message, readErr := clientPeer.ReadMessage()
		require.NoError(t, readErr)
		var event dto.RealtimeEvent
		require.NoError(t, common.Unmarshal(message, &event))
		if event.Type == dto.RealtimeEventTypeResponseDone {
			break
		}
	}
	close(closeTarget)

	select {
	case got := <-resultCh:
		require.NoError(t, got.err)
		require.NotNil(t, got.usage)
		require.Equal(t, 1, got.usage.FinalResponseCount)
		require.Positive(t, got.usage.LocalEstimatedTokens)
	case <-time.After(5 * time.Second):
		t.Fatal("realtime handler did not join both websocket readers")
	}
	_ = clientPeer.Close()
	writers.Wait()
}

func TestOpenaiRealtimeHandlerNormalCloseWithoutResponseDoneRemainsIncomplete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	clientHandler, _ := newRealtimeWebsocketPair(t)
	targetHandler, targetPeer := newRealtimeWebsocketPair(t)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/realtime", nil)
	info := &relaycommon.RelayInfo{
		ClientWs: clientHandler, TargetWs: targetHandler, UsePrice: true,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "realtime-tail-test"},
	}

	done := make(chan struct{})
	go func() {
		_, _ = OpenaiRealtimeHandler(ctx, info)
		close(done)
	}()
	require.NoError(t, targetPeer.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "complete"),
		time.Now().Add(time.Second),
	))
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("realtime handler did not finish after normal close")
	}
	require.NotNil(t, info.StreamStatus)
	require.False(t, info.StreamStatus.IsNormalEnd())
}

func TestOpenaiRealtimeHandlerResponseDoneWithoutTailMarksComplete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	clientHandler, clientPeer := newRealtimeWebsocketPair(t)
	targetHandler, targetPeer := newRealtimeWebsocketPair(t)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/realtime", nil)
	info := &relaycommon.RelayInfo{ClientWs: clientHandler, TargetWs: targetHandler, UsePrice: true}

	done := make(chan *dto.RealtimeUsage, 1)
	go func() {
		_, usage := OpenaiRealtimeHandler(ctx, info)
		done <- usage
	}()
	payload, err := common.Marshal(dto.RealtimeEvent{Type: dto.RealtimeEventTypeResponseDone, Response: &dto.RealtimeResponse{Usage: &dto.RealtimeUsage{TotalTokens: 1, InputTokens: 1}}})
	require.NoError(t, err)
	require.NoError(t, targetPeer.WriteMessage(websocket.TextMessage, payload))
	require.NoError(t, clientPeer.SetReadDeadline(time.Now().Add(3*time.Second)))
	_, _, err = clientPeer.ReadMessage()
	require.NoError(t, err)
	require.NoError(t, targetPeer.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "complete"), time.Now().Add(time.Second)))

	select {
	case usage := <-done:
		require.Equal(t, 1, usage.FinalResponseCount)
	case <-time.After(3 * time.Second):
		t.Fatal("realtime handler did not finish")
	}
	require.True(t, info.StreamStatus.IsNormalEnd())
}

func TestOpenaiRealtimeHandlerResponseDoneWithTrailingUsageRemainsIncomplete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	clientHandler, clientPeer := newRealtimeWebsocketPair(t)
	targetHandler, targetPeer := newRealtimeWebsocketPair(t)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/realtime", nil)
	info := &relaycommon.RelayInfo{
		ClientWs: clientHandler, TargetWs: targetHandler, UsePrice: true,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "realtime-tail-test"},
	}

	done := make(chan struct{})
	go func() {
		_, _ = OpenaiRealtimeHandler(ctx, info)
		close(done)
	}()
	donePayload, err := common.Marshal(dto.RealtimeEvent{Type: dto.RealtimeEventTypeResponseDone, Response: &dto.RealtimeResponse{Usage: &dto.RealtimeUsage{TotalTokens: 1, InputTokens: 1}}})
	require.NoError(t, err)
	tailPayload, err := common.Marshal(dto.RealtimeEvent{Type: dto.RealtimeEventResponseFunctionCallArgumentsDelta, Delta: "tail usage"})
	require.NoError(t, err)
	require.NoError(t, targetPeer.WriteMessage(websocket.TextMessage, donePayload))
	require.NoError(t, targetPeer.WriteMessage(websocket.TextMessage, tailPayload))
	for range 2 {
		require.NoError(t, clientPeer.SetReadDeadline(time.Now().Add(3*time.Second)))
		_, _, err = clientPeer.ReadMessage()
		require.NoError(t, err)
	}
	require.NoError(t, targetPeer.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "complete"), time.Now().Add(time.Second)))
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("realtime handler did not finish")
	}
	require.False(t, info.StreamStatus.IsNormalEnd())
}

func TestOpenaiRealtimeHandlerTargetErrorWinsClientCloseRace(t *testing.T) {
	gin.SetMode(gin.TestMode)
	clientHandler, clientPeer := newRealtimeWebsocketPair(t)
	targetHandler, targetPeer := newRealtimeWebsocketPair(t)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/realtime", nil)
	info := &relaycommon.RelayInfo{ClientWs: clientHandler, TargetWs: targetHandler}

	done := make(chan struct{})
	go func() {
		_, _ = OpenaiRealtimeHandler(ctx, info)
		close(done)
	}()
	require.NoError(t, targetPeer.Close())
	_ = clientPeer.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "client done"), time.Now().Add(time.Second))
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("realtime handler did not finish")
	}
	require.False(t, info.StreamStatus.IsNormalEnd())
}

func TestOpenaiRealtimeHandlerMarksAbruptDisconnectUnknown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	clientHandler, _ := newRealtimeWebsocketPair(t)
	targetHandler, targetPeer := newRealtimeWebsocketPair(t)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/realtime", nil)
	info := &relaycommon.RelayInfo{ClientWs: clientHandler, TargetWs: targetHandler}

	done := make(chan struct{})
	go func() {
		_, _ = OpenaiRealtimeHandler(ctx, info)
		close(done)
	}()
	require.NoError(t, targetPeer.Close())
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("realtime handler did not finish after abrupt disconnect")
	}
	require.NotNil(t, info.StreamStatus)
	require.False(t, info.StreamStatus.IsNormalEnd())
}

func newRealtimeWebsocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	accepted := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}).Upgrade(w, r, nil)
		if err == nil {
			accepted <- conn
		}
	}))
	t.Cleanup(server.Close)
	peer, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	handler := <-accepted
	t.Cleanup(func() {
		_ = handler.Close()
		_ = peer.Close()
	})
	return handler, peer
}
