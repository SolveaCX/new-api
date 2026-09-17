package compute

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseRFQHeuristicChinese(t *testing.T) {
	text := "我们想在日本租 20 台 B300 服务器,每台 8 卡,从 11 月 1 号开始租一年,预算每卡每小时 3.5 到 5 美金,需要裸金属和 IB 400G,存储要 2PB 左右的 NVMe,机房要 SOC2 或 ISO,SLA 99.5 以上,数据不能出日本。"
	d := ParseRFQHeuristic(text)
	if d.GpuModel != "B300" {
		t.Fatalf("gpu_model = %q", d.GpuModel)
	}
	if d.Nodes != 20 || d.GpusPerNode != 8 {
		t.Fatalf("nodes=%d gpus_per_node=%d", d.Nodes, d.GpusPerNode)
	}
	if d.TermMonths != 12 {
		t.Fatalf("term_months = %d", d.TermMonths)
	}
	if len(d.StartDate) != 10 || d.StartDate[5:] != "11-01" {
		t.Fatalf("start_date = %q", d.StartDate)
	}
	if d.TargetPriceMin != 3.5 || d.TargetPriceMax != 5 || d.PriceCeiling != 5 {
		t.Fatalf("price band = %v-%v ceiling %v", d.TargetPriceMin, d.TargetPriceMax, d.PriceCeiling)
	}
	if len(d.Regions) != 1 || d.Regions[0] != "JP" {
		t.Fatalf("regions = %v", d.Regions)
	}
	if d.Delivery != "bare_metal" || d.Interconnect != "IB 400G" || d.StorageTB != 2000 {
		t.Fatalf("delivery=%q interconnect=%q storage=%v", d.Delivery, d.Interconnect, d.StorageTB)
	}
	for _, want := range []string{"SOC 2", "ISO 27001", "SLA 99.5%", "Data residency"} {
		if !containsString(d.Compliance, want) {
			t.Fatalf("compliance missing %q in %v", want, d.Compliance)
		}
	}
}

func TestParseRFQHeuristicEnglishAndMissing(t *testing.T) {
	d := ParseRFQHeuristic("Need 4 nodes of H200 (8 GPUs per node) in Singapore for 3 months starting Oct 15, up to $3.2/GPU-hr, Slurm please.")
	if d.GpuModel != "H200" || d.Nodes != 4 || d.GpusPerNode != 8 || d.TermMonths != 3 {
		t.Fatalf("unexpected draft: %+v", d)
	}
	if d.PriceCeiling != 3.2 {
		t.Fatalf("price_ceiling = %v", d.PriceCeiling)
	}
	if len(d.Regions) != 1 || d.Regions[0] != "SG" || d.Delivery != "slurm" {
		t.Fatalf("regions=%v delivery=%q", d.Regions, d.Delivery)
	}
	if d.StartDate[5:] != "10-15" {
		t.Fatalf("start_date = %q", d.StartDate)
	}

	empty := ParseRFQText(t.Context(), "hello")
	if empty.Source != "heuristic" {
		t.Fatalf("source = %q", empty.Source)
	}
	if len(empty.Missing) != len(rfqRequiredFields) {
		t.Fatalf("expected all required fields missing, got %v", empty.Missing)
	}
}

func TestParseRFQDraftAlwaysEmitsArrays(t *testing.T) {
	for _, text := range []string{"", "hello", "20 台 B300 在日本"} {
		raw, err := json.Marshal(ParseRFQText(t.Context(), text))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), `"filled":null`) || strings.Contains(string(raw), `"missing":null`) {
			t.Fatalf("draft for %q serialised null arrays: %s", text, raw)
		}
	}
}
