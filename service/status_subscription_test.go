package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestStatusDeliveryTaskStartsOnlyWhenNotificationsAreEnabled(t *testing.T) {
	originalMaster := common.IsMasterNode
	originalOnce := statusCenterTaskOnce
	originalStatusLaunch := statusCenterTaskLaunch
	originalDeliveryLaunch := statusDeliveryTaskLaunch
	t.Cleanup(func() {
		common.IsMasterNode = originalMaster
		statusCenterTaskOnce = originalOnce
		statusCenterTaskLaunch = originalStatusLaunch
		statusDeliveryTaskLaunch = originalDeliveryLaunch
	})
	common.IsMasterNode = true
	t.Setenv("STATUS_CENTER_ENABLED", "true")
	t.Setenv("ROUTER_ORIGIN", "https://router.flatkey.ai")
	statusCenterTaskLaunch = func(*StatusScheduler) {}
	var launches atomic.Int64
	statusDeliveryTaskLaunch = func(StatusDeliveryWorker) { launches.Add(1) }

	statusCenterTaskOnce = &sync.Once{}
	t.Setenv("STATUS_CENTER_NOTIFICATIONS_ENABLED", "false")
	require.True(t, StartStatusCenterTasks())
	require.Zero(t, launches.Load())

	statusCenterTaskOnce = &sync.Once{}
	t.Setenv("STATUS_CENTER_NOTIFICATIONS_ENABLED", "true")
	require.True(t, StartStatusCenterTasks())
	require.EqualValues(t, 1, launches.Load())
}

type statusEmailMessage struct {
	subject  string
	receiver string
	content  string
}

type statusEmailRecorder struct {
	messages []statusEmailMessage
	err      error
}

func (sender *statusEmailRecorder) SendEmail(subject string, receiver string, content string) error {
	sender.messages = append(sender.messages, statusEmailMessage{subject: subject, receiver: receiver, content: content})
	return sender.err
}

type statusExpiringBatchEmailSender struct {
	db               *gorm.DB
	now              *atomic.Int64
	firstEventID     string
	otherWorker      StatusDeliveryWorker
	messages         []statusEmailMessage
	otherProcessed   int
	otherWorkerError error
}

type statusBlockingEmailSender struct {
	started chan struct{}
	release chan struct{}
}

func (sender *statusBlockingEmailSender) SendEmail(string, string, string) error {
	close(sender.started)
	<-sender.release
	return nil
}

func (sender *statusExpiringBatchEmailSender) SendEmail(subject string, receiver string, content string) error {
	sender.messages = append(sender.messages, statusEmailMessage{subject: subject, receiver: receiver, content: content})
	if len(sender.messages) != 1 {
		return nil
	}
	sender.now.Store(70_031)
	if err := sender.db.Model(&model.StatusDeliveryOutbox{}).
		Where("event_id = ?", sender.firstEventID).
		Updates(map[string]any{"locked_until": 70_061, "updated_at": 70_031}).Error; err != nil {
		return err
	}
	sender.otherProcessed, sender.otherWorkerError = sender.otherWorker.RunOnce(context.Background(), "worker-b", 1)
	return sender.otherWorkerError
}

func statusTokenFromMessage(t *testing.T, content string, name string) string {
	t.Helper()
	prefix := name + "="
	for _, field := range strings.Fields(content) {
		if strings.HasPrefix(field, prefix) {
			return strings.TrimPrefix(field, prefix)
		}
	}
	t.Fatalf("%s was not present in message", name)
	return ""
}

func TestStatusSubscriptionEmailIsNormalizedUniquePendingAndAntiEnumerating(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	component := createStatusIncidentTestComponent(t, model.StatusOperational)
	email := &statusEmailRecorder{}
	service := StatusEmailSubscriptionService{
		Email: email,
		Now:   func() int64 { return 10_000 },
	}

	first, err := service.Subscribe(context.Background(), "  Person@Example.COM ", []int64{component.ID})
	require.NoError(t, err)
	second, err := service.Subscribe(context.Background(), "person@example.com", []int64{component.ID})
	require.NoError(t, err)
	require.Equal(t, first.Message, second.Message)
	require.Equal(t, StatusSubscriptionGenericMessage, first.Message)

	var subscribers []model.StatusSubscriber
	require.NoError(t, db.Find(&subscribers).Error)
	require.Len(t, subscribers, 1)
	require.Equal(t, model.StatusSubscriberKindEmail, subscribers[0].Kind)
	require.Equal(t, model.StatusSubscriberPending, subscribers[0].Status)
	require.Equal(t, "person@example.com", subscribers[0].DisplayAddress)
	require.Equal(t, HashStatusIdentity(model.StatusSubscriberKindEmail, "person@example.com"), subscribers[0].IdentityHash)
	require.NotEmpty(t, subscribers[0].VerificationTokenHash)
	require.NotEmpty(t, subscribers[0].ManageTokenHash)
	require.EqualValues(t, 10_000+statusSubscriptionVerificationTTLSeconds, subscribers[0].VerificationExpiresAt)
	require.Len(t, email.messages, 2)
	verificationToken := statusTokenFromMessage(t, email.messages[1].content, "verification_token")
	manageToken := statusTokenFromMessage(t, email.messages[1].content, "manage_token")
	require.NotContains(t, subscribers[0].VerificationTokenHash, verificationToken)
	require.NotContains(t, subscribers[0].ManageTokenHash, manageToken)

	var filters []model.StatusSubscriberComponent
	require.NoError(t, db.Find(&filters).Error)
	require.Equal(t, []model.StatusSubscriberComponent{{SubscriberID: subscribers[0].ID, ComponentID: component.ID}}, []model.StatusSubscriberComponent{{SubscriberID: filters[0].SubscriberID, ComponentID: filters[0].ComponentID}})
}

func TestStatusSubscriptionEmptyComponentFilterMeansAll(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	service := StatusEmailSubscriptionService{Email: &statusEmailRecorder{}, Now: func() int64 { return 11_000 }}
	_, err := service.Subscribe(context.Background(), "all@example.com", nil)
	require.NoError(t, err)
	var count int64
	require.NoError(t, db.Model(&model.StatusSubscriberComponent{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestStatusSubscriptionVerificationTokenIsOneTimeAndExpiresAfterTwentyFourHours(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	email := &statusEmailRecorder{}
	service := StatusEmailSubscriptionService{Email: email, Now: func() int64 { return 20_000 }}
	_, err := service.Subscribe(context.Background(), "valid@example.com", nil)
	require.NoError(t, err)
	validToken := statusTokenFromMessage(t, email.messages[0].content, "verification_token")

	response, err := service.Verify(validToken, 20_000+statusSubscriptionVerificationTTLSeconds-1)
	require.NoError(t, err)
	require.Equal(t, StatusSubscriptionGenericMessage, response.Message)
	var subscriber model.StatusSubscriber
	require.NoError(t, db.Where("identity_hash = ?", HashStatusIdentity(model.StatusSubscriberKindEmail, "valid@example.com")).First(&subscriber).Error)
	require.Equal(t, model.StatusSubscriberActive, subscriber.Status)
	require.Empty(t, subscriber.VerificationTokenHash)

	response, err = service.Verify(validToken, 20_001+statusSubscriptionVerificationTTLSeconds)
	require.NoError(t, err)
	require.Equal(t, StatusSubscriptionGenericMessage, response.Message)

	email.messages = nil
	service.Now = func() int64 { return 30_000 }
	_, err = service.Subscribe(context.Background(), "expired@example.com", nil)
	require.NoError(t, err)
	expiredToken := statusTokenFromMessage(t, email.messages[0].content, "verification_token")
	_, err = service.Verify(expiredToken, 30_000+statusSubscriptionVerificationTTLSeconds+1)
	require.NoError(t, err)
	subscriber = model.StatusSubscriber{}
	require.NoError(t, db.Where("identity_hash = ?", HashStatusIdentity(model.StatusSubscriberKindEmail, "expired@example.com")).First(&subscriber).Error)
	require.Equal(t, model.StatusSubscriberPending, subscriber.Status)
}

func TestStatusSubscriptionUnsubscribePreviewNeverMutatesAndPostContractDoes(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	email := &statusEmailRecorder{}
	service := StatusEmailSubscriptionService{Email: email, Now: func() int64 { return 40_000 }}
	_, err := service.Subscribe(context.Background(), "leave@example.com", nil)
	require.NoError(t, err)
	manageToken := statusTokenFromMessage(t, email.messages[0].content, "manage_token")

	preview, err := service.PreviewUnsubscribe(manageToken)
	require.NoError(t, err)
	require.True(t, preview.CanUnsubscribe)
	var subscriber model.StatusSubscriber
	require.NoError(t, db.Where("identity_hash = ?", HashStatusIdentity(model.StatusSubscriberKindEmail, "leave@example.com")).First(&subscriber).Error)
	require.Equal(t, model.StatusSubscriberPending, subscriber.Status)

	response, err := service.Unsubscribe(manageToken, 40_100)
	require.NoError(t, err)
	require.Equal(t, StatusSubscriptionGenericMessage, response.Message)
	require.NoError(t, db.First(&subscriber, subscriber.ID).Error)
	require.Equal(t, model.StatusSubscriberUnsubscribed, subscriber.Status)
	preview, err = service.PreviewUnsubscribe(manageToken)
	require.NoError(t, err)
	require.False(t, preview.CanUnsubscribe)
}

func TestStatusSubscriptionWebhookAndDiscordExplicitlyRejectMissingKeyring(t *testing.T) {
	setupStatusServiceTestDB(t)
	disabled, err := ParseStatusSecretKeyring("", "")
	require.NoError(t, err)
	webhook := StatusWebhookRegistrationService{Keyring: disabled, Sender: &statusChallengeSender{echo: true}, Now: func() int64 { return 50_000 }}
	_, err = webhook.Register(context.Background(), "https://hooks.example.com/status", nil)
	require.ErrorIs(t, err, ErrStatusSecretKeyringDisabled)
	_, err = ConfigureStatusDiscordEndpoint(statusRootActor(true), "https://discord.com/api/webhooks/1/token", 0, disabled, 50_000)
	require.ErrorIs(t, err, ErrStatusSecretKeyringDisabled)
}

func TestStatusSubscriptionDiscordEndpointIsEncryptedAndUsesOneGlobalSetting(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	keyring := statusSecretTestKeyring(t)
	_, err := ConfigureStatusDiscordEndpoint(statusAdminActor(), "https://discord.com/api/webhooks/1/token", 0, keyring, 60_000)
	require.ErrorIs(t, err, ErrStatusRootRequired)
	_, err = ConfigureStatusDiscordEndpoint(statusRootActor(false), "https://discord.com/api/webhooks/1/token", 0, keyring, 60_000)
	require.ErrorIs(t, err, ErrStatusSecureVerificationRequired)

	configured, err := ConfigureStatusDiscordEndpoint(statusRootActor(true), "https://discord.com/api/webhooks/1/token", 0, keyring, 60_000)
	require.NoError(t, err)
	require.Equal(t, StatusDiscordEndpointSettingKey, configured.Key)
	require.True(t, configured.Sensitive)
	require.NotContains(t, configured.Value, "discord.com")

	configured, err = ConfigureStatusDiscordEndpoint(statusRootActor(true), "https://discord.com/api/webhooks/2/token", configured.Version, keyring, 60_001)
	require.NoError(t, err)
	var count int64
	require.NoError(t, db.Model(&model.StatusSetting{}).Where("key = ?", StatusDiscordEndpointSettingKey).Count(&count).Error)
	require.EqualValues(t, 1, count)
	endpoint, err := StatusDiscordEndpoint(keyring)
	require.NoError(t, err)
	require.Equal(t, "https://discord.com/api/webhooks/2/token", endpoint)
}

func TestStatusSubscriptionDiscordTestDeliveryRequiresVerifiedRootAndConfiguration(t *testing.T) {
	setupStatusServiceTestDB(t)
	keyring := statusSecretTestKeyring(t)
	sender := &statusDeliveryWebhookRecorder{}

	_, err := SendStatusDiscordTest(context.Background(), statusAdminActor(), keyring, sender, 65_000)
	require.ErrorIs(t, err, ErrStatusRootRequired)
	_, err = SendStatusDiscordTest(context.Background(), statusRootActor(false), keyring, sender, 65_000)
	require.ErrorIs(t, err, ErrStatusSecureVerificationRequired)

	disabled, err := ParseStatusSecretKeyring("", "")
	require.NoError(t, err)
	_, err = SendStatusDiscordTest(context.Background(), statusRootActor(true), disabled, sender, 65_000)
	require.ErrorIs(t, err, ErrStatusSecretKeyringDisabled)
	_, err = SendStatusDiscordTest(context.Background(), statusRootActor(true), keyring, nil, 65_000)
	require.ErrorContains(t, err, "sender is not configured")
	_, err = SendStatusDiscordTest(context.Background(), statusRootActor(true), keyring, sender, 65_000)
	require.ErrorContains(t, err, "not configured")
	require.Empty(t, sender.requests)
}

func TestStatusSubscriptionDiscordTestDeliveryUsesConfiguredSafeSenderWithoutReturningEndpoint(t *testing.T) {
	setupStatusServiceTestDB(t)
	keyring := statusSecretTestKeyring(t)
	_, err := ConfigureStatusDiscordEndpoint(statusRootActor(true), "https://discord.com/api/webhooks/1/token", 0, keyring, 66_000)
	require.NoError(t, err)
	sender := &statusDeliveryWebhookRecorder{}

	result, err := SendStatusDiscordTest(context.Background(), statusRootActor(true), keyring, sender, 66_001)
	require.NoError(t, err)
	require.True(t, result.Success)
	require.Equal(t, "Discord test delivery succeeded.", result.Message)
	require.Len(t, sender.requests, 1)
	require.Equal(t, "https://discord.com/api/webhooks/1/token", sender.requests[0].Endpoint)
	require.Equal(t, "status.discord.test", sender.requests[0].EventID)
	require.LessOrEqual(t, len(sender.requests[0].Body), statusDiscordTestMaxPayloadBytes)
	encoded, err := common.Marshal(result)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "discord.com")
}

func TestStatusSubscriptionDiscordTestDeliveryReportsNonSuccessAndTransportFailure(t *testing.T) {
	setupStatusServiceTestDB(t)
	keyring := statusSecretTestKeyring(t)
	endpoint := "https://discord.com/api/webhooks/1/token"
	_, err := ConfigureStatusDiscordEndpoint(statusRootActor(true), endpoint, 0, keyring, 67_000)
	require.NoError(t, err)

	tests := []struct {
		name   string
		sender *statusDeliveryWebhookRecorder
	}{
		{
			name: "non 2xx",
			sender: &statusDeliveryWebhookRecorder{results: map[string]StatusWebhookResponse{
				endpoint: {StatusCode: http.StatusBadGateway},
			}},
		},
		{
			name: "transport error",
			sender: &statusDeliveryWebhookRecorder{errors: map[string]error{
				endpoint: context.DeadlineExceeded,
			}},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := SendStatusDiscordTest(context.Background(), statusRootActor(true), keyring, testCase.sender, 67_001)
			require.ErrorIs(t, err, ErrStatusDiscordTestDeliveryFailed)
			require.False(t, result.Success)
			require.Contains(t, result.Message, "failed")
			require.NotContains(t, err.Error(), endpoint)
		})
	}
}

func TestStatusSubscriptionSuccessfulDiscordTestClearsSuspensionButFailedTestDoesNot(t *testing.T) {
	setupStatusServiceTestDB(t)
	keyring := statusSecretTestKeyring(t)
	endpoint := "https://discord.com/api/webhooks/recovery/token"
	_, err := ConfigureStatusDiscordEndpoint(statusRootActor(true), endpoint, 0, keyring, 68_000)
	require.NoError(t, err)
	state, err := model.RecordStatusDiscordDeliveryResult(false, true, 1, 68_001)
	require.NoError(t, err)
	require.EqualValues(t, 1, state.FailureCount)
	require.EqualValues(t, 68_001, state.SuspendedAt)

	failedSender := &statusDeliveryWebhookRecorder{results: map[string]StatusWebhookResponse{
		endpoint: {StatusCode: http.StatusBadGateway},
	}}
	result, err := SendStatusDiscordTest(context.Background(), statusRootActor(true), keyring, failedSender, 68_002)
	require.ErrorIs(t, err, ErrStatusDiscordTestDeliveryFailed)
	require.False(t, result.Success)
	state, err = model.GetStatusDiscordDeliveryState()
	require.NoError(t, err)
	require.EqualValues(t, 1, state.FailureCount)
	require.EqualValues(t, 68_001, state.SuspendedAt)

	result, err = SendStatusDiscordTest(
		context.Background(), statusRootActor(true), keyring, &statusDeliveryWebhookRecorder{}, 68_003,
	)
	require.NoError(t, err)
	require.True(t, result.Success)
	state, err = model.GetStatusDiscordDeliveryState()
	require.NoError(t, err)
	require.Zero(t, state.FailureCount)
	require.Zero(t, state.SuspendedAt)
}

func TestStatusDeliveryPortableClaimUsesCASAndExpiredLeaseCanBeReclaimed(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	outbox := model.StatusDeliveryOutbox{
		PublishedUpdateID: 1, DestinationType: model.StatusDestinationEmail, DestinationID: 1,
		EventID: "event-cas", Payload: `{"body":"test"}`, Status: model.StatusDeliveryPending,
		NextAttemptAt: 70_000, Version: 1, CreatedAt: 70_000, UpdatedAt: 70_000,
	}
	require.NoError(t, db.Create(&outbox).Error)

	first, err := model.ClaimStatusDeliveryOutbox("worker-a", 70_000, 30, 1)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, model.StatusDeliveryProcessing, first[0].Status)
	require.NotEmpty(t, first[0].LockToken)

	contended, err := model.ClaimStatusDeliveryOutbox("worker-b", 70_010, 30, 1)
	require.NoError(t, err)
	require.Empty(t, contended)
	reclaimed, err := model.ClaimStatusDeliveryOutbox("worker-b", 70_031, 30, 1)
	require.NoError(t, err)
	require.Len(t, reclaimed, 1)
	require.NotEqual(t, first[0].LockToken, reclaimed[0].LockToken)
	require.Greater(t, reclaimed[0].Version, first[0].Version)
}

func TestStatusDeliveryClaimsEachRowFreshBeforeSequentialOutboundAttempts(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	subscriber := model.StatusSubscriber{
		Kind: model.StatusSubscriberKindEmail, IdentityHash: "fresh-row-claim",
		DisplayAddress: "status@example.com", Status: model.StatusSubscriberActive,
	}
	require.NoError(t, db.Create(&subscriber).Error)
	rows := []model.StatusDeliveryOutbox{
		{
			PublishedUpdateID: 71, DestinationType: model.StatusDestinationEmail, DestinationID: subscriber.ID,
			EventID: "fresh-row-first", Payload: `{"body":"first","state":"degraded"}`, Status: model.StatusDeliveryPending,
			NextAttemptAt: 70_000, Version: 1, CreatedAt: 70_000, UpdatedAt: 70_000,
		},
		{
			PublishedUpdateID: 72, DestinationType: model.StatusDestinationEmail, DestinationID: subscriber.ID,
			EventID: "fresh-row-second", Payload: `{"body":"second","state":"degraded"}`, Status: model.StatusDeliveryPending,
			NextAttemptAt: 70_000, Version: 1, CreatedAt: 70_000, UpdatedAt: 70_000,
		},
	}
	require.NoError(t, db.Create(&rows).Error)

	var now atomic.Int64
	now.Store(70_000)
	otherEmail := &statusEmailRecorder{}
	sender := &statusExpiringBatchEmailSender{
		db: db, now: &now, firstEventID: rows[0].EventID,
		otherWorker: StatusDeliveryWorker{
			Email: otherEmail, LeaseTTL: 30, Now: now.Load,
		},
	}
	worker := StatusDeliveryWorker{Email: sender, LeaseTTL: 30, Now: now.Load}

	processed, err := worker.RunOnce(context.Background(), "worker-a", 2)

	require.NoError(t, err)
	require.Equal(t, 1, processed)
	require.Equal(t, 1, sender.otherProcessed)
	require.Len(t, sender.messages, 1)
	require.Len(t, otherEmail.messages, 1)
	require.Equal(t, "status@example.com", sender.messages[0].receiver)
	require.Equal(t, "status@example.com", otherEmail.messages[0].receiver)
	var delivered int64
	require.NoError(t, db.Model(&model.StatusDeliveryOutbox{}).Where("status = ?", model.StatusDeliveryDelivered).Count(&delivered).Error)
	require.EqualValues(t, 2, delivered)
}

func TestStatusDeliveryRenewsLeaseWhileOutboundAttemptIsBlocked(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	subscriber := model.StatusSubscriber{
		Kind: model.StatusSubscriberKindEmail, IdentityHash: "renew-blocked-attempt",
		DisplayAddress: "blocked@example.com", Status: model.StatusSubscriberActive,
	}
	require.NoError(t, db.Create(&subscriber).Error)
	delivery := model.StatusDeliveryOutbox{
		PublishedUpdateID: 73, DestinationType: model.StatusDestinationEmail, DestinationID: subscriber.ID,
		EventID: "renew-blocked-attempt", Payload: `{"body":"blocked","state":"degraded"}`, Status: model.StatusDeliveryPending,
		NextAttemptAt: 80_000, Version: 1, CreatedAt: 80_000, UpdatedAt: 80_000,
	}
	require.NoError(t, db.Create(&delivery).Error)

	var now atomic.Int64
	now.Store(80_000)
	sender := &statusBlockingEmailSender{started: make(chan struct{}), release: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		_, err := (StatusDeliveryWorker{Email: sender, LeaseTTL: 3, Now: now.Load}).RunOnce(context.Background(), "worker-renew", 1)
		done <- err
	}()
	defer func() {
		close(sender.release)
		require.NoError(t, <-done)
	}()

	<-sender.started
	now.Store(80_002)
	require.Eventually(t, func() bool {
		var stored model.StatusDeliveryOutbox
		return db.First(&stored, delivery.ID).Error == nil && stored.LockedUntil >= 80_005
	}, 3*time.Second, 10*time.Millisecond)

	contended, err := model.ClaimStatusDeliveryOutbox("worker-other", 80_004, 3, 1)
	require.NoError(t, err)
	require.Empty(t, contended)
}

func TestStatusDeliveryClassifiesResultsAndBackoffWithJitter(t *testing.T) {
	require.Equal(t, StatusDeliveryResultDelivered, ClassifyStatusDeliveryResult(http.StatusNoContent, nil))
	require.Equal(t, StatusDeliveryResultPermanent, ClassifyStatusDeliveryResult(http.StatusBadRequest, nil))
	require.Equal(t, StatusDeliveryResultPermanent, ClassifyStatusDeliveryResult(http.StatusNotFound, gorm.ErrRecordNotFound))
	require.Equal(t, StatusDeliveryResultPermanent, ClassifyStatusDeliveryResult(http.StatusUnprocessableEntity, errors.New("invalid immutable payload")))
	require.Equal(t, StatusDeliveryResultRetry, ClassifyStatusDeliveryResult(http.StatusTemporaryRedirect, errors.New("redirect blocked")))
	require.Equal(t, StatusDeliveryResultRetry, ClassifyStatusDeliveryResult(http.StatusTooManyRequests, nil))
	require.Equal(t, StatusDeliveryResultRetry, ClassifyStatusDeliveryResult(http.StatusTooManyRequests, errors.New("rate limited")))
	require.Equal(t, StatusDeliveryResultRetry, ClassifyStatusDeliveryResult(http.StatusServiceUnavailable, nil))
	require.Equal(t, StatusDeliveryResultRetry, ClassifyStatusDeliveryResult(0, context.DeadlineExceeded))
	require.Equal(t, StatusDeliveryResultRetry, ClassifyStatusDeliveryResult(0, errors.New("network unavailable")))

	require.EqualValues(t, statusDeliveryBaseRetrySeconds+7, StatusDeliveryRetryDelay(1, func(max int64) int64 {
		require.EqualValues(t, statusDeliveryBaseRetrySeconds, max)
		return 7
	}))
	require.EqualValues(t, statusDeliveryMaxRetrySeconds, StatusDeliveryRetryDelay(99, func(int64) int64 { return statusDeliveryMaxRetrySeconds }))
}

func TestStatusDeliveryEmailDatabaseFailureRemainsRetryable(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	expectedErr := errors.New("subscriber database unavailable")
	const callbackName = "status-email-subscriber-query-failure"
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Name == "StatusSubscriber" {
			tx.AddError(expectedErr)
		}
	}))
	t.Cleanup(func() { require.NoError(t, db.Callback().Query().Remove(callbackName)) })

	email := &statusEmailRecorder{}
	statusCode, err := (StatusDeliveryWorker{Email: email}).deliver(context.Background(), model.StatusDeliveryOutbox{
		DestinationType: model.StatusDestinationEmail,
		DestinationID:   99,
		Payload:         `{"body":"database failure","state":"degraded"}`,
	}, 81_000)

	require.ErrorIs(t, err, expectedErr)
	require.Zero(t, statusCode)
	require.Empty(t, email.messages)
	require.Equal(t, StatusDeliveryResultRetry, ClassifyStatusDeliveryResult(statusCode, err))
}

func TestStatusDeliveryInternalPermanentFailuresAreDeadWithoutRetry(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, *gorm.DB) (model.StatusDeliveryOutbox, StatusDeliveryWorker)
	}{
		{
			name: "missing subscriber",
			setup: func(t *testing.T, db *gorm.DB) (model.StatusDeliveryOutbox, StatusDeliveryWorker) {
				delivery := model.StatusDeliveryOutbox{
					PublishedUpdateID: 201, DestinationType: model.StatusDestinationWebhook, DestinationID: 999,
					EventID: "missing-subscriber", Payload: `{"body":"test"}`, Status: model.StatusDeliveryPending,
					NextAttemptAt: 100_000, Version: 1, CreatedAt: 100_000, UpdatedAt: 100_000,
				}
				require.NoError(t, db.Create(&delivery).Error)
				return delivery, StatusDeliveryWorker{Keyring: statusSecretTestKeyring(t), Webhook: &statusDeliveryWebhookRecorder{}}
			},
		},
		{
			name: "inactive subscriber",
			setup: func(t *testing.T, db *gorm.DB) (model.StatusDeliveryOutbox, StatusDeliveryWorker) {
				subscriber := model.StatusSubscriber{Kind: model.StatusSubscriberKindEmail, IdentityHash: "inactive", DisplayAddress: "inactive@example.com", Status: model.StatusSubscriberPending}
				require.NoError(t, db.Create(&subscriber).Error)
				delivery := model.StatusDeliveryOutbox{
					PublishedUpdateID: 202, DestinationType: model.StatusDestinationEmail, DestinationID: subscriber.ID,
					EventID: "inactive-subscriber", Payload: `{"body":"test"}`, Status: model.StatusDeliveryPending,
					NextAttemptAt: 100_000, Version: 1, CreatedAt: 100_000, UpdatedAt: 100_000,
				}
				require.NoError(t, db.Create(&delivery).Error)
				return delivery, StatusDeliveryWorker{Email: &statusEmailRecorder{}}
			},
		},
		{
			name: "malformed immutable payload",
			setup: func(t *testing.T, db *gorm.DB) (model.StatusDeliveryOutbox, StatusDeliveryWorker) {
				subscriber := model.StatusSubscriber{Kind: model.StatusSubscriberKindEmail, IdentityHash: "malformed", DisplayAddress: "malformed@example.com", Status: model.StatusSubscriberActive}
				require.NoError(t, db.Create(&subscriber).Error)
				delivery := model.StatusDeliveryOutbox{
					PublishedUpdateID: 203, DestinationType: model.StatusDestinationEmail, DestinationID: subscriber.ID,
					EventID: "malformed-payload", Payload: "{", Status: model.StatusDeliveryPending,
					NextAttemptAt: 100_000, Version: 1, CreatedAt: 100_000, UpdatedAt: 100_000,
				}
				require.NoError(t, db.Create(&delivery).Error)
				return delivery, StatusDeliveryWorker{Email: &statusEmailRecorder{}}
			},
		},
		{
			name: "unsupported destination",
			setup: func(t *testing.T, db *gorm.DB) (model.StatusDeliveryOutbox, StatusDeliveryWorker) {
				delivery := model.StatusDeliveryOutbox{
					PublishedUpdateID: 204, DestinationType: "unsupported", DestinationID: 1,
					EventID: "unsupported-destination", Payload: `{"body":"test"}`, Status: model.StatusDeliveryPending,
					NextAttemptAt: 100_000, Version: 1, CreatedAt: 100_000, UpdatedAt: 100_000,
				}
				require.NoError(t, db.Create(&delivery).Error)
				return delivery, StatusDeliveryWorker{}
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			db := setupStatusServiceTestDB(t)
			delivery, worker := testCase.setup(t, db)
			worker.Now = func() int64 { return 100_000 }
			worker.Jitter = func(int64) int64 { return 0 }
			processed, err := worker.RunOnce(context.Background(), "worker-internal", 1)
			require.NoError(t, err)
			require.Equal(t, 1, processed)
			var stored model.StatusDeliveryOutbox
			require.NoError(t, db.First(&stored, delivery.ID).Error)
			require.Equal(t, model.StatusDeliveryDead, stored.Status)
			require.Zero(t, stored.NextAttemptAt)
			require.EqualValues(t, 1, stored.Attempts)
		})
	}
}

type statusDeliveryWebhookRecorder struct {
	requests []StatusWebhookRequest
	results  map[string]StatusWebhookResponse
	errors   map[string]error
}

func (sender *statusDeliveryWebhookRecorder) Send(_ context.Context, request StatusWebhookRequest) (StatusWebhookResponse, error) {
	sender.requests = append(sender.requests, request)
	if err := sender.errors[request.Endpoint]; err != nil {
		return StatusWebhookResponse{}, err
	}
	if result, ok := sender.results[request.Endpoint]; ok {
		return result, nil
	}
	return StatusWebhookResponse{StatusCode: http.StatusNoContent}, nil
}

func TestStatusDeliveryFailuresDoNotBlockIndependentDestinationsAndPayloadStaysImmutable(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	keyring := statusSecretTestKeyring(t)
	failedEndpoint, err := keyring.Encrypt("https://failed.example.com/hook")
	require.NoError(t, err)
	secret, err := keyring.Encrypt("failed-secret")
	require.NoError(t, err)
	failedSubscriber := model.StatusSubscriber{Kind: model.StatusSubscriberKindWebhook, IdentityHash: "failed", EncryptedEndpoint: failedEndpoint, EncryptedSigningSecret: secret, Status: model.StatusSubscriberActive}
	require.NoError(t, db.Create(&failedSubscriber).Error)
	successEndpoint, err := keyring.Encrypt("https://success.example.com/hook")
	require.NoError(t, err)
	successSecret, err := keyring.Encrypt("success-secret")
	require.NoError(t, err)
	successSubscriber := model.StatusSubscriber{Kind: model.StatusSubscriberKindWebhook, IdentityHash: "success", EncryptedEndpoint: successEndpoint, EncryptedSigningSecret: successSecret, Status: model.StatusSubscriberActive}
	require.NoError(t, db.Create(&successSubscriber).Error)

	originalPayload := `{"event_id":"published-event","body":"degraded"}`
	rows := []model.StatusDeliveryOutbox{
		{PublishedUpdateID: 81, DestinationType: model.StatusDestinationWebhook, DestinationID: failedSubscriber.ID, EventID: "delivery-failed", Payload: originalPayload, Status: model.StatusDeliveryPending, NextAttemptAt: 80_000, Version: 1, CreatedAt: 80_000, UpdatedAt: 80_000},
		{PublishedUpdateID: 81, DestinationType: model.StatusDestinationWebhook, DestinationID: successSubscriber.ID, EventID: "delivery-success", Payload: originalPayload, Status: model.StatusDeliveryPending, NextAttemptAt: 80_000, Version: 1, CreatedAt: 80_000, UpdatedAt: 80_000},
	}
	require.NoError(t, db.Create(&rows).Error)
	sender := &statusDeliveryWebhookRecorder{
		errors:  map[string]error{"https://failed.example.com/hook": context.DeadlineExceeded},
		results: map[string]StatusWebhookResponse{"https://success.example.com/hook": {StatusCode: http.StatusNoContent}},
	}
	worker := StatusDeliveryWorker{Keyring: keyring, Webhook: sender, Email: &statusEmailRecorder{}, Now: func() int64 { return 80_000 }, Jitter: func(int64) int64 { return 0 }}
	processed, err := worker.RunOnce(context.Background(), "worker-a", 10)
	require.NoError(t, err)
	require.Equal(t, 2, processed)
	require.Len(t, sender.requests, 2)

	var stored []model.StatusDeliveryOutbox
	require.NoError(t, db.Order("id ASC").Find(&stored).Error)
	require.Equal(t, model.StatusDeliveryPending, stored[0].Status)
	require.EqualValues(t, 1, stored[0].Attempts)
	require.Greater(t, stored[0].NextAttemptAt, int64(80_000))
	require.Equal(t, model.StatusDeliveryDelivered, stored[1].Status)
	for _, delivery := range stored {
		require.Equal(t, originalPayload, delivery.Payload)
		require.Contains(t, delivery.EventID, "delivery-")
	}
}

func TestStatusDeliveryPermanentFailuresBecomeDeadAndSuspendDestination(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	keyring := statusSecretTestKeyring(t)
	endpoint, err := keyring.Encrypt("https://gone.example.com/hook")
	require.NoError(t, err)
	secret, err := keyring.Encrypt("secret")
	require.NoError(t, err)
	subscriber := model.StatusSubscriber{Kind: model.StatusSubscriberKindWebhook, IdentityHash: "gone", EncryptedEndpoint: endpoint, EncryptedSigningSecret: secret, Status: model.StatusSubscriberActive}
	require.NoError(t, db.Create(&subscriber).Error)
	for index := int64(1); index <= statusDeliveryPermanentFailureSuspendThreshold; index++ {
		require.NoError(t, db.Create(&model.StatusDeliveryOutbox{
			PublishedUpdateID: 90 + index, DestinationType: model.StatusDestinationWebhook, DestinationID: subscriber.ID,
			EventID: "permanent-" + string(rune('0'+index)), Payload: `{"body":"gone"}`, Status: model.StatusDeliveryPending,
			NextAttemptAt: 90_000, Version: 1, CreatedAt: 90_000, UpdatedAt: 90_000,
		}).Error)
	}
	sender := &statusDeliveryWebhookRecorder{results: map[string]StatusWebhookResponse{"https://gone.example.com/hook": {StatusCode: http.StatusGone}}}
	worker := StatusDeliveryWorker{Keyring: keyring, Webhook: sender, Email: &statusEmailRecorder{}, Now: func() int64 { return 90_000 }}
	processed, err := worker.RunOnce(context.Background(), "worker-a", 10)
	require.NoError(t, err)
	require.Equal(t, int(statusDeliveryPermanentFailureSuspendThreshold), processed)

	var dead int64
	require.NoError(t, db.Model(&model.StatusDeliveryOutbox{}).Where("status = ?", model.StatusDeliveryDead).Count(&dead).Error)
	require.Equal(t, statusDeliveryPermanentFailureSuspendThreshold, dead)
	require.NoError(t, db.First(&subscriber, subscriber.ID).Error)
	require.Equal(t, model.StatusSubscriberSuspended, subscriber.Status)
	require.Equal(t, statusDeliveryPermanentFailureSuspendThreshold, subscriber.FailureCount)
}

func TestStatusDeliveryTerminalCompletionRollsBackWhenSubscriberHealthWriteFails(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	subscriber := model.StatusSubscriber{
		Kind: model.StatusSubscriberKindEmail, IdentityHash: "atomic-subscriber-health",
		DisplayAddress: "atomic@example.com", Status: model.StatusSubscriberActive, FailureCount: 2,
	}
	require.NoError(t, db.Create(&subscriber).Error)
	delivery := model.StatusDeliveryOutbox{
		PublishedUpdateID: 299, DestinationType: model.StatusDestinationEmail, DestinationID: subscriber.ID,
		EventID: "atomic-subscriber-health", Payload: `{"body":"atomic","state":"degraded"}`, Status: model.StatusDeliveryPending,
		NextAttemptAt: 109_000, Version: 1, CreatedAt: 109_000, UpdatedAt: 109_000,
	}
	require.NoError(t, db.Create(&delivery).Error)

	expectedErr := errors.New("subscriber health write failed")
	const callbackName = "status-subscriber-health-write-failure"
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Name == "StatusSubscriber" {
			tx.AddError(expectedErr)
		}
	}))
	t.Cleanup(func() { require.NoError(t, db.Callback().Update().Remove(callbackName)) })

	processed, err := (StatusDeliveryWorker{
		Email: &statusEmailRecorder{}, Now: func() int64 { return 109_000 },
	}).RunOnce(context.Background(), "worker-atomic-health", 1)

	require.Equal(t, 1, processed)
	require.ErrorIs(t, err, expectedErr)
	var storedDelivery model.StatusDeliveryOutbox
	require.NoError(t, db.First(&storedDelivery, delivery.ID).Error)
	require.Equal(t, model.StatusDeliveryProcessing, storedDelivery.Status)
	require.NotEmpty(t, storedDelivery.LockToken)
	require.Zero(t, storedDelivery.Attempts)
	var storedSubscriber model.StatusSubscriber
	require.NoError(t, db.First(&storedSubscriber, subscriber.ID).Error)
	require.EqualValues(t, 2, storedSubscriber.FailureCount)
}

type statusDiscordDeliveryStateSnapshot struct {
	FailureCount int64 `json:"failure_count"`
	SuspendedAt  int64 `json:"suspended_at"`
}

func loadStatusDiscordDeliveryStateSnapshot(t *testing.T) (statusDiscordDeliveryStateSnapshot, model.StatusSetting) {
	t.Helper()
	setting, found, err := model.GetStatusSetting("status.discord.delivery_state")
	require.NoError(t, err)
	require.True(t, found)
	var state statusDiscordDeliveryStateSnapshot
	require.NoError(t, common.UnmarshalJsonStr(setting.Value, &state))
	return state, setting
}

func createStatusDiscordDelivery(t *testing.T, db *gorm.DB, publishedUpdateID int64, eventID string, now int64) model.StatusDeliveryOutbox {
	t.Helper()
	delivery := model.StatusDeliveryOutbox{
		PublishedUpdateID: publishedUpdateID, DestinationType: model.StatusDestinationDiscord,
		DestinationID: statusDeliveryDiscordDestinationID, EventID: eventID,
		Payload: `{"body":"Discord update","state":"degraded"}`, Status: model.StatusDeliveryPending,
		NextAttemptAt: now, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, db.Create(&delivery).Error)
	return delivery
}

func TestStatusDeliveryDiscordPermanentFailuresPersistSuspendAndDoNotBlockOtherDestinations(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	keyring := statusSecretTestKeyring(t)
	discordEndpoint := "https://discord.com/api/webhooks/health/token"
	_, err := ConfigureStatusDiscordEndpoint(statusRootActor(true), discordEndpoint, 0, keyring, 110_000)
	require.NoError(t, err)
	permanentSender := &statusDeliveryWebhookRecorder{results: map[string]StatusWebhookResponse{
		discordEndpoint: {StatusCode: http.StatusGone},
	}}

	for attempt := int64(1); attempt <= statusDeliveryPermanentFailureSuspendThreshold; attempt++ {
		now := 110_000 + attempt
		createStatusDiscordDelivery(t, db, 300+attempt, fmt.Sprintf("discord-permanent-%d", attempt), now)
		worker := StatusDeliveryWorker{Keyring: keyring, Webhook: permanentSender, Now: func() int64 { return now }}
		processed, err := worker.RunOnce(context.Background(), fmt.Sprintf("worker-node-%d", attempt), 1)
		require.NoError(t, err)
		require.Equal(t, 1, processed)

		state, setting := loadStatusDiscordDeliveryStateSnapshot(t)
		require.Equal(t, attempt, state.FailureCount)
		require.False(t, setting.Sensitive)
		require.NotContains(t, setting.Value, discordEndpoint)
		if attempt < statusDeliveryPermanentFailureSuspendThreshold {
			require.Zero(t, state.SuspendedAt)
		} else {
			require.Equal(t, now, state.SuspendedAt)
		}
	}

	emailSubscriber := model.StatusSubscriber{
		Kind: model.StatusSubscriberKindEmail, IdentityHash: "discord-independent-email",
		DisplayAddress: "status@example.com", Status: model.StatusSubscriberActive,
	}
	require.NoError(t, db.Create(&emailSubscriber).Error)
	webhookEndpoint, err := keyring.Encrypt("https://independent.example.com/hook")
	require.NoError(t, err)
	webhookSecret, err := keyring.Encrypt("independent-secret")
	require.NoError(t, err)
	webhookSubscriber := model.StatusSubscriber{
		Kind: model.StatusSubscriberKindWebhook, IdentityHash: "discord-independent-webhook",
		EncryptedEndpoint: webhookEndpoint, EncryptedSigningSecret: webhookSecret, Status: model.StatusSubscriberActive,
	}
	require.NoError(t, db.Create(&webhookSubscriber).Error)
	now := int64(110_010)
	discordDelivery := createStatusDiscordDelivery(t, db, 310, "discord-suspended", now)
	rows := []model.StatusDeliveryOutbox{
		{PublishedUpdateID: 311, DestinationType: model.StatusDestinationEmail, DestinationID: emailSubscriber.ID, EventID: "email-independent", Payload: `{"body":"Email update","state":"degraded"}`, Status: model.StatusDeliveryPending, NextAttemptAt: now, Version: 1, CreatedAt: now, UpdatedAt: now},
		{PublishedUpdateID: 312, DestinationType: model.StatusDestinationWebhook, DestinationID: webhookSubscriber.ID, EventID: "webhook-independent", Payload: `{"body":"Webhook update","state":"degraded"}`, Status: model.StatusDeliveryPending, NextAttemptAt: now, Version: 1, CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, db.Create(&rows).Error)
	successSender := &statusDeliveryWebhookRecorder{}
	emailSender := &statusEmailRecorder{}
	worker := StatusDeliveryWorker{Keyring: keyring, Webhook: successSender, Email: emailSender, Now: func() int64 { return now }}
	processed, err := worker.RunOnce(context.Background(), "worker-independent", 10)
	require.NoError(t, err)
	require.Equal(t, 3, processed)
	require.Len(t, successSender.requests, 1)
	require.Equal(t, "https://independent.example.com/hook", successSender.requests[0].Endpoint)
	require.Len(t, emailSender.messages, 1)

	var storedDiscord model.StatusDeliveryOutbox
	require.NoError(t, db.First(&storedDiscord, discordDelivery.ID).Error)
	require.Equal(t, model.StatusDeliveryDead, storedDiscord.Status)
	state, _ := loadStatusDiscordDeliveryStateSnapshot(t)
	require.Equal(t, statusDeliveryPermanentFailureSuspendThreshold, state.FailureCount)
	for _, delivery := range rows {
		var stored model.StatusDeliveryOutbox
		require.NoError(t, db.First(&stored, delivery.ID).Error)
		require.Equal(t, model.StatusDeliveryDelivered, stored.Status)
	}
}

func TestStatusDeliveryDiscordSuccessResetsPersistedFailureState(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	keyring := statusSecretTestKeyring(t)
	endpoint := "https://discord.com/api/webhooks/reset/token"
	_, err := ConfigureStatusDiscordEndpoint(statusRootActor(true), endpoint, 0, keyring, 120_000)
	require.NoError(t, err)

	createStatusDiscordDelivery(t, db, 320, "discord-failure-before-success", 120_001)
	failureWorker := StatusDeliveryWorker{
		Keyring: keyring,
		Webhook: &statusDeliveryWebhookRecorder{results: map[string]StatusWebhookResponse{endpoint: {StatusCode: http.StatusGone}}},
		Now:     func() int64 { return 120_001 },
	}
	processed, err := failureWorker.RunOnce(context.Background(), "worker-failure", 1)
	require.NoError(t, err)
	require.Equal(t, 1, processed)
	state, _ := loadStatusDiscordDeliveryStateSnapshot(t)
	require.EqualValues(t, 1, state.FailureCount)

	createStatusDiscordDelivery(t, db, 321, "discord-success-reset", 120_002)
	successWorker := StatusDeliveryWorker{Keyring: keyring, Webhook: &statusDeliveryWebhookRecorder{}, Now: func() int64 { return 120_002 }}
	processed, err = successWorker.RunOnce(context.Background(), "worker-success", 1)
	require.NoError(t, err)
	require.Equal(t, 1, processed)
	state, _ = loadStatusDiscordDeliveryStateSnapshot(t)
	require.Zero(t, state.FailureCount)
	require.Zero(t, state.SuspendedAt)
}
