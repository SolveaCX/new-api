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

const stripeCheckoutSelectionDigestMaxLength = 64

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

// StripeCheckoutSelectionDigest returns the stable identity stored in the
// checkout revision ledger. Stripe provider idempotency keys intentionally keep
// their readable prefix, but a ledger may still be backed by the historical
// varchar(64) schema, so oversized keys are reduced to a fixed-length hash.
func StripeCheckoutSelectionDigest(prefix string, checkoutRevision int64, selection StripeCheckoutDiscountSelection) (string, error) {
	key, err := StripeCheckoutIdempotencyKey(prefix, checkoutRevision, selection)
	if err != nil {
		return "", err
	}
	if len(key) <= stripeCheckoutSelectionDigestMaxLength {
		return key, nil
	}
	digest := sha256.Sum256([]byte(key))
	return hex.EncodeToString(digest[:]), nil
}
