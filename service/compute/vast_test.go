package compute

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
)

// TestOfferWhitelabelSerialization asserts a proxied offer exposes only GPU /
// price / spec fields — never the upstream contract / host / machine / provider
// identifiers.
func TestOfferWhitelabelSerialization(t *testing.T) {
	offer := Offer{
		GpuName:     "RTX 4090",
		NumGpus:     2,
		GpuRamGB:    24,
		CostPerHour: 0.55,
		CpuCores:    16,
		RamGB:       64,
		DiskGB:      200,
		Reliability: 0.987,

		// Internal-only — must not leak.
		ContractID: 987654,
		HostID:     55123,
		MachineID:  44001,
		Datacenter: "US-CA",
	}

	raw, err := common.Marshal(offer)
	if err != nil {
		t.Fatalf("marshal offer: %v", err)
	}
	out := string(raw)

	for _, want := range []string{
		`"gpu_name":"RTX 4090"`,
		`"cost_per_hour":0.55`,
		`"num_gpus":2`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected public field %s, got: %s", want, out)
		}
	}

	for _, leak := range []string{"987654", "55123", "44001", "US-CA", "contract", "host_id", "machine"} {
		if strings.Contains(strings.ToLower(out), strings.ToLower(leak)) {
			t.Errorf("whitelabel leak: offer JSON contains %q: %s", leak, out)
		}
	}
}

func TestBuildOffersQuery(t *testing.T) {
	q, err := buildOffersQuery("RTX 4090")
	if err != nil {
		t.Fatalf("buildOffersQuery: %v", err)
	}
	if !strings.Contains(q, `"gpu_name"`) || !strings.Contains(q, "RTX 4090") {
		t.Errorf("expected gpu_name filter in query, got: %s", q)
	}
	if !strings.Contains(q, `"rentable"`) {
		t.Errorf("expected rentable filter in query, got: %s", q)
	}

	// Empty gpu name → no gpu_name filter, still valid.
	q2, err := buildOffersQuery("")
	if err != nil {
		t.Fatalf("buildOffersQuery empty: %v", err)
	}
	if strings.Contains(q2, "gpu_name") {
		t.Errorf("did not expect gpu_name filter for empty input, got: %s", q2)
	}
}

func TestMapUpstreamStatus(t *testing.T) {
	cases := map[string]string{
		"running": "running",
		"exited":  "stopped",
		"offline": "stopped",
		"created": "provisioning",
		"loading": "provisioning",
		"":        "provisioning",
		"weird":   "error",
	}
	for in, want := range cases {
		if got := mapUpstreamStatus(in); got != want {
			t.Errorf("mapUpstreamStatus(%q) = %q, want %q", in, got, want)
		}
	}
}
