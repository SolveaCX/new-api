package model

import (
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

const (
	BatchUpdateTypeUserQuota = iota
	BatchUpdateTypeTokenQuota
	BatchUpdateTypeUsedQuota
	BatchUpdateTypeChannelUsedQuota
	BatchUpdateTypeRequestCount
	BatchUpdateTypeCount // if you add a new type, you need to add a new map and a new lock
)

var batchUpdateStores []map[int]int
var batchUpdateLocks []sync.Mutex

var (
	batchUpdaterStopOnce sync.Once
	batchUpdaterStop     = make(chan struct{})
	batchUpdaterDone     = make(chan struct{})
	batchUpdaterRunning  atomic.Bool
)

func init() {
	for i := 0; i < BatchUpdateTypeCount; i++ {
		batchUpdateStores = append(batchUpdateStores, make(map[int]int))
		batchUpdateLocks = append(batchUpdateLocks, sync.Mutex{})
	}
}

// sanitizeBatchUpdateInterval 把非正数 interval 兜底为 5s。
// time.NewTicker(0) 会 panic 让整个 batchUpdater goroutine 死。
func sanitizeBatchUpdateInterval(interval int) int {
	if interval <= 0 {
		return 5
	}
	return interval
}

func InitBatchUpdater() {
	// gopool.Go 是异步派发，goroutine 可能还在队列里没跑起来。
	// 同步等它进入循环（Store(true) 完成）再返回，这样后续 WaitBatchUpdaterStopped
	// 不会因为 batchUpdaterRunning 仍是 false 而错过这个 goroutine。
	interval := sanitizeBatchUpdateInterval(common.BatchUpdateInterval)
	started := make(chan struct{})
	gopool.Go(func() {
		batchUpdaterRunning.Store(true)
		close(started)
		// 退出时 defer 按 LIFO 顺序：close(done) 先跑（晚注册→先执行），
		// 然后 Store(false)（早注册→后执行）。
		// 这样 WaitBatchUpdaterStopped 走 done 路径返回时，running 仍是 true；
		// 之后再调 Wait 才会走 fast-return（已退出场景）。
		// 顺序倒过来则违反 FlushBatchUpdate 的 doc 契约（参见那里说明）。
		defer batchUpdaterRunning.Store(false)
		defer close(batchUpdaterDone)
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-batchUpdaterStop:
				// 最后一次 flush，避免内存里的扣费在退出时丢失
				batchUpdate()
				return
			case <-ticker.C:
				batchUpdate()
			}
		}
	})
	<-started
}

// StopBatchUpdater 通知 batchUpdater goroutine 退出（多次调用安全）。
// 退出前 goroutine 会触发一次最终 flush，把残留的内存数据落库。
func StopBatchUpdater() {
	batchUpdaterStopOnce.Do(func() {
		close(batchUpdaterStop)
	})
}

// FlushBatchUpdate 同步执行一次 flush。
// 用作 shutdown 兜底：StopBatchUpdater + WaitBatchUpdaterStopped 之后调一次，
// 把 final flush 与本调用之间 addNewRecord 漏网的扣费再扫一次落库。
//
// 推荐调用顺序：StopBatchUpdater → WaitBatchUpdaterStopped → FlushBatchUpdate。
// 这个顺序下 batchUpdater goroutine 已彻底退出，本函数独占 store。
// 即使调用方未按顺序（如未先 Wait），per-type Mutex 仍保证正确性，
// 只是语义上两个 flusher 并发对同一个 map 取出操作，没有性能优势。
func FlushBatchUpdate() {
	batchUpdate()
}

// WaitBatchUpdaterStopped 阻塞等待 batchUpdater goroutine 完成最终 flush 并退出。
// 超时则放弃等待，避免 shutdown 永远挂住。
//
// 如果 batchUpdater 从未被启动（BATCH_UPDATE_ENABLED=false 部署或测试初始态），
// 立即返回，不浪费 timeout、不打无意义的 timeout 警告日志。
func WaitBatchUpdaterStopped(timeout time.Duration) {
	if !batchUpdaterRunning.Load() {
		return
	}
	select {
	case <-batchUpdaterDone:
	case <-time.After(timeout):
		common.SysError("batch updater did not stop within timeout")
	}
}

func addNewRecord(type_ int, id int, value int) {
	batchUpdateLocks[type_].Lock()
	defer batchUpdateLocks[type_].Unlock()
	if _, ok := batchUpdateStores[type_][id]; !ok {
		batchUpdateStores[type_][id] = value
	} else {
		batchUpdateStores[type_][id] += value
	}
}

func batchUpdate() {
	defer func() {
		if r := recover(); r != nil {
			common.SysError(fmt.Sprintf("batch update panic recovered: %v\n%s", r, debug.Stack()))
		}
	}()
	// check if there's any data to update
	hasData := false
	for i := 0; i < BatchUpdateTypeCount; i++ {
		batchUpdateLocks[i].Lock()
		if len(batchUpdateStores[i]) > 0 {
			hasData = true
			batchUpdateLocks[i].Unlock()
			break
		}
		batchUpdateLocks[i].Unlock()
	}

	if !hasData {
		return
	}

	common.SysLog("batch update started")
	for i := 0; i < BatchUpdateTypeCount; i++ {
		batchUpdateLocks[i].Lock()
		store := batchUpdateStores[i]
		batchUpdateStores[i] = make(map[int]int)
		batchUpdateLocks[i].Unlock()
		// TODO: maybe we can combine updates with same key?
		for key, value := range store {
			switch i {
			case BatchUpdateTypeUserQuota:
				err := increaseUserQuota(key, value)
				if err != nil {
					common.SysError("failed to batch update user quota: " + err.Error())
				}
			case BatchUpdateTypeTokenQuota:
				err := increaseTokenQuota(key, value)
				if err != nil {
					common.SysError("failed to batch update token quota: " + err.Error())
				}
			case BatchUpdateTypeUsedQuota:
				updateUserUsedQuota(key, value)
			case BatchUpdateTypeRequestCount:
				updateUserRequestCount(key, value)
			case BatchUpdateTypeChannelUsedQuota:
				updateChannelUsedQuota(key, value)
			}
		}
	}
	common.SysLog("batch update finished")
}

func RecordExist(err error) (bool, error) {
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func shouldUpdateRedis(fromDB bool, err error) bool {
	return common.RedisEnabled && fromDB && err == nil
}
