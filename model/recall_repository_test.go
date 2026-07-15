package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRecallRepositoryTestDB(t *testing.T) (*gorm.DB, *gorm.DB) {
	t.Helper()

	originalDB := DB
	originalLogDB := LOG_DB
	mainDB, err := gorm.Open(sqlite.Open("file:"+t.Name()+"-main?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	logDB, err := gorm.Open(sqlite.Open("file:"+t.Name()+"-log?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	DB = mainDB
	LOG_DB = logDB
	t.Cleanup(func() {
		DB = originalDB
		LOG_DB = originalLogDB
	})

	require.NoError(t, DB.AutoMigrate(
		&User{},
		&RecallCampaign{},
		&RecallRecipient{},
		&RecallMessage{},
		&RecallEvent{},
	))
	return mainDB, logDB
}

func newRecallRepositoryCampaign(name string) RecallCampaign {
	return RecallCampaign{
		Name:                name,
		Status:              RecallCampaignDraft,
		AudienceTemplate:    "inactive_users",
		AudienceConfig:      `{}`,
		ExecutionMode:       "manual",
		CouponSource:        "stripe",
		DiscountConfig:      `{}`,
		ProductScope:        `[]`,
		EmailSequenceConfig: `[]`,
	}
}

func TestRecallRepositoryMigrationCreatesMainDBTablesAndUniqueIndexes(t *testing.T) {
	mainDB, logDB := setupRecallRepositoryTestDB(t)

	for _, table := range []any{&RecallCampaign{}, &RecallRecipient{}, &RecallMessage{}, &RecallEvent{}} {
		require.True(t, mainDB.Migrator().HasTable(table))
		require.False(t, logDB.Migrator().HasTable(table), "recall tables must never be migrated to LOG_DB")
	}

	recipient := RecallRecipient{
		CampaignId:          11,
		UserId:              22,
		EligibilitySnapshot: `{}`,
		EmailSnapshot:       "first@example.com",
		LanguageSnapshot:    "en",
		State:               RecallRecipientQueued,
	}
	require.NoError(t, mainDB.Create(&recipient).Error)
	duplicateRecipient := recipient
	duplicateRecipient.Id = 0
	duplicateRecipient.EmailSnapshot = "duplicate@example.com"
	require.Error(t, mainDB.Create(&duplicateRecipient).Error)

	message := RecallMessage{
		RecipientId:      recipient.Id,
		StageNo:          1,
		TemplateVersion:  1,
		TemplateSnapshot: `{}`,
		State:            RecallMessageScheduled,
	}
	require.NoError(t, mainDB.Create(&message).Error)
	duplicateMessage := message
	duplicateMessage.Id = 0
	require.Error(t, mainDB.Create(&duplicateMessage).Error)

	event := RecallEvent{
		CampaignId:    recipient.CampaignId,
		RecipientId:   recipient.Id,
		EventType:     "campaign_run",
		Source:        "worker",
		SourceEventId: "run-1",
		EventData:     `{}`,
	}
	require.NoError(t, mainDB.Create(&event).Error)
	duplicateEvent := event
	duplicateEvent.Id = 0
	duplicateEvent.EventType = "duplicate"
	require.Error(t, mainDB.Create(&duplicateEvent).Error)
}

func TestRecallRepositoryCampaignCRUDAndConditionalTransition(t *testing.T) {
	setupRecallRepositoryTestDB(t)

	campaign := newRecallRepositoryCampaign("draft campaign")
	require.NoError(t, CreateRecallCampaign(&campaign))
	require.NotZero(t, campaign.Id)

	stored, err := GetRecallCampaignByID(campaign.Id)
	require.NoError(t, err)
	require.Equal(t, campaign.Name, stored.Name)

	campaign.Name = "updated draft"
	require.NoError(t, UpdateRecallCampaignDraft(&campaign))
	stored, err = GetRecallCampaignByID(campaign.Id)
	require.NoError(t, err)
	require.Equal(t, "updated draft", stored.Name)

	transitioned, err := TransitionRecallCampaign(campaign.Id, []string{RecallCampaignRunning}, RecallCampaignPaused, nil)
	require.NoError(t, err)
	require.False(t, transitioned)

	transitioned, err = TransitionRecallCampaign(campaign.Id, []string{RecallCampaignDraft}, RecallCampaignScheduled, map[string]any{
		"scheduled_at": int64(1234),
	})
	require.NoError(t, err)
	require.True(t, transitioned)
	stored, err = GetRecallCampaignByID(campaign.Id)
	require.NoError(t, err)
	require.Equal(t, RecallCampaignScheduled, stored.Status)
	require.Equal(t, int64(1234), stored.ScheduledAt)

	campaign.Name = "must not update after scheduling"
	require.NoError(t, UpdateRecallCampaignDraft(&campaign))
	stored, err = GetRecallCampaignByID(campaign.Id)
	require.NoError(t, err)
	require.Equal(t, "updated draft", stored.Name)
}

func TestRecallRepositoryInsertAndListRecipients(t *testing.T) {
	setupRecallRepositoryTestDB(t)

	campaign := newRecallRepositoryCampaign("recipient campaign")
	require.NoError(t, CreateRecallCampaign(&campaign))
	recipients := []RecallRecipient{
		{CampaignId: campaign.Id, UserId: 101, EligibilitySnapshot: `{}`, EmailSnapshot: "one@example.com", LanguageSnapshot: "en", State: RecallRecipientQueued},
		{CampaignId: campaign.Id, UserId: 102, EligibilitySnapshot: `{}`, EmailSnapshot: "two@example.com", LanguageSnapshot: "zh", State: RecallRecipientQueued},
	}
	runEvent := RecallEvent{CampaignId: campaign.Id, EventType: "campaign_run", Source: "worker", SourceEventId: "run-1", EventData: `{}`}

	inserted, err := InsertRecallRecipientsAndRunEvent(campaign.Id, recipients, runEvent)
	require.NoError(t, err)
	require.Equal(t, 2, inserted)

	inserted, err = InsertRecallRecipientsAndRunEvent(campaign.Id, recipients, RecallEvent{
		CampaignId: campaign.Id, EventType: "campaign_run", Source: "worker", SourceEventId: "run-2", EventData: `{}`,
	})
	require.NoError(t, err)
	require.Zero(t, inserted)

	page, total, err := ListRecallRecipients(campaign.Id, 1, 1)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, page, 1)
	require.Equal(t, 102, page[0].UserId)

	inserted, err = InsertRecallRecipientsAndRunEvent(campaign.Id, []RecallRecipient{
		{CampaignId: campaign.Id, UserId: 103, EligibilitySnapshot: `{}`, EmailSnapshot: "three@example.com", LanguageSnapshot: "en", State: RecallRecipientQueued},
	}, runEvent)
	require.Error(t, err)
	require.Zero(t, inserted)
	_, total, err = ListRecallRecipients(campaign.Id, 0, 10)
	require.NoError(t, err)
	require.Equal(t, int64(2), total, "a duplicate run event must roll back recipient inserts")
}

func TestRecallRepositoryMaskPromotionCode(t *testing.T) {
	require.Equal(t, "ABCD****YZ", MaskPromotionCode("ABCDEFGHIJKLYZ"))
	require.Equal(t, "........", MaskPromotionCode("ABCDEFGH"))
	require.Equal(t, "........", MaskPromotionCode("short"))
}
