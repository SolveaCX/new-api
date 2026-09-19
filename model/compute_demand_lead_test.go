package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestComputeDemandLeadsAndPools(t *testing.T) {
	setupComputeMarketTestDB(t)
	if err := DB.AutoMigrate(&ComputeDemandLead{}); err != nil {
		t.Fatal(err)
	}
	a := &ComputeDemandLead{Contact: "a@example.com", GpuModel: "H200", Gpus: 32, TermMonths: 6}
	if err := a.Insert(); err != nil || len(a.Code) != 8 || a.Status != ComputeDemandLeadStatusNew {
		t.Fatalf("insert a: %v %+v", err, a)
	}
	raw, _ := common.Marshal(a)
	if strings.Contains(string(raw), "a@example.com") || strings.Contains(string(raw), "client_ip") {
		t.Fatalf("private lead fields leaked: %s", raw)
	}
	b := &ComputeDemandLead{Contact: "+1 408 555 0100", GpuModel: "H200", Gpus: 40, TermMonths: 3, ReferredBy: a.Code}
	if err := b.Insert(); err != nil {
		t.Fatal(err)
	}
	refs, _ := CountComputeDemandReferrals(a.Code)
	if refs != 1 {
		t.Fatalf("referrals = %d", refs)
	}
	got, err := GetComputeDemandLeadByCode(strings.ToLower(a.Code))
	if err != nil || got.Id != a.Id {
		t.Fatalf("lookup by code: %v", err)
	}
	rfq := newTestRFQ(t, 77, 3.0) // B300 x 20 nodes x 8 = 160 GPUs matching
	pools, err := GetComputeDemandPools()
	if err != nil {
		t.Fatal(err)
	}
	byGpu := map[string]*ComputeDemandPool{}
	for _, p := range pools {
		byGpu[p.GpuModel] = p
	}
	if byGpu["H200"] == nil || byGpu["H200"].Gpus != 72 || byGpu["H200"].Leads != 2 || byGpu["H200"].Tier != 1 || byGpu["H200"].NextTierGpu != 256 {
		t.Fatalf("H200 pool = %+v", byGpu["H200"])
	}
	if byGpu[rfq.GpuModel] == nil || byGpu[rfq.GpuModel].Gpus != 160 || byGpu[rfq.GpuModel].Requests != 1 {
		t.Fatalf("B300 pool = %+v", byGpu[rfq.GpuModel])
	}
	if pools[0].GpuModel != "B300" {
		t.Fatalf("pools should sort by size, got %s first", pools[0].GpuModel)
	}
	admin, _ := ListComputeDemandLeads(10)
	if len(admin) != 2 || admin[1].Contact != "a@example.com" || admin[1].Referrals != 1 {
		t.Fatalf("admin view = %+v", admin)
	}
}
