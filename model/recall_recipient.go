package model

import (
	"fmt"

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

func ListDueRecallRecipientIDs(now int64, limit int) ([]int64, error) {
	ids := make([]int64, 0)
	if limit <= 0 {
		return ids, nil
	}
	err := DB.Model(&RecallRecipient{}).
		Where("state IN ? AND (lease_expires_at = 0 OR lease_expires_at < ?)", []string{
			RecallRecipientQueued,
			RecallRecipientCustomerReady,
			RecallRecipientCodeReady,
		}, now).
		Order("id ASC").
		Limit(limit).
		Pluck("id", &ids).Error
	return ids, err
}

func LeaseRecallRecipient(id int64, owner string, now int64, leaseUntil int64) (bool, error) {
	result := DB.Model(&RecallRecipient{}).
		Where("id = ? AND state IN ? AND (lease_expires_at = 0 OR lease_expires_at < ?)", id, []string{
			RecallRecipientQueued,
			RecallRecipientCustomerReady,
			RecallRecipientCodeReady,
		}, now).
		Updates(map[string]any{
			"lease_owner":      owner,
			"lease_expires_at": leaseUntil,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func ReleaseRecallRecipientLease(id int64, owner string, expectedLeaseUntil int64) error {
	return DB.Model(&RecallRecipient{}).
		Where("id = ? AND lease_owner = ? AND lease_expires_at = ?", id, owner, expectedLeaseUntil).
		Updates(map[string]any{
			"lease_owner":      "",
			"lease_expires_at": int64(0),
		}).Error
}

func insertRecallRunEvent(tx *gorm.DB, runEvent *RecallEvent) *gorm.DB {
	if tx.Dialector.Name() == "mysql" {
		// A duplicate INSERT IGNORE reports zero affected rows; unlike UPDATE, this ownership signal is not changed by clientFoundRows.
		return tx.Clauses(clause.Insert{Modifier: "IGNORE"}).Create(runEvent)
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(runEvent)
}

func InsertRecallRecipientsAndRunEvent(campaignID int64, recipients []RecallRecipient, messages []RecallMessage, runEvent RecallEvent) (int, error) {
	alignedMessages := make([]bool, len(messages))
	hasAlignedMessages := false
	for i := range messages {
		if messages[i].RecipientId == 0 {
			if len(messages) != len(recipients) {
				return 0, fmt.Errorf("cannot align %d recall messages with %d recipients", len(messages), len(recipients))
			}
			alignedMessages[i] = true
			hasAlignedMessages = true
		}
	}
	for i := range recipients {
		recipients[i].CampaignId = campaignID
	}
	runEvent.CampaignId = campaignID

	inserted := int64(0)
	ownedRun := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		eventResult := insertRecallRunEvent(tx, &runEvent)
		if eventResult.Error != nil {
			return eventResult.Error
		}
		if eventResult.RowsAffected == 0 {
			return nil
		}
		ownedRun = true

		if len(recipients) > 0 {
			result := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "campaign_id"}, {Name: "user_id"}},
				DoNothing: true,
			}).Create(&recipients)
			if result.Error != nil {
				return result.Error
			}
			inserted = result.RowsAffected
		}

		if hasAlignedMessages {
			userIDs := make([]int, len(recipients))
			for i := range recipients {
				userIDs[i] = recipients[i].UserId
			}
			var storedRecipients []RecallRecipient
			if err := tx.Select("id", "user_id").
				Where("campaign_id = ? AND user_id IN ?", campaignID, userIDs).
				Find(&storedRecipients).Error; err != nil {
				return err
			}
			recipientIDsByUserID := make(map[int]int64, len(storedRecipients))
			for _, recipient := range storedRecipients {
				recipientIDsByUserID[recipient.UserId] = recipient.Id
			}
			for i, aligned := range alignedMessages {
				if !aligned {
					continue
				}
				recipientID, ok := recipientIDsByUserID[recipients[i].UserId]
				if !ok {
					return fmt.Errorf("recall recipient for campaign %d user %d was not persisted", campaignID, recipients[i].UserId)
				}
				messages[i].RecipientId = recipientID
			}
		}
		if len(messages) == 0 {
			return nil
		}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "recipient_id"}, {Name: "stage_no"}},
			DoNothing: true,
		}).Create(&messages).Error
	})
	if err != nil {
		return 0, err
	}
	if !ownedRun {
		return 0, nil
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
