package model

type RecallEvent struct {
	Id            int64  `json:"id" gorm:"primaryKey"`
	CampaignId    int64  `json:"campaign_id" gorm:"index"`
	RecipientId   int64  `json:"recipient_id" gorm:"index"`
	EventType     string `json:"event_type" gorm:"type:varchar(48);not null;index"`
	Source        string `json:"source" gorm:"type:varchar(32);uniqueIndex:idx_recall_source_event,priority:1"`
	SourceEventId string `json:"source_event_id" gorm:"type:varchar(160);uniqueIndex:idx_recall_source_event,priority:2"`
	EventData     string `json:"event_data" gorm:"type:text"`
	CreatedAt     int64  `json:"created_at" gorm:"autoCreateTime;index"`
}
