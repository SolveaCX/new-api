package constant

import "testing"

func TestPath2RelayModeSupportsOpenRouterDecisions(t *testing.T) {
	if got := Path2RelayMode("/api/alpha/decisions"); got != RelayModeDecisions {
		t.Fatalf("Path2RelayMode(%q) = %d, want RelayModeDecisions (%d)",
			"/api/alpha/decisions", got, RelayModeDecisions)
	}
}
