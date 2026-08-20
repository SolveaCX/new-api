package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v86"
)

const stripeCheckoutContextTTL = 10 * time.Minute

type stripeCheckoutDiscountRequest struct {
	CheckoutContext  string `json:"checkout_context"`
	ExpectedRevision int64  `json:"expected_revision"`
	RequestID        string `json:"request_id"`
	Action           string `json:"action"`
	PromotionCode    string `json:"promotion_code,omitempty"`
}

type StripeCheckoutDiscountState struct {
	Source              service.StripeCheckoutDiscountSource `json:"source"`
	DisplayName         string                               `json:"display_name,omitempty"`
	PromotionCodeMasked string                               `json:"promotion_code_masked,omitempty"`
	ReplacedSource      service.StripeCheckoutDiscountSource `json:"replaced_source,omitempty"`
}

type StripeTopUpSummary struct {
	PayAmount    float64 `json:"pay_amount"`
	BonusAmount  float64 `json:"bonus_amount"`
	CreditAmount float64 `json:"credit_amount"`
	ShowAmounts  bool    `json:"show_amounts"`
}

type StripeCheckoutRevisionResponse struct {
	ClientSecret     string                      `json:"client_secret"`
	PublishableKey   string                      `json:"publishable_key"`
	FallbackURL      string                      `json:"fallback_url,omitempty"`
	CheckoutContext  string                      `json:"checkout_context"`
	CheckoutRevision int64                       `json:"checkout_revision"`
	DiscountState    StripeCheckoutDiscountState `json:"discount_state"`
	TopUpSummary     *StripeTopUpSummary         `json:"topup_summary,omitempty"`
}

type stripeCheckoutSessionSnapshot struct {
	ID            string
	URL           string
	ClientSecret  string
	Status        string
	PaymentStatus string
}

type stripeCheckoutPurchase struct {
	Kind            service.StripeCheckoutPurchaseKind
	OrderType       string
	TradeNo         string
	UserID          int
	Revision        int64
	OldSessionID    string
	CustomerID      string
	PriceID         string
	ProductID       string
	Currency        string
	SubtotalMinor   int64
	DiscountPayload string
	TopUpSummary    *StripeTopUpSummary
	TopUp           *model.TopUp
	Order           *model.SubscriptionOrder
	User            *model.User
}

type stripeCheckoutDiscountRuntime struct {
	ResolvePromotion func(context.Context, service.StripeCheckoutPromotionQuery) (service.StripeCheckoutResolvedPromotion, error)
	LoadPurchase     func(context.Context, service.StripeCheckoutContextClaims) (stripeCheckoutPurchase, error)
	CreateCandidate  func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error)
	GetSession       func(context.Context, service.StripeCheckoutPurchaseKind, string) (*stripeCheckoutSessionSnapshot, error)
	ExpireSession    func(context.Context, service.StripeCheckoutPurchaseKind, string) (*stripeCheckoutSessionSnapshot, error)
	ActivateRevision func(model.StripeCheckoutRevisionActivation) (*model.StripeCheckoutRevision, error)
}

var currentStripeCheckoutDiscountRuntime = defaultStripeCheckoutDiscountRuntime()

func defaultStripeCheckoutDiscountRuntime() stripeCheckoutDiscountRuntime {
	return stripeCheckoutDiscountRuntime{
		ResolvePromotion: func(ctx context.Context, query service.StripeCheckoutPromotionQuery) (service.StripeCheckoutResolvedPromotion, error) {
			return (service.StripeCheckoutPromotionResolver{}).ResolveManualPromotion(ctx, query)
		},
		LoadPurchase:     loadStripeCheckoutPurchase,
		CreateCandidate:  createStripeCheckoutCandidate,
		GetSession:       getStripeCheckoutSessionSnapshot,
		ExpireSession:    expireStripeCheckoutSessionSnapshot,
		ActivateRevision: model.ActivateStripeCheckoutRevision,
	}
}

func replaceStripeCheckoutDiscountRuntimeForTest(replacement stripeCheckoutDiscountRuntime) func() {
	original := currentStripeCheckoutDiscountRuntime
	updated := currentStripeCheckoutDiscountRuntime
	if replacement.ResolvePromotion != nil {
		updated.ResolvePromotion = replacement.ResolvePromotion
	}
	if replacement.LoadPurchase != nil {
		updated.LoadPurchase = replacement.LoadPurchase
	}
	if replacement.CreateCandidate != nil {
		updated.CreateCandidate = replacement.CreateCandidate
	}
	if replacement.GetSession != nil {
		updated.GetSession = replacement.GetSession
	}
	if replacement.ExpireSession != nil {
		updated.ExpireSession = replacement.ExpireSession
	}
	if replacement.ActivateRevision != nil {
		updated.ActivateRevision = replacement.ActivateRevision
	}
	currentStripeCheckoutDiscountRuntime = updated
	return func() { currentStripeCheckoutDiscountRuntime = original }
}

func UpdateStripeCheckoutDiscount(c *gin.Context) {
	if !setting.StripePromotionCodeEnabled {
		writeStripeCheckoutDiscountError(c, http.StatusNotFound, "stripe_promotion_disabled", nil)
		return
	}
	var request stripeCheckoutDiscountRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeStripeCheckoutDiscountError(c, http.StatusBadRequest, "checkout_request_invalid", nil)
		return
	}
	request.CheckoutContext = strings.TrimSpace(request.CheckoutContext)
	request.RequestID = strings.TrimSpace(request.RequestID)
	request.Action = strings.ToLower(strings.TrimSpace(request.Action))
	request.PromotionCode = strings.TrimSpace(request.PromotionCode)
	if request.CheckoutContext == "" || request.ExpectedRevision <= 0 || request.RequestID == "" || len(request.RequestID) > 64 ||
		(request.Action != "apply" && request.Action != "restore") || (request.Action == "apply" && request.PromotionCode == "") {
		writeStripeCheckoutDiscountError(c, http.StatusBadRequest, "checkout_request_invalid", nil)
		return
	}

	claims, err := service.VerifyStripeCheckoutContext(request.CheckoutContext, time.Now())
	if err != nil {
		code := "checkout_context_invalid"
		if errors.Is(err, service.ErrStripeCheckoutContextExpired) {
			code = "checkout_context_expired"
		}
		writeStripeCheckoutDiscountError(c, http.StatusBadRequest, code, nil)
		return
	}
	if claims.UserID != c.GetInt("id") {
		writeStripeCheckoutDiscountError(c, http.StatusBadRequest, "checkout_context_invalid", nil)
		return
	}
	if claims.Revision != request.ExpectedRevision {
		writeStripeCheckoutConflict(c, claims)
		return
	}

	purchase, err := currentStripeCheckoutDiscountRuntime.LoadPurchase(c.Request.Context(), claims)
	if err != nil || purchase.UserID != claims.UserID || purchase.TradeNo != strings.TrimSpace(claims.TradeNo) || purchase.Kind != claims.PurchaseKind {
		writeStripeCheckoutDiscountError(c, http.StatusBadRequest, "checkout_context_invalid", nil)
		return
	}
	selection, err := resolveStripeCheckoutDiscountSelection(c.Request.Context(), request, purchase)
	if err != nil {
		code := "promotion_code_invalid"
		if errors.Is(err, service.ErrStripePromotionAmbiguous) {
			code = "promotion_code_ambiguous"
		} else if errors.Is(err, service.ErrStripePromotionLookup) {
			code = "checkout_replacement_failed"
		}
		writeStripeCheckoutDiscountError(c, http.StatusBadRequest, code, nil)
		return
	}

	selectionDigest, err := service.StripeCheckoutIdempotencyKey("stripe-checkout-ledger:"+purchase.OrderType+":"+purchase.TradeNo, request.ExpectedRevision+1, selection)
	if err != nil {
		writeStripeCheckoutDiscountError(c, http.StatusBadRequest, "checkout_request_invalid", nil)
		return
	}
	prepared, replay, err := model.PrepareStripeCheckoutRevision(model.StripeCheckoutRevisionPrepare{
		OrderType: purchase.OrderType, TradeNo: purchase.TradeNo, UserID: purchase.UserID,
		ExpectedRevision: request.ExpectedRevision, RequestID: request.RequestID, SelectionDigest: selectionDigest,
		DiscountSource: string(selection.Source), ReplacedSource: string(selection.ReplacedSource), CouponID: selection.CouponID,
		PromotionCodeID: selection.PromotionCodeID, PromotionCodeMask: selection.MaskedCode,
		DiscountPayload: purchase.DiscountPayload, Currency: purchase.Currency, SubtotalMinor: purchase.SubtotalMinor,
		SummaryPayload: stripeCheckoutTopUpSummaryPayload(purchase.TopUpSummary),
	})
	if err != nil {
		if errors.Is(err, model.ErrStripeCheckoutRevisionConflict) {
			writeStripeCheckoutConflict(c, claims)
			return
		}
		writeStripeCheckoutDiscountError(c, http.StatusInternalServerError, "checkout_replacement_failed", nil)
		return
	}
	if prepared.Revision != request.ExpectedRevision+1 {
		writeStripeCheckoutConflict(c, claims)
		return
	}
	if replay {
		reconcileStripeCheckoutDiscountReplay(c, claims, purchase, prepared)
		return
	}

	candidate, err := currentStripeCheckoutDiscountRuntime.CreateCandidate(c.Request.Context(), purchase, prepared.Revision, selection)
	if err != nil || candidate == nil || strings.TrimSpace(candidate.ID) == "" {
		_ = model.AbandonStripeCheckoutRevision(prepared.Id)
		code := "checkout_replacement_failed"
		if request.Action == "apply" {
			code = "promotion_code_ineligible"
		}
		writeStripeCheckoutDiscountError(c, http.StatusBadRequest, code, nil)
		return
	}
	candidate.ID = strings.TrimSpace(candidate.ID)
	stored, err := model.RecordStripeCheckoutCandidate(model.StripeCheckoutRevisionCandidate{
		RevisionID: prepared.Id, ProviderSessionID: &candidate.ID, ProviderSessionURL: candidate.URL,
		SummaryPayload: stripeCheckoutTopUpSummaryPayload(purchase.TopUpSummary),
	})
	if err != nil {
		_, _ = currentStripeCheckoutDiscountRuntime.ExpireSession(c.Request.Context(), purchase.Kind, candidate.ID)
		_ = model.AbandonStripeCheckoutRevision(prepared.Id)
		writeStripeCheckoutDiscountError(c, http.StatusInternalServerError, "checkout_replacement_failed", nil)
		return
	}
	finishStripeCheckoutDiscountTransition(c, claims, purchase, stored, candidate)
}

func resolveStripeCheckoutDiscountSelection(ctx context.Context, request stripeCheckoutDiscountRequest, purchase stripeCheckoutPurchase) (service.StripeCheckoutDiscountSelection, error) {
	if request.Action == "restore" {
		canonical, err := model.GetStripeCheckoutRevision(purchase.OrderType, purchase.TradeNo, 1)
		if err != nil {
			return service.StripeCheckoutDiscountSelection{}, err
		}
		return service.ValidateStripeCheckoutDiscountSelection(stripeCheckoutSelectionFromRevision(canonical))
	}
	productID, err := stripeCheckoutStableProductID(ctx, purchase)
	if err != nil {
		return service.StripeCheckoutDiscountSelection{}, err
	}
	resolved, err := currentStripeCheckoutDiscountRuntime.ResolvePromotion(ctx, service.StripeCheckoutPromotionQuery{
		Code: request.PromotionCode, CustomerID: purchase.CustomerID, ProductID: productID,
		Currency: serviceStripeCurrency(purchase.Currency), Subtotal: purchase.SubtotalMinor,
	})
	if err != nil {
		return service.StripeCheckoutDiscountSelection{}, err
	}
	active, err := model.GetActiveStripeCheckoutRevision(purchase.OrderType, purchase.TradeNo)
	if err != nil {
		return service.StripeCheckoutDiscountSelection{}, err
	}
	return service.ValidateStripeCheckoutDiscountSelection(service.StripeCheckoutDiscountSelection{
		Source: service.StripeCheckoutDiscountManual, PromotionCodeID: resolved.PromotionCodeID,
		MaskedCode: resolved.MaskedCode, ReplacedSource: service.StripeCheckoutDiscountSource(active.DiscountSource),
	})
}

func finishStripeCheckoutDiscountTransition(c *gin.Context, claims service.StripeCheckoutContextClaims, purchase stripeCheckoutPurchase, stored *model.StripeCheckoutRevision, candidate *stripeCheckoutSessionSnapshot) {
	oldSession, err := currentStripeCheckoutDiscountRuntime.GetSession(c.Request.Context(), purchase.Kind, purchase.OldSessionID)
	if err != nil || oldSession == nil {
		writeStripeCheckoutDiscountError(c, http.StatusInternalServerError, "checkout_replacement_failed", nil)
		return
	}
	if stripeCheckoutSessionCompleted(oldSession) {
		if cleanupErr := expireAndAbandonStripeCheckoutCandidate(c.Request.Context(), purchase.Kind, stored, candidate.ID); cleanupErr != nil {
			writeStripeCheckoutDiscountError(c, http.StatusInternalServerError, "checkout_replacement_failed", nil)
			return
		}
		writeStripeCheckoutDiscountError(c, http.StatusConflict, "checkout_already_completed", nil)
		return
	}
	if !stripeCheckoutSessionExpired(oldSession) {
		expired, expireErr := currentStripeCheckoutDiscountRuntime.ExpireSession(c.Request.Context(), purchase.Kind, purchase.OldSessionID)
		if expireErr != nil {
			refreshed, getErr := currentStripeCheckoutDiscountRuntime.GetSession(c.Request.Context(), purchase.Kind, purchase.OldSessionID)
			if getErr == nil && stripeCheckoutSessionCompleted(refreshed) {
				if cleanupErr := expireAndAbandonStripeCheckoutCandidate(c.Request.Context(), purchase.Kind, stored, candidate.ID); cleanupErr != nil {
					writeStripeCheckoutDiscountError(c, http.StatusInternalServerError, "checkout_replacement_failed", nil)
					return
				}
				writeStripeCheckoutDiscountError(c, http.StatusConflict, "checkout_already_completed", nil)
				return
			}
			if getErr != nil || !stripeCheckoutSessionExpired(refreshed) {
				writeStripeCheckoutDiscountError(c, http.StatusInternalServerError, "checkout_replacement_failed", nil)
				return
			}
		} else if stripeCheckoutSessionCompleted(expired) {
			if cleanupErr := expireAndAbandonStripeCheckoutCandidate(c.Request.Context(), purchase.Kind, stored, candidate.ID); cleanupErr != nil {
				writeStripeCheckoutDiscountError(c, http.StatusInternalServerError, "checkout_replacement_failed", nil)
				return
			}
			writeStripeCheckoutDiscountError(c, http.StatusConflict, "checkout_already_completed", nil)
			return
		}
	}

	active, err := currentStripeCheckoutDiscountRuntime.ActivateRevision(model.StripeCheckoutRevisionActivation{
		RevisionID: stored.Id, ExpectedRevision: claims.Revision, OldProviderSessionID: purchase.OldSessionID,
	})
	if err != nil {
		if errors.Is(err, model.ErrStripeCheckoutRevisionConflict) {
			if cleanupErr := expireAndAbandonStripeCheckoutCandidate(c.Request.Context(), purchase.Kind, stored, candidate.ID); cleanupErr != nil {
				writeStripeCheckoutDiscountError(c, http.StatusInternalServerError, "checkout_replacement_failed", nil)
				return
			}
			writeStripeCheckoutConflict(c, claims)
			return
		}
		writeStripeCheckoutDiscountError(c, http.StatusInternalServerError, "checkout_replacement_failed", nil)
		return
	}
	response, err := stripeCheckoutRevisionResponse(claims.PurchaseKind, active, candidate)
	if err != nil {
		writeStripeCheckoutDiscountError(c, http.StatusInternalServerError, "checkout_replacement_failed", nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": response})
}

func reconcileStripeCheckoutDiscountReplay(c *gin.Context, claims service.StripeCheckoutContextClaims, purchase stripeCheckoutPurchase, revision *model.StripeCheckoutRevision) {
	switch revision.State {
	case model.StripeCheckoutRevisionStateActive:
		snapshot, err := currentStripeCheckoutDiscountRuntime.GetSession(c.Request.Context(), purchase.Kind, stripeCheckoutRevisionSessionID(revision))
		if err != nil || snapshot == nil {
			writeStripeCheckoutDiscountError(c, http.StatusInternalServerError, "checkout_replacement_failed", nil)
			return
		}
		response, err := stripeCheckoutRevisionResponse(claims.PurchaseKind, revision, snapshot)
		if err != nil {
			writeStripeCheckoutDiscountError(c, http.StatusInternalServerError, "checkout_replacement_failed", nil)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": response})
	case model.StripeCheckoutRevisionStatePreparing:
		candidateID := stripeCheckoutRevisionSessionID(revision)
		if candidateID == "" {
			writeStripeCheckoutDiscountError(c, http.StatusInternalServerError, "checkout_replacement_failed", nil)
			return
		}
		candidate, candidateErr := currentStripeCheckoutDiscountRuntime.GetSession(c.Request.Context(), purchase.Kind, candidateID)
		oldSession, oldErr := currentStripeCheckoutDiscountRuntime.GetSession(c.Request.Context(), purchase.Kind, purchase.OldSessionID)
		if candidateErr != nil || oldErr != nil || candidate == nil || oldSession == nil {
			writeStripeCheckoutDiscountError(c, http.StatusInternalServerError, "checkout_replacement_failed", nil)
			return
		}
		if stripeCheckoutSessionCompleted(oldSession) {
			if cleanupErr := expireAndAbandonStripeCheckoutCandidate(c.Request.Context(), purchase.Kind, revision, candidateID); cleanupErr != nil {
				writeStripeCheckoutDiscountError(c, http.StatusInternalServerError, "checkout_replacement_failed", nil)
				return
			}
			writeStripeCheckoutDiscountError(c, http.StatusConflict, "checkout_already_completed", nil)
			return
		}
		if !stripeCheckoutSessionExpired(oldSession) {
			if cleanupErr := expireAndAbandonStripeCheckoutCandidate(c.Request.Context(), purchase.Kind, revision, candidateID); cleanupErr != nil {
				writeStripeCheckoutDiscountError(c, http.StatusInternalServerError, "checkout_replacement_failed", nil)
				return
			}
			writeStripeCheckoutConflict(c, claims)
			return
		}
		finishStripeCheckoutDiscountTransition(c, claims, purchase, revision, candidate)
	case model.StripeCheckoutRevisionStateAbandoned:
		writeStripeCheckoutConflict(c, claims)
	default:
		writeStripeCheckoutConflict(c, claims)
	}
}

func stripeCheckoutRevisionResponse(kind service.StripeCheckoutPurchaseKind, revision *model.StripeCheckoutRevision, session *stripeCheckoutSessionSnapshot) (*StripeCheckoutRevisionResponse, error) {
	if revision == nil || session == nil || revision.Revision <= 0 {
		return nil, errors.New("Stripe checkout response is incomplete")
	}
	token, err := service.SignStripeCheckoutContext(service.StripeCheckoutContextClaims{
		UserID: revision.UserId, PurchaseKind: kind, TradeNo: revision.TradeNo,
		Revision: revision.Revision, ExpiresAt: time.Now().Add(stripeCheckoutContextTTL).Unix(),
	})
	if err != nil {
		return nil, err
	}
	response := &StripeCheckoutRevisionResponse{
		ClientSecret: strings.TrimSpace(session.ClientSecret), PublishableKey: strings.TrimSpace(setting.StripePublishableKey),
		FallbackURL: strings.TrimSpace(session.URL), CheckoutContext: token, CheckoutRevision: revision.Revision,
		DiscountState: stripeCheckoutDiscountStateFromRevision(revision),
	}
	if response.ClientSecret == "" {
		response.PublishableKey = ""
	}
	if revision.SummaryPayload != "" {
		var summary StripeTopUpSummary
		if err := common.Unmarshal([]byte(revision.SummaryPayload), &summary); err == nil {
			response.TopUpSummary = &summary
		}
	}
	return response, nil
}

func writeStripeCheckoutConflict(c *gin.Context, claims service.StripeCheckoutContextClaims) {
	orderType := stripeCheckoutOrderType(claims.PurchaseKind)
	var latest *StripeCheckoutRevisionResponse
	if active, err := model.GetActiveStripeCheckoutRevision(orderType, claims.TradeNo); err == nil {
		snapshot, _ := currentStripeCheckoutDiscountRuntime.GetSession(c.Request.Context(), claims.PurchaseKind, stripeCheckoutRevisionSessionID(active))
		if snapshot == nil {
			snapshot = &stripeCheckoutSessionSnapshot{ID: stripeCheckoutRevisionSessionID(active), URL: active.ProviderSessionURL}
		}
		latest, _ = stripeCheckoutRevisionResponse(claims.PurchaseKind, active, snapshot)
	}
	writeStripeCheckoutDiscountError(c, http.StatusConflict, "checkout_revision_conflict", latest)
}

func writeStripeCheckoutDiscountError(c *gin.Context, status int, code string, latest *StripeCheckoutRevisionResponse) {
	envelope := gin.H{"success": false, "message": code, "error_code": code}
	if code == "checkout_revision_conflict" && latest != nil {
		envelope["data"] = latest
	}
	c.JSON(status, envelope)
}

func stripeCheckoutSelectionFromRevision(revision *model.StripeCheckoutRevision) service.StripeCheckoutDiscountSelection {
	if revision == nil {
		return service.StripeCheckoutDiscountSelection{}
	}
	return service.StripeCheckoutDiscountSelection{
		Source: service.StripeCheckoutDiscountSource(revision.DiscountSource), CouponID: revision.CouponId,
		PromotionCodeID: revision.PromotionCodeId, MaskedCode: revision.PromotionCodeMask,
		ReplacedSource: service.StripeCheckoutDiscountSource(revision.ReplacedSource),
	}
}

func stripeCheckoutDiscountStateFromRevision(revision *model.StripeCheckoutRevision) StripeCheckoutDiscountState {
	selection := stripeCheckoutSelectionFromRevision(revision)
	return StripeCheckoutDiscountState{
		Source: selection.Source, DisplayName: selection.MaskedCode,
		PromotionCodeMasked: selection.MaskedCode, ReplacedSource: selection.ReplacedSource,
	}
}

func stripeCheckoutTopUpSummaryPayload(summary *StripeTopUpSummary) string {
	if summary == nil {
		return ""
	}
	payload, err := common.Marshal(summary)
	if err != nil {
		return ""
	}
	return string(payload)
}

func stripeCheckoutOrderType(kind service.StripeCheckoutPurchaseKind) string {
	if kind == service.StripeCheckoutPurchaseTopUp {
		return model.StripeCheckoutOrderTopUp
	}
	return model.StripeCheckoutOrderSubscription
}

func stripeCheckoutRevisionSessionID(revision *model.StripeCheckoutRevision) string {
	if revision == nil || revision.ProviderSessionId == nil {
		return ""
	}
	return strings.TrimSpace(*revision.ProviderSessionId)
}

func stripeCheckoutSessionCompleted(session *stripeCheckoutSessionSnapshot) bool {
	if session == nil {
		return false
	}
	return strings.EqualFold(session.PaymentStatus, "paid") || strings.EqualFold(session.Status, "complete")
}

func stripeCheckoutSessionExpired(session *stripeCheckoutSessionSnapshot) bool {
	return session != nil && strings.EqualFold(session.Status, "expired")
}

func expireAndAbandonStripeCheckoutCandidate(ctx context.Context, kind service.StripeCheckoutPurchaseKind, revision *model.StripeCheckoutRevision, sessionID string) error {
	if strings.TrimSpace(sessionID) != "" {
		expired, err := currentStripeCheckoutDiscountRuntime.ExpireSession(ctx, kind, sessionID)
		if err != nil {
			return err
		}
		if expired != nil && !stripeCheckoutSessionExpired(expired) && !stripeCheckoutSessionCompleted(expired) {
			return errors.New("Stripe checkout candidate did not expire")
		}
	}
	if revision != nil {
		if err := model.AbandonStripeCheckoutRevision(revision.Id); err != nil {
			return err
		}
	}
	return nil
}

func createInitialStripeCheckoutRevision(
	ctx context.Context,
	purchase stripeCheckoutPurchase,
	selection service.StripeCheckoutDiscountSelection,
	summary *StripeTopUpSummary,
	create func(int64) (*stripeCheckoutSessionSnapshot, error),
) (*stripeCheckoutSessionSnapshot, *model.StripeCheckoutRevision, error) {
	selection, err := service.ValidateStripeCheckoutDiscountSelection(selection)
	if err != nil {
		return nil, nil, err
	}
	digest, err := service.StripeCheckoutIdempotencyKey("stripe-checkout-initial:"+purchase.OrderType+":"+purchase.TradeNo, 1, selection)
	if err != nil {
		return nil, nil, err
	}
	prepared, replay, err := model.PrepareStripeCheckoutRevision(model.StripeCheckoutRevisionPrepare{
		OrderType: purchase.OrderType, TradeNo: purchase.TradeNo, UserID: purchase.UserID, ExpectedRevision: 0,
		RequestID: "initial:" + string(purchase.Kind) + ":" + purchase.TradeNo, SelectionDigest: digest,
		DiscountSource: string(selection.Source), ReplacedSource: string(selection.ReplacedSource), CouponID: selection.CouponID,
		PromotionCodeID: selection.PromotionCodeID, PromotionCodeMask: selection.MaskedCode,
		DiscountPayload: purchase.DiscountPayload, Currency: purchase.Currency, SubtotalMinor: purchase.SubtotalMinor, SummaryPayload: stripeCheckoutTopUpSummaryPayload(summary),
	})
	if err != nil {
		return nil, nil, err
	}
	if prepared.Revision != 1 {
		return nil, nil, model.ErrStripeCheckoutRevisionConflict
	}
	if replay {
		if prepared.State != model.StripeCheckoutRevisionStateActive {
			return nil, nil, model.ErrStripeCheckoutRevisionConflict
		}
		snapshot, getErr := currentStripeCheckoutDiscountRuntime.GetSession(ctx, purchase.Kind, stripeCheckoutRevisionSessionID(prepared))
		return snapshot, prepared, getErr
	}
	candidate, err := create(prepared.Revision)
	if err != nil || candidate == nil || strings.TrimSpace(candidate.ID) == "" {
		_ = model.AbandonStripeCheckoutRevision(prepared.Id)
		if err == nil {
			err = errors.New("Stripe checkout candidate is incomplete")
		}
		return nil, nil, err
	}
	candidate.ID = strings.TrimSpace(candidate.ID)
	stored, err := model.RecordStripeCheckoutCandidate(model.StripeCheckoutRevisionCandidate{
		RevisionID: prepared.Id, ProviderSessionID: &candidate.ID, ProviderSessionURL: candidate.URL,
		SummaryPayload: stripeCheckoutTopUpSummaryPayload(summary),
	})
	if err != nil {
		_ = expireAndAbandonStripeCheckoutCandidate(ctx, purchase.Kind, prepared, candidate.ID)
		return nil, nil, err
	}
	active, err := currentStripeCheckoutDiscountRuntime.ActivateRevision(model.StripeCheckoutRevisionActivation{
		RevisionID: stored.Id, ExpectedRevision: 0, OldProviderSessionID: "",
	})
	if err != nil {
		_ = expireAndAbandonStripeCheckoutCandidate(ctx, purchase.Kind, stored, candidate.ID)
		return nil, nil, err
	}
	return candidate, active, nil
}

func serviceStripeCurrency(currency string) stripe.Currency {
	return stripe.Currency(strings.ToLower(strings.TrimSpace(currency)))
}

func loadStripeCheckoutPurchase(ctx context.Context, claims service.StripeCheckoutContextClaims) (stripeCheckoutPurchase, error) {
	tradeNo := strings.TrimSpace(claims.TradeNo)
	user, err := model.GetUserById(claims.UserID, false)
	if err != nil || user == nil {
		return stripeCheckoutPurchase{}, errors.New("Stripe checkout user not found")
	}
	if claims.PurchaseKind == service.StripeCheckoutPurchaseTopUp {
		topUp := model.GetTopUpByTradeNo(tradeNo)
		if topUp == nil || topUp.UserId != claims.UserID || topUp.PaymentProvider != model.PaymentProviderStripe || topUp.Status != common.TopUpStatusPending {
			return stripeCheckoutPurchase{}, errors.New("Stripe top-up checkout not found")
		}
		return stripeCheckoutPurchase{
			Kind: claims.PurchaseKind, OrderType: model.StripeCheckoutOrderTopUp, TradeNo: tradeNo, UserID: topUp.UserId,
			Revision: topUp.CheckoutRevision, OldSessionID: strings.TrimSpace(topUp.GatewayTradeNo), CustomerID: strings.TrimSpace(user.StripeCustomer),
			PriceID: strings.TrimSpace(topUp.PaymentPriceId), Currency: strings.TrimSpace(topUp.PaymentCurrency), SubtotalMinor: topUp.PaymentAmountMinor,
			TopUpSummary: &StripeTopUpSummary{PayAmount: float64(topUp.Amount), BonusAmount: float64(topUp.BonusAmount), CreditAmount: float64(topUp.Amount + topUp.BonusAmount), ShowAmounts: operation_setting.GetQuotaDisplayType() != operation_setting.QuotaDisplayTypeTokens && strings.EqualFold(topUp.PaymentCurrency, "USD")},
			TopUp:        topUp, User: user,
		}, nil
	}
	order := model.GetSubscriptionOrderByTradeNo(tradeNo)
	if order == nil || order.UserId != claims.UserID || order.PaymentProvider != model.PaymentProviderStripe || order.Status != common.TopUpStatusPending {
		return stripeCheckoutPurchase{}, errors.New("Stripe subscription checkout not found")
	}
	purchase := stripeCheckoutPurchase{
		Kind: claims.PurchaseKind, OrderType: model.StripeCheckoutOrderSubscription, TradeNo: tradeNo, UserID: order.UserId,
		Revision: order.CheckoutRevision, OldSessionID: strings.TrimSpace(order.ProviderSessionId), CustomerID: strings.TrimSpace(user.StripeCustomer),
		Currency: strings.TrimSpace(order.PaymentCurrency), SubtotalMinor: order.PaymentAmountMinor, Order: order, User: user,
	}
	if claims.PurchaseKind == service.StripeCheckoutPurchaseRecurringSubscription {
		if !strings.EqualFold(strings.TrimSpace(order.PaymentMethod), model.PaymentMethodStripe) {
			return stripeCheckoutPurchase{}, errors.New("Stripe recurring checkout purchase kind mismatch")
		}
		facts, factsErr := service.StripeSubscriptionCheckoutFactsFromOrder(order)
		if factsErr != nil {
			return stripeCheckoutPurchase{}, factsErr
		}
		purchase.PriceID = facts.PriceID
		purchase.Currency = facts.Currency
		purchase.SubtotalMinor = facts.SubtotalMinor
	} else {
		if claims.PurchaseKind != service.StripeCheckoutPurchaseOneTimeSubscription || !isOneTimePlanStripeMethod(order.PaymentMethod) {
			return stripeCheckoutPurchase{}, errors.New("Stripe one-time checkout purchase kind mismatch")
		}
		quote, quoteErr := oneTimePlanQuoteFromOrder(order)
		if quoteErr != nil {
			return stripeCheckoutPurchase{}, quoteErr
		}
		purchase.Currency = quote.Currency
		purchase.SubtotalMinor = quote.TotalAmountMinor
	}
	return purchase, nil
}

func stripeCheckoutStableProductID(ctx context.Context, purchase stripeCheckoutPurchase) (string, error) {
	if purchase.ProductID != "" {
		return strings.TrimSpace(purchase.ProductID), nil
	}
	if purchase.Kind == service.StripeCheckoutPurchaseOneTimeSubscription {
		return oneTimePlanStableStripeProductForCheckout(ctx, purchase.Order)
	}
	if strings.TrimSpace(purchase.PriceID) == "" {
		return "", errors.New("Stripe checkout Price is missing")
	}
	params := &stripe.PriceParams{}
	params.AddExpand("product")
	price, err := stripePriceGetter(strings.TrimSpace(purchase.PriceID), params)
	if err != nil {
		return "", err
	}
	if price == nil || price.Product == nil || strings.TrimSpace(price.Product.ID) == "" {
		return "", errors.New("Stripe checkout Product is missing")
	}
	return strings.TrimSpace(price.Product.ID), nil
}

func createStripeCheckoutCandidate(ctx context.Context, purchase stripeCheckoutPurchase, revision int64, selection service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
	switch purchase.Kind {
	case service.StripeCheckoutPurchaseTopUp:
		if purchase.TopUp == nil || purchase.User == nil {
			return nil, errors.New("Stripe top-up checkout facts are incomplete")
		}
		checkout := &stripeTopUpCheckout{
			PriceId: purchase.TopUp.PaymentPriceId, Quantity: stripeTopUpLineQuantity, Money: purchase.TopUp.Money,
			PaymentCurrency: purchase.TopUp.PaymentCurrency, AmountMinor: purchase.TopUp.PaymentAmountMinor,
		}
		invoiceRequested := false
		if invoice, invoiceErr := model.GetPaymentInvoiceByTradeNo(purchase.TradeNo); invoiceErr == nil && invoice != nil {
			invoiceRequested = invoice.InvoiceRequested
		}
		var recall *service.RecallCheckoutDiscount
		if selection.Source == service.StripeCheckoutDiscountRecall {
			canonical, canonicalErr := model.GetStripeCheckoutRevision(model.StripeCheckoutOrderTopUp, purchase.TradeNo, 1)
			if canonicalErr != nil {
				return nil, canonicalErr
			}
			if strings.TrimSpace(canonical.DiscountPayload) != "" {
				if decodeErr := common.Unmarshal([]byte(canonical.DiscountPayload), &recall); decodeErr != nil {
					return nil, decodeErr
				}
			}
		}
		created, err := genStripeLink(
			purchase.TradeNo, purchase.CustomerID, purchase.User.Email, checkout, "", "", invoiceRequested, purchase.TopUp.SaveCard,
			service.StripeCheckoutPresentation{RequestedUIMode: "elements", Elements: true},
			stripeCheckoutSubmitMessage(purchase.TopUp.Amount, purchase.TopUp.BonusAmount), revision, selection, recall,
		)
		if err != nil {
			return nil, err
		}
		return stripeCheckoutSnapshotFromStripe(created), nil
	case service.StripeCheckoutPurchaseRecurringSubscription:
		created, err := service.CreateStripeSubscriptionCheckoutCandidate(ctx, purchase.Order, purchase.User, revision, selection)
		if err != nil {
			return nil, err
		}
		return &stripeCheckoutSessionSnapshot{ID: created.ID, URL: created.URL, ClientSecret: created.ClientSecret, Status: "open", PaymentStatus: "unpaid"}, nil
	case service.StripeCheckoutPurchaseOneTimeSubscription:
		created, err := createOneTimeStripeCheckoutSessionForRevision(
			ctx, purchase.Order, purchase.User, revision, selection,
			service.StripeCheckoutPresentation{RequestedUIMode: "elements", Elements: true},
		)
		if err != nil {
			return nil, err
		}
		return stripeCheckoutSnapshotFromOneTime(created), nil
	default:
		return nil, fmt.Errorf("unsupported Stripe checkout purchase kind %q", purchase.Kind)
	}
}

func getStripeCheckoutSessionSnapshot(ctx context.Context, kind service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
	switch kind {
	case service.StripeCheckoutPurchaseTopUp:
		got, err := stripeCheckoutSessionGetter(strings.TrimSpace(sessionID), nil)
		return stripeCheckoutSnapshotFromStripe(got), err
	case service.StripeCheckoutPurchaseRecurringSubscription:
		got, err := service.GetStripeSubscriptionCheckoutSession(ctx, sessionID)
		return stripeCheckoutSnapshotFromStripe(got), err
	case service.StripeCheckoutPurchaseOneTimeSubscription:
		got, err := stripeOneTimeCheckoutSessionGetter(ctx, sessionID)
		return stripeCheckoutSnapshotFromOneTime(got), err
	default:
		return nil, fmt.Errorf("unsupported Stripe checkout purchase kind %q", kind)
	}
}

func expireStripeCheckoutSessionSnapshot(ctx context.Context, kind service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
	switch kind {
	case service.StripeCheckoutPurchaseTopUp:
		expired, err := stripeCheckoutSessionExpirer(strings.TrimSpace(sessionID), nil)
		return stripeCheckoutSnapshotFromStripe(expired), err
	case service.StripeCheckoutPurchaseRecurringSubscription:
		expired, err := service.ExpireStripeSubscriptionCheckoutSession(ctx, sessionID)
		return stripeCheckoutSnapshotFromStripe(expired), err
	case service.StripeCheckoutPurchaseOneTimeSubscription:
		expired, err := stripeOneTimeCheckoutSessionExpirer(ctx, sessionID)
		return stripeCheckoutSnapshotFromOneTime(expired), err
	default:
		return nil, fmt.Errorf("unsupported Stripe checkout purchase kind %q", kind)
	}
}

func stripeCheckoutSnapshotFromStripe(session *stripe.CheckoutSession) *stripeCheckoutSessionSnapshot {
	if session == nil {
		return nil
	}
	return &stripeCheckoutSessionSnapshot{
		ID: strings.TrimSpace(session.ID), URL: strings.TrimSpace(session.URL), ClientSecret: strings.TrimSpace(session.ClientSecret),
		Status: string(session.Status), PaymentStatus: string(session.PaymentStatus),
	}
}

func stripeCheckoutSnapshotFromOneTime(session *oneTimeStripeCheckoutSession) *stripeCheckoutSessionSnapshot {
	if session == nil {
		return nil
	}
	return &stripeCheckoutSessionSnapshot{
		ID: strings.TrimSpace(session.ID), URL: strings.TrimSpace(session.URL), ClientSecret: strings.TrimSpace(session.ClientSecret),
		Status: session.Status, PaymentStatus: session.PaymentStatus,
	}
}
