package model

import (
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func setupRecallRepositoryTestDB(t *testing.T) (*gorm.DB, *gorm.DB) {
	t.Helper()

	originalDB := DB
	originalLogDB := LOG_DB
	mainDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	mainSQLDB, err := mainDB.DB()
	require.NoError(t, err)
	mainSQLDB.SetMaxOpenConns(1)
	logDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	logSQLDB, err := logDB.DB()
	require.NoError(t, err)
	logSQLDB.SetMaxOpenConns(1)
	DB = mainDB
	LOG_DB = logDB
	t.Cleanup(func() {
		_ = mainSQLDB.Close()
		_ = logSQLDB.Close()
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

func setupRecallRepositoryFileDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := DB
	originalLogDB := LOG_DB
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/recall.db?_pragma=busy_timeout(5000)"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)
	DB = db
	LOG_DB = db
	t.Cleanup(func() {
		_ = sqlDB.Close()
		DB = originalDB
		LOG_DB = originalLogDB
	})

	require.NoError(t, DB.AutoMigrate(
		&RecallCampaign{},
		&RecallRecipient{},
		&RecallMessage{},
		&RecallEvent{},
	))
	return db
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
		"next_run_at":  int64(2345),
	})
	require.NoError(t, err)
	require.True(t, transitioned)
	stored, err = GetRecallCampaignByID(campaign.Id)
	require.NoError(t, err)
	require.Equal(t, RecallCampaignScheduled, stored.Status)
	require.Equal(t, int64(1234), stored.ScheduledAt)
	require.Equal(t, int64(2345), stored.NextRunAt)

	campaign.Name = "must not update after scheduling"
	require.NoError(t, UpdateRecallCampaignDraft(&campaign))
	stored, err = GetRecallCampaignByID(campaign.Id)
	require.NoError(t, err)
	require.Equal(t, "updated draft", stored.Name)
}

func TestRecallRepositoryDraftUpdatePersistsZeroValuesAndPreservesImmutableFields(t *testing.T) {
	setupRecallRepositoryTestDB(t)

	campaign := newRecallRepositoryCampaign("draft with values")
	campaign.AudienceTemplate = "previous_template"
	campaign.AudienceConfig = `{"days":90}`
	campaign.ExecutionMode = "scheduled"
	campaign.ScheduledAt = 101
	campaign.RecurrenceConfig = `{"interval":"weekly"}`
	campaign.NextRunAt = 202
	campaign.CouponSource = "stripe"
	campaign.StripeCouponId = "coupon_old"
	campaign.DiscountConfig = `{"percent":25}`
	campaign.ProductScope = `["pro"]`
	campaign.PromotionValidSeconds = 303
	campaign.EmailSequenceConfig = `[{"stage":1}]`
	campaign.EnrollmentLimit = 404
	campaign.WorkerConcurrency = 5
	campaign.CreatedBy = 606
	campaign.CreatedAt = 707
	campaign.ActivatedAt = 808
	campaign.CompletedAt = 909
	require.NoError(t, CreateRecallCampaign(&campaign))

	update := RecallCampaign{
		Id:          campaign.Id,
		Status:      RecallCampaignRunning,
		CreatedBy:   999,
		CreatedAt:   999,
		ActivatedAt: 999,
		CompletedAt: 999,
	}
	require.NoError(t, UpdateRecallCampaignDraft(&update))

	stored, err := GetRecallCampaignByID(campaign.Id)
	require.NoError(t, err)
	require.Equal(t, campaign.Id, stored.Id)
	require.Equal(t, RecallCampaignDraft, stored.Status)
	require.Equal(t, 606, stored.CreatedBy)
	require.Equal(t, int64(707), stored.CreatedAt)
	require.Equal(t, int64(808), stored.ActivatedAt)
	require.Equal(t, int64(909), stored.CompletedAt)
	require.Empty(t, stored.Name)
	require.Empty(t, stored.AudienceTemplate)
	require.Empty(t, stored.AudienceConfig)
	require.Empty(t, stored.ExecutionMode)
	require.Zero(t, stored.ScheduledAt)
	require.Empty(t, stored.RecurrenceConfig)
	require.Equal(t, int64(202), stored.NextRunAt)
	require.Empty(t, stored.CouponSource)
	require.Empty(t, stored.StripeCouponId)
	require.Empty(t, stored.DiscountConfig)
	require.Empty(t, stored.ProductScope)
	require.Zero(t, stored.PromotionValidSeconds)
	require.Empty(t, stored.EmailSequenceConfig)
	require.Zero(t, stored.EnrollmentLimit)
	require.Zero(t, stored.WorkerConcurrency)
}

func TestRecallRepositoryTransitionRejectsUnsafeMetadata(t *testing.T) {
	for _, field := range []string{"id", "created_at", "created_by", "status", "updated_at"} {
		t.Run(field, func(t *testing.T) {
			setupRecallRepositoryTestDB(t)
			campaign := newRecallRepositoryCampaign("protected transition")
			campaign.CreatedBy = 41
			campaign.CreatedAt = 42
			require.NoError(t, CreateRecallCampaign(&campaign))

			transitioned, err := TransitionRecallCampaign(
				campaign.Id,
				[]string{RecallCampaignDraft},
				RecallCampaignScheduled,
				map[string]any{field: 999},
			)
			require.Error(t, err)
			require.False(t, transitioned)

			stored, err := GetRecallCampaignByID(campaign.Id)
			require.NoError(t, err)
			require.Equal(t, RecallCampaignDraft, stored.Status)
			require.Equal(t, 41, stored.CreatedBy)
			require.Equal(t, int64(42), stored.CreatedAt)
		})
	}
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

	inserted, err := InsertRecallRecipientsAndRunEvent(campaign.Id, recipients, nil, runEvent)
	require.NoError(t, err)
	require.Equal(t, 2, inserted)

	inserted, err = InsertRecallRecipientsAndRunEvent(campaign.Id, recipients, nil, RecallEvent{
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
	}, nil, runEvent)
	require.NoError(t, err)
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

func TestRecallLeaseMessageHasOneWinnerAndRecoversAfterExpiry(t *testing.T) {
	setupRecallRepositoryTestDB(t)

	message := RecallMessage{
		RecipientId:      101,
		StageNo:          1,
		TemplateSnapshot: `{}`,
		ScheduledAt:      1_721_000_000,
		State:            RecallMessageScheduled,
	}
	require.NoError(t, DB.Create(&message).Error)

	now := int64(1_721_000_000)
	won, err := LeaseRecallMessage(message.Id, "node-a", now, now+60)
	require.NoError(t, err)
	require.True(t, won)

	won, err = LeaseRecallMessage(message.Id, "node-b", now, now+60)
	require.NoError(t, err)
	require.False(t, won)

	won, err = LeaseRecallMessage(message.Id, "node-b", now+61, now+121)
	require.NoError(t, err)
	require.True(t, won)

	var stored RecallMessage
	require.NoError(t, DB.First(&stored, message.Id).Error)
	require.Equal(t, RecallMessageLeased, stored.State)
	require.Equal(t, "node-b", stored.LeaseOwner)
	require.Equal(t, now+121, stored.LeaseExpiresAt)
}

func TestRecallLeaseRecipientHasExactlyOneConcurrentWinner(t *testing.T) {
	setupRecallRepositoryFileDB(t)

	recipient := RecallRecipient{
		CampaignId:          1,
		UserId:              201,
		EligibilitySnapshot: `{}`,
		EmailSnapshot:       "lease@example.com",
		LanguageSnapshot:    "en",
		State:               RecallRecipientQueued,
	}
	require.NoError(t, DB.Create(&recipient).Error)

	type leaseResult struct {
		owner string
		won   bool
		err   error
	}
	start := make(chan struct{})
	results := make(chan leaseResult, 2)
	var workers sync.WaitGroup
	for _, owner := range []string{"node-a", "node-b"} {
		workers.Add(1)
		go func(owner string) {
			defer workers.Done()
			<-start
			won, err := LeaseRecallRecipient(recipient.Id, owner, 1_721_000_000, 1_721_000_060)
			results <- leaseResult{owner: owner, won: won, err: err}
		}(owner)
	}
	close(start)
	workers.Wait()
	close(results)

	winners := make([]string, 0, 1)
	for result := range results {
		require.NoError(t, result.err)
		if result.won {
			winners = append(winners, result.owner)
		}
	}
	require.Len(t, winners, 1)

	var stored RecallRecipient
	require.NoError(t, DB.First(&stored, recipient.Id).Error)
	require.Equal(t, RecallRecipientQueued, stored.State, "a lease must not become durable workflow state")
	require.Equal(t, winners[0], stored.LeaseOwner)

	require.NoError(t, ReleaseRecallRecipientLease(recipient.Id, "not-the-owner", 1_721_000_060))
	require.NoError(t, DB.First(&stored, recipient.Id).Error)
	require.Equal(t, winners[0], stored.LeaseOwner)
	require.NoError(t, ReleaseRecallRecipientLease(recipient.Id, winners[0], 1_721_000_060))
	require.NoError(t, DB.First(&stored, recipient.Id).Error)
	require.Empty(t, stored.LeaseOwner)
	require.Zero(t, stored.LeaseExpiresAt)
}

func TestRecallLeaseRecipientFencesSameOwnerReacquisition(t *testing.T) {
	setupRecallRepositoryTestDB(t)

	recipient := RecallRecipient{
		CampaignId:          1,
		UserId:              251,
		EligibilitySnapshot: `{}`,
		EmailSnapshot:       "fence@example.com",
		LanguageSnapshot:    "en",
		State:               RecallRecipientQueued,
	}
	require.NoError(t, DB.Create(&recipient).Error)
	requireLease, err := LeaseRecallRecipient(recipient.Id, "node-a", 100, 160)
	require.NoError(t, err)
	require.True(t, requireLease)
	requireLease, err = LeaseRecallRecipient(recipient.Id, "node-a", 161, 221)
	require.NoError(t, err)
	require.True(t, requireLease)

	require.NoError(t, ReleaseRecallRecipientLease(recipient.Id, "node-a", 160))
	var stored RecallRecipient
	require.NoError(t, DB.First(&stored, recipient.Id).Error)
	require.Equal(t, "node-a", stored.LeaseOwner)
	require.Equal(t, int64(221), stored.LeaseExpiresAt)

	require.NoError(t, ReleaseRecallRecipientLease(recipient.Id, "node-a", 221))
	require.NoError(t, DB.First(&stored, recipient.Id).Error)
	require.Empty(t, stored.LeaseOwner)
	require.Zero(t, stored.LeaseExpiresAt)
}

func TestRecallLeaseListsOnlyDueProvisioningAndMessageWork(t *testing.T) {
	setupRecallRepositoryTestDB(t)

	now := int64(1_721_000_000)
	recipients := []RecallRecipient{
		{CampaignId: 1, UserId: 301, EligibilitySnapshot: `{}`, EmailSnapshot: "queued@example.com", LanguageSnapshot: "en", State: RecallRecipientQueued},
		{CampaignId: 1, UserId: 302, EligibilitySnapshot: `{}`, EmailSnapshot: "customer@example.com", LanguageSnapshot: "en", State: RecallRecipientCustomerReady, LeaseOwner: "old", LeaseExpiresAt: now - 1},
		{CampaignId: 1, UserId: 303, EligibilitySnapshot: `{}`, EmailSnapshot: "code@example.com", LanguageSnapshot: "en", State: RecallRecipientCodeReady, LeaseOwner: "live", LeaseExpiresAt: now + 1},
		{CampaignId: 1, UserId: 304, EligibilitySnapshot: `{}`, EmailSnapshot: "contacting@example.com", LanguageSnapshot: "en", State: RecallRecipientContacting},
		{CampaignId: 1, UserId: 305, EligibilitySnapshot: `{}`, EmailSnapshot: "terminal@example.com", LanguageSnapshot: "en", State: RecallRecipientConverted},
	}
	require.NoError(t, DB.Create(&recipients).Error)

	recipientIDs, err := ListDueRecallRecipientIDs(now, 1)
	require.NoError(t, err)
	require.Equal(t, []int64{recipients[0].Id}, recipientIDs)
	recipientIDs, err = ListDueRecallRecipientIDs(now, 10)
	require.NoError(t, err)
	require.Equal(t, []int64{recipients[0].Id, recipients[1].Id}, recipientIDs)

	messages := []RecallMessage{
		{RecipientId: recipients[0].Id, StageNo: 1, TemplateSnapshot: `{}`, ScheduledAt: now, State: RecallMessageScheduled},
		{RecipientId: recipients[0].Id, StageNo: 2, TemplateSnapshot: `{}`, ScheduledAt: now + 1, State: RecallMessageScheduled},
		{RecipientId: recipients[1].Id, StageNo: 1, TemplateSnapshot: `{}`, NextAttemptAt: now, State: RecallMessageRetryWait},
		{RecipientId: recipients[1].Id, StageNo: 2, TemplateSnapshot: `{}`, NextAttemptAt: now + 1, State: RecallMessageRetryWait},
		{RecipientId: recipients[2].Id, StageNo: 1, TemplateSnapshot: `{}`, State: RecallMessageLeased, LeaseOwner: "old", LeaseExpiresAt: now - 1},
		{RecipientId: recipients[2].Id, StageNo: 2, TemplateSnapshot: `{}`, State: RecallMessageLeased, LeaseOwner: "live", LeaseExpiresAt: now + 1},
		{RecipientId: recipients[3].Id, StageNo: 1, TemplateSnapshot: `{}`, ScheduledAt: now, State: RecallMessageAccepted},
		{RecipientId: recipients[4].Id, StageNo: 1, TemplateSnapshot: `{}`, ScheduledAt: now, State: RecallMessageCancelled},
	}
	require.NoError(t, DB.Create(&messages).Error)

	messageIDs, err := ListDueRecallMessageIDs(now, 2)
	require.NoError(t, err)
	require.Equal(t, []int64{messages[0].Id, messages[2].Id}, messageIDs)
	messageIDs, err = ListDueRecallMessageIDs(now, 10)
	require.NoError(t, err)
	require.Equal(t, []int64{messages[0].Id, messages[2].Id, messages[4].Id}, messageIDs)
}

func TestRecallLeaseMessageRejectsStaleListingAfterFutureRetryTransition(t *testing.T) {
	setupRecallRepositoryTestDB(t)

	now := int64(1_721_000_000)
	message := RecallMessage{
		RecipientId:      351,
		StageNo:          1,
		TemplateSnapshot: `{}`,
		ScheduledAt:      now,
		State:            RecallMessageScheduled,
	}
	require.NoError(t, DB.Create(&message).Error)
	dueIDs, err := ListDueRecallMessageIDs(now, 10)
	require.NoError(t, err)
	require.Equal(t, []int64{message.Id}, dueIDs)

	require.NoError(t, DB.Model(&RecallMessage{}).
		Where("id = ?", message.Id).
		Updates(map[string]any{
			"state":           RecallMessageRetryWait,
			"next_attempt_at": now + 60,
		}).Error)
	won, err := LeaseRecallMessage(message.Id, "stale-worker", now, now+30)
	require.NoError(t, err)
	require.False(t, won)

	var stored RecallMessage
	require.NoError(t, DB.First(&stored, message.Id).Error)
	require.Equal(t, RecallMessageRetryWait, stored.State)
	require.Empty(t, stored.LeaseOwner)
	require.Zero(t, stored.LeaseExpiresAt)
}

func TestRecallRunIdempotencyInsertsRecipientsMessagesAndEventOnce(t *testing.T) {
	setupRecallRepositoryTestDB(t)

	campaign := newRecallRepositoryCampaign("idempotent run")
	require.NoError(t, CreateRecallCampaign(&campaign))
	recipients := []RecallRecipient{
		{UserId: 401, EligibilitySnapshot: `{}`, EmailSnapshot: "one@example.com", LanguageSnapshot: "en", State: RecallRecipientQueued},
		{UserId: 402, EligibilitySnapshot: `{}`, EmailSnapshot: "two@example.com", LanguageSnapshot: "en", State: RecallRecipientQueued},
	}
	messages := []RecallMessage{
		{StageNo: 1, TemplateSnapshot: `{"subject":"one"}`, ScheduledAt: 1_721_000_000, State: RecallMessageScheduled},
		{StageNo: 1, TemplateSnapshot: `{"subject":"two"}`, ScheduledAt: 1_721_000_000, State: RecallMessageScheduled},
	}
	runEvent := RecallEvent{
		EventType:     "campaign_run",
		Source:        "scheduler",
		SourceEventId: "recurring:7:1721000000",
		EventData:     `{}`,
	}

	inserted, err := InsertRecallRecipientsAndRunEvent(campaign.Id, recipients, messages, runEvent)
	require.NoError(t, err)
	require.Equal(t, 2, inserted)
	inserted, err = InsertRecallRecipientsAndRunEvent(campaign.Id, recipients, messages, runEvent)
	require.NoError(t, err)
	require.Zero(t, inserted)

	var storedRecipients []RecallRecipient
	require.NoError(t, DB.Order("id ASC").Find(&storedRecipients).Error)
	require.Len(t, storedRecipients, 2)
	require.Equal(t, campaign.Id, storedRecipients[0].CampaignId)
	require.Equal(t, campaign.Id, storedRecipients[1].CampaignId)
	var storedMessages []RecallMessage
	require.NoError(t, DB.Order("id ASC").Find(&storedMessages).Error)
	require.Len(t, storedMessages, 2)
	require.Equal(t, storedRecipients[0].Id, storedMessages[0].RecipientId)
	require.Equal(t, storedRecipients[1].Id, storedMessages[1].RecipientId)
	var eventCount int64
	require.NoError(t, DB.Model(&RecallEvent{}).Count(&eventCount).Error)
	require.Equal(t, int64(1), eventCount)
}

func TestRecallRunIdempotencyMixedRecipientConflictBindsMessagesByUser(t *testing.T) {
	setupRecallRepositoryTestDB(t)

	campaign := newRecallRepositoryCampaign("mixed recipient run")
	require.NoError(t, CreateRecallCampaign(&campaign))
	existing := RecallRecipient{
		CampaignId:          campaign.Id,
		UserId:              451,
		EligibilitySnapshot: `{}`,
		EmailSnapshot:       "existing@example.com",
		LanguageSnapshot:    "en",
		State:               RecallRecipientQueued,
	}
	require.NoError(t, DB.Create(&existing).Error)
	recipients := []RecallRecipient{
		{UserId: existing.UserId, EligibilitySnapshot: `{}`, EmailSnapshot: existing.EmailSnapshot, LanguageSnapshot: "en", State: RecallRecipientQueued},
		{UserId: 452, EligibilitySnapshot: `{}`, EmailSnapshot: "new@example.com", LanguageSnapshot: "en", State: RecallRecipientQueued},
	}
	messages := []RecallMessage{
		{StageNo: 1, TemplateSnapshot: `{"recipient":"existing"}`, State: RecallMessageScheduled},
		{StageNo: 1, TemplateSnapshot: `{"recipient":"new"}`, State: RecallMessageScheduled},
	}
	runEvent := RecallEvent{EventType: "campaign_run", Source: "scheduler", SourceEventId: "recurring:mixed:1721000000", EventData: `{}`}

	inserted, err := InsertRecallRecipientsAndRunEvent(campaign.Id, recipients, messages, runEvent)
	require.NoError(t, err)
	require.Equal(t, 1, inserted)

	var storedRecipients []RecallRecipient
	require.NoError(t, DB.Where("campaign_id = ?", campaign.Id).Find(&storedRecipients).Error)
	recipientByID := make(map[int64]RecallRecipient, len(storedRecipients))
	for _, recipient := range storedRecipients {
		recipientByID[recipient.Id] = recipient
	}
	var storedMessages []RecallMessage
	require.NoError(t, DB.Order("id ASC").Find(&storedMessages).Error)
	require.Len(t, storedMessages, 2)
	messageUsers := make(map[string]int, len(storedMessages))
	for _, message := range storedMessages {
		messageUsers[message.TemplateSnapshot] = recipientByID[message.RecipientId].UserId
	}
	require.Equal(t, existing.UserId, messageUsers[`{"recipient":"existing"}`])
	require.Equal(t, 452, messageUsers[`{"recipient":"new"}`])
}

func TestRecallRunIdempotencyRejectsAmbiguousMessageMapping(t *testing.T) {
	setupRecallRepositoryTestDB(t)

	recipients := []RecallRecipient{
		{UserId: 501, EligibilitySnapshot: `{}`, EmailSnapshot: "one@example.com", LanguageSnapshot: "en", State: RecallRecipientQueued},
		{UserId: 502, EligibilitySnapshot: `{}`, EmailSnapshot: "two@example.com", LanguageSnapshot: "en", State: RecallRecipientQueued},
	}
	messages := []RecallMessage{{StageNo: 1, TemplateSnapshot: `{}`, State: RecallMessageScheduled}}
	runEvent := RecallEvent{EventType: "campaign_run", Source: "scheduler", SourceEventId: "recurring:8:1721000000", EventData: `{}`}

	inserted, err := InsertRecallRecipientsAndRunEvent(8, recipients, messages, runEvent)
	require.Error(t, err)
	require.Zero(t, inserted)
	for _, table := range []any{&RecallRecipient{}, &RecallMessage{}, &RecallEvent{}} {
		var count int64
		require.NoError(t, DB.Model(table).Count(&count).Error)
		require.Zero(t, count)
	}
}

func TestRecallRunIdempotencySchedulesNextStagesOnce(t *testing.T) {
	setupRecallRepositoryTestDB(t)

	messages := []RecallMessage{
		{RecipientId: 999, StageNo: 2, TemplateSnapshot: `{"stage":2}`, State: RecallMessageScheduled},
		{StageNo: 3, TemplateSnapshot: `{"stage":3}`, State: RecallMessageScheduled},
	}
	require.NoError(t, ScheduleNextRecallStages(601, messages))
	require.NoError(t, ScheduleNextRecallStages(601, messages))

	var stored []RecallMessage
	require.NoError(t, DB.Order("stage_no ASC").Find(&stored).Error)
	require.Len(t, stored, 2)
	require.Equal(t, 601, int(stored[0].RecipientId))
	require.Equal(t, 601, int(stored[1].RecipientId))
}

func TestRecallRunIdempotencyMySQLUsesInsertIgnoreForRunOwnership(t *testing.T) {
	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := sqliteDB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	mysqlDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
	})
	require.NoError(t, err)

	result := insertRecallRunEvent(mysqlDB, &RecallEvent{
		CampaignId:    9,
		EventType:     "campaign_run",
		Source:        "scheduler",
		SourceEventId: "recurring:9:1721000000",
		EventData:     `{}`,
	})
	require.NoError(t, result.Error)
	sql := result.Statement.SQL.String()
	require.Contains(t, sql, "INSERT IGNORE INTO")
	require.NotContains(t, sql, "ON DUPLICATE KEY UPDATE")
}

func TestRecallTransitionCampaignRequiresExpectedState(t *testing.T) {
	setupRecallRepositoryTestDB(t)

	campaign := newRecallRepositoryCampaign("conditional transition")
	require.NoError(t, CreateRecallCampaign(&campaign))

	won, err := TransitionRecallCampaign(campaign.Id, []string{RecallCampaignRunning}, RecallCampaignPaused, nil)
	require.NoError(t, err)
	require.False(t, won)
	won, err = TransitionRecallCampaign(campaign.Id, []string{RecallCampaignDraft}, RecallCampaignScheduled, map[string]any{"next_run_at": int64(701)})
	require.NoError(t, err)
	require.True(t, won)

	var stored RecallCampaign
	require.NoError(t, DB.First(&stored, campaign.Id).Error)
	require.Equal(t, RecallCampaignScheduled, stored.Status)
	require.Equal(t, int64(701), stored.NextRunAt)
}

func TestRecallTransitionMessageRequiresLeaseOwnerAndExactState(t *testing.T) {
	setupRecallRepositoryTestDB(t)

	message := RecallMessage{RecipientId: 701, StageNo: 1, TemplateSnapshot: `{}`, ScheduledAt: 800, State: RecallMessageScheduled}
	require.NoError(t, DB.Create(&message).Error)
	won, err := LeaseRecallMessage(message.Id, "node-a", 800, 860)
	require.NoError(t, err)
	require.True(t, won)

	won, err = CompleteRecallMessageLease(message.Id, "node-b", 860, RecallMessageLeased, RecallMessageAccepted, map[string]any{"accepted_at": int64(810)})
	require.NoError(t, err)
	require.False(t, won)
	won, err = CompleteRecallMessageLease(message.Id, "node-a", 860, RecallMessageScheduled, RecallMessageAccepted, map[string]any{"accepted_at": int64(810)})
	require.NoError(t, err)
	require.False(t, won)
	won, err = CompleteRecallMessageLease(message.Id, "node-a", 860, RecallMessageLeased, RecallMessageAccepted, map[string]any{
		"accepted_at":         int64(810),
		"provider_message_id": "provider-701",
	})
	require.NoError(t, err)
	require.True(t, won)

	var stored RecallMessage
	require.NoError(t, DB.First(&stored, message.Id).Error)
	require.Equal(t, RecallMessageAccepted, stored.State)
	require.Equal(t, int64(810), stored.AcceptedAt)
	require.Equal(t, "provider-701", stored.ProviderMessageId)
	require.Empty(t, stored.LeaseOwner)
	require.Zero(t, stored.LeaseExpiresAt)
}

func TestRecallTransitionMessageFencesSameOwnerReacquisition(t *testing.T) {
	setupRecallRepositoryTestDB(t)

	message := RecallMessage{
		RecipientId:      725,
		StageNo:          1,
		TemplateSnapshot: `{}`,
		ScheduledAt:      100,
		State:            RecallMessageScheduled,
	}
	require.NoError(t, DB.Create(&message).Error)
	won, err := LeaseRecallMessage(message.Id, "node-a", 100, 160)
	require.NoError(t, err)
	require.True(t, won)
	won, err = LeaseRecallMessage(message.Id, "node-a", 161, 221)
	require.NoError(t, err)
	require.True(t, won)

	won, err = CompleteRecallMessageLease(message.Id, "node-a", 160, RecallMessageLeased, RecallMessageAccepted, map[string]any{"accepted_at": int64(170)})
	require.NoError(t, err)
	require.False(t, won)
	var stored RecallMessage
	require.NoError(t, DB.First(&stored, message.Id).Error)
	require.Equal(t, RecallMessageLeased, stored.State)
	require.Equal(t, "node-a", stored.LeaseOwner)
	require.Equal(t, int64(221), stored.LeaseExpiresAt)
	require.Zero(t, stored.AcceptedAt)

	won, err = CompleteRecallMessageLease(message.Id, "node-a", 221, RecallMessageLeased, RecallMessageAccepted, map[string]any{"accepted_at": int64(170)})
	require.NoError(t, err)
	require.True(t, won)
	require.NoError(t, DB.First(&stored, message.Id).Error)
	require.Equal(t, RecallMessageAccepted, stored.State)
	require.Equal(t, int64(170), stored.AcceptedAt)
	require.Empty(t, stored.LeaseOwner)
	require.Zero(t, stored.LeaseExpiresAt)
}

func TestRecallTransitionMessageRejectsUnsupportedCompletionFieldsWithoutWrite(t *testing.T) {
	for _, field := range []string{
		"id",
		"recipient_id",
		"stage_no",
		"template_version",
		"template_snapshot",
		"scheduled_at",
		"state",
		"lease_owner",
		"lease_expires_at",
		"created_at",
		"updated_at",
	} {
		t.Run(field, func(t *testing.T) {
			setupRecallRepositoryTestDB(t)

			message := RecallMessage{
				RecipientId:       751,
				StageNo:           1,
				TemplateVersion:   2,
				TemplateSnapshot:  `{"subject":"original"}`,
				ScheduledAt:       700,
				State:             RecallMessageLeased,
				LeaseOwner:        "node-a",
				LeaseExpiresAt:    760,
				LastErrorCode:     "original_code",
				LastErrorMessage:  "original message",
				ProviderMessageId: "original-provider",
			}
			require.NoError(t, DB.Create(&message).Error)
			var before RecallMessage
			require.NoError(t, DB.First(&before, message.Id).Error)

			won, err := CompleteRecallMessageLease(
				message.Id,
				"node-a",
				760,
				RecallMessageLeased,
				RecallMessageAccepted,
				map[string]any{field: 999},
			)
			require.Error(t, err)
			require.False(t, won)

			var after RecallMessage
			require.NoError(t, DB.First(&after, message.Id).Error)
			require.Equal(t, before, after)
		})
	}
}

func TestRecallTransitionCancelsOnlyPendingMessages(t *testing.T) {
	setupRecallRepositoryTestDB(t)

	states := []string{
		RecallMessageScheduled,
		RecallMessageRetryWait,
		RecallMessageAccepted,
		RecallMessageLeased,
		RecallMessageUncertain,
		RecallMessageFailed,
		RecallMessageCancelled,
	}
	messages := make([]RecallMessage, 0, len(states))
	for i, state := range states {
		messages = append(messages, RecallMessage{
			RecipientId:      801,
			StageNo:          i + 1,
			TemplateSnapshot: `{}`,
			State:            state,
		})
	}
	require.NoError(t, DB.Create(&messages).Error)

	cancelled, err := CancelPendingRecallMessages(801, "recipient_converted", 900)
	require.NoError(t, err)
	require.Equal(t, int64(2), cancelled)

	var stored []RecallMessage
	require.NoError(t, DB.Order("stage_no ASC").Find(&stored).Error)
	require.Equal(t, RecallMessageCancelled, stored[0].State)
	require.Equal(t, RecallMessageCancelled, stored[1].State)
	for _, message := range stored[:2] {
		require.Equal(t, "recipient_converted", message.LastErrorCode)
		require.Equal(t, int64(900), message.FailedAt)
	}
	for i, state := range states[2:] {
		require.Equal(t, state, stored[i+2].State)
	}
}
