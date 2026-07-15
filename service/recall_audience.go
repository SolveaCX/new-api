package service

import (
	"context"
	"fmt"
	"net/mail"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

const recallDaySeconds int64 = 24 * 60 * 60

var recallAudienceExclusionKeys = []string{
	"payment_exists",
	"recent_api_activity",
	"active_subscription",
	"opted_out",
	"invalid_email",
	"unverified_email",
	"group_filtered",
	"threshold_not_met",
}

type RecallAudienceSelector struct {
	MainBatchSize int
	LogBatchSize  int
}

func NewRecallAudienceSelector() *RecallAudienceSelector {
	return &RecallAudienceSelector{MainBatchSize: 500, LogBatchSize: 200}
}

func ValidateRecallAudience(template string, cfg RecallAudienceConfig) error {
	switch template {
	case "first_purchase", "lapsed_payer", "expired_subscription":
	default:
		return fmt.Errorf("unknown recall audience template %q", template)
	}
	if cfg.RegistrationAgeDays < 0 || cfg.MinRequestCount < 0 || cfg.MaxQuota < 0 ||
		cfg.MinPaidAmount < 0 || cfg.LastAPICallAgeDays < 0 || cfg.LastPaymentAgeDays < 0 ||
		cfg.SubscriptionExpiredDays < 0 || cfg.MinSubscriptionAmount < 0 || cfg.MinSubscriptionCount < 0 {
		return fmt.Errorf("recall audience thresholds must not be negative")
	}
	if cfg.GroupMode != "" && cfg.GroupMode != "allow" && cfg.GroupMode != "block" {
		return fmt.Errorf("unknown recall audience group mode %q", cfg.GroupMode)
	}
	if len(cfg.Groups) == 0 && cfg.GroupMode != "" {
		return fmt.Errorf("recall audience group mode requires groups")
	}
	if len(cfg.Groups) > 0 && cfg.GroupMode == "" {
		return fmt.Errorf("recall audience groups require a group mode")
	}
	for _, group := range cfg.Groups {
		if strings.TrimSpace(group) == "" {
			return fmt.Errorf("recall audience groups must not contain empty values")
		}
	}
	for _, provider := range cfg.PaymentProviders {
		if strings.TrimSpace(provider) == "" {
			return fmt.Errorf("recall audience payment providers must not contain empty values")
		}
	}
	return nil
}

func (selector *RecallAudienceSelector) Preview(ctx context.Context, draft RecallCampaignDraft, sampleSize int, now int64) (RecallAudiencePreview, error) {
	preview := RecallAudiencePreview{
		Sample:     make([]RecallAudienceCandidate, 0),
		Exclusions: newRecallAudienceExclusions(),
	}
	if sampleSize < 0 {
		return preview, fmt.Errorf("recall audience sample size must not be negative")
	}
	exclusions, err := selector.iterate(ctx, draft, now, func(selection recallAudienceSelection) bool {
		candidate := selection.Candidate
		preview.EligibleTotal++
		if len(preview.Sample) < sampleSize {
			candidate.SnapshotJSON = ""
			preview.Sample = append(preview.Sample, candidate)
		}
		return true
	})
	preview.Exclusions = exclusions
	return preview, err
}

func (selector *RecallAudienceSelector) Snapshot(ctx context.Context, draft RecallCampaignDraft, limit int, now int64) ([]model.RecallRecipient, error) {
	recipients := make([]model.RecallRecipient, 0)
	if limit < 0 {
		return nil, fmt.Errorf("recall audience snapshot limit must not be negative")
	}
	if limit == 0 {
		return recipients, nil
	}
	_, err := selector.iterate(ctx, draft, now, func(selection recallAudienceSelection) bool {
		candidate := selection.Candidate
		recipients = append(recipients, model.RecallRecipient{
			UserId:              candidate.UserID,
			EligibilitySnapshot: candidate.SnapshotJSON,
			EmailSnapshot:       selection.Email,
			LanguageSnapshot:    candidate.Language,
			State:               model.RecallRecipientQueued,
		})
		return len(recipients) < limit
	})
	return recipients, err
}

func (selector *RecallAudienceSelector) iterate(
	ctx context.Context,
	draft RecallCampaignDraft,
	now int64,
	onEligible func(recallAudienceSelection) bool,
) (map[string]int64, error) {
	exclusions := newRecallAudienceExclusions()
	if selector == nil || selector.MainBatchSize <= 0 || selector.LogBatchSize <= 0 {
		return exclusions, fmt.Errorf("recall audience batch sizes must be positive")
	}
	if err := ValidateRecallAudience(draft.AudienceTemplate, draft.Audience); err != nil {
		return exclusions, err
	}
	query := recallCandidateQuery(draft, now, selector.MainBatchSize)
	for {
		if err := ctx.Err(); err != nil {
			return exclusions, err
		}
		facts, err := model.ListRecallCandidateFacts(query)
		if err != nil {
			return exclusions, err
		}
		if len(facts) == 0 {
			return exclusions, nil
		}

		candidates := make([]recallAudienceSelection, 0, len(facts))
		for _, fact := range facts {
			if err := ctx.Err(); err != nil {
				return exclusions, err
			}
			candidate, reason, err := recallAudienceCandidate(draft, fact, now)
			if err != nil {
				return exclusions, err
			}
			if reason != "" {
				exclusions[reason]++
				continue
			}
			candidates = append(candidates, candidate)
		}

		recentlyActive := make(map[int]struct{})
		if draft.Audience.LastAPICallAgeDays > 0 && len(candidates) > 0 {
			userIDs := make([]int, len(candidates))
			for i := range candidates {
				userIDs[i] = candidates[i].Candidate.UserID
			}
			if err := ctx.Err(); err != nil {
				return exclusions, err
			}
			recentlyActive, err = model.FindRecentlyActiveRecallUserIDs(
				userIDs,
				now-int64(draft.Audience.LastAPICallAgeDays)*recallDaySeconds,
				selector.LogBatchSize,
			)
			if err != nil {
				return exclusions, err
			}
		}
		for _, candidate := range candidates {
			if _, active := recentlyActive[candidate.Candidate.UserID]; active {
				exclusions["recent_api_activity"]++
				continue
			}
			if !onEligible(candidate) {
				return exclusions, nil
			}
		}

		query.AfterUserID = facts[len(facts)-1].User.Id
		if len(facts) < selector.MainBatchSize {
			return exclusions, nil
		}
	}
}

func newRecallAudienceExclusions() map[string]int64 {
	exclusions := make(map[string]int64, len(recallAudienceExclusionKeys)+1)
	for _, key := range recallAudienceExclusionKeys {
		exclusions[key] = 0
	}
	exclusions["disabled"] = 0
	return exclusions
}

func recallCandidateQuery(draft RecallCampaignDraft, now int64, limit int) model.RecallCandidateQuery {
	cfg := draft.Audience
	paymentProviders := make([]string, len(cfg.PaymentProviders))
	for i, provider := range cfg.PaymentProviders {
		paymentProviders[i] = strings.TrimSpace(provider)
	}
	return model.RecallCandidateQuery{
		Template:              draft.AudienceTemplate,
		Now:                   now,
		RegistrationBefore:    now - int64(cfg.RegistrationAgeDays)*recallDaySeconds,
		LastPaymentBefore:     now - int64(cfg.LastPaymentAgeDays)*recallDaySeconds,
		SubscriptionBefore:    now - int64(cfg.SubscriptionExpiredDays)*recallDaySeconds,
		MaxQuota:              cfg.MaxQuota,
		MinRequestCount:       cfg.MinRequestCount,
		MinPaidAmount:         cfg.MinPaidAmount,
		MinSubscriptionAmount: cfg.MinSubscriptionAmount,
		MinSubscriptionCount:  cfg.MinSubscriptionCount,
		PaymentProviders:      paymentProviders,
		Groups:                append([]string(nil), cfg.Groups...),
		GroupMode:             cfg.GroupMode,
		Limit:                 limit,
	}
}

type recallEligibilitySnapshot struct {
	Template              string  `json:"template"`
	UserID                int     `json:"user_id"`
	RegisteredAt          int64   `json:"registered_at"`
	Quota                 int     `json:"quota"`
	RequestCount          int     `json:"request_count"`
	PaidAmount            float64 `json:"paid_amount"`
	LastPaymentAt         int64   `json:"last_payment_at"`
	SubscriptionAmount    float64 `json:"subscription_amount"`
	SubscriptionCount     int64   `json:"subscription_count"`
	LastSubscriptionEndAt int64   `json:"last_subscription_end_at"`
}

type recallAudienceSelection struct {
	Candidate RecallAudienceCandidate
	Email     string
}

func recallAudienceCandidate(draft RecallCampaignDraft, fact model.RecallCandidateFact, now int64) (recallAudienceSelection, string, error) {
	user := fact.User
	if user.Status != common.UserStatusEnabled {
		return recallAudienceSelection{}, "disabled", nil
	}
	email, ok := recallAudienceEmail(user.Email)
	if !ok {
		return recallAudienceSelection{}, "invalid_email", nil
	}
	setting, err := recallAudienceUserSetting(user.Setting)
	if err != nil {
		return recallAudienceSelection{}, "", fmt.Errorf("decode recall setting for user %d: %w", user.Id, err)
	}
	if setting.RecallMarketingOptOut {
		return recallAudienceSelection{}, "opted_out", nil
	}
	if draft.Audience.RequireVerifiedEmail && user.EmailVerifiedAt <= 0 {
		return recallAudienceSelection{}, "unverified_email", nil
	}
	if !recallAudienceGroupAllowed(user.Group, draft.Audience.Groups, draft.Audience.GroupMode) {
		return recallAudienceSelection{}, "group_filtered", nil
	}
	if reason := recallTemplateExclusion(draft, fact, now); reason != "" {
		return recallAudienceSelection{}, reason, nil
	}
	snapshot, err := common.Marshal(recallEligibilitySnapshot{
		Template:              draft.AudienceTemplate,
		UserID:                user.Id,
		RegisteredAt:          user.CreatedAt,
		Quota:                 user.Quota,
		RequestCount:          user.RequestCount,
		PaidAmount:            fact.PaidAmount,
		LastPaymentAt:         fact.LastPaymentAt,
		SubscriptionAmount:    fact.SubscriptionAmount,
		SubscriptionCount:     fact.SubscriptionCount,
		LastSubscriptionEndAt: fact.LastSubscriptionEndAt,
	})
	if err != nil {
		return recallAudienceSelection{}, "", err
	}
	return recallAudienceSelection{
		Candidate: RecallAudienceCandidate{
			UserID:       user.Id,
			EmailMasked:  model.MaskInvitationIdentity(email, ""),
			Language:     recallAudienceLanguage(setting.Language, user.BrowserLang),
			SnapshotJSON: string(snapshot),
		},
		Email: email,
	}, "", nil
}

func recallTemplateExclusion(draft RecallCampaignDraft, fact model.RecallCandidateFact, now int64) string {
	cfg := draft.Audience
	switch draft.AudienceTemplate {
	case "first_purchase":
		if fact.HasPayment {
			return "payment_exists"
		}
		if fact.User.CreatedAt > now-int64(cfg.RegistrationAgeDays)*recallDaySeconds ||
			fact.User.Quota > cfg.MaxQuota || fact.User.RequestCount < cfg.MinRequestCount {
			return "threshold_not_met"
		}
	case "lapsed_payer":
		if !fact.HasPayment || fact.PaidAmount < cfg.MinPaidAmount || fact.User.Quota > cfg.MaxQuota ||
			fact.LastPaymentAt > now-int64(cfg.LastPaymentAgeDays)*recallDaySeconds {
			return "threshold_not_met"
		}
	case "expired_subscription":
		if fact.HasActiveSubscription {
			return "active_subscription"
		}
		if fact.SubscriptionCount < int64(cfg.MinSubscriptionCount) ||
			fact.SubscriptionAmount < cfg.MinSubscriptionAmount ||
			fact.LastSubscriptionEndAt == 0 ||
			fact.LastSubscriptionEndAt > now-int64(cfg.SubscriptionExpiredDays)*recallDaySeconds ||
			(len(cfg.PaymentProviders) > 0 && fact.SubscriptionAmount <= 0) {
			return "threshold_not_met"
		}
	}
	return ""
}

func recallAudienceEmail(stored string) (string, bool) {
	trimmed := strings.TrimSpace(stored)
	if trimmed == "" {
		return "", false
	}
	parsed, err := mail.ParseAddress(trimmed)
	if err != nil || parsed.Address != trimmed {
		return "", false
	}
	return trimmed, true
}

func recallAudienceUserSetting(raw string) (dto.UserSetting, error) {
	setting := dto.UserSetting{}
	if strings.TrimSpace(raw) == "" {
		return setting, nil
	}
	if err := common.Unmarshal([]byte(raw), &setting); err != nil {
		return dto.UserSetting{}, err
	}
	return setting, nil
}

func recallAudienceGroupAllowed(userGroup string, groups []string, mode string) bool {
	if len(groups) == 0 {
		return true
	}
	matched := false
	for _, group := range groups {
		if strings.TrimSpace(group) == strings.TrimSpace(userGroup) {
			matched = true
			break
		}
	}
	if mode == "allow" {
		return matched
	}
	return !matched
}

func recallAudienceLanguage(settingLanguage string, browserLanguage string) string {
	if language := recallAudiencePrimaryLanguage(settingLanguage); language != "" {
		return language
	}
	if language := recallAudiencePrimaryLanguage(browserLanguage); language != "" {
		return language
	}
	return "en"
}

func recallAudiencePrimaryLanguage(language string) string {
	language = strings.TrimSpace(strings.ToLower(language))
	if language == "" {
		return ""
	}
	return strings.FieldsFunc(language, func(r rune) bool { return r == '-' || r == '_' })[0]
}
