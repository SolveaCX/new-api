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
	StripeCheckoutOrderTopUp        = "topup"
	StripeCheckoutOrderSubscription = "subscription"

	StripeCheckoutRevisionStatePreparing  = "preparing"
	StripeCheckoutRevisionStateActive     = "active"
	StripeCheckoutRevisionStateSuperseded = "superseded"
	StripeCheckoutRevisionStateAbandoned  = "abandoned"
)

var ErrStripeCheckoutRevisionConflict = errors.New("stripe checkout revision conflict")

// StripeCheckoutRevision is the durable server-side history for replaceable
// Stripe Checkout Sessions. Provider client secrets are deliberately excluded:
// callers must retrieve a Session by ProviderSessionId when replaying a request.
type StripeCheckoutRevision struct {
	Id                 int64   `gorm:"primaryKey"`
	OrderType          string  `gorm:"type:varchar(32);not null;uniqueIndex:idx_stripe_checkout_revision;uniqueIndex:idx_stripe_checkout_request"`
	TradeNo            string  `gorm:"type:varchar(128);not null;uniqueIndex:idx_stripe_checkout_revision;uniqueIndex:idx_stripe_checkout_request;index"`
	Revision           int64   `gorm:"not null;uniqueIndex:idx_stripe_checkout_revision"`
	UserId             int     `gorm:"not null;index"`
	RequestId          string  `gorm:"type:varchar(64);not null;uniqueIndex:idx_stripe_checkout_request"`
	SelectionDigest    string  `gorm:"type:varchar(96);not null"`
	State              string  `gorm:"type:varchar(16);not null;index"`
	DiscountSource     string  `gorm:"type:varchar(24);not null"`
	ReplacedSource     string  `gorm:"type:varchar(24);not null;default:''"`
	CouponId           string  `gorm:"type:varchar(128);default:''"`
	PromotionCodeId    string  `gorm:"type:varchar(128);default:''"`
	PromotionCodeMask  string  `gorm:"type:varchar(64);default:''"`
	DiscountPayload    string  `gorm:"type:text"`
	Currency           string  `gorm:"type:varchar(8);not null;default:''"`
	SubtotalMinor      int64   `gorm:"not null;default:0"`
	DiscountMinor      int64   `gorm:"not null;default:0"`
	TotalMinor         int64   `gorm:"not null;default:0"`
	ProviderSessionId  *string `gorm:"type:varchar(128);uniqueIndex"`
	ProviderSessionURL string  `gorm:"type:text"`
	SummaryPayload     string  `gorm:"type:text"`
	CreatedAt          int64   `gorm:"autoCreateTime"`
	UpdatedAt          int64   `gorm:"autoUpdateTime"`
}

type StripeCheckoutRevisionPrepare struct {
	OrderType         string
	TradeNo           string
	UserID            int
	ExpectedRevision  int64
	RequestID         string
	SelectionDigest   string
	DiscountSource    string
	ReplacedSource    string
	CouponID          string
	PromotionCodeID   string
	PromotionCodeMask string
	DiscountPayload   string
	Currency          string
	SubtotalMinor     int64
	DiscountMinor     int64
	TotalMinor        int64
	SummaryPayload    string
}

type StripeCheckoutRevisionCandidate struct {
	RevisionID         int64
	ProviderSessionID  *string
	ProviderSessionURL string
	SummaryPayload     string
}

type StripeCheckoutRevisionActivation struct {
	RevisionID           int64
	ExpectedRevision     int64
	OldProviderSessionID string
}

// PrepareStripeCheckoutRevision reserves the next monotonic ledger revision
// without advancing the owning order. Abandoned revisions remain immutable and
// are skipped; any other unactivated revision fences a competing request. The
// request key makes an exact request replay return the same row.
func PrepareStripeCheckoutRevision(input StripeCheckoutRevisionPrepare) (*StripeCheckoutRevision, bool, error) {
	input.normalize()
	if err := input.validate(); err != nil {
		return nil, false, err
	}

	var prepared StripeCheckoutRevision
	replay := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		currentRevision, err := lockStripeCheckoutOrder(tx, input.OrderType, input.TradeNo, input.UserID)
		if err != nil {
			return err
		}

		existingRequest := tx.Where("order_type = ? AND trade_no = ? AND request_id = ?", input.OrderType, input.TradeNo, input.RequestID).
			Limit(1).Find(&prepared)
		if existingRequest.Error != nil {
			return existingRequest.Error
		}
		if existingRequest.RowsAffected == 1 {
			if prepared.UserId != input.UserID ||
				prepared.SelectionDigest != input.SelectionDigest {
				return ErrStripeCheckoutRevisionConflict
			}
			replay = true
			return nil
		}
		if currentRevision != input.ExpectedRevision {
			return ErrStripeCheckoutRevisionConflict
		}

		nextRevision := currentRevision + 1
		var latest StripeCheckoutRevision
		latestResult := tx.Where("order_type = ? AND trade_no = ?", input.OrderType, input.TradeNo).
			Order("revision DESC").Limit(1).Find(&latest)
		if latestResult.Error != nil {
			return latestResult.Error
		}
		if latestResult.RowsAffected == 1 && latest.Revision > currentRevision {
			if latest.State != StripeCheckoutRevisionStateAbandoned {
				return ErrStripeCheckoutRevisionConflict
			}
			nextRevision = latest.Revision + 1
		}

		prepared = StripeCheckoutRevision{
			OrderType:         input.OrderType,
			TradeNo:           input.TradeNo,
			Revision:          nextRevision,
			UserId:            input.UserID,
			RequestId:         input.RequestID,
			SelectionDigest:   input.SelectionDigest,
			State:             StripeCheckoutRevisionStatePreparing,
			DiscountSource:    input.DiscountSource,
			ReplacedSource:    input.ReplacedSource,
			CouponId:          input.CouponID,
			PromotionCodeId:   input.PromotionCodeID,
			PromotionCodeMask: input.PromotionCodeMask,
			DiscountPayload:   input.DiscountPayload,
			Currency:          input.Currency,
			SubtotalMinor:     input.SubtotalMinor,
			DiscountMinor:     input.DiscountMinor,
			TotalMinor:        input.TotalMinor,
			SummaryPayload:    input.SummaryPayload,
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&prepared)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrStripeCheckoutRevisionConflict
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return &prepared, replay, nil
}

// RecordStripeCheckoutCandidate attaches the provider-safe Session fields to a
// preparing revision. It never stores a Stripe client secret.
func RecordStripeCheckoutCandidate(input StripeCheckoutRevisionCandidate) (*StripeCheckoutRevision, error) {
	if input.RevisionID <= 0 || input.ProviderSessionID == nil || strings.TrimSpace(*input.ProviderSessionID) == "" {
		return nil, fmt.Errorf("invalid Stripe checkout candidate")
	}
	providerSessionID := strings.TrimSpace(*input.ProviderSessionID)
	var stored StripeCheckoutRevision
	providerSessionURL := strings.TrimSpace(input.ProviderSessionURL)
	err := DB.Transaction(func(tx *gorm.DB) error {
		attached := tx.Model(&StripeCheckoutRevision{}).
			Where("id = ? AND state = ? AND provider_session_id IS NULL", input.RevisionID, StripeCheckoutRevisionStatePreparing).
			Updates(map[string]any{
				"provider_session_id":  providerSessionID,
				"provider_session_url": providerSessionURL,
				"summary_payload":      input.SummaryPayload,
			})
		if attached.Error != nil {
			return attached.Error
		}

		lookup := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", input.RevisionID).Limit(1).Find(&stored)
		if lookup.Error != nil {
			return lookup.Error
		}
		if lookup.RowsAffected != 1 {
			return ErrStripeCheckoutRevisionConflict
		}
		if attached.RowsAffected == 1 || stripeCheckoutCandidateAttachmentMatches(stored, providerSessionID, providerSessionURL, input.SummaryPayload) {
			return nil
		}
		return ErrStripeCheckoutRevisionConflict
	})
	if err != nil {
		return nil, err
	}
	return &stored, nil
}

func stripeCheckoutCandidateAttachmentMatches(stored StripeCheckoutRevision, providerSessionID string, providerSessionURL string, summaryPayload string) bool {
	return stored.ProviderSessionId != nil &&
		strings.TrimSpace(*stored.ProviderSessionId) == providerSessionID &&
		stored.ProviderSessionURL == providerSessionURL &&
		stored.SummaryPayload == summaryPayload
}

// ActivateStripeCheckoutRevision atomically advances the order pointer, retires
// the previous active ledger row, and promotes the prepared candidate.
func ActivateStripeCheckoutRevision(input StripeCheckoutRevisionActivation) (*StripeCheckoutRevision, error) {
	input.OldProviderSessionID = strings.TrimSpace(input.OldProviderSessionID)
	if input.RevisionID <= 0 || input.ExpectedRevision < 0 {
		return nil, fmt.Errorf("invalid Stripe checkout activation")
	}

	var active StripeCheckoutRevision
	err := DB.Transaction(func(tx *gorm.DB) error {
		var candidate StripeCheckoutRevision
		if err := tx.First(&candidate, input.RevisionID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStripeCheckoutRevisionConflict
			}
			return err
		}
		if candidate.State != StripeCheckoutRevisionStatePreparing ||
			candidate.Revision <= input.ExpectedRevision ||
			candidate.ProviderSessionId == nil || strings.TrimSpace(*candidate.ProviderSessionId) == "" {
			return ErrStripeCheckoutRevisionConflict
		}

		newProviderSessionID := strings.TrimSpace(*candidate.ProviderSessionId)
		result, err := activateStripeCheckoutOrderPointer(tx, candidate, input.ExpectedRevision, input.OldProviderSessionID, newProviderSessionID)
		if err != nil {
			return err
		}
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrStripeCheckoutRevisionConflict
		}

		superseded := tx.Model(&StripeCheckoutRevision{}).
			Where("order_type = ? AND trade_no = ? AND revision = ? AND state = ?", candidate.OrderType, candidate.TradeNo, input.ExpectedRevision, StripeCheckoutRevisionStateActive).
			Update("state", StripeCheckoutRevisionStateSuperseded)
		if superseded.Error != nil {
			return superseded.Error
		}
		if input.ExpectedRevision > 0 && superseded.RowsAffected != 1 {
			return ErrStripeCheckoutRevisionConflict
		}
		promoted := tx.Model(&StripeCheckoutRevision{}).
			Where("id = ? AND state = ?", candidate.Id, StripeCheckoutRevisionStatePreparing).
			Update("state", StripeCheckoutRevisionStateActive)
		if promoted.Error != nil {
			return promoted.Error
		}
		if promoted.RowsAffected != 1 {
			return ErrStripeCheckoutRevisionConflict
		}
		if err := tx.First(&active, candidate.Id).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &active, nil
}

// AbandonStripeCheckoutRevision is called only after the caller has expired a
// created candidate, or after candidate creation failed. Active and historical
// rows cannot be abandoned.
func AbandonStripeCheckoutRevision(revisionID int64) error {
	if revisionID <= 0 {
		return fmt.Errorf("invalid Stripe checkout revision")
	}
	result := DB.Model(&StripeCheckoutRevision{}).
		Where("id = ? AND state = ?", revisionID, StripeCheckoutRevisionStatePreparing).
		Update("state", StripeCheckoutRevisionStateAbandoned)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrStripeCheckoutRevisionConflict
	}
	return nil
}

func GetStripeCheckoutRevisionByRequestID(orderType string, tradeNo string, requestID string) (*StripeCheckoutRevision, error) {
	orderType = strings.TrimSpace(orderType)
	tradeNo = strings.TrimSpace(tradeNo)
	requestID = strings.TrimSpace(requestID)
	if !validStripeCheckoutOrderType(orderType) || tradeNo == "" || requestID == "" {
		return nil, fmt.Errorf("invalid Stripe checkout revision lookup")
	}
	var revision StripeCheckoutRevision
	if err := DB.Where("order_type = ? AND trade_no = ? AND request_id = ?", orderType, tradeNo, requestID).
		First(&revision).Error; err != nil {
		return nil, err
	}
	return &revision, nil
}

func lockStripeCheckoutOrder(tx *gorm.DB, orderType string, tradeNo string, userID int) (int64, error) {
	switch orderType {
	case StripeCheckoutOrderTopUp:
		var order TopUp
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, ErrStripeCheckoutRevisionConflict
			}
			return 0, err
		}
		if order.UserId != userID || order.Status != common.TopUpStatusPending {
			return 0, ErrStripeCheckoutRevisionConflict
		}
		return order.CheckoutRevision, nil
	case StripeCheckoutOrderSubscription:
		var order SubscriptionOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, ErrStripeCheckoutRevisionConflict
			}
			return 0, err
		}
		if order.UserId != userID || order.Status != common.TopUpStatusPending {
			return 0, ErrStripeCheckoutRevisionConflict
		}
		return order.CheckoutRevision, nil
	default:
		return 0, fmt.Errorf("unsupported Stripe checkout order type %q", orderType)
	}
}

func activateStripeCheckoutOrderPointer(tx *gorm.DB, candidate StripeCheckoutRevision, expectedRevision int64, oldProviderSessionID string, newProviderSessionID string) (*gorm.DB, error) {
	switch candidate.OrderType {
	case StripeCheckoutOrderTopUp:
		return tx.Model(&TopUp{}).
			Where("trade_no = ? AND user_id = ? AND status = ? AND checkout_revision = ? AND gateway_trade_no = ?", candidate.TradeNo, candidate.UserId, common.TopUpStatusPending, expectedRevision, oldProviderSessionID).
			Updates(map[string]any{"checkout_revision": candidate.Revision, "gateway_trade_no": newProviderSessionID}), nil
	case StripeCheckoutOrderSubscription:
		return tx.Model(&SubscriptionOrder{}).
			Where("trade_no = ? AND user_id = ? AND status = ? AND checkout_revision = ? AND provider_session_id = ?", candidate.TradeNo, candidate.UserId, common.TopUpStatusPending, expectedRevision, oldProviderSessionID).
			Updates(map[string]any{
				"checkout_revision":    candidate.Revision,
				"provider_session_id":  newProviderSessionID,
				"provider_session_url": candidate.ProviderSessionURL,
			}), nil
	default:
		return nil, fmt.Errorf("unsupported Stripe checkout order type %q", candidate.OrderType)
	}
}

func validStripeCheckoutOrderType(orderType string) bool {
	return orderType == StripeCheckoutOrderTopUp || orderType == StripeCheckoutOrderSubscription
}

func (input *StripeCheckoutRevisionPrepare) normalize() {
	input.OrderType = strings.TrimSpace(input.OrderType)
	input.TradeNo = strings.TrimSpace(input.TradeNo)
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.SelectionDigest = strings.TrimSpace(input.SelectionDigest)
	input.DiscountSource = strings.TrimSpace(input.DiscountSource)
	input.ReplacedSource = strings.TrimSpace(input.ReplacedSource)
	input.CouponID = strings.TrimSpace(input.CouponID)
	input.PromotionCodeID = strings.TrimSpace(input.PromotionCodeID)
	input.PromotionCodeMask = strings.TrimSpace(input.PromotionCodeMask)
	input.Currency = strings.TrimSpace(input.Currency)
}

func (input StripeCheckoutRevisionPrepare) validate() error {
	if !validStripeCheckoutOrderType(input.OrderType) || input.TradeNo == "" || input.UserID <= 0 || input.ExpectedRevision < 0 || input.RequestID == "" || input.SelectionDigest == "" {
		return fmt.Errorf("invalid Stripe checkout revision preparation")
	}
	if input.SubtotalMinor < 0 || input.DiscountMinor < 0 || input.TotalMinor < 0 {
		return fmt.Errorf("Stripe checkout amounts cannot be negative")
	}
	return nil
}
