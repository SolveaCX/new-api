package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

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
	var contracts []model.UserSubscriptionContract
	if err := model.DB.
		Where("status = ? AND renewal_source = ? AND renewal_status = ? AND current_period_end > ? AND current_period_end <= ?",
			model.SubscriptionContractStatusActive,
			model.SubscriptionRenewalSourceWallet,
			model.SubscriptionRenewalStatusEnabled,
			0,
			now).
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
	invalidateUserID := 0
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
		planSnapshot, err := subscriptionPurchasePlanSnapshot(plan)
		if err != nil {
			return err
		}
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
			PlanSnapshot:    planSnapshot,
			ProviderPayload: fmt.Sprintf("charged_quota=%d;contract_id=%d;renewal_key=%s", requiredQuota, contract.Id, renewalKey),
			RenewalSource:   model.SubscriptionRenewalSourceWallet,
		}
		if err := tx.Create(order).Error; err != nil {
			return handleExistingWalletRenewalTx(tx, &contract, renewalKey, result, err)
		}
		if requiredQuota > 0 {
			res := tx.Model(&model.User{}).Where("id = ? AND quota >= ?", user.Id, requiredQuota).
				Update("quota", gorm.Expr("quota - ?", requiredQuota))
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return pauseWalletRenewalTx(tx, &contract, model.SubscriptionRenewalStatusPausedInsufficientBalance, result)
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
		invalidateUserID = user.Id
		return nil
	})
	if err != nil {
		return nil, err
	}
	if invalidateUserID > 0 {
		if err := model.InvalidateUserCache(invalidateUserID); err != nil {
			common.SysLog("failed to invalidate user cache after wallet renewal: " + err.Error())
		}
	}
	return result, nil
}

func walletContractIsRenewable(contract model.UserSubscriptionContract) bool {
	return contract.Status == model.SubscriptionContractStatusActive &&
		contract.RenewalSource == model.SubscriptionRenewalSourceWallet &&
		contract.RenewalStatus == model.SubscriptionRenewalStatusEnabled &&
		contract.CurrentPlanId > 0 &&
		contract.CurrentPeriodEnd > 0 &&
		contract.CurrentPeriodEnd <= common.GetTimestamp()
}

func pauseWalletRenewalTx(tx *gorm.DB, contract *model.UserSubscriptionContract, status string, result *WalletSubscriptionRenewalResult) error {
	if tx == nil || contract == nil {
		return errors.New("subscription renewal facts are incomplete")
	}
	now := common.GetTimestamp()
	if contract.CurrentEntitlementId > 0 {
		if err := tx.Model(&model.UserSubscription{}).
			Where("id = ? AND contract_id = ? AND status = ?", contract.CurrentEntitlementId, contract.Id, model.SubscriptionEntitlementStatusActive).
			Updates(map[string]interface{}{
				"status":     model.SubscriptionEntitlementStatusHistorical,
				"end_reason": model.SubscriptionEntitlementEndReasonExpired,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}
	}
	if err := tx.Model(&model.UserSubscriptionContract{}).Where("id = ?", contract.Id).Updates(map[string]interface{}{
		"status":         model.SubscriptionContractStatusEnded,
		"renewal_status": status,
		"updated_at":     now,
	}).Error; err != nil {
		return err
	}
	if result != nil {
		result.PausedStatus = status
	}
	return nil
}

func handleExistingWalletRenewalTx(tx *gorm.DB, contract *model.UserSubscriptionContract, renewalKey string, result *WalletSubscriptionRenewalResult, originalErr error) error {
	if common.UsingPostgreSQL && isWalletRenewalDuplicateError(originalErr) {
		return originalErr
	}
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

func isWalletRenewalDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062
	}
	var sqliteErr interface{ Code() int }
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code()&0xff == 19
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "constraint failed: unique")
}
