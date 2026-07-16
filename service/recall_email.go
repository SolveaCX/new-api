package service

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

const (
	recallEmailLeaseSeconds = int64(60)
	recallEmailMaxAttempts  = 5
)

var ErrRecallEmailLeaseLost = errors.New("recall email message lease was lost")

type RecallEmailSender func(subject, receiver, content, messageID string) error

type RecallEmailWorker struct {
	sender   RecallEmailSender
	audience *RecallAudienceSelector
	claims   *RecallClaimService
	now      func() time.Time
	owner    string
}

type RecallEmailRenderInput struct {
	Template            RecallEmailTemplate
	RecipientName       string
	PromotionCodeMasked string
	ExpiresAt           int64
	ProductSummary      string
	ClaimURL            string
	UnsubscribeURL      string
}

func NewRecallEmailWorker(sender RecallEmailSender, audience *RecallAudienceSelector, claims *RecallClaimService, owner string) *RecallEmailWorker {
	if sender == nil {
		sender = common.SendEmailWithMessageID
	}
	if audience == nil {
		audience = NewRecallAudienceSelector()
	}
	if claims == nil {
		claims = NewRecallClaimService()
	}
	return &RecallEmailWorker{
		sender:   sender,
		audience: audience,
		claims:   claims,
		now:      time.Now,
		owner:    strings.TrimSpace(owner),
	}
}

func (w *RecallEmailWorker) RunBatch(ctx context.Context, limit int) (int, error) {
	if w == nil || limit <= 0 {
		return 0, nil
	}
	if w.owner == "" {
		return 0, fmt.Errorf("recall email worker owner is required")
	}
	now := w.now().Unix()
	messageIDs, err := model.ListDueRecallMessageIDs(now, limit)
	if err != nil {
		return 0, err
	}
	type leasedEmail struct {
		item *model.RecallEmailWorkItem
	}
	leased := make([]leasedEmail, 0, len(messageIDs))
	activityChecks := make([]model.RecallAPIActivityCheck, 0, len(messageIDs))
	processed := 0
	var firstErr error
	for _, messageID := range messageIDs {
		if err := ctx.Err(); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			break
		}
		won, leaseErr := model.LeaseRecallMessage(messageID, w.owner, now, now+recallEmailLeaseSeconds)
		if leaseErr != nil {
			if firstErr == nil {
				firstErr = leaseErr
			}
			continue
		}
		if !won {
			continue
		}
		processed++
		item, loadErr := model.GetRecallEmailWorkItemForLeaseWithContext(ctx, messageID, w.owner)
		if loadErr != nil {
			if firstErr == nil {
				firstErr = loadErr
			}
			continue
		}
		leased = append(leased, leasedEmail{item: item})
		activityChecks = append(activityChecks, model.RecallAPIActivityCheck{
			MessageId: item.Message.Id,
			UserId:    item.Recipient.UserId,
			After:     item.Recipient.CreatedAt,
		})
	}

	activeMessageIDs, activityErr := model.FindRecallMessageIDsWithAPIActivityAfterWithContext(ctx, activityChecks, w.audience.LogBatchSize)
	if activityErr != nil {
		if firstErr == nil {
			firstErr = activityErr
		}
		return processed, firstErr
	}
	for _, leasedMessage := range leased {
		_, recentlyActive := activeMessageIDs[leasedMessage.item.Message.Id]
		if processErr := w.processLeasedItem(ctx, leasedMessage.item, recentlyActive); processErr != nil && !errors.Is(processErr, ErrRecallEmailLeaseLost) && firstErr == nil {
			firstErr = processErr
		}
	}
	return processed, firstErr
}

func (w *RecallEmailWorker) ProcessLeased(ctx context.Context, messageID int64) error {
	if w == nil || w.owner == "" {
		return fmt.Errorf("recall email worker owner is required")
	}
	item, err := model.GetRecallEmailWorkItemForLeaseWithContext(ctx, messageID, w.owner)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRecallEmailLeaseLost
		}
		return err
	}
	activeMessageIDs, err := model.FindRecallMessageIDsWithAPIActivityAfterWithContext(ctx, []model.RecallAPIActivityCheck{{
		MessageId: item.Message.Id,
		UserId:    item.Recipient.UserId,
		After:     item.Recipient.CreatedAt,
	}}, w.audience.LogBatchSize)
	if err != nil {
		return err
	}
	_, recentlyActive := activeMessageIDs[item.Message.Id]
	return w.processLeasedItem(ctx, item, recentlyActive)
}

func (w *RecallEmailWorker) processLeasedItem(ctx context.Context, item *model.RecallEmailWorkItem, recentlyActive bool) error {
	now := w.now().Unix()
	if item.Message.LeaseExpiresAt <= now {
		return ErrRecallEmailLeaseLost
	}
	stopReason, err := w.recallEmailStopReason(ctx, item, recentlyActive, now)
	if err != nil {
		return err
	}
	if stopReason != "" {
		cancelled, err := model.CancelRecallEmailFlowWithContext(
			ctx,
			item.Message.Id,
			item.Recipient.Id,
			w.owner,
			item.Message.LeaseExpiresAt,
			stopReason,
			now,
		)
		if err != nil {
			return err
		}
		if !cancelled {
			return ErrRecallEmailLeaseLost
		}
		return nil
	}

	providerMessageID := strings.TrimSpace(item.Message.ProviderMessageId)
	if providerMessageID == "" {
		providerMessageID, err = recallEmailMessageID(item.Recipient.Id, item.Message.StageNo)
		if err != nil {
			return w.finishPreAcceptError(ctx, item, "message_id_invalid", false)
		}
		var won bool
		providerMessageID, won, err = model.EnsureRecallMessageProviderIDWithContext(
			ctx,
			item.Message.Id,
			w.owner,
			item.Message.LeaseExpiresAt,
			providerMessageID,
		)
		if err != nil {
			return err
		}
		if !won {
			return ErrRecallEmailLeaseLost
		}
	}
	if err := common.ValidateEmailMessageID(providerMessageID); err != nil {
		return w.finishPreAcceptError(ctx, item, "message_id_invalid", false)
	}
	next, err := nextRecallEmailMessage(item, now)
	if err != nil {
		return w.finishPreAcceptError(ctx, item, "next_stage_invalid", false)
	}

	rawClaim, err := w.claims.IssueClaim(ctx, item.Message.Id, w.owner, item.Message.LeaseExpiresAt)
	if err != nil {
		if errors.Is(err, ErrRecallClaimLeaseLost) {
			return ErrRecallEmailLeaseLost
		}
		return w.finishPreAcceptError(ctx, item, "claim_issue_failed", true)
	}
	unsubscribeToken, err := w.claims.CreateUnsubscribeToken(item.User.Id, time.Unix(item.Recipient.PromotionExpiresAt, 0))
	if err != nil {
		return w.finishPreAcceptError(ctx, item, "unsubscribe_token_failed", true)
	}
	template, err := recallEmailTemplateForLanguage(item.Message.TemplateSnapshot, item.Recipient.LanguageSnapshot)
	if err != nil {
		return w.finishPreAcceptError(ctx, item, "template_invalid", false)
	}
	baseOrigin := strings.TrimRight(strings.TrimSpace(topUpBaseOrigin()), "/")
	claimURL := baseOrigin + "/console/recall/claim?claim=" + url.QueryEscape(rawClaim)
	unsubscribeURL := baseOrigin + "/api/recall/unsubscribe?token=" + url.QueryEscape(unsubscribeToken)
	productSummary, err := recallEmailProductSummary(item.Campaign.ProductScope)
	if err != nil {
		return w.finishPreAcceptError(ctx, item, "product_scope_invalid", false)
	}
	recipientName := strings.TrimSpace(item.User.DisplayName)
	if recipientName == "" {
		recipientName = strings.TrimSpace(item.User.Username)
	}
	subject, htmlBody, err := RenderRecallEmail(RecallEmailRenderInput{
		Template:            template,
		RecipientName:       recipientName,
		PromotionCodeMasked: model.MaskPromotionCode(item.Recipient.PromotionCode),
		ExpiresAt:           item.Recipient.PromotionExpiresAt,
		ProductSummary:      productSummary,
		ClaimURL:            claimURL,
		UnsubscribeURL:      unsubscribeURL,
	})
	if err != nil {
		return w.finishPreAcceptError(ctx, item, "render_invalid", false)
	}
	sending, err := model.MarkRecallMessageSendingWithContext(ctx, item.Message.Id, w.owner, item.Message.LeaseExpiresAt)
	if err != nil {
		return err
	}
	if !sending {
		return ErrRecallEmailLeaseLost
	}

	if err := w.sender(subject, item.Recipient.EmailSnapshot, htmlBody, providerMessageID); err != nil {
		if common.IsEmailSendUncertain(err) {
			won, updateErr := model.CompleteRecallMessageLease(
				item.Message.Id,
				w.owner,
				item.Message.LeaseExpiresAt,
				model.RecallMessageSending,
				model.RecallMessageUncertain,
				map[string]any{
					"attempt_count":       item.Message.AttemptCount + 1,
					"next_attempt_at":     int64(0),
					"provider_message_id": providerMessageID,
					"last_error_code":     "smtp_uncertain",
					"last_error_message":  "",
				},
			)
			if updateErr != nil {
				return updateErr
			}
			if !won {
				return ErrRecallEmailLeaseLost
			}
			return nil
		}
		return w.finishSendingError(ctx, item, "smtp_definite", true)
	}
	acceptedAt := w.now().Unix()
	if next != nil && item.Recipient.FirstSentAt == 0 {
		next.ScheduledAt += acceptedAt - now
	}

	accepted, err := model.AcceptRecallMessageAndScheduleNextWithContext(
		ctx,
		item.Message.Id,
		w.owner,
		item.Message.LeaseExpiresAt,
		acceptedAt,
		next,
	)
	if err != nil {
		return err
	}
	if !accepted {
		return ErrRecallEmailLeaseLost
	}
	return nil
}

func (w *RecallEmailWorker) recallEmailStopReason(ctx context.Context, item *model.RecallEmailWorkItem, recentlyActive bool, now int64) (string, error) {
	switch item.Campaign.Status {
	case model.RecallCampaignScheduled, model.RecallCampaignRunning, model.RecallCampaignCompleted:
	case model.RecallCampaignPaused:
		return "campaign_paused", nil
	case model.RecallCampaignCancelled:
		return "campaign_cancelled", nil
	default:
		return "campaign_inactive", nil
	}
	if !operation_setting.IsRecallCampaignEnabled() {
		return "campaign_disabled", nil
	}
	if item.Recipient.State == model.RecallRecipientConverted || item.Recipient.ConvertedAt > 0 {
		return "recipient_converted", nil
	}
	if item.Recipient.State == model.RecallRecipientSuppressed {
		return "recipient_suppressed", nil
	}
	if item.Recipient.PromotionExpiresAt <= now {
		return "promotion_expired", nil
	}
	if item.Recipient.StripePromotionCodeId == nil || strings.TrimSpace(*item.Recipient.StripePromotionCodeId) == "" || strings.TrimSpace(item.Recipient.PromotionCode) == "" {
		return "promotion_unavailable", nil
	}
	if item.User.Status != common.UserStatusEnabled {
		return "user_disabled", nil
	}
	snapshotEmail, snapshotOK := recallAudienceEmail(item.Recipient.EmailSnapshot)
	currentEmail, currentOK := recallAudienceEmail(item.User.Email)
	if !snapshotOK || !currentOK || !strings.EqualFold(snapshotEmail, currentEmail) {
		return "email_unavailable", nil
	}
	if item.User.GetSetting().RecallMarketingOptOut {
		return "user_opted_out", nil
	}
	paid, err := model.HasRecallPaymentAfterWithContext(ctx, item.Recipient.UserId, item.Recipient.CreatedAt)
	if err != nil {
		return "", err
	}
	if paid {
		return "payment_after_enrollment", nil
	}
	if recentlyActive {
		return "api_activity_after_enrollment", nil
	}
	return "", nil
}

func (w *RecallEmailWorker) finishPreAcceptError(ctx context.Context, item *model.RecallEmailWorkItem, errorCode string, retryable bool) error {
	return w.finishError(ctx, item, model.RecallMessageLeased, errorCode, retryable)
}

func (w *RecallEmailWorker) finishSendingError(ctx context.Context, item *model.RecallEmailWorkItem, errorCode string, retryable bool) error {
	return w.finishError(ctx, item, model.RecallMessageSending, errorCode, retryable)
}

func (w *RecallEmailWorker) finishError(ctx context.Context, item *model.RecallEmailWorkItem, from string, errorCode string, retryable bool) error {
	attemptCount := item.Message.AttemptCount + 1
	state := model.RecallMessageFailed
	fields := map[string]any{
		"attempt_count":      attemptCount,
		"next_attempt_at":    int64(0),
		"failed_at":          w.now().Unix(),
		"last_error_code":    errorCode,
		"last_error_message": "",
	}
	if retryable && attemptCount < recallEmailMaxAttempts {
		state = model.RecallMessageRetryWait
		fields["next_attempt_at"] = w.now().Add(recallEmailRetryDelay(attemptCount)).Unix()
		fields["failed_at"] = int64(0)
	}
	won, err := model.CompleteRecallMessageLease(
		item.Message.Id,
		w.owner,
		item.Message.LeaseExpiresAt,
		from,
		state,
		fields,
	)
	if err != nil {
		return err
	}
	if !won {
		return ErrRecallEmailLeaseLost
	}
	return nil
}

func recallEmailRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := 30 * time.Second
	for step := 1; step < attempt && delay < time.Hour; step++ {
		delay *= 2
		if delay > time.Hour {
			delay = time.Hour
		}
	}
	return delay
}

func recallEmailMessageID(recipientID int64, stageNo int) (string, error) {
	if recipientID <= 0 || stageNo < 1 || stageNo > 3 {
		return "", fmt.Errorf("invalid recall email Message-ID inputs")
	}
	domain, err := common.EmailMessageIDDomain()
	if err != nil {
		return "", err
	}
	messageID := fmt.Sprintf("<recall-%d-%d@%s>", recipientID, stageNo, domain)
	if err := common.ValidateEmailMessageID(messageID); err != nil {
		return "", err
	}
	return messageID, nil
}

func recallEmailTemplateForLanguage(snapshot string, language string) (RecallEmailTemplate, error) {
	templates := make(map[string]RecallEmailTemplate)
	if err := common.Unmarshal([]byte(snapshot), &templates); err != nil {
		return RecallEmailTemplate{}, err
	}
	if template, ok := templates[language]; ok {
		return template, nil
	}
	if template, ok := templates["en"]; ok {
		return template, nil
	}
	return RecallEmailTemplate{}, fmt.Errorf("recall email template has no exact language or English fallback")
}

func nextRecallEmailMessage(item *model.RecallEmailWorkItem, acceptedAt int64) (*model.RecallMessage, error) {
	stages := make([]RecallEmailStage, 0)
	if err := common.Unmarshal([]byte(item.Campaign.EmailSequenceConfig), &stages); err != nil {
		return nil, err
	}
	if len(stages) < 1 || len(stages) > 3 {
		return nil, fmt.Errorf("recall email sequence must contain one to three stages")
	}
	var nextStage *RecallEmailStage
	for index := range stages {
		if stages[index].StageNo == item.Message.StageNo+1 {
			nextStage = &stages[index]
			break
		}
	}
	if nextStage == nil {
		return nil, nil
	}
	templateSnapshot, err := common.Marshal(nextStage.Templates)
	if err != nil {
		return nil, err
	}
	firstAcceptedAt := item.Recipient.FirstSentAt
	if firstAcceptedAt == 0 {
		firstAcceptedAt = acceptedAt
	}
	return &model.RecallMessage{
		StageNo:          nextStage.StageNo,
		TemplateVersion:  nextStage.TemplateVersion,
		TemplateSnapshot: string(templateSnapshot),
		ScheduledAt:      firstAcceptedAt + nextStage.DelaySeconds,
		State:            model.RecallMessageScheduled,
	}, nil
}

func recallEmailProductSummary(productScopeJSON string) (string, error) {
	products := RecallProductScope{}
	if err := common.Unmarshal([]byte(productScopeJSON), &products); err != nil {
		return "", err
	}
	hasTopUps := len(products.TopUpPriceIDs) > 0
	hasSubscriptions := len(products.SubscriptionPriceIDs) > 0
	switch {
	case hasTopUps && hasSubscriptions:
		return "Top-ups and subscriptions", nil
	case hasTopUps:
		return "Top-ups", nil
	case hasSubscriptions:
		return "Subscriptions", nil
	default:
		return "Eligible products", nil
	}
}

func RenderRecallEmail(input RecallEmailRenderInput) (subject string, htmlBody string, err error) {
	if strings.ContainsAny(input.Template.Subject, "\r\n") {
		return "", "", fmt.Errorf("recall email subject must not contain CR or LF")
	}
	paragraphs := make([]string, 0)
	bodyText := strings.ReplaceAll(input.Template.BodyText, "\r\n", "\n")
	bodyText = strings.ReplaceAll(bodyText, "\r", "\n")
	for _, line := range strings.Split(bodyText, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		paragraphs = append(paragraphs, "<p>"+html.EscapeString(line)+"</p>")
	}
	expires := time.Unix(input.ExpiresAt, 0).UTC().Format(time.RFC1123)
	htmlBody = "<!doctype html><html><body>" +
		"<p>Hello " + html.EscapeString(input.RecipientName) + ",</p>" +
		strings.Join(paragraphs, "") +
		"<p>Offer code: <code>" + html.EscapeString(input.PromotionCodeMasked) + "</code></p>" +
		"<p>Eligible for: " + html.EscapeString(input.ProductSummary) + "</p>" +
		"<p>Expires: " + html.EscapeString(expires) + "</p>" +
		"<p><a href=\"" + html.EscapeString(input.ClaimURL) + "\">Claim your offer</a></p>" +
		"<p><a href=\"" + html.EscapeString(input.UnsubscribeURL) + "\">Unsubscribe</a></p>" +
		"</body></html>"
	return input.Template.Subject, htmlBody, nil
}
