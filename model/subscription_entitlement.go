package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	SubscriptionEntitlementEndReasonRenewed          = "renewed"
	SubscriptionEntitlementEndReasonUpgraded         = "upgraded"
	SubscriptionEntitlementEndReasonExpired          = "expired"
	SubscriptionEntitlementEndReasonCancelled        = "cancelled"
	SubscriptionEntitlementEndReasonAdminInvalidated = "admin_invalidated"

	SubscriptionEntitlementStatusActive     = "active"
	SubscriptionEntitlementStatusHistorical = "historical"
)

var ErrSubscriptionEntitlementGrantConflict = errors.New("subscription entitlement grant conflict")

type GrantEntitlementInput struct {
	ContractId           int64
	UserId               int
	PlanId               int
	ProviderBindingId    int64
	GrantKey             string
	PaymentMode          string
	AmountTotal          int64
	PeriodStart          int64
	PeriodEnd            int64
	EndReasonForPrevious string
	Source               string
}

type GrantEntitlementResult struct {
	Entitlement *UserSubscription
	Applied     bool
}

func RotateCurrentEntitlement(input GrantEntitlementInput) (*GrantEntitlementResult, error) {
	input.normalize()
	if err := input.validate(); err != nil {
		return nil, err
	}

	var result *GrantEntitlementResult
	err := DB.Transaction(func(tx *gorm.DB) error {
		applied, err := rotateCurrentEntitlementTx(tx, input)
		if err != nil {
			return err
		}
		result = applied
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func RotateCurrentEntitlementTx(tx *gorm.DB, input GrantEntitlementInput) (*GrantEntitlementResult, error) {
	return rotateCurrentEntitlementTx(tx, input)
}

func rotateCurrentEntitlementTx(tx *gorm.DB, input GrantEntitlementInput) (*GrantEntitlementResult, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	input.normalize()
	if err := input.validate(); err != nil {
		return nil, err
	}
	plan, err := getSubscriptionPlanByIdTx(tx, input.PlanId)
	if err != nil {
		return nil, err
	}

	if existing, found, err := findGrantEntitlementByKeyTx(tx, input.GrantKey); err != nil {
		return nil, err
	} else if found {
		if grantMatchesInput(existing, input) {
			return &GrantEntitlementResult{Entitlement: existing, Applied: false}, nil
		}
		return nil, ErrSubscriptionEntitlementGrantConflict
	}

	var contract UserSubscriptionContract
	if err := lockQuery(tx).Where("id = ? AND user_id = ?", input.ContractId, input.UserId).
		First(&contract).Error; err != nil {
		return nil, err
	}

	if existing, found, err := findGrantEntitlementByKeyTx(tx, input.GrantKey); err != nil {
		return nil, err
	} else if found {
		if grantMatchesInput(existing, input) {
			return &GrantEntitlementResult{Entitlement: existing, Applied: false}, nil
		}
		return nil, ErrSubscriptionEntitlementGrantConflict
	}

	if contract.CurrentEntitlementId > 0 {
		var old UserSubscription
		err := lockQuery(tx).Where("id = ? AND user_id = ? AND contract_id = ? AND current_slot = ?",
			contract.CurrentEntitlementId, input.UserId, input.ContractId, 1).
			First(&old).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if err == nil {
			updates := map[string]interface{}{
				"current_slot": nil,
				"end_reason":   input.EndReasonForPrevious,
				"updated_at":   common.GetTimestamp(),
			}
			if old.Status == SubscriptionEntitlementStatusActive {
				updates["status"] = SubscriptionEntitlementStatusHistorical
			}
			if err := tx.Model(&UserSubscription{}).Where("id = ?", old.Id).Updates(updates).Error; err != nil {
				return nil, err
			}
		}
	}

	currentSlot := 1
	grantKey := input.GrantKey
	sub := &UserSubscription{
		UserId:            input.UserId,
		PlanId:            plan.Id,
		ContractId:        input.ContractId,
		ProviderBindingId: input.ProviderBindingId,
		GrantKey:          &grantKey,
		CurrentSlot:       &currentSlot,
		AmountTotal:       input.AmountTotal,
		AmountUsed:        0,
		StartTime:         input.PeriodStart,
		EndTime:           input.PeriodEnd,
		AccessEndTime:     input.PeriodEnd,
		Status:            SubscriptionEntitlementStatusActive,
		Source:            input.Source,
		PaymentMode:       input.PaymentMode,
		UpgradeGroup:      strings.TrimSpace(plan.UpgradeGroup),
	}
	if err := tx.Create(sub).Error; err != nil {
		return nil, err
	}

	contractUpdates := map[string]interface{}{
		"current_plan_id":             plan.Id,
		"current_entitlement_id":      sub.Id,
		"current_provider_binding_id": input.ProviderBindingId,
		"current_period_start":        input.PeriodStart,
		"current_period_end":          input.PeriodEnd,
		"payment_mode":                input.PaymentMode,
		"status":                      SubscriptionContractStatusActive,
		"change_version":              contract.ChangeVersion + 1,
		"updated_at":                  common.GetTimestamp(),
	}
	if err := applyEntitlementGroupChangeTx(tx, &contract, plan, input.UserId, contractUpdates); err != nil {
		return nil, err
	}
	if err := tx.Model(&UserSubscriptionContract{}).Where("id = ?", contract.Id).Updates(contractUpdates).Error; err != nil {
		return nil, err
	}
	return &GrantEntitlementResult{Entitlement: sub, Applied: true}, nil
}

func (input *GrantEntitlementInput) normalize() {
	input.GrantKey = strings.TrimSpace(input.GrantKey)
	input.PaymentMode = normalizeSubscriptionPaymentMode(input.PaymentMode)
	input.EndReasonForPrevious = normalizeSubscriptionEntitlementEndReason(input.EndReasonForPrevious)
	input.Source = strings.TrimSpace(input.Source)
	if input.Source == "" {
		input.Source = "subscription"
	}
}

func (input GrantEntitlementInput) validate() error {
	if input.ContractId <= 0 {
		return errors.New("invalid contract id")
	}
	if input.UserId <= 0 {
		return errors.New("invalid user id")
	}
	if input.PlanId <= 0 {
		return errors.New("invalid plan id")
	}
	if input.GrantKey == "" {
		return errors.New("grant key is empty")
	}
	if input.PeriodEnd <= input.PeriodStart {
		return errors.New("period end must be after start")
	}
	if input.AmountTotal < 0 {
		return errors.New("amount total must be >= 0")
	}
	return nil
}

func normalizeSubscriptionEntitlementEndReason(reason string) string {
	switch strings.TrimSpace(reason) {
	case SubscriptionEntitlementEndReasonUpgraded,
		SubscriptionEntitlementEndReasonExpired,
		SubscriptionEntitlementEndReasonCancelled,
		SubscriptionEntitlementEndReasonAdminInvalidated:
		return strings.TrimSpace(reason)
	default:
		return SubscriptionEntitlementEndReasonRenewed
	}
}

func findGrantEntitlementByKeyTx(tx *gorm.DB, grantKey string) (*UserSubscription, bool, error) {
	var existing UserSubscription
	query := tx.Where("grant_key = ?", strings.TrimSpace(grantKey)).Limit(1).Find(&existing)
	if query.Error != nil {
		return nil, false, query.Error
	}
	if query.RowsAffected == 0 {
		return nil, false, nil
	}
	return &existing, true, nil
}

func grantMatchesInput(existing *UserSubscription, input GrantEntitlementInput) bool {
	return existing != nil &&
		existing.ContractId == input.ContractId &&
		existing.UserId == input.UserId &&
		existing.PlanId == input.PlanId &&
		existing.ProviderBindingId == input.ProviderBindingId &&
		existing.AmountTotal == input.AmountTotal &&
		existing.StartTime == input.PeriodStart &&
		existing.EndTime == input.PeriodEnd &&
		normalizeSubscriptionPaymentMode(existing.PaymentMode) == input.PaymentMode &&
		strings.TrimSpace(existing.Source) == input.Source
}

func applyEntitlementGroupChangeTx(tx *gorm.DB, contract *UserSubscriptionContract, plan *SubscriptionPlan, userId int, contractUpdates map[string]interface{}) error {
	upgradeGroup := strings.TrimSpace(plan.UpgradeGroup)
	if upgradeGroup == "" {
		return nil
	}
	currentGroup, err := getUserGroupByIdTx(tx, userId)
	if err != nil {
		return err
	}
	if strings.TrimSpace(contract.BaseUserGroup) == "" {
		contractUpdates["base_user_group"] = currentGroup
	}
	if currentGroup != upgradeGroup {
		if err := tx.Model(&User{}).Where("id = ?", userId).Update("group", upgradeGroup).Error; err != nil {
			return err
		}
	}
	return nil
}

func lockQuery(tx *gorm.DB) *gorm.DB {
	if common.UsingSQLite {
		return tx
	}
	return tx.Clauses(clause.Locking{Strength: "UPDATE"})
}

func contractAllowsEntitlementConsumption(status string) bool {
	switch normalizeSubscriptionContractStatus(status) {
	case SubscriptionContractStatusActive, SubscriptionContractStatusGrace, SubscriptionContractStatusNeedsAttention:
		return true
	default:
		return false
	}
}

func findContractCurrentEntitlementForUserTx(tx *gorm.DB, userId int, now int64, lock bool) (*UserSubscription, bool, string, error) {
	if tx == nil {
		tx = DB
	}
	var contract UserSubscriptionContract
	contractQuery := tx
	if lock {
		contractQuery = lockQuery(tx)
	}
	query := contractQuery.Where("user_id = ?", userId).Limit(1).Find(&contract)
	if query.Error != nil {
		return nil, false, "", query.Error
	}
	if query.RowsAffected == 0 {
		return nil, false, "", nil
	}
	if !contractAllowsEntitlementConsumption(contract.Status) || contract.CurrentEntitlementId <= 0 {
		return nil, true, contract.Status, nil
	}

	subQuery := tx
	if lock {
		subQuery = lockQuery(tx)
	}
	var sub UserSubscription
	entitlementQuery := subQuery.Where(
		"id = ? AND user_id = ? AND contract_id = ? AND current_slot = ? AND status = ? AND access_end_time > ?",
		contract.CurrentEntitlementId,
		userId,
		contract.Id,
		1,
		SubscriptionEntitlementStatusActive,
		now,
	).Limit(1).Find(&sub)
	if entitlementQuery.Error != nil {
		return nil, true, contract.Status, entitlementQuery.Error
	}
	if entitlementQuery.RowsAffected == 0 {
		return nil, true, contract.Status, nil
	}
	return &sub, true, contract.Status, nil
}

func getPreConsumableSubscriptionCandidatesTx(tx *gorm.DB, userId int, now int64) ([]UserSubscription, bool, string, error) {
	current, hasContract, contractStatus, err := findContractCurrentEntitlementForUserTx(tx, userId, now, true)
	if err != nil {
		return nil, hasContract, contractStatus, err
	}
	if hasContract {
		if current == nil {
			return nil, true, contractStatus, nil
		}
		return []UserSubscription{*current}, true, contractStatus, nil
	}
	var subs []UserSubscription
	if err := lockQuery(tx).
		Where("user_id = ? AND status = ? AND end_time > ?", userId, SubscriptionEntitlementStatusActive, now).
		Order("end_time asc, id asc").
		Find(&subs).Error; err != nil {
		return nil, false, "", err
	}
	return subs, false, "", nil
}

func hasActiveUserSubscriptionTx(tx *gorm.DB, userId int, now int64) (bool, error) {
	current, hasContract, _, err := findContractCurrentEntitlementForUserTx(tx, userId, now, false)
	if err != nil {
		return false, err
	}
	if hasContract {
		return current != nil, nil
	}
	var count int64
	if err := tx.Model(&UserSubscription{}).
		Where("user_id = ? AND status = ? AND end_time > ?", userId, SubscriptionEntitlementStatusActive, now).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func noActiveSubscriptionError(hasContract bool) error {
	if hasContract {
		return errors.New("no active contract entitlement")
	}
	return errors.New("no active subscription")
}

func insufficientSubscriptionQuotaError(amount int64) error {
	return fmt.Errorf("subscription quota insufficient, need=%d", amount)
}

func isGraceContractCurrentEntitlementTx(tx *gorm.DB, sub *UserSubscription) (bool, error) {
	if tx == nil || sub == nil || sub.ContractId <= 0 {
		return false, nil
	}
	var count int64
	err := tx.Model(&UserSubscriptionContract{}).
		Where("id = ? AND user_id = ? AND status = ? AND current_entitlement_id = ?",
			sub.ContractId, sub.UserId, SubscriptionContractStatusGrace, sub.Id).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
