package service

import (
	"crypto/hmac"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

var (
	ErrStripeCheckoutContextInvalid = errors.New("stripe checkout context is invalid")
	ErrStripeCheckoutContextExpired = errors.New("stripe checkout context has expired")
)

type StripeCheckoutPurchaseKind string

const (
	StripeCheckoutPurchaseTopUp                 StripeCheckoutPurchaseKind = "topup"
	StripeCheckoutPurchaseRecurringSubscription StripeCheckoutPurchaseKind = "recurring_subscription"
	StripeCheckoutPurchaseOneTimeSubscription   StripeCheckoutPurchaseKind = "one_time_subscription"
)

type StripeCheckoutContextClaims struct {
	UserID       int                        `json:"uid"`
	PurchaseKind StripeCheckoutPurchaseKind `json:"kind"`
	TradeNo      string                     `json:"trade_no"`
	Revision     int64                      `json:"revision"`
	ExpiresAt    int64                      `json:"expires_at"`
}

func SignStripeCheckoutContext(claims StripeCheckoutContextClaims) (string, error) {
	if err := validateStripeCheckoutContextClaims(claims); err != nil {
		return "", err
	}
	payload, err := common.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("%w: encode claims: %v", ErrStripeCheckoutContextInvalid, err)
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	return encodedPayload + "." + common.GenerateHMAC(encodedPayload), nil
}

func VerifyStripeCheckoutContext(token string, now time.Time) (StripeCheckoutContextClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return StripeCheckoutContextClaims{}, ErrStripeCheckoutContextInvalid
	}
	if !hmac.Equal([]byte(common.GenerateHMAC(parts[0])), []byte(parts[1])) {
		return StripeCheckoutContextClaims{}, ErrStripeCheckoutContextInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return StripeCheckoutContextClaims{}, fmt.Errorf("%w: decode payload", ErrStripeCheckoutContextInvalid)
	}
	var claims StripeCheckoutContextClaims
	if err := common.Unmarshal(payload, &claims); err != nil {
		return StripeCheckoutContextClaims{}, fmt.Errorf("%w: decode claims", ErrStripeCheckoutContextInvalid)
	}
	if err := validateStripeCheckoutContextClaims(claims); err != nil {
		return StripeCheckoutContextClaims{}, err
	}
	if claims.ExpiresAt <= now.Unix() {
		return StripeCheckoutContextClaims{}, ErrStripeCheckoutContextExpired
	}
	return claims, nil
}

func validateStripeCheckoutContextClaims(claims StripeCheckoutContextClaims) error {
	if claims.UserID <= 0 || claims.Revision <= 0 || claims.ExpiresAt <= 0 || strings.TrimSpace(claims.TradeNo) == "" {
		return ErrStripeCheckoutContextInvalid
	}
	switch claims.PurchaseKind {
	case StripeCheckoutPurchaseTopUp, StripeCheckoutPurchaseRecurringSubscription, StripeCheckoutPurchaseOneTimeSubscription:
		return nil
	default:
		return ErrStripeCheckoutContextInvalid
	}
}
