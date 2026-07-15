package model

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
