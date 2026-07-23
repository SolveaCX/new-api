package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const subscriptionWalletRenewalLeadTime = 1 * time.Minute

type WalletSubscriptionRenewalResult struct {
	ContractID    int64
	Renewed       bool
	PausedStatus  string
	ChargedQuota  int64
	EntitlementID int
	OrderID       int
}

func RunWalletSubscriptionRenewalOnce(limit int) (int, error) {
	if limit <= 0 {
		limit = subscriptionResetBatchSize
	}
	now := common.GetTimestamp()
	dueBefore := now + int64(subscriptionWalletRenewalLeadTime.Seconds())
	var contracts []model.UserSubscriptionContract
	if err := model.DB.
		Where("status = ? AND renewal_source = ? AND renewal_status = ? AND current_period_end > ? AND current_period_end <= ?",
			model.SubscriptionContractStatusActive,
			model.SubscriptionRenewalSourceWallet,
			model.SubscriptionRenewalStatusEnabled,
			now,
			dueBefore).
		Order("current_period_end asc, id asc").
		Limit(limit).
		Find(&contracts).Error; err != nil {
		return 0, err
	}
	renewed := 0
	for _, contract := range contracts {
		result, err := RenewWalletSubscriptionContract(contract.Id)
		if err != nil {
			return renewed, err
		}
		if result.Renewed {
			renewed++
		}
	}
	return renewed, nil
}

func RenewWalletSubscriptionContract(contractID int64) (*WalletSubscriptionRenewalResult, error) {
	if contractID <= 0 {
		return nil, errors.New("invalid contract id")
	}
	var result *WalletSubscriptionRenewalResult
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var contract model.UserSubscriptionContract
		if err := subscriptionCommandLock(tx).Where("id = ?", contractID).First(&contract).Error; err != nil {
			return err
		}
		result = &WalletSubscriptionRenewalResult{ContractID: contract.Id}
		if !walletContractIsRenewable(contract) {
			return nil
		}
		plan, err := loadEnabledSubscriptionPlanTx(tx, contract.CurrentPlanId)
		if err != nil {
			return pauseWalletRenewalTx(tx, &contract, model.SubscriptionRenewalStatusPausedPlanUnavailable, result)
		}
		if err := validateFlexiblePrepaidPlan(plan); err != nil {
			return pauseWalletRenewalTx(tx, &contract, model.SubscriptionRenewalStatusPausedPlanUnavailable, result)
		}
		if plan.AllowBalancePay != nil && !*plan.AllowBalancePay {
			return pauseWalletRenewalTx(tx, &contract, model.SubscriptionRenewalStatusPausedPlanUnavailable, result)
		}
		var user model.User
		if err := subscriptionCommandLock(tx).Where("id = ?", contract.UserId).First(&user).Error; err != nil {
			return err
		}
		requiredQuota, err := subscriptionBalanceQuota(plan.PriceAmount)
		if err != nil {
			return err
		}
		if requiredQuota > 0 && user.Quota < requiredQuota {
			return pauseWalletRenewalTx(tx, &contract, model.SubscriptionRenewalStatusPausedInsufficientBalance, result)
		}

		periodStart := contract.CurrentPeriodEnd
		periodEnd := time.Unix(periodStart, 0).AddDate(0, 1, 0).Unix()
		renewalKey := walletRenewalKey(contract.Id, periodStart, plan.Id)
		tradeNo := walletRenewalTradeNo(contract.Id, periodStart, plan.Id)
		now := common.GetTimestamp()
		order := &model.SubscriptionOrder{
			UserId:          user.Id,
			PlanId:          plan.Id,
			Money:           plan.PriceAmount,
			TradeNo:         tradeNo,
			PaymentMethod:   model.PaymentMethodBalance,
			PaymentProvider: model.PaymentProviderBalance,
			Status:          common.TopUpStatusSuccess,
			CreateTime:      now,
			CompleteTime:    now,
			PurchaseMonths:  1,
			UnitPrice:       plan.PriceAmount,
			PaymentCurrency: plan.Currency,
			ProviderPayload: fmt.Sprintf("charged_quota=%d;contract_id=%d;renewal_key=%s", requiredQuota, contract.Id, renewalKey),
			RenewalSource:   model.SubscriptionRenewalSourceWallet,
		}
		if err := tx.Create(order).Error; err != nil {
			return handleExistingWalletRenewalTx(tx, &contract, renewalKey, result, err)
		}
		if requiredQuota > 0 {
			if err := tx.Model(&model.User{}).Where("id = ?", user.Id).
				Update("quota", gorm.Expr("quota - ?", requiredQuota)).Error; err != nil {
				return err
			}
			if err := tx.Create(&model.WalletLedgerEntry{
				UserId:      user.Id,
				EntryKey:    renewalKey,
				QuotaDelta:  -int64(requiredQuota),
				MoneyAmount: plan.PriceAmount,
				EntryType:   model.WalletLedgerEntryTypePrepaidDebit,
				OrderId:     order.Id,
			}).Error; err != nil {
				return handleExistingWalletRenewalTx(tx, &contract, renewalKey, result, err)
			}
		}
		grant, err := model.RotateCurrentEntitlementTx(tx, model.GrantEntitlementInput{
			ContractId:           contract.Id,
			UserId:               user.Id,
			PlanId:               plan.Id,
			ProviderBindingId:    0,
			GrantKey:             renewalKey,
			PaymentMode:          model.SubscriptionPaymentModePrepaid,
			AmountTotal:          plan.TotalAmount,
			MediaCreditsTotal:    plan.MediaCreditsMonthly,
			Window5hAmount:       common.GetPointer(plan.Window5hAmount),
			WindowWeekAmount:     common.GetPointer(plan.WindowWeekAmount),
			UpgradeGroup:         common.GetPointer(plan.UpgradeGroup),
			PeriodStart:          periodStart,
			PeriodEnd:            periodEnd,
			EndReasonForPrevious: model.SubscriptionEntitlementEndReasonRenewed,
			Source:               model.PaymentMethodBalance,
		})
		if err != nil {
			return handleExistingWalletRenewalTx(tx, &contract, renewalKey, result, err)
		}
		if err := createPrepaidTermSegmentsTx(tx, contract.Id, order.Id, plan.Id, PrepaidTermAllocation{
			CanonicalWalletUnitPrice: plan.PriceAmount,
		}, periodStart, 1); err != nil {
			return err
		}
		if err := tx.Model(&model.UserSubscriptionContract{}).Where("id = ?", contract.Id).Updates(map[string]interface{}{
			"renewal_status":       model.SubscriptionRenewalStatusEnabled,
			"current_period_start": periodStart,
			"current_period_end":   periodEnd,
			"updated_at":           now,
		}).Error; err != nil {
			return err
		}
		result.Renewed = true
		result.ChargedQuota = int64(requiredQuota)
		result.OrderID = order.Id
		if grant != nil && grant.Entitlement != nil {
			result.EntitlementID = grant.Entitlement.Id
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func walletContractIsRenewable(contract model.UserSubscriptionContract) bool {
	return contract.Status == model.SubscriptionContractStatusActive &&
		contract.RenewalSource == model.SubscriptionRenewalSourceWallet &&
		contract.RenewalStatus == model.SubscriptionRenewalStatusEnabled &&
		contract.CurrentPlanId > 0 &&
		contract.CurrentPeriodEnd > 0 &&
		contract.CurrentPeriodEnd <= common.GetTimestamp()+int64(subscriptionWalletRenewalLeadTime.Seconds())
}

func pauseWalletRenewalTx(tx *gorm.DB, contract *model.UserSubscriptionContract, status string, result *WalletSubscriptionRenewalResult) error {
	if tx == nil || contract == nil {
		return errors.New("subscription renewal facts are incomplete")
	}
	if err := tx.Model(&model.UserSubscriptionContract{}).Where("id = ?", contract.Id).Updates(map[string]interface{}{
		"renewal_status": status,
		"updated_at":     common.GetTimestamp(),
	}).Error; err != nil {
		return err
	}
	if result != nil {
		result.PausedStatus = status
	}
	return nil
}

func handleExistingWalletRenewalTx(tx *gorm.DB, contract *model.UserSubscriptionContract, renewalKey string, result *WalletSubscriptionRenewalResult, originalErr error) error {
	var entitlement model.UserSubscription
	query := tx.Where("grant_key = ?", renewalKey).Limit(1).Find(&entitlement)
	if query.Error != nil {
		return query.Error
	}
	if query.RowsAffected == 0 {
		return originalErr
	}
	if result != nil {
		result.EntitlementID = entitlement.Id
	}
	if contract == nil || contract.CurrentPeriodEnd >= entitlement.EndTime {
		return nil
	}
	return tx.Model(&model.UserSubscriptionContract{}).Where("id = ?", contract.Id).Updates(map[string]interface{}{
		"current_entitlement_id": entitlement.Id,
		"current_period_start":   entitlement.StartTime,
		"current_period_end":     entitlement.EndTime,
		"updated_at":             common.GetTimestamp(),
	}).Error
}

func walletRenewalKey(contractID int64, periodStart int64, planID int) string {
	return fmt.Sprintf("subscription:renewal:debit:contract:%d:period:%d:plan:%d", contractID, periodStart, planID)
}

func walletRenewalTradeNo(contractID int64, periodStart int64, planID int) string {
	return fmt.Sprintf("SUBRENEWCON%dPER%dPLAN%d", contractID, periodStart, planID)
}
