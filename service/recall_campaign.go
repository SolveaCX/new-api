package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

var ErrRecallDisabled = errors.New("recall campaigns are disabled")

type RecallCampaignService struct {
	audience *RecallAudienceSelector
	stripe   *RecallStripeService
	now      func() time.Time
}

func NewRecallCampaignService(audience *RecallAudienceSelector, stripeService *RecallStripeService) *RecallCampaignService {
	if audience == nil {
		audience = NewRecallAudienceSelector()
	}
	if stripeService == nil {
		stripeService = NewRecallStripeService(nil)
	}
	return &RecallCampaignService{
		audience: audience,
		stripe:   stripeService,
		now:      time.Now,
	}
}

func (s *RecallCampaignService) SaveDraft(ctx context.Context, actorID int, draft RecallCampaignDraft) (*model.RecallCampaign, error) {
	if err := recallCampaignGate(ctx); err != nil {
		return nil, err
	}
	if actorID <= 0 {
		return nil, fmt.Errorf("recall campaign actor ID must be positive")
	}
	normalized, err := validateAndNormalizeRecallCampaignDraft(draft, s.now())
	if err != nil {
		return nil, err
	}
	campaign, err := recallCampaignModelFromDraft(normalized, actorID)
	if err != nil {
		return nil, err
	}
	if err := model.CreateRecallCampaignWithContext(ctx, campaign); err != nil {
		return nil, err
	}
	return campaign, nil
}

func (s *RecallCampaignService) UpdateDraft(ctx context.Context, actorID int, id int64, draft RecallCampaignDraft) (*model.RecallCampaign, error) {
	if err := recallCampaignGate(ctx); err != nil {
		return nil, err
	}
	if actorID <= 0 || id <= 0 {
		return nil, fmt.Errorf("recall campaign actor and campaign IDs must be positive")
	}
	stored, err := model.GetRecallCampaignByIDWithContext(ctx, id)
	if err != nil {
		return nil, err
	}
	validationTime := s.now()
	if stored.Status != model.RecallCampaignDraft {
		validationTime = time.Unix(0, 0).UTC()
	}
	normalized, err := validateAndNormalizeRecallCampaignDraft(draft, validationTime)
	if err != nil {
		return nil, err
	}
	if stored.Status == model.RecallCampaignDraft {
		updated, err := recallCampaignModelFromDraft(normalized, stored.CreatedBy)
		if err != nil {
			return nil, err
		}
		updated.Id = stored.Id
		won, err := model.UpdateRecallCampaignDraftWithContext(ctx, updated)
		if err != nil {
			return nil, err
		}
		if !won {
			return nil, fmt.Errorf("recall campaign %d is no longer editable as a draft", id)
		}
		return model.GetRecallCampaignByIDWithContext(ctx, id)
	}
	if stored.Status == model.RecallCampaignCancelled || stored.Status == model.RecallCampaignCompleted {
		return nil, fmt.Errorf("recall campaign %d is in terminal state %s", id, stored.Status)
	}

	current, err := recallCampaignDraftFromModel(stored)
	if err != nil {
		return nil, err
	}
	currentNormalized, err := validateAndNormalizeRecallCampaignDraft(current, time.Unix(0, 0).UTC())
	if err != nil {
		return nil, fmt.Errorf("stored recall campaign %d is invalid: %w", id, err)
	}
	if !reflect.DeepEqual(recallCampaignImmutableDraft(currentNormalized), recallCampaignImmutableDraft(normalized)) {
		return nil, fmt.Errorf("activated recall campaign configuration is immutable")
	}
	emails, err := incrementRecallEmailTemplateVersions(current.Emails, normalized.Emails)
	if err != nil {
		return nil, err
	}
	emailJSON, err := common.Marshal(emails)
	if err != nil {
		return nil, err
	}
	won, err := model.UpdateRecallCampaignEmailSequenceWithContext(ctx, id, normalized.Name, string(emailJSON))
	if err != nil {
		return nil, err
	}
	if !won {
		return nil, fmt.Errorf("recall campaign %d state changed while updating email content", id)
	}
	return model.GetRecallCampaignByIDWithContext(ctx, id)
}

func (s *RecallCampaignService) Preview(ctx context.Context, id int64, sampleSize int) (RecallAudiencePreview, RecallStripePreview, error) {
	if err := recallCampaignGate(ctx); err != nil {
		return RecallAudiencePreview{}, RecallStripePreview{}, err
	}
	if id <= 0 {
		return RecallAudiencePreview{}, RecallStripePreview{}, fmt.Errorf("recall campaign ID must be positive")
	}
	campaign, err := model.GetRecallCampaignByIDWithContext(ctx, id)
	if err != nil {
		return RecallAudiencePreview{}, RecallStripePreview{}, err
	}
	draft, err := recallCampaignDraftFromModel(campaign)
	if err != nil {
		return RecallAudiencePreview{}, RecallStripePreview{}, err
	}
	audiencePreview, err := s.audience.Preview(ctx, draft, sampleSize, s.now())
	if err != nil {
		return RecallAudiencePreview{}, RecallStripePreview{}, err
	}
	stripePreview, err := s.validateStripe(ctx, draft)
	if err != nil {
		return RecallAudiencePreview{}, RecallStripePreview{}, err
	}
	return audiencePreview, stripePreview, nil
}

func (s *RecallCampaignService) ValidateStripe(ctx context.Context, draft RecallCampaignDraft) (RecallStripePreview, error) {
	if err := recallCampaignGate(ctx); err != nil {
		return RecallStripePreview{}, err
	}
	normalized, err := validateAndNormalizeRecallCampaignDraft(draft, s.now())
	if err != nil {
		return RecallStripePreview{}, err
	}
	return s.validateStripe(ctx, normalized)
}

func (s *RecallCampaignService) validateStripe(ctx context.Context, draft RecallCampaignDraft) (RecallStripePreview, error) {
	resolved, err := s.stripe.ValidateAndResolveProducts(ctx, draft.Products)
	if err != nil {
		return RecallStripePreview{}, err
	}
	preview := RecallStripePreview{
		CouponSource:         draft.CouponSource,
		Discount:             draft.Discount,
		TopUpPriceIDs:        append([]string(nil), resolved.TopUpPriceIDs...),
		SubscriptionPriceIDs: append([]string(nil), resolved.SubscriptionPriceIDs...),
		ProductIDs:           append([]string(nil), resolved.ProductIDs...),
	}
	if draft.CouponSource == "existing" {
		coupon, discount, err := s.stripe.EnsureCoupon(
			ctx,
			1,
			draft.CouponSource,
			draft.ExistingCouponID,
			draft.Discount,
			resolved,
			draft.EnrollmentLimit,
		)
		if err != nil {
			return RecallStripePreview{}, err
		}
		preview.CouponID = coupon.ID
		preview.Discount = discount
	}
	return preview, nil
}

func (s *RecallCampaignService) Activate(ctx context.Context, actorID int, id int64) error {
	if err := recallCampaignGate(ctx); err != nil {
		return err
	}
	if actorID <= 0 || id <= 0 {
		return fmt.Errorf("recall campaign actor and campaign IDs must be positive")
	}
	campaign, err := model.GetRecallCampaignByIDWithContext(ctx, id)
	if err != nil {
		return err
	}
	if campaign.Status != model.RecallCampaignDraft {
		if campaign.Status == model.RecallCampaignScheduled || campaign.Status == model.RecallCampaignRunning {
			return nil
		}
		return fmt.Errorf("recall campaign %d cannot activate from %s", id, campaign.Status)
	}
	draft, err := recallCampaignDraftFromModel(campaign)
	if err != nil {
		return err
	}
	activationNow := s.now()
	draft, err = validateAndNormalizeRecallCampaignDraft(draft, activationNow)
	if err != nil {
		return err
	}
	resolved, err := s.stripe.ValidateAndResolveProducts(ctx, draft.Products)
	if err != nil {
		return err
	}
	coupon, discount, err := s.stripe.EnsureCoupon(
		ctx,
		campaign.Id,
		draft.CouponSource,
		draft.ExistingCouponID,
		draft.Discount,
		resolved,
		draft.EnrollmentLimit,
	)
	if err != nil {
		return err
	}
	draft.Discount = discount
	fields, err := recallCampaignActivationFields(draft, coupon.ID, activationNow.Unix())
	if err != nil {
		return err
	}
	switch draft.ExecutionMode {
	case "manual":
		committed, err := s.commitCampaignRun(
			ctx,
			campaign,
			draft,
			[]string{model.RecallCampaignDraft},
			model.RecallCampaignRunning,
			nil,
			fields,
			"manual:"+strconv.FormatInt(campaign.Id, 10),
			activationNow,
		)
		if err != nil {
			return err
		}
		if !committed {
			return s.acceptRecallCampaignTargetState(ctx, id, model.RecallCampaignRunning)
		}
		return nil
	case "scheduled_once":
		fields["scheduled_at"] = draft.Schedule.ScheduledAt
		fields["next_run_at"] = draft.Schedule.ScheduledAt
		won, err := model.TransitionRecallCampaignWithContext(ctx, id, []string{model.RecallCampaignDraft}, model.RecallCampaignScheduled, fields)
		if err != nil {
			return err
		}
		if !won {
			return s.acceptRecallCampaignTargetState(ctx, id, model.RecallCampaignScheduled)
		}
		return nil
	case "recurring":
		nextRun, err := NextRecallRun(activationNow, draft.Schedule)
		if err != nil {
			return err
		}
		fields["next_run_at"] = nextRun.Unix()
		won, err := model.TransitionRecallCampaignWithContext(ctx, id, []string{model.RecallCampaignDraft}, model.RecallCampaignScheduled, fields)
		if err != nil {
			return err
		}
		if !won {
			return s.acceptRecallCampaignTargetState(ctx, id, model.RecallCampaignScheduled)
		}
		return nil
	default:
		return fmt.Errorf("unsupported recall execution mode %q", draft.ExecutionMode)
	}
}

func (s *RecallCampaignService) Pause(ctx context.Context, actorID int, id int64) error {
	return s.transitionCampaign(ctx, actorID, id, []string{
		model.RecallCampaignScheduled,
		model.RecallCampaignRunning,
	}, model.RecallCampaignPaused, nil)
}

func (s *RecallCampaignService) Resume(ctx context.Context, actorID int, id int64) error {
	return s.transitionCampaign(ctx, actorID, id, []string{model.RecallCampaignPaused}, model.RecallCampaignRunning, nil)
}

func (s *RecallCampaignService) Cancel(ctx context.Context, actorID int, id int64) error {
	if err := recallCampaignGate(ctx); err != nil {
		return err
	}
	if actorID <= 0 || id <= 0 {
		return fmt.Errorf("recall campaign actor and campaign IDs must be positive")
	}
	campaign, err := model.GetRecallCampaignByIDWithContext(ctx, id)
	if err != nil {
		return err
	}
	if campaign.Status == model.RecallCampaignCancelled {
		return nil
	}
	if campaign.Status == model.RecallCampaignCompleted {
		return fmt.Errorf("completed recall campaign %d cannot be cancelled", id)
	}
	won, err := model.CancelRecallCampaignWithContext(ctx, id, []string{
		model.RecallCampaignDraft,
		model.RecallCampaignScheduled,
		model.RecallCampaignRunning,
		model.RecallCampaignPaused,
	}, s.now().Unix(), "campaign_cancelled")
	if err != nil {
		return err
	}
	if !won {
		return s.acceptRecallCampaignTargetState(ctx, id, model.RecallCampaignCancelled)
	}
	return nil
}

func (s *RecallCampaignService) Complete(ctx context.Context, actorID int, id int64) error {
	return s.transitionCampaign(ctx, actorID, id, []string{
		model.RecallCampaignScheduled,
		model.RecallCampaignRunning,
		model.RecallCampaignPaused,
	}, model.RecallCampaignCompleted, map[string]any{"completed_at": s.now().Unix()})
}

func (s *RecallCampaignService) transitionCampaign(ctx context.Context, actorID int, id int64, from []string, to string, fields map[string]any) error {
	if err := recallCampaignGate(ctx); err != nil {
		return err
	}
	if actorID <= 0 || id <= 0 {
		return fmt.Errorf("recall campaign actor and campaign IDs must be positive")
	}
	campaign, err := model.GetRecallCampaignByIDWithContext(ctx, id)
	if err != nil {
		return err
	}
	if campaign.Status == to {
		return nil
	}
	if !containsRecallCampaignStatus(from, campaign.Status) {
		return fmt.Errorf("recall campaign %d cannot transition from %s to %s", id, campaign.Status, to)
	}
	won, err := model.TransitionRecallCampaignWithContext(ctx, id, from, to, fields)
	if err != nil {
		return err
	}
	if !won {
		return s.acceptRecallCampaignTargetState(ctx, id, to)
	}
	return nil
}

func (s *RecallCampaignService) acceptRecallCampaignTargetState(ctx context.Context, id int64, target string) error {
	campaign, err := model.GetRecallCampaignByIDWithContext(ctx, id)
	if err != nil {
		return err
	}
	if campaign.Status == target {
		return nil
	}
	return fmt.Errorf("recall campaign %d state changed to %s", id, campaign.Status)
}

func containsRecallCampaignStatus(statuses []string, status string) bool {
	for _, candidate := range statuses {
		if candidate == status {
			return true
		}
	}
	return false
}

func (s *RecallCampaignService) RunDueCampaigns(ctx context.Context, now time.Time, limit int) (int, error) {
	if err := recallCampaignGate(ctx); err != nil {
		return 0, err
	}
	if limit <= 0 {
		return 0, nil
	}
	campaigns, err := model.ListDueRecallCampaignsWithContext(ctx, now.Unix(), limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	for i := range campaigns {
		campaign := &campaigns[i]
		draft, err := recallCampaignDraftFromModel(campaign)
		if err != nil {
			return processed, err
		}
		switch campaign.ExecutionMode {
		case "scheduled_once":
			runKey := fmt.Sprintf("scheduled_once:%d:%d", campaign.Id, campaign.ScheduledAt)
			committed, err := s.commitCampaignRun(
				ctx,
				campaign,
				draft,
				[]string{model.RecallCampaignScheduled, model.RecallCampaignRunning},
				model.RecallCampaignRunning,
				nil,
				map[string]any{"next_run_at": int64(0)},
				runKey,
				now,
			)
			if err != nil {
				return processed, err
			}
			if committed {
				processed++
			}
		case "recurring":
			next, err := NextRecallRun(time.Unix(campaign.NextRunAt, 0), draft.Schedule)
			if err != nil {
				return processed, err
			}
			expected := campaign.NextRunAt
			runKey := fmt.Sprintf("recurring:%d:%d", campaign.Id, expected)
			committed, err := s.commitCampaignRun(
				ctx,
				campaign,
				draft,
				[]string{model.RecallCampaignScheduled, model.RecallCampaignRunning},
				model.RecallCampaignRunning,
				&expected,
				map[string]any{"next_run_at": next.Unix()},
				runKey,
				now,
			)
			if err != nil {
				return processed, err
			}
			if committed {
				processed++
			}
		}
	}
	return processed, nil
}

func (s *RecallCampaignService) commitCampaignRun(
	ctx context.Context,
	campaign *model.RecallCampaign,
	draft RecallCampaignDraft,
	from []string,
	to string,
	expectedNextRunAt *int64,
	fields map[string]any,
	runKey string,
	runAt time.Time,
) (bool, error) {
	snapshotLimit := draft.EnrollmentLimit
	if campaign.ExecutionMode == "recurring" {
		enrolled, err := model.CountRecallCampaignRecipientsWithContext(ctx, campaign.Id)
		if err != nil {
			return false, err
		}
		remaining := int64(draft.EnrollmentLimit) - enrolled
		if remaining <= 0 {
			snapshotLimit = 0
		} else {
			snapshotLimit = int(remaining)
		}
	}
	recipients, exclusions, err := s.audience.Snapshot(ctx, draft, snapshotLimit, runAt)
	if err != nil {
		return false, err
	}
	expiresAt := runAt.Add(time.Duration(campaign.PromotionValidSeconds) * time.Second).Unix()
	if draft.Discount.CouponRedeemBy > 0 && draft.Discount.CouponRedeemBy < expiresAt {
		expiresAt = draft.Discount.CouponRedeemBy
	}
	if expiresAt <= runAt.Unix() {
		return false, fmt.Errorf("recall promotion expiry must be after its campaign run")
	}
	messages, err := initialRecallMessages(recipients, draft.Emails[0], runAt)
	if err != nil {
		return false, err
	}
	for i := range recipients {
		recipients[i].PromotionExpiresAt = expiresAt
	}
	eventData, err := common.Marshal(map[string]any{
		"eligible_total": len(recipients),
		"exclusions":     exclusions,
	})
	if err != nil {
		return false, err
	}
	committed, _, err := model.CommitRecallCampaignRun(
		ctx,
		campaign.Id,
		from,
		to,
		expectedNextRunAt,
		fields,
		recipients,
		messages,
		model.RecallEvent{
			EventType:     "campaign_run",
			Source:        "scheduler",
			SourceEventId: runKey,
			EventData:     string(eventData),
		},
	)
	return committed, err
}

func initialRecallMessages(recipients []model.RecallRecipient, stage RecallEmailStage, runAt time.Time) ([]model.RecallMessage, error) {
	templateJSON, err := common.Marshal(stage.Templates)
	if err != nil {
		return nil, err
	}
	messages := make([]model.RecallMessage, len(recipients))
	for i := range recipients {
		messages[i] = model.RecallMessage{
			StageNo:          stage.StageNo,
			TemplateVersion:  stage.TemplateVersion,
			TemplateSnapshot: string(templateJSON),
			ScheduledAt:      runAt.Add(time.Duration(stage.DelaySeconds) * time.Second).Unix(),
			State:            model.RecallMessageScheduled,
		}
	}
	return messages, nil
}

func recallCampaignActivationFields(draft RecallCampaignDraft, couponID string, activatedAt int64) (map[string]any, error) {
	discountJSON, err := common.Marshal(draft.Discount)
	if err != nil {
		return nil, err
	}
	productJSON, err := common.Marshal(draft.Products)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"stripe_coupon_id": couponID,
		"discount_config":  string(discountJSON),
		"product_scope":    string(productJSON),
		"activated_at":     activatedAt,
	}, nil
}

func recallCampaignGate(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("recall campaign context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !operation_setting.IsRecallCampaignEnabled() {
		return ErrRecallDisabled
	}
	return nil
}

func validateAndNormalizeRecallCampaignDraft(draft RecallCampaignDraft, now time.Time) (RecallCampaignDraft, error) {
	draft.Name = strings.TrimSpace(draft.Name)
	if draft.Name == "" || len(draft.Name) > 128 {
		return RecallCampaignDraft{}, fmt.Errorf("recall campaign name must contain 1 to 128 characters")
	}
	draft.AudienceTemplate = strings.ToLower(strings.TrimSpace(draft.AudienceTemplate))
	draft.Audience.GroupMode = strings.ToLower(strings.TrimSpace(draft.Audience.GroupMode))
	if err := ValidateRecallAudience(draft.AudienceTemplate, draft.Audience); err != nil {
		return RecallCampaignDraft{}, err
	}
	draft.Audience = normalizeRecallAudienceConfig(draft.Audience)

	draft.ExecutionMode = strings.ToLower(strings.TrimSpace(draft.ExecutionMode))
	switch draft.ExecutionMode {
	case "manual":
		draft.Schedule = RecallScheduleConfig{}
	case "scheduled_once":
		if draft.Schedule.ScheduledAt <= now.Unix() {
			return RecallCampaignDraft{}, fmt.Errorf("scheduled recall campaign must run in the future")
		}
		draft.Schedule = RecallScheduleConfig{ScheduledAt: draft.Schedule.ScheduledAt}
	case "recurring":
		if _, err := NextRecallRun(now, draft.Schedule); err != nil {
			return RecallCampaignDraft{}, err
		}
		draft.Schedule.ScheduledAt = 0
		draft.Schedule.Timezone = strings.TrimSpace(draft.Schedule.Timezone)
		draft.Schedule.Frequency = strings.ToLower(strings.TrimSpace(draft.Schedule.Frequency))
		if draft.Schedule.Frequency == "daily" {
			draft.Schedule.Weekday = 0
		}
	default:
		return RecallCampaignDraft{}, fmt.Errorf("unsupported recall execution mode %q", draft.ExecutionMode)
	}

	draft.CouponSource = strings.ToLower(strings.TrimSpace(draft.CouponSource))
	draft.ExistingCouponID = strings.TrimSpace(draft.ExistingCouponID)
	switch draft.CouponSource {
	case "automatic":
		if draft.ExistingCouponID != "" {
			return RecallCampaignDraft{}, fmt.Errorf("automatic recall coupon cannot set an existing coupon ID")
		}
	case "existing":
		if draft.ExistingCouponID == "" {
			return RecallCampaignDraft{}, fmt.Errorf("existing recall coupon ID is required")
		}
	default:
		return RecallCampaignDraft{}, fmt.Errorf("unsupported recall coupon source %q", draft.CouponSource)
	}

	discount, err := normalizeRecallDiscount(draft.Discount)
	if err != nil {
		return RecallCampaignDraft{}, err
	}
	if discount.Type != "percent" && discount.Type != "fixed" {
		return RecallCampaignDraft{}, fmt.Errorf("recall discount type must be percent or fixed")
	}
	if discount.CouponRedeemBy > 0 && discount.CouponRedeemBy <= now.Unix() {
		return RecallCampaignDraft{}, fmt.Errorf("recall coupon redeem-by must be in the future")
	}
	draft.Discount = discount
	draft.Products.TopUpPriceIDs = normalizeRecallStripeIDs(draft.Products.TopUpPriceIDs)
	draft.Products.SubscriptionPriceIDs = normalizeRecallStripeIDs(draft.Products.SubscriptionPriceIDs)
	if len(draft.Products.TopUpPriceIDs)+len(draft.Products.SubscriptionPriceIDs) == 0 {
		return RecallCampaignDraft{}, fmt.Errorf("recall campaign requires at least one Stripe Price")
	}
	if draft.PromotionValidSeconds <= 0 {
		return RecallCampaignDraft{}, fmt.Errorf("recall promotion validity must be positive")
	}
	if draft.EnrollmentLimit < 1 || draft.EnrollmentLimit > 100000 {
		return RecallCampaignDraft{}, fmt.Errorf("recall enrollment limit must be between 1 and 100000")
	}
	if draft.WorkerConcurrency < 1 || draft.WorkerConcurrency > 20 {
		return RecallCampaignDraft{}, fmt.Errorf("recall worker concurrency must be between 1 and 20")
	}
	emails, err := normalizeRecallEmailStages(draft.Emails)
	if err != nil {
		return RecallCampaignDraft{}, err
	}
	draft.Emails = emails
	return draft, nil
}

func normalizeRecallAudienceConfig(cfg RecallAudienceConfig) RecallAudienceConfig {
	cfg.Groups = normalizeRecallStrings(cfg.Groups)
	cfg.PaymentProviders = normalizeRecallStrings(cfg.PaymentProviders)
	cfg.GroupMode = strings.ToLower(strings.TrimSpace(cfg.GroupMode))
	return cfg
}

func normalizeRecallStrings(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func normalizeRecallEmailStages(stages []RecallEmailStage) ([]RecallEmailStage, error) {
	if len(stages) < 1 || len(stages) > 3 {
		return nil, fmt.Errorf("recall campaign requires one to three email stages")
	}
	normalized := make([]RecallEmailStage, len(stages))
	previousDelay := int64(-1)
	for i, stage := range stages {
		if stage.StageNo != i+1 {
			return nil, fmt.Errorf("recall email stages must be unique and ordered from one")
		}
		if stage.DelaySeconds < 0 || (i == 0 && stage.DelaySeconds != 0) || (i > 0 && stage.DelaySeconds <= previousDelay) {
			return nil, fmt.Errorf("recall email stage delays must start at zero and increase")
		}
		if len(stage.Templates) == 0 {
			return nil, fmt.Errorf("recall email stage %d requires templates", stage.StageNo)
		}
		templates := make(map[string]RecallEmailTemplate, len(stage.Templates))
		for language, template := range stage.Templates {
			language = strings.ToLower(strings.TrimSpace(language))
			if language == "" {
				return nil, fmt.Errorf("recall email stage %d has an empty language", stage.StageNo)
			}
			if _, exists := templates[language]; exists {
				return nil, fmt.Errorf("recall email stage %d has duplicate language %q", stage.StageNo, language)
			}
			template.Subject = strings.TrimSpace(template.Subject)
			template.BodyText = strings.TrimSpace(template.BodyText)
			if template.Subject == "" || template.BodyText == "" {
				return nil, fmt.Errorf("recall email stage %d language %q requires subject and body", stage.StageNo, language)
			}
			templates[language] = template
		}
		if _, exists := templates["en"]; !exists {
			return nil, fmt.Errorf("recall email stage %d requires an English template", stage.StageNo)
		}
		normalized[i] = RecallEmailStage{
			StageNo:         stage.StageNo,
			DelaySeconds:    stage.DelaySeconds,
			TemplateVersion: 1,
			Templates:       templates,
		}
		previousDelay = stage.DelaySeconds
	}
	return normalized, nil
}

func recallCampaignModelFromDraft(draft RecallCampaignDraft, actorID int) (*model.RecallCampaign, error) {
	audienceJSON, err := common.Marshal(draft.Audience)
	if err != nil {
		return nil, err
	}
	discountJSON, err := common.Marshal(draft.Discount)
	if err != nil {
		return nil, err
	}
	productJSON, err := common.Marshal(draft.Products)
	if err != nil {
		return nil, err
	}
	emailJSON, err := common.Marshal(draft.Emails)
	if err != nil {
		return nil, err
	}
	recurrenceJSON := ""
	if draft.ExecutionMode == "recurring" {
		encoded, marshalErr := common.Marshal(draft.Schedule)
		if marshalErr != nil {
			return nil, marshalErr
		}
		recurrenceJSON = string(encoded)
	}
	return &model.RecallCampaign{
		Name:                  draft.Name,
		Status:                model.RecallCampaignDraft,
		AudienceTemplate:      draft.AudienceTemplate,
		AudienceConfig:        string(audienceJSON),
		ExecutionMode:         draft.ExecutionMode,
		ScheduledAt:           draft.Schedule.ScheduledAt,
		RecurrenceConfig:      recurrenceJSON,
		CouponSource:          draft.CouponSource,
		StripeCouponId:        draft.ExistingCouponID,
		DiscountConfig:        string(discountJSON),
		ProductScope:          string(productJSON),
		PromotionValidSeconds: draft.PromotionValidSeconds,
		EmailSequenceConfig:   string(emailJSON),
		EnrollmentLimit:       draft.EnrollmentLimit,
		WorkerConcurrency:     draft.WorkerConcurrency,
		CreatedBy:             actorID,
	}, nil
}

func recallCampaignDraftFromModel(campaign *model.RecallCampaign) (RecallCampaignDraft, error) {
	if campaign == nil {
		return RecallCampaignDraft{}, fmt.Errorf("recall campaign is nil")
	}
	draft := RecallCampaignDraft{
		Name:                  campaign.Name,
		AudienceTemplate:      campaign.AudienceTemplate,
		ExecutionMode:         campaign.ExecutionMode,
		CouponSource:          campaign.CouponSource,
		PromotionValidSeconds: campaign.PromotionValidSeconds,
		EnrollmentLimit:       campaign.EnrollmentLimit,
		WorkerConcurrency:     campaign.WorkerConcurrency,
	}
	if campaign.CouponSource == "existing" {
		draft.ExistingCouponID = campaign.StripeCouponId
	}
	if err := common.Unmarshal([]byte(campaign.AudienceConfig), &draft.Audience); err != nil {
		return RecallCampaignDraft{}, fmt.Errorf("decode recall audience config: %w", err)
	}
	if err := common.Unmarshal([]byte(campaign.DiscountConfig), &draft.Discount); err != nil {
		return RecallCampaignDraft{}, fmt.Errorf("decode recall discount config: %w", err)
	}
	if err := common.Unmarshal([]byte(campaign.ProductScope), &draft.Products); err != nil {
		return RecallCampaignDraft{}, fmt.Errorf("decode recall product scope: %w", err)
	}
	if err := common.Unmarshal([]byte(campaign.EmailSequenceConfig), &draft.Emails); err != nil {
		return RecallCampaignDraft{}, fmt.Errorf("decode recall email sequence: %w", err)
	}
	switch campaign.ExecutionMode {
	case "scheduled_once":
		draft.Schedule.ScheduledAt = campaign.ScheduledAt
	case "recurring":
		if err := common.Unmarshal([]byte(campaign.RecurrenceConfig), &draft.Schedule); err != nil {
			return RecallCampaignDraft{}, fmt.Errorf("decode recall recurrence config: %w", err)
		}
	}
	return draft, nil
}

type recallImmutableCampaignDraft struct {
	AudienceTemplate      string
	Audience              RecallAudienceConfig
	ExecutionMode         string
	Schedule              RecallScheduleConfig
	CouponSource          string
	ExistingCouponID      string
	Discount              RecallDiscountConfig
	Products              RecallProductScope
	PromotionValidSeconds int64
	EnrollmentLimit       int
	WorkerConcurrency     int
	EmailStages           []recallImmutableEmailStage
}

type recallImmutableEmailStage struct {
	StageNo      int
	DelaySeconds int64
}

func recallCampaignImmutableDraft(draft RecallCampaignDraft) recallImmutableCampaignDraft {
	emailStages := make([]recallImmutableEmailStage, len(draft.Emails))
	for i, stage := range draft.Emails {
		emailStages[i] = recallImmutableEmailStage{StageNo: stage.StageNo, DelaySeconds: stage.DelaySeconds}
	}
	return recallImmutableCampaignDraft{
		AudienceTemplate:      draft.AudienceTemplate,
		Audience:              draft.Audience,
		ExecutionMode:         draft.ExecutionMode,
		Schedule:              draft.Schedule,
		CouponSource:          draft.CouponSource,
		ExistingCouponID:      draft.ExistingCouponID,
		Discount:              draft.Discount,
		Products:              draft.Products,
		PromotionValidSeconds: draft.PromotionValidSeconds,
		EnrollmentLimit:       draft.EnrollmentLimit,
		WorkerConcurrency:     draft.WorkerConcurrency,
		EmailStages:           emailStages,
	}
}

func incrementRecallEmailTemplateVersions(current []RecallEmailStage, next []RecallEmailStage) ([]RecallEmailStage, error) {
	if len(current) != len(next) {
		return nil, fmt.Errorf("activated recall email stages cannot be added or removed")
	}
	updated := make([]RecallEmailStage, len(next))
	for i := range next {
		if current[i].StageNo != next[i].StageNo || current[i].DelaySeconds != next[i].DelaySeconds {
			return nil, fmt.Errorf("activated recall email stage numbers and delays are immutable")
		}
		updated[i] = next[i]
		updated[i].TemplateVersion = current[i].TemplateVersion
		if !reflect.DeepEqual(current[i].Templates, next[i].Templates) {
			updated[i].TemplateVersion++
		}
	}
	return updated, nil
}

func NextRecallRun(after time.Time, cfg RecallScheduleConfig) (time.Time, error) {
	timezone := strings.TrimSpace(cfg.Timezone)
	if timezone == "" || timezone == "Local" {
		return time.Time{}, fmt.Errorf("recall recurrence requires an IANA timezone")
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid recall recurrence timezone %q: %w", timezone, err)
	}
	frequency := strings.ToLower(strings.TrimSpace(cfg.Frequency))
	if cfg.Hour < 0 || cfg.Hour > 23 || cfg.Minute < 0 || cfg.Minute > 59 {
		return time.Time{}, fmt.Errorf("recall recurrence hour or minute is invalid")
	}
	localAfter := after.In(location)
	wallClockOnDay := func(day time.Time) (time.Time, bool) {
		candidate := time.Date(day.Year(), day.Month(), day.Day(), cfg.Hour, cfg.Minute, 0, 0, location)
		localCandidate := candidate.In(location)
		valid := localCandidate.Year() == day.Year() &&
			localCandidate.Month() == day.Month() &&
			localCandidate.Day() == day.Day() &&
			localCandidate.Hour() == cfg.Hour &&
			localCandidate.Minute() == cfg.Minute
		return candidate, valid
	}
	var candidate time.Time
	switch frequency {
	case "daily":
		for days := 0; days <= 366; days++ {
			day := localAfter.AddDate(0, 0, days)
			possible, valid := wallClockOnDay(day)
			if valid && possible.After(localAfter) {
				candidate = possible
				break
			}
		}
		if candidate.IsZero() {
			return time.Time{}, fmt.Errorf("could not find the next daily recall wall clock")
		}
	case "weekly":
		if cfg.Weekday < int(time.Sunday) || cfg.Weekday > int(time.Saturday) {
			return time.Time{}, fmt.Errorf("recall weekly recurrence weekday must be between 0 and 6")
		}
		days := (cfg.Weekday - int(localAfter.Weekday()) + 7) % 7
		for weeks := 0; weeks <= 53; weeks++ {
			day := localAfter.AddDate(0, 0, days+weeks*7)
			possible, valid := wallClockOnDay(day)
			if valid && possible.After(localAfter) {
				candidate = possible
				break
			}
		}
		if candidate.IsZero() {
			return time.Time{}, fmt.Errorf("could not find the next weekly recall wall clock")
		}
	default:
		return time.Time{}, fmt.Errorf("recall recurrence frequency must be daily or weekly")
	}
	return candidate.UTC(), nil
}
