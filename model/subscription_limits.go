package model

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// StandardSubscriptionPlanLimit is the public USD contract for the three
// standard plans. Database amounts are stored as quota units (USD multiplied
// by common.QuotaPerUnit); keeping the contract in dollars avoids coupling
// product pricing to an installation's quota-unit setting.
type StandardSubscriptionPlanLimit struct {
	Title         string
	PriceUSD      float64
	Window5hUSD   float64
	WindowWeekUSD float64
	MonthlyUSD    float64
}

// Bump this marker whenever the published standard-plan contract changes so a
// deployment that already applied an earlier contract gets the new values.
const subscriptionStandardLimitsMigrationKey = "subscription_standard_limits_v5"

var standardSubscriptionPlanLimits = []StandardSubscriptionPlanLimit{
	{Title: "Go", PriceUSD: 10, Window5hUSD: 8, WindowWeekUSD: 12, MonthlyUSD: 25},
	{Title: "Pro", PriceUSD: 30, Window5hUSD: 18, WindowWeekUSD: 45, MonthlyUSD: 90},
	{Title: "Max", PriceUSD: 100, Window5hUSD: 78, WindowWeekUSD: 220, MonthlyUSD: 450},
}

// StandardSubscriptionPlanLimits returns a copy of the product contract.
func StandardSubscriptionPlanLimits() []StandardSubscriptionPlanLimit {
	limits := make([]StandardSubscriptionPlanLimit, len(standardSubscriptionPlanLimits))
	copy(limits, standardSubscriptionPlanLimits)
	return limits
}

func standardSubscriptionPlanTitle(title string) string {
	normalizedTitle := strings.ToLower(strings.TrimSpace(title))
	// Staging payment plans may be prefixed with [TEST]; treat that explicit
	// alias as the same standard tier while leaving custom titles untouched.
	if strings.HasPrefix(normalizedTitle, "[test]") {
		normalizedTitle = strings.TrimSpace(strings.TrimPrefix(normalizedTitle, "[test]"))
	}
	return normalizedTitle
}

func standardSubscriptionPlanLimit(title string, priceUSD float64, currency string) (StandardSubscriptionPlanLimit, bool) {
	if currency != "" && !strings.EqualFold(strings.TrimSpace(currency), "USD") {
		return StandardSubscriptionPlanLimit{}, false
	}
	normalizedTitle := standardSubscriptionPlanTitle(title)
	for _, limit := range standardSubscriptionPlanLimits {
		if strings.ToLower(limit.Title) != normalizedTitle {
			continue
		}
		if math.Abs(priceUSD-limit.PriceUSD) > 0.000001 {
			return StandardSubscriptionPlanLimit{}, false
		}
		return limit, true
	}
	return StandardSubscriptionPlanLimit{}, false
}

func subscriptionQuotaFromUSDWithUnit(usd, quotaPerUnit float64) int64 {
	if usd <= 0 || quotaPerUnit <= 0 {
		return 0
	}
	return decimal.NewFromFloat(usd).
		Mul(decimal.NewFromFloat(quotaPerUnit)).
		Ceil().IntPart()
}

func applyStandardSubscriptionPlanLimit(plan *SubscriptionPlan, limit StandardSubscriptionPlanLimit) {
	applyStandardSubscriptionPlanLimitWithUnit(plan, limit, common.QuotaPerUnit)
}

func applyStandardSubscriptionPlanLimitWithUnit(plan *SubscriptionPlan, limit StandardSubscriptionPlanLimit, quotaPerUnit float64) {
	if plan == nil {
		return
	}
	plan.DurationUnit = SubscriptionDurationMonth
	plan.DurationValue = 1
	plan.CustomSeconds = 0
	plan.QuotaResetPeriod = SubscriptionResetMonthly
	plan.QuotaResetCustomSeconds = 0
	plan.TotalAmount = subscriptionQuotaFromUSDWithUnit(limit.MonthlyUSD, quotaPerUnit)
	plan.Window5hAmount = subscriptionQuotaFromUSDWithUnit(limit.Window5hUSD, quotaPerUnit)
	plan.WindowWeekAmount = subscriptionQuotaFromUSDWithUnit(limit.WindowWeekUSD, quotaPerUnit)
}

// InitDB runs before InitOptionMap. Read the persisted quota unit directly so
// migration values are not accidentally scaled using the process default.
func subscriptionQuotaPerUnitForMigration(db *gorm.DB) float64 {
	quotaPerUnit := common.QuotaPerUnit
	if db == nil || !db.Migrator().HasTable(&Option{}) {
		return quotaPerUnit
	}
	var option Option
	if err := db.Where(&Option{Key: "QuotaPerUnit"}).First(&option).Error; err != nil {
		return quotaPerUnit
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(option.Value), 64)
	if err != nil || parsed <= 0 || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return quotaPerUnit
	}
	return parsed
}

func loadPersistedQuotaPerUnit(db *gorm.DB) {
	if db == nil {
		return
	}
	if quotaPerUnit := subscriptionQuotaPerUnitForMigration(db); quotaPerUnit > 0 {
		common.QuotaPerUnit = quotaPerUnit
	}
}

// migrateStandardSubscriptionPlanLimits restores limits on existing canonical
// Go/Pro/Max rows. It deliberately does not create payment plans: provider
// product IDs, tier ranks, and enablement are operator-owned values. A row is
// identified by title + USD price so custom plans at the same price remain
// untouched. If an operator has accidentally created duplicate rows for one
// tier, the single enabled row is treated as canonical; multiple enabled (or
// multiple disabled) rows remain unresolved and are retried on a later start.
// Active entitlements for a migrated plan receive the same quota snapshot so
// existing subscribers are governed by the corrected contract immediately.
func migrateStandardSubscriptionPlanLimits() error {
	if DB == nil || !DB.Migrator().HasTable(&SubscriptionPlan{}) {
		return nil
	}
	hasOptions := DB.Migrator().HasTable(&Option{})
	if hasOptions {
		var marker Option
		err := DB.Where(&Option{Key: subscriptionStandardLimitsMigrationKey}).First(&marker).Error
		if err == nil {
			return nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}

	matched := false
	ambiguous := false
	quotaPerUnit := subscriptionQuotaPerUnitForMigration(DB)
	err := DB.Transaction(func(tx *gorm.DB) error {
		var plans []SubscriptionPlan
		if err := tx.Find(&plans).Error; err != nil {
			return err
		}
		limitsByIndex := make(map[int]StandardSubscriptionPlanLimit)
		candidates := make(map[string][]int)
		for i := range plans {
			plan := &plans[i]
			limit, ok := standardSubscriptionPlanLimit(plan.Title, plan.PriceAmount, plan.Currency)
			if !ok {
				continue
			}
			limitsByIndex[i] = limit
			key := standardSubscriptionPlanTitle(plan.Title)
			candidates[key] = append(candidates[key], i)
		}

		// Resolve each tier independently. A duplicate Pro row must not prevent
		// the unambiguous Go and Max rows from being repaired.
		selected := make(map[int]StandardSubscriptionPlanLimit)
		for key, indexes := range candidates {
			selectedIndexes := indexes
			if len(indexes) > 1 {
				enabledIndexes := make([]int, 0, len(indexes))
				for _, index := range indexes {
					if plans[index].Enabled {
						enabledIndexes = append(enabledIndexes, index)
					}
				}
				switch len(enabledIndexes) {
				case 1:
					selectedIndexes = enabledIndexes
					common.SysLog(fmt.Sprintf("found duplicate standard subscription plan title %q (%d matching rows); selecting the only enabled row", key, len(indexes)))
				default:
					ambiguous = true
					common.SysLog(fmt.Sprintf("skip standard subscription limit migration for ambiguous plan title %q (%d matching rows, %d enabled)", key, len(indexes), len(enabledIndexes)))
					continue
				}
			}
			for _, index := range selectedIndexes {
				selected[index] = limitsByIndex[index]
			}
		}

		if len(selected) == 0 {
			return nil
		}
		hasSubscriptions := tx.Migrator().HasTable(&UserSubscription{})
		now := common.GetTimestamp()
		for i, limit := range selected {
			plan := &plans[i]
			matched = true
			applyStandardSubscriptionPlanLimitWithUnit(plan, limit, quotaPerUnit)
			updates := map[string]any{
				"duration_unit":              plan.DurationUnit,
				"duration_value":             plan.DurationValue,
				"custom_seconds":             plan.CustomSeconds,
				"total_amount":               plan.TotalAmount,
				"window_5h_amount":           plan.Window5hAmount,
				"window_week_amount":         plan.WindowWeekAmount,
				"quota_reset_period":         plan.QuotaResetPeriod,
				"quota_reset_custom_seconds": plan.QuotaResetCustomSeconds,
				"updated_at":                 now,
			}
			if err := tx.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(updates).Error; err != nil {
				return err
			}
			if hasSubscriptions {
				snapshotUpdates := map[string]any{
					"amount_total":       plan.TotalAmount,
					"window_5h_amount":   plan.Window5hAmount,
					"window_week_amount": plan.WindowWeekAmount,
					"updated_at":         now,
				}
				if err := tx.Model(&UserSubscription{}).
					Where("plan_id = ? AND status = ? AND end_time > ?", plan.Id, SubscriptionEntitlementStatusActive, now).
					Updates(snapshotUpdates).Error; err != nil {
					return err
				}
			}
		}
		if !matched || !hasOptions || ambiguous {
			return nil
		}
		return tx.Create(&Option{Key: subscriptionStandardLimitsMigrationKey, Value: "applied"}).Error
	})
	if err != nil {
		if hasOptions {
			var marker Option
			if markerErr := DB.Where(&Option{Key: subscriptionStandardLimitsMigrationKey}).First(&marker).Error; markerErr == nil {
				return nil
			}
		}
		return err
	}
	if matched {
		common.SysLog("restored standard subscription plan window limits")
	}
	return nil
}
