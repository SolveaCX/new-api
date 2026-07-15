package model

import "fmt"

const (
	RecallCampaignDraft     = "draft"
	RecallCampaignScheduled = "scheduled"
	RecallCampaignRunning   = "running"
	RecallCampaignPaused    = "paused"
	RecallCampaignCancelled = "cancelled"
	RecallCampaignCompleted = "completed"
)

type RecallCampaign struct {
	Id                    int64  `json:"id" gorm:"primaryKey"`
	Name                  string `json:"name" gorm:"type:varchar(128);not null"`
	Status                string `json:"status" gorm:"type:varchar(24);not null;index"`
	AudienceTemplate      string `json:"audience_template" gorm:"type:varchar(32);not null"`
	AudienceConfig        string `json:"audience_config" gorm:"type:text;not null"`
	ExecutionMode         string `json:"execution_mode" gorm:"type:varchar(24);not null"`
	ScheduledAt           int64  `json:"scheduled_at" gorm:"index"`
	RecurrenceConfig      string `json:"recurrence_config" gorm:"type:text"`
	NextRunAt             int64  `json:"next_run_at" gorm:"index"`
	CouponSource          string `json:"coupon_source" gorm:"type:varchar(16);not null"`
	StripeCouponId        string `json:"stripe_coupon_id" gorm:"type:varchar(128);index"`
	DiscountConfig        string `json:"discount_config" gorm:"type:text;not null"`
	ProductScope          string `json:"product_scope" gorm:"type:text;not null"`
	PromotionValidSeconds int64  `json:"promotion_valid_seconds"`
	EmailSequenceConfig   string `json:"email_sequence_config" gorm:"type:text;not null"`
	EnrollmentLimit       int    `json:"enrollment_limit"`
	WorkerConcurrency     int    `json:"worker_concurrency"`
	CreatedBy             int    `json:"created_by" gorm:"index"`
	CreatedAt             int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt             int64  `json:"updated_at" gorm:"autoUpdateTime"`
	ActivatedAt           int64  `json:"activated_at"`
	CompletedAt           int64  `json:"completed_at"`
}

func CreateRecallCampaign(campaign *RecallCampaign) error {
	return DB.Create(campaign).Error
}

func GetRecallCampaignByID(id int64) (*RecallCampaign, error) {
	campaign := &RecallCampaign{}
	if err := DB.First(campaign, id).Error; err != nil {
		return nil, err
	}
	return campaign, nil
}

func UpdateRecallCampaignDraft(campaign *RecallCampaign) error {
	return DB.Model(&RecallCampaign{}).
		Where("id = ? AND status = ?", campaign.Id, RecallCampaignDraft).
		Updates(map[string]any{
			"name":                    campaign.Name,
			"audience_template":       campaign.AudienceTemplate,
			"audience_config":         campaign.AudienceConfig,
			"execution_mode":          campaign.ExecutionMode,
			"scheduled_at":            campaign.ScheduledAt,
			"recurrence_config":       campaign.RecurrenceConfig,
			"next_run_at":             campaign.NextRunAt,
			"coupon_source":           campaign.CouponSource,
			"stripe_coupon_id":        campaign.StripeCouponId,
			"discount_config":         campaign.DiscountConfig,
			"product_scope":           campaign.ProductScope,
			"promotion_valid_seconds": campaign.PromotionValidSeconds,
			"email_sequence_config":   campaign.EmailSequenceConfig,
			"enrollment_limit":        campaign.EnrollmentLimit,
			"worker_concurrency":      campaign.WorkerConcurrency,
		}).Error
}

func TransitionRecallCampaign(id int64, from []string, to string, fields map[string]any) (bool, error) {
	allowedFields := map[string]struct{}{
		"scheduled_at":            {},
		"recurrence_config":       {},
		"next_run_at":             {},
		"stripe_coupon_id":        {},
		"discount_config":         {},
		"product_scope":           {},
		"promotion_valid_seconds": {},
		"email_sequence_config":   {},
		"enrollment_limit":        {},
		"worker_concurrency":      {},
		"activated_at":            {},
		"completed_at":            {},
		"updated_at":              {},
	}
	updates := make(map[string]any, len(fields)+1)
	for key, value := range fields {
		if _, ok := allowedFields[key]; !ok {
			return false, fmt.Errorf("unsupported recall campaign transition field %q", key)
		}
		updates[key] = value
	}
	if len(from) == 0 {
		return false, nil
	}
	updates["status"] = to
	result := DB.Model(&RecallCampaign{}).
		Where("id = ? AND status IN ?", id, from).
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}
