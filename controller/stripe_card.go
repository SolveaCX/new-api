package controller

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v81"
	stripecustomer "github.com/stripe/stripe-go/v81/customer"
	stripepaymentintent "github.com/stripe/stripe-go/v81/paymentintent"
	stripepaymentmethod "github.com/stripe/stripe-go/v81/paymentmethod"
	stripeprice "github.com/stripe/stripe-go/v81/price"
)

// stripeCardBindReferencePrefix tags the client_reference_id so the webhook can
// distinguish a card-binding setup session from a regular top-up payment.
const stripeCardBindReferencePrefix = "cardbind_"

func ensureStripeKey() error {
	if !strings.HasPrefix(setting.StripeApiSecret, "sk_") && !strings.HasPrefix(setting.StripeApiSecret, "rk_") {
		return fmt.Errorf("无效的Stripe API密钥")
	}
	stripe.Key = setting.StripeApiSecret
	return nil
}

// resolveStripeCurrency returns the lowercase ISO currency to charge in, taken from the
// configured template price; falls back to "usd" when the price is unavailable.
func resolveStripeCurrency() string {
	priceId := strings.TrimSpace(setting.StripePriceId)
	if priceId == "" {
		logger.LogWarn(nil, "Stripe 货币解析：未配置 StripePriceId，回退到 usd（请确认计费货币是否正确）")
		return "usd"
	}
	tp, err := stripeprice.Get(priceId, nil)
	if err != nil || tp == nil || tp.Currency == "" {
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		logger.LogError(nil, fmt.Sprintf("Stripe 货币解析：读取 Price 失败，回退到 usd（可能 live/test Price 配错）price_id=%s error=%q", priceId, errMsg))
		return "usd"
	}
	return strings.ToLower(string(tp.Currency))
}

// ensureStripeCustomerForUser returns the user's Stripe customer id, creating one if needed
// and persisting it back to the user row.
func ensureStripeCustomerForUser(user *model.User) (string, error) {
	customerId := strings.TrimSpace(user.StripeCustomer)
	if customerId != "" {
		return customerId, nil
	}

	params := &stripe.CustomerParams{}
	if strings.TrimSpace(user.Email) != "" {
		params.Email = stripe.String(strings.TrimSpace(user.Email))
	}
	if strings.TrimSpace(user.Username) != "" {
		params.Name = stripe.String(strings.TrimSpace(user.Username))
	}
	customer, err := stripecustomer.New(params)
	if err != nil {
		return "", err
	}
	if customer == nil || strings.TrimSpace(customer.ID) == "" {
		return "", errors.New("Stripe customer is unavailable")
	}
	customerId = strings.TrimSpace(customer.ID)

	if err := model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("stripe_customer", customerId).Error; err != nil {
		logger.LogWarn(nil, fmt.Sprintf("Stripe 绑卡：保存 customer 失败 user_id=%d customer=%s error=%q", user.Id, customerId, err.Error()))
	}
	user.StripeCustomer = customerId
	return customerId, nil
}

// StripeCardStatus is the card-binding status returned to the frontend.
type StripeCardStatus struct {
	CardBound  bool   `json:"card_bound"`
	BonusGiven bool   `json:"bonus_given"`
	Brand      string `json:"brand,omitempty"`
	Last4      string `json:"last4,omitempty"`
}

// GetStripeCardStatus returns whether the current user has a bound card, plus brand/last4
// pulled from Stripe when available.
func GetStripeCardStatus(c *gin.Context) {
	id := c.GetInt("id")
	user, err := model.GetUserById(id, false)
	if err != nil || user == nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}

	status := StripeCardStatus{
		CardBound:  user.StripeCardBound,
		BonusGiven: user.NewUserBonusGiven,
	}

	// Best-effort enrichment: fetch default payment method brand/last4.
	if user.StripeCardBound && strings.TrimSpace(user.StripeCustomer) != "" {
		if err := ensureStripeKey(); err == nil {
			if brand, last4 := fetchDefaultCard(strings.TrimSpace(user.StripeCustomer)); brand != "" {
				status.Brand = brand
				status.Last4 = last4
			}
		}
	}

	common.ApiSuccess(c, status)
}

// fetchDefaultCard returns the brand and last4 of the customer's first card payment method.
// Errors are swallowed (best-effort display only).
func fetchDefaultCard(customerId string) (brand string, last4 string) {
	listParams := &stripe.PaymentMethodListParams{
		Customer: stripe.String(customerId),
		Type:     stripe.String(string(stripe.PaymentMethodTypeCard)),
	}
	listParams.Limit = stripe.Int64(1)
	iter := stripepaymentmethod.List(listParams)
	for iter.Next() {
		pm := iter.PaymentMethod()
		if pm != nil && pm.Card != nil {
			return string(pm.Card.Brand), pm.Card.Last4
		}
	}
	return "", ""
}

// fetchCardFingerprint returns the Stripe fingerprint of the customer's first card, or ""
// with a nil error when the customer genuinely has no saved card. A non-nil error means the
// lookup itself failed (key/network/API) and says nothing about whether a card exists —
// callers deciding card_bound must not treat the two the same. The fingerprint is stable for
// the same physical card across customers/accounts, so it is used for anti-abuse dedup.
func fetchCardFingerprint(customerId string) (string, error) {
	if customerId == "" {
		return "", nil
	}
	if err := ensureStripeKey(); err != nil {
		return "", err
	}
	listParams := &stripe.PaymentMethodListParams{
		Customer: stripe.String(customerId),
		Type:     stripe.String(string(stripe.PaymentMethodTypeCard)),
	}
	listParams.Limit = stripe.Int64(1)
	iter := stripepaymentmethod.List(listParams)
	for iter.Next() {
		pm := iter.PaymentMethod()
		if pm != nil && pm.Card != nil {
			return strings.TrimSpace(pm.Card.Fingerprint), nil
		}
	}
	if err := iter.Err(); err != nil {
		return "", err
	}
	return "", nil
}

// fetchCardCountry returns the ISO issuing country of the card used for a
// payment (analytics: real payment geography for the ops report), or "" when
// unavailable. It reads the charge behind paymentIntentId first — that country
// is recorded on every successful charge regardless of whether the card was
// saved — and only falls back to the customer's saved payment methods (which
// are empty for non-save-card payments). A non-nil error means the lookup
// itself failed.
func fetchCardCountry(paymentIntentId string, customerId string) (string, error) {
	if err := ensureStripeKey(); err != nil {
		return "", err
	}
	if paymentIntentId != "" {
		piParams := &stripe.PaymentIntentParams{}
		piParams.AddExpand("latest_charge")
		pi, err := stripepaymentintent.Get(paymentIntentId, piParams)
		if err != nil {
			// Lookup failure is not "no country": don't fall through to the saved
			// payment methods, which may be a different card than this payment —
			// the best-effort caller simply skips the update.
			return "", err
		}
		if pi != nil && pi.LatestCharge != nil &&
			pi.LatestCharge.PaymentMethodDetails != nil &&
			pi.LatestCharge.PaymentMethodDetails.Card != nil {
			if cc := strings.ToUpper(strings.TrimSpace(pi.LatestCharge.PaymentMethodDetails.Card.Country)); cc != "" {
				return cc, nil
			}
		}
	}
	if customerId == "" {
		return "", nil
	}
	listParams := &stripe.PaymentMethodListParams{
		Customer: stripe.String(customerId),
		Type:     stripe.String(string(stripe.PaymentMethodTypeCard)),
	}
	listParams.Limit = stripe.Int64(1)
	iter := stripepaymentmethod.List(listParams)
	for iter.Next() {
		pm := iter.PaymentMethod()
		if pm != nil && pm.Card != nil {
			return strings.ToUpper(strings.TrimSpace(pm.Card.Country)), nil
		}
	}
	if err := iter.Err(); err != nil {
		return "", err
	}
	return "", nil
}

// RemoveStripeCard detaches the user's saved card(s) and clears the bound flag.
func RemoveStripeCard(c *gin.Context) {
	id := c.GetInt("id")
	user, err := model.GetUserById(id, false)
	if err != nil || user == nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}
	if !user.StripeCardBound || strings.TrimSpace(user.StripeCustomer) == "" {
		if err := model.SetStripeCardUnbound(id); err != nil {
			common.ApiErrorMsg(c, "解绑失败")
			return
		}
		common.ApiSuccess(c, gin.H{"card_bound": false})
		return
	}

	if err := ensureStripeKey(); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	customerId := strings.TrimSpace(user.StripeCustomer)
	listParams := &stripe.PaymentMethodListParams{
		Customer: stripe.String(customerId),
		Type:     stripe.String(string(stripe.PaymentMethodTypeCard)),
	}
	iter := stripepaymentmethod.List(listParams)
	for iter.Next() {
		pm := iter.PaymentMethod()
		if pm == nil {
			continue
		}
		if _, derr := stripepaymentmethod.Detach(pm.ID, nil); derr != nil {
			logger.LogWarn(c.Request.Context(), fmt.Sprintf("Stripe 解绑：detach 失败 user_id=%d pm=%s error=%q", id, pm.ID, derr.Error()))
		}
	}

	if err := model.SetStripeCardUnbound(id); err != nil {
		common.ApiErrorMsg(c, "解绑失败")
		return
	}
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("Stripe 解绑成功 user_id=%d", id))
	common.ApiSuccess(c, gin.H{"card_bound": false})
}

// --- Automatic off-session charging ---

// autoChargeInFlight prevents concurrent auto-charges for the same user on this node
// (e.g. a burst of requests all crossing the threshold at once). Cross-node correctness
// does NOT depend on it: the DB-side episode claim is the authoritative guard (Rule 11).
var autoChargeInFlight sync.Map

// autoChargeCooldownSeconds is the minimum gap between two automatic charges for one user,
// guarding against repeated charges before the credited quota propagates. It is enforced
// authoritatively inside model.ClaimStripeAutoTopUpEpisode (DB-side, cross-node); the
// in-memory autoChargeLastAt map is only a cheap local pre-filter.
const autoChargeCooldownSeconds int64 = 120

// autoChargeLastAt tracks the last auto-charge attempt per user on this node (unix seconds).
var autoChargeLastAt sync.Map

// Stripe seams, replaceable in tests so no test ever hits the network.
var (
	stripeAutoChargeCreatePaymentIntent = func(params *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error) {
		return stripepaymentintent.New(params)
	}
	stripeAutoChargeFindPaymentMethod = findDefaultPaymentMethodId
	stripeAutoChargeResolveCurrency   = resolveStripeCurrency
)

func init() {
	// Register the controller-side implementation with the service hook.
	service.TriggerStripeAutoCharge = performStripeAutoCharge
}

// autoTopUpConfig is the effective auto top-up configuration for one user.
type autoTopUpConfig struct {
	ThresholdUSD int
	AmountUSD    int
	// UserOptIn is true when the config comes from the user's own opt-in setting
	// (definitive card failures then disable that setting), false when it comes from
	// the legacy operator-level StripeAutoCharge* options.
	UserOptIn bool
}

// resolveAutoTopUpConfig returns the effective auto top-up config for the user.
// Precedence: the user's own opt-in setting wins; otherwise the legacy global
// auto-charge config applies when the operator enabled it. An opted-in user whose
// stored values fail validation is treated as disabled (never silently re-priced).
func resolveAutoTopUpConfig(user *model.User) (autoTopUpConfig, bool) {
	if user == nil {
		return autoTopUpConfig{}, false
	}
	userSetting := user.GetSetting()
	if userSetting.AutoTopUpEnabled {
		if setting.StripeAutoTopUpDailyMaxCharges <= 0 {
			return autoTopUpConfig{}, false
		}
		if err := validateAutoTopUpParams(userSetting.AutoTopUpThresholdUSD, userSetting.AutoTopUpAmountUSD); err != nil {
			logger.LogWarn(nil, fmt.Sprintf("Stripe 自动充值：用户配置无效，跳过 user_id=%d error=%q", user.Id, err.Error()))
			return autoTopUpConfig{}, false
		}
		return autoTopUpConfig{
			ThresholdUSD: userSetting.AutoTopUpThresholdUSD,
			AmountUSD:    userSetting.AutoTopUpAmountUSD,
			UserOptIn:    true,
		}, true
	}
	if setting.StripeAutoChargeEnabled {
		return autoTopUpConfig{
			ThresholdUSD: setting.StripeAutoChargeThreshold,
			AmountUSD:    setting.StripeAutoChargeAmount,
		}, true
	}
	return autoTopUpConfig{}, false
}

// isDefinitiveAutoChargeCardFailure reports whether an off-session PaymentIntent error is
// a definitive card problem (declined, expired, requires on-session authentication, ...)
// rather than a transient API/network fault. Definitive failures disable the user's
// opt-in so a dead card is not retried against Stripe on every exhaustion episode.
func isDefinitiveAutoChargeCardFailure(err error) bool {
	var stripeErr *stripe.Error
	if !errors.As(err, &stripeErr) {
		return false
	}
	if stripeErr.Type == stripe.ErrorTypeCard {
		return true
	}
	switch stripeErr.Code {
	case stripe.ErrorCodeAuthenticationRequired,
		stripe.ErrorCodeCardDeclined,
		stripe.ErrorCodeExpiredCard,
		stripe.ErrorCodePaymentIntentAuthenticationFailure:
		return true
	}
	return false
}

// handleStripeAutoChargeFailure records a user-visible failure log and, for definitive
// card failures on the opt-in path, turns the user's auto top-up off (no tight retry
// loops: the failed claim row already consumed a daily slot and arms the cooldown).
func handleStripeAutoChargeFailure(userId int, cfg autoTopUpConfig, reason string, definitive bool) {
	model.RecordStripeAutoChargeFailure(userId, cfg.AmountUSD, reason)
	if !cfg.UserOptIn || !definitive {
		return
	}
	changed, err := model.DisableUserAutoTopUpSetting(userId)
	if err != nil {
		logger.LogError(nil, fmt.Sprintf("Stripe 自动充值：停用用户自动充值失败 user_id=%d error=%q", userId, err.Error()))
		return
	}
	if changed {
		model.RecordLog(userId, model.LogTypeSystem, "自动充值已因扣款失败自动关闭，请检查或更新支付方式后在控制台重新开启。")
		logger.LogWarn(nil, fmt.Sprintf("Stripe 自动充值已自动停用 user_id=%d reason=%q", userId, reason))
	}
}

// performStripeAutoCharge charges the user's bound card off-session for the configured
// amount and credits the resulting quota. It is invoked asynchronously from the relay hot
// path; all failures are logged and left for the next trigger. Off-session charges are
// merchant-initiated transactions and therefore exempt from the 3DS requirement that the
// on-session Checkout flow requests.
//
// Multi-node idempotency (Rule 11): before touching Stripe, the node must win a DB-side
// episode claim (model.ClaimStripeAutoTopUpEpisode) — a pending top-up order whose
// trade_no is deterministic per (user, UTC day, slot) and protected by the trade_no
// unique index, with the daily cap and cooldown evaluated in the same claim. The Stripe
// idempotency key is derived from that order's trade_no, so even a stripe-go internal
// retry cannot double-charge one claim.
func performStripeAutoCharge(userId int) {
	if userId <= 0 {
		return
	}

	// In-flight dedup: only one evaluation per user at a time on this node.
	if _, loaded := autoChargeInFlight.LoadOrStore(userId, true); loaded {
		return
	}
	defer autoChargeInFlight.Delete(userId)

	// Local cooldown pre-filter: skip if this node attempted a charge very recently.
	now := time.Now().Unix()
	if last, ok := autoChargeLastAt.Load(userId); ok {
		if lastAt, ok2 := last.(int64); ok2 && now-lastAt < autoChargeCooldownSeconds {
			return
		}
	}

	user, err := model.GetUserById(userId, false)
	if err != nil || user == nil {
		return
	}
	cfg, enabled := resolveAutoTopUpConfig(user)
	if !enabled {
		return
	}
	if !user.StripeCardBound || strings.TrimSpace(user.StripeCustomer) == "" {
		return
	}

	// Cross-format cooldown pre-filter (also covers legacy-format rows written by nodes
	// still running the previous release during a rolling deploy).
	if recent, err := model.HasRecentStripeAutoCharge(userId, autoChargeCooldownSeconds); err != nil {
		logger.LogWarn(nil, fmt.Sprintf("Stripe 自动充值：查询近期扣费记录失败，跳过本次以防重复扣款 user_id=%d error=%q", userId, err.Error()))
		return
	} else if recent {
		autoChargeLastAt.Store(userId, now)
		return
	}

	// Re-check balance against the threshold with fresh data to avoid racing a manual top-up.
	freshQuota, err := model.GetUserQuota(userId, false)
	if err != nil {
		return
	}
	threshold := cfg.ThresholdUSD * int(common.QuotaPerUnit)
	if threshold <= 0 || freshQuota >= threshold {
		return
	}
	amountUnits := cfg.AmountUSD
	if amountUnits <= 0 {
		return
	}

	if err := ensureStripeKey(); err != nil {
		logger.LogError(nil, fmt.Sprintf("Stripe 自动充值：密钥无效 user_id=%d error=%q", userId, err.Error()))
		return
	}

	// Resolve currency from the configured template price (same source as manual top-up).
	currency := stripeAutoChargeResolveCurrency()
	money := float64(amountUnits) * setting.StripeUnitPrice
	minorAmount, err := stripeMinorUnitAmount(money, currency)
	if err != nil {
		logger.LogError(nil, fmt.Sprintf("Stripe 自动充值：金额换算失败 user_id=%d error=%q", userId, err.Error()))
		return
	}

	dailyCap := setting.StripeAutoTopUpDailyMaxCharges
	if dailyCap <= 0 && !cfg.UserOptIn {
		// The legacy global path predates the cap option; never let a misconfigured cap
		// turn it into an unbounded charger — fall back to the shipped default.
		dailyCap = 2
	}
	day := time.Now().UTC().Format("20060102")
	order, claimed, err := model.ClaimStripeAutoTopUpEpisode(userId, day, dailyCap, autoChargeCooldownSeconds, amountUnits, money)
	if err != nil {
		logger.LogWarn(nil, fmt.Sprintf("Stripe 自动充值：预占订单失败，跳过本次以防重复扣款 user_id=%d error=%q", userId, err.Error()))
		return
	}
	if !claimed {
		// Daily cap reached, cooling down, or a concurrent node owns this episode.
		autoChargeLastAt.Store(userId, now)
		return
	}

	autoChargeLastAt.Store(userId, time.Now().Unix())

	// Find the customer's default card payment method.
	customerId := strings.TrimSpace(user.StripeCustomer)
	paymentMethodId := stripeAutoChargeFindPaymentMethod(customerId)
	if paymentMethodId == "" {
		logger.LogWarn(nil, fmt.Sprintf("Stripe 自动充值：未找到可用支付方式 user_id=%d customer=%s trade_no=%s", userId, customerId, order.TradeNo))
		if err := model.MarkStripeAutoTopUpOrderFailed(order.TradeNo, ""); err != nil {
			logger.LogError(nil, fmt.Sprintf("Stripe 自动充值：标记订单失败状态失败 trade_no=%s error=%q", order.TradeNo, err.Error()))
		}
		// No card on file is definitive for the opt-in path: without a payment method the
		// next episode cannot succeed either.
		handleStripeAutoChargeFailure(userId, cfg, "未找到可用的支付方式", true)
		return
	}

	params := &stripe.PaymentIntentParams{
		Amount:        stripe.Int64(minorAmount),
		Currency:      stripe.String(currency),
		Customer:      stripe.String(customerId),
		PaymentMethod: stripe.String(paymentMethodId),
		Confirm:       stripe.Bool(true),
		OffSession:    stripe.Bool(true),
	}
	params.Metadata = map[string]string{
		"user_id":  strconv.Itoa(userId),
		"purpose":  "auto_topup",
		"trade_no": order.TradeNo,
	}
	// Idempotency key derived from the claimed order id: a stripe-go internal retry (or a
	// replay of this exact claim) can never produce a second charge.
	params.SetIdempotencyKey("autotopup_" + order.TradeNo)

	intent, err := stripeAutoChargeCreatePaymentIntent(params)
	if err != nil {
		logger.LogWarn(nil, fmt.Sprintf("Stripe 自动充值扣款失败 user_id=%d trade_no=%s amount_units=%d error=%q", userId, order.TradeNo, amountUnits, err.Error()))
		if markErr := model.MarkStripeAutoTopUpOrderFailed(order.TradeNo, ""); markErr != nil {
			logger.LogError(nil, fmt.Sprintf("Stripe 自动充值：标记订单失败状态失败 trade_no=%s error=%q", order.TradeNo, markErr.Error()))
		}
		handleStripeAutoChargeFailure(userId, cfg, "扣款被拒绝或需要验证", isDefinitiveAutoChargeCardFailure(err))
		return
	}
	if intent == nil || intent.Status != stripe.PaymentIntentStatusSucceeded {
		status := ""
		gatewayTradeNo := ""
		if intent != nil {
			status = string(intent.Status)
			gatewayTradeNo = intent.ID
		}
		logger.LogWarn(nil, fmt.Sprintf("Stripe 自动充值未成功 user_id=%d trade_no=%s status=%s", userId, order.TradeNo, status))
		if markErr := model.MarkStripeAutoTopUpOrderFailed(order.TradeNo, gatewayTradeNo); markErr != nil {
			logger.LogError(nil, fmt.Sprintf("Stripe 自动充值：标记订单失败状态失败 trade_no=%s error=%q", order.TradeNo, markErr.Error()))
		}
		// An off-session intent that did not reach succeeded (e.g. requires_action) cannot
		// be completed without the cardholder present — definitive for the opt-in path.
		handleStripeAutoChargeFailure(userId, cfg, "扣款未完成", true)
		return
	}

	if err := model.CompleteStripeAutoTopUpOrder(order.TradeNo, intent.ID, common.GetIp()); err != nil {
		// Money was captured but crediting failed. The claim row stays pending (its daily
		// slot and cooldown remain armed, so no re-charge), and the PaymentIntent carries
		// user_id + trade_no metadata for manual reconciliation.
		logger.LogError(nil, fmt.Sprintf("Stripe 自动充值已扣款但额度入账失败 user_id=%d payment_intent=%s trade_no=%s amount_units=%d error=%q", userId, intent.ID, order.TradeNo, amountUnits, err.Error()))
		model.RecordLog(userId, model.LogTypeSystem, fmt.Sprintf(
			"自动充值已成功扣款 $%d，但额度入账失败（支付单号 %s），我们将尽快为您处理，如未到账请联系客服。",
			amountUnits, intent.ID,
		))
		return
	}
	logger.LogInfo(nil, fmt.Sprintf("Stripe 自动充值成功 user_id=%d payment_intent=%s trade_no=%s amount_units=%d money=%.2f", userId, intent.ID, order.TradeNo, amountUnits, money))
}

// findDefaultPaymentMethodId returns the customer's default card payment method id, falling
// back to the first card on file. Preferring invoice_settings.default_payment_method ensures
// a multi-card customer is charged on the card they designated as default.
func findDefaultPaymentMethodId(customerId string) string {
	// Prefer the customer's explicitly designated default payment method.
	if cust, err := stripecustomer.Get(customerId, nil); err == nil && cust != nil {
		if is := cust.InvoiceSettings; is != nil && is.DefaultPaymentMethod != nil {
			if id := strings.TrimSpace(is.DefaultPaymentMethod.ID); id != "" {
				return id
			}
		}
	}

	// Fall back to the first card on file.
	listParams := &stripe.PaymentMethodListParams{
		Customer: stripe.String(customerId),
		Type:     stripe.String(string(stripe.PaymentMethodTypeCard)),
	}
	listParams.Limit = stripe.Int64(1)
	iter := stripepaymentmethod.List(listParams)
	for iter.Next() {
		pm := iter.PaymentMethod()
		if pm != nil && strings.TrimSpace(pm.ID) != "" {
			return pm.ID
		}
	}
	return ""
}
