package model

import (
	"context"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
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

type RecallClaimRecord struct {
	Recipient      RecallRecipient
	Campaign       RecallCampaign
	ClaimTokenHash string
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

func SetRecallMessageClaimHash(ctx context.Context, messageID int64, leaseOwner string, expectedLeaseUntil int64, claimHash string) (bool, error) {
	updated := false
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var message RecallMessage
		if err := tx.Select("recipient_id", "stage_no").
			Where("id = ? AND state = ? AND lease_owner = ? AND lease_expires_at = ?", messageID, RecallMessageLeased, leaseOwner, expectedLeaseUntil).
			First(&message).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil
			}
			return err
		}
		result := tx.Model(&RecallMessage{}).
			Where("id = ? AND state = ? AND lease_owner = ? AND lease_expires_at = ?", messageID, RecallMessageLeased, leaseOwner, expectedLeaseUntil).
			Update("claim_token_hash", claimHash)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if message.StageNo == 1 {
			if err := tx.Model(&RecallRecipient{}).
				Where("id = ?", message.RecipientId).
				Update("claim_token_hash", claimHash).Error; err != nil {
				return err
			}
		}
		updated = true
		return nil
	})
	return updated, err
}

func FindRecallClaimByHashWithContext(ctx context.Context, claimHash string) (*RecallClaimRecord, bool, error) {
	recipient := RecallRecipient{}
	storedHash := ""
	err := DB.WithContext(ctx).Where("claim_token_hash = ?", claimHash).First(&recipient).Error
	if err == gorm.ErrRecordNotFound {
		message := RecallMessage{}
		if err := DB.WithContext(ctx).Where("claim_token_hash = ?", claimHash).First(&message).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, false, nil
			}
			return nil, false, err
		}
		if message.ClaimTokenHash != nil {
			storedHash = *message.ClaimTokenHash
		}
		if err := DB.WithContext(ctx).First(&recipient, message.RecipientId).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, false, nil
			}
			return nil, false, err
		}
	} else if err != nil {
		return nil, false, err
	} else if recipient.ClaimTokenHash != nil {
		storedHash = *recipient.ClaimTokenHash
	}
	campaign := RecallCampaign{}
	if err := DB.WithContext(ctx).First(&campaign, recipient.CampaignId).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &RecallClaimRecord{Recipient: recipient, Campaign: campaign, ClaimTokenHash: storedHash}, true, nil
}

func SetRecallMarketingOptOutWithContext(ctx context.Context, userID int, now int64) (bool, error) {
	found := false
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user := User{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil
			}
			return err
		}
		found = true
		setting := dto.UserSetting{}
		if user.Setting != "" {
			if err := common.Unmarshal([]byte(user.Setting), &setting); err != nil {
				return err
			}
		}
		setting.RecallMarketingOptOut = true
		settingJSON, err := common.Marshal(setting)
		if err != nil {
			return err
		}
		if err := tx.Model(&User{}).Where("id = ?", userID).Update("setting", string(settingJSON)).Error; err != nil {
			return err
		}
		recipientIDs := tx.Model(&RecallRecipient{}).Select("id").Where("user_id = ?", userID)
		return tx.Model(&RecallMessage{}).
			Where("recipient_id IN (?) AND state IN ?", recipientIDs, []string{RecallMessageScheduled, RecallMessageRetryWait, RecallMessageLeased}).
			Updates(map[string]any{
				"state":            RecallMessageCancelled,
				"lease_owner":      "",
				"lease_expires_at": int64(0),
				"failed_at":        now,
				"last_error_code":  "user_opted_out",
			}).Error
	})
	if err != nil || !found {
		return found, err
	}
	return true, invalidateUserCache(userID)
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

type RecallCandidateQuery struct {
	Template              string
	Now                   int64
	RegistrationBefore    int64
	LastPaymentBefore     int64
	SubscriptionBefore    int64
	MaxQuota              int
	MinRequestCount       int
	MinPaidAmount         float64
	MinSubscriptionAmount float64
	MinSubscriptionCount  int
	PaymentProviders      []string
	Groups                []string
	GroupMode             string
	AfterUserID           int
	Limit                 int
}

type RecallCandidateFact struct {
	User                  User
	HasPayment            bool
	PaidAmount            float64
	LastPaymentAt         int64
	SubscriptionAmount    float64
	SubscriptionCount     int64
	LastSubscriptionEndAt int64
	HasActiveSubscription bool
}

type recallPaymentFactRow struct {
	Id              int
	UserId          int
	Money           float64
	PaymentProvider string
	TradeNo         string
	CreateTime      int64
	CompleteTime    int64
}

func ListRecallCandidateFacts(query RecallCandidateQuery) ([]RecallCandidateFact, error) {
	return ListRecallCandidateFactsWithContext(context.Background(), query)
}

func ListRecallCandidateFactsWithContext(ctx context.Context, query RecallCandidateQuery) ([]RecallCandidateFact, error) {
	facts := make([]RecallCandidateFact, 0)
	if query.Limit <= 0 {
		return facts, nil
	}
	var users []User
	if err := DB.WithContext(ctx).Where("id > ?", query.AfterUserID).
		Order("id ASC").
		Limit(query.Limit).
		Find(&users).Error; err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return facts, nil
	}

	userIDs := make([]int, len(users))
	facts = make([]RecallCandidateFact, len(users))
	factByUserID := make(map[int]*RecallCandidateFact, len(users))
	for i := range users {
		userIDs[i] = users[i].Id
		facts[i] = RecallCandidateFact{User: users[i]}
		factByUserID[users[i].Id] = &facts[i]
	}

	providerFilter := query.Template != "first_purchase" && len(query.PaymentProviders) > 0
	topupQuery := DB.WithContext(ctx).Model(&TopUp{}).
		Select("id", "user_id", "money", "payment_provider", "trade_no", "create_time", "complete_time").
		Where("user_id IN ? AND status = ?", userIDs, common.TopUpStatusSuccess)
	if providerFilter {
		topupQuery = topupQuery.Where("payment_provider IN ?", query.PaymentProviders)
	}
	var topups []recallPaymentFactRow
	if err := topupQuery.Find(&topups).Error; err != nil {
		return nil, err
	}

	subscriptionOrderQuery := DB.WithContext(ctx).Model(&SubscriptionOrder{}).
		Select("id", "user_id", "money", "payment_provider", "trade_no", "create_time", "complete_time").
		Where("user_id IN ? AND status = ?", userIDs, common.TopUpStatusSuccess)
	if providerFilter {
		subscriptionOrderQuery = subscriptionOrderQuery.Where("payment_provider IN ?", query.PaymentProviders)
	}
	var subscriptionOrders []recallPaymentFactRow
	if err := subscriptionOrderQuery.Find(&subscriptionOrders).Error; err != nil {
		return nil, err
	}

	seenPayments := make(map[int]map[string]struct{}, len(users))
	addPayment := func(row recallPaymentFactRow, source string) {
		fact := factByUserID[row.UserId]
		if fact == nil {
			return
		}
		fact.HasPayment = true
		paidAt := row.CompleteTime
		if paidAt == 0 {
			paidAt = row.CreateTime
		}
		if paidAt > fact.LastPaymentAt {
			fact.LastPaymentAt = paidAt
		}
		key := row.TradeNo
		if key == "" {
			key = fmt.Sprintf("%s:%d", source, row.Id)
		}
		if seenPayments[row.UserId] == nil {
			seenPayments[row.UserId] = make(map[string]struct{})
		}
		if _, exists := seenPayments[row.UserId][key]; exists {
			return
		}
		seenPayments[row.UserId][key] = struct{}{}
		fact.PaidAmount += row.Money
	}
	for _, topup := range topups {
		addPayment(topup, "topup")
	}
	for _, order := range subscriptionOrders {
		addPayment(order, "subscription")
		if fact := factByUserID[order.UserId]; fact != nil {
			fact.SubscriptionAmount += order.Money
		}
	}

	var subscriptions []UserSubscription
	if err := DB.WithContext(ctx).Where("user_id IN ?", userIDs).Find(&subscriptions).Error; err != nil {
		return nil, err
	}
	for _, subscription := range subscriptions {
		fact := factByUserID[subscription.UserId]
		if fact == nil {
			continue
		}
		fact.SubscriptionCount++
		if subscription.EndTime > fact.LastSubscriptionEndAt {
			fact.LastSubscriptionEndAt = subscription.EndTime
		}
		if subscription.Status == "active" && subscription.EndTime > query.Now {
			fact.HasActiveSubscription = true
		}
	}
	return facts, nil
}

func HasRecallPaymentAfter(userID int, after int64) (bool, error) {
	return HasRecallPaymentAfterWithContext(context.Background(), userID, after)
}

func HasRecallPaymentAfterWithContext(ctx context.Context, userID int, after int64) (bool, error) {
	var count int64
	if err := DB.WithContext(ctx).Model(&TopUp{}).
		Where("user_id = ? AND status = ? AND (complete_time > ? OR (complete_time = 0 AND create_time > ?))", userID, common.TopUpStatusSuccess, after, after).
		Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	if err := DB.WithContext(ctx).Model(&SubscriptionOrder{}).
		Where("user_id = ? AND status = ? AND (complete_time > ? OR (complete_time = 0 AND create_time > ?))", userID, common.TopUpStatusSuccess, after, after).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
