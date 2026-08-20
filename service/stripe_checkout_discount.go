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
	default:
		selection.Source = StripeCheckoutDiscountNone
		selection.CouponID = ""
		selection.PromotionCodeID = ""
		selection.MaskedCode = ""
		selection.ReplacedSource = ""
	}
	return selection
}

func ApplyStripeCheckoutDiscount(params *stripe.CheckoutSessionParams, selection StripeCheckoutDiscountSelection) {
	if params == nil {
		return
	}
	params.Discounts = nil
	selection = NormalizeStripeCheckoutDiscountSelection(selection)
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
}

func StripeCheckoutIdempotencyKey(prefix string, checkoutRevision int64, selection StripeCheckoutDiscountSelection) string {
	selection = NormalizeStripeCheckoutDiscountSelection(selection)
	identity := strings.Join([]string{
		string(selection.Source),
		selection.CouponID,
		selection.PromotionCodeID,
		string(selection.ReplacedSource),
	}, "\x00")
	digest := sha256.Sum256([]byte(identity))
	return fmt.Sprintf("%s:rev:%d:discount:%s", strings.TrimSpace(prefix), checkoutRevision, hex.EncodeToString(digest[:16]))
}
