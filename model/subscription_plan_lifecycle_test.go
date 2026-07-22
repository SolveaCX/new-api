package model

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func setupSubscriptionPlanLifecycleTestDB(t *testing.T) {
	t.Helper()
	setupSubscriptionRecurringTestDB(t)
	migrateSubscriptionContractTestDB(t)
}

func lifecycleTestPlan(rank int) *SubscriptionPlan {
	return &SubscriptionPlan{
		Title:         fmt.Sprintf("Lifecycle Rank %d", rank),
		Subtitle:      "initial",
		PriceAmount:   9.99,
		Currency:      "USD",
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		SortOrder:     rank,
		TierRank:      rank,
		StripePriceId: fmt.Sprintf("price_%d", rank),
		UpgradeGroup:  "",
		TotalAmount:   1000,
	}
}

func requireReservation(t *testing.T, rank int, planID int) {
	t.Helper()
	var reservation SubscriptionTierRankReservation
	require.NoError(t, DB.First(&reservation, "tier_rank = ?", rank).Error)
	require.Equal(t, planID, reservation.PlanId)
}

func TestTierRankReservationCreatedForEnabledAndDisabledPlans(t *testing.T) {
	setupSubscriptionPlanLifecycleTestDB(t)

	enabledPlan := lifecycleTestPlan(10)
	require.NoError(t, CreateSubscriptionPlan(enabledPlan))
	requireReservation(t, 10, enabledPlan.Id)

	disabledPlan := lifecycleTestPlan(11)
	disabledPlan.Enabled = false
	require.NoError(t, CreateSubscriptionPlan(disabledPlan))
	requireReservation(t, 11, disabledPlan.Id)
}

func TestTierRankReservationRejectsDuplicateAfterDisableAndDelete(t *testing.T) {
	setupSubscriptionPlanLifecycleTestDB(t)

	plan := lifecycleTestPlan(20)
	require.NoError(t, CreateSubscriptionPlan(plan))
	requireReservation(t, 20, plan.Id)

	disabled := *plan
	disabled.Enabled = false
	require.NoError(t, UpdateSubscriptionPlan(&disabled))

	duplicateAfterDisable := lifecycleTestPlan(20)
	err := CreateSubscriptionPlan(duplicateAfterDisable)
	require.ErrorIs(t, err, ErrSubscriptionTierRankReserved)

	require.NoError(t, DB.Delete(&SubscriptionPlan{}, "id = ?", plan.Id).Error)
	requireReservation(t, 20, plan.Id)

	duplicateAfterDelete := lifecycleTestPlan(20)
	err = CreateSubscriptionPlan(duplicateAfterDelete)
	require.ErrorIs(t, err, ErrSubscriptionTierRankReserved)
	requireReservation(t, 20, plan.Id)
}

func TestTierRankReservationIdempotentRepairForSamePlanRank(t *testing.T) {
	setupSubscriptionPlanLifecycleTestDB(t)

	plan := lifecycleTestPlan(30)
	require.NoError(t, DB.Create(plan).Error)

	require.NoError(t, ReserveSubscriptionTierRank(DB, plan.TierRank, plan.Id))
	require.NoError(t, ReserveSubscriptionTierRank(DB, plan.TierRank, plan.Id))
	requireReservation(t, 30, plan.Id)
}

func TestSubscriptionPlanLifecycleCanCorrectRankBeforeReferenceWithoutReusingOldRank(t *testing.T) {
	setupSubscriptionPlanLifecycleTestDB(t)

	plan := lifecycleTestPlan(40)
	require.NoError(t, CreateSubscriptionPlan(plan))

	corrected := *plan
	corrected.TierRank = 41
	corrected.DurationValue = 2
	corrected.TotalAmount = 2000
	require.NoError(t, UpdateSubscriptionPlan(&corrected))
	requireReservation(t, 40, plan.Id)
	requireReservation(t, 41, plan.Id)

	duplicateOldRank := lifecycleTestPlan(40)
	err := CreateSubscriptionPlan(duplicateOldRank)
	require.ErrorIs(t, err, ErrSubscriptionTierRankReserved)

	duplicateNewRank := lifecycleTestPlan(41)
	err = CreateSubscriptionPlan(duplicateNewRank)
	require.ErrorIs(t, err, ErrSubscriptionTierRankReserved)
}

func TestSubscriptionPlanLifecycleFieldsImmutableAfterReference(t *testing.T) {
	setupSubscriptionPlanLifecycleTestDB(t)

	cases := []struct {
		name       string
		reference  func(t *testing.T, planID int)
		mutatePlan func(plan *SubscriptionPlan)
	}{
		{
			name: "contract current plan freezes tier rank",
			reference: func(t *testing.T, planID int) {
				require.NoError(t, DB.Create(&UserSubscriptionContract{UserId: 5001, CurrentPlanId: planID}).Error)
			},
			mutatePlan: func(plan *SubscriptionPlan) { plan.TierRank++ },
		},
		{
			name: "contract pending plan freezes duration",
			reference: func(t *testing.T, planID int) {
				require.NoError(t, DB.Create(&UserSubscriptionContract{UserId: 5002, PendingPlanId: planID}).Error)
			},
			mutatePlan: func(plan *SubscriptionPlan) { plan.DurationUnit = SubscriptionDurationYear },
		},
		{
			name: "entitlement history freezes total amount",
			reference: func(t *testing.T, planID int) {
				require.NoError(t, DB.Create(&UserSubscription{UserId: 5003, PlanId: planID, Status: "expired"}).Error)
			},
			mutatePlan: func(plan *SubscriptionPlan) { plan.TotalAmount++ },
		},
		{
			name: "order freezes stripe price id",
			reference: func(t *testing.T, planID int) {
				require.NoError(t, DB.Create(&SubscriptionOrder{UserId: 5004, PlanId: planID, TradeNo: "lifecycle-order", Status: common.TopUpStatusPending}).Error)
			},
			mutatePlan: func(plan *SubscriptionPlan) { plan.StripePriceId = "price_changed" },
		},
		{
			name: "change intent from plan freezes upgrade group",
			reference: func(t *testing.T, planID int) {
				require.NoError(t, DB.Create(&SubscriptionChangeIntent{ContractId: 1, UserId: 5005, RequestId: "from-plan", FromPlanId: planID}).Error)
			},
			mutatePlan: func(plan *SubscriptionPlan) { plan.UpgradeGroup = "vip" },
		},
		{
			name: "change intent to plan freezes custom seconds",
			reference: func(t *testing.T, planID int) {
				require.NoError(t, DB.Create(&SubscriptionChangeIntent{ContractId: 2, UserId: 5006, RequestId: "to-plan", ToPlanId: planID}).Error)
			},
			mutatePlan: func(plan *SubscriptionPlan) {
				plan.DurationUnit = SubscriptionDurationCustom
				plan.CustomSeconds = 7200
			},
		},
	}

	for i, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			setupSubscriptionPlanLifecycleTestDB(t)

			plan := lifecycleTestPlan(50 + i)
			require.NoError(t, CreateSubscriptionPlan(plan))
			tt.reference(t, plan.Id)

			updated := *plan
			tt.mutatePlan(&updated)
			err := UpdateSubscriptionPlan(&updated)
			require.ErrorIs(t, err, ErrSubscriptionPlanLifecycleFieldsImmutable)
		})
	}
}

func TestReferencedSubscriptionPlanAllowsMetadataAndDisableEdits(t *testing.T) {
	setupSubscriptionPlanLifecycleTestDB(t)

	plan := lifecycleTestPlan(70)
	require.NoError(t, CreateSubscriptionPlan(plan))
	require.NoError(t, DB.Create(&UserSubscription{UserId: 5070, PlanId: plan.Id, Status: "active"}).Error)

	updated := *plan
	updated.Title = "Renamed"
	updated.Subtitle = "metadata"
	updated.PriceAmount = 19.99
	updated.Enabled = false
	updated.SortOrder = 99

	require.NoError(t, UpdateSubscriptionPlan(&updated))

	var stored SubscriptionPlan
	require.NoError(t, DB.First(&stored, "id = ?", plan.Id).Error)
	require.Equal(t, "Renamed", stored.Title)
	require.Equal(t, "metadata", stored.Subtitle)
	require.Equal(t, 19.99, stored.PriceAmount)
	require.False(t, stored.Enabled)
	require.Equal(t, 99, stored.SortOrder)
	requireReservation(t, 70, plan.Id)
}

func TestLegacyTierRankZeroPlanCanUpdateMetadataWithoutReservation(t *testing.T) {
	setupSubscriptionPlanLifecycleTestDB(t)

	plan := lifecycleTestPlan(0)
	require.NoError(t, DB.Create(plan).Error)

	updated := *plan
	updated.Title = "Legacy renamed"
	updated.Enabled = false
	require.NoError(t, UpdateSubscriptionPlan(&updated))

	var count int64
	require.NoError(t, DB.Model(&SubscriptionTierRankReservation{}).Where("plan_id = ?", plan.Id).Count(&count).Error)
	require.Zero(t, count)
}

func TestConcurrentCreateSubscriptionPlanSameTierRankReturnsReservedError(t *testing.T) {
	setupSubscriptionPlanLifecycleTestDB(t)

	const rank = 90
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			plan := lifecycleTestPlan(rank)
			plan.Title = fmt.Sprintf("Concurrent %d", i)
			errs[i] = CreateSubscriptionPlan(plan)
		}(i)
	}
	wg.Wait()

	successes := 0
	reserved := 0
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrSubscriptionTierRankReserved):
			reserved++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, reserved)
}
