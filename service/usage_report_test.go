package service

import (
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

func TestUsageReportBackfillDaysCapsToRecentWindow(t *testing.T) {
	if got := usageReportBackfillDays(30); got != usageReportBackfillMaxDays {
		t.Fatalf("got %d days, want %d", got, usageReportBackfillMaxDays)
	}
	if got := usageReportBackfillDays(3); got != 3 {
		t.Fatalf("got %d days, want 3", got)
	}
}
