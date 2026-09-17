package model

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupComputeMarketTestDB(t *testing.T) {
	t.Helper()
	originalDB := DB
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "compute-market.db")+"?_pragma=busy_timeout(5000)"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&ComputeRFQ{}, &ComputeBid{}, &ComputeSupplier{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	DB = db
	t.Cleanup(func() {
		DB = originalDB
		if sqlDB, _ := db.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
}

func newTestRFQ(t *testing.T, buyer int, ceiling float64) *ComputeRFQ {
	t.Helper()
	rfq := &ComputeRFQ{UserId: buyer, GpuModel: "B300", GpusPerNode: 8, Nodes: 20, TermMonths: 12, StartDate: "2026-11-01",
		PriceCeiling: ceiling, TargetPriceMin: 3.5, TargetPriceMax: 4, Regions: "JP"}
	if err := rfq.Insert(); err != nil {
		t.Fatalf("insert rfq: %v", err)
	}
	return rfq
}

func TestComputeRFQSerializationHidesPrivateFields(t *testing.T) {
	rfq := &ComputeRFQ{Id: 41, UserId: 117, GpuModel: "B300", TargetPriceMin: 3.5, TargetPriceMax: 4, Status: ComputeRFQStatusMatching}
	rfq.decorate(999)
	raw, err := common.Marshal(rfq)
	if err != nil {
		t.Fatal(err)
	}
	out := string(raw)
	for _, leak := range []string{"user_id", "target_price", "3.5", `"117"`} {
		if strings.Contains(out, leak) {
			t.Fatalf("private field leaked: %s in %s", leak, out)
		}
	}
	if !strings.Contains(out, `"code":"RFQ-41"`) || !strings.Contains(out, `"buyer_alias":"#B-`) {
		t.Fatalf("missing derived fields: %s", out)
	}
	bid := &ComputeBid{Id: 1, SupplierUserId: 55, PricePerGpuHour: 3.85}
	bid.SupplierAlias = ComputeMarketAlias("S", 55)
	raw, _ = common.Marshal(bid)
	if strings.Contains(string(raw), "supplier_user_id") || strings.Contains(string(raw), `:55`) {
		t.Fatalf("supplier id leaked: %s", raw)
	}
	if ComputeMarketAlias("S", 55) != ComputeMarketAlias("S", 55) || ComputeMarketAlias("S", 55) == ComputeMarketAlias("B", 55) {
		t.Fatal("alias must be stable per side")
	}
}

func TestComputeMarketBidLifecycle(t *testing.T) {
	setupComputeMarketTestDB(t)
	buyer, s1, s2, s3 := 1, 2, 3, 4
	rfq := newTestRFQ(t, buyer, 5.0)
	if rfq.Status != ComputeRFQStatusMatching || rfq.MatchingDeadline <= common.GetTimestamp() {
		t.Fatalf("rfq not matching: %+v", rfq)
	}

	// Ceiling enforced.
	if _, err := PlaceComputeBid(rfq, s1, 5.0, "2026-11-01", 99.5, "{}", "", true); err != ErrComputeBidTooHigh {
		t.Fatalf("expected ceiling error, got %v", err)
	}
	b1, err := PlaceComputeBid(rfq, s1, 4.60, "2026-11-15", 99.5, `{"region":true}`, "IB 200G", true)
	if err != nil {
		t.Fatal(err)
	}
	// Re-bid must be lower by >= 0.05 and updates the same row.
	if _, err := PlaceComputeBid(rfq, s1, 4.58, "2026-11-15", 99.5, "{}", "", true); err != ErrComputeBidNotLower {
		t.Fatalf("expected not-lower error, got %v", err)
	}
	b1b, err := PlaceComputeBid(rfq, s1, 3.85, "2026-11-15", 99.5, "{}", "IB 200G", true)
	if err != nil || b1b.Id != b1.Id || b1b.PricePerGpuHour != 3.85 {
		t.Fatalf("re-bid failed: %v %+v", err, b1b)
	}
	if _, err := PlaceComputeBid(rfq, s2, 4.20, "2026-11-01", 99.9, "{}", "", true); err != nil {
		t.Fatal(err)
	}
	// Ineligible (hard requirement failed) bid is visible but never rankable.
	if _, err := PlaceComputeBid(rfq, s3, 3.60, "2026-11-01", 99.5, `{"region":false}`, "region mismatch", false); err != nil {
		t.Fatal(err)
	}

	bids, err := ListComputeBidsByRFQ(rfq.Id, buyer)
	if err != nil {
		t.Fatal(err)
	}
	if len(bids) != 3 || bids[0].PricePerGpuHour != 3.60 || bids[0].Rank != 0 || bids[1].Rank != 1 || bids[2].Rank != 2 {
		for _, b := range bids {
			t.Logf("%+v", b)
		}
		t.Fatal("unexpected ladder ordering/ranks")
	}
	if LowestEligibleComputeBidPrice(bids) != 3.85 {
		t.Fatalf("lowest eligible = %v", LowestEligibleComputeBidPrice(bids))
	}
	top := TopEligibleComputeBids(bids, 3)
	if len(top) != 2 || top[0].PricePerGpuHour != 3.85 {
		t.Fatalf("top = %+v", top)
	}

	// Ineligible bid cannot be accepted; non-owner cannot accept.
	if _, err := AcceptComputeBid(rfq, bids[0].Id, buyer); err != ErrComputeBidNotAcceptable {
		t.Fatalf("expected not-acceptable, got %v", err)
	}
	if _, err := AcceptComputeBid(rfq, bids[2].Id, s1); err != ErrComputeRFQNotOwner {
		t.Fatalf("expected not-owner, got %v", err)
	}
	// Buyer may accept a non-lowest eligible bid while matching (Didi-style).
	accepted, err := AcceptComputeBid(rfq, bids[2].Id, buyer)
	if err != nil {
		t.Fatal(err)
	}
	if accepted.PricePerGpuHour != 4.20 || rfq.Status != ComputeRFQStatusMatched || rfq.AcceptedBidId != accepted.Id {
		t.Fatalf("accept state wrong: %+v rfq=%+v", accepted, rfq)
	}
	after, _ := ListComputeBidsByRFQ(rfq.Id, buyer)
	for _, b := range after {
		switch {
		case b.Id == accepted.Id && b.Status != ComputeBidStatusAccepted:
			t.Fatalf("accepted bid status %s", b.Status)
		case b.Id != accepted.Id && b.Status != ComputeBidStatusRejected:
			t.Fatalf("other bid %d status %s", b.Id, b.Status)
		}
	}
	// No more bids once matched.
	if _, err := PlaceComputeBid(rfq, s2, 4.0, "", 0, "{}", "", true); err != ErrComputeRFQNotMatching {
		t.Fatalf("expected not-matching, got %v", err)
	}
}

func TestComputeMarketAntiSnipeAndChoosing(t *testing.T) {
	setupComputeMarketTestDB(t)
	buyer := 10
	rfq := newTestRFQ(t, buyer, 5.0)
	now := common.GetTimestamp()

	// A bid inside the last 15 minutes extends the deadline.
	rfq.MatchingDeadline = now + 5*60
	if err := rfq.save(); err != nil {
		t.Fatal(err)
	}
	if _, err := PlaceComputeBid(rfq, 11, 4.0, "", 0, "{}", "", true); err != nil {
		t.Fatal(err)
	}
	if rfq.MatchingDeadline < now+ComputeMarketAntiSnipeSeconds-1 {
		t.Fatalf("anti-snipe did not extend: %d vs now %d", rfq.MatchingDeadline, now)
	}
	for i, s := range []int{12, 13, 14} {
		if _, err := PlaceComputeBid(rfq, s, 4.5+float64(i)*0.1, "", 0, "{}", "", true); err != nil {
			t.Fatal(err)
		}
	}

	// Expire the window: reading the RFQ flips it to choosing.
	rfq.MatchingDeadline = now - 1
	if err := rfq.save(); err != nil {
		t.Fatal(err)
	}
	reloaded, err := GetComputeRFQById(rfq.Id, buyer)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Status != ComputeRFQStatusChoosing {
		t.Fatalf("status = %s", reloaded.Status)
	}
	bids, _ := ListComputeBidsByRFQ(rfq.Id, buyer)
	// Fourth-lowest bid (4.7) is outside the top 3 and cannot be accepted.
	var fourth, second *ComputeBid
	for _, b := range bids {
		if b.PricePerGpuHour == 4.7 {
			fourth = b
		}
		if b.PricePerGpuHour == 4.5 {
			second = b
		}
	}
	if _, err := AcceptComputeBid(reloaded, fourth.Id, buyer); err != ErrComputeBidNotAcceptable {
		t.Fatalf("expected top-3 restriction, got %v", err)
	}
	// Extending returns to matching, capped at 3 extensions.
	if err := ExtendComputeRFQ(reloaded, buyer); err != nil {
		t.Fatal(err)
	}
	if reloaded.Status != ComputeRFQStatusMatching || reloaded.Extensions != 1 || reloaded.MatchingDeadline < now+ComputeMarketMatchingWindowSeconds-5 {
		t.Fatalf("extend state wrong: %+v", reloaded)
	}
	reloaded.Extensions = ComputeMarketMaxExtensions
	if err := ExtendComputeRFQ(reloaded, buyer); err != ErrComputeRFQMaxExtensions {
		t.Fatalf("expected max extensions, got %v", err)
	}
	// Raising the ceiling only goes up.
	if err := RaiseComputeRFQCeiling(reloaded, buyer, 4.9); err != ErrComputeCeilingNotHigher {
		t.Fatalf("expected ceiling error, got %v", err)
	}
	if err := RaiseComputeRFQCeiling(reloaded, buyer, 5.5); err != nil || reloaded.PriceCeiling != 5.5 {
		t.Fatalf("raise failed: %v", err)
	}
	// Accept second-lowest while matching again, then cancel is refused.
	if _, err := AcceptComputeBid(reloaded, second.Id, buyer); err != nil {
		t.Fatal(err)
	}
	if err := CancelComputeRFQ(reloaded, buyer); err != ErrComputeRFQNotMatching {
		t.Fatalf("expected cancel refused after match, got %v", err)
	}
}

func TestComputeMarketCancelAndSupplier(t *testing.T) {
	setupComputeMarketTestDB(t)
	rfq := newTestRFQ(t, 20, 3.0)
	if _, err := PlaceComputeBid(rfq, 21, 2.5, "", 0, "{}", "", true); err != nil {
		t.Fatal(err)
	}
	if err := CancelComputeRFQ(rfq, 21); err != ErrComputeRFQNotOwner {
		t.Fatalf("expected not-owner, got %v", err)
	}
	if err := CancelComputeRFQ(rfq, 20); err != nil {
		t.Fatal(err)
	}
	bids, _ := ListComputeBidsByRFQ(rfq.Id, 21)
	if bids[0].Status != ComputeBidStatusRejected {
		t.Fatalf("bid should be rejected on cancel, got %s", bids[0].Status)
	}
	open, _ := ListOpenComputeRFQs(21)
	if len(open) != 0 {
		t.Fatalf("cancelled rfq still open: %d", len(open))
	}

	s, err := UpsertComputeSupplier(21, "Tokyo GPU KK", "ops@example.com", "JP,SG", "24x B300")
	if err != nil || s.Level != ComputeSupplierLevelRegistered {
		t.Fatalf("upsert: %v %+v", err, s)
	}
	s2, err := UpsertComputeSupplier(21, "Tokyo GPU KK", "ops@example.com", "JP", "24x B300, 8x H200")
	if err != nil || s2.Id != s.Id || s2.Regions != "JP" {
		t.Fatalf("second upsert: %v %+v", err, s2)
	}
	if err := SetComputeSupplierLevel(21, 5); err == nil {
		t.Fatal("level range not enforced")
	}
	if err := SetComputeSupplierLevel(21, ComputeSupplierLevelVerified); err != nil {
		t.Fatal(err)
	}
	got, _ := GetComputeSupplierByUserId(21)
	if got.Level != ComputeSupplierLevelVerified {
		t.Fatalf("level = %d", got.Level)
	}
	if _, err := WithdrawComputeBid(0, 21), error(nil); err != nil {
		_ = err
	}
}

func TestListRecentlyMatchedComputeRFQs(t *testing.T) {
	setupComputeMarketTestDB(t)
	rfq := newTestRFQ(t, 30, 4.0)
	if _, err := PlaceComputeBid(rfq, 31, 3.0, "", 0, "{}", "", true); err != nil {
		t.Fatal(err)
	}
	bids, _ := ListComputeBidsByRFQ(rfq.Id, 30)
	if _, err := AcceptComputeBid(rfq, bids[0].Id, 30); err != nil {
		t.Fatal(err)
	}
	recent, err := ListRecentlyMatchedComputeRFQs(31, common.GetTimestamp()-60)
	if err != nil || len(recent) != 1 || recent[0].Status != ComputeRFQStatusMatched || recent[0].Mine {
		t.Fatalf("recent = %v %+v", err, recent)
	}
	old, _ := ListRecentlyMatchedComputeRFQs(31, common.GetTimestamp()+60)
	if len(old) != 0 {
		t.Fatalf("expected no rows after since, got %d", len(old))
	}
}
