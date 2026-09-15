package service

import (
	"context"
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

func setupWindowTestRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr := miniredis.RunT(t)
	prevRDB := common.RDB
	prevRedisEnabled := common.RedisEnabled
	common.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	common.RedisEnabled = true
	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RDB = prevRDB
		common.RedisEnabled = prevRedisEnabled
	})
	return mr
}

func TestAdjustSubscriptionWindowFromSnapshotOnceIsAtomicAndIdempotent(t *testing.T) {
	setupWindowTestRedis(t)
	now := common.GetTimestamp()
	currentBucket := now / subscriptionWindowBucketSeconds * subscriptionWindowBucketSeconds
	bucketKey := subscriptionWindowBucketKey(88, currentBucket)
	weekKey := subscriptionWindowWeekKey(88, subscriptionWindowWeekIndex(now-3600, now))
	snap := &model.TaskSubscriptionWindow{
		SubId:     88,
		SubStart:  now - 3600,
		Limit5h:   1000,
		LimitWeek: 1000,
	}

	changed, err := AdjustSubscriptionWindowFromSnapshotOnce(snap, 50, "task_window_once")
	if err != nil || !changed {
		t.Fatalf("first once adjust failed changed=%v err=%v", changed, err)
	}
	if got, _ := common.RDB.Get(context.Background(), bucketKey).Int64(); got != 50 {
		t.Fatalf("bucket after first adjust = %d, want 50", got)
	}
	if got, _ := common.RDB.Get(context.Background(), weekKey).Int64(); got != 50 {
		t.Fatalf("week after first adjust = %d, want 50", got)
	}
	changed, err = AdjustSubscriptionWindowFromSnapshotOnce(snap, 50, "task_window_once")
	if err != nil || changed {
		t.Fatalf("duplicate once adjust changed=%v err=%v, want no-op", changed, err)
	}
	if got, _ := common.RDB.Get(context.Background(), bucketKey).Int64(); got != 50 {
		t.Fatalf("bucket after duplicate adjust = %d, want 50", got)
	}

	badBucket := subscriptionWindowBucketKey(89, currentBucket)
	requireErrSnap := &model.TaskSubscriptionWindow{
		SubId:     89,
		SubStart:  now - 3600,
		Limit5h:   1000,
		LimitWeek: 0,
	}
	if err := common.RDB.Set(context.Background(), badBucket, "not-an-int", 0).Err(); err != nil {
		t.Fatalf("seed bad bucket: %v", err)
	}
	changed, err = AdjustSubscriptionWindowFromSnapshotOnce(requireErrSnap, 10, "task_window_retry")
	if err == nil || changed {
		t.Fatalf("bad redis value should fail without claim, changed=%v err=%v", changed, err)
	}
	if exists, _ := common.RDB.Exists(context.Background(), acceptedAccountingRedisStepKey("task_window_retry", model.TaskAcceptedAccountingStepSubscriptionWindow)).Result(); exists != 0 {
		t.Fatal("failed once adjust must not leave idempotency key")
	}
	if err := common.RDB.Del(context.Background(), badBucket).Err(); err != nil {
		t.Fatalf("clear bad bucket: %v", err)
	}
	changed, err = AdjustSubscriptionWindowFromSnapshotOnce(requireErrSnap, 10, "task_window_retry")
	if err != nil || !changed {
		t.Fatalf("retry after clearing bad bucket failed changed=%v err=%v", changed, err)
	}
}

func TestEntitlementWindowSnapshotSettlesOnlyNegativeIdentityKeys(t *testing.T) {
	setupWindowTestRedis(t)
	now := common.GetTimestamp()
	info := &model.SubscriptionWindowInfo{
		UserSubscriptionId: 77,
		WindowScopeVersion: model.SubscriptionWindowScopeVersionEntitlement,
		SubscriptionStart:  now - 60,
		Window5hAmount:     100,
		WindowWeekAmount:   200,
	}

	guard, err := reserveSubscriptionWindows(info, 40)
	if err != nil || guard == nil {
		t.Fatalf("reserve entitlement-scoped window: guard=%v err=%v", guard, err)
	}
	snap := guard.Snapshot()
	if snap == nil || snap.SubId != -77 {
		t.Fatalf("snapshot identity = %+v, want sub_id=-77", snap)
	}
	changed, err := AdjustSubscriptionWindowFromSnapshotOnce(snap, 10, "negative-window-identity")
	if err != nil || !changed {
		t.Fatalf("settle entitlement-scoped window: changed=%v err=%v", changed, err)
	}

	currentBucket := now / subscriptionWindowBucketSeconds * subscriptionWindowBucketSeconds
	negativeBucketKey := subscriptionWindowBucketKey(-77, currentBucket)
	negativeWeekKey := subscriptionWindowWeekKey(-77, subscriptionWindowWeekIndex(info.SubscriptionStart, now))
	if got, _ := common.RDB.Get(context.Background(), negativeBucketKey).Int64(); got != 50 {
		t.Fatalf("negative bucket usage = %d, want 50", got)
	}
	if got, _ := common.RDB.Get(context.Background(), negativeWeekKey).Int64(); got != 50 {
		t.Fatalf("negative week usage = %d, want 50", got)
	}
	positiveBucketKey := subscriptionWindowBucketKey(77, currentBucket)
	positiveWeekKey := subscriptionWindowWeekKey(77, subscriptionWindowWeekIndex(info.SubscriptionStart, now))
	if exists, _ := common.RDB.Exists(context.Background(), positiveBucketKey, positiveWeekKey).Result(); exists != 0 {
		t.Fatalf("positive legacy keys unexpectedly exist: %d", exists)
	}
}

func TestSubscriptionWindowWeekIndex(t *testing.T) {
	const week = int64(subscriptionWindowWeekSeconds)
	cases := []struct {
		name     string
		start    int64
		now      int64
		expected int64
	}{
		{"first week", 1000, 1000 + 100, 0},
		{"exact boundary", 1000, 1000 + week, 1},
		{"third cycle", 1000, 1000 + 2*week + 5, 2},
		{"zero start falls back", 0, 123456, 0},
		{"now before start", 5000, 4000, 0},
	}
	for _, c := range cases {
		if got := subscriptionWindowWeekIndex(c.start, c.now); got != c.expected {
			t.Errorf("%s: subscriptionWindowWeekIndex(%d, %d) = %d, want %d", c.name, c.start, c.now, got, c.expected)
		}
	}
}

func TestSubscriptionFundingWeightedRounding(t *testing.T) {
	f := &SubscriptionFunding{weight: 1.5}
	if got := f.weighted(10); got != 15 {
		t.Errorf("weighted(10) = %d, want 15", got)
	}
	if got := f.weighted(11); got != 17 { // ceil(16.5)
		t.Errorf("weighted(11) = %d, want 17", got)
	}
	// 正负对称：结算回补不应产生单向漂移
	if f.weighted(11)+f.weighted(-11) != 0 {
		t.Errorf("weighted must be symmetric: +11 -> %d, -11 -> %d", f.weighted(11), f.weighted(-11))
	}
	// 权重 1.0 / 未设置时原样返回
	for _, w := range []float64{0, 1} {
		f := &SubscriptionFunding{weight: w}
		if got := f.weighted(123); got != 123 {
			t.Errorf("weight=%v: weighted(123) = %d, want 123", w, got)
		}
	}
}

func TestSubscriptionFundingWindowSnapshotNilWithoutReservation(t *testing.T) {
	guard := &subscriptionWindowGuard{
		subId:      12,
		limit5h:    100,
		limitWeek:  200,
		reserved:   10,
		bucketHeld: map[string]int64{"legacy": 10},
	}
	if setupSnapshot := guard.Snapshot(); setupSnapshot == nil {
		t.Fatal("test setup must represent a legacy active window guard")
	}
	funding := &SubscriptionFunding{}
	if snapshot := funding.WindowSnapshot(); snapshot != nil {
		t.Fatalf("window snapshot without a reservation must be nil, got %+v", snapshot)
	}
}

func TestReserveSubscriptionWindows5hLimit(t *testing.T) {
	setupWindowTestRedis(t)
	info := &model.SubscriptionWindowInfo{
		UserSubscriptionId: 42,
		SubscriptionStart:  common.GetTimestamp() - 3600,
		Window5hAmount:     100,
		WindowWeekAmount:   0,
	}

	guard, err := reserveSubscriptionWindows(info, 60)
	if err != nil || guard == nil {
		t.Fatalf("first reserve should pass, guard=%v err=%v", guard, err)
	}
	if _, err := reserveSubscriptionWindows(info, 50); err == nil {
		t.Fatal("second reserve (60+50 > 100) should be rejected")
	} else {
		var winErr *subscriptionWindowExceededError
		if !errors.As(err, &winErr) || winErr.Window != "5h" {
			t.Fatalf("expected 5h window error, got %v", err)
		}
		if !errors.Is(err, ErrSubscriptionWindowExceeded) {
			t.Fatal("window error must unwrap to ErrSubscriptionWindowExceeded")
		}
	}
	// 释放后可再预留
	guard.Release()
	if g, err := reserveSubscriptionWindows(info, 50); err != nil || g == nil {
		t.Fatalf("reserve after release should pass, err=%v", err)
	}
}

func TestReserveSubscriptionWindowsWeekLimit(t *testing.T) {
	setupWindowTestRedis(t)
	info := &model.SubscriptionWindowInfo{
		UserSubscriptionId: 43,
		SubscriptionStart:  common.GetTimestamp() - 3600,
		Window5hAmount:     0,
		WindowWeekAmount:   80,
	}
	if g, err := reserveSubscriptionWindows(info, 80); err != nil || g == nil {
		t.Fatalf("exact-limit reserve should pass, err=%v", err)
	}
	_, err := reserveSubscriptionWindows(info, 1)
	var winErr *subscriptionWindowExceededError
	if !errors.As(err, &winErr) || winErr.Window != "week" {
		t.Fatalf("expected week window error, got %v", err)
	}
	if winErr.ResetAt <= common.GetTimestamp() {
		t.Fatalf("week ResetAt should be in the future, got %d", winErr.ResetAt)
	}
}

func TestGetSubscriptionWindowUsage(t *testing.T) {
	setupWindowTestRedis(t)
	info := &model.SubscriptionWindowInfo{
		UserSubscriptionId: 46,
		SubscriptionStart:  common.GetTimestamp() - 3600,
		Window5hAmount:     100,
		WindowWeekAmount:   200,
	}
	if guard, err := reserveSubscriptionWindows(info, 40); err != nil || guard == nil {
		t.Fatalf("reserve failed: guard=%v err=%v", guard, err)
	}
	usage := GetSubscriptionWindowUsage(info)
	if usage.Window5hUsed != 40 || usage.WindowWeekUsed != 40 {
		t.Fatalf("usage = 5h:%d week:%d, want 40/40", usage.Window5hUsed, usage.WindowWeekUsed)
	}
	if usage.Window5hResetAt <= common.GetTimestamp() || usage.WindowWeekResetAt <= common.GetTimestamp() {
		t.Fatalf("reset times must be in the future: %+v", usage)
	}
}

func TestReserveSubscriptionWindowsSettleAdjust(t *testing.T) {
	setupWindowTestRedis(t)
	info := &model.SubscriptionWindowInfo{
		UserSubscriptionId: 44,
		SubscriptionStart:  common.GetTimestamp() - 3600,
		Window5hAmount:     100,
		WindowWeekAmount:   1000,
	}
	guard, err := reserveSubscriptionWindows(info, 90)
	if err != nil || guard == nil {
		t.Fatalf("reserve failed: %v", err)
	}
	// 结算回补 −50 后，窗口应有 60 的余量
	guard.Adjust(-50)
	if g, err := reserveSubscriptionWindows(info, 60); err != nil || g == nil {
		t.Fatalf("reserve after negative settle should pass, err=%v", err)
	}
	// 40 + 60 = 100，已打满
	if _, err := reserveSubscriptionWindows(info, 1); err == nil {
		t.Fatal("window should be full after refill")
	}
}

func TestReserveSubscriptionWindowsDisabledOrNoRedis(t *testing.T) {
	// 窗口未配置 → 直接放行
	if g, err := reserveSubscriptionWindows(&model.SubscriptionWindowInfo{UserSubscriptionId: 1}, 10); g != nil || err != nil {
		t.Fatalf("disabled windows should be pass-through, guard=%v err=%v", g, err)
	}
	// Redis 不可用 → fail-open
	prevRDB := common.RDB
	prevEnabled := common.RedisEnabled
	common.RDB = nil
	common.RedisEnabled = false
	defer func() { common.RDB = prevRDB; common.RedisEnabled = prevEnabled }()
	info := &model.SubscriptionWindowInfo{UserSubscriptionId: 2, Window5hAmount: 10}
	if g, err := reserveSubscriptionWindows(info, 999); g != nil || err != nil {
		t.Fatalf("no-redis should fail open, guard=%v err=%v", g, err)
	}
}

func TestGuardReleaseIdempotent(t *testing.T) {
	setupWindowTestRedis(t)
	info := &model.SubscriptionWindowInfo{
		UserSubscriptionId: 45,
		SubscriptionStart:  common.GetTimestamp() - 60,
		Window5hAmount:     100,
	}
	guard, err := reserveSubscriptionWindows(info, 100)
	if err != nil || guard == nil {
		t.Fatalf("reserve failed: %v", err)
	}
	guard.Release()
	guard.Release() // 第二次应为 no-op，不能把计数减成 -100
	if g, err := reserveSubscriptionWindows(info, 100); err != nil || g == nil {
		t.Fatalf("full reserve after single release should pass, err=%v", err)
	}
	if _, err := reserveSubscriptionWindows(info, 1); err == nil {
		t.Fatal("double release must not create extra capacity")
	}
}

// --- weekly window must not schedule a cycle past the subscription's end ---

// TestSubscriptionWindowKeysClampWeekExpiryToAccessEnd pins the core fix: the
// weekly cycle boundary is min(next 7-day anchor, access end). Without the
// clamp a subscription ending mid-cycle advertises a reset that never arrives
// (renewal creates a new entitlement with its own counters).
func TestSubscriptionWindowKeysClampWeekExpiryToAccessEnd(t *testing.T) {
	const week = int64(subscriptionWindowWeekSeconds)
	start := int64(1_700_000_000)
	// Subscription ends 2 days into cycle 4 (a 30-day month over 7-day cycles).
	accessEnd := start + 4*week + 2*24*3600
	// "now" sits inside that final, truncated cycle.
	now := start + 4*week + 24*3600

	info := &model.SubscriptionWindowInfo{
		UserSubscriptionId: 77,
		SubscriptionStart:  start,
		AccessEndTime:      accessEnd,
		WindowWeekAmount:   100,
	}
	_, _, expireAt := subscriptionWindowKeys(info, now)

	// Unclamped behaviour would be start+5*week+3600, i.e. 5 days past the end.
	if unclamped := start + 5*week + 3600; expireAt == unclamped {
		t.Fatalf("week expiry ignored access end: got %d (unclamped)", expireAt)
	}
	if want := accessEnd + 3600; expireAt != want {
		t.Fatalf("week expiry = %d, want %d (access end + 1h grace)", expireAt, want)
	}
}

// TestSubscriptionWindowUsageResetAtClampedToAccessEnd covers the value users
// actually see: the advertised weekly reset must never be later than the point
// the subscription stops working.
func TestSubscriptionWindowUsageResetAtClampedToAccessEnd(t *testing.T) {
	const week = int64(subscriptionWindowWeekSeconds)
	now := common.GetTimestamp()
	start := now - 4*week - 24*3600 // one day into cycle 4
	accessEnd := now + 24*3600      // subscription ends tomorrow

	info := &model.SubscriptionWindowInfo{
		UserSubscriptionId: 78,
		SubscriptionStart:  start,
		AccessEndTime:      accessEnd,
		WindowWeekAmount:   100,
	}
	usage := GetSubscriptionWindowUsage(info)
	if usage.WindowWeekResetAt != accessEnd {
		t.Fatalf("WindowWeekResetAt = %d, want %d (clamped to access end)", usage.WindowWeekResetAt, accessEnd)
	}
	if usage.WindowWeekResetAt <= now {
		t.Fatalf("reset must stay in the future, got %d (now %d)", usage.WindowWeekResetAt, now)
	}
}

// TestSubscriptionWindowKeysUnclampedWhenEndIsFar guards against over-clamping:
// a cycle that finishes well before the subscription ends keeps its natural
// 7-day boundary.
func TestSubscriptionWindowKeysUnclampedWhenEndIsFar(t *testing.T) {
	const week = int64(subscriptionWindowWeekSeconds)
	start := int64(1_700_000_000)
	now := start + 24*3600 // cycle 0
	info := &model.SubscriptionWindowInfo{
		UserSubscriptionId: 79,
		SubscriptionStart:  start,
		AccessEndTime:      start + 30*24*3600, // ends far beyond cycle 0
		WindowWeekAmount:   100,
	}
	_, _, expireAt := subscriptionWindowKeys(info, now)
	if want := start + week + 3600; expireAt != want {
		t.Fatalf("week expiry = %d, want natural boundary %d", expireAt, want)
	}
}

// TestSubscriptionWindowKeysZeroAccessEndKeepsNaturalBoundary keeps legacy rows
// (access_end_time defaults to 0) on the pre-fix behaviour instead of clamping
// them to the epoch, which would expire every counter immediately.
func TestSubscriptionWindowKeysZeroAccessEndKeepsNaturalBoundary(t *testing.T) {
	const week = int64(subscriptionWindowWeekSeconds)
	start := int64(1_700_000_000)
	now := start + 24*3600
	info := &model.SubscriptionWindowInfo{
		UserSubscriptionId: 80,
		SubscriptionStart:  start,
		AccessEndTime:      0, // legacy row
		WindowWeekAmount:   100,
	}
	_, _, expireAt := subscriptionWindowKeys(info, now)
	if want := start + week + 3600; expireAt != want {
		t.Fatalf("legacy zero access-end must keep natural boundary: got %d, want %d", expireAt, want)
	}
	if expireAt <= now {
		t.Fatalf("legacy week expiry must stay in the future, got %d (now %d)", expireAt, now)
	}
}

// TestReserveSubscriptionWindowsWeekRejectionResetAtClamped checks the rejection
// path end-to-end: the ResetAt carried by the error (and rendered into the
// "resets in about N hours" message) must respect the subscription's end.
func TestReserveSubscriptionWindowsWeekRejectionResetAtClamped(t *testing.T) {
	const week = int64(subscriptionWindowWeekSeconds)
	setupWindowTestRedis(t)

	now := common.GetTimestamp()
	start := now - 4*week - 24*3600
	accessEnd := now + 12*3600

	info := &model.SubscriptionWindowInfo{
		UserSubscriptionId: 81,
		SubscriptionStart:  start,
		AccessEndTime:      accessEnd,
		WindowWeekAmount:   10,
	}
	if _, err := reserveSubscriptionWindows(info, 10); err != nil {
		t.Fatalf("first reservation should fit the weekly limit: %v", err)
	}
	_, err := reserveSubscriptionWindows(info, 1)
	if err == nil {
		t.Fatal("expected weekly window rejection")
	}
	var winErr *subscriptionWindowExceededError
	if !errors.As(err, &winErr) {
		t.Fatalf("expected subscriptionWindowExceededError, got %T", err)
	}
	if winErr.Window != "week" {
		t.Fatalf("expected week window rejection, got %q", winErr.Window)
	}
	if winErr.ResetAt != accessEnd {
		t.Fatalf("rejection ResetAt = %d, want %d (clamped to access end)", winErr.ResetAt, accessEnd)
	}
}

// --- the 5h window must not outlive the subscription either ---

// TestSubscriptionWindow5hResetAtClampedToAccessEnd pins the displayed 5h reset
// to the subscription end. The bucket rotation is at most 30 minutes out, so the
// overshoot is far smaller than the weekly window's, but the promise is equally
// false: once access stops, no capacity ages back in.
func TestSubscriptionWindow5hResetAtClampedToAccessEnd(t *testing.T) {
	now := common.GetTimestamp()
	// Subscription ends before the next bucket rotation.
	nextRotation := (now/subscriptionWindowBucketSeconds + 1) * subscriptionWindowBucketSeconds
	accessEnd := nextRotation - 60
	if accessEnd <= now {
		t.Skip("clock sits within a minute of the rotation; skip this tick")
	}

	info := &model.SubscriptionWindowInfo{
		UserSubscriptionId: 91,
		SubscriptionStart:  now - 3600,
		AccessEndTime:      accessEnd,
		Window5hAmount:     100,
	}
	usage := GetSubscriptionWindowUsage(info)
	if usage.Window5hResetAt != accessEnd {
		t.Fatalf("Window5hResetAt = %d, want %d (clamped to access end)", usage.Window5hResetAt, accessEnd)
	}
}

// TestSubscriptionWindow5hResetAtUnclampedWhenEndIsFar guards the common case:
// a subscription with time left keeps the natural bucket rotation.
func TestSubscriptionWindow5hResetAtUnclampedWhenEndIsFar(t *testing.T) {
	now := common.GetTimestamp()
	nextRotation := (now/subscriptionWindowBucketSeconds + 1) * subscriptionWindowBucketSeconds

	info := &model.SubscriptionWindowInfo{
		UserSubscriptionId: 92,
		SubscriptionStart:  now - 3600,
		AccessEndTime:      now + 30*24*3600,
		Window5hAmount:     100,
	}
	usage := GetSubscriptionWindowUsage(info)
	if usage.Window5hResetAt != nextRotation {
		t.Fatalf("Window5hResetAt = %d, want natural rotation %d", usage.Window5hResetAt, nextRotation)
	}
}

// TestSubscriptionWindow5hResetAtZeroAccessEndKeepsRotation keeps legacy rows
// (access_end_time absent) on the pre-fix behaviour.
func TestSubscriptionWindow5hResetAtZeroAccessEndKeepsRotation(t *testing.T) {
	now := common.GetTimestamp()
	nextRotation := (now/subscriptionWindowBucketSeconds + 1) * subscriptionWindowBucketSeconds

	info := &model.SubscriptionWindowInfo{
		UserSubscriptionId: 93,
		SubscriptionStart:  now - 3600,
		AccessEndTime:      0,
		Window5hAmount:     100,
	}
	usage := GetSubscriptionWindowUsage(info)
	if usage.Window5hResetAt != nextRotation {
		t.Fatalf("legacy zero access-end must keep rotation: got %d, want %d", usage.Window5hResetAt, nextRotation)
	}
}

// TestReserveSubscriptionWindows5hRejectionResetAtClamped covers the rejection
// path: the "frees up in about N minutes" message must not outrun the
// subscription either.
func TestReserveSubscriptionWindows5hRejectionResetAtClamped(t *testing.T) {
	setupWindowTestRedis(t)

	now := common.GetTimestamp()
	nextRotation := (now/subscriptionWindowBucketSeconds + 1) * subscriptionWindowBucketSeconds
	accessEnd := nextRotation - 60
	if accessEnd <= now {
		t.Skip("clock sits within a minute of the rotation; skip this tick")
	}

	info := &model.SubscriptionWindowInfo{
		UserSubscriptionId: 94,
		SubscriptionStart:  now - 3600,
		AccessEndTime:      accessEnd,
		Window5hAmount:     10,
	}
	if _, err := reserveSubscriptionWindows(info, 10); err != nil {
		t.Fatalf("first reservation should fit the 5h limit: %v", err)
	}
	_, err := reserveSubscriptionWindows(info, 1)
	if err == nil {
		t.Fatal("expected 5h window rejection")
	}
	var winErr *subscriptionWindowExceededError
	if !errors.As(err, &winErr) {
		t.Fatalf("expected subscriptionWindowExceededError, got %T", err)
	}
	if winErr.Window != "5h" {
		t.Fatalf("expected 5h window rejection, got %q", winErr.Window)
	}
	if winErr.ResetAt != accessEnd {
		t.Fatalf("rejection ResetAt = %d, want %d (clamped to access end)", winErr.ResetAt, accessEnd)
	}
}
