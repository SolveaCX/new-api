package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// Tool access is a plan entitlement: the entry-level plan (Go) buys model
// tokens only, and data tools unlock from Pro upwards.
//
// The check lives in the model layer, not the UI, because hiding a button is
// not access control — an agent calling /api/tools/run directly must hit the
// same gate a browser does.

// toolIneligiblePlanKeywords lists the plan titles that do NOT include tool
// access. Matching on an exclusion list rather than an inclusion list means a
// newly added premium plan grants tools by default instead of silently
// locking out paying customers until someone remembers to whitelist it.
var toolIneligiblePlanKeywords = []string{"go"}

// PlanGrantsToolAccess reports whether a plan title carries tool entitlement.
func PlanGrantsToolAccess(planTitle string) bool {
	normalized := strings.ToLower(strings.TrimSpace(planTitle))
	if normalized == "" {
		return false
	}
	for _, blocked := range toolIneligiblePlanKeywords {
		// Exact word match: "Go" is blocked, "Go Pro" is not.
		if normalized == blocked || strings.HasPrefix(normalized, blocked+" ") {
			return false
		}
	}
	return true
}

// ToolAccess describes why a user may or may not call data tools.
type ToolAccess struct {
	Allowed bool   `json:"allowed"`
	Plan    string `json:"plan"`
	// Reason is empty when allowed; otherwise it is a translation key the
	// caller renders.
	Reason string `json:"reason,omitempty"`
}

// CheckToolAccess resolves a user's tool entitlement from their active
// subscriptions. A user with several active plans is granted access when any
// one of them qualifies — the most generous plan wins, which is what a
// customer who upgraded mid-cycle expects.
func CheckToolAccess(userId int) (ToolAccess, error) {
	if userId <= 0 {
		return ToolAccess{Reason: "unauthorized"}, nil
	}

	now := common.GetTimestamp()
	var subs []UserSubscription
	if err := DB.Where("user_id = ? AND status = ? AND end_time > ?", userId, "active", now).
		Find(&subs).Error; err != nil {
		return ToolAccess{}, err
	}

	if len(subs) == 0 {
		return ToolAccess{Reason: "no_active_plan"}, nil
	}

	planIds := make([]int, 0, len(subs))
	for _, s := range subs {
		planIds = append(planIds, s.PlanId)
	}

	var plans []SubscriptionPlan
	if err := DB.Where("id IN ?", planIds).Find(&plans).Error; err != nil {
		return ToolAccess{}, err
	}

	best := ToolAccess{Reason: "plan_not_eligible"}
	for _, p := range plans {
		if PlanGrantsToolAccess(p.Title) {
			return ToolAccess{Allowed: true, Plan: p.Title}, nil
		}
		// Remember a plan name so the upsell message can be specific about
		// what the user currently has.
		if best.Plan == "" {
			best.Plan = p.Title
		}
	}
	return best, nil
}
