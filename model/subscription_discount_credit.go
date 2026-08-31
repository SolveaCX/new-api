package model

import (
	"errors"
	"math"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	SubscriptionDiscountEntryTypeGrantInvitee = "grant_invitee"
	SubscriptionDiscountEntryTypeGrantInviter = "grant_inviter"
	SubscriptionDiscountEntryTypeMigration    = "migration"
	SubscriptionDiscountEntryTypeReserve      = "reserve"
	SubscriptionDiscountEntryTypeCommit       = "commit"
	SubscriptionDiscountEntryTypeRelease      = "release"

	SubscriptionDiscountFundingSourceNone    = "none"
	SubscriptionDiscountFundingSourceInvitee = "invitee"
	SubscriptionDiscountFundingSourceInviter = "inviter"
	SubscriptionDiscountFundingSourceMixed   = "mixed"
	SubscriptionDiscountFundingSourceUnknown = "unknown"

	subscriptionDiscountMaxBusinessKeyLength         = 191
	subscriptionDiscountMaxTerminalIdempotencyLength = 191
	subscriptionDiscountMaxSourceTypeLength          = 64
	subscriptionDiscountMaxTradeNoLength             = 255
)

var (
	ErrSubscriptionDiscountInvalidAmount       = errors.New("subscription discount invalid amount")
	ErrSubscriptionDiscountInsufficient        = errors.New("subscription discount insufficient available credit")
	ErrSubscriptionDiscountInvalidReservation  = errors.New("subscription discount invalid reservation")
	ErrSubscriptionDiscountReservationNotFound = errors.New("subscription discount reservation not found")
	ErrSubscriptionDiscountInvalidAccountState = errors.New("subscription discount invalid account state")
	ErrSubscriptionDiscountImmutableEntry      = errors.New("subscription discount entry is immutable")
	ErrSubscriptionDiscountInvalidEntryType    = errors.New("subscription discount invalid entry type")
)

type SubscriptionDiscountAccount struct {
	UserID            int   `json:"user_id" gorm:"primaryKey;autoIncrement:false"`
	AvailableUSDMinor int64 `json:"available_usd_minor" gorm:"type:bigint;not null;default:0"`
	ReservedUSDMinor  int64 `json:"reserved_usd_minor" gorm:"type:bigint;not null;default:0"`
	CreatedAt         int64 `json:"created_at" gorm:"type:bigint;not null;default:0"`
	UpdatedAt         int64 `json:"updated_at" gorm:"type:bigint;not null;default:0"`
}

func (a *SubscriptionDiscountAccount) BeforeCreate(tx *gorm.DB) error {
	return validateSubscriptionDiscountAccount(a)
}

func (a *SubscriptionDiscountAccount) BeforeUpdate(tx *gorm.DB) error {
	if a.UserID == 0 {
		return nil
	}
	return validateSubscriptionDiscountAccount(a)
}

type SubscriptionDiscountEntry struct {
	ID int64 `json:"id"`

	UserID                 int     `json:"user_id" gorm:"not null;index"`
	EntryType              string  `json:"entry_type" gorm:"type:varchar(32);not null;default:'';index"`
	FundingSource          string  `json:"funding_source" gorm:"type:varchar(16);not null;default:'unknown';index"`
	AvailableDeltaUSDMinor int64   `json:"available_delta_usd_minor" gorm:"type:bigint;not null;default:0"`
	ReservedDeltaUSDMinor  int64   `json:"reserved_delta_usd_minor" gorm:"type:bigint;not null;default:0"`
	AvailableAfterUSDMinor int64   `json:"available_after_usd_minor" gorm:"type:bigint;not null;default:0"`
	ReservedAfterUSDMinor  int64   `json:"reserved_after_usd_minor" gorm:"type:bigint;not null;default:0"`
	SourceType             string  `json:"source_type" gorm:"type:varchar(64);not null;default:'';index"`
	SourceKey              string  `json:"source_key" gorm:"type:varchar(191);not null;default:'';index"`
	OrderID                int     `json:"order_id" gorm:"not null;default:0;index"`
	TradeNo                string  `json:"trade_no" gorm:"type:varchar(255);not null;default:'';index"`
	PaymentCurrency        string  `json:"payment_currency" gorm:"type:varchar(16);not null;default:''"`
	AppliedAmountMinor     int64   `json:"applied_amount_minor" gorm:"type:bigint;not null;default:0"`
	PricingSnapshot        string  `json:"pricing_snapshot" gorm:"type:text"`
	IdempotencyKey         string  `json:"idempotency_key" gorm:"type:varchar(191);not null;uniqueIndex"`
	TerminalReservationKey *string `json:"terminal_reservation_key,omitempty" gorm:"type:varchar(191);uniqueIndex"`
	ExpiresAt              int64   `json:"expires_at" gorm:"type:bigint;not null;default:0;index"`
	CreatedAt              int64   `json:"created_at" gorm:"type:bigint;not null;default:0;index"`
}

func (e *SubscriptionDiscountEntry) BeforeUpdate(tx *gorm.DB) error {
	return ErrSubscriptionDiscountImmutableEntry
}

func (e *SubscriptionDiscountEntry) BeforeDelete(tx *gorm.DB) error {
	return ErrSubscriptionDiscountImmutableEntry
}

type SubscriptionDiscountGrantInput struct {
	UserID          int
	USDMinor        int64
	EntryType       string
	SourceType      string
	SourceKey       string
	IdempotencyKey  string
	PricingSnapshot string
}

type SubscriptionDiscountReservationInput struct {
	UserID             int
	USDMinor           int64
	OrderID            int
	TradeNo            string
	PaymentCurrency    string
	AppliedAmountMinor int64
	FundingSource      string
	PricingSnapshot    string
	IdempotencyKey     string
	ExpiresAt          int64
}

func GetSubscriptionDiscountAccount(userID int) (*SubscriptionDiscountAccount, error) {
	return GetSubscriptionDiscountAccountTx(DB, userID)
}

func GetSubscriptionDiscountAccountTx(tx *gorm.DB, userID int) (*SubscriptionDiscountAccount, error) {
	if userID <= 0 {
		return nil, ErrSubscriptionDiscountInvalidAccountState
	}
	var account SubscriptionDiscountAccount
	err := tx.Where("user_id = ?", userID).First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &SubscriptionDiscountAccount{UserID: userID}, nil
	}
	if err != nil {
		return nil, err
	}
	if err := validateSubscriptionDiscountAccount(&account); err != nil {
		return nil, err
	}
	return &account, nil
}

func GrantSubscriptionDiscountTx(tx *gorm.DB, input SubscriptionDiscountGrantInput) (bool, error) {
	if input.UserID <= 0 {
		return false, ErrSubscriptionDiscountInvalidAccountState
	}
	if input.USDMinor < 0 {
		return false, ErrSubscriptionDiscountInvalidAmount
	}
	if input.USDMinor == 0 {
		return false, nil
	}
	entryType := strings.TrimSpace(input.EntryType)
	if !isValidSubscriptionDiscountGrantEntryType(entryType) {
		return false, ErrSubscriptionDiscountInvalidEntryType
	}
	sourceType, sourceKey, idempotencyKey, err := normalizeSubscriptionDiscountGrantInput(input)
	if err != nil {
		return false, err
	}
	if err := validateSubscriptionDiscountPricingSnapshot(input.PricingSnapshot); err != nil {
		return false, err
	}
	exists, err := subscriptionDiscountIdempotencyExistsTx(tx, idempotencyKey)
	if err != nil || exists {
		return false, err
	}

	account, now, accountCreated, err := lockSubscriptionDiscountAccountTx(tx, input.UserID)
	if err != nil {
		return false, err
	}
	afterAvailable, ok := checkedAddInt64(account.AvailableUSDMinor, input.USDMinor)
	if !ok {
		if accountCreated {
			if cleanupErr := deleteNewEmptySubscriptionDiscountAccountTx(tx, input.UserID); cleanupErr != nil {
				return false, cleanupErr
			}
		}
		return false, ErrSubscriptionDiscountInvalidAmount
	}
	entry := SubscriptionDiscountEntry{
		UserID:                 input.UserID,
		EntryType:              entryType,
		FundingSource:          subscriptionDiscountFundingSourceForGrantInput(entryType, sourceType),
		AvailableDeltaUSDMinor: input.USDMinor,
		ReservedDeltaUSDMinor:  0,
		AvailableAfterUSDMinor: afterAvailable,
		ReservedAfterUSDMinor:  account.ReservedUSDMinor,
		SourceType:             sourceType,
		SourceKey:              sourceKey,
		PricingSnapshot:        input.PricingSnapshot,
		IdempotencyKey:         idempotencyKey,
		CreatedAt:              now,
	}
	created, err := createSubscriptionDiscountEntryTx(tx, &entry)
	if err != nil || !created {
		if !created && accountCreated {
			if cleanupErr := deleteNewEmptySubscriptionDiscountAccountTx(tx, input.UserID); cleanupErr != nil {
				return false, cleanupErr
			}
		}
		return created, err
	}
	update := tx.Model(&SubscriptionDiscountAccount{}).
		Where("user_id = ?", input.UserID).
		Updates(map[string]any{
			"available_usd_minor": afterAvailable,
			"reserved_usd_minor":  account.ReservedUSDMinor,
			"updated_at":          now,
		})
	if update.Error != nil {
		return false, update.Error
	}
	if update.RowsAffected != 1 {
		return false, ErrSubscriptionDiscountInvalidAccountState
	}
	return true, nil
}

func subscriptionDiscountUSDToMinor(usd float64) (int64, error) {
	if math.IsNaN(usd) || math.IsInf(usd, 0) || usd < 0 {
		return 0, ErrSubscriptionDiscountInvalidAmount
	}
	minor := decimal.NewFromFloat(usd).
		Mul(decimal.NewFromInt(100)).
		Round(0)
	if minor.GreaterThan(decimal.NewFromInt(math.MaxInt64)) {
		return 0, ErrSubscriptionDiscountInvalidAmount
	}
	return minor.IntPart(), nil
}

func ReserveSubscriptionDiscountTx(tx *gorm.DB, input SubscriptionDiscountReservationInput) (bool, error) {
	if input.UserID <= 0 {
		return false, ErrSubscriptionDiscountInvalidAccountState
	}
	if input.USDMinor <= 0 {
		return false, ErrSubscriptionDiscountInvalidAmount
	}
	normalized, err := normalizeSubscriptionDiscountReservationInput(input)
	if err != nil {
		return false, err
	}
	if err := validateSubscriptionDiscountPricingSnapshot(input.PricingSnapshot); err != nil {
		return false, err
	}
	exists, err := subscriptionDiscountIdempotencyExistsTx(tx, normalized.idempotencyKey)
	if err != nil || exists {
		return false, err
	}
	if normalized.expiresAt <= common.GetTimestamp() {
		return false, ErrSubscriptionDiscountInvalidReservation
	}

	account, now, accountCreated, err := lockSubscriptionDiscountAccountTx(tx, input.UserID)
	if err != nil {
		return false, err
	}
	// Recompute provenance while the account is locked. The caller's marker is
	// only an audit hint; never let a forged `invitee` value hide inviter-origin
	// credit. Service paths resolve this before calling Reserve as well, so this
	// second check also protects internal/legacy callers.
	resolvedSource, resolveErr := resolveSubscriptionDiscountFundingSourceFromGrantHistoryTx(tx, input.UserID, 0, input.USDMinor)
	if resolveErr != nil {
		return false, resolveErr
	}
	if resolvedSource == SubscriptionDiscountFundingSourceNone {
		resolvedSource = SubscriptionDiscountFundingSourceUnknown
	}
	normalized.fundingSource = resolvedSource
	if account.AvailableUSDMinor < input.USDMinor {
		if accountCreated {
			if cleanupErr := deleteNewEmptySubscriptionDiscountAccountTx(tx, input.UserID); cleanupErr != nil {
				return false, cleanupErr
			}
		}
		return false, ErrSubscriptionDiscountInsufficient
	}
	afterAvailable, ok := checkedSubInt64(account.AvailableUSDMinor, input.USDMinor)
	if !ok {
		return false, ErrSubscriptionDiscountInvalidAccountState
	}
	afterReserved, ok := checkedAddInt64(account.ReservedUSDMinor, input.USDMinor)
	if !ok {
		if accountCreated {
			if cleanupErr := deleteNewEmptySubscriptionDiscountAccountTx(tx, input.UserID); cleanupErr != nil {
				return false, cleanupErr
			}
		}
		return false, ErrSubscriptionDiscountInvalidAccountState
	}
	entry := SubscriptionDiscountEntry{
		UserID:                 input.UserID,
		EntryType:              SubscriptionDiscountEntryTypeReserve,
		FundingSource:          normalized.fundingSource,
		AvailableDeltaUSDMinor: -input.USDMinor,
		ReservedDeltaUSDMinor:  input.USDMinor,
		AvailableAfterUSDMinor: afterAvailable,
		ReservedAfterUSDMinor:  afterReserved,
		SourceType:             SubscriptionDiscountEntryTypeReserve,
		SourceKey:              normalized.idempotencyKey,
		OrderID:                normalized.orderID,
		TradeNo:                normalized.tradeNo,
		PaymentCurrency:        normalized.paymentCurrency,
		AppliedAmountMinor:     normalized.appliedAmountMinor,
		PricingSnapshot:        input.PricingSnapshot,
		IdempotencyKey:         normalized.idempotencyKey,
		ExpiresAt:              normalized.expiresAt,
		CreatedAt:              now,
	}
	created, err := createSubscriptionDiscountEntryTx(tx, &entry)
	if err != nil || !created {
		if !created && accountCreated {
			if cleanupErr := deleteNewEmptySubscriptionDiscountAccountTx(tx, input.UserID); cleanupErr != nil {
				return false, cleanupErr
			}
		}
		return created, err
	}
	update := tx.Model(&SubscriptionDiscountAccount{}).
		Where("user_id = ? AND available_usd_minor >= ?", input.UserID, input.USDMinor).
		Updates(map[string]any{
			"available_usd_minor": afterAvailable,
			"reserved_usd_minor":  afterReserved,
			"updated_at":          now,
		})
	if update.Error != nil {
		return false, update.Error
	}
	if update.RowsAffected != 1 {
		return false, ErrSubscriptionDiscountInvalidAccountState
	}
	return true, nil
}

func CommitSubscriptionDiscountTx(tx *gorm.DB, reservationKey string) (bool, error) {
	return closeSubscriptionDiscountReservationTx(tx, reservationKey, SubscriptionDiscountEntryTypeCommit)
}

func ReleaseSubscriptionDiscountTx(tx *gorm.DB, reservationKey string) (bool, error) {
	return closeSubscriptionDiscountReservationTx(tx, reservationKey, SubscriptionDiscountEntryTypeRelease)
}

func closeSubscriptionDiscountReservationTx(tx *gorm.DB, reservationKey string, entryType string) (bool, error) {
	reservationKey, err := normalizeSubscriptionDiscountTerminalReservationKey(reservationKey, entryType)
	if err != nil {
		return false, ErrSubscriptionDiscountInvalidReservation
	}
	reservation, err := getSubscriptionDiscountReservationEntryTx(tx, reservationKey)
	if err != nil {
		return false, err
	}
	account, now, _, err := lockSubscriptionDiscountAccountTx(tx, reservation.UserID)
	if err != nil {
		return false, err
	}
	terminalEntryType, err := subscriptionDiscountTerminalMarkerTypeTx(tx, reservationKey)
	if err != nil {
		return false, err
	}
	if terminalEntryType != "" {
		if terminalEntryType == entryType {
			return false, nil
		}
		return false, ErrSubscriptionDiscountInvalidReservation
	}
	amount := reservation.ReservedDeltaUSDMinor
	if amount <= 0 {
		return false, ErrSubscriptionDiscountInvalidReservation
	}

	availableDelta := int64(0)
	if entryType == SubscriptionDiscountEntryTypeRelease {
		availableDelta = amount
	}
	reservedDelta := -amount
	afterAvailable, ok := checkedAddInt64(account.AvailableUSDMinor, availableDelta)
	if !ok {
		return false, ErrSubscriptionDiscountInvalidAccountState
	}
	afterReserved, ok := checkedAddInt64(account.ReservedUSDMinor, reservedDelta)
	if !ok || afterReserved < 0 {
		return false, ErrSubscriptionDiscountInvalidAccountState
	}

	terminalReservationKey := reservationKey
	entry := SubscriptionDiscountEntry{
		UserID:                 reservation.UserID,
		EntryType:              entryType,
		FundingSource:          reservation.FundingSource,
		AvailableDeltaUSDMinor: availableDelta,
		ReservedDeltaUSDMinor:  reservedDelta,
		AvailableAfterUSDMinor: afterAvailable,
		ReservedAfterUSDMinor:  afterReserved,
		SourceType:             entryType,
		SourceKey:              reservationKey,
		OrderID:                reservation.OrderID,
		TradeNo:                reservation.TradeNo,
		PaymentCurrency:        reservation.PaymentCurrency,
		AppliedAmountMinor:     reservation.AppliedAmountMinor,
		PricingSnapshot:        reservation.PricingSnapshot,
		IdempotencyKey:         reservationKey + ":" + entryType,
		TerminalReservationKey: &terminalReservationKey,
		CreatedAt:              now,
	}
	created, err := createSubscriptionDiscountEntryTx(tx, &entry)
	if err != nil || !created {
		if !created {
			terminalEntryType, markerErr := subscriptionDiscountTerminalMarkerTypeTx(tx, reservationKey)
			if markerErr != nil {
				return false, markerErr
			}
			if terminalEntryType == entryType {
				return false, nil
			}
			if terminalEntryType != "" {
				return false, ErrSubscriptionDiscountInvalidReservation
			}
		}
		return created, err
	}
	if account.ReservedUSDMinor < amount {
		return false, ErrSubscriptionDiscountInvalidAccountState
	}
	update := tx.Model(&SubscriptionDiscountAccount{}).
		Where("user_id = ? AND reserved_usd_minor >= ?", reservation.UserID, amount).
		Updates(map[string]any{
			"available_usd_minor": afterAvailable,
			"reserved_usd_minor":  afterReserved,
			"updated_at":          now,
		})
	if update.Error != nil {
		return false, update.Error
	}
	if update.RowsAffected != 1 {
		return false, ErrSubscriptionDiscountInvalidAccountState
	}
	return true, nil
}

func lockSubscriptionDiscountAccountTx(tx *gorm.DB, userID int) (*SubscriptionDiscountAccount, int64, bool, error) {
	now := getDBTimestampTx(tx)
	account := SubscriptionDiscountAccount{
		UserID:            userID,
		AvailableUSDMinor: 0,
		ReservedUSDMinor:  0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if common.UsingSQLite {
		err := tx.Where("user_id = ?", userID).First(&account).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			insert := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&account)
			if insert.Error != nil {
				return nil, 0, false, insert.Error
			}
			if err := tx.Where("user_id = ?", userID).First(&account).Error; err != nil {
				return nil, 0, false, err
			}
			if err := validateSubscriptionDiscountAccount(&account); err != nil {
				return nil, 0, false, err
			}
			return &account, now, insert.RowsAffected > 0, nil
		}
		if err != nil {
			return nil, 0, false, err
		}
		if err := retrySQLiteBusy(func() error {
			return tx.Model(&SubscriptionDiscountAccount{}).
				Where("user_id = ?", userID).
				Update("updated_at", gorm.Expr("updated_at")).Error
		}); err != nil {
			return nil, 0, false, err
		}
		if err := tx.Where("user_id = ?", userID).First(&account).Error; err != nil {
			return nil, 0, false, err
		}
		if err := validateSubscriptionDiscountAccount(&account); err != nil {
			return nil, 0, false, err
		}
		return &account, now, false, nil
	}
	insert := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&account)
	if insert.Error != nil {
		return nil, 0, false, insert.Error
	}
	accountCreated := insert.RowsAffected > 0
	query := tx
	if common.UsingMySQL || common.UsingPostgreSQL {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.Where("user_id = ?", userID).First(&account).Error; err != nil {
		return nil, 0, false, err
	}
	if err := validateSubscriptionDiscountAccount(&account); err != nil {
		return nil, 0, false, err
	}
	return &account, now, accountCreated, nil
}

func createSubscriptionDiscountEntryTx(tx *gorm.DB, entry *SubscriptionDiscountEntry) (bool, error) {
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(entry)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func getSubscriptionDiscountReservationEntryTx(tx *gorm.DB, reservationKey string) (*SubscriptionDiscountEntry, error) {
	var entry SubscriptionDiscountEntry
	err := tx.Where("idempotency_key = ?", reservationKey).First(&entry).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSubscriptionDiscountReservationNotFound
	}
	if err != nil {
		return nil, err
	}
	if entry.EntryType != SubscriptionDiscountEntryTypeReserve || entry.ReservedDeltaUSDMinor <= 0 {
		return nil, ErrSubscriptionDiscountInvalidReservation
	}
	return &entry, nil
}

func subscriptionDiscountIdempotencyExistsTx(tx *gorm.DB, idempotencyKey string) (bool, error) {
	var count int64
	err := tx.Model(&SubscriptionDiscountEntry{}).
		Where("idempotency_key = ?", idempotencyKey).
		Count(&count).Error
	return count > 0, err
}

// ResolveSubscriptionDiscountFundingSourceTx determines the provenance of a
// requested USD-minor discount while the caller's transaction is active.  The
// account and ledger are read under the same transaction so the result can be
// persisted alongside the reservation without trusting a client-supplied
// marker.
func ResolveSubscriptionDiscountFundingSourceTx(tx *gorm.DB, userID int, amountMinor int64) (string, error) {
	return resolveSubscriptionDiscountFundingSourceByUserTx(tx, userID, amountMinor)
}

// ResolveSubscriptionDiscountFundingSourceForOrderTx resolves an order's
// persisted provenance.  New orders carry the source on the order and linked
// reservation; older rows are reconstructed conservatively from their
// snapshot/ledger history.
func ResolveSubscriptionDiscountFundingSourceForOrderTx(tx *gorm.DB, order *SubscriptionOrder) (string, error) {
	return resolveSubscriptionDiscountFundingSourceForOrderTx(tx, order)
}

func resolveSubscriptionDiscountFundingSourceByUserTx(tx *gorm.DB, userID int, amountMinor int64) (string, error) {
	if userID <= 0 {
		return SubscriptionDiscountFundingSourceUnknown, ErrSubscriptionDiscountInvalidAccountState
	}
	if amountMinor < 0 {
		return SubscriptionDiscountFundingSourceUnknown, ErrSubscriptionDiscountInvalidAmount
	}
	if amountMinor <= 0 {
		return SubscriptionDiscountFundingSourceNone, nil
	}
	if tx == nil {
		tx = DB
	}
	if _, err := lockSubscriptionDiscountAccountReadTx(tx, userID); err != nil {
		return SubscriptionDiscountFundingSourceUnknown, err
	}
	source, err := resolveSubscriptionDiscountFundingSourceFromGrantHistoryTx(tx, userID, 0, amountMinor)
	if err != nil {
		return SubscriptionDiscountFundingSourceUnknown, err
	}
	return source, nil
}

func resolveSubscriptionDiscountFundingSourceForOrderTx(tx *gorm.DB, order *SubscriptionOrder) (string, error) {
	if order == nil || order.UserId <= 0 {
		return SubscriptionDiscountFundingSourceUnknown, ErrSubscriptionDiscountInvalidAccountState
	}
	if tx == nil {
		tx = DB
	}

	rawOrderSource := strings.TrimSpace(order.InvitationFundingSource)
	normalizedOrderSource := normalizeSubscriptionDiscountFundingSource(rawOrderSource)
	invalidOrderSource := rawOrderSource != "" && normalizedOrderSource == SubscriptionDiscountFundingSourceUnknown &&
		!strings.EqualFold(rawOrderSource, SubscriptionDiscountFundingSourceUnknown)

	// A linked reservation/terminal row is the strongest source of truth.  It
	// is written inside the discount transaction and therefore cannot be
	// replaced by a client quote after the fact.
	cutoff := order.CompleteTime
	if cutoff <= 0 {
		cutoff = order.CreateTime
	}
	reservationKey := strings.TrimSpace(order.SubscriptionDiscountReservationKey)
	if reservationKey != "" {
		var reservation SubscriptionDiscountEntry
		err := tx.Where("idempotency_key = ?", reservationKey).First(&reservation).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = tx.Where("terminal_reservation_key = ?", reservationKey).First(&reservation).Error
		}
		if err == nil {
			if reservation.UserID != order.UserId || (reservation.OrderID > 0 && order.Id > 0 && reservation.OrderID != order.Id) {
				return SubscriptionDiscountFundingSourceUnknown, nil
			}
			if strings.TrimSpace(reservation.SourceKey) != reservationKey {
				return SubscriptionDiscountFundingSourceUnknown, nil
			}
			if strings.TrimSpace(reservation.TradeNo) != "" && strings.TrimSpace(order.TradeNo) != "" &&
				strings.TrimSpace(reservation.TradeNo) != strings.TrimSpace(order.TradeNo) {
				return SubscriptionDiscountFundingSourceUnknown, nil
			}
			if reservation.EntryType != SubscriptionDiscountEntryTypeReserve &&
				reservation.EntryType != SubscriptionDiscountEntryTypeCommit &&
				reservation.EntryType != SubscriptionDiscountEntryTypeRelease {
				return SubscriptionDiscountFundingSourceUnknown, nil
			}
			// Reserve rows carry a positive delta while commit/release rows carry
			// the corresponding negative terminal delta.  Validate the absolute
			// amount so a forged order cannot reuse a valid reservation for a
			// different discount amount, regardless of which terminal row won.
			if order.SubscriptionDiscountUSDMinor > 0 {
				reservationAmount, ok := absoluteSubscriptionDiscountAmount(reservation.ReservedDeltaUSDMinor)
				if !ok || reservationAmount != order.SubscriptionDiscountUSDMinor {
					return SubscriptionDiscountFundingSourceUnknown, nil
				}
			}
			if order.SubscriptionDiscountAmountMinor > 0 &&
				(reservation.AppliedAmountMinor <= 0 || reservation.AppliedAmountMinor != order.SubscriptionDiscountAmountMinor) {
				return SubscriptionDiscountFundingSourceUnknown, nil
			}
			if strings.TrimSpace(reservation.PaymentCurrency) != "" && strings.TrimSpace(order.PaymentCurrency) != "" &&
				!strings.EqualFold(strings.TrimSpace(reservation.PaymentCurrency), strings.TrimSpace(order.PaymentCurrency)) {
				return SubscriptionDiscountFundingSourceUnknown, nil
			}
			if source, present := subscriptionDiscountFundingSourceFromReservationEntry(reservation); present {
				if source == SubscriptionDiscountFundingSourceNone {
					// A positive reservation cannot legitimately be `none`; treat a
					// contradictory legacy row as unknown rather than rewarding it.
					return SubscriptionDiscountFundingSourceUnknown, nil
				}
				return source, nil
			}
			// A linked positive reservation with no source marker is itself
			// ambiguous.  Do not fall through to a weaker order marker: doing so
			// would let a legacy/manual `invitee` value mask an unknown ledger row.
			reservationAmount, amountKnown := absoluteSubscriptionDiscountAmount(reservation.ReservedDeltaUSDMinor)
			if !amountKnown || reservationAmount > 0 || reservation.AppliedAmountMinor > 0 {
				return SubscriptionDiscountFundingSourceUnknown, nil
			}
			if reservation.CreatedAt > 0 {
				cutoff = reservation.CreatedAt
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return SubscriptionDiscountFundingSourceUnknown, err
		}
	}

	// A snapshot source is useful for old orders whose reservation row was
	// pruned or never written.  `none` is only safe when there is no invitation
	// discount evidence; with a positive discount it is contradictory.
	if snapshotSource, present := subscriptionDiscountFundingSourceFromPricingSnapshot(order.DiscountPricingSnapshot); present {
		// When both fields are present they must agree.  In particular, the
		// schema default `unknown` must not be replaced by a more permissive
		// snapshot value; only an actually empty legacy column may use the
		// snapshot as its fallback.
		if invalidOrderSource || (rawOrderSource != "" && normalizedOrderSource != snapshotSource) {
			return SubscriptionDiscountFundingSourceUnknown, nil
		}
		switch snapshotSource {
		case SubscriptionDiscountFundingSourceNone:
			if subscriptionDiscountOrderHasFundingEvidence(order) {
				return SubscriptionDiscountFundingSourceUnknown, nil
			}
			return SubscriptionDiscountFundingSourceNone, nil
		case SubscriptionDiscountFundingSourceInvitee, SubscriptionDiscountFundingSourceInviter, SubscriptionDiscountFundingSourceMixed:
			return snapshotSource, nil
		default:
			return SubscriptionDiscountFundingSourceUnknown, nil
		}
	}

	// An order column by itself is only an audit hint.  In particular, do not
	// let a forged/manual `invitee` value become the authorization for an
	// upstream reward when the reservation, snapshot, and ledger provide no
	// corroborating evidence.  Explicit unsafe markers remain conservative
	// blocks even when an old order has no other metadata.
	evidence := subscriptionDiscountOrderHasFundingEvidence(order)
	if invalidOrderSource {
		// An invalid non-empty marker is not evidence of a safe source.  This is
		// evaluated only after stronger reservation/snapshot evidence has been
		// checked.
		return SubscriptionDiscountFundingSourceUnknown, nil
	}
	switch normalizedOrderSource {
	case SubscriptionDiscountFundingSourceInviter, SubscriptionDiscountFundingSourceMixed:
		return normalizedOrderSource, nil
	case SubscriptionDiscountFundingSourceNone, SubscriptionDiscountFundingSourceUnknown:
		if !evidence {
			return SubscriptionDiscountFundingSourceNone, nil
		}
	case SubscriptionDiscountFundingSourceInvitee:
		// Continue to the legacy/ledger evidence paths below.  A concrete
		// invitee marker without proof is deliberately not sufficient.
	}
	if !evidence {
		// The only remaining case is a lone concrete `invitee` marker.  Treat it
		// as ambiguous rather than silently converting it into `none`.
		return SubscriptionDiscountFundingSourceUnknown, nil
	}
	// The legacy balance-purchase helper applied only the invitee's own
	// first-subscription discount and never touched the subscription-credit
	// ledger.  Preserve that provable fact for pre-migration rows that have no
	// provenance column yet.
	if (order.PaymentProvider == PaymentProviderBalance || order.PaymentMethod == PaymentMethodBalance) &&
		order.DiscountUSD > 0 && reservationKey == "" {
		return SubscriptionDiscountFundingSourceInvitee, nil
	}

	if _, err := lockSubscriptionDiscountAccountReadTx(tx, order.UserId); err != nil {
		return SubscriptionDiscountFundingSourceUnknown, err
	}

	amountMinor := order.SubscriptionDiscountUSDMinor
	if amountMinor <= 0 && order.DiscountUSD > 0 {
		converted, err := subscriptionDiscountUSDToMinor(order.DiscountUSD)
		if err != nil {
			return SubscriptionDiscountFundingSourceUnknown, nil
		}
		amountMinor = converted
	}
	if amountMinor <= 0 {
		return SubscriptionDiscountFundingSourceUnknown, nil
	}
	if cutoff <= 0 {
		// A positive legacy order without an event timestamp cannot be bounded
		// against later grants.  Fail closed instead of replaying the user's
		// current account as if it predated this order.
		return SubscriptionDiscountFundingSourceUnknown, nil
	}
	// Ledger timestamps are currently Unix seconds.  If any entry for this
	// user shares the order boundary second, the relative order is unknowable;
	// reject the historical fallback rather than allowing a later grant to be
	// mistaken for pre-order credit.
	sameSecond, err := subscriptionDiscountLedgerHasEntriesAtTimestampTx(tx, order.UserId, cutoff)
	if err != nil {
		return SubscriptionDiscountFundingSourceUnknown, err
	}
	if sameSecond {
		return SubscriptionDiscountFundingSourceUnknown, nil
	}

	source, err := resolveSubscriptionDiscountFundingSourceFromGrantHistoryTx(tx, order.UserId, cutoff, amountMinor)
	if err != nil {
		return SubscriptionDiscountFundingSourceUnknown, err
	}
	return source, nil
}

func subscriptionDiscountLedgerHasEntriesAtTimestampTx(tx *gorm.DB, userID int, timestamp int64) (bool, error) {
	if tx == nil || userID <= 0 || timestamp <= 0 {
		return false, nil
	}
	var count int64
	err := tx.Model(&SubscriptionDiscountEntry{}).
		Where("user_id = ? AND created_at = ?", userID, timestamp).
		Count(&count).Error
	return count > 0, err
}

func subscriptionDiscountTerminalMarkerExistsTx(tx *gorm.DB, reservationKey string) (bool, error) {
	entryType, err := subscriptionDiscountTerminalMarkerTypeTx(tx, reservationKey)
	return entryType != "", err
}

func subscriptionDiscountTerminalMarkerTypeTx(tx *gorm.DB, reservationKey string) (string, error) {
	var entry SubscriptionDiscountEntry
	err := tx.Select("entry_type").
		Where("terminal_reservation_key = ?", reservationKey).
		First(&entry).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return entry.EntryType, nil
}

func deleteNewEmptySubscriptionDiscountAccountTx(tx *gorm.DB, userID int) error {
	return tx.Where("user_id = ? AND available_usd_minor = 0 AND reserved_usd_minor = 0", userID).
		Delete(&SubscriptionDiscountAccount{}).Error
}

func isValidSubscriptionDiscountGrantEntryType(entryType string) bool {
	switch entryType {
	case SubscriptionDiscountEntryTypeGrantInvitee, SubscriptionDiscountEntryTypeGrantInviter, SubscriptionDiscountEntryTypeMigration:
		return true
	default:
		return false
	}
}

type normalizedSubscriptionDiscountReservationInput struct {
	idempotencyKey     string
	orderID            int
	tradeNo            string
	paymentCurrency    string
	appliedAmountMinor int64
	fundingSource      string
	expiresAt          int64
}

func normalizeSubscriptionDiscountGrantInput(input SubscriptionDiscountGrantInput) (string, string, string, error) {
	sourceType := strings.TrimSpace(input.SourceType)
	if sourceType == "" || len(sourceType) > subscriptionDiscountMaxSourceTypeLength {
		return "", "", "", ErrSubscriptionDiscountInvalidReservation
	}
	sourceKey, err := normalizeSubscriptionDiscountBusinessKey(input.SourceKey)
	if err != nil {
		return "", "", "", err
	}
	idempotencyKey, err := normalizeSubscriptionDiscountBusinessKey(input.IdempotencyKey)
	if err != nil {
		return "", "", "", err
	}
	return sourceType, sourceKey, idempotencyKey, nil
}

func normalizeSubscriptionDiscountReservationInput(input SubscriptionDiscountReservationInput) (normalizedSubscriptionDiscountReservationInput, error) {
	idempotencyKey, err := normalizeSubscriptionDiscountTerminalReservationKey(input.IdempotencyKey, SubscriptionDiscountEntryTypeRelease)
	if err != nil {
		return normalizedSubscriptionDiscountReservationInput{}, err
	}
	tradeNo := strings.TrimSpace(input.TradeNo)
	if tradeNo == "" || len(tradeNo) > subscriptionDiscountMaxTradeNoLength {
		return normalizedSubscriptionDiscountReservationInput{}, ErrSubscriptionDiscountInvalidReservation
	}
	paymentCurrency, err := normalizeSubscriptionDiscountCurrency(input.PaymentCurrency)
	if err != nil {
		return normalizedSubscriptionDiscountReservationInput{}, err
	}
	if input.OrderID < 0 || input.AppliedAmountMinor < 0 || input.ExpiresAt <= common.GetTimestamp() {
		return normalizedSubscriptionDiscountReservationInput{}, ErrSubscriptionDiscountInvalidReservation
	}
	fundingSource := strings.TrimSpace(input.FundingSource)
	if fundingSource == "" {
		// A reservation without an explicit server-side provenance marker is
		// legacy/ambiguous and must fail closed at reward time.
		fundingSource = SubscriptionDiscountFundingSourceUnknown
	}
	return normalizedSubscriptionDiscountReservationInput{
		idempotencyKey:     idempotencyKey,
		orderID:            input.OrderID,
		tradeNo:            tradeNo,
		paymentCurrency:    paymentCurrency,
		appliedAmountMinor: input.AppliedAmountMinor,
		fundingSource:      normalizeSubscriptionDiscountFundingSource(fundingSource),
		expiresAt:          input.ExpiresAt,
	}, nil
}

func subscriptionDiscountFundingSourceForGrantInput(entryType string, sourceType string) string {
	switch entryType {
	case SubscriptionDiscountEntryTypeGrantInvitee:
		return SubscriptionDiscountFundingSourceInvitee
	case SubscriptionDiscountEntryTypeGrantInviter:
		return SubscriptionDiscountFundingSourceInviter
	case SubscriptionDiscountEntryTypeMigration:
		switch strings.TrimSpace(sourceType) {
		case legacyInvitationValueAffQuotaSourceType, legacyInvitationValueRewardSourceType:
			return SubscriptionDiscountFundingSourceInviter
		default:
			return SubscriptionDiscountFundingSourceUnknown
		}
	default:
		return SubscriptionDiscountFundingSourceUnknown
	}
}

func normalizeSubscriptionDiscountFundingSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "", SubscriptionDiscountFundingSourceNone:
		return SubscriptionDiscountFundingSourceNone
	case SubscriptionDiscountFundingSourceInvitee:
		return SubscriptionDiscountFundingSourceInvitee
	case SubscriptionDiscountFundingSourceInviter:
		return SubscriptionDiscountFundingSourceInviter
	case SubscriptionDiscountFundingSourceMixed:
		return SubscriptionDiscountFundingSourceMixed
	case SubscriptionDiscountFundingSourceUnknown:
		return SubscriptionDiscountFundingSourceUnknown
	default:
		return SubscriptionDiscountFundingSourceUnknown
	}
}

func subscriptionDiscountOrderHasFundingEvidence(order *SubscriptionOrder) bool {
	if order == nil {
		return false
	}
	if order.SubscriptionDiscountUSDMinor > 0 || order.SubscriptionDiscountAmountMinor > 0 {
		return true
	}
	if strings.TrimSpace(order.SubscriptionDiscountReservationKey) != "" {
		return true
	}
	discountKind := strings.TrimSpace(order.DiscountKind)
	if strings.EqualFold(discountKind, "invitation") {
		return true
	}
	// Older balance orders only persisted DiscountUSD.  A positive value is
	// invitation discount evidence even when DiscountKind was not backfilled.
	return order.DiscountUSD > 0 && !strings.EqualFold(discountKind, "recall")
}

func resolveSubscriptionDiscountFundingSourceFromGrantHistoryTx(tx *gorm.DB, userID int, cutoff int64, amountMinor int64) (string, error) {
	if amountMinor <= 0 {
		return SubscriptionDiscountFundingSourceNone, nil
	}
	query := tx.Model(&SubscriptionDiscountEntry{}).
		Where("user_id = ?", userID)
	if cutoff > 0 {
		// Callers that need an order boundary separately reject same-second
		// entries; use a strict bound here so an ambiguous row can never leak
		// into the historical balance replay.
		query = query.Where("created_at < ?", cutoff)
	}
	var entries []SubscriptionDiscountEntry
	if err := query.Order("id ASC").Find(&entries).Error; err != nil {
		return SubscriptionDiscountFundingSourceUnknown, err
	}
	balances := subscriptionDiscountFundingBalances{}
	for i := range entries {
		if err := balances.apply(entries[i]); err != nil {
			return SubscriptionDiscountFundingSourceUnknown, nil
		}
	}
	if cutoff <= 0 {
		account, err := GetSubscriptionDiscountAccountTx(tx, userID)
		if err != nil {
			return SubscriptionDiscountFundingSourceUnknown, err
		}
		if account.AvailableUSDMinor > balances.total {
			unknown, ok := checkedAddInt64(balances.unknown, account.AvailableUSDMinor-balances.total)
			if !ok {
				return SubscriptionDiscountFundingSourceUnknown, nil
			}
			balances.unknown = unknown
			balances.total = account.AvailableUSDMinor
		} else if account.AvailableUSDMinor < balances.total {
			return SubscriptionDiscountFundingSourceUnknown, nil
		}
	}
	return balances.sourceFor(amountMinor), nil
}

type subscriptionDiscountFundingBalances struct {
	invitee int64
	inviter int64
	unknown int64
	total   int64
}

func (b *subscriptionDiscountFundingBalances) sourceFor(amountMinor int64) string {
	if b == nil || amountMinor <= 0 || b.total < amountMinor || b.unknown > 0 {
		return SubscriptionDiscountFundingSourceUnknown
	}
	// Registration credit is consumed before inviter-origin credit.  This is
	// the same deterministic priority used when a new reservation is stamped:
	// a current invitee discount remains eligible for the direct inviter reward
	// even if an older, unrelated inviter lot is also sitting in the account.
	if b.invitee >= amountMinor {
		return SubscriptionDiscountFundingSourceInvitee
	}
	if b.invitee > 0 {
		// The requested amount necessarily crosses into inviter-origin credit.
		return SubscriptionDiscountFundingSourceMixed
	}
	if b.inviter >= amountMinor {
		return SubscriptionDiscountFundingSourceInviter
	}
	return SubscriptionDiscountFundingSourceMixed
}

func (b *subscriptionDiscountFundingBalances) apply(entry SubscriptionDiscountEntry) error {
	if b == nil {
		return ErrSubscriptionDiscountInvalidAccountState
	}
	switch entry.EntryType {
	case SubscriptionDiscountEntryTypeGrantInvitee, SubscriptionDiscountEntryTypeGrantInviter, SubscriptionDiscountEntryTypeMigration:
		if entry.AvailableDeltaUSDMinor < 0 {
			return ErrSubscriptionDiscountInvalidAccountState
		}
		if entry.AvailableDeltaUSDMinor == 0 {
			return nil
		}
		source := subscriptionDiscountFundingSourceFromGrantEntry(&entry)
		if !b.add(source, entry.AvailableDeltaUSDMinor) {
			return ErrSubscriptionDiscountInvalidAccountState
		}
	case SubscriptionDiscountEntryTypeReserve:
		amount := -entry.AvailableDeltaUSDMinor
		if amount <= 0 {
			return ErrSubscriptionDiscountInvalidAccountState
		}
		source, known := subscriptionDiscountFundingSourceFromReservationEntry(entry)
		if !known || source == SubscriptionDiscountFundingSourceMixed || source == SubscriptionDiscountFundingSourceUnknown {
			b.consumeAmbiguous(amount)
			return nil
		}
		if !b.consume(source, amount) {
			b.consumeAmbiguous(amount)
		}
	case SubscriptionDiscountEntryTypeRelease:
		if entry.AvailableDeltaUSDMinor <= 0 {
			return ErrSubscriptionDiscountInvalidAccountState
		}
		source, known := subscriptionDiscountFundingSourceFromReservationEntry(entry)
		if !known || source == SubscriptionDiscountFundingSourceMixed || source == SubscriptionDiscountFundingSourceUnknown {
			if !b.add(SubscriptionDiscountFundingSourceUnknown, entry.AvailableDeltaUSDMinor) {
				return ErrSubscriptionDiscountInvalidAccountState
			}
		} else {
			if !b.add(source, entry.AvailableDeltaUSDMinor) {
				return ErrSubscriptionDiscountInvalidAccountState
			}
		}
	default:
		// Commit and unrelated zero-delta rows do not change available credit.
		if entry.AvailableDeltaUSDMinor > 0 {
			if !b.add(SubscriptionDiscountFundingSourceUnknown, entry.AvailableDeltaUSDMinor) {
				return ErrSubscriptionDiscountInvalidAccountState
			}
		} else if entry.AvailableDeltaUSDMinor < 0 {
			amount, ok := absoluteSubscriptionDiscountAmount(entry.AvailableDeltaUSDMinor)
			if !ok {
				return ErrSubscriptionDiscountInvalidAccountState
			}
			b.consumeAmbiguous(amount)
		}
	}
	return nil
}

func (b *subscriptionDiscountFundingBalances) add(source string, amount int64) bool {
	if amount <= 0 {
		return amount == 0
	}
	var next int64
	var ok bool
	switch source {
	case SubscriptionDiscountFundingSourceInvitee:
		next, ok = checkedAddInt64(b.invitee, amount)
		if !ok {
			return false
		}
		b.invitee = next
	case SubscriptionDiscountFundingSourceInviter:
		next, ok = checkedAddInt64(b.inviter, amount)
		if !ok {
			return false
		}
		b.inviter = next
	default:
		next, ok = checkedAddInt64(b.unknown, amount)
		if !ok {
			return false
		}
		b.unknown = next
	}
	next, ok = checkedAddInt64(b.total, amount)
	if !ok {
		return false
	}
	b.total = next
	return true
}

func (b *subscriptionDiscountFundingBalances) consume(source string, amount int64) bool {
	if amount <= 0 || b.total < amount {
		return false
	}
	switch source {
	case SubscriptionDiscountFundingSourceInvitee:
		if b.invitee < amount {
			return false
		}
		b.invitee -= amount
	case SubscriptionDiscountFundingSourceInviter:
		if b.inviter < amount {
			return false
		}
		b.inviter -= amount
	default:
		return false
	}
	b.total -= amount
	return true
}

func (b *subscriptionDiscountFundingBalances) consumeAmbiguous(amount int64) {
	if amount <= 0 {
		return
	}
	originalTotal := b.total
	if amount >= originalTotal {
		b.invitee, b.inviter, b.unknown, b.total = 0, 0, 0, 0
		if amount > originalTotal {
			b.unknown = amount - originalTotal
			b.total = amount
		}
		return
	}
	b.total -= amount
	b.invitee, b.inviter = 0, 0
	b.unknown = b.total
}

func subscriptionDiscountFundingSourceFromReservationEntry(entry SubscriptionDiscountEntry) (string, bool) {
	raw := strings.TrimSpace(entry.FundingSource)
	if raw != "" {
		// The persisted ledger marker is written under the account lock and is
		// authoritative, including an explicit `unknown` or `none`.  Never let
		// a duplicate JSON snapshot override it: snapshots are audit context,
		// not a second trust boundary.
		return normalizeSubscriptionDiscountFundingSource(raw), true
	}
	if source, present := subscriptionDiscountFundingSourceFromPricingSnapshot(entry.PricingSnapshot); present {
		return source, true
	}
	return "", false
}

func subscriptionDiscountFundingSourceFromPricingSnapshot(raw string) (string, bool) {
	if strings.TrimSpace(raw) == "" {
		return "", false
	}
	var payload map[string]any
	if err := common.Unmarshal([]byte(raw), &payload); err != nil {
		return SubscriptionDiscountFundingSourceUnknown, true
	}
	value, exists := payload["funding_source"]
	if !exists {
		return "", false
	}
	source, ok := value.(string)
	if !ok || strings.TrimSpace(source) == "" {
		return SubscriptionDiscountFundingSourceUnknown, true
	}
	return normalizeSubscriptionDiscountFundingSource(source), true
}

func subscriptionDiscountFundingSourceFromGrantEntry(entry *SubscriptionDiscountEntry) string {
	if entry == nil {
		return SubscriptionDiscountFundingSourceUnknown
	}
	// Grant entry type is an authoritative legacy discriminator.  Existing
	// rows acquire the column default (`unknown`) during AutoMigrate, so do not
	// let that schema backfill erase the meaning of grant_invitee/grant_inviter.
	switch entry.EntryType {
	case SubscriptionDiscountEntryTypeGrantInvitee:
		return SubscriptionDiscountFundingSourceInvitee
	case SubscriptionDiscountEntryTypeGrantInviter:
		return SubscriptionDiscountFundingSourceInviter
	case SubscriptionDiscountEntryTypeMigration:
		switch strings.TrimSpace(entry.SourceType) {
		case legacyInvitationValueAffQuotaSourceType, legacyInvitationValueRewardSourceType:
			return SubscriptionDiscountFundingSourceInviter
		}
		return SubscriptionDiscountFundingSourceUnknown
	}
	return SubscriptionDiscountFundingSourceUnknown
}

func lockSubscriptionDiscountAccountReadTx(tx *gorm.DB, userID int) (*SubscriptionDiscountAccount, error) {
	if tx == nil {
		tx = DB
	}
	var account SubscriptionDiscountAccount
	query := tx.Model(&SubscriptionDiscountAccount{}).Where("user_id = ?", userID)
	if common.UsingMySQL || common.UsingPostgreSQL {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := validateSubscriptionDiscountAccount(&account); err != nil {
		return nil, err
	}
	return &account, nil
}

func normalizeSubscriptionDiscountTerminalReservationKey(reservationKey string, terminalEntryType string) (string, error) {
	key, err := normalizeSubscriptionDiscountBusinessKey(reservationKey)
	if err != nil {
		return "", err
	}
	if len(key)+1+len(terminalEntryType) > subscriptionDiscountMaxTerminalIdempotencyLength {
		return "", ErrSubscriptionDiscountInvalidReservation
	}
	return key, nil
}

func normalizeSubscriptionDiscountBusinessKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > subscriptionDiscountMaxBusinessKeyLength {
		return "", ErrSubscriptionDiscountInvalidReservation
	}
	return key, nil
}

func normalizeSubscriptionDiscountCurrency(currency string) (string, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if len(currency) != 3 {
		return "", ErrSubscriptionDiscountInvalidReservation
	}
	for _, r := range currency {
		if r < 'A' || r > 'Z' {
			return "", ErrSubscriptionDiscountInvalidReservation
		}
	}
	return currency, nil
}

func checkedAddInt64(a int64, b int64) (int64, bool) {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		return 0, false
	}
	return a + b, true
}

func checkedSubInt64(a int64, b int64) (int64, bool) {
	if b == math.MinInt64 {
		return 0, false
	}
	return checkedAddInt64(a, -b)
}

func absoluteSubscriptionDiscountAmount(value int64) (int64, bool) {
	if value == math.MinInt64 {
		return 0, false
	}
	if value < 0 {
		return -value, true
	}
	return value, true
}

func retrySQLiteBusy(fn func() error) error {
	var err error
	for attempt := 0; attempt < 50; attempt++ {
		err = fn()
		if !common.UsingSQLite || !isSQLiteBusyError(err) {
			return err
		}
		time.Sleep(time.Duration(attempt+1) * 5 * time.Millisecond)
	}
	return err
}

func isSQLiteBusyError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") || strings.Contains(message, "sqlite_busy")
}

func validateSubscriptionDiscountAccount(account *SubscriptionDiscountAccount) error {
	if account == nil || account.UserID <= 0 || account.AvailableUSDMinor < 0 || account.ReservedUSDMinor < 0 {
		return ErrSubscriptionDiscountInvalidAccountState
	}
	return nil
}

func validateSubscriptionDiscountPricingSnapshot(snapshot string) error {
	if strings.TrimSpace(snapshot) == "" {
		return nil
	}
	var raw any
	if err := common.Unmarshal([]byte(snapshot), &raw); err != nil {
		return ErrSubscriptionDiscountInvalidReservation
	}
	return nil
}
