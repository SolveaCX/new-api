package service

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestUTCDateBounds(t *testing.T) {
	start, end, err := utcDateBounds("2025-08-18")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantStart := time.Date(2025, 8, 18, 0, 0, 0, 0, time.UTC).Unix()
	if start != wantStart {
		t.Errorf("start = %d, want %d", start, wantStart)
	}
	if end-start != 24*60*60 {
		t.Errorf("day length = %d, want 86400", end-start)
	}
	if _, _, err := utcDateBounds("not-a-date"); err == nil {
		t.Error("expected error for invalid date, got nil")
	}
}

func TestUTCToday(t *testing.T) {
	now := time.Date(2025, 8, 18, 15, 4, 5, 0, time.UTC)
	if got := utcToday(now); got != "2025-08-18" {
		t.Errorf("utcToday = %q, want 2025-08-18", got)
	}
	// A near-midnight instant still maps to its UTC calendar day.
	late := time.Date(2025, 8, 18, 23, 59, 59, 0, time.UTC)
	if got := utcToday(late); got != "2025-08-18" {
		t.Errorf("utcToday(late) = %q, want 2025-08-18", got)
	}
}

func TestUsageReportTaskWindowDays(t *testing.T) {
	if usageReportTaskWindowDays != 7 {
		t.Fatalf("automatic task window = %d, want 7", usageReportTaskWindowDays)
	}
}

func TestUsageReportAggregationSlotSerializes(t *testing.T) {
	var active int32
	var maxActive int32
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := withUsageReportAggregationSlot(func() error {
				current := atomic.AddInt32(&active, 1)
				for {
					max := atomic.LoadInt32(&maxActive)
					if current <= max || atomic.CompareAndSwapInt32(&maxActive, max, current) {
						break
					}
				}
				time.Sleep(5 * time.Millisecond)
				atomic.AddInt32(&active, -1)
				return nil
			}); err != nil {
				t.Errorf("aggregation slot returned error: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := atomic.LoadInt32(&maxActive); got != 1 {
		t.Fatalf("max concurrent aggregations = %d, want 1", got)
	}
}
