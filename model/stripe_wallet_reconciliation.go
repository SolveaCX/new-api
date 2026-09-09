package model

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrStripeWalletReconciliationLeaseLost = errors.New("stripe wallet reconciliation lease lost")

// StripeWalletScanState persists both the recent and one-time backfill cursors
// for one Stripe account/configuration scope. LeaseToken fences concurrent
// scanners across application nodes.
type StripeWalletScanState struct {
	Scope                string `gorm:"primaryKey;type:varchar(120);autoIncrement:false"`
	InitialAt            int64  `gorm:"not null"`
	RecentThrough        int64  `gorm:"not null"`
	RecentWindowEnd      int64  `gorm:"not null;default:0"`
	RecentAfter          string `gorm:"type:varchar(255);not null;default:''"`
	BackfillAfter        string `gorm:"type:varchar(255);not null;default:''"`
	BackfillDone         bool   `gorm:"not null;default:false"`
	LeaseToken           string `gorm:"type:varchar(128);not null;default:'';index"`
	LeaseUntil           int64  `gorm:"not null;default:0;index"`
	FailureSince         int64  `gorm:"not null;default:0"`
	LastFailureAlertAt   int64  `gorm:"not null;default:0"`
	LastFailureAttemptAt int64  `gorm:"not null;default:0"`
	LastSuccessAt        int64  `gorm:"not null;default:0"`
}

// StripeWalletPaymentCheck is a durable reconciliation work item. Successful
// automatic repairs are fenced by its lease and recorded on the same database
// transaction as the wallet credit.
type StripeWalletPaymentCheck struct {
	Id                  int64  `gorm:"primaryKey"`
	Scope               string `gorm:"type:varchar(120);not null;uniqueIndex:idx_stripe_wallet_check_scope_session,priority:1;index:idx_stripe_wallet_check_due,priority:1"`
	SessionID           string `gorm:"type:varchar(255);not null;uniqueIndex:idx_stripe_wallet_check_scope_session,priority:2"`
	EventID             string `gorm:"type:varchar(255);not null;default:''"`
	TradeNo             string `gorm:"type:varchar(255);not null;default:'';index"`
	PriceID             string `gorm:"type:varchar(255);not null;default:''"`
	UserID              int    `gorm:"not null;default:0"`
	PaidAt              int64  `gorm:"not null;default:0"`
	Historical          bool   `gorm:"not null;default:false"`
	Amount              int64  `gorm:"not null;default:0"`
	Currency            string `gorm:"type:varchar(16);not null;default:''"`
	WalletVerified      bool   `gorm:"not null;default:false"`
	SessionVerified     bool   `gorm:"not null;default:false"`
	DiscoveryError      string `gorm:"type:text"`
	VerificationError   string `gorm:"type:text"`
	VerificationPending bool   `gorm:"not null;default:false"`
	FirstAnomalyAt      int64  `gorm:"not null;default:0"`
	LastNotifiedAt      int64  `gorm:"not null;default:0"`
	LastAttemptAt       int64  `gorm:"not null;default:0"`
	LastAttemptKind     string `gorm:"type:varchar(32);not null;default:''"`
	NotificationCount   int    `gorm:"not null;default:0"`
	RecoveredAt         int64  `gorm:"not null;default:0"`
	ClosedAt            int64  `gorm:"not null;default:0;index:idx_stripe_wallet_check_due,priority:2"`
	NextCheckAt         int64  `gorm:"not null;default:0;index:idx_stripe_wallet_check_due,priority:3"`
	LocalStatus         string `gorm:"type:varchar(32);not null;default:''"`
	RepairError         string `gorm:"type:text"`
	AutoRepairedAt      int64  `gorm:"not null;default:0"`
	RepairFromStatus    string `gorm:"type:varchar(32);not null;default:''"`
	RepairCredit        int64  `gorm:"not null;default:0"`
	LeaseToken          string `gorm:"type:varchar(128);not null;default:''"`
	LeaseUntil          int64  `gorm:"not null;default:0"`
}

func AcquireStripeWalletScan(ctx context.Context, scope string, now int64, leaseSeconds int64, token string) (*StripeWalletScanState, bool, error) {
	if err := validateStripeWalletLeaseInput(ctx, scope, leaseSeconds, token); err != nil {
		return nil, false, err
	}
	scope = strings.TrimSpace(scope)
	token = strings.TrimSpace(token)
	if now <= 0 {
		return nil, false, errors.New("stripe wallet scan initial timestamp must be positive")
	}

	var state StripeWalletScanState
	seed := StripeWalletScanState{Scope: scope, InitialAt: now, RecentThrough: now}
	if err := DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
		return nil, false, err
	}
	dbNow, err := GetDBTimestampWithContext(ctx)
	if err != nil {
		return nil, false, err
	}
	leaseUntil := dbNow + leaseSeconds
	result := DB.WithContext(ctx).Model(&StripeWalletScanState{}).
		Where("scope = ? AND (lease_token = ? OR lease_until <= ?)", scope, "", dbNow).
		Updates(map[string]any{"lease_token": token, "lease_until": leaseUntil})
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, false, nil
	}
	if err := DB.WithContext(ctx).Where("scope = ? AND lease_token = ? AND lease_until = ?", scope, token, leaseUntil).First(&state).Error; err != nil {
		return nil, false, err
	}
	return &state, true, nil
}

func SaveStripeWalletScan(ctx context.Context, state *StripeWalletScanState, token string) error {
	if state == nil {
		return errors.New("stripe wallet scan state is nil")
	}
	if err := validateStripeWalletLeaseInput(ctx, state.Scope, 1, token); err != nil {
		return err
	}
	token = strings.TrimSpace(token)
	dbNow, err := GetDBTimestampWithContext(ctx)
	if err != nil {
		return err
	}
	result := DB.WithContext(ctx).Model(&StripeWalletScanState{}).
		Where("scope = ? AND lease_token = ? AND lease_until > ?", state.Scope, token, dbNow).
		Updates(map[string]any{
			"recent_through":          state.RecentThrough,
			"recent_window_end":       state.RecentWindowEnd,
			"recent_after":            state.RecentAfter,
			"backfill_after":          state.BackfillAfter,
			"backfill_done":           state.BackfillDone,
			"failure_since":           state.FailureSince,
			"last_failure_alert_at":   state.LastFailureAlertAt,
			"last_failure_attempt_at": state.LastFailureAttemptAt,
			"last_success_at":         state.LastSuccessAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		var count int64
		if err := DB.WithContext(ctx).Model(&StripeWalletScanState{}).
			Where("scope = ? AND lease_token = ? AND lease_until > ?", state.Scope, token, dbNow).
			Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return ErrStripeWalletReconciliationLeaseLost
		}
	}
	return nil
}

func ReleaseStripeWalletScan(ctx context.Context, scope string, token string) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	scope = strings.TrimSpace(scope)
	token = strings.TrimSpace(token)
	if scope == "" || token == "" {
		return errors.New("stripe wallet scan scope and lease token are required")
	}
	if DB == nil {
		return errors.New("database is not initialized")
	}
	result := DB.WithContext(ctx).Model(&StripeWalletScanState{}).
		Where("scope = ? AND lease_token = ?", scope, token).
		Updates(map[string]any{"lease_token": "", "lease_until": int64(0)})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrStripeWalletReconciliationLeaseLost
	}
	return nil
}

func InsertStripeWalletPaymentCheck(ctx context.Context, row *StripeWalletPaymentCheck) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	if row == nil {
		return errors.New("stripe wallet payment check is nil")
	}
	row.Scope = strings.TrimSpace(row.Scope)
	row.SessionID = strings.TrimSpace(row.SessionID)
	if row.Scope == "" || row.SessionID == "" {
		return errors.New("stripe wallet payment check scope and session id are required")
	}
	if DB == nil {
		return errors.New("database is not initialized")
	}
	return DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error
}

func ClaimStripeWalletPaymentChecks(ctx context.Context, scope string, now int64, leaseSeconds int64, limit int, token string) ([]StripeWalletPaymentCheck, error) {
	return claimStripeWalletPaymentChecks(ctx, scope, now, leaseSeconds, limit, token, "")
}

func ClaimStripeWalletPaymentChecksForLane(ctx context.Context, scope string, now int64, leaseSeconds int64, limit int, token, lane string) ([]StripeWalletPaymentCheck, error) {
	if lane != "recent" && lane != "active" && lane != "history" {
		return nil, errors.New("invalid Stripe wallet check lane")
	}
	return claimStripeWalletPaymentChecks(ctx, scope, now, leaseSeconds, limit, token, lane)
}

func claimStripeWalletPaymentChecks(ctx context.Context, scope string, now int64, leaseSeconds int64, limit int, token, lane string) ([]StripeWalletPaymentCheck, error) {
	if err := validateStripeWalletLeaseInput(ctx, scope, leaseSeconds, token); err != nil {
		return nil, err
	}
	scope = strings.TrimSpace(scope)
	token = strings.TrimSpace(token)
	if now <= 0 {
		return nil, errors.New("stripe wallet payment check due timestamp must be positive")
	}
	if limit <= 0 {
		return []StripeWalletPaymentCheck{}, nil
	}
	if limit > 100 {
		limit = 100
	}

	claimed := make([]StripeWalletPaymentCheck, 0, limit)
	dbNow, err := GetDBTimestampWithContext(ctx)
	if err != nil {
		return nil, err
	}
	var candidates []StripeWalletPaymentCheck
	query := DB.WithContext(ctx).
		Where("scope = ? AND closed_at = 0 AND next_check_at <= ? AND (lease_token = ? OR lease_until <= ?)", scope, now, "", dbNow)
	switch lane {
	case "active":
		query = query.Where("first_anomaly_at > 0")
	case "recent":
		query = query.Where("first_anomaly_at = 0 AND historical = ?", false)
	case "history":
		query = query.Where("first_anomaly_at = 0 AND historical = ?", true)
	}
	if err := query.Order("next_check_at ASC").Order("id ASC").Limit(limit).Find(&candidates).Error; err != nil {
		return nil, err
	}
	leaseUntil := dbNow + leaseSeconds
	for i := range candidates {
		result := DB.WithContext(ctx).Model(&StripeWalletPaymentCheck{}).
			Where("id = ? AND scope = ? AND closed_at = 0 AND next_check_at <= ? AND (lease_token = ? OR lease_until <= ?)", candidates[i].Id, scope, now, "", dbNow).
			Updates(map[string]any{"lease_token": token, "lease_until": leaseUntil})
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected == 1 {
			candidates[i].LeaseToken = token
			candidates[i].LeaseUntil = leaseUntil
			claimed = append(claimed, candidates[i])
		}
	}
	return claimed, nil
}

func SaveStripeWalletPaymentCheck(ctx context.Context, row *StripeWalletPaymentCheck, token string) error {
	return persistStripeWalletPaymentCheck(ctx, row, token, true)
}

// CheckpointStripeWalletPaymentCheck persists the latest observation while
// retaining the caller's lease across the subsequent notification attempt.
func CheckpointStripeWalletPaymentCheck(ctx context.Context, row *StripeWalletPaymentCheck, token string) error {
	return persistStripeWalletPaymentCheck(ctx, row, token, false)
}

func persistStripeWalletPaymentCheck(ctx context.Context, row *StripeWalletPaymentCheck, token string, release bool) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	if row == nil || row.Id <= 0 {
		return errors.New("stripe wallet payment check id is required")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("stripe wallet payment check lease token is required")
	}
	dbNow, err := GetDBTimestampWithContext(ctx)
	if err != nil {
		return err
	}
	updates := map[string]any{
		"price_id":             row.PriceID,
		"session_verified":     row.SessionVerified,
		"verification_error":   row.VerificationError,
		"verification_pending": row.VerificationPending,
		"user_id":              row.UserID,
		"first_anomaly_at":     row.FirstAnomalyAt,
		"last_notified_at":     row.LastNotifiedAt,
		"last_attempt_at":      row.LastAttemptAt,
		"last_attempt_kind":    row.LastAttemptKind,
		"notification_count":   row.NotificationCount,
		"recovered_at":         row.RecoveredAt,
		"closed_at":            row.ClosedAt,
		"next_check_at":        row.NextCheckAt,
		"local_status":         row.LocalStatus,
		"repair_error":         row.RepairError,
		"wallet_verified":      row.WalletVerified,
	}
	if release {
		updates["lease_token"] = ""
		updates["lease_until"] = int64(0)
	}
	result := DB.WithContext(ctx).Model(&StripeWalletPaymentCheck{}).
		Where("id = ? AND lease_token = ? AND lease_until > ?", row.Id, token, dbNow).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		if !release {
			var count int64
			if err := DB.WithContext(ctx).Model(&StripeWalletPaymentCheck{}).
				Where("id = ? AND lease_token = ? AND lease_until > ?", row.Id, token, dbNow).
				Count(&count).Error; err != nil {
				return err
			}
			if count == 1 {
				return nil
			}
		}
		return ErrStripeWalletReconciliationLeaseLost
	}
	return nil
}

// Existing observations can be checked against local orders even when Stripe
// account discovery is unavailable. This never guesses a scope from a new key.
func ListActiveStripeWalletCheckScopes(ctx context.Context) ([]string, error) {
	var scopes []string
	if DB == nil {
		return nil, errors.New("database is not initialized")
	}
	err := DB.WithContext(ctx).Model(&StripeWalletPaymentCheck{}).
		Where("closed_at = 0").Distinct("scope").Order("scope").Limit(100).Pluck("scope", &scopes).Error
	return scopes, err
}

func HasStripeWalletVerificationFailures(ctx context.Context, scope string) (bool, error) {
	if DB == nil {
		return false, errors.New("database is not initialized")
	}
	var row StripeWalletPaymentCheck
	err := DB.WithContext(ctx).Select("id").Where("scope = ? AND closed_at = 0 AND verification_pending = ?", scope, true).Limit(1).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return err == nil, err
}

func GetStripeWalletTopUpForReconciliation(ctx context.Context, tradeNo string) (*TopUp, error) {
	if ctx == nil {
		return nil, errors.New("context is nil")
	}
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo == "" {
		return nil, ErrTopUpNotFound
	}
	if DB == nil {
		return nil, errors.New("database is not initialized")
	}
	topUp := &TopUp{}
	if err := DB.WithContext(ctx).Where("trade_no = ?", tradeNo).First(topUp).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTopUpNotFound
		}
		return nil, err
	}
	return topUp, nil
}

func validateStripeWalletLeaseInput(ctx context.Context, scope string, leaseSeconds int64, token string) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	if DB == nil {
		return errors.New("database is not initialized")
	}
	if strings.TrimSpace(scope) == "" || strings.TrimSpace(token) == "" {
		return errors.New("stripe wallet reconciliation scope and lease token are required")
	}
	if leaseSeconds <= 0 {
		return fmt.Errorf("stripe wallet reconciliation lease seconds must be positive: %d", leaseSeconds)
	}
	return nil
}
