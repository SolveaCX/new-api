package controller

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func backfillDates(t *testing.T, query string) []string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/data/usage_report_backfill?"+query, nil)
	dates, err := usageReportBackfillDates(c)
	if err != nil {
		t.Fatalf("unexpected error for %q: %v", query, err)
	}
	return dates
}

func TestUsageReportBackfillDatesSingleDay(t *testing.T) {
	got := backfillDates(t, "date=2025-08-18")
	if len(got) != 1 || got[0] != "2025-08-18" {
		t.Fatalf("got %v, want [2025-08-18]", got)
	}
}

func TestUsageReportBackfillDatesRangeInclusive(t *testing.T) {
	got := backfillDates(t, "from=2025-08-01&to=2025-08-03")
	want := []string{"2025-08-01", "2025-08-02", "2025-08-03"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestUsageReportBackfillDatesTrailingDays(t *testing.T) {
	got := backfillDates(t, "days=3")
	if len(got) != 3 {
		t.Fatalf("got %d dates, want 3", len(got))
	}
	today := time.Now().UTC().Format(usageReportDateLayout)
	if got[len(got)-1] != today {
		t.Fatalf("last date = %s, want today %s", got[len(got)-1], today)
	}
}

func TestUsageReportBackfillDatesRejectsBadInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []string{
		"",                              // nothing specified
		"date=2025-13-40",               // invalid calendar date
		"date=not-a-date",               // invalid format
		"from=2025-08-01",               // missing to
		"from=2025-08-03&to=2025-08-01", // to before from
		"days=0",                        // non-positive
		"days=abc",                      // not a number
		"from=2020-01-01&to=2025-01-01", // span over the 180-day cap
	}
	for _, q := range cases {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/api/data/usage_report_backfill?"+q, nil)
		if _, err := usageReportBackfillDates(c); err == nil {
			t.Errorf("query %q: expected error, got nil", q)
		}
	}
}

func TestUsageReportBackfillDatesRejectsFuture(t *testing.T) {
	gin.SetMode(gin.TestMode)
	future := time.Now().UTC().AddDate(0, 0, 1).Format(usageReportDateLayout)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/data/usage_report_backfill?date="+future, nil)
	if _, err := usageReportBackfillDates(c); err == nil {
		t.Fatalf("future date %s: expected error, got nil", future)
	}
}

func TestUsageReportGroupParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		query string
		want  string
	}{
		{"", service.UsageReportGroupPLG},
		{"group=plg", service.UsageReportGroupPLG},
		{"group=PLG", service.UsageReportGroupPLG},
		{"group=all", service.UsageReportGroupAll},
	}
	for _, tc := range cases {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/api/data/usage_report?"+tc.query, nil)
		got, err := usageReportGroupParam(c)
		if err != nil {
			t.Fatalf("query %q: unexpected error %v", tc.query, err)
		}
		if got != tc.want {
			t.Errorf("query %q: got %q want %q", tc.query, got, tc.want)
		}
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/data/usage_report?group=team", nil)
	if _, err := usageReportGroupParam(c); err == nil {
		t.Error("group=team: expected error, got nil")
	}
}
