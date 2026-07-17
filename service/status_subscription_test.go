package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
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
	_, err = ConfigureStatusDiscordEndpoint(statusRootActor(true), "https://discord.com/api/webhooks/1/token", disabled, 50_000)
	require.ErrorIs(t, err, ErrStatusSecretKeyringDisabled)
}

func TestStatusSubscriptionDiscordEndpointIsEncryptedAndUsesOneGlobalSetting(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	keyring := statusSecretTestKeyring(t)
	_, err := ConfigureStatusDiscordEndpoint(statusAdminActor(), "https://discord.com/api/webhooks/1/token", keyring, 60_000)
	require.ErrorIs(t, err, ErrStatusRootRequired)
	_, err = ConfigureStatusDiscordEndpoint(statusRootActor(false), "https://discord.com/api/webhooks/1/token", keyring, 60_000)
	require.ErrorIs(t, err, ErrStatusSecureVerificationRequired)

	configured, err := ConfigureStatusDiscordEndpoint(statusRootActor(true), "https://discord.com/api/webhooks/1/token", keyring, 60_000)
	require.NoError(t, err)
	require.Equal(t, StatusDiscordEndpointSettingKey, configured.Key)
	require.True(t, configured.Sensitive)
	require.NotContains(t, configured.Value, "discord.com")

	configured, err = ConfigureStatusDiscordEndpoint(statusRootActor(true), "https://discord.com/api/webhooks/2/token", keyring, 60_001)
	require.NoError(t, err)
	var count int64
	require.NoError(t, db.Model(&model.StatusSetting{}).Where("key = ?", StatusDiscordEndpointSettingKey).Count(&count).Error)
	require.EqualValues(t, 1, count)
	endpoint, err := StatusDiscordEndpoint(keyring)
	require.NoError(t, err)
	require.Equal(t, "https://discord.com/api/webhooks/2/token", endpoint)
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

func TestStatusDeliveryClassifiesResultsAndBackoffWithJitter(t *testing.T) {
	require.Equal(t, StatusDeliveryResultDelivered, ClassifyStatusDeliveryResult(http.StatusNoContent, nil))
	require.Equal(t, StatusDeliveryResultPermanent, ClassifyStatusDeliveryResult(http.StatusBadRequest, nil))
	require.Equal(t, StatusDeliveryResultRetry, ClassifyStatusDeliveryResult(http.StatusTooManyRequests, nil))
	require.Equal(t, StatusDeliveryResultRetry, ClassifyStatusDeliveryResult(http.StatusServiceUnavailable, nil))
	require.Equal(t, StatusDeliveryResultRetry, ClassifyStatusDeliveryResult(0, context.DeadlineExceeded))
	require.Equal(t, StatusDeliveryResultRetry, ClassifyStatusDeliveryResult(0, errors.New("network unavailable")))

	require.EqualValues(t, statusDeliveryBaseRetrySeconds+7, StatusDeliveryRetryDelay(1, func(max int64) int64 {
		require.EqualValues(t, statusDeliveryBaseRetrySeconds, max)
		return 7
	}))
	require.EqualValues(t, statusDeliveryMaxRetrySeconds, StatusDeliveryRetryDelay(99, func(int64) int64 { return statusDeliveryMaxRetrySeconds }))
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
