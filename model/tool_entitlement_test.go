package model

import "testing"

// Plan entitlement decides whether a paying customer can use the marketplace at
// all, so the matching rule gets pinned down explicitly. The rule is an
// exclusion list: only the entry-level Go plan is denied, everything else is
// granted, so adding a new premium plan cannot silently lock customers out.
func TestPlanGrantsToolAccess(t *testing.T) {
	cases := []struct {
		title string
		want  bool
	}{
		// Entry-level plan: models only.
		{"Go", false},
		{"go", false},
		{"  GO  ", false},
		{"Go Monthly", false},
		{"Go Yearly", false},

		// Pro and above include tools.
		{"Pro", true},
		{"pro", true},
		{"Pro Monthly", true},
		{"Max", true},
		{"Team", true},
		{"Enterprise", true},

		// A plan that merely starts with the letters "go" is not the Go plan.
		{"Gold", true},
		{"Google Partner", true},

		// No plan title at all cannot grant access.
		{"", false},
	}

	for _, tc := range cases {
		t.Run(tc.title, func(t *testing.T) {
			if got := PlanGrantsToolAccess(tc.title); got != tc.want {
				t.Fatalf("PlanGrantsToolAccess(%q) = %v, want %v", tc.title, got, tc.want)
			}
		})
	}
}

func TestCheckToolAccess_RejectsInvalidUser(t *testing.T) {
	access, err := CheckToolAccess(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if access.Allowed {
		t.Fatal("an unauthenticated caller must never be granted tool access")
	}
	if access.Reason != "unauthorized" {
		t.Fatalf("reason = %q, want unauthorized", access.Reason)
	}
}
