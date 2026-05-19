package model

import (
	"bytes"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 在所有测试启动 batchUpdater goroutine 之前一次性确保 BatchUpdateInterval > 0。
// TestMain 没初始化它，默认为 0；time.NewTicker(0) 会 panic。
// 在包级 init 里设置（test goroutine 起来前），就不会与任何 goroutine 形成 race。
func init() {
	if common.BatchUpdateInterval <= 0 {
		common.BatchUpdateInterval = 60
	}
}

// resetBatchUpdaterStateForTest 让多个测试可以独立启停 batchUpdater goroutine，
// 不会被前一个测试的 closed channel 污染。仅测试文件可见。
//
// 顺序很关键：必须先 Stop（多次调用安全）+ Wait（让上一个 goroutine 真退出），
// 再重置 channel。否则上个测试启动的 goroutine 还活着、
// 看不到 close(batchUpdaterStop) 信号，会变成幽灵 goroutine 污染后续测试。
//
// 同时清空 batchUpdateStores 防止测试间数据 bleed。
//
// 安全性依赖：Stop → Wait → reassign 在单 goroutine 上严格顺序执行。
// WaitBatchUpdaterStopped 内部读取的是 reassign **之前**的 batchUpdaterDone 引用，
// 所以即使后面立刻重新 make 新 channel，也不会破坏对老 goroutine 的等待。
// 如果将来把 Wait 包到另一个 goroutine 里跑，这个保证就失效了，注意。
func resetBatchUpdaterStateForTest() {
	// 先确保上一个 goroutine 已经退出（如果有），用短 timeout 避免无限挂
	StopBatchUpdater()
	WaitBatchUpdaterStopped(500 * time.Millisecond)

	batchUpdaterStop = make(chan struct{})
	batchUpdaterDone = make(chan struct{})
	batchUpdaterStopOnce = sync.Once{}
	batchUpdaterRunning.Store(false)

	for i := 0; i < BatchUpdateTypeCount; i++ {
		batchUpdateLocks[i].Lock()
		batchUpdateStores[i] = make(map[int]int)
		batchUpdateLocks[i].Unlock()
	}
}

// batchUpdate 内部一旦触发 panic，必须被吞下不传播。
// 否则 flusher goroutine 会永久死亡（这就是 2026-05-18 09:04 死锁的同款机制）。
//
// 用 DB=nil 强制内部 increaseUserQuota 在 gorm 调用处 nil-pointer panic。
func TestBatchUpdate_RecoversFromInternalPanic(t *testing.T) {
	resetBatchUpdaterStateForTest()

	origDB := DB
	DB = nil
	t.Cleanup(func() { DB = origDB })

	addNewRecord(BatchUpdateTypeUserQuota, 1, 100)

	require.NotPanics(t, func() {
		batchUpdate()
	}, "batchUpdate must recover internal panic, never propagate")
}

// graceful shutdown 流程的核心：
// 1) StopBatchUpdater 通知 goroutine 退出
// 2) 退出前必须再 flush 一次，把内存里残留的扣费数据落库（否则重启就丢）
// 3) WaitBatchUpdaterStopped 在合理时间内返回（不能死锁）
func TestStopBatchUpdater_TriggersFinalFlushAndExits(t *testing.T) {
	resetBatchUpdaterStateForTest()

	// 不动 common.BatchUpdateInterval、不动 DB：TestMain 已经把 DB 设成 sqlite memory，
	// increaseUserQuota(1, 100) 会跑 UPDATE users SET quota = quota+100 WHERE id=1，
	// 影响 0 行（没有 id=1 用户），但无 error 也不 panic。
	// 不修改全局变量就不会与 cleanup 在 race detector 下被报为 race
	// （gopool worker 的 channel happens-before 关系 race detector 跨不过去）。

	InitBatchUpdater()

	addNewRecord(BatchUpdateTypeUserQuota, 1, 100)

	batchUpdateLocks[BatchUpdateTypeUserQuota].Lock()
	sizeBefore := len(batchUpdateStores[BatchUpdateTypeUserQuota])
	batchUpdateLocks[BatchUpdateTypeUserQuota].Unlock()
	require.Equal(t, 1, sizeBefore, "fixture: store should have 1 record before stop")

	StopBatchUpdater()

	exited := make(chan struct{})
	go func() {
		WaitBatchUpdaterStopped(2 * time.Second)
		close(exited)
	}()
	select {
	case <-exited:
	case <-time.After(3 * time.Second):
		t.Fatal("batch updater goroutine did not exit within 3s after StopBatchUpdater")
	}

	batchUpdateLocks[BatchUpdateTypeUserQuota].Lock()
	sizeAfter := len(batchUpdateStores[BatchUpdateTypeUserQuota])
	batchUpdateLocks[BatchUpdateTypeUserQuota].Unlock()
	require.Equal(t, 0, sizeAfter, "final flush must drain the store before goroutine exits")
}

// StopBatchUpdater 可能在不同 shutdown 路径被多次调用（信号 handler + 兜底 cleanup），
// 多次 close(channel) 会 panic — 必须用 sync.Once 保护。
func TestStopBatchUpdater_SafeToCallMultipleTimes(t *testing.T) {
	resetBatchUpdaterStateForTest()

	// 第二次调用必须不 panic。我们不需要 goroutine 真的在跑，只验证 Stop 自身幂等。
	StopBatchUpdater()
	require.NotPanics(t, func() {
		StopBatchUpdater()
		StopBatchUpdater()
	}, "StopBatchUpdater must be idempotent (sync.Once)")
}

// 防御性 helper：BatchUpdateInterval 若被运维误设为 0 或负数（如 BATCH_UPDATE_INTERVAL=0），
// time.NewTicker 会 panic 让整个 batchUpdater goroutine 死。守卫函数兜底回退到 5s。
func TestSanitizeBatchUpdateInterval_DefaultsForNonPositive(t *testing.T) {
	require.Equal(t, 5, sanitizeBatchUpdateInterval(0))
	require.Equal(t, 5, sanitizeBatchUpdateInterval(-1))
}

func TestSanitizeBatchUpdateInterval_PreservesValidValue(t *testing.T) {
	require.Equal(t, 30, sanitizeBatchUpdateInterval(30))
	require.Equal(t, 1, sanitizeBatchUpdateInterval(1))
}

// reset helper 必须清掉 batchUpdaterRunning 标志，否则测试间顺序成为 load-bearing：
// 如果前一个测试人工 Store(true) 后没还原，后面的测试 reset 还是看到 true。
func TestResetBatchUpdater_ClearsRunningFlag(t *testing.T) {
	batchUpdaterRunning.Store(true) // 模拟上一个测试遗留
	resetBatchUpdaterStateForTest()
	require.False(t, batchUpdaterRunning.Load(),
		"reset helper must clear batchUpdaterRunning to keep tests independent")
}

// FlushBatchUpdate 用于 shutdown 兜底：在 StopBatchUpdater 之后再保险跑一次同步 flush，
// 把任何残留数据落库。验证它确实把 store 清空（而不是异步派一个 goroutine 去做）。
func TestFlushBatchUpdate_DrainsStoreSynchronously(t *testing.T) {
	resetBatchUpdaterStateForTest()

	origDB := DB
	DB = nil // 让 update 内部 panic，由 batchUpdate 的 recover 吞下
	t.Cleanup(func() { DB = origDB })

	addNewRecord(BatchUpdateTypeUserQuota, 1, 100)

	require.NotPanics(t, func() {
		FlushBatchUpdate()
	})

	batchUpdateLocks[BatchUpdateTypeUserQuota].Lock()
	size := len(batchUpdateStores[BatchUpdateTypeUserQuota])
	batchUpdateLocks[BatchUpdateTypeUserQuota].Unlock()
	require.Equal(t, 0, size, "FlushBatchUpdate must synchronously drain the store")
}

// 从未启动过 batchUpdater 时（如 BATCH_UPDATE_ENABLED=false 的部署 或
// 测试启动初始状态），WaitBatchUpdaterStopped 必须立刻返回、不打 timeout 警告日志。
// 否则每次 reset 浪费 500ms + 污染 stderr（reviewer LOW-1 指出）。
func TestWaitBatchUpdaterStopped_FastReturnWhenNeverStarted(t *testing.T) {
	resetBatchUpdaterStateForTest()

	var buf bytes.Buffer
	orig := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &buf
	t.Cleanup(func() { gin.DefaultErrorWriter = orig })

	start := time.Now()
	WaitBatchUpdaterStopped(500 * time.Millisecond)
	elapsed := time.Since(start)

	require.Less(t, elapsed, 50*time.Millisecond,
		"should return immediately when no updater is running, got %s", elapsed)
	require.NotContains(t, buf.String(), "batch updater did not stop within timeout",
		"no warning should be emitted when never started")
}

// 当 goroutine 正在跑但卡住（不 close batchUpdaterDone）时，
// WaitBatchUpdaterStopped 必须遵守 timeout，不能让 shutdown 永久挂住。
func TestWaitBatchUpdaterStopped_HonorsTimeoutWhenGoroutineHangs(t *testing.T) {
	resetBatchUpdaterStateForTest()

	// 模拟 batchUpdater goroutine 已启动但卡死（典型场景：DB 半开连接 read wait）
	batchUpdaterRunning.Store(true)
	t.Cleanup(func() { batchUpdaterRunning.Store(false) })

	var elapsed atomic.Int64
	done := make(chan struct{})
	go func() {
		start := time.Now()
		WaitBatchUpdaterStopped(100 * time.Millisecond)
		elapsed.Store(time.Since(start).Milliseconds())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitBatchUpdaterStopped did not respect timeout, hung forever")
	}
	require.LessOrEqual(t, elapsed.Load(), int64(500),
		"WaitBatchUpdaterStopped should return shortly after timeout, got %dms", elapsed.Load())
}
