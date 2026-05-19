package lifecycle

import (
	"errors"
	"net"
	"net/http"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// AwaitShutdown 在 server 启动失败时必须返回该 error，而不是直接调 os.Exit。
// 否则 main 里 defer 的 CloseDB 等清理逻辑不会执行——这正是 review M1 指出的反例：
// graceful shutdown PR 自己 bypass 了 graceful shutdown。
func TestAwaitShutdown_ReturnsServerStartError(t *testing.T) {
	serverErr := make(chan error, 1)
	sentinel := errors.New("listen tcp :3000: bind: address already in use")
	serverErr <- sentinel

	got := AwaitShutdown(serverErr)

	require.Equal(t, sentinel, got, "server start error must be returned, not swallowed")
}

// AwaitShutdown 收到 SIGTERM 时返回 nil（正常 shutdown 信号）。
func TestAwaitShutdown_ReturnsNilOnSIGTERM(t *testing.T) {
	serverErr := make(chan error, 1)

	done := make(chan error, 1)
	go func() {
		done <- AwaitShutdown(serverErr)
	}()

	// 给 goroutine 一点时间注册 signal handler
	time.Sleep(50 * time.Millisecond)
	require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))

	select {
	case err := <-done:
		require.NoError(t, err, "SIGTERM should not produce an error")
	case <-time.After(2 * time.Second):
		t.Fatal("AwaitShutdown did not return within 2s after SIGTERM")
	}
}

// Graceful 必须把 HTTP server 关掉：之后向其再发请求应该失败。
func TestGraceful_ShutsDownHTTPServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := &http.Server{Handler: http.NewServeMux()}
	serverDone := make(chan struct{})
	go func() {
		_ = srv.Serve(ln)
		close(serverDone)
	}()

	addr := "http://" + ln.Addr().String()
	resp, err := http.Get(addr)
	require.NoError(t, err, "server should be reachable before Graceful")
	resp.Body.Close()

	Graceful(srv, 2*time.Second, nil)

	_, err = http.Get(addr)
	require.Error(t, err, "server should be closed after Graceful")

	select {
	case <-serverDone:
	case <-time.After(2 * time.Second):
		t.Fatal("srv.Serve did not return after Graceful")
	}
}

// 即使没有 HTTP server（srv 为 nil），cleanup callback 也必须被执行。
// 用作 BATCH_UPDATE_ENABLED=false 等场景的兜底。
func TestGraceful_RunsCleanupWithNilServer(t *testing.T) {
	var called atomic.Bool
	Graceful(nil, 1*time.Second, func() { called.Store(true) })
	require.True(t, called.Load(), "cleanup must run even when srv is nil")
}

// 即使 server.Shutdown 失败（如 in-flight 请求超时），cleanup 仍必须被调用。
// 否则 batchUpdate 内存里残留的扣费就丢了——这是这次 bug fix 的核心保障。
func TestGraceful_RunsCleanupAfterServerShutdownTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	// handler 故意挂住，让 srv.Shutdown 在 ctx 超时前完不成
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
	})
	srv := &http.Server{Handler: handler}
	go func() { _ = srv.Serve(ln) }()

	// 起一个正在飞的请求
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String())
		if err == nil {
			resp.Body.Close()
		}
	}()
	time.Sleep(100 * time.Millisecond)

	var called atomic.Bool
	Graceful(srv, 50*time.Millisecond, func() { called.Store(true) })

	require.True(t, called.Load(),
		"cleanup must run even when server.Shutdown times out on in-flight request")
}
