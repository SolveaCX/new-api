package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/stripe/stripe-go/v86"
)

type StripeCheckoutDiscountSource string

const (
	StripeCheckoutDiscountNone       StripeCheckoutDiscountSource = "none"
	StripeCheckoutDiscountInvitation StripeCheckoutDiscountSource = "invitation"
	StripeCheckoutDiscountRecall     StripeCheckoutDiscountSource = "recall"
	StripeCheckoutDiscountManual     StripeCheckoutDiscountSource = "manual"
)

type StripeCheckoutDiscountSelection struct {
	Source          StripeCheckoutDiscountSource
	CouponID        string
	PromotionCodeID string
	MaskedCode      string
	ReplacedSource  StripeCheckoutDiscountSource
}

const stripeCheckoutRevisionRequestIDMaxLength = 64

// StripeCheckoutInitialRequestID returns the stable request key used for the
// first revision. Keep the legacy readable form when it fits the database
// column, and compact longer trade numbers into a deterministic key.
func StripeCheckoutInitialRequestID(kind StripeCheckoutPurchaseKind, tradeNo string) string {
	return boundStripeCheckoutRevisionRequestID(fmt.Sprintf("initial:%s:%s", kind, strings.TrimSpace(tradeNo)))
}

// StripeCheckoutRetryRequestID returns a stable bounded key for retrying an
// abandoned initial revision.
func StripeCheckoutRetryRequestID(requestID string, revision int64) string {
	return boundStripeCheckoutRevisionRequestID(fmt.Sprintf("%s:retry:%d", strings.TrimSpace(requestID), revision))
}

func boundStripeCheckoutRevisionRequestID(requestID string) string {
	requestID = strings.TrimSpace(requestID)
	if len(requestID) <= stripeCheckoutRevisionRequestIDMaxLength {
		return requestID
	}
	digest := sha256.Sum256([]byte("stripe-checkout-request-id:v1\x00" + requestID))
	return "initial:" + hex.EncodeToString(digest[:])[:56]
}

func NormalizeStripeCheckoutDiscountSelection(selection StripeCheckoutDiscountSelection) StripeCheckoutDiscountSelection {
	selection.Source = StripeCheckoutDiscountSource(strings.ToLower(strings.TrimSpace(string(selection.Source))))
	selection.CouponID = strings.TrimSpace(selection.CouponID)
	selection.PromotionCodeID = strings.TrimSpace(selection.PromotionCodeID)
	selection.MaskedCode = strings.TrimSpace(selection.MaskedCode)
	selection.ReplacedSource = StripeCheckoutDiscountSource(strings.ToLower(strings.TrimSpace(string(selection.ReplacedSource))))
	switch selection.Source {
	case StripeCheckoutDiscountInvitation:
		selection.PromotionCodeID = ""
	case StripeCheckoutDiscountRecall, StripeCheckoutDiscountManual:
		selection.CouponID = ""
	case StripeCheckoutDiscountNone:
		selection.CouponID = ""
		selection.PromotionCodeID = ""
		selection.MaskedCode = ""
		selection.ReplacedSource = ""
	}
	return selection
}

func ValidateStripeCheckoutDiscountSelection(selection StripeCheckoutDiscountSelection) (StripeCheckoutDiscountSelection, error) {
	selection = NormalizeStripeCheckoutDiscountSelection(selection)
	switch selection.Source {
	case StripeCheckoutDiscountNone:
		return selection, nil
	case StripeCheckoutDiscountInvitation:
		if selection.CouponID == "" {
			return StripeCheckoutDiscountSelection{}, fmt.Errorf("Stripe invitation discount coupon id is required")
		}
	case StripeCheckoutDiscountRecall, StripeCheckoutDiscountManual:
		if selection.PromotionCodeID == "" {
			return StripeCheckoutDiscountSelection{}, fmt.Errorf("Stripe %s discount promotion code id is required", selection.Source)
		}
	default:
		return StripeCheckoutDiscountSelection{}, fmt.Errorf("unknown Stripe checkout discount source %q", selection.Source)
	}
	return selection, nil
}

func ApplyStripeCheckoutDiscount(params *stripe.CheckoutSessionParams, selection StripeCheckoutDiscountSelection) error {
	selection, err := ValidateStripeCheckoutDiscountSelection(selection)
	if err != nil {
		return err
	}
	if params == nil {
		return nil
	}
	params.Discounts = nil
	switch selection.Source {
	case StripeCheckoutDiscountInvitation:
		if selection.CouponID != "" {
			params.Discounts = []*stripe.CheckoutSessionDiscountParams{{Coupon: stripe.String(selection.CouponID)}}
		}
	case StripeCheckoutDiscountRecall, StripeCheckoutDiscountManual:
		if selection.PromotionCodeID != "" {
			params.Discounts = []*stripe.CheckoutSessionDiscountParams{{PromotionCode: stripe.String(selection.PromotionCodeID)}}
		}
	}
	return nil
}

func StripeCheckoutIdempotencyKey(prefix string, checkoutRevision int64, selection StripeCheckoutDiscountSelection) (string, error) {
	selection, err := ValidateStripeCheckoutDiscountSelection(selection)
	if err != nil {
		return "", err
	}
	identity := strings.Join([]string{
		string(selection.Source),
		selection.CouponID,
		selection.PromotionCodeID,
		string(selection.ReplacedSource),
	}, "\x00")
	digest := sha256.Sum256([]byte(identity))
	return fmt.Sprintf("%s:rev:%d:discount:%s", strings.TrimSpace(prefix), checkoutRevision, hex.EncodeToString(digest[:16])), nil
}
