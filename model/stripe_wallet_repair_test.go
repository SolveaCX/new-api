package model

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupStripeWalletRepairTestDB(t *testing.T, maxOpenConns int) {
	t.Helper()
	setupTopUpLifecycleTestDB(t, maxOpenConns)
	require.NoError(t, DB.AutoMigrate(&StripeWalletPaymentCheck{}, &PaymentInvoice{}, &StripeCheckoutRevision{}, &SubscriptionOrder{}))
	originalQuotaPerUnit := common.QuotaPerUnit
	originalHook := PaymentSuccessHook
	originalLimits := operation_setting.GetPaymentSetting().AmountBonusLimit
	common.QuotaPerUnit = 100
	PaymentSuccessHook = nil
	operation_setting.GetPaymentSetting().AmountBonusLimit = map[int]int{}
	t.Cleanup(func() {
		common.QuotaPerUnit = originalQuotaPerUnit
		PaymentSuccessHook = originalHook
		operation_setting.GetPaymentSetting().AmountBonusLimit = originalLimits
	})
}

func seedStripeWalletRepair(t *testing.T, status string) (*User, *TopUp, *StripeWalletPaymentCheck, StripeWalletRepairPayment) {
	t.Helper()
	user := createLifecycleQuotaTestUser(t, "repair-"+status, 0, 100)
	topUp := &TopUp{
		UserId: user.Id, Amount: 2, BonusAmount: 1, BonusTier: 2, Money: 2,
		TradeNo: "trade-" + status, GatewayTradeNo: "cs_" + status,
		PaymentPriceId: "price_wallet", PaymentMethod: PaymentMethodStripe,
		PaymentProvider: PaymentProviderStripe, CreateTime: 1_700_000_000, Status: status,
	}
	require.NoError(t, DB.Create(topUp).Error)
	now, err := GetDBTimestampWithContext(context.Background())
	require.NoError(t, err)
	check := &StripeWalletPaymentCheck{
		Scope: "stripe:acct_test:test", SessionID: topUp.GatewayTradeNo, TradeNo: topUp.TradeNo,
		PriceID: topUp.PaymentPriceId, UserID: user.Id, Amount: 175, Currency: "USD",
		WalletVerified: true, SessionVerified: true, NextCheckAt: now,
		LeaseToken: "repair-worker", LeaseUntil: now + 120,
	}
	require.NoError(t, DB.Create(check).Error)
	proof := StripeWalletRepairPayment{
		UserID: user.Id, SessionID: topUp.GatewayTradeNo, TradeNo: topUp.TradeNo,
		PriceID: topUp.PaymentPriceId, CustomerID: "cus_repair", AmountMinor: 175,
		Currency: "usd", Snapshot: PaymentSnapshot{Money: 1.75, Currency: "usd"},
	}
	return &user, topUp, check, proof
}

func TestRechargeStripeWalletForReconciliationAtomicallyRepairsOrderInvoiceAndAudit(t *testing.T) {
	setupStripeWalletRepairTestDB(t, 1)
	user, topUp, check, proof := seedStripeWalletRepair(t, common.TopUpStatusFailed)
	require.NoError(t, DB.Create(&PaymentInvoice{
		TradeNo: topUp.TradeNo, UserId: user.Id, OrderType: PaymentOrderTypeTopUp,
		PaymentProvider: PaymentProviderStripe, InvoiceStatus: PaymentInvoiceStatusFailed,
	}).Error)

	credited, err := RechargeStripeWalletForReconciliation(context.Background(), check, proof)
	require.NoError(t, err)
	require.True(t, credited)
	require.Positive(t, check.AutoRepairedAt)
	require.Equal(t, common.TopUpStatusFailed, check.RepairFromStatus)
	require.EqualValues(t, 300, check.RepairCredit)

	var storedTopUp TopUp
	require.NoError(t, DB.First(&storedTopUp, topUp.Id).Error)
	require.Equal(t, common.TopUpStatusSuccess, storedTopUp.Status)
	require.Equal(t, 1.75, storedTopUp.Money)
	require.Equal(t, "USD", storedTopUp.PaymentCurrency)
	require.Equal(t, "cus_repair", stripeCustomerForRepairTest(t, user.Id))
	require.Equal(t, 300, walletQuotaForTest(t, user.Id))

	var storedCheck StripeWalletPaymentCheck
	require.NoError(t, DB.First(&storedCheck, check.Id).Error)
	require.Equal(t, check.AutoRepairedAt, storedCheck.AutoRepairedAt)
	require.EqualValues(t, 300, storedCheck.RepairCredit)
	var invoice PaymentInvoice
	require.NoError(t, DB.Where("trade_no = ?", topUp.TradeNo).First(&invoice).Error)
	require.Equal(t, PaymentInvoiceStatusPaid, invoice.InvoiceStatus)
	require.Equal(t, proof.SessionID, invoice.StripeCheckoutSessionId)
	require.Equal(t, proof.CustomerID, invoice.StripeCustomerId)
	requireTopUpLifecycleEventCount(t, topUp.TradeNo, RecallLifecycleTriggerPaymentSucceeded, 1)

	stale := storedCheck
	stale.AutoRepairedAt = 0
	stale.RepairFromStatus = ""
	stale.RepairCredit = 0
	stale.RepairError = "notification failed"
	require.NoError(t, CheckpointStripeWalletPaymentCheck(context.Background(), &stale, stale.LeaseToken))
	require.NoError(t, DB.First(&storedCheck, check.Id).Error)
	require.Equal(t, check.AutoRepairedAt, storedCheck.AutoRepairedAt)
	require.EqualValues(t, 300, storedCheck.RepairCredit)
	require.Equal(t, "notification failed", storedCheck.RepairError)
}

func TestRechargeStripeWalletForReconciliationSupportsRepairableStatusesAndReplay(t *testing.T) {
	for _, status := range []string{common.TopUpStatusPending, common.TopUpStatusFailed, common.TopUpStatusExpired, "cancelled", "canceled"} {
		t.Run(status, func(t *testing.T) {
			setupStripeWalletRepairTestDB(t, 1)
			user, topUp, check, proof := seedStripeWalletRepair(t, status)
			credited, err := RechargeStripeWalletForReconciliation(context.Background(), check, proof)
			require.NoError(t, err)
			require.True(t, credited)
			credited, err = RechargeStripeWalletForReconciliation(context.Background(), check, proof)
			require.NoError(t, err)
			require.False(t, credited)
			require.Equal(t, 300, walletQuotaForTest(t, user.Id))
			requireTopUpLifecycleEventCount(t, topUp.TradeNo, RecallLifecycleTriggerPaymentSucceeded, 1)
			var bonusCount int64
			require.NoError(t, DB.Model(&TopUpBonusClaim{}).Where("trade_no = ?", topUp.TradeNo).Count(&bonusCount).Error)
			require.EqualValues(t, 1, bonusCount)
		})
	}
}

func TestRechargeStripeWalletForReconciliationRejectsIdentityRevisionAndLeaseMismatch(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*TopUp, *StripeWalletPaymentCheck, *StripeWalletRepairPayment)
	}{
		{"user", func(_ *TopUp, _ *StripeWalletPaymentCheck, p *StripeWalletRepairPayment) { p.UserID++ }},
		{"session", func(_ *TopUp, _ *StripeWalletPaymentCheck, p *StripeWalletRepairPayment) { p.SessionID = "cs_other" }},
		{"price", func(_ *TopUp, _ *StripeWalletPaymentCheck, p *StripeWalletRepairPayment) { p.PriceID = "price_other" }},
		{"missing_customer", func(_ *TopUp, _ *StripeWalletPaymentCheck, p *StripeWalletRepairPayment) { p.CustomerID = " " }},
		{"amount", func(_ *TopUp, _ *StripeWalletPaymentCheck, p *StripeWalletRepairPayment) { p.AmountMinor++ }},
		{"currency", func(_ *TopUp, _ *StripeWalletPaymentCheck, p *StripeWalletRepairPayment) {
			p.Currency = "eur"
			p.Snapshot.Currency = "eur"
		}},
		{"revision", func(_ *TopUp, _ *StripeWalletPaymentCheck, p *StripeWalletRepairPayment) { p.CheckoutRevision = 2 }},
		{"legacy_discount", func(_ *TopUp, _ *StripeWalletPaymentCheck, p *StripeWalletRepairPayment) {
			p.DiscountSelection = "invitation"
		}},
		{"blank_gateway", func(tu *TopUp, _ *StripeWalletPaymentCheck, _ *StripeWalletRepairPayment) {
			require.NoError(t, DB.Model(tu).Update("gateway_trade_no", "").Error)
		}},
		{"blank_price", func(tu *TopUp, _ *StripeWalletPaymentCheck, _ *StripeWalletRepairPayment) {
			require.NoError(t, DB.Model(tu).Update("payment_price_id", "").Error)
		}},
		{"expired_lease", func(_ *TopUp, c *StripeWalletPaymentCheck, _ *StripeWalletRepairPayment) {
			require.NoError(t, DB.Model(c).Update("lease_until", 1).Error)
		}},
		{"stale_token", func(_ *TopUp, c *StripeWalletPaymentCheck, _ *StripeWalletRepairPayment) {
			require.NoError(t, DB.Model(&StripeWalletPaymentCheck{}).Where("id = ?", c.Id).Update("lease_token", "other").Error)
		}},
		{"closed", func(_ *TopUp, c *StripeWalletPaymentCheck, _ *StripeWalletRepairPayment) {
			require.NoError(t, DB.Model(&StripeWalletPaymentCheck{}).Where("id = ?", c.Id).Update("closed_at", 1).Error)
		}},
		{"verification_error", func(_ *TopUp, c *StripeWalletPaymentCheck, _ *StripeWalletRepairPayment) {
			require.NoError(t, DB.Model(&StripeWalletPaymentCheck{}).Where("id = ?", c.Id).Update("verification_error", "bad proof").Error)
		}},
		{"customer_conflict", func(_ *TopUp, _ *StripeWalletPaymentCheck, p *StripeWalletRepairPayment) {
			require.NoError(t, DB.Model(&User{}).Where("id = ?", p.UserID).Update("stripe_customer", "cus_other").Error)
		}},
		{"subscription_trade", func(_ *TopUp, _ *StripeWalletPaymentCheck, p *StripeWalletRepairPayment) {
			require.NoError(t, DB.Create(&SubscriptionOrder{UserId: p.UserID, TradeNo: p.TradeNo, PaymentProvider: PaymentProviderStripe, Status: common.TopUpStatusPending}).Error)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupStripeWalletRepairTestDB(t, 1)
			user, topUp, check, proof := seedStripeWalletRepair(t, common.TopUpStatusFailed)
			tc.mutate(topUp, check, &proof)
			credited, err := RechargeStripeWalletForReconciliation(context.Background(), check, proof)
			require.False(t, credited)
			require.Error(t, err)
			require.Equal(t, 0, walletQuotaForTest(t, user.Id))
			var stored TopUp
			require.NoError(t, DB.First(&stored, topUp.Id).Error)
			require.Equal(t, common.TopUpStatusFailed, stored.Status)
		})
	}
}

func TestRechargeStripeWalletForReconciliationValidatesRevisionLedger(t *testing.T) {
	setupStripeWalletRepairTestDB(t, 1)
	user, topUp, check, proof := seedStripeWalletRepair(t, common.TopUpStatusExpired)
	topUp.CheckoutRevision = 2
	require.NoError(t, DB.Model(topUp).Update("checkout_revision", 2).Error)
	proof.CheckoutRevision = 2
	proof.DiscountSelection = "invitation"
	sessionID := proof.SessionID
	require.NoError(t, DB.Create(&StripeCheckoutRevision{
		OrderType: StripeCheckoutOrderTopUp, TradeNo: proof.TradeNo, Revision: 2,
		UserId: user.Id, RequestId: "req-repair", SelectionDigest: "digest", State: StripeCheckoutRevisionStateActive,
		DiscountSource: "invitation", ProviderSessionId: &sessionID,
	}).Error)

	credited, err := RechargeStripeWalletForReconciliation(context.Background(), check, proof)
	require.NoError(t, err)
	require.True(t, credited)
}

func TestRechargeStripeWalletForReconciliationAcceptsUnversionedCheckoutMetadata(t *testing.T) {
	for _, selection := range []string{"", "none"} {
		t.Run("selection_"+selection, func(t *testing.T) {
			setupStripeWalletRepairTestDB(t, 1)
			user, _, check, proof := seedStripeWalletRepair(t, common.TopUpStatusFailed)
			proof.DiscountSelection = selection
			credited, err := RechargeStripeWalletForReconciliation(context.Background(), check, proof)
			require.NoError(t, err)
			require.True(t, credited)
			require.Equal(t, 300, walletQuotaForTest(t, user.Id))
		})
	}
}

func TestRechargeStripeWalletForReconciliationRollsBackInvoiceAndAuditFailures(t *testing.T) {
	for _, target := range []string{"invoice", "audit"} {
		t.Run(target, func(t *testing.T) {
			setupStripeWalletRepairTestDB(t, 1)
			user, topUp, check, proof := seedStripeWalletRepair(t, common.TopUpStatusFailed)
			if target == "invoice" {
				require.NoError(t, DB.Create(&PaymentInvoice{TradeNo: topUp.TradeNo, UserId: user.Id, OrderType: PaymentOrderTypeTopUp, PaymentProvider: PaymentProviderStripe, InvoiceStatus: PaymentInvoiceStatusFailed}).Error)
				require.NoError(t, DB.Exec(`CREATE TRIGGER reject_repair_invoice BEFORE UPDATE ON payment_invoices BEGIN SELECT RAISE(ABORT, 'invoice repair rejected'); END`).Error)
			} else {
				require.NoError(t, DB.Exec(`CREATE TRIGGER reject_repair_audit BEFORE UPDATE ON stripe_wallet_payment_checks WHEN NEW.auto_repaired_at > 0 BEGIN SELECT RAISE(ABORT, 'audit repair rejected'); END`).Error)
			}
			credited, err := RechargeStripeWalletForReconciliation(context.Background(), check, proof)
			require.False(t, credited)
			require.Error(t, err)
			require.Equal(t, 0, walletQuotaForTest(t, user.Id))
			var stored TopUp
			require.NoError(t, DB.First(&stored, topUp.Id).Error)
			require.Equal(t, common.TopUpStatusFailed, stored.Status)
			var storedCheck StripeWalletPaymentCheck
			require.NoError(t, DB.First(&storedCheck, check.Id).Error)
			require.Zero(t, storedCheck.AutoRepairedAt)
			requireTopUpLifecycleEventCount(t, topUp.TradeNo, RecallLifecycleTriggerPaymentSucceeded, 0)
		})
	}
}

func TestRechargeStripeWalletForReconciliationRejectsInactiveRevision(t *testing.T) {
	for _, state := range []string{StripeCheckoutRevisionStatePreparing, StripeCheckoutRevisionStateAbandoned, StripeCheckoutRevisionStateSuperseded} {
		t.Run(state, func(t *testing.T) {
			setupStripeWalletRepairTestDB(t, 1)
			user, topUp, check, proof := seedStripeWalletRepair(t, common.TopUpStatusFailed)
			require.NoError(t, DB.Model(topUp).Update("checkout_revision", 2).Error)
			proof.CheckoutRevision, proof.DiscountSelection = 2, "none"
			require.NoError(t, DB.Create(&StripeCheckoutRevision{
				OrderType: StripeCheckoutOrderTopUp, TradeNo: proof.TradeNo, Revision: 2,
				UserId: user.Id, RequestId: "req-inactive", SelectionDigest: "digest", State: state,
				DiscountSource: "none", ProviderSessionId: &proof.SessionID,
			}).Error)
			credited, err := RechargeStripeWalletForReconciliation(context.Background(), check, proof)
			require.ErrorIs(t, err, ErrStripeWalletRepairRejected)
			require.False(t, credited)
			require.Equal(t, 0, walletQuotaForTest(t, user.Id))
			var stored TopUp
			require.NoError(t, DB.First(&stored, topUp.Id).Error)
			require.Equal(t, common.TopUpStatusFailed, stored.Status)
		})
	}
}

func TestRechargeStripeWalletForReconciliationRollsBackWhenLeaseExpiresDuringTransaction(t *testing.T) {
	setupStripeWalletRepairTestDB(t, 1)
	user, topUp, check, proof := seedStripeWalletRepair(t, common.TopUpStatusFailed)
	now, err := GetDBTimestampWithContext(context.Background())
	require.NoError(t, err)
	require.NoError(t, DB.Model(check).Update("lease_until", now+2).Error)
	const callbackName = "test:expire_wallet_repair_lease"
	waited := false
	require.NoError(t, DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if !waited && tx.Statement.Table == "stripe_wallet_payment_checks" {
			waited = true
			time.Sleep(2100 * time.Millisecond)
		}
	}))
	t.Cleanup(func() { require.NoError(t, DB.Callback().Update().Remove(callbackName)) })

	credited, err := RechargeStripeWalletForReconciliation(context.Background(), check, proof)
	require.True(t, waited, "must pass initial lease validation before expiring at the audit write")
	require.ErrorIs(t, err, ErrStripeWalletReconciliationLeaseLost)
	require.False(t, credited)
	require.Equal(t, 0, walletQuotaForTest(t, user.Id))
	var stored TopUp
	require.NoError(t, DB.First(&stored, topUp.Id).Error)
	require.Equal(t, common.TopUpStatusFailed, stored.Status)
	var audit StripeWalletPaymentCheck
	require.NoError(t, DB.First(&audit, check.Id).Error)
	require.Zero(t, audit.AutoRepairedAt)
	var bonusCount int64
	require.NoError(t, DB.Model(&TopUpBonusClaim{}).Where("trade_no = ?", topUp.TradeNo).Count(&bonusCount).Error)
	require.Zero(t, bonusCount)
	requireTopUpLifecycleEventCount(t, topUp.TradeNo, RecallLifecycleTriggerPaymentSucceeded, 0)
}

func TestRechargeStripeWalletForReconciliationFailsClosedForDeletedUserAndPriorCredit(t *testing.T) {
	t.Run("deleted_user", func(t *testing.T) {
		setupStripeWalletRepairTestDB(t, 1)
		user, topUp, check, proof := seedStripeWalletRepair(t, common.TopUpStatusFailed)
		require.NoError(t, DB.Unscoped().Delete(&User{}, user.Id).Error)
		credited, err := RechargeStripeWalletForReconciliation(context.Background(), check, proof)
		require.False(t, credited)
		require.Error(t, err)
		var stored TopUp
		require.NoError(t, DB.First(&stored, topUp.Id).Error)
		require.Equal(t, common.TopUpStatusFailed, stored.Status)
	})

	t.Run("already_credited_corrupt_status", func(t *testing.T) {
		setupStripeWalletRepairTestDB(t, 1)
		user, topUp, check, proof := seedStripeWalletRepair(t, common.TopUpStatusPending)
		credited, err := RechargeWithPaymentSnapshot(topUp.TradeNo, proof.CustomerID, "", proof.Snapshot)
		require.NoError(t, err)
		require.True(t, credited)
		require.NoError(t, DB.Model(topUp).Updates(map[string]any{"status": common.TopUpStatusFailed, "complete_time": 0}).Error)
		credited, err = RechargeStripeWalletForReconciliation(context.Background(), check, proof)
		require.False(t, credited)
		require.ErrorIs(t, err, ErrStripeWalletRepairRejected)
		require.Equal(t, 300, walletQuotaForTest(t, user.Id))
		requireTopUpLifecycleEventCount(t, topUp.TradeNo, RecallLifecycleTriggerPaymentSucceeded, 1)
	})
}

func TestRechargeStripeWalletForReconciliationConcurrentWebhookCreditsOnce(t *testing.T) {
	setupStripeWalletRepairTestDB(t, 4)
	user, topUp, check, proof := seedStripeWalletRepair(t, common.TopUpStatusFailed)
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, err := RechargeWithPaymentSnapshot(topUp.TradeNo, proof.CustomerID, "", proof.Snapshot)
		errs <- err
	}()
	go func() {
		defer wg.Done()
		<-start
		_, err := RechargeStripeWalletForReconciliation(context.Background(), check, proof)
		errs <- err
	}()
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, 300, walletQuotaForTest(t, user.Id))
	requireTopUpLifecycleEventCount(t, topUp.TradeNo, RecallLifecycleTriggerPaymentSucceeded, 1)
	var bonusCount int64
	require.NoError(t, DB.Model(&TopUpBonusClaim{}).Where("trade_no = ?", topUp.TradeNo).Count(&bonusCount).Error)
	require.EqualValues(t, 1, bonusCount)
	var stateCount int64
	require.NoError(t, DB.Model(&QuotaLifecycleState{}).Where("user_id = ? AND scope_type = ?", user.Id, QuotaLifecycleScopeWallet).Count(&stateCount).Error)
	require.EqualValues(t, 1, stateCount)
}

func stripeCustomerForRepairTest(t *testing.T, userID int) string {
	t.Helper()
	var user User
	require.NoError(t, DB.Select("stripe_customer").First(&user, userID).Error)
	return user.StripeCustomer
}
