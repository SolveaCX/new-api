package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
)

func TestParseRecallPaymentReadsCheckoutDiscountShapes(t *testing.T) {
	tests := []struct {
		name            string
		raw             string
		promotionCodeID string
		sessionID       string
		amountTotal     int64
		currency        string
		discountAmount  int64
	}{
		{
			name: "checkout session discounts",
			raw: `{
				"id":"cs_session_discount",
				"amount_total":12345,
				"currency":"usd",
				"discounts":[{"promotion_code":{"id":"promo_session"}}],
				"total_details":{"amount_discount":2345,"breakdown":{"discounts":[]}},
				"metadata":{"recall_campaign_id":"41","recall_recipient_id":"82"}
			}`,
			promotionCodeID: "promo_session",
			sessionID:       "cs_session_discount",
			amountTotal:     12345,
			currency:        "USD",
			discountAmount:  2345,
		},
		{
			name: "total details breakdown discounts",
			raw: `{
				"id":"cs_breakdown_discount",
				"amount_total":8000,
				"currency":"jpy",
				"discounts":[],
				"total_details":{"amount_discount":500,"breakdown":{"discounts":[{"amount":500,"discount":{"id":"di_123","promotion_code":{"id":"promo_breakdown"}}}]}},
				"metadata":{}
			}`,
			promotionCodeID: "promo_breakdown",
			sessionID:       "cs_breakdown_discount",
			amountTotal:     8000,
			currency:        "JPY",
			discountAmount:  500,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event := stripe.Event{ID: "evt_123", Data: &stripe.EventData{Raw: []byte(test.raw)}}

			fact, err := ParseRecallPayment(event, "trade_123", 7)

			require.NoError(t, err)
			require.Equal(t, "evt_123", fact.SourceEventID)
			require.Equal(t, "trade_123", fact.TradeNo)
			require.Equal(t, 7, fact.UserID)
			require.Equal(t, test.promotionCodeID, fact.PromotionCodeID)
			require.Equal(t, test.sessionID, fact.CheckoutSessionID)
			require.Equal(t, test.amountTotal, fact.AmountTotal)
			require.Equal(t, test.currency, fact.Currency)
			require.Equal(t, test.discountAmount, fact.DiscountAmount)
		})
	}
}

func TestParseRecallPaymentStoresAuthoritativeMinorUnits(t *testing.T) {
	event := stripe.Event{ID: "evt_amounts", Data: &stripe.EventData{Raw: []byte(`{
		"id":"cs_amounts",
		"amount_total":12345,
		"currency":"usd",
		"total_details":{"amount_discount":2345,"breakdown":{"discounts":[]}},
		"metadata":{"recall_campaign_id":"41","recall_recipient_id":"82"}
	}`)}}

	fact, err := ParseRecallPayment(event, "trade_amounts", 7)

	require.NoError(t, err)
	require.Equal(t, int64(12345), fact.AmountTotal)
	require.Equal(t, "USD", fact.Currency)
	require.Equal(t, int64(2345), fact.DiscountAmount)
	require.Equal(t, int64(41), fact.ClaimCampaignID)
	require.Equal(t, int64(82), fact.ClaimRecipientID)
}

func TestRecallAttributionClassifiesOwnedPayments(t *testing.T) {
	tests := []struct {
		name               string
		promotionCodeID    string
		claimMetadata      bool
		discountJSON       string
		wantConversionKind string
	}{
		{
			name:               "actual promotion code is direct without claim metadata",
			promotionCodeID:    "promo_owned",
			discountJSON:       `"discounts":[{"promotion_code":{"id":"promo_owned"}}],"total_details":{"amount_discount":250,"breakdown":{"discounts":[]}}`,
			wantConversionKind: model.RecallConversionDirect,
		},
		{
			name:               "valid claim with another discount is assisted",
			promotionCodeID:    "promo_owned",
			claimMetadata:      true,
			discountJSON:       `"discounts":[{"promotion_code":{"id":"promo_other"}}],"total_details":{"amount_discount":250,"breakdown":{"discounts":[]}}`,
			wantConversionKind: model.RecallConversionAssisted,
		},
		{
			name:               "valid claim without discount is no coupon",
			promotionCodeID:    "promo_owned",
			claimMetadata:      true,
			discountJSON:       `"discounts":[],"total_details":{"amount_discount":0,"breakdown":{"discounts":[]}}`,
			wantConversionKind: model.RecallConversionNoCoupon,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupRecallCampaignTestDB(t)
			campaign, recipient := createRecallAttributionRecipient(t, test.promotionCodeID)
			metadata := "{}"
			if test.claimMetadata {
				metadata = fmt.Sprintf(`{"recall_campaign_id":"%d","recall_recipient_id":"%d"}`, campaign.Id, recipient.Id)
			}
			raw := fmt.Sprintf(`{
				"id":"cs_owned","created":1700000100,"amount_total":9750,"currency":"usd",
				%s,"metadata":%s
			}`, test.discountJSON, metadata)
			fact, err := ParseRecallPayment(stripe.Event{ID: "evt_123", Data: &stripe.EventData{Raw: []byte(raw)}}, "trade_first", recipient.UserId)
			require.NoError(t, err)
			service := NewRecallAttributionService(&recallStripeFakeClient{})
			service.now = func() time.Time { return time.Unix(1_700_000_200, 0).UTC() }

			require.NoError(t, service.Attribute(context.Background(), fact))

			stored := model.RecallRecipient{}
			require.NoError(t, model.DB.First(&stored, recipient.Id).Error)
			require.Equal(t, model.RecallRecipientConverted, stored.State)
			require.Equal(t, test.wantConversionKind, stored.ConversionKind)
			require.Equal(t, "trade_first", stored.ConversionTradeNo)
			require.Equal(t, "USD", stored.ConversionCurrency)
			require.Equal(t, int64(9750), stored.ConversionAmount)
			if test.wantConversionKind == model.RecallConversionNoCoupon {
				require.Zero(t, stored.DiscountAmount)
			} else {
				require.Equal(t, int64(250), stored.DiscountAmount)
			}
			var events []model.RecallEvent
			require.NoError(t, model.DB.Find(&events).Error)
			require.Len(t, events, 1)
			require.Equal(t, "evt_123", events[0].SourceEventId)
			require.Equal(t, recipient.Id, events[0].RecipientId)
		})
	}
}

func TestRecallAttributionClickAloneNeverConverts(t *testing.T) {
	setupRecallCampaignTestDB(t)
	campaign, recipient := createRecallAttributionRecipient(t, "promo_owned")
	require.NoError(t, model.DB.Model(&model.RecallRecipient{}).Where("id = ?", recipient.Id).Update("clicked_at", int64(1_700_000_050)).Error)
	fact, err := ParseRecallPayment(stripe.Event{ID: "evt_unowned", Data: &stripe.EventData{Raw: []byte(`{
		"id":"cs_unowned","created":1700000100,"amount_total":1000,"currency":"usd",
		"discounts":[],"total_details":{"amount_discount":0,"breakdown":{"discounts":[]}},"metadata":{}
	}`)}}, "trade_unowned", recipient.UserId)
	require.NoError(t, err)

	require.NoError(t, NewRecallAttributionService(&recallStripeFakeClient{}).Attribute(context.Background(), fact))

	stored := model.RecallRecipient{}
	require.NoError(t, model.DB.First(&stored, recipient.Id).Error)
	require.Zero(t, stored.ConvertedAt)
	require.Equal(t, model.RecallRecipientContacting, stored.State)
	var eventCount int64
	require.NoError(t, model.DB.Model(&model.RecallEvent{}).Where("event_type = ?", "conversion").Count(&eventCount).Error)
	require.Zero(t, eventCount)
	_ = campaign
}

func TestRecallAttributionReplayAndLaterOrderCannotOverwriteFirstConversion(t *testing.T) {
	setupRecallCampaignTestDB(t)
	campaign, recipient := createRecallAttributionRecipient(t, "promo_owned")
	first := RecallPaymentFact{
		SourceEventID: "evt_123", CheckoutSessionID: "cs_first", TradeNo: "trade_first", UserID: recipient.UserId,
		AmountTotal: 1200, Currency: "USD", DiscountAmount: 200, PromotionCodeID: "promo_owned",
		hasDiscount: true, discountDetailsLoaded: true,
	}
	service := NewRecallAttributionService(&recallStripeFakeClient{})
	service.now = func() time.Time { return time.Unix(1_700_000_200, 0).UTC() }

	require.NoError(t, service.Attribute(context.Background(), first))
	require.NoError(t, service.Attribute(context.Background(), first))
	second := first
	second.SourceEventID = "evt_later"
	second.CheckoutSessionID = "cs_later"
	second.TradeNo = "trade_later"
	second.AmountTotal = 9900
	second.DiscountAmount = 900
	require.NoError(t, service.Attribute(context.Background(), second))

	stored := model.RecallRecipient{}
	require.NoError(t, model.DB.First(&stored, recipient.Id).Error)
	require.Equal(t, "trade_first", stored.ConversionTradeNo)
	require.Equal(t, int64(1200), stored.ConversionAmount)
	require.Equal(t, int64(200), stored.DiscountAmount)
	var events []model.RecallEvent
	require.NoError(t, model.DB.Where("recipient_id = ? AND event_type = ?", recipient.Id, "conversion").Find(&events).Error)
	require.Len(t, events, 1)
	require.Equal(t, "evt_123", events[0].SourceEventId)
	_ = campaign
}

func TestRecallAttributionFetchesExactDiscountExpansions(t *testing.T) {
	setupRecallCampaignTestDB(t)
	_, recipient := createRecallAttributionRecipient(t, "promo_expanded")
	fact, err := ParseRecallPayment(stripe.Event{ID: "evt_expand", Data: &stripe.EventData{Raw: []byte(`{
		"id":"cs_expand","amount_total":7500,"currency":"usd",
		"discounts":[{"coupon":{"id":"coupon_unexpanded"}}],
		"total_details":{"amount_discount":500},"metadata":{}
	}`)}}, "trade_expand", recipient.UserId)
	require.NoError(t, err)
	var gotExpansions []string
	client := &recallStripeFakeClient{getCheckoutSessionFn: func(_ context.Context, id string, expand ...string) (*stripe.CheckoutSession, error) {
		require.Equal(t, "cs_expand", id)
		gotExpansions = append([]string(nil), expand...)
		return &stripe.CheckoutSession{
			ID: "cs_expand", AmountTotal: 7500, Currency: stripe.CurrencyUSD,
			Discounts:    []*stripe.CheckoutSessionDiscount{{PromotionCode: &stripe.PromotionCode{ID: "promo_expanded"}}},
			TotalDetails: &stripe.CheckoutSessionTotalDetails{AmountDiscount: 500},
		}, nil
	}}

	require.NoError(t, NewRecallAttributionService(client).Attribute(context.Background(), fact))

	require.Equal(t, []string{
		"discounts.promotion_code",
		"total_details.breakdown.discounts.discount.promotion_code",
	}, gotExpansions)
	stored := model.RecallRecipient{}
	require.NoError(t, model.DB.First(&stored, recipient.Id).Error)
	require.Equal(t, model.RecallConversionDirect, stored.ConversionKind)
}

func TestRecallAttributionHydratesOmittedDiscountFieldsBeforeClassifyingClaim(t *testing.T) {
	setupRecallCampaignTestDB(t)
	campaign, recipient := createRecallAttributionRecipient(t, "promo_owned")
	raw := fmt.Sprintf(`{
		"id":"cs_omitted_discounts","amount_total":1000,"currency":"usd",
		"metadata":{"recall_campaign_id":"%d","recall_recipient_id":"%d"}
	}`, campaign.Id, recipient.Id)
	fact, err := ParseRecallPayment(
		stripe.Event{ID: "evt_omitted_discounts", Data: &stripe.EventData{Raw: []byte(raw)}},
		"trade_omitted_discounts",
		recipient.UserId,
	)
	require.NoError(t, err)
	fetched := false
	client := &recallStripeFakeClient{getCheckoutSessionFn: func(_ context.Context, id string, _ ...string) (*stripe.CheckoutSession, error) {
		fetched = true
		require.Equal(t, "cs_omitted_discounts", id)
		return &stripe.CheckoutSession{
			ID: id, AmountTotal: 750, Currency: stripe.CurrencyUSD,
			Discounts:    []*stripe.CheckoutSessionDiscount{{PromotionCode: &stripe.PromotionCode{ID: "promo_other"}}},
			TotalDetails: &stripe.CheckoutSessionTotalDetails{AmountDiscount: 250},
		}, nil
	}}

	require.NoError(t, NewRecallAttributionService(client).Attribute(context.Background(), fact))

	require.True(t, fetched)
	stored := model.RecallRecipient{}
	require.NoError(t, model.DB.First(&stored, recipient.Id).Error)
	require.Equal(t, model.RecallConversionAssisted, stored.ConversionKind)
	require.Equal(t, int64(750), stored.ConversionAmount)
	require.Equal(t, int64(250), stored.DiscountAmount)
}

func TestRecallAttributionMetricsKeepCurrenciesSeparate(t *testing.T) {
	setupRecallCampaignTestDB(t)
	campaign, first := createRecallAttributionRecipient(t, "promo_usd")
	secondPromotion := "promo_jpy"
	second := model.RecallRecipient{
		CampaignId: campaign.Id, UserId: 9002, EligibilitySnapshot: `{}`, EmailSnapshot: "jpy@example.com",
		LanguageSnapshot: "en", State: model.RecallRecipientConverted, StripePromotionCodeId: &secondPromotion,
		StripeCustomerId: "cus_jpy",
		ConvertedAt:      1_700_000_300, ConversionKind: model.RecallConversionAssisted, ConversionTradeNo: "trade_jpy",
		ConversionCurrency: "JPY", ConversionAmount: 8000, DiscountAmount: 500,
	}
	require.NoError(t, model.DB.Create(&second).Error)
	codeFailure := model.RecallRecipient{
		CampaignId: campaign.Id, UserId: 9003, EligibilitySnapshot: `{}`, EmailSnapshot: "code-failure@example.com",
		LanguageSnapshot: "en", State: model.RecallRecipientFailed, StripeCustomerId: "cus_code_failure",
	}
	require.NoError(t, model.DB.Create(&codeFailure).Error)
	customerFailure := model.RecallRecipient{
		CampaignId: campaign.Id, UserId: 9004, EligibilitySnapshot: `{}`, EmailSnapshot: "customer-failure@example.com",
		LanguageSnapshot: "en", State: model.RecallRecipientFailed,
	}
	require.NoError(t, model.DB.Create(&customerFailure).Error)
	noCoupon := model.RecallRecipient{
		CampaignId: campaign.Id, UserId: 9005, EligibilitySnapshot: `{}`, EmailSnapshot: "no-coupon@example.com",
		LanguageSnapshot: "en", State: model.RecallRecipientConverted, StripeCustomerId: "cus_no_coupon",
		ConvertedAt: 1_700_000_400, ConversionKind: model.RecallConversionNoCoupon, ConversionTradeNo: "trade_eur",
		ConversionCurrency: "EUR", ConversionAmount: 3000,
	}
	require.NoError(t, model.DB.Create(&noCoupon).Error)
	require.NoError(t, model.DB.Model(&model.RecallRecipient{}).Where("id = ?", first.Id).Updates(map[string]any{
		"state": model.RecallRecipientConverted, "converted_at": int64(1_700_000_200),
		"stripe_customer_id": "cus_usd",
		"conversion_kind":    model.RecallConversionDirect, "conversion_trade_no": "trade_usd",
		"conversion_currency": "USD", "conversion_amount": int64(1200), "discount_amount": int64(200),
	}).Error)
	for _, message := range []model.RecallMessage{
		{RecipientId: first.Id, StageNo: 1, State: model.RecallMessageAccepted},
		{RecipientId: second.Id, StageNo: 1, State: model.RecallMessageFailed},
		{RecipientId: codeFailure.Id, StageNo: 1, State: model.RecallMessageCancelled},
		{RecipientId: noCoupon.Id, StageNo: 1, State: model.RecallMessageScheduled},
	} {
		require.NoError(t, model.DB.Create(&message).Error)
	}
	require.NoError(t, model.DB.Create(&model.RecallEvent{
		CampaignId: campaign.Id, EventType: "campaign_run", Source: "scheduler", SourceEventId: "run_metrics",
		EventData: `{"eligible_total":2,"exclusions":{"paid":3}}`, CreatedAt: 1_700_000_000,
	}).Error)
	require.NoError(t, model.DB.Create(&model.RecallEvent{
		CampaignId: campaign.Id, RecipientId: first.Id, EventType: "observed_click", Source: "claim",
		SourceEventId: "metrics_click", EventData: `{}`, CreatedAt: 1_700_000_100,
	}).Error)

	metrics, err := NewRecallAttributionService(&recallStripeFakeClient{}).GetMetrics(context.Background(), campaign.Id)

	require.NoError(t, err)
	require.Equal(t, int64(5), metrics.CandidateCount)
	require.Equal(t, int64(5), metrics.EnrolledCount)
	require.Equal(t, int64(3), metrics.ExcludedCount)
	require.Equal(t, int64(4), metrics.CustomerSuccessCount)
	require.Equal(t, int64(1), metrics.CustomerFailureCount)
	require.Equal(t, int64(2), metrics.CodeSuccessCount)
	require.Equal(t, int64(1), metrics.CodeFailureCount)
	require.Equal(t, int64(4), metrics.MessagesScheduledCount)
	require.Equal(t, int64(1), metrics.MessagesAcceptedCount)
	require.Equal(t, int64(1), metrics.MessagesFailedCount)
	require.Equal(t, int64(1), metrics.MessagesCancelledCount)
	require.Equal(t, int64(1), metrics.ObservedClickCount)
	require.Equal(t, int64(1), metrics.DirectCount)
	require.Equal(t, int64(1), metrics.AssistedCount)
	require.Equal(t, int64(1), metrics.NoCouponCount)
	require.Equal(t, []RecallCurrencyMetrics{
		{Currency: "EUR", NoCouponCount: 1, PaymentAmount: 3000},
		{Currency: "JPY", AssistedCount: 1, PaymentAmount: 8000, DiscountAmount: 500},
		{Currency: "USD", DirectCount: 1, PaymentAmount: 1200, DiscountAmount: 200},
	}, metrics.CurrencyMetrics)
}

func TestRecallAttributionReconcileUsesOnlyRecoverableSuccessfulStripeOrders(t *testing.T) {
	setupRecallCampaignTestDB(t)
	_, topUpRecipient := createRecallAttributionRecipient(t, "promo_topup_reconcile")
	subCampaign, subRecipient := createRecallAttributionRecipient(t, "promo_sub_reconcile")
	require.NoError(t, model.DB.Model(&model.RecallRecipient{}).Where("id = ?", subRecipient.Id).Update("user_id", 9002).Error)
	subRecipient.UserId = 9002
	for _, userID := range []int{topUpRecipient.UserId, subRecipient.UserId} {
		require.NoError(t, model.DB.Create(&model.User{
			Id: userID, Username: fmt.Sprintf("reconcile-%d", userID), Email: fmt.Sprintf("%d@example.com", userID),
			AffCode: fmt.Sprintf("reconcile-aff-%d", userID),
		}).Error)
	}
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId: topUpRecipient.UserId, TradeNo: "trade_topup_reconcile", GatewayTradeNo: "cs_topup_reconcile",
		PaymentProvider: model.PaymentProviderStripe, Status: "success", CreateTime: 1_700_000_100, CompleteTime: 1_700_000_200,
	}).Error)
	require.NoError(t, model.DB.Create(&model.SubscriptionOrder{
		UserId: subRecipient.UserId, PlanId: 1, TradeNo: "trade_sub_reconcile", PaymentProvider: model.PaymentProviderStripe,
		Status: "success", CreateTime: 1_700_000_110, CompleteTime: 1_700_000_210,
		ProviderPayload: `{"checkout_session_id":"cs_sub_reconcile"}`,
	}).Error)
	// CompleteSubscriptionOrder creates this duplicate TopUp row; reconciliation must not fetch it twice.
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId: subRecipient.UserId, TradeNo: "trade_sub_reconcile", PaymentProvider: model.PaymentProviderStripe,
		Status: "success", CreateTime: 1_700_000_110, CompleteTime: 1_700_000_210,
	}).Error)
	// These orders are intentionally unrecoverable or non-authoritative.
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId: topUpRecipient.UserId, TradeNo: "trade_missing_session", PaymentProvider: model.PaymentProviderStripe,
		Status: "success", CreateTime: 1_700_000_120,
	}).Error)
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId: topUpRecipient.UserId, TradeNo: "trade_pending", GatewayTradeNo: "cs_pending",
		PaymentProvider: model.PaymentProviderStripe, Status: "pending", CreateTime: 1_700_000_130,
	}).Error)

	var fetched []string
	client := &recallStripeFakeClient{getCheckoutSessionFn: func(_ context.Context, id string, expand ...string) (*stripe.CheckoutSession, error) {
		fetched = append(fetched, id)
		if id == "cs_topup_reconcile" {
			return &stripe.CheckoutSession{
				ID: id, Created: 1_700_000_100, PaymentStatus: stripe.CheckoutSessionPaymentStatusPaid,
				AmountTotal: 1200, Currency: stripe.CurrencyUSD,
				Discounts:    []*stripe.CheckoutSessionDiscount{{PromotionCode: &stripe.PromotionCode{ID: "promo_topup_reconcile"}}},
				TotalDetails: &stripe.CheckoutSessionTotalDetails{AmountDiscount: 200},
			}, nil
		}
		return &stripe.CheckoutSession{
			ID: id, Created: 1_700_000_110, PaymentStatus: stripe.CheckoutSessionPaymentStatusPaid,
			AmountTotal: 8000, Currency: stripe.CurrencyJPY,
			Metadata: map[string]string{
				"recall_campaign_id":  fmt.Sprintf("%d", subCampaign.Id),
				"recall_recipient_id": fmt.Sprintf("%d", subRecipient.Id),
			},
			TotalDetails: &stripe.CheckoutSessionTotalDetails{},
		}, nil
	}}
	service := NewRecallAttributionService(client)
	service.now = func() time.Time { return time.Unix(1_700_000_300, 0).UTC() }

	processed, err := service.ReconcileBatch(context.Background(), 20)

	require.NoError(t, err)
	require.Equal(t, 2, processed)
	require.ElementsMatch(t, []string{"cs_topup_reconcile", "cs_sub_reconcile"}, fetched)
	for _, recipientID := range []int64{topUpRecipient.Id, subRecipient.Id} {
		stored := model.RecallRecipient{}
		require.NoError(t, model.DB.First(&stored, recipientID).Error)
		require.Equal(t, model.RecallRecipientConverted, stored.State)
	}
	var subscriptions int64
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Count(&subscriptions).Error)
	require.Zero(t, subscriptions, "reconciliation must never provision subscriptions")
}

func TestRecallMaintenanceRunsAttributionOncePerUTCWindow(t *testing.T) {
	setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	_, recipient := createRecallAttributionRecipient(t, "promo_scheduler")
	require.NoError(t, model.DB.Create(&model.User{
		Id: recipient.UserId, Username: "scheduler-reconcile", Email: "scheduler@example.com", AffCode: "scheduler-reconcile-aff",
	}).Error)
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId: recipient.UserId, TradeNo: "trade_scheduler", GatewayTradeNo: "cs_scheduler",
		PaymentProvider: model.PaymentProviderStripe, Status: "success", CreateTime: 1_700_000_100, CompleteTime: 1_700_000_200,
	}).Error)
	fetches := 0
	client := &recallStripeFakeClient{getCheckoutSessionFn: func(_ context.Context, id string, _ ...string) (*stripe.CheckoutSession, error) {
		fetches++
		return &stripe.CheckoutSession{
			ID: id, Created: 1_700_000_100, PaymentStatus: stripe.CheckoutSessionPaymentStatusPaid,
			AmountTotal: 1000, Currency: stripe.CurrencyUSD,
			Discounts:    []*stripe.CheckoutSessionDiscount{{PromotionCode: &stripe.PromotionCode{ID: "promo_scheduler"}}},
			TotalDetails: &stripe.CheckoutSessionTotalDetails{AmountDiscount: 100},
		}, nil
	}}
	stripeService := NewRecallStripeService(client)
	setRecallRuntimeForTest(t, &RecallRuntime{
		Campaigns:   NewRecallCampaignService(NewRecallAudienceSelector(), stripeService),
		Claims:      NewRecallClaimService(),
		Recipients:  NewRecallRecipientWorker(stripeService, NewRecallClaimService(), "scheduler-test"),
		Attribution: NewRecallAttributionService(client),
	})

	RunRecallMaintenanceTick(context.Background())
	// Reset the terminal fields to prove the database window event, rather than
	// the recipient terminal predicate, prevents a second reconciliation scan.
	require.NoError(t, model.DB.Model(&model.RecallRecipient{}).Where("id = ?", recipient.Id).Updates(map[string]any{
		"state": model.RecallRecipientContacting, "converted_at": int64(0), "conversion_kind": "",
		"conversion_trade_no": "", "conversion_currency": "", "conversion_amount": int64(0), "discount_amount": int64(0),
	}).Error)
	RunRecallMaintenanceTick(context.Background())

	require.Equal(t, 1, fetches)
	var windowEvents int64
	require.NoError(t, model.DB.Model(&model.RecallEvent{}).Where("event_type = ?", "reconciliation_run").Count(&windowEvents).Error)
	require.Equal(t, int64(1), windowEvents)
}

func createRecallAttributionRecipient(t *testing.T, promotionCodeID string) (model.RecallCampaign, model.RecallRecipient) {
	t.Helper()
	campaign := model.RecallCampaign{
		Name: "attribution campaign", Status: model.RecallCampaignRunning, AudienceTemplate: "first_purchase",
		AudienceConfig: `{}`, ExecutionMode: "manual", CouponSource: "automatic", DiscountConfig: `{}`,
		ProductScope: `{}`, EmailSequenceConfig: `[]`,
	}
	require.NoError(t, model.DB.Create(&campaign).Error)
	recipient := model.RecallRecipient{
		CampaignId: campaign.Id, UserId: 9001, EligibilitySnapshot: `{}`, EmailSnapshot: "attribution@example.com",
		LanguageSnapshot: "en", State: model.RecallRecipientContacting, StripePromotionCodeId: &promotionCodeID,
		PromotionCode: "FKOWNED234", CreatedAt: 1_700_000_000,
	}
	require.NoError(t, model.DB.Create(&recipient).Error)
	return campaign, recipient
}
