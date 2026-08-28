package model

import (
	"fmt"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestClassifyInviteBenefitRiskR1ThresholdAnd79PercentBoundary(t *testing.T) {
	rule, reason, matched := classifyInviteBenefitRisk(inviteBenefitRiskMetrics{
		PaidInviteeCount:          5,
		MinimumAmountInviteeCount: 4,
		MinimumAmountRatioBPS:     8000,
		FirstPaymentSpanSeconds:   InviteBenefitRiskR2MaxSpanSeconds + 1,
	})
	require.True(t, matched)
	require.Equal(t, InviteBenefitRiskRuleR1, rule)
	require.Equal(t, InviteBenefitRiskReasonR1, reason)

	rule, reason, matched = classifyInviteBenefitRisk(inviteBenefitRiskMetrics{
		PaidInviteeCount:          100,
		MinimumAmountInviteeCount: 79,
		MinimumAmountRatioBPS:     7900,
	})
	require.False(t, matched)
	require.Empty(t, rule)
	require.Empty(t, reason)
}

func TestClassifyInviteBenefitRiskR2TimeBoundaryAndPriority(t *testing.T) {
	rule, reason, matched := classifyInviteBenefitRisk(inviteBenefitRiskMetrics{
		PaidInviteeCount:          3,
		MinimumAmountInviteeCount: 3,
		MinimumAmountRatioBPS:     10000,
		FirstPaymentSpanSeconds:   InviteBenefitRiskR2MaxSpanSeconds,
	})
	require.True(t, matched)
	require.Equal(t, InviteBenefitRiskRuleR2, rule, "R2 must win when both R1 and R2 match")
	require.Equal(t, InviteBenefitRiskReasonR2, reason)

	rule, reason, matched = classifyInviteBenefitRisk(inviteBenefitRiskMetrics{
		PaidInviteeCount:          3,
		MinimumAmountInviteeCount: 3,
		MinimumAmountRatioBPS:     10000,
		FirstPaymentSpanSeconds:   InviteBenefitRiskR2MaxSpanSeconds + 1,
	})
	require.True(t, matched)
	require.Equal(t, InviteBenefitRiskRuleR1, rule)
	require.Equal(t, InviteBenefitRiskReasonR1, reason)
}

func TestInviteBenefitRiskMetricsDeduplicatesMirrorsAndSkipsNonUSDAndZero(t *testing.T) {
	setupInviteRewardModelTest(t)
	inviter := createInviteRewardUser(t, "risk_metrics_inviter", 0)
	paid := createInviteRewardUser(t, "risk_metrics_paid", inviter.Id)
	statusMismatch := createInviteRewardUser(t, "risk_metrics_status_mismatch", inviter.Id)
	balance := createInviteRewardUser(t, "risk_metrics_balance", inviter.Id)
	nonUSD := createInviteRewardUser(t, "risk_metrics_non_usd", inviter.Id)
	emptyCurrency := createInviteRewardUser(t, "risk_metrics_empty_currency", inviter.Id)
	zero := createInviteRewardUser(t, "risk_metrics_zero", inviter.Id)

	baseTime := int64(1_787_900_000)
	require.NoError(t, DB.Create(&SubscriptionOrder{
		UserId: paid.Id, TradeNo: "mirror-trade", Status: common.TopUpStatusSuccess,
		PaymentCurrency: "USD", PaymentProvider: PaymentProviderStripe,
		PaymentAmountMinor: 1000, Money: 5,
		CreateTime: baseTime, CompleteTime: baseTime,
	}).Error)
	require.NoError(t, DB.Create(&TopUp{
		UserId: paid.Id, TradeNo: "mirror-trade", Status: common.TopUpStatusSuccess,
		PaymentCurrency: "USD", PaymentProvider: PaymentProviderStripe,
		PaymentAmountMinor: 1000, Money: 5,
		CreateTime: baseTime, CompleteTime: baseTime,
	}).Error)
	require.NoError(t, DB.Create(&SubscriptionOrder{
		UserId: statusMismatch.Id, TradeNo: "status-mismatch-trade", Status: common.TopUpStatusPending,
		PaymentCurrency: "USD", PaymentProvider: PaymentProviderStripe, Money: 5,
		CreateTime: baseTime + 1,
	}).Error)
	require.NoError(t, DB.Create(&TopUp{
		UserId: statusMismatch.Id, TradeNo: "status-mismatch-trade", Status: common.TopUpStatusSuccess,
		PaymentCurrency: "USD", PaymentProvider: PaymentProviderStripe, Money: 5,
		CreateTime: baseTime + 1, CompleteTime: baseTime + 1,
	}).Error)
	require.NoError(t, DB.Create(&TopUp{
		UserId: balance.Id, TradeNo: "balance-trade", Status: common.TopUpStatusSuccess,
		PaymentCurrency: "USD", PaymentProvider: PaymentProviderBalance, Money: 5,
		CreateTime: baseTime + 2, CompleteTime: baseTime + 2,
	}).Error)
	require.NoError(t, DB.Create(&TopUp{
		UserId: nonUSD.Id, TradeNo: "eur-trade", Status: common.TopUpStatusSuccess,
		PaymentCurrency: "EUR", PaymentProvider: PaymentProviderStripe, Money: 5,
		CreateTime: baseTime + 3, CompleteTime: baseTime + 3,
	}).Error)
	require.NoError(t, DB.Create(&TopUp{
		UserId: emptyCurrency.Id, TradeNo: "empty-currency-trade", Status: common.TopUpStatusSuccess,
		PaymentCurrency: "", PaymentProvider: PaymentProviderStripe, Money: 5,
		CreateTime: baseTime + 4, CompleteTime: baseTime + 4,
	}).Error)
	require.NoError(t, DB.Create(&TopUp{
		UserId: zero.Id, TradeNo: "zero-trade", Status: common.TopUpStatusSuccess,
		PaymentCurrency: "USD", PaymentProvider: PaymentProviderStripe,
		PaymentAmountMinor: 500, Money: 0,
		CreateTime: baseTime + 5, CompleteTime: baseTime + 5,
	}).Error)

	metrics, err := evaluateInviteBenefitRiskMetricsTx(DB, inviter.Id)
	require.NoError(t, err)
	require.Equal(t, 1, metrics.PaidInviteeCount)
	require.Equal(t, 1, metrics.MinimumAmountInviteeCount)
	require.Equal(t, 10000, metrics.MinimumAmountRatioBPS)
}

func TestEnsureInviteBenefitBlacklistConcurrentIsIdempotent(t *testing.T) {
	setupInviteRewardModelTest(t)
	inviter := createInviteRewardUser(t, "risk_concurrent_inviter", 0)
	createRiskPaidInvitees(t, inviter.Id, 3, 1_787_910_000)

	const workers = 6
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := retrySQLiteBusy(func() error {
				return DB.Transaction(func(tx *gorm.DB) error {
					_, _, err := ensureInviteBenefitBlacklistForInviterTx(tx, inviter.Id)
					return err
				})
			})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var count int64
	require.NoError(t, DB.Model(&InviteBenefitBlacklist{}).Where("inviter_id = ?", inviter.Id).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestEnsureInviteBenefitBlacklistPersistsR1FromAggregatedInviteePayments(t *testing.T) {
	setupInviteRewardModelTest(t)
	inviter := createInviteRewardUser(t, "risk_r1_aggregate_inviter", 0)
	baseTime := int64(1_787_920_000)

	for i := 0; i < 3; i++ {
		invitee := createInviteRewardUser(t, fmt.Sprintf("risk_r1_minimum_%d", i), inviter.Id)
		createSuccessfulRiskTopUp(t, invitee.Id, fmt.Sprintf("risk-r1-minimum-%d", i), baseTime+int64(i))
	}
	splitInvitee := createInviteRewardUser(t, "risk_r1_split", inviter.Id)
	require.NoError(t, DB.Create(&TopUp{
		UserId: splitInvitee.Id, Money: 2, PaymentCurrency: "USD",
		TradeNo: "risk-r1-split-2", PaymentMethod: PaymentMethodStripe,
		PaymentProvider: PaymentProviderStripe, Status: common.TopUpStatusSuccess,
		CreateTime: baseTime + 3, CompleteTime: baseTime + 3,
	}).Error)
	require.NoError(t, DB.Create(&TopUp{
		UserId: splitInvitee.Id, Money: 3, PaymentCurrency: "USD",
		TradeNo: "risk-r1-split-3", PaymentMethod: PaymentMethodStripe,
		PaymentProvider: PaymentProviderStripe, Status: common.TopUpStatusSuccess,
		CreateTime: baseTime + 4, CompleteTime: baseTime + 4,
	}).Error)
	nonMinimum := createInviteRewardUser(t, "risk_r1_ten_dollars", inviter.Id)
	require.NoError(t, DB.Create(&TopUp{
		UserId: nonMinimum.Id, Money: 10, PaymentCurrency: "USD",
		TradeNo: "risk-r1-ten-dollars", PaymentMethod: PaymentMethodStripe,
		PaymentProvider: PaymentProviderStripe, Status: common.TopUpStatusSuccess,
		CreateTime:   baseTime + InviteBenefitRiskR2MaxSpanSeconds + 1,
		CompleteTime: baseTime + InviteBenefitRiskR2MaxSpanSeconds + 1,
	}).Error)

	var blacklist *InviteBenefitBlacklist
	err := DB.Transaction(func(tx *gorm.DB) error {
		var ensureErr error
		blacklist, _, ensureErr = ensureInviteBenefitBlacklistForInviterTx(tx, inviter.Id)
		return ensureErr
	})
	require.NoError(t, err)
	require.NotNil(t, blacklist)
	require.Equal(t, InviteBenefitRiskRuleR1, blacklist.Rule)
	require.Equal(t, InviteBenefitRiskReasonR1, blacklist.Reason)
	require.Equal(t, 5, blacklist.PaidInviteeCount)
	require.Equal(t, 4, blacklist.MinimumAmountInviteeCount)
	require.Equal(t, 8000, blacklist.MinimumAmountRatioBPS)
	require.Greater(t, blacklist.FirstPaymentSpanSeconds, InviteBenefitRiskR2MaxSpanSeconds)

	var persisted InviteBenefitBlacklist
	require.NoError(t, DB.First(&persisted, "inviter_id = ?", inviter.Id).Error)
	require.Equal(t, blacklist.Rule, persisted.Rule)
	require.Equal(t, blacklist.MinimumAmountRatioBPS, persisted.MinimumAmountRatioBPS)
}

func TestOrdinaryTopUpTriggerBlocksBothRewardsWithoutAffectingPurchasedQuota(t *testing.T) {
	setupInviteRewardModelTest(t)
	inviter := createInviteRewardUser(t, "risk_topup_inviter", 0)
	first := createInviteRewardUser(t, "risk_topup_first", inviter.Id)
	second := createInviteRewardUser(t, "risk_topup_second", inviter.Id)
	trigger := createInviteRewardUser(t, "risk_topup_trigger", inviter.Id)
	baseTime := common.GetTimestamp() - 120
	createSuccessfulRiskTopUp(t, first.Id, "risk-topup-first", baseTime)
	createSuccessfulRiskTopUp(t, second.Id, "risk-topup-second", baseTime+30)
	triggerTopUp := &TopUp{
		UserId: trigger.Id, Amount: 2, Money: 5, PaymentCurrency: "USD",
		TradeNo: "risk-topup-trigger", PaymentMethod: PaymentMethodStripe,
		PaymentProvider: PaymentProviderStripe, Status: common.TopUpStatusPending,
		CreateTime: baseTime + 60,
	}
	require.NoError(t, DB.Create(triggerTopUp).Error)

	recharged, err := RechargeWithPaymentSnapshot(triggerTopUp.TradeNo, "cus_risk_trigger", "127.0.0.1", PaymentSnapshot{Money: 5, Currency: "USD"})
	require.NoError(t, err)
	require.True(t, recharged)

	var blacklist InviteBenefitBlacklist
	require.NoError(t, DB.First(&blacklist, "inviter_id = ?", inviter.Id).Error)
	require.Equal(t, InviteBenefitRiskRuleR2, blacklist.Rule)
	var event InviteRewardEvent
	require.NoError(t, DB.First(&event, "invitee_id = ?", trigger.Id).Error)
	require.Equal(t, InviteRewardEventStatusBlocked, event.Status)
	require.Equal(t, InviteRewardBlockReasonBenefitRisk, event.Reason)
	require.Zero(t, event.InviterRewardQuota)
	require.Zero(t, event.InviteeRewardQuota)

	var refreshedTrigger User
	require.NoError(t, DB.First(&refreshedTrigger, trigger.Id).Error)
	require.Equal(t, InviteRewardStatusBlocked, refreshedTrigger.InviteRewardStatus)
	require.Equal(t, int(2*common.QuotaPerUnit), refreshedTrigger.Quota, "purchased quota must still be credited")
	var refreshedInviter User
	require.NoError(t, DB.First(&refreshedInviter, inviter.Id).Error)
	require.Zero(t, refreshedInviter.Quota)
}

func TestSubscriptionTriggerKeepsConsumedInviteeDiscountButBlocksInviterAndFutureDiscount(t *testing.T) {
	setupInviteSubRewardTest(t)
	common.InviteFirstSubDiscountUSD = 5
	inviter := createInviteRewardUser(t, "risk_sub_inviter", 0)
	first := createInviteRewardUser(t, "risk_sub_first", inviter.Id)
	second := createInviteRewardUser(t, "risk_sub_second", inviter.Id)
	trigger := createInviteRewardUser(t, "risk_sub_trigger", inviter.Id)
	baseTime := common.GetTimestamp() - 120
	firstOrder := createRiskSubscriptionOrder(t, first.Id, "risk-sub-first", baseTime, 0)
	secondOrder := createRiskSubscriptionOrder(t, second.Id, "risk-sub-second", baseTime+30, 0)
	triggerOrder := createRiskSubscriptionOrder(t, trigger.Id, "risk-sub-trigger", baseTime+60, 5)
	require.NoError(t, TryGrantInviteSubscriptionRewardAfterOrderCompleted(firstOrder.TradeNo))
	require.NoError(t, TryGrantInviteSubscriptionRewardAfterOrderCompleted(secondOrder.TradeNo))
	var inviterSystemLogsBeforeRisk int64
	require.NoError(t, LOG_DB.Model(&Log{}).
		Where("user_id = ? AND type = ?", inviter.Id, LogTypeSystem).
		Count(&inviterSystemLogsBeforeRisk).Error)
	require.NoError(t, TryGrantInviteSubscriptionRewardAfterOrderCompleted(triggerOrder.TradeNo))

	var triggerReward InviteSubscriptionReward
	require.NoError(t, DB.First(&triggerReward, "invitee_id = ?", trigger.Id).Error)
	require.Equal(t, InviteSubRewardStatusBlocked, triggerReward.Status)
	require.Equal(t, InviteRewardBlockReasonBenefitRisk, triggerReward.Reason)
	require.Zero(t, triggerReward.RewardQuota)
	var inviterSystemLogsAfterRisk int64
	require.NoError(t, LOG_DB.Model(&Log{}).
		Where("user_id = ? AND type = ?", inviter.Id, LogTypeSystem).
		Count(&inviterSystemLogsAfterRisk).Error)
	require.Equal(t, inviterSystemLogsBeforeRisk, inviterSystemLogsAfterRisk, "risk block must not create a user-visible system log")
	var storedTriggerOrder SubscriptionOrder
	require.NoError(t, DB.First(&storedTriggerOrder, triggerOrder.Id).Error)
	require.Equal(t, 5.0, storedTriggerOrder.DiscountUSD, "the already-consumed checkout discount must not be clawed back")

	future := &User{Username: "risk_sub_future", Password: "password123", Role: common.RoleCommonUser, InviterId: inviter.Id, InviteRewardStatus: InviteRewardStatusPending}
	require.NoError(t, DB.Create(future).Error)
	futureOrder := &SubscriptionOrder{
		UserId: future.Id, PlanId: 1, TradeNo: "risk-sub-future",
		PaymentMethod: PaymentMethodStripe, PaymentProvider: PaymentProviderStripe,
		Status: common.TopUpStatusPending, PaymentCurrency: "USD",
	}
	require.NoError(t, CreateSubscriptionOrderWithInviteDiscount(futureOrder, 10, 0))
	require.Zero(t, futureOrder.DiscountUSD)
	require.Equal(t, 10.0, futureOrder.Money)

	// Completing a later purchase still succeeds, while the persistent
	// blacklist continues to block only the inviter benefit.
	paidAt := baseTime + 90
	require.NoError(t, DB.Model(&SubscriptionOrder{}).Where("id = ?", futureOrder.Id).Updates(map[string]interface{}{
		"status":        common.TopUpStatusSuccess,
		"complete_time": paidAt,
	}).Error)
	require.NoError(t, TryGrantInviteSubscriptionRewardAfterOrderCompleted(futureOrder.TradeNo))
	var futureReward InviteSubscriptionReward
	require.NoError(t, DB.First(&futureReward, "invitee_id = ?", future.Id).Error)
	require.Equal(t, InviteSubRewardStatusBlocked, futureReward.Status)
	require.Equal(t, InviteRewardBlockReasonBenefitRisk, futureReward.Reason)
	var inviterSystemLogsAfterFutureRisk int64
	require.NoError(t, LOG_DB.Model(&Log{}).
		Where("user_id = ? AND type = ?", inviter.Id, LogTypeSystem).
		Count(&inviterSystemLogsAfterFutureRisk).Error)
	require.Equal(t, inviterSystemLogsBeforeRisk, inviterSystemLogsAfterFutureRisk, "persistent risk blocks must remain internal-only")
	var storedFutureOrder SubscriptionOrder
	require.NoError(t, DB.First(&storedFutureOrder, futureOrder.Id).Error)
	require.Equal(t, common.TopUpStatusSuccess, storedFutureOrder.Status)
	require.Equal(t, 10.0, storedFutureOrder.Money)
}

func TestSubscriptionDiscountClaimMaterializesBlacklistFromHistoricalPayments(t *testing.T) {
	setupInviteSubRewardTest(t)
	common.InviteFirstSubDiscountUSD = 5
	inviter := createInviteRewardUser(t, "risk_historical_inviter", 0)
	baseTime := common.GetTimestamp() - 3600
	createRiskPaidInvitees(t, inviter.Id, 3, baseTime)

	var before int64
	require.NoError(t, DB.Model(&InviteBenefitBlacklist{}).Where("inviter_id = ?", inviter.Id).Count(&before).Error)
	require.Zero(t, before, "historical payments alone must not require a pre-created blacklist")

	fourth := createInviteRewardUser(t, "risk_historical_fourth", inviter.Id)
	order := &SubscriptionOrder{
		UserId: fourth.Id, PlanId: 1, TradeNo: "risk-historical-fourth",
		PaymentMethod: PaymentMethodStripe, PaymentProvider: PaymentProviderStripe,
		Status: common.TopUpStatusPending, PaymentCurrency: "USD",
	}
	require.NoError(t, CreateSubscriptionOrderWithInviteDiscount(order, 10, 0))
	require.Zero(t, order.DiscountUSD)
	require.Equal(t, 10.0, order.Money)
	require.Equal(t, common.TopUpStatusPending, order.Status, "no fourth successful payment is needed to materialize the decision")

	var blacklist InviteBenefitBlacklist
	require.NoError(t, DB.First(&blacklist, "inviter_id = ?", inviter.Id).Error)
	require.Equal(t, InviteBenefitRiskRuleR2, blacklist.Rule)
	require.Equal(t, InviteBenefitRiskReasonR2, blacklist.Reason)
	require.Equal(t, 3, blacklist.PaidInviteeCount)
}

func createRiskPaidInvitees(t *testing.T, inviterId int, count int, baseTime int64) []*User {
	t.Helper()
	users := make([]*User, 0, count)
	for i := 0; i < count; i++ {
		user := createInviteRewardUser(t, fmt.Sprintf("risk_paid_%d_%d", inviterId, i), inviterId)
		createSuccessfulRiskTopUp(t, user.Id, fmt.Sprintf("risk-paid-%d-%d", inviterId, i), baseTime+int64(i))
		users = append(users, user)
	}
	return users
}

func createSuccessfulRiskTopUp(t *testing.T, userId int, tradeNo string, paidAt int64) *TopUp {
	t.Helper()
	topUp := &TopUp{
		UserId: userId, Money: 5, PaymentCurrency: "USD", PaymentAmountMinor: 500,
		TradeNo: tradeNo, PaymentMethod: PaymentMethodStripe, PaymentProvider: PaymentProviderStripe,
		Status: common.TopUpStatusSuccess, CreateTime: paidAt, CompleteTime: paidAt,
	}
	require.NoError(t, DB.Create(topUp).Error)
	return topUp
}

func createRiskSubscriptionOrder(t *testing.T, userId int, tradeNo string, paidAt int64, discountUSD float64) *SubscriptionOrder {
	t.Helper()
	order := &SubscriptionOrder{
		UserId: userId, PlanId: 1, Money: 5, DiscountUSD: discountUSD,
		TradeNo: tradeNo, PaymentMethod: PaymentMethodStripe, PaymentProvider: PaymentProviderStripe,
		Status: common.TopUpStatusSuccess, PaymentCurrency: "USD", PaymentAmountMinor: 500,
		CreateTime: paidAt, CompleteTime: paidAt,
	}
	require.NoError(t, order.Insert())
	return order
}
