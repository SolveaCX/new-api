package model

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
		Omit("id", "status", "created_at", "activated_at", "completed_at").
		Updates(campaign).Error
}

func TransitionRecallCampaign(id int64, from []string, to string, fields map[string]any) (bool, error) {
	if len(from) == 0 {
		return false, nil
	}
	updates := make(map[string]any, len(fields)+1)
	for key, value := range fields {
		updates[key] = value
	}
	updates["status"] = to
	result := DB.Model(&RecallCampaign{}).
		Where("id = ? AND status IN ?", id, from).
		Updates(updates)
	return result.RowsAffected == 1, result.Error
}
