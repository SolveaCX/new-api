package model

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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

var errRecallRunNotOwned = errors.New("recall campaign run not owned")

const recallRunBatchSize = 200

// CommitRecallCampaignRun makes the campaign state change, idempotency event,
// recipient snapshot, and initial message snapshot one database transaction.
// expectedNextRunAt is nil for manual runs and is a fencing value for scheduled
// runs.
func CommitRecallCampaignRun(
	ctx context.Context,
	campaignID int64,
	from []string,
	to string,
	expectedNextRunAt *int64,
	expectedConfigRevision int64,
	fields map[string]any,
	recipients []RecallRecipient,
	messages []RecallMessage,
	runEvent RecallEvent,
) (bool, int, error) {
	if len(from) == 0 {
		return false, 0, nil
	}
	if len(messages) != 0 && len(messages) != len(recipients) {
		return false, 0, fmt.Errorf("cannot align %d recall messages with %d recipients", len(messages), len(recipients))
	}
	updates, err := recallCampaignTransitionUpdates(to, fields)
	if err != nil {
		return false, 0, err
	}
	for i := range recipients {
		recipients[i].CampaignId = campaignID
	}
	runEvent.CampaignId = campaignID
	owned := false
	inserted := int64(0)
	err = DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		campaignQuery := tx.Model(&RecallCampaign{}).
			Where("id = ? AND status IN ? AND config_revision = ?", campaignID, from, expectedConfigRevision)
		if expectedNextRunAt != nil {
			campaignQuery = campaignQuery.Where("next_run_at = ?", *expectedNextRunAt)
		}
		campaignResult := campaignQuery.Updates(updates)
		if campaignResult.Error != nil {
			return campaignResult.Error
		}
		if campaignResult.RowsAffected == 0 {
			return nil
		}
		owned = true

		eventResult := insertRecallRunEvent(tx, &runEvent)
		if eventResult.Error != nil {
			return eventResult.Error
		}
		if eventResult.RowsAffected == 0 {
			return errRecallRunNotOwned
		}
		if len(recipients) > 0 {
			result := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "campaign_id"}, {Name: "user_id"}},
				DoNothing: true,
			}).CreateInBatches(&recipients, recallRunBatchSize)
			if result.Error != nil {
				return result.Error
			}
			inserted = result.RowsAffected
		}
		if len(messages) == 0 {
			return nil
		}

		userIDs := make([]int, len(recipients))
		for i := range recipients {
			userIDs[i] = recipients[i].UserId
		}
		storedRecipients := make([]RecallRecipient, 0, len(userIDs))
		for start := 0; start < len(userIDs); start += recallRunBatchSize {
			end := start + recallRunBatchSize
			if end > len(userIDs) {
				end = len(userIDs)
			}
			var batch []RecallRecipient
			if err := tx.Select("id", "user_id").
				Where("campaign_id = ? AND user_id IN ?", campaignID, userIDs[start:end]).
				Find(&batch).Error; err != nil {
				return err
			}
			storedRecipients = append(storedRecipients, batch...)
		}
		recipientIDsByUserID := make(map[int]int64, len(storedRecipients))
		for _, recipient := range storedRecipients {
			recipientIDsByUserID[recipient.UserId] = recipient.Id
		}
		for i := range messages {
			recipientID, ok := recipientIDsByUserID[recipients[i].UserId]
			if !ok {
				return fmt.Errorf("recall recipient for campaign %d user %d was not persisted", campaignID, recipients[i].UserId)
			}
			messages[i].RecipientId = recipientID
		}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "recipient_id"}, {Name: "stage_no"}},
			DoNothing: true,
		}).CreateInBatches(&messages, recallRunBatchSize).Error
	})
	if errors.Is(err, errRecallRunNotOwned) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	return owned, int(inserted), nil
}
