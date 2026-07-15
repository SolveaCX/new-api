package model

import "gorm.io/gorm/clause"

const (
	RecallMessageScheduled = "scheduled"
	RecallMessageLeased    = "leased"
	RecallMessageAccepted  = "accepted"
	RecallMessageRetryWait = "retry_wait"
	RecallMessageUncertain = "uncertain"
	RecallMessageFailed    = "failed"
	RecallMessageCancelled = "cancelled"
)

type RecallMessage struct {
	Id                int64   `json:"id" gorm:"primaryKey"`
	RecipientId       int64   `json:"recipient_id" gorm:"uniqueIndex:idx_recall_recipient_stage,priority:1;index"`
	StageNo           int     `json:"stage_no" gorm:"uniqueIndex:idx_recall_recipient_stage,priority:2"`
	TemplateVersion   int     `json:"template_version"`
	TemplateSnapshot  string  `json:"-" gorm:"type:text;not null"`
	ScheduledAt       int64   `json:"scheduled_at" gorm:"index"`
	State             string  `json:"state" gorm:"type:varchar(24);not null;index"`
	AttemptCount      int     `json:"attempt_count"`
	NextAttemptAt     int64   `json:"next_attempt_at" gorm:"index"`
	LeaseOwner        string  `json:"-" gorm:"type:varchar(96);index"`
	LeaseExpiresAt    int64   `json:"-" gorm:"index"`
	ProviderMessageId string  `json:"provider_message_id" gorm:"type:varchar(255)"`
	ClaimTokenHash    *string `json:"-" gorm:"type:char(64);uniqueIndex"`
	AcceptedAt        int64   `json:"accepted_at"`
	FailedAt          int64   `json:"failed_at"`
	LastErrorCode     string  `json:"last_error_code" gorm:"type:varchar(64)"`
	LastErrorMessage  string  `json:"last_error_message" gorm:"type:varchar(512)"`
	CreatedAt         int64   `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         int64   `json:"updated_at" gorm:"autoUpdateTime"`
}

func ListDueRecallMessageIDs(now int64, limit int) ([]int64, error) {
	ids := make([]int64, 0)
	if limit <= 0 {
		return ids, nil
	}
	err := DB.Model(&RecallMessage{}).
		Where(
			"(state = ? AND scheduled_at <= ?) OR (state = ? AND next_attempt_at <= ?) OR (state = ? AND (lease_expires_at = 0 OR lease_expires_at < ?))",
			RecallMessageScheduled,
			now,
			RecallMessageRetryWait,
			now,
			RecallMessageLeased,
			now,
		).
		Order("id ASC").
		Limit(limit).
		Pluck("id", &ids).Error
	return ids, err
}

func LeaseRecallMessage(id int64, owner string, now int64, leaseUntil int64) (bool, error) {
	result := DB.Model(&RecallMessage{}).
		Where("id = ? AND state IN ? AND (lease_expires_at = 0 OR lease_expires_at < ?)", id, []string{
			RecallMessageScheduled,
			RecallMessageRetryWait,
			RecallMessageLeased,
		}, now).
		Updates(map[string]any{
			"state":            RecallMessageLeased,
			"lease_owner":      owner,
			"lease_expires_at": leaseUntil,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func CompleteRecallMessageLease(id int64, owner string, from string, to string, fields map[string]any) (bool, error) {
	updates := make(map[string]any, len(fields)+3)
	for key, value := range fields {
		updates[key] = value
	}
	updates["state"] = to
	updates["lease_owner"] = ""
	updates["lease_expires_at"] = int64(0)
	result := DB.Model(&RecallMessage{}).
		Where("id = ? AND lease_owner = ? AND state = ?", id, owner, from).
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func ScheduleNextRecallStages(recipientID int64, messages []RecallMessage) error {
	if len(messages) == 0 {
		return nil
	}
	for i := range messages {
		messages[i].RecipientId = recipientID
	}
	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "recipient_id"}, {Name: "stage_no"}},
		DoNothing: true,
	}).Create(&messages).Error
}

func CancelPendingRecallMessages(recipientID int64, reasonCode string, now int64) (int64, error) {
	result := DB.Model(&RecallMessage{}).
		Where("recipient_id = ? AND state IN ?", recipientID, []string{
			RecallMessageScheduled,
			RecallMessageRetryWait,
		}).
		Updates(map[string]any{
			"state":           RecallMessageCancelled,
			"last_error_code": reasonCode,
			"failed_at":       now,
		})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}
