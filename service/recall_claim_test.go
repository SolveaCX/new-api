package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestRecallClaimIssueStoresOnlyHashAndCopiesStageOneLookup(t *testing.T) {
	db := setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	now := time.Unix(1_721_000_000, 0).UTC()

	campaign := model.RecallCampaign{Name: "win-back", Status: model.RecallCampaignRunning, AudienceTemplate: "first_purchase", AudienceConfig: `{}`, ExecutionMode: "manual", CouponSource: "automatic", DiscountConfig: `{}`, ProductScope: `{}`, EmailSequenceConfig: `[]`}
	require.NoError(t, db.Create(&campaign).Error)
	recipient := model.RecallRecipient{CampaignId: campaign.Id, UserId: 7, EligibilitySnapshot: `{}`, EmailSnapshot: "user@example.com", LanguageSnapshot: "en", State: model.RecallRecipientContacting}
	require.NoError(t, db.Create(&recipient).Error)
	message := model.RecallMessage{RecipientId: recipient.Id, StageNo: 1, TemplateVersion: 1, TemplateSnapshot: `{}`, ScheduledAt: now.Unix(), State: model.RecallMessageLeased, LeaseOwner: "worker-a", LeaseExpiresAt: now.Add(time.Minute).Unix()}
	require.NoError(t, db.Create(&message).Error)

	claimService := NewRecallClaimService()
	claimService.random = bytes.NewReader(bytes.Repeat([]byte{0xab}, 36))
	claim, err := claimService.IssueClaim(context.Background(), message.Id, "worker-a", message.LeaseExpiresAt)
	require.NoError(t, err)
	require.Len(t, claim, 48)

	wantHashBytes := sha256.Sum256([]byte(claim))
	wantHash := hex.EncodeToString(wantHashBytes[:])
	var storedMessage model.RecallMessage
	require.NoError(t, db.First(&storedMessage, message.Id).Error)
	require.NotNil(t, storedMessage.ClaimTokenHash)
	require.Equal(t, wantHash, *storedMessage.ClaimTokenHash)
	require.NotContains(t, *storedMessage.ClaimTokenHash, claim)
	var storedRecipient model.RecallRecipient
	require.NoError(t, db.First(&storedRecipient, recipient.Id).Error)
	require.NotNil(t, storedRecipient.ClaimTokenHash)
	require.Equal(t, wantHash, *storedRecipient.ClaimTokenHash)
}

func TestRecallClaimIssueKeepsIndependentAcceptedAndUncertainLinks(t *testing.T) {
	for _, state := range []string{model.RecallMessageAccepted, model.RecallMessageUncertain} {
		t.Run(state, func(t *testing.T) {
			db := setupRecallCampaignTestDB(t)
			now := time.Unix(1_721_000_000, 0).UTC()
			campaign := model.RecallCampaign{Name: "win-back", Status: model.RecallCampaignRunning, AudienceTemplate: "first_purchase", AudienceConfig: `{}`, ExecutionMode: "manual", CouponSource: "automatic", DiscountConfig: `{}`, ProductScope: `{}`, EmailSequenceConfig: `[]`}
			require.NoError(t, db.Create(&campaign).Error)
			stageOneHash := recallClaimHash(strings.Repeat("a", 48))
			recipient := model.RecallRecipient{CampaignId: campaign.Id, UserId: 7, EligibilitySnapshot: `{}`, EmailSnapshot: "user@example.com", LanguageSnapshot: "en", State: model.RecallRecipientContacting, ClaimTokenHash: &stageOneHash}
			require.NoError(t, db.Create(&recipient).Error)
			preservedHash := recallClaimHash(strings.Repeat("p", 48))
			preserved := model.RecallMessage{RecipientId: recipient.Id, StageNo: 2, TemplateVersion: 1, TemplateSnapshot: `{}`, State: state, ClaimTokenHash: &preservedHash}
			require.NoError(t, db.Create(&preserved).Error)
			leased := model.RecallMessage{RecipientId: recipient.Id, StageNo: 3, TemplateVersion: 1, TemplateSnapshot: `{}`, State: model.RecallMessageLeased, LeaseOwner: "worker-a", LeaseExpiresAt: now.Add(time.Minute).Unix()}
			require.NoError(t, db.Create(&leased).Error)

			claimService := NewRecallClaimService()
			claimService.random = bytes.NewReader(bytes.Repeat([]byte{0xbc}, 36))
			claim, err := claimService.IssueClaim(context.Background(), leased.Id, leased.LeaseOwner, leased.LeaseExpiresAt)
			require.NoError(t, err)
			require.NotEqual(t, preservedHash, recallClaimHash(claim))

			var storedPreserved model.RecallMessage
			require.NoError(t, db.First(&storedPreserved, preserved.Id).Error)
			require.Equal(t, preservedHash, *storedPreserved.ClaimTokenHash)
			var storedRecipient model.RecallRecipient
			require.NoError(t, db.First(&storedRecipient, recipient.Id).Error)
			require.Equal(t, stageOneHash, *storedRecipient.ClaimTokenHash, "later stages must not replace the stage-one lookup")
		})
	}
}

type recallClaimFixture struct {
	campaign  model.RecallCampaign
	recipient model.RecallRecipient
	message   model.RecallMessage
	claim     string
}

func createRecallClaimFixture(t *testing.T, now time.Time) recallClaimFixture {
	t.Helper()
	discountJSON, err := common.Marshal(RecallDiscountConfig{Type: "percent", PercentOff: 20})
	require.NoError(t, err)
	productsJSON, err := common.Marshal(RecallProductScope{
		TopUpPriceIDs:        []string{"price_topup"},
		SubscriptionPriceIDs: []string{"price_subscription"},
	})
	require.NoError(t, err)
	campaign := model.RecallCampaign{
		Name: "win-back", Status: model.RecallCampaignRunning, AudienceTemplate: "first_purchase",
		AudienceConfig: `{}`, ExecutionMode: "manual", CouponSource: "automatic",
		DiscountConfig: string(discountJSON), ProductScope: string(productsJSON), EmailSequenceConfig: `[]`,
	}
	require.NoError(t, model.DB.Create(&campaign).Error)
	promotionID := "promo_recall"
	recipient := model.RecallRecipient{
		CampaignId: campaign.Id, UserId: 7, EligibilitySnapshot: `{}`, EmailSnapshot: "user@example.com",
		LanguageSnapshot: "en", State: model.RecallRecipientContacting,
		StripePromotionCodeId: &promotionID, PromotionCode: "FKSECRET234", PromotionExpiresAt: now.Add(time.Hour).Unix(),
	}
	require.NoError(t, model.DB.Create(&recipient).Error)
	claim := strings.Repeat("c", 48)
	claimHash := recallClaimHash(claim)
	message := model.RecallMessage{
		RecipientId: recipient.Id, StageNo: 2, TemplateVersion: 1, TemplateSnapshot: `{}`,
		ScheduledAt: now.Unix(), State: model.RecallMessageAccepted, ClaimTokenHash: &claimHash,
	}
	require.NoError(t, model.DB.Create(&message).Error)
	return recallClaimFixture{campaign: campaign, recipient: recipient, message: message, claim: claim}
}

func recallClaimHash(claim string) string {
	digest := sha256.Sum256([]byte(claim))
	return hex.EncodeToString(digest[:])
}

func TestRecallClaimValidateFindsAnyStageHashAndRecordsOneObservedClick(t *testing.T) {
	db := setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	now := time.Unix(1_721_000_000, 0).UTC()
	fixture := createRecallClaimFixture(t, now)
	claimService := NewRecallClaimService()
	claimService.now = func() time.Time { return now }

	first, err := claimService.ValidateClaim(context.Background(), fixture.recipient.UserId, fixture.claim)
	require.NoError(t, err)
	require.Equal(t, fixture.campaign.Id, first.CampaignID)
	require.Equal(t, fixture.recipient.Id, first.RecipientID)
	require.Equal(t, model.MaskPromotionCode(fixture.recipient.PromotionCode), first.PromotionCodeMasked)
	require.NotEqual(t, fixture.recipient.PromotionCode, first.PromotionCodeMasked)
	second, err := claimService.ValidateClaim(context.Background(), fixture.recipient.UserId, fixture.claim)
	require.NoError(t, err)
	require.Equal(t, first, second)

	var storedRecipient model.RecallRecipient
	require.NoError(t, db.First(&storedRecipient, fixture.recipient.Id).Error)
	require.Equal(t, now.Unix(), storedRecipient.ClickedAt)
	require.Zero(t, storedRecipient.ConvertedAt, "a click must never be a conversion")
	var events []model.RecallEvent
	require.NoError(t, db.Where("recipient_id = ? AND event_type = ?", fixture.recipient.Id, "observed_click").Find(&events).Error)
	require.Len(t, events, 1)
}

func TestRecallClaimValidateAcceptsStageOneRecipientLookupAfterMessageAcceptance(t *testing.T) {
	setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	now := time.Unix(1_721_000_000, 0).UTC()
	fixture := createRecallClaimFixture(t, now)
	stageOneClaim := strings.Repeat("a", 48)
	stageOneHash := recallClaimHash(stageOneClaim)
	require.NoError(t, model.DB.Model(&model.RecallRecipient{}).Where("id = ?", fixture.recipient.Id).Update("claim_token_hash", stageOneHash).Error)

	claimService := NewRecallClaimService()
	claimService.now = func() time.Time { return now }
	view, err := claimService.ValidateClaim(context.Background(), fixture.recipient.UserId, stageOneClaim)
	require.NoError(t, err)
	require.Equal(t, fixture.recipient.Id, view.RecipientID)

	view, err = claimService.ValidateClaim(context.Background(), fixture.recipient.UserId, fixture.claim)
	require.NoError(t, err, "an accepted later-stage message link must remain independently valid")
	require.Equal(t, fixture.recipient.Id, view.RecipientID)
}

func TestRecallClaimValidateRejectsInvalidClaimsWithTypedErrors(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*testing.T, recallClaimFixture, time.Time)
		userID  int
		claim   func(recallClaimFixture) string
		wantErr error
	}{
		{name: "wrong user", userID: 8, claim: func(f recallClaimFixture) string { return f.claim }, wantErr: ErrRecallClaimWrongUser},
		{name: "unknown", userID: 7, claim: func(recallClaimFixture) string { return strings.Repeat("z", 48) }, wantErr: ErrRecallClaimUnknown},
		{name: "expired", userID: 7, claim: func(f recallClaimFixture) string { return f.claim }, wantErr: ErrRecallClaimExpired, mutate: func(t *testing.T, f recallClaimFixture, now time.Time) {
			require.NoError(t, model.DB.Model(&model.RecallRecipient{}).Where("id = ?", f.recipient.Id).Update("promotion_expires_at", now.Unix()).Error)
		}},
		{name: "converted", userID: 7, claim: func(f recallClaimFixture) string { return f.claim }, wantErr: ErrRecallClaimConverted, mutate: func(t *testing.T, f recallClaimFixture, _ time.Time) {
			require.NoError(t, model.DB.Model(&model.RecallRecipient{}).Where("id = ?", f.recipient.Id).Updates(map[string]any{"state": model.RecallRecipientConverted, "converted_at": int64(1)}).Error)
		}},
		{name: "suppressed", userID: 7, claim: func(f recallClaimFixture) string { return f.claim }, wantErr: ErrRecallClaimSuppressed, mutate: func(t *testing.T, f recallClaimFixture, _ time.Time) {
			require.NoError(t, model.DB.Model(&model.RecallRecipient{}).Where("id = ?", f.recipient.Id).Update("state", model.RecallRecipientSuppressed).Error)
		}},
		{name: "terminal campaign", userID: 7, claim: func(f recallClaimFixture) string { return f.claim }, wantErr: ErrRecallClaimInactive, mutate: func(t *testing.T, f recallClaimFixture, _ time.Time) {
			require.NoError(t, model.DB.Model(&model.RecallCampaign{}).Where("id = ?", f.campaign.Id).Update("status", model.RecallCampaignCancelled).Error)
		}},
		{name: "invalid promotion", userID: 7, claim: func(f recallClaimFixture) string { return f.claim }, wantErr: ErrRecallClaimPromotionInvalid, mutate: func(t *testing.T, f recallClaimFixture, _ time.Time) {
			require.NoError(t, model.DB.Model(&model.RecallRecipient{}).Where("id = ?", f.recipient.Id).Update("promotion_code", "").Error)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupRecallCampaignTestDB(t)
			setRecallCampaignEnabled(t, true)
			now := time.Unix(1_721_000_000, 0).UTC()
			fixture := createRecallClaimFixture(t, now)
			if test.mutate != nil {
				test.mutate(t, fixture, now)
			}
			claimService := NewRecallClaimService()
			claimService.now = func() time.Time { return now }
			_, err := claimService.ValidateClaim(context.Background(), test.userID, test.claim(fixture))
			require.ErrorIs(t, err, test.wantErr)
		})
	}
}

func TestRecallClaimBuildCheckoutDiscountValidatesKindAndPrice(t *testing.T) {
	setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	now := time.Unix(1_721_000_000, 0).UTC()
	fixture := createRecallClaimFixture(t, now)
	claimService := NewRecallClaimService()
	claimService.now = func() time.Time { return now }

	topup, err := claimService.BuildCheckoutDiscount(context.Background(), fixture.recipient.UserId, fixture.claim, RecallPurchaseKindTopUp, "price_topup")
	require.NoError(t, err)
	require.NotNil(t, topup)
	require.Equal(t, *fixture.recipient.StripePromotionCodeId, topup.PromotionCodeID)
	require.Equal(t, fixture.campaign.Id, topup.CampaignID)
	require.Equal(t, fixture.recipient.Id, topup.RecipientID)
	subscription, err := claimService.BuildCheckoutDiscount(context.Background(), fixture.recipient.UserId, fixture.claim, RecallPurchaseKindSubscription, "price_subscription")
	require.NoError(t, err)
	require.Equal(t, *fixture.recipient.StripePromotionCodeId, subscription.PromotionCodeID)
	require.Equal(t, fixture.campaign.Id, subscription.CampaignID)
	require.Equal(t, fixture.recipient.Id, subscription.RecipientID)

	_, err = claimService.BuildCheckoutDiscount(context.Background(), fixture.recipient.UserId, fixture.claim, RecallPurchaseKindTopUp, "price_other")
	require.ErrorIs(t, err, ErrRecallClaimWrongPrice)
	_, err = claimService.BuildCheckoutDiscount(context.Background(), fixture.recipient.UserId, fixture.claim, RecallPurchaseKindSubscription, "price_topup")
	require.ErrorIs(t, err, ErrRecallClaimWrongPrice)
	_, err = claimService.BuildCheckoutDiscount(context.Background(), fixture.recipient.UserId, fixture.claim, "other", "price_topup")
	require.ErrorIs(t, err, ErrRecallClaimPurchaseKind)
}

func TestRecallClaimDisabledAndEmptyCheckoutClaim(t *testing.T) {
	setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, false)
	claimService := NewRecallClaimService()

	discount, err := claimService.BuildCheckoutDiscount(context.Background(), 7, "", RecallPurchaseKindTopUp, "price_topup")
	require.NoError(t, err)
	require.Nil(t, discount)
	_, err = claimService.ValidateClaim(context.Background(), 7, strings.Repeat("c", 48))
	require.ErrorIs(t, err, ErrRecallDisabled)
	_, err = claimService.BuildCheckoutDiscount(context.Background(), 7, strings.Repeat("c", 48), RecallPurchaseKindTopUp, "price_topup")
	require.ErrorIs(t, err, ErrRecallDisabled)
}

func TestRecallClaimAPITypesDoNotExposeSecrets(t *testing.T) {
	promotionID := "promo_secret"
	hash := strings.Repeat("f", 64)
	viewRaw, err := common.Marshal(RecallClaimView{PromotionCodeMasked: "FKSE****34"})
	require.NoError(t, err)
	require.NotContains(t, string(viewRaw), "promo_secret")
	require.NotContains(t, string(viewRaw), hash)
	recipientRaw, err := common.Marshal(model.RecallRecipient{StripePromotionCodeId: &promotionID, PromotionCode: "FKSECRET234", ClaimTokenHash: &hash})
	require.NoError(t, err)
	require.NotContains(t, string(recipientRaw), promotionID)
	require.NotContains(t, string(recipientRaw), "FKSECRET234")
	require.NotContains(t, string(recipientRaw), hash)
	messageRaw, err := common.Marshal(model.RecallMessage{ClaimTokenHash: &hash})
	require.NoError(t, err)
	require.NotContains(t, string(messageRaw), hash)
	checkoutRaw, err := common.Marshal(RecallCheckoutDiscount{PromotionCodeID: promotionID, CampaignID: 12, RecipientID: 34})
	require.NoError(t, err)
	var checkoutJSON map[string]any
	require.NoError(t, common.Unmarshal(checkoutRaw, &checkoutJSON))
	require.ElementsMatch(t, []string{"promotion_code_id", "campaign_id", "recipient_id"}, recallAudienceJSONKeys(checkoutJSON))
	require.NotContains(t, string(checkoutRaw), "FKSECRET234")
	require.NotContains(t, string(checkoutRaw), hash)
}

func TestRecallClaimSignedUnsubscribePreservesSettingsAndCancelsAllPendingMessages(t *testing.T) {
	db := setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	now := time.Unix(1_721_000_000, 0).UTC()
	originalSecret := common.CryptoSecret
	common.CryptoSecret = "recall-test-secret"
	t.Cleanup(func() { common.CryptoSecret = originalSecret })
	settingJSON, err := common.Marshal(dto.UserSetting{Language: "zh", BillingPreference: "wallet_first", RecallMarketingOptOut: false})
	require.NoError(t, err)
	user := model.User{Username: "unsubscribe-user", AffCode: "unsubscribe-aff", Password: "hash", Status: common.UserStatusEnabled, Email: "unsubscribe@example.com", Setting: string(settingJSON)}
	require.NoError(t, db.Create(&user).Error)

	for campaignIndex := 0; campaignIndex < 2; campaignIndex++ {
		campaign := model.RecallCampaign{Name: "campaign", Status: model.RecallCampaignRunning, AudienceTemplate: "first_purchase", AudienceConfig: `{}`, ExecutionMode: "manual", CouponSource: "automatic", DiscountConfig: `{}`, ProductScope: `{}`, EmailSequenceConfig: `[]`}
		require.NoError(t, db.Create(&campaign).Error)
		recipient := model.RecallRecipient{CampaignId: campaign.Id, UserId: user.Id, EligibilitySnapshot: `{}`, EmailSnapshot: user.Email, LanguageSnapshot: "zh", State: model.RecallRecipientContacting}
		require.NoError(t, db.Create(&recipient).Error)
		for stage, state := range []string{model.RecallMessageScheduled, model.RecallMessageRetryWait, model.RecallMessageAccepted} {
			message := model.RecallMessage{RecipientId: recipient.Id, StageNo: stage + 1, TemplateVersion: 1, TemplateSnapshot: `{}`, State: state}
			require.NoError(t, db.Create(&message).Error)
		}
	}

	claimService := NewRecallClaimService()
	claimService.now = func() time.Time { return now }
	token, err := claimService.CreateUnsubscribeToken(user.Id, now.Add(time.Hour))
	require.NoError(t, err)
	require.NoError(t, claimService.Unsubscribe(context.Background(), token))

	var storedUser model.User
	require.NoError(t, db.First(&storedUser, user.Id).Error)
	storedSetting := dto.UserSetting{}
	require.NoError(t, common.Unmarshal([]byte(storedUser.Setting), &storedSetting))
	require.True(t, storedSetting.RecallMarketingOptOut)
	require.Equal(t, "zh", storedSetting.Language)
	require.Equal(t, "wallet_first", storedSetting.BillingPreference)
	var cancelled int64
	require.NoError(t, db.Model(&model.RecallMessage{}).Where("state = ?", model.RecallMessageCancelled).Count(&cancelled).Error)
	require.EqualValues(t, 4, cancelled)
	var accepted int64
	require.NoError(t, db.Model(&model.RecallMessage{}).Where("state = ?", model.RecallMessageAccepted).Count(&accepted).Error)
	require.EqualValues(t, 2, accepted)
}

func TestRecallClaimUnsubscribeRejectsTamperingAndExpiry(t *testing.T) {
	setupRecallCampaignTestDB(t)
	now := time.Unix(1_721_000_000, 0).UTC()
	originalSecret := common.CryptoSecret
	common.CryptoSecret = "recall-test-secret"
	t.Cleanup(func() { common.CryptoSecret = originalSecret })
	claimService := NewRecallClaimService()
	claimService.now = func() time.Time { return now }

	token, err := claimService.CreateUnsubscribeToken(7, now.Add(time.Hour))
	require.NoError(t, err)
	tampered := token[:len(token)-1] + "0"
	err = claimService.Unsubscribe(context.Background(), tampered)
	require.True(t, errors.Is(err, ErrRecallUnsubscribeInvalid))
	expired, err := claimService.CreateUnsubscribeToken(7, now.Add(-time.Second))
	require.NoError(t, err)
	err = claimService.Unsubscribe(context.Background(), expired)
	require.ErrorIs(t, err, ErrRecallUnsubscribeExpired)
}

func TestRecallClaimRuntimeContainsClaimsWithoutSchedulerWork(t *testing.T) {
	runtime := GetRecallRuntime()
	require.NotNil(t, runtime.Claims)
}
