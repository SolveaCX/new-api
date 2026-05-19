// Package lifecycle 提供进程优雅退出工具，便于在 main 之外单独单测。
// 当前只暴露 graceful shutdown 与 SIGTERM/SIGINT 等待两条逻辑；
// 如有其它"生命周期"相关逻辑，这里是 home。不要膨胀到无关 utility。
package lifecycle

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// AwaitShutdown 阻塞直到收到 SIGTERM/SIGINT 或 serverErr 收到错误。
//
// - 收到信号：返回 nil（main 走 graceful shutdown 路径）。
// - 收到 serverErr：返回该错误（main 让 defer 跑、不要 os.Exit）。
//
// 抽出函数的核心目的是避免 main 里直接 common.FatalLog → os.Exit，
// 否则 defer model.CloseDB 等清理逻辑会被跳过。
func AwaitShutdown(serverErr <-chan error) error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(quit)

	select {
	case sig := <-quit:
		common.SysLog(fmt.Sprintf("shutdown signal received: %s, draining...", sig))
		return nil
	case err := <-serverErr:
		return err
	}
}

// Graceful 在 timeout 内停止 HTTP server（如果非 nil）并调用 cleanup（如果非 nil）。
//
// 设计意图：把"优雅退出"逻辑独立成可测函数。
// 在生产代码（main.go）里，信号 handler 收到 SIGTERM 后调用此函数；
// cleanup 通常用来 flush batchUpdate / 关 DB 等。
//
// 不返回 error：每一步失败都打日志继续往下走，确保 cleanup 一定被执行。
//
// 已知限制：本函数 best-effort 让 server 优雅关闭，但流式响应（如 Claude SSE
// 30-60s 大输出）不会在 timeout 内主动让出。要让长流式也协作 drain，
// 需要 stream handler 监听 request ctx（本 PR 范围之外，留作 follow-up）。
func Graceful(srv *http.Server, timeout time.Duration, cleanup func()) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if srv != nil {
		// 让 keep-alive 闲置连接立刻被关掉，把 grace window 留给真正的存量请求。
		srv.SetKeepAlivesEnabled(false)
		if err := srv.Shutdown(ctx); err != nil {
			common.SysError("HTTP server shutdown error: " + err.Error())
		}
	}
	if cleanup != nil {
		cleanup()
	}
}
