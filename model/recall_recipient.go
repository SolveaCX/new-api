package model

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	RecallRecipientQueued        = "queued"
	RecallRecipientCustomerReady = "customer_ready"
	RecallRecipientCodeReady     = "code_ready"
	RecallRecipientContacting    = "contacting"
	RecallRecipientConverted     = "converted"
	RecallRecipientSuppressed    = "suppressed"
	RecallRecipientIneligible    = "ineligible"
	RecallRecipientExpired       = "expired"
	RecallRecipientFailed        = "failed"

	RecallConversionDirect   = "direct"
	RecallConversionAssisted = "assisted"
	RecallConversionNoCoupon = "no_coupon"
)

type RecallRecipient struct {
	Id                    int64   `json:"id" gorm:"primaryKey"`
	CampaignId            int64   `json:"campaign_id" gorm:"uniqueIndex:idx_recall_campaign_user,priority:1;index"`
	UserId                int     `json:"user_id" gorm:"uniqueIndex:idx_recall_campaign_user,priority:2;index"`
	EligibilitySnapshot   string  `json:"eligibility_snapshot" gorm:"type:text;not null"`
	EmailSnapshot         string  `json:"email_snapshot" gorm:"type:varchar(254);not null"`
	LanguageSnapshot      string  `json:"language_snapshot" gorm:"type:varchar(16);not null"`
	State                 string  `json:"state" gorm:"type:varchar(24);not null;index"`
	LeaseOwner            string  `json:"-" gorm:"type:varchar(96);index"`
	LeaseExpiresAt        int64   `json:"-" gorm:"index"`
	StripeCustomerId      string  `json:"stripe_customer_id" gorm:"type:varchar(128)"`
	StripePromotionCodeId *string `json:"-" gorm:"type:varchar(128);uniqueIndex"`
	PromotionCode         string  `json:"-" gorm:"type:varchar(64)"`
	PromotionExpiresAt    int64   `json:"promotion_expires_at" gorm:"index"`
	ClaimTokenHash        *string `json:"-" gorm:"type:char(64);uniqueIndex"`
	FirstSentAt           int64   `json:"first_sent_at"`
	LastSentAt            int64   `json:"last_sent_at"`
	ClickedAt             int64   `json:"clicked_at"`
	ConvertedAt           int64   `json:"converted_at"`
	ConversionKind        string  `json:"conversion_kind" gorm:"type:varchar(16)"`
	ConversionTradeNo     string  `json:"conversion_trade_no" gorm:"type:varchar(128);index"`
	ConversionCurrency    string  `json:"conversion_currency" gorm:"type:varchar(8)"`
	ConversionAmount      int64   `json:"conversion_amount"`
	DiscountAmount        int64   `json:"discount_amount"`
	LastErrorCode         string  `json:"last_error_code" gorm:"type:varchar(64)"`
	LastErrorMessage      string  `json:"last_error_message" gorm:"type:varchar(512)"`
	CreatedAt             int64   `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt             int64   `json:"updated_at" gorm:"autoUpdateTime"`
}

func InsertRecallRecipientsAndRunEvent(campaignID int64, recipients []RecallRecipient, runEvent RecallEvent) (int, error) {
	inserted := int64(0)
	err := DB.Transaction(func(tx *gorm.DB) error {
		if len(recipients) > 0 {
			for i := range recipients {
				recipients[i].CampaignId = campaignID
			}
			result := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "campaign_id"}, {Name: "user_id"}},
				DoNothing: true,
			}).Create(&recipients)
			if result.Error != nil {
				return result.Error
			}
			inserted = result.RowsAffected
		}
		runEvent.CampaignId = campaignID
		return tx.Create(&runEvent).Error
	})
	if err != nil {
		return 0, err
	}
	return int(inserted), nil
}

func ListRecallRecipients(campaignID int64, offset int, limit int) ([]RecallRecipient, int64, error) {
	recipients := make([]RecallRecipient, 0)
	var total int64
	query := DB.Model(&RecallRecipient{}).Where("campaign_id = ?", campaignID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id ASC").Offset(offset).Limit(limit).Find(&recipients).Error; err != nil {
		return nil, 0, err
	}
	return recipients, total, nil
}

func MaskPromotionCode(code string) string {
	if len(code) <= 8 {
		return "........"
	}
	return code[:4] + "****" + code[len(code)-2:]
}
