package service

import (
	"database/sql"
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

type walletRenewalAttempt struct {
	ContractID      int64
	UserID          int
	PlanID          int
	PeriodStart     int64
	PeriodEnd       int64
	RenewalKey      string
	TradeNo         string
	RequiredQuota   int
	PriceAmount     float64
	PaymentCurrency string
	PlanSnapshot    string
	GrantInput      model.GrantEntitlementInput
	CatalogIntentID int64
}

type walletCatalogRenewal struct {
	Intent model.SubscriptionChangeIntent
	Plan   model.SubscriptionPlan
}

func RunWalletSubscriptionRenewalOnce(limit int) (int, error) {
	if limit <= 0 {
		limit = subscriptionResetBatchSize
	}
	now := common.GetTimestamp()
	var contracts []model.UserSubscriptionContract
	if err := model.DB.
		Where("status = ? AND renewal_source = ? AND (renewal_status = ? OR (renewal_status = ? AND latest_change_intent_id > ?)) AND current_period_end > ? AND current_period_end <= ?",
			model.SubscriptionContractStatusActive,
			model.SubscriptionRenewalSourceWallet,
			model.SubscriptionRenewalStatusEnabled,
			model.SubscriptionRenewalStatusPausedInsufficientBalance,
			0,
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
	var attempt *walletRenewalAttempt
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
		catalogRenewal, err := loadReachedWalletCatalogRenewalTx(tx, &contract)
		if err != nil {
			return err
		}
		if contract.RenewalStatus != model.SubscriptionRenewalStatusEnabled &&
			!(contract.RenewalStatus == model.SubscriptionRenewalStatusPausedInsufficientBalance && catalogRenewal != nil) {
			return nil
		}
		var plan *model.SubscriptionPlan
		if catalogRenewal != nil {
			plan = &catalogRenewal.Plan
		} else {
			plan, err = loadWalletRenewalPlanTx(tx, &contract)
		}
		if err != nil {
			common.SysLog(fmt.Sprintf("wallet renewal paused because plan facts are unavailable: contract_id=%d plan_id=%d error=%q", contract.Id, contract.CurrentPlanId, err.Error()))
			return pauseWalletRenewalTx(tx, &contract, model.SubscriptionRenewalStatusPausedPlanUnavailable, result)
		}
		if catalogRenewal == nil {
			if err := validateFlexiblePrepaidPlan(plan); err != nil {
				return pauseWalletRenewalTx(tx, &contract, model.SubscriptionRenewalStatusPausedPlanUnavailable, result)
			}
		}
		if plan.Enabled && plan.AllowBalancePay != nil && !*plan.AllowBalancePay {
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
			if catalogRenewal != nil {
				return pauseWalletCatalogRenewalForBalanceTx(tx, &contract, result)
			}
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
		grantInput := model.GrantEntitlementInput{
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
		}
		attempt = &walletRenewalAttempt{
			ContractID:      contract.Id,
			UserID:          user.Id,
			PlanID:          plan.Id,
			PeriodStart:     periodStart,
			PeriodEnd:       periodEnd,
			RenewalKey:      renewalKey,
			TradeNo:         tradeNo,
			RequiredQuota:   requiredQuota,
			PriceAmount:     plan.PriceAmount,
			PaymentCurrency: plan.Currency,
			PlanSnapshot:    planSnapshot,
			GrantInput:      grantInput,
		}
		if catalogRenewal != nil {
			attempt.CatalogIntentID = catalogRenewal.Intent.Id
		}
		providerPayload := fmt.Sprintf("charged_quota=%d;contract_id=%d;renewal_key=%s", requiredQuota, contract.Id, renewalKey)
		if catalogRenewal != nil {
			providerPayload += fmt.Sprintf(";change_intent_id=%d", catalogRenewal.Intent.Id)
		}
		order := &model.SubscriptionOrder{
			UserId:          user.Id,
			PlanId:          plan.Id,
			Money:           plan.PriceAmount,
			TradeNo:         tradeNo,
			PaymentMethod:   model.PaymentMethodBalance,
			PaymentProvider: model.PaymentProviderBalance,
			Status:          common.TopUpStatusPending,
			CreateTime:      now,
			PurchaseMonths:  1,
			UnitPrice:       plan.PriceAmount,
			PaymentCurrency: plan.Currency,
			PlanSnapshot:    planSnapshot,
			ProviderPayload: providerPayload,
			RenewalSource:   model.SubscriptionRenewalSourceWallet,
		}
		if err := tx.Create(order).Error; err != nil {
			return handleExistingWalletRenewalTx(tx, &contract, renewalKey, result, err)
		}
		if requiredQuota > 0 {
			var mutation model.LifecycleQuotaMutationResult
			err := tx.Transaction(func(debitTx *gorm.DB) error {
				var err error
				mutation, err = model.ApplyWalletQuotaMutationTx(debitTx, user.Id, -int64(requiredQuota), int64(requiredQuota), "subscription_renewal_wallet_debit", renewalKey)
				return err
			})
			if err != nil {
				if errors.Is(err, model.ErrLifecycleQuotaWalletBalanceChanged) {
					deleted := tx.Where("id = ? AND trade_no = ?", order.Id, order.TradeNo).Delete(&model.SubscriptionOrder{})
					if deleted.Error != nil {
						return deleted.Error
					}
					if deleted.RowsAffected != 1 {
						return errors.New("wallet renewal success order cleanup failed")
					}
					if catalogRenewal != nil {
						return pauseWalletCatalogRenewalForBalanceTx(tx, &contract, result)
					}
					return pauseWalletRenewalTx(tx, &contract, model.SubscriptionRenewalStatusPausedInsufficientBalance, result)
				}
				return err
			}
			if !mutation.Applied {
				deleted := tx.Where("id = ? AND trade_no = ?", order.Id, order.TradeNo).Delete(&model.SubscriptionOrder{})
				if deleted.Error != nil {
					return deleted.Error
				}
				if deleted.RowsAffected != 1 {
					return errors.New("wallet renewal success order cleanup failed")
				}
				if catalogRenewal != nil {
					return pauseWalletCatalogRenewalForBalanceTx(tx, &contract, result)
				}
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
		applied, err := model.PersistSubscriptionPurchaseLifecycleTransitionWithWinner(tx, model.PurchaseLifecycleTransition{
			SourceID:   int64(order.Id),
			TradeNo:    order.TradeNo,
			UserID:     order.UserId,
			FromStatus: []string{common.TopUpStatusPending},
			ToStatus:   common.TopUpStatusSuccess,
			OccurredAt: now,
			SourceRef:  renewalKey,
		}, func(tx *gorm.DB, locked *model.SubscriptionOrder, transition *model.PurchaseLifecycleTransition) error {
			grant, err := model.RotateCurrentEntitlementTx(tx, grantInput)
			if err != nil {
				return handleExistingWalletRenewalTx(tx, &contract, renewalKey, result, err)
			}
			if grant != nil && grant.Entitlement != nil {
				transition.SubscriptionScopeID = int64(grant.Entitlement.Id)
			}
			if err := createPrepaidTermSegmentsTx(tx, contract.Id, locked.Id, plan.Id, PrepaidTermAllocation{
				CanonicalWalletUnitPrice: plan.PriceAmount,
			}, periodStart, 1); err != nil {
				return err
			}
			contractUpdates := map[string]interface{}{
				"renewal_status":       model.SubscriptionRenewalStatusEnabled,
				"current_period_start": periodStart,
				"current_period_end":   periodEnd,
				"updated_at":           now,
			}
			if catalogRenewal != nil {
				intentUpdate := tx.Model(&model.SubscriptionChangeIntent{}).
					Where("id = ? AND contract_id = ? AND status = ?", catalogRenewal.Intent.Id, contract.Id, model.SubscriptionChangeIntentStatusScheduled).
					Updates(map[string]interface{}{
						"status":                model.SubscriptionChangeIntentStatusApplied,
						"wallet_debit_trade_no": order.TradeNo,
						"last_error":            "",
						"updated_at":            now,
					})
				if intentUpdate.Error != nil || intentUpdate.RowsAffected != 1 {
					return firstError(intentUpdate.Error, ErrSubscriptionChangeInProgress)
				}
				contractUpdates["current_plan_id"] = plan.Id
				contractUpdates["pending_plan_id"] = 0
				contractUpdates["pending_effective_at"] = 0
			}
			contractUpdate := tx.Model(&model.UserSubscriptionContract{}).Where("id = ?", contract.Id)
			if catalogRenewal != nil {
				contractUpdate = contractUpdate.Where(
					"latest_change_intent_id = ? AND pending_plan_id = ? AND pending_effective_at = ?",
					catalogRenewal.Intent.Id, catalogRenewal.Intent.ToPlanId, catalogRenewal.Intent.EffectiveAt,
				)
			}
			contractUpdate = contractUpdate.Updates(contractUpdates)
			if contractUpdate.Error != nil || contractUpdate.RowsAffected != 1 {
				return firstError(contractUpdate.Error, ErrSubscriptionChangeInProgress)
			}
			if grant != nil && grant.Entitlement != nil {
				result.EntitlementID = grant.Entitlement.Id
			}
			return nil
		})
		if err != nil {
			return err
		}
		if !applied {
			return handleExistingWalletRenewalTx(tx, &contract, renewalKey, result, model.ErrSubscriptionOrderStatusInvalid)
		}
		result.Renewed = true
		result.ChargedQuota = int64(requiredQuota)
		result.OrderID = order.Id
		invalidateUserID = user.Id
		return nil
	})
	if err != nil {
		if !common.UsingPostgreSQL || attempt == nil || !isWalletRenewalDuplicateError(err) {
			return nil, err
		}
		result, err = recoverPostgresWalletRenewalDuplicate(*attempt)
		if err != nil {
			return nil, err
		}
		invalidateUserID = attempt.UserID
	}
	if invalidateUserID > 0 {
		if err := model.InvalidateUserCache(invalidateUserID); err != nil {
			common.SysLog("failed to invalidate user cache after wallet renewal: " + err.Error())
		}
	}
	return result, nil
}

func loadReachedWalletCatalogRenewalTx(tx *gorm.DB, contract *model.UserSubscriptionContract) (*walletCatalogRenewal, error) {
	if tx == nil || contract == nil || contract.LatestChangeIntentId <= 0 {
		return nil, nil
	}
	var intent model.SubscriptionChangeIntent
	if err := subscriptionCommandLock(tx).Where("id = ?", contract.LatestChangeIntentId).First(&intent).Error; err != nil {
		return nil, err
	}
	if intent.Kind != model.SubscriptionChangeIntentKindCatalogMigration {
		return nil, nil
	}
	if contract.CurrentProviderBindingId != 0 ||
		(contract.PaymentMode != model.SubscriptionPaymentModePrepaid && contract.PaymentMode != model.SubscriptionPaymentModeBalanceOnePeriod) {
		return nil, errors.New("wallet catalog migration contract payment facts drifted")
	}
	var currentEntitlement model.UserSubscription
	if err := subscriptionCommandLock(tx).
		Where("id = ? AND contract_id = ? AND user_id = ?", contract.CurrentEntitlementId, contract.Id, contract.UserId).
		First(&currentEntitlement).Error; err != nil {
		return nil, err
	}
	if !walletRenewalCurrentEntitlementMatchesContract(&currentEntitlement, contract) {
		return nil, errors.New("wallet catalog migration current entitlement drifted")
	}
	if intent.Status != model.SubscriptionChangeIntentStatusScheduled || intent.ContractId != contract.Id || intent.UserId != contract.UserId ||
		intent.FromPlanId != contract.CurrentPlanId || intent.ToPlanId <= 0 || intent.ToPlanId != contract.PendingPlanId ||
		intent.EffectiveAt != contract.PendingEffectiveAt || intent.EffectiveAt != contract.CurrentPeriodEnd ||
		intent.EffectiveAt > common.GetTimestamp() || intent.PaymentMode != contract.PaymentMode || intent.ProviderBindingId != 0 || intent.CatalogMigrationBatchId == nil {
		return nil, errors.New("wallet catalog migration intent facts drifted")
	}
	sandbox := catalogMigrationRuntimeSandboxConfig()
	if err := ValidateCatalogMigrationCommonSandbox(sandbox, contract.Id); err != nil {
		return nil, err
	}
	var batch model.SubscriptionCatalogMigrationBatch
	if err := subscriptionCommandLock(tx).Where("id = ?", strings.TrimSpace(*intent.CatalogMigrationBatchId)).First(&batch).Error; err != nil {
		return nil, err
	}
	if !batch.SandboxOnly || batch.Livemode || batch.DeploymentEnvironment != strings.TrimSpace(sandbox.DeploymentEnvironment) ||
		batch.ServiceName != strings.TrimSpace(sandbox.ServiceName) {
		return nil, errors.New("wallet catalog migration sandbox facts drifted")
	}
	snapshot, err := DecodeRecurringPlanSnapshotV1(intent.TargetPlanSnapshot)
	if err != nil {
		return nil, err
	}
	if snapshot.PlanID != intent.ToPlanId {
		return nil, errors.New("wallet catalog migration target snapshot plan drifted")
	}
	var target model.SubscriptionPlan
	if err := subscriptionCommandLock(tx).Where("id = ? AND enabled = ?", snapshot.PlanID, true).First(&target).Error; err != nil {
		return nil, err
	}
	target.NormalizeDefaults()
	if err := ValidateRecurringPlanSnapshotV1AgainstPlan(snapshot, &target); err != nil {
		return nil, err
	}
	if target.AllowBalancePay != nil && !*target.AllowBalancePay {
		return nil, errors.New("wallet catalog migration target plan does not allow balance payment")
	}
	plan := model.SubscriptionPlan{
		Id: snapshot.PlanID, PriceAmount: stripeMinorUnitValue(snapshot.BasePriceMinor, snapshot.Currency), Currency: snapshot.Currency,
		StripePriceId: snapshot.StripePriceID, DurationUnit: snapshot.DurationUnit, DurationValue: snapshot.DurationValue,
		CustomSeconds: snapshot.DurationCustomSeconds, QuotaResetPeriod: snapshot.QuotaResetPeriod,
		QuotaResetCustomSeconds: snapshot.QuotaResetCustomSeconds, TotalAmount: snapshot.TotalAmount,
		MediaCreditsMonthly: snapshot.MediaCreditsMonthly, Window5hAmount: snapshot.Window5hAmount,
		WindowWeekAmount: snapshot.WindowWeekAmount, UpgradeGroup: snapshot.UpgradeGroup, Enabled: true,
	}
	return &walletCatalogRenewal{Intent: intent, Plan: plan}, nil
}

func loadWalletRenewalPlanTx(tx *gorm.DB, contract *model.UserSubscriptionContract) (*model.SubscriptionPlan, error) {
	if tx == nil || contract == nil || contract.CurrentPlanId <= 0 {
		return nil, errors.New("wallet renewal contract facts are incomplete")
	}
	var plan model.SubscriptionPlan
	if err := subscriptionCommandLock(tx).Where("id = ?", contract.CurrentPlanId).First(&plan).Error; err != nil {
		return nil, err
	}
	plan.NormalizeDefaults()
	if plan.Enabled {
		if plan.PriceAmount < 0 {
			return nil, errors.New("subscription plan price cannot be negative")
		}
		if plan.TierRank == nil || *plan.TierRank <= 0 {
			return nil, errors.New("subscription plan tier rank is required")
		}
		return &plan, nil
	}

	var entitlement model.UserSubscription
	if err := subscriptionCommandLock(tx).
		Where("id = ? AND user_id = ? AND contract_id = ?", contract.CurrentEntitlementId, contract.UserId, contract.Id).
		First(&entitlement).Error; err != nil {
		return nil, err
	}
	if !walletRenewalCurrentEntitlementMatchesContract(&entitlement, contract) {
		return nil, errors.New("disabled subscription plan is not the contract's current plan")
	}

	sourceSnapshot, err := loadWalletRenewalSourceSnapshotTx(tx, contract, &entitlement)
	if err != nil {
		return nil, err
	}

	// A retired plan is renewable only through its existing contract. Billing
	// facts come from the exact order referenced by the current entitlement, while
	// benefits come from the entitlement actually granted to the user. This keeps
	// later catalog edits from changing either side of the existing contract.
	plan.Title = sourceSnapshot.Title
	plan.PriceAmount = sourceSnapshot.PriceAmount
	plan.Currency = strings.ToUpper(strings.TrimSpace(sourceSnapshot.Currency))
	plan.StripePriceId = strings.TrimSpace(sourceSnapshot.StripePriceID)
	plan.DurationUnit = sourceSnapshot.DurationUnit
	plan.DurationValue = sourceSnapshot.DurationValue
	plan.QuotaResetPeriod = sourceSnapshot.QuotaResetPeriod
	plan.TotalAmount = entitlement.AmountTotal
	plan.MediaCreditsMonthly = entitlement.MediaCreditsTotal
	if entitlement.Window5hAmount != nil {
		plan.Window5hAmount = *entitlement.Window5hAmount
	} else {
		plan.Window5hAmount = sourceSnapshot.Window5hAmount
	}
	if entitlement.WindowWeekAmount != nil {
		plan.WindowWeekAmount = *entitlement.WindowWeekAmount
	} else {
		plan.WindowWeekAmount = sourceSnapshot.WindowWeekAmount
	}
	plan.UpgradeGroup = entitlement.UpgradeGroup
	return &plan, nil
}

func loadWalletRenewalSourceSnapshotTx(tx *gorm.DB, contract *model.UserSubscriptionContract, entitlement *model.UserSubscription) (purchasePlanSnapshot, error) {
	if tx == nil || contract == nil || entitlement == nil || entitlement.GrantKey == nil {
		return purchasePlanSnapshot{}, errors.New("retired wallet renewal source facts are incomplete")
	}
	grantKey := strings.TrimSpace(*entitlement.GrantKey)
	tradeNo := ""
	switch {
	case strings.HasPrefix(grantKey, "prepaid:"):
		tradeNo = strings.TrimSpace(strings.TrimPrefix(grantKey, "prepaid:"))
	case grantKey == walletRenewalKey(contract.Id, entitlement.StartTime, entitlement.PlanId):
		tradeNo = walletRenewalTradeNo(contract.Id, entitlement.StartTime, entitlement.PlanId)
	}
	if tradeNo == "" {
		return purchasePlanSnapshot{}, errors.New("retired wallet renewal entitlement does not identify a trusted source order")
	}

	var order model.SubscriptionOrder
	if err := tx.Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
		return purchasePlanSnapshot{}, err
	}
	if order.UserId != contract.UserId || order.PlanId != contract.CurrentPlanId ||
		order.PaymentMethod != model.PaymentMethodBalance || order.PaymentProvider != model.PaymentProviderBalance ||
		order.RenewalSource != model.SubscriptionRenewalSourceWallet || order.Status != common.TopUpStatusSuccess ||
		order.PurchaseMonths <= 0 || walletRenewalProviderPayloadValue(order.ProviderPayload, "contract_id") != fmt.Sprint(contract.Id) {
		return purchasePlanSnapshot{}, errors.New("retired wallet renewal source order facts do not match the current contract")
	}
	if strings.HasPrefix(grantKey, "subscription:renewal:") &&
		walletRenewalProviderPayloadValue(order.ProviderPayload, "renewal_key") != grantKey {
		return purchasePlanSnapshot{}, errors.New("retired wallet renewal source order key does not match the current entitlement")
	}

	parsed, err := recurringPlanSnapshotFromOrder(&order)
	if err != nil {
		return purchasePlanSnapshot{}, err
	}
	if !parsed.Found {
		return purchasePlanSnapshot{}, errors.New("retired wallet renewal source order plan snapshot is missing")
	}
	var presence struct {
		PriceAmount      *float64 `json:"price_amount"`
		Currency         *string  `json:"currency"`
		DurationUnit     *string  `json:"duration_unit"`
		DurationValue    *int     `json:"duration_value"`
		Window5hAmount   *int64   `json:"window_5h_amount"`
		WindowWeekAmount *int64   `json:"window_week_amount"`
	}
	if err := common.Unmarshal([]byte(order.PlanSnapshot), &presence); err != nil {
		return purchasePlanSnapshot{}, err
	}
	if presence.PriceAmount == nil || presence.Currency == nil || presence.DurationUnit == nil || presence.DurationValue == nil {
		return purchasePlanSnapshot{}, errors.New("retired wallet renewal source order billing snapshot is incomplete")
	}
	if entitlement.Window5hAmount == nil && presence.Window5hAmount == nil {
		return purchasePlanSnapshot{}, errors.New("retired wallet renewal 5h limit snapshot is missing")
	}
	if entitlement.WindowWeekAmount == nil && presence.WindowWeekAmount == nil {
		return purchasePlanSnapshot{}, errors.New("retired wallet renewal weekly limit snapshot is missing")
	}

	snapshotCurrency := strings.ToUpper(strings.TrimSpace(parsed.Snapshot.Currency))
	orderCurrency := strings.ToUpper(strings.TrimSpace(order.PaymentCurrency))
	if snapshotCurrency == "" || snapshotCurrency != orderCurrency || order.UnitPrice < 0 {
		return purchasePlanSnapshot{}, errors.New("retired wallet renewal source order currency or price is invalid")
	}
	snapshotMinor, err := stripeMinorUnitAmountForSubscription(parsed.Snapshot.PriceAmount, snapshotCurrency)
	if err != nil {
		return purchasePlanSnapshot{}, err
	}
	orderMinor, err := stripeMinorUnitAmountForSubscription(order.UnitPrice, orderCurrency)
	if err != nil {
		return purchasePlanSnapshot{}, err
	}
	if snapshotMinor != orderMinor {
		return purchasePlanSnapshot{}, errors.New("retired wallet renewal source order price does not match its plan snapshot")
	}
	// The successful order's canonical unit price is the amount that was
	// actually accepted for future wallet renewals. The snapshot remains a
	// second consistency check, but its six-decimal value must not create a
	// sub-minor-unit debit that the order itself never recorded.
	parsed.Snapshot.PriceAmount = order.UnitPrice
	parsed.Snapshot.Currency = orderCurrency
	return parsed.Snapshot, nil
}

func walletRenewalProviderPayloadValue(payload string, wantedKey string) string {
	for _, part := range strings.Split(payload, ";") {
		key, value, ok := strings.Cut(part, "=")
		if ok && strings.TrimSpace(key) == wantedKey {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func recoverPostgresWalletRenewalDuplicate(attempt walletRenewalAttempt) (*WalletSubscriptionRenewalResult, error) {
	if attempt.ContractID <= 0 || attempt.UserID <= 0 || attempt.PlanID <= 0 || attempt.PeriodEnd <= attempt.PeriodStart ||
		strings.TrimSpace(attempt.TradeNo) == "" || strings.TrimSpace(attempt.RenewalKey) == "" || attempt.RequiredQuota < 0 ||
		strings.TrimSpace(attempt.PlanSnapshot) == "" || !walletRenewalAttemptGrantInputMatches(attempt) {
		return nil, errors.New("wallet renewal duplicate recovery input is invalid")
	}

	recovered := &WalletSubscriptionRenewalResult{
		ContractID:   attempt.ContractID,
		ChargedQuota: int64(attempt.RequiredQuota),
	}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var order model.SubscriptionOrder
		if err := tx.Where("trade_no = ?", attempt.TradeNo).First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return walletRenewalDuplicateFactsError("success order %q is missing", attempt.TradeNo)
			}
			return fmt.Errorf("read wallet renewal duplicate order: %w", err)
		}
		expectedPayload := fmt.Sprintf("charged_quota=%d;contract_id=%d;renewal_key=%s", attempt.RequiredQuota, attempt.ContractID, attempt.RenewalKey)
		if attempt.CatalogIntentID > 0 {
			expectedPayload += fmt.Sprintf(";change_intent_id=%d", attempt.CatalogIntentID)
		}
		if order.UserId != attempt.UserID || order.PlanId != attempt.PlanID || order.TradeNo != attempt.TradeNo ||
			order.PaymentMethod != model.PaymentMethodBalance || order.PaymentProvider != model.PaymentProviderBalance ||
			order.Status != common.TopUpStatusSuccess || order.PurchaseMonths != 1 || order.Money != attempt.PriceAmount ||
			order.UnitPrice != attempt.PriceAmount || order.PaymentCurrency != attempt.PaymentCurrency ||
			order.RenewalSource != model.SubscriptionRenewalSourceWallet || order.ProviderPayload != expectedPayload ||
			order.PlanSnapshot != attempt.PlanSnapshot {
			return walletRenewalDuplicateFactsError("success order %q is inconsistent", attempt.TradeNo)
		}

		var entitlement model.UserSubscription
		if err := tx.Where("grant_key = ?", attempt.RenewalKey).First(&entitlement).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return walletRenewalDuplicateFactsError("entitlement grant %q is missing", attempt.RenewalKey)
			}
			return fmt.Errorf("read wallet renewal duplicate entitlement: %w", err)
		}
		if !walletRenewalEntitlementMatchesGrantInput(&entitlement, attempt.GrantInput) ||
			(entitlement.Status != model.SubscriptionEntitlementStatusActive && entitlement.Status != model.SubscriptionEntitlementStatusHistorical) {
			return walletRenewalDuplicateFactsError("entitlement grant %q is inconsistent", attempt.RenewalKey)
		}

		var ledger model.WalletLedgerEntry
		ledgerQuery := tx.Where("entry_key = ?", attempt.RenewalKey).Limit(1).Find(&ledger)
		if ledgerQuery.Error != nil {
			return fmt.Errorf("read wallet renewal duplicate ledger: %w", ledgerQuery.Error)
		}
		if attempt.RequiredQuota == 0 {
			if ledgerQuery.RowsAffected != 0 {
				return walletRenewalDuplicateFactsError("zero-cost renewal ledger %q is inconsistent", attempt.RenewalKey)
			}
		} else if ledgerQuery.RowsAffected != 1 {
			return walletRenewalDuplicateFactsError("debit ledger %q is missing", attempt.RenewalKey)
		} else if ledger.UserId != attempt.UserID || ledger.EntryKey != attempt.RenewalKey ||
			ledger.QuotaDelta != -int64(attempt.RequiredQuota) || ledger.MoneyAmount != attempt.PriceAmount ||
			ledger.EntryType != model.WalletLedgerEntryTypePrepaidDebit || ledger.OrderId != order.Id {
			return walletRenewalDuplicateFactsError("debit ledger %q is inconsistent", attempt.RenewalKey)
		}

		var contract model.UserSubscriptionContract
		if err := tx.Where("id = ? AND user_id = ?", attempt.ContractID, attempt.UserID).First(&contract).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return walletRenewalDuplicateFactsError("contract %d is missing", attempt.ContractID)
			}
			return fmt.Errorf("read wallet renewal duplicate contract: %w", err)
		}
		if contract.CurrentPeriodEnd < attempt.PeriodEnd {
			return walletRenewalDuplicateFactsError("contract %d was not advanced through %d", attempt.ContractID, attempt.PeriodEnd)
		}
		if !walletRenewalEntitlementLifecycleMatchesContractPeriod(&entitlement, &contract, attempt) {
			return walletRenewalDuplicateFactsError("entitlement grant %q is inconsistent", attempt.RenewalKey)
		}

		var currentEntitlement model.UserSubscription
		if err := tx.Where("id = ? AND user_id = ? AND contract_id = ?", contract.CurrentEntitlementId, attempt.UserID, attempt.ContractID).
			First(&currentEntitlement).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return walletRenewalDuplicateFactsError("current entitlement %d is missing", contract.CurrentEntitlementId)
			}
			return fmt.Errorf("read wallet renewal duplicate current entitlement: %w", err)
		}
		if !walletRenewalContractMatchesRecoveredGrant(&contract, &entitlement, &currentEntitlement, attempt) {
			return walletRenewalDuplicateFactsError("contract %d is inconsistent", attempt.ContractID)
		}
		if attempt.CatalogIntentID > 0 {
			var intent model.SubscriptionChangeIntent
			if err := tx.Where("id = ? AND contract_id = ?", attempt.CatalogIntentID, attempt.ContractID).First(&intent).Error; err != nil {
				return walletRenewalDuplicateFactsError("catalog intent %d is missing", attempt.CatalogIntentID)
			}
			if intent.Status != model.SubscriptionChangeIntentStatusApplied || intent.WalletDebitTradeNo != attempt.TradeNo ||
				contract.LatestChangeIntentId != attempt.CatalogIntentID || contract.PendingPlanId != 0 || contract.PendingEffectiveAt != 0 {
				return walletRenewalDuplicateFactsError("catalog intent %d is inconsistent", attempt.CatalogIntentID)
			}
		}

		recovered.OrderID = order.Id
		recovered.EntitlementID = entitlement.Id
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return recovered, nil
}

func walletRenewalAttemptGrantInputMatches(attempt walletRenewalAttempt) bool {
	input := attempt.GrantInput
	return input.ContractId == attempt.ContractID &&
		input.UserId == attempt.UserID &&
		input.PlanId == attempt.PlanID &&
		input.ProviderBindingId == 0 &&
		strings.TrimSpace(input.GrantKey) == attempt.RenewalKey &&
		input.PaymentMode == model.SubscriptionPaymentModePrepaid &&
		input.PeriodStart == attempt.PeriodStart &&
		input.PeriodEnd == attempt.PeriodEnd &&
		input.EndReasonForPrevious == model.SubscriptionEntitlementEndReasonRenewed &&
		input.Source == model.PaymentMethodBalance &&
		input.AmountTotal >= 0 &&
		input.MediaCreditsTotal >= 0 &&
		input.Window5hAmount != nil &&
		*input.Window5hAmount >= 0 &&
		input.WindowWeekAmount != nil &&
		*input.WindowWeekAmount >= 0 &&
		input.UpgradeGroup != nil
}

func walletRenewalEntitlementMatchesGrantInput(entitlement *model.UserSubscription, input model.GrantEntitlementInput) bool {
	return entitlement != nil &&
		entitlement.GrantKey != nil &&
		strings.TrimSpace(*entitlement.GrantKey) == strings.TrimSpace(input.GrantKey) &&
		entitlement.ContractId == input.ContractId &&
		entitlement.UserId == input.UserId &&
		entitlement.PlanId == input.PlanId &&
		entitlement.ProviderBindingId == input.ProviderBindingId &&
		entitlement.AmountTotal == input.AmountTotal &&
		entitlement.MediaCreditsTotal == input.MediaCreditsTotal &&
		walletRenewalWindowAmountMatches(entitlement.Window5hAmount, input.Window5hAmount) &&
		walletRenewalWindowAmountMatches(entitlement.WindowWeekAmount, input.WindowWeekAmount) &&
		strings.TrimSpace(entitlement.UpgradeGroup) == strings.TrimSpace(*input.UpgradeGroup) &&
		entitlement.StartTime == input.PeriodStart &&
		entitlement.EndTime == input.PeriodEnd &&
		entitlement.AccessEndTime == input.PeriodEnd &&
		entitlement.PaymentMode == input.PaymentMode &&
		strings.TrimSpace(entitlement.Source) == input.Source
}

func walletRenewalWindowAmountMatches(existing *int64, expected *int64) bool {
	return existing != nil && expected != nil && *existing == *expected
}

func walletRenewalEntitlementLifecycleMatchesContractPeriod(entitlement *model.UserSubscription, contract *model.UserSubscriptionContract, attempt walletRenewalAttempt) bool {
	if entitlement == nil || contract == nil {
		return false
	}
	if contract.CurrentPeriodEnd == attempt.PeriodEnd {
		return entitlement.Status == model.SubscriptionEntitlementStatusActive &&
			entitlement.CurrentSlot != nil && *entitlement.CurrentSlot == 1
	}
	return contract.CurrentPeriodEnd > attempt.PeriodEnd &&
		entitlement.Status == model.SubscriptionEntitlementStatusHistorical &&
		entitlement.CurrentSlot == nil
}

func walletRenewalContractMatchesRecoveredGrant(contract *model.UserSubscriptionContract, entitlement *model.UserSubscription, currentEntitlement *model.UserSubscription, attempt walletRenewalAttempt) bool {
	if contract == nil || entitlement == nil ||
		contract.Id != attempt.ContractID ||
		contract.UserId != attempt.UserID ||
		contract.Status != model.SubscriptionContractStatusActive ||
		!walletRenewalCurrentEntitlementMatchesContract(currentEntitlement, contract) {
		return false
	}
	if contract.CurrentPeriodEnd == attempt.PeriodEnd {
		return contract.RenewalSource == model.SubscriptionRenewalSourceWallet &&
			contract.RenewalStatus == model.SubscriptionRenewalStatusEnabled &&
			contract.CurrentPlanId == attempt.PlanID &&
			contract.CurrentEntitlementId == entitlement.Id &&
			contract.CurrentProviderBindingId == attempt.GrantInput.ProviderBindingId &&
			contract.CurrentPeriodStart == attempt.PeriodStart &&
			contract.PaymentMode == attempt.GrantInput.PaymentMode
	}
	return contract.CurrentPeriodEnd > attempt.PeriodEnd &&
		contract.CurrentPeriodStart >= attempt.PeriodEnd &&
		contract.CurrentPeriodStart < contract.CurrentPeriodEnd &&
		contract.CurrentEntitlementId != entitlement.Id
}

func walletRenewalCurrentEntitlementMatchesContract(entitlement *model.UserSubscription, contract *model.UserSubscriptionContract) bool {
	return entitlement != nil &&
		contract != nil &&
		entitlement.Id == contract.CurrentEntitlementId &&
		entitlement.ContractId == contract.Id &&
		entitlement.UserId == contract.UserId &&
		entitlement.Status == model.SubscriptionEntitlementStatusActive &&
		entitlement.CurrentSlot != nil &&
		*entitlement.CurrentSlot == 1 &&
		entitlement.PlanId == contract.CurrentPlanId &&
		entitlement.ProviderBindingId == contract.CurrentProviderBindingId &&
		entitlement.StartTime == contract.CurrentPeriodStart &&
		entitlement.EndTime == contract.CurrentPeriodEnd &&
		entitlement.PaymentMode == contract.PaymentMode
}

func walletRenewalDuplicateFactsError(format string, args ...interface{}) error {
	return fmt.Errorf("wallet renewal duplicate facts are incomplete or inconsistent: "+format, args...)
}

func walletContractIsRenewable(contract model.UserSubscriptionContract) bool {
	return contract.Status == model.SubscriptionContractStatusActive &&
		contract.RenewalSource == model.SubscriptionRenewalSourceWallet &&
		contract.CurrentPlanId > 0 &&
		contract.CurrentPeriodEnd > 0 &&
		contract.CurrentPeriodEnd <= common.GetTimestamp()
}

func pauseWalletCatalogRenewalForBalanceTx(tx *gorm.DB, contract *model.UserSubscriptionContract, result *WalletSubscriptionRenewalResult) error {
	if tx == nil || contract == nil {
		return errors.New("subscription renewal facts are incomplete")
	}
	update := tx.Model(&model.UserSubscriptionContract{}).
		Where("id = ? AND status = ?", contract.Id, model.SubscriptionContractStatusActive).
		Updates(map[string]interface{}{
			"renewal_status": model.SubscriptionRenewalStatusPausedInsufficientBalance,
			"updated_at":     common.GetTimestamp(),
		})
	if update.Error != nil || update.RowsAffected != 1 {
		return firstError(update.Error, ErrSubscriptionChangeInProgress)
	}
	if result != nil {
		result.PausedStatus = model.SubscriptionRenewalStatusPausedInsufficientBalance
	}
	return nil
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
