package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v86"
	stripecharge "github.com/stripe/stripe-go/v86/charge"
	checkoutsession "github.com/stripe/stripe-go/v86/checkout/session"
	stripepaymentintent "github.com/stripe/stripe-go/v86/paymentintent"
)

const (
	stripeCardBackfillDefaultDays = 30
	stripeCardBackfillMaxDays     = 90
	stripeCardBackfillDefaultMax  = 1000
	stripeCardBackfillMaxObjects  = 3000
	stripeCardBackfillTimeout     = 45 * time.Second
)

var stripeCardPaymentIntentChargeFetcher = func(ctx context.Context, paymentIntentId string) (*stripe.Charge, error) {
	if err := ensureStripeKey(); err != nil {
		return nil, err
	}
	params := &stripe.PaymentIntentParams{}
	params.Context = ctx
	params.AddExpand("latest_charge")
	intent, err := stripepaymentintent.Get(paymentIntentId, params)
	if err != nil {
		return nil, err
	}
	if intent == nil || intent.LatestCharge == nil {
		return nil, errors.New("Stripe payment intent has no latest charge")
	}
	return intent.LatestCharge, nil
}

type stripeCardObservationSource struct {
	UserId            int
	TradeNo           string
	CheckoutSessionId string
	PaymentIntentId   string
	StripeCustomerId  string
}

type stripeCardBackfillRequest struct {
	Days       int `json:"days"`
	MaxObjects int `json:"max_objects"`
}

type stripeCardBackfillResult struct {
	Days            int  `json:"days"`
	SessionsScanned int  `json:"sessions_scanned"`
	SessionsCapped  bool `json:"sessions_capped"`
	Scanned         int  `json:"scanned"`
	CardCharges     int  `json:"card_charges"`
	Inserted        int  `json:"inserted"`
	Duplicates      int  `json:"duplicates"`
	Unmatched       int  `json:"unmatched"`
	Skipped         int  `json:"skipped"`
	Failed          int  `json:"failed"`
	MaxObjects      int  `json:"max_objects"`
	Capped          bool `json:"capped"`
}

func stripeCardWalletValues(card *stripe.ChargePaymentMethodDetailsCard) (string, string) {
	if card == nil || card.Wallet == nil {
		return "", ""
	}
	return string(card.Wallet.Type), card.Wallet.DynamicLast4
}

func stripeCardObservationFromCharge(source stripeCardObservationSource, charge *stripe.Charge) *model.StripePaymentCardObservation {
	if charge == nil || !charge.Paid || charge.PaymentMethodDetails == nil {
		return nil
	}
	card := charge.PaymentMethodDetails.Card
	walletType, dynamicLast4 := stripeCardWalletValues(card)
	paymentIntentId := strings.TrimSpace(source.PaymentIntentId)
	if paymentIntentId == "" && charge.PaymentIntent != nil {
		paymentIntentId = strings.TrimSpace(charge.PaymentIntent.ID)
	}
	customerId := strings.TrimSpace(source.StripeCustomerId)
	if customerId == "" && charge.Customer != nil {
		customerId = strings.TrimSpace(charge.Customer.ID)
	}
	observedAt := charge.Created
	if observedAt <= 0 {
		observedAt = common.GetTimestamp()
	}
	observation := &model.StripePaymentCardObservation{
		UserId:            source.UserId,
		TradeNo:           source.TradeNo,
		CheckoutSessionId: source.CheckoutSessionId,
		PaymentIntentId:   paymentIntentId,
		ChargeId:          strings.TrimSpace(charge.ID),
		StripeCustomerId:  customerId,
		PaymentMethodId:   strings.TrimSpace(charge.PaymentMethod),
		PaymentMethodType: string(charge.PaymentMethodDetails.Type),
		WalletType:        walletType,
		DynamicLast4:      dynamicLast4,
		ObservedAt:        observedAt,
	}
	if card != nil {
		observation.Fingerprint = strings.TrimSpace(card.Fingerprint)
		observation.Brand = string(card.Brand)
		observation.Last4 = card.Last4
		observation.ExpMonth = card.ExpMonth
		observation.ExpYear = card.ExpYear
		observation.Country = card.Country
		observation.Funding = string(card.Funding)
	}
	return observation
}

func resolveStripeCardObservationUser(tradeNo, customerId string) (int, error) {
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo != "" {
		if topUp := model.GetTopUpByTradeNo(tradeNo); topUp != nil {
			return topUp.UserId, nil
		}
		if order := model.GetSubscriptionOrderByTradeNo(tradeNo); order != nil {
			return order.UserId, nil
		}
	}
	return model.GetUserIDByStripeCustomer(customerId)
}

func persistStripeCheckoutCardObservation(ctx context.Context, source stripeCardObservationSource) (bool, error) {
	if strings.TrimSpace(source.PaymentIntentId) == "" {
		return false, nil
	}
	if source.UserId <= 0 {
		userId, err := resolveStripeCardObservationUser(source.TradeNo, source.StripeCustomerId)
		if err != nil {
			return false, err
		}
		source.UserId = userId
	}
	charge, err := stripeCardPaymentIntentChargeFetcher(ctx, source.PaymentIntentId)
	if err != nil {
		return false, err
	}
	observation := stripeCardObservationFromCharge(source, charge)
	if observation == nil {
		return false, errors.New("Stripe latest charge has no durable payment method details")
	}
	return model.ObserveStripePaymentCard(observation)
}

func stripeCheckoutCardObservationSource(event stripe.Event) (stripeCardObservationSource, error) {
	var checkoutSession stripe.CheckoutSession
	if event.Data == nil || len(event.Data.Raw) == 0 {
		return stripeCardObservationSource{}, nil
	}
	if err := common.Unmarshal(event.Data.Raw, &checkoutSession); err != nil {
		return stripeCardObservationSource{}, err
	}
	source := stripeCardObservationSource{
		TradeNo:           checkoutSession.ClientReferenceID,
		CheckoutSessionId: checkoutSession.ID,
	}
	if checkoutSession.PaymentIntent != nil {
		source.PaymentIntentId = checkoutSession.PaymentIntent.ID
	}
	if checkoutSession.Customer != nil {
		source.StripeCustomerId = checkoutSession.Customer.ID
	}
	return source, nil
}

// observeStripeCheckoutCardBeforeTopUpBestEffort runs before the top-up transaction so the
// invite-reward grant can atomically see an exact-fingerprint duplicate. Stripe/API/DB failures
// remain unknown and are deliberately allowed: payment fulfillment must never fail closed.
func finalizeStripeCheckoutCardObservation(ctx context.Context, event stripe.Event) error {
	source, err := stripeCheckoutCardObservationSource(event)
	if err != nil {
		return err
	}
	if source.PaymentIntentId == "" {
		return nil
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if _, err := persistStripeCheckoutCardObservation(lookupCtx, source); err != nil {
		return err
	}
	if topUp := model.GetTopUpByTradeNo(source.TradeNo); topUp != nil && topUp.PaymentProvider == model.PaymentProviderStripe {
		return model.TryGrantInviteRewardAfterTopUpSucceeded(topUp.UserId, topUp.Id)
	}
	return nil
}

func persistStripeChargeObservationFromEvent(event stripe.Event) error {
	if event.Data == nil || len(event.Data.Raw) == 0 {
		return errors.New("Stripe charge.succeeded has no event payload")
	}
	var charge stripe.Charge
	if err := common.Unmarshal(event.Data.Raw, &charge); err != nil {
		return err
	}
	source := stripeCardObservationSource{}
	if charge.PaymentIntent != nil {
		source.PaymentIntentId = charge.PaymentIntent.ID
	}
	if charge.Customer != nil {
		source.StripeCustomerId = charge.Customer.ID
		userId, err := model.GetUserIDByStripeCustomer(source.StripeCustomerId)
		if err != nil {
			return err
		}
		source.UserId = userId
	}
	observation := stripeCardObservationFromCharge(source, &charge)
	if observation == nil {
		return errors.New("Stripe charge.succeeded has no durable payment method details")
	}
	_, err := model.ObserveStripePaymentCard(observation)
	return err
}

func scheduleStripeChargeObservation(event stripe.Event) {
	if event.Type != stripe.EventTypeChargeSucceeded {
		return
	}
	if event.Data == nil {
		return
	}
	raw := append([]byte(nil), event.Data.Raw...)
	gopool.Go(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var charge stripe.Charge
		if err := common.Unmarshal(raw, &charge); err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("Stripe charge.succeeded 支付卡观测解析失败 error=%q", err.Error()))
			return
		}
		source, err := lookupStripeCardObservationSource(ctx, &charge)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("Stripe charge.succeeded 支付卡归属查询失败 charge_id=%s error=%q", charge.ID, err.Error()))
			return
		}
		observation := stripeCardObservationFromCharge(source, &charge)
		if observation == nil {
			return
		}
		if _, err := model.ObserveStripePaymentCard(observation); err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("Stripe charge.succeeded 支付卡观测写入失败 charge_id=%s error=%q", charge.ID, err.Error()))
			return
		}
		if topUp := model.GetTopUpByTradeNo(source.TradeNo); topUp != nil && topUp.PaymentProvider == model.PaymentProviderStripe {
			if err := model.TryGrantInviteRewardAfterTopUpSucceeded(topUp.UserId, topUp.Id); err != nil {
				logger.LogWarn(ctx, fmt.Sprintf("Stripe charge.succeeded 邀请奖励结算失败 trade_no=%s error=%q", source.TradeNo, err.Error()))
			}
		}
	})
}

func lookupStripeCardObservationSource(ctx context.Context, charge *stripe.Charge) (stripeCardObservationSource, error) {
	source := stripeCardObservationSource{}
	if charge == nil {
		return source, nil
	}
	if err := ensureStripeKey(); err != nil {
		return source, err
	}
	if charge.PaymentIntent != nil {
		source.PaymentIntentId = charge.PaymentIntent.ID
	}
	if charge.Customer != nil {
		source.StripeCustomerId = charge.Customer.ID
	}
	if source.PaymentIntentId != "" {
		params := &stripe.CheckoutSessionListParams{PaymentIntent: stripe.String(source.PaymentIntentId)}
		params.Context = ctx
		params.Limit = stripe.Int64(1)
		iter := checkoutsession.List(params)
		if iter.Next() {
			session := iter.CheckoutSession()
			if session != nil {
				source.TradeNo = session.ClientReferenceID
				source.CheckoutSessionId = session.ID
			}
		}
		if err := iter.Err(); err != nil {
			return source, err
		}
	}
	userId, err := resolveStripeCardObservationUser(source.TradeNo, source.StripeCustomerId)
	if err != nil {
		return source, err
	}
	source.UserId = userId
	return source, nil
}

func AdminGetStripeCardRiskGroups(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	groups, err := model.GetStripeCardRiskGroups(limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	maskedGroups := make([]gin.H, 0, len(groups))
	for _, group := range groups {
		users := make([]gin.H, 0, len(group.Users))
		for _, user := range group.Users {
			users = append(users, gin.H{
				"user_id":            user.UserId,
				"masked_email":       user.Email,
				"fingerprint_digest": user.FingerprintDigest,
				"observed_at":        user.ObservedAt,
				"reward_decision":    user.RewardDecision,
			})
		}
		maskedGroups = append(maskedGroups, gin.H{
			"match_basis":        group.MatchBasis,
			"fingerprint_digest": group.FingerprintDigest,
			"fingerprint_count":  group.FingerprintCount,
			"user_count":         group.UserCount,
			"charge_count":       group.ChargeCount,
			"brand":              group.Brand,
			"last4":              group.Last4,
			"exp_month":          group.ExpMonth,
			"exp_year":           group.ExpYear,
			"country":            group.Country,
			"funding":            group.Funding,
			"wallet_type":        group.WalletType,
			"dynamic_last4":      group.DynamicLast4,
			"users":              users,
		})
	}
	common.ApiSuccess(c, gin.H{
		"exact_match_enforcement":     "topup_invite_reward_only",
		"suspected_match_enforcement": "manual_review_only",
		"groups":                      maskedGroups,
	})
}

func AdminBackfillStripeCardObservations(c *gin.Context) {
	if strings.TrimSpace(setting.StripeApiSecret) == "" {
		common.ApiError(c, errors.New("stripe api secret is not configured"))
		return
	}
	var request stripeCardBackfillRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if request.Days <= 0 {
		request.Days = stripeCardBackfillDefaultDays
	}
	if request.Days > stripeCardBackfillMaxDays {
		request.Days = stripeCardBackfillMaxDays
	}
	if request.MaxObjects <= 0 {
		request.MaxObjects = stripeCardBackfillDefaultMax
	}
	if request.MaxObjects > stripeCardBackfillMaxObjects {
		request.MaxObjects = stripeCardBackfillMaxObjects
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), stripeCardBackfillTimeout)
	defer cancel()
	result, err := backfillStripeCardObservations(ctx, request)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

func backfillStripeCardObservations(ctx context.Context, request stripeCardBackfillRequest) (*stripeCardBackfillResult, error) {
	if err := ensureStripeKey(); err != nil {
		return nil, err
	}
	leaseKey := "stripe_card_observation_backfill"
	leaseOwner := common.GetUUID()
	now := common.GetTimestamp()
	acquired, err := model.AcquireStripeCardBackfillLease(leaseKey, leaseOwner, now, now+60)
	if err != nil {
		return nil, err
	}
	if !acquired {
		return nil, errors.New("Stripe card observation backfill is already running")
	}
	defer func() {
		if err := model.ReleaseStripeCardBackfillLease(leaseKey, leaseOwner); err != nil {
			common.SysError("release Stripe card backfill lease failed: " + err.Error())
		}
	}()
	result := &stripeCardBackfillResult{Days: request.Days, MaxObjects: request.MaxObjects}
	createdAfter := time.Now().Add(-time.Duration(request.Days) * 24 * time.Hour).Unix()

	// Fetch checkout sessions once and build the PaymentIntent attribution map. A bounded
	// backfill must not issue one Checkout Sessions API request per charge.
	sourcesByPaymentIntent := make(map[string]stripeCardObservationSource)
	// A delayed/async payment can succeed after its Checkout Session was created. Look back
	// one additional day so charges at the start of the requested window still find attribution.
	sessionParams := &stripe.CheckoutSessionListParams{CreatedRange: &stripe.RangeQueryParams{GreaterThanOrEqual: createdAfter - 24*60*60}}
	sessionParams.Context = ctx
	sessionParams.Limit = stripe.Int64(100)
	sessionParams.AddExpand("data.payment_intent")
	sessionParams.AddExpand("data.customer")
	sessionIter := checkoutsession.List(sessionParams)
	for sessionIter.Next() {
		if result.SessionsScanned >= stripeCardBackfillMaxObjects {
			result.SessionsCapped = true
			break
		}
		result.SessionsScanned++
		session := sessionIter.CheckoutSession()
		if session == nil || session.PaymentIntent == nil || strings.TrimSpace(session.PaymentIntent.ID) == "" {
			continue
		}
		source := stripeCardObservationSource{
			TradeNo:           session.ClientReferenceID,
			CheckoutSessionId: session.ID,
			PaymentIntentId:   session.PaymentIntent.ID,
		}
		if session.Customer != nil {
			source.StripeCustomerId = session.Customer.ID
		}
		sourcesByPaymentIntent[source.PaymentIntentId] = source
	}
	if err := sessionIter.Err(); err != nil {
		return nil, err
	}

	params := &stripe.ChargeListParams{CreatedRange: &stripe.RangeQueryParams{GreaterThanOrEqual: createdAfter}}
	params.Context = ctx
	params.Limit = stripe.Int64(100)
	params.AddExpand("data.payment_intent")
	params.AddExpand("data.customer")
	iter := stripecharge.List(params)
	charges := make([]*stripe.Charge, 0, request.MaxObjects)
	for iter.Next() {
		if result.Scanned >= request.MaxObjects {
			result.Capped = true
			break
		}
		result.Scanned++
		charges = append(charges, iter.Charge())
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	sort.Slice(charges, func(i, j int) bool {
		if charges[i] == nil || charges[j] == nil {
			return charges[j] == nil
		}
		if charges[i].Created == charges[j].Created {
			return charges[i].ID < charges[j].ID
		}
		return charges[i].Created < charges[j].Created
	})
	for _, charge := range charges {
		if charge == nil || !charge.Paid || charge.PaymentMethodDetails == nil {
			result.Skipped++
			continue
		}
		if charge.PaymentMethodDetails.Card != nil && strings.TrimSpace(charge.PaymentMethodDetails.Card.Fingerprint) != "" {
			result.CardCharges++
		}
		paymentIntentId := ""
		if charge.PaymentIntent != nil {
			paymentIntentId = strings.TrimSpace(charge.PaymentIntent.ID)
		}
		source := sourcesByPaymentIntent[paymentIntentId]
		if source.PaymentIntentId == "" {
			source.PaymentIntentId = paymentIntentId
		}
		if source.StripeCustomerId == "" && charge.Customer != nil {
			source.StripeCustomerId = strings.TrimSpace(charge.Customer.ID)
		}
		userId, err := resolveStripeCardObservationUser(source.TradeNo, source.StripeCustomerId)
		if err != nil {
			result.Failed++
			continue
		}
		source.UserId = userId
		if source.UserId <= 0 {
			result.Unmatched++
		}
		observation := stripeCardObservationFromCharge(source, charge)
		created, err := model.ObserveStripePaymentCard(observation)
		if err != nil {
			result.Failed++
			continue
		}
		if created {
			result.Inserted++
		} else {
			result.Duplicates++
		}
	}
	return result, nil
}
