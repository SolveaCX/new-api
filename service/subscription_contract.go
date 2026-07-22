package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ChangePlanStatusApplied               = "applied"
	ChangePlanStatusScheduled             = "scheduled"
	ChangePlanStatusCheckoutRequired      = "checkout_required"
	ChangePlanStatusPaymentActionRequired = "payment_action_required"
)

var (
	ErrSubscriptionChangeInProgress  = errors.New("subscription change in progress")
	ErrSubscriptionPlanUnchanged     = errors.New("subscription plan unchanged")
	ErrSubscriptionDowngradeDeferred = errors.New("subscription downgrade scheduling is not implemented")
)

type ChangePlanCommand struct {
	UserID      int
	PlanID      int
	PaymentMode string
	RequestID   string
}

type ChangePlanResult struct {
	Status           string                          `json:"status"`
	Contract         *model.UserSubscriptionContract `json:"contract"`
	Intent           *model.SubscriptionChangeIntent `json:"intent"`
	CheckoutURL      string                          `json:"checkout_url,omitempty"`
	HostedInvoiceURL string                          `json:"hosted_invoice_url,omitempty"`
}

func ChangeSubscriptionPlan(cmd ChangePlanCommand) (*ChangePlanResult, error) {
	cmd.normalize()
	if err := cmd.validate(); err != nil {
		return nil, err
	}

	var result *ChangePlanResult
	var balanceEffects *balanceOnePeriodSideEffects
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := subscriptionCommandLock(tx).Where("id = ?", cmd.UserID).First(&user).Error; err != nil {
			return err
		}

		contract, err := getOrCreateContractForUserTx(tx, cmd.UserID)
		if err != nil {
			return err
		}

		if existing, found, err := findIntentByRequestTx(tx, cmd.UserID, cmd.RequestID); err != nil {
			return err
		} else if found {
			result = buildChangePlanResult(existing, contract)
			return nil
		}

		if err := validateChangePaymentMode(cmd.PaymentMode); err != nil {
			return err
		}

		if err := rejectUnresolvedPlanChangeTx(tx, cmd.UserID); err != nil {
			return err
		}

		plan, err := loadEnabledSubscriptionPlanTx(tx, cmd.PlanID)
		if err != nil {
			return err
		}
		if cmd.PaymentMode == model.SubscriptionPaymentModeBalanceOnePeriod && plan.AllowBalancePay != nil && !*plan.AllowBalancePay {
			return errors.New("subscription plan does not allow balance payment")
		}
		if err := enforceMaxPurchasePerUserTx(tx, cmd.UserID, plan); err != nil {
			return err
		}

		kind, err := classifyPlanChangeTx(tx, contract, plan)
		if err != nil {
			return err
		}

		intent := &model.SubscriptionChangeIntent{
			ContractId:    contract.Id,
			UserId:        cmd.UserID,
			RequestId:     cmd.RequestID,
			ChangeVersion: contract.ChangeVersion + 1,
			Kind:          kind,
			PaymentMode:   cmd.PaymentMode,
			Status:        model.SubscriptionChangeIntentStatusCreated,
			FromPlanId:    contract.CurrentPlanId,
			ToPlanId:      plan.Id,
			EffectiveAt:   common.GetTimestamp(),
		}
		if err := tx.Create(intent).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.UserSubscriptionContract{}).Where("id = ?", contract.Id).
			Update("latest_change_intent_id", intent.Id).Error; err != nil {
			return err
		}
		contract.LatestChangeIntentId = intent.Id

		switch cmd.PaymentMode {
		case model.SubscriptionPaymentModeBalanceOnePeriod:
			if kind == model.SubscriptionChangeIntentKindDowngrade {
				return ErrSubscriptionDowngradeDeferred
			}
			effects, err := applyBalanceOnePeriodChangeTx(tx, &user, contract, intent, plan)
			if err != nil {
				return err
			}
			balanceEffects = effects
			result = &ChangePlanResult{
				Status:   ChangePlanStatusApplied,
				Contract: contract,
				Intent:   intent,
			}
			return nil
		case model.SubscriptionPaymentModeStripeRecurring:
			intent.Status = model.SubscriptionChangeIntentStatusAwaitingPayment
			if err := tx.Model(intent).Updates(map[string]interface{}{
				"status":     intent.Status,
				"updated_at": common.GetTimestamp(),
			}).Error; err != nil {
				return err
			}
			result = &ChangePlanResult{
				Status:   ChangePlanStatusCheckoutRequired,
				Contract: contract,
				Intent:   intent,
			}
			return nil
		default:
			return errors.New("unsupported subscription payment mode")
		}
	})
	if err != nil {
		return nil, err
	}
	applyBalanceOnePeriodSideEffects(balanceEffects)
	return result, nil
}

func (cmd *ChangePlanCommand) normalize() {
	cmd.PaymentMode = strings.TrimSpace(cmd.PaymentMode)
	cmd.RequestID = strings.TrimSpace(cmd.RequestID)
}

func (cmd ChangePlanCommand) validate() error {
	if cmd.UserID <= 0 {
		return errors.New("invalid user id")
	}
	if cmd.PlanID <= 0 {
		return errors.New("invalid plan id")
	}
	if cmd.RequestID == "" {
		return errors.New("request_id is required")
	}
	return nil
}

func validateChangePaymentMode(paymentMode string) error {
	switch paymentMode {
	case model.SubscriptionPaymentModeBalanceOnePeriod, model.SubscriptionPaymentModeStripeRecurring:
		return nil
	default:
		return errors.New("payment_mode is required")
	}
}

func subscriptionCommandLock(tx *gorm.DB) *gorm.DB {
	if common.UsingSQLite {
		return tx
	}
	return tx.Clauses(clause.Locking{Strength: "UPDATE"})
}

func loadEnabledSubscriptionPlanTx(tx *gorm.DB, planID int) (*model.SubscriptionPlan, error) {
	var plan model.SubscriptionPlan
	if err := tx.Where("id = ?", planID).First(&plan).Error; err != nil {
		return nil, err
	}
	plan.NormalizeDefaults()
	if !plan.Enabled {
		return nil, errors.New("subscription plan is disabled")
	}
	if plan.PriceAmount < 0 {
		return nil, errors.New("subscription plan price cannot be negative")
	}
	if plan.TierRank == nil || *plan.TierRank <= 0 {
		return nil, errors.New("subscription plan tier rank is required")
	}
	return &plan, nil
}

func enforceMaxPurchasePerUserTx(tx *gorm.DB, userID int, plan *model.SubscriptionPlan) error {
	if plan == nil || plan.MaxPurchasePerUser <= 0 {
		return nil
	}
	var count int64
	if err := tx.Model(&model.UserSubscription{}).
		Where("user_id = ? AND plan_id = ?", userID, plan.Id).
		Count(&count).Error; err != nil {
		return err
	}
	if count >= int64(plan.MaxPurchasePerUser) {
		return errors.New("subscription plan purchase limit reached")
	}
	return nil
}

func getOrCreateContractForUserTx(tx *gorm.DB, userID int) (*model.UserSubscriptionContract, error) {
	var contract model.UserSubscriptionContract
	query := subscriptionCommandLock(tx).Where("user_id = ?", userID).Limit(1).Find(&contract)
	if query.Error != nil {
		return nil, query.Error
	}
	if query.RowsAffected > 0 {
		return &contract, nil
	}
	contract = model.UserSubscriptionContract{
		UserId:      userID,
		Status:      model.SubscriptionContractStatusEnded,
		PaymentMode: model.SubscriptionPaymentModeExternalOnePeriod,
	}
	if err := tx.Create(&contract).Error; err != nil {
		return nil, err
	}
	if err := subscriptionCommandLock(tx).Where("id = ?", contract.Id).First(&contract).Error; err != nil {
		return nil, err
	}
	return &contract, nil
}

func findIntentByRequestTx(tx *gorm.DB, userID int, requestID string) (*model.SubscriptionChangeIntent, bool, error) {
	var intent model.SubscriptionChangeIntent
	query := tx.Where("user_id = ? AND request_id = ?", userID, requestID).Limit(1).Find(&intent)
	if query.Error != nil {
		return nil, false, query.Error
	}
	if query.RowsAffected == 0 {
		return nil, false, nil
	}
	return &intent, true, nil
}

func rejectUnresolvedPlanChangeTx(tx *gorm.DB, userID int) error {
	var count int64
	err := tx.Model(&model.SubscriptionChangeIntent{}).
		Where("user_id = ? AND kind IN ? AND status IN ?",
			userID,
			[]string{
				model.SubscriptionChangeIntentKindPurchase,
				model.SubscriptionChangeIntentKindUpgrade,
				model.SubscriptionChangeIntentKindDowngrade,
			},
			[]string{
				model.SubscriptionChangeIntentStatusCreated,
				model.SubscriptionChangeIntentStatusSyncing,
				model.SubscriptionChangeIntentStatusAwaitingPayment,
				model.SubscriptionChangeIntentStatusScheduled,
				model.SubscriptionChangeIntentStatusCompensationRequired,
			},
		).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrSubscriptionChangeInProgress
	}
	return nil
}

func classifyPlanChangeTx(tx *gorm.DB, contract *model.UserSubscriptionContract, target *model.SubscriptionPlan) (string, error) {
	if contract.CurrentPlanId <= 0 {
		return model.SubscriptionChangeIntentKindPurchase, nil
	}
	if contract.CurrentPlanId == target.Id {
		return "", ErrSubscriptionPlanUnchanged
	}
	var current model.SubscriptionPlan
	if err := tx.Where("id = ?", contract.CurrentPlanId).First(&current).Error; err != nil {
		return "", err
	}
	if current.TierRank == nil || *current.TierRank <= 0 {
		return "", errors.New("current subscription plan tier rank is required")
	}
	if *current.TierRank == *target.TierRank {
		return "", ErrSubscriptionPlanUnchanged
	}
	if *target.TierRank > *current.TierRank {
		return model.SubscriptionChangeIntentKindUpgrade, nil
	}
	return model.SubscriptionChangeIntentKindDowngrade, nil
}

type balanceOnePeriodSideEffects struct {
	userID       int
	planTitle    string
	money        float64
	chargedQuota int
}

func applyBalanceOnePeriodChangeTx(tx *gorm.DB, user *model.User, contract *model.UserSubscriptionContract, intent *model.SubscriptionChangeIntent, plan *model.SubscriptionPlan) (*balanceOnePeriodSideEffects, error) {
	requiredQuota, err := subscriptionBalanceQuota(plan.PriceAmount)
	if err != nil {
		return nil, err
	}
	if requiredQuota > 0 && user.Quota < requiredQuota {
		return nil, errors.New("insufficient balance")
	}
	if requiredQuota > 0 {
		if err := tx.Model(&model.User{}).Where("id = ?", user.Id).
			Update("quota", gorm.Expr("quota - ?", requiredQuota)).Error; err != nil {
			return nil, err
		}
	}

	now := common.GetTimestamp()
	tradeNo := fmt.Sprintf("SUBCONUSR%dINT%dNO%s%d", user.Id, intent.Id, common.GetRandomString(6), time.Now().UnixNano())
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
		ProviderPayload: fmt.Sprintf("charged_quota=%d;change_intent_id=%d", requiredQuota, intent.Id),
	}
	if err := tx.Create(order).Error; err != nil {
		return nil, err
	}

	periodStart := common.GetTimestamp()
	periodEnd, err := subscriptionPlanPeriodEnd(periodStart, plan)
	if err != nil {
		return nil, err
	}
	grant, err := model.RotateCurrentEntitlementTx(tx, model.GrantEntitlementInput{
		ContractId:           contract.Id,
		UserId:               user.Id,
		PlanId:               plan.Id,
		ProviderBindingId:    0,
		GrantKey:             "balance:" + tradeNo,
		PaymentMode:          model.SubscriptionPaymentModeBalanceOnePeriod,
		AmountTotal:          plan.TotalAmount,
		PeriodStart:          periodStart,
		PeriodEnd:            periodEnd,
		EndReasonForPrevious: previousEntitlementEndReason(intent.Kind),
		Source:               model.PaymentMethodBalance,
	})
	if err != nil {
		return nil, err
	}

	intent.Status = model.SubscriptionChangeIntentStatusApplied
	intent.WalletDebitTradeNo = tradeNo
	intent.EffectiveAt = periodStart
	if err := tx.Model(intent).Updates(map[string]interface{}{
		"status":                intent.Status,
		"wallet_debit_trade_no": intent.WalletDebitTradeNo,
		"effective_at":          intent.EffectiveAt,
		"updated_at":            common.GetTimestamp(),
	}).Error; err != nil {
		return nil, err
	}
	if err := tx.Where("id = ?", contract.Id).First(contract).Error; err != nil {
		return nil, err
	}
	if grant != nil && grant.Entitlement != nil {
		contract.CurrentEntitlementId = grant.Entitlement.Id
	}
	return &balanceOnePeriodSideEffects{
		userID:       user.Id,
		planTitle:    plan.Title,
		money:        plan.PriceAmount,
		chargedQuota: requiredQuota,
	}, nil
}

func applyBalanceOnePeriodSideEffects(effects *balanceOnePeriodSideEffects) {
	if effects == nil || effects.userID <= 0 {
		return
	}
	if err := model.InvalidateUserCache(effects.userID); err != nil {
		common.SysLog("failed to invalidate user cache after subscription balance purchase: " + err.Error())
	}
	model.RecordLog(
		effects.userID,
		model.LogTypeTopup,
		fmt.Sprintf("Subscription balance purchase succeeded, plan: %s, amount: %.2f, charged quota: %d", effects.planTitle, effects.money, effects.chargedQuota),
	)
}

func subscriptionBalanceQuota(priceAmount float64) (int, error) {
	if priceAmount <= 0 {
		return 0, nil
	}
	if common.QuotaPerUnit <= 0 {
		return 0, errors.New("quota unit is invalid")
	}
	quota := decimal.NewFromFloat(priceAmount).
		Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
		Ceil().
		IntPart()
	return int(quota), nil
}

func subscriptionPlanPeriodEnd(startUnix int64, plan *model.SubscriptionPlan) (int64, error) {
	if plan == nil {
		return 0, errors.New("plan is nil")
	}
	start := time.Unix(startUnix, 0)
	if plan.DurationValue <= 0 && plan.DurationUnit != model.SubscriptionDurationCustom {
		return 0, errors.New("duration_value must be > 0")
	}
	switch plan.DurationUnit {
	case model.SubscriptionDurationYear:
		return start.AddDate(plan.DurationValue, 0, 0).Unix(), nil
	case model.SubscriptionDurationMonth:
		return start.AddDate(0, plan.DurationValue, 0).Unix(), nil
	case model.SubscriptionDurationDay:
		return start.Add(time.Duration(plan.DurationValue) * 24 * time.Hour).Unix(), nil
	case model.SubscriptionDurationHour:
		return start.Add(time.Duration(plan.DurationValue) * time.Hour).Unix(), nil
	case model.SubscriptionDurationCustom:
		if plan.CustomSeconds <= 0 {
			return 0, errors.New("custom_seconds must be > 0")
		}
		return start.Add(time.Duration(plan.CustomSeconds) * time.Second).Unix(), nil
	default:
		return 0, fmt.Errorf("invalid duration_unit: %s", plan.DurationUnit)
	}
}

func previousEntitlementEndReason(kind string) string {
	switch kind {
	case model.SubscriptionChangeIntentKindUpgrade:
		return model.SubscriptionEntitlementEndReasonUpgraded
	default:
		return model.SubscriptionEntitlementEndReasonRenewed
	}
}

func buildChangePlanResult(intent *model.SubscriptionChangeIntent, contract *model.UserSubscriptionContract) *ChangePlanResult {
	status := changePlanResultStatus(intent.Status)
	return &ChangePlanResult{
		Status:   status,
		Contract: contract,
		Intent:   intent,
	}
}

func changePlanResultStatus(intentStatus string) string {
	switch intentStatus {
	case model.SubscriptionChangeIntentStatusApplied:
		return ChangePlanStatusApplied
	case model.SubscriptionChangeIntentStatusScheduled:
		return ChangePlanStatusScheduled
	case model.SubscriptionChangeIntentStatusAwaitingPayment, model.SubscriptionChangeIntentStatusSyncing:
		return ChangePlanStatusCheckoutRequired
	default:
		return intentStatus
	}
}
