package service

import (
	"context"
	cryptorand "crypto/rand"
	"errors"
	"fmt"
	"html"
	"math/big"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

const (
	StatusSubscriptionGenericMessage               = "If the request can be completed, you will receive further instructions."
	StatusDiscordEndpointSettingKey                = "status.discord.webhook_endpoint"
	statusSubscriptionVerificationTTLSeconds       = int64(24 * 60 * 60)
	statusDeliveryClaimLeaseSeconds                = int64(30)
	statusDeliveryBaseRetrySeconds                 = int64(30)
	statusDeliveryMaxRetrySeconds                  = int64(6 * 60 * 60)
	statusDeliveryPermanentFailureSuspendThreshold = int64(3)
	statusDeliveryDiscordDestinationID             = int64(1)
	statusDiscordTestMaxPayloadBytes               = 512

	StatusDeliveryResultDelivered = "delivered"
	StatusDeliveryResultPermanent = "permanent"
	StatusDeliveryResultRetry     = "retry"
)

var ErrStatusDiscordTestDeliveryFailed = errors.New("status Discord test delivery failed")

type StatusEmailSender interface {
	SendEmail(subject string, receiver string, content string) error
}

type statusCommonEmailSender struct{}

func (statusCommonEmailSender) SendEmail(subject string, receiver string, content string) error {
	return common.SendEmail(subject, receiver, content)
}

type StatusEmailSubscriptionService struct {
	Email StatusEmailSender
	Now   func() int64
}

type StatusSubscriptionResponse struct {
	Message string `json:"message"`
}

type StatusUnsubscribePreview struct {
	Message        string `json:"message"`
	CanUnsubscribe bool   `json:"can_unsubscribe"`
}

type StatusDiscordTestResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type StatusDeliveryWorker struct {
	Keyring  *StatusSecretKeyring
	Webhook  StatusWebhookSender
	Email    StatusEmailSender
	Now      func() int64
	Jitter   func(max int64) int64
	LeaseTTL int64
}

type statusPublishedDeliveryPayload struct {
	EventID     string `json:"event_id"`
	IncidentID  int64  `json:"incident_id"`
	UpdateID    int64  `json:"update_id"`
	State       string `json:"state"`
	Body        string `json:"body"`
	PublishedAt int64  `json:"published_at"`
}

func NormalizeStatusEmail(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 254 {
		return "", errors.New("invalid email address")
	}
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value || !strings.Contains(address.Address, "@") {
		return "", errors.New("invalid email address")
	}
	local, domain, ok := strings.Cut(address.Address, "@")
	if !ok || local == "" || domain == "" {
		return "", errors.New("invalid email address")
	}
	return strings.ToLower(local + "@" + domain), nil
}

func (service StatusEmailSubscriptionService) Subscribe(_ context.Context, email string, componentIDs []int64) (StatusSubscriptionResponse, error) {
	normalizedEmail, err := NormalizeStatusEmail(email)
	if err != nil {
		return StatusSubscriptionResponse{}, err
	}
	now := time.Now().Unix()
	if service.Now != nil {
		now = service.Now()
	}
	if now <= 0 {
		return StatusSubscriptionResponse{}, errors.New("invalid status subscription time")
	}
	verificationToken, err := GenerateStatusToken()
	if err != nil {
		return StatusSubscriptionResponse{}, err
	}
	manageToken, err := GenerateStatusToken()
	if err != nil {
		return StatusSubscriptionResponse{}, err
	}
	_, shouldNotify, err := model.CreateOrRefreshStatusSubscriber(model.StatusSubscriberMutation{
		Subscriber: model.StatusSubscriber{
			Kind:                  model.StatusSubscriberKindEmail,
			IdentityHash:          HashStatusIdentity(model.StatusSubscriberKindEmail, normalizedEmail),
			DisplayAddress:        normalizedEmail,
			Status:                model.StatusSubscriberPending,
			VerificationTokenHash: HashStatusToken(verificationToken),
			VerificationExpiresAt: now + statusSubscriptionVerificationTTLSeconds,
			ManageTokenHash:       HashStatusToken(manageToken),
			CreatedAt:             now,
			UpdatedAt:             now,
		},
		ComponentIDs: componentIDs,
	})
	if errors.Is(err, model.ErrStatusSubscriberAlreadyActive) {
		return StatusSubscriptionResponse{Message: StatusSubscriptionGenericMessage}, nil
	}
	if err != nil {
		return StatusSubscriptionResponse{}, err
	}
	if shouldNotify {
		sender := service.Email
		if sender == nil {
			sender = statusCommonEmailSender{}
		}
		content := fmt.Sprintf(
			"Verify your status subscription within 24 hours. verification_token=%s manage_token=%s",
			verificationToken,
			manageToken,
		)
		if err := sender.SendEmail("Verify your status subscription", normalizedEmail, content); err != nil {
			return StatusSubscriptionResponse{}, fmt.Errorf("send status subscription email: %w", err)
		}
	}
	return StatusSubscriptionResponse{Message: StatusSubscriptionGenericMessage}, nil
}

func (service StatusEmailSubscriptionService) Verify(token string, now int64) (StatusSubscriptionResponse, error) {
	if strings.TrimSpace(token) != "" && now > 0 {
		if _, err := model.ConsumeStatusSubscriberVerification(HashStatusToken(strings.TrimSpace(token)), now); err != nil {
			return StatusSubscriptionResponse{}, err
		}
	}
	return StatusSubscriptionResponse{Message: StatusSubscriptionGenericMessage}, nil
}

func (service StatusEmailSubscriptionService) PreviewUnsubscribe(token string) (StatusUnsubscribePreview, error) {
	preview := StatusUnsubscribePreview{Message: StatusSubscriptionGenericMessage}
	token = strings.TrimSpace(token)
	if token == "" {
		return preview, nil
	}
	subscriber, found, err := model.GetStatusSubscriberByManageTokenHash(HashStatusToken(token))
	if err != nil {
		return StatusUnsubscribePreview{}, err
	}
	preview.CanUnsubscribe = found && subscriber.Status != model.StatusSubscriberUnsubscribed
	return preview, nil
}

func (service StatusEmailSubscriptionService) Unsubscribe(token string, now int64) (StatusSubscriptionResponse, error) {
	if strings.TrimSpace(token) != "" && now > 0 {
		if _, err := model.UnsubscribeStatusSubscriber(HashStatusToken(strings.TrimSpace(token)), now); err != nil {
			return StatusSubscriptionResponse{}, err
		}
	}
	return StatusSubscriptionResponse{Message: StatusSubscriptionGenericMessage}, nil
}

func ConfigureStatusDiscordEndpoint(actor StatusMutationActor, endpoint string, keyring *StatusSecretKeyring, now int64) (model.StatusSetting, error) {
	actor, err := requireStatusAdmin(actor)
	if err != nil {
		return model.StatusSetting{}, err
	}
	if actor.Role < common.RoleRootUser {
		return model.StatusSetting{}, ErrStatusRootRequired
	}
	if !actor.SecureVerified {
		return model.StatusSetting{}, ErrStatusSecureVerificationRequired
	}
	if keyring == nil || !keyring.Enabled() {
		return model.StatusSetting{}, ErrStatusSecretKeyringDisabled
	}
	if now <= 0 {
		return model.StatusSetting{}, errors.New("invalid status Discord setting time")
	}
	normalizedEndpoint, err := normalizeStatusWebhookEndpoint(endpoint)
	if err != nil {
		return model.StatusSetting{}, err
	}
	encryptedEndpoint, err := keyring.Encrypt(normalizedEndpoint)
	if err != nil {
		return model.StatusSetting{}, err
	}
	return model.UpsertStatusSetting(model.StatusSetting{
		Key: StatusDiscordEndpointSettingKey, Value: encryptedEndpoint, Sensitive: true,
		Version: 1, UpdatedBy: actor.ID, UpdatedAt: now,
	})
}

func StatusDiscordEndpoint(keyring *StatusSecretKeyring) (string, error) {
	if keyring == nil || !keyring.Enabled() {
		return "", ErrStatusSecretKeyringDisabled
	}
	setting, found, err := model.GetStatusSetting(StatusDiscordEndpointSettingKey)
	if err != nil {
		return "", err
	}
	if !found || !setting.Sensitive || setting.Value == "" {
		return "", errors.New("status Discord endpoint is not configured")
	}
	return keyring.Decrypt(setting.Value)
}

func SendStatusDiscordTest(ctx context.Context, actor StatusMutationActor, keyring *StatusSecretKeyring, sender StatusWebhookSender, now int64) (StatusDiscordTestResult, error) {
	actor, err := requireStatusAdmin(actor)
	if err != nil {
		return StatusDiscordTestResult{}, err
	}
	if actor.Role < common.RoleRootUser {
		return StatusDiscordTestResult{}, ErrStatusRootRequired
	}
	if !actor.SecureVerified {
		return StatusDiscordTestResult{}, ErrStatusSecureVerificationRequired
	}
	if keyring == nil || !keyring.Enabled() {
		return StatusDiscordTestResult{}, ErrStatusSecretKeyringDisabled
	}
	if sender == nil || now <= 0 {
		return StatusDiscordTestResult{}, errors.New("status Discord test sender is not configured")
	}
	endpoint, err := StatusDiscordEndpoint(keyring)
	if err != nil {
		return StatusDiscordTestResult{}, err
	}
	body, err := common.Marshal(struct {
		Content string `json:"content"`
	}{Content: "NewAPI status notification test"})
	if err != nil {
		return StatusDiscordTestResult{}, err
	}
	if len(body) > statusDiscordTestMaxPayloadBytes {
		return StatusDiscordTestResult{}, errors.New("status Discord test payload is too large")
	}
	response, err := sender.Send(ctx, StatusWebhookRequest{
		Endpoint: endpoint, EventID: "status.discord.test", Timestamp: now, Body: body,
	})
	if err != nil {
		return StatusDiscordTestResult{Message: "Discord test delivery failed."}, fmt.Errorf("%w: transport failure", ErrStatusDiscordTestDeliveryFailed)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return StatusDiscordTestResult{Message: "Discord test delivery failed."}, fmt.Errorf("%w: HTTP %d", ErrStatusDiscordTestDeliveryFailed, response.StatusCode)
	}
	if _, err := model.RecordStatusDiscordDeliveryResult(
		true, false, statusDeliveryPermanentFailureSuspendThreshold, now,
	); err != nil {
		return StatusDiscordTestResult{}, fmt.Errorf("persist status Discord test recovery: %w", err)
	}
	return StatusDiscordTestResult{Success: true, Message: "Discord test delivery succeeded."}, nil
}

func ClassifyStatusDeliveryResult(statusCode int, deliveryErr error) string {
	if statusCode >= http.StatusBadRequest && statusCode < http.StatusInternalServerError && statusCode != http.StatusTooManyRequests {
		return StatusDeliveryResultPermanent
	}
	if deliveryErr != nil {
		return StatusDeliveryResultRetry
	}
	if statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices {
		return StatusDeliveryResultDelivered
	}
	if statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError || statusCode <= 0 {
		return StatusDeliveryResultRetry
	}
	if statusCode >= http.StatusMultipleChoices && statusCode < http.StatusBadRequest {
		return StatusDeliveryResultPermanent
	}
	return StatusDeliveryResultRetry
}

func StatusDeliveryRetryDelay(attempt int64, jitter func(max int64) int64) int64 {
	if attempt < 1 {
		attempt = 1
	}
	delay := statusDeliveryBaseRetrySeconds
	for current := int64(1); current < attempt && delay < statusDeliveryMaxRetrySeconds; current++ {
		if delay > statusDeliveryMaxRetrySeconds/2 {
			delay = statusDeliveryMaxRetrySeconds
			break
		}
		delay *= 2
	}
	if delay > statusDeliveryMaxRetrySeconds {
		delay = statusDeliveryMaxRetrySeconds
	}
	if jitter == nil {
		jitter = statusDeliveryCryptoJitter
	}
	jitterValue := jitter(delay)
	if jitterValue < 0 {
		jitterValue = 0
	}
	if jitterValue > statusDeliveryMaxRetrySeconds-delay {
		return statusDeliveryMaxRetrySeconds
	}
	return delay + jitterValue
}

func (worker StatusDeliveryWorker) RunOnce(ctx context.Context, workerID string, limit int) (int, error) {
	leaseTTL := worker.LeaseTTL
	if leaseTTL <= 0 {
		leaseTTL = statusDeliveryClaimLeaseSeconds
	}
	if limit <= 0 {
		return 0, errors.New("invalid status delivery limit")
	}
	processed := 0
	var firstPersistenceError error
	for processed < limit {
		now := worker.deliveryNow()
		deliveries, err := model.ClaimStatusDeliveryOutbox(workerID, now, leaseTTL, 1)
		if err != nil {
			return processed, err
		}
		if len(deliveries) == 0 {
			break
		}
		delivery := deliveries[0]
		processed++
		statusCode, deliveryErr, leaseErr := worker.deliverWithLease(ctx, delivery, now, leaseTTL)
		if leaseErr != nil && firstPersistenceError == nil {
			firstPersistenceError = leaseErr
		}
		now = worker.deliveryNow()
		classification := ClassifyStatusDeliveryResult(statusCode, deliveryErr)
		result := model.StatusDeliveryResultMutation{
			ID: delivery.ID, LockToken: delivery.LockToken, ExpectedVersion: delivery.Version,
			DestinationType: delivery.DestinationType, DestinationID: delivery.DestinationID,
			SuspendThreshold: statusDeliveryPermanentFailureSuspendThreshold, Now: now,
		}
		switch classification {
		case StatusDeliveryResultDelivered:
			result.Status = model.StatusDeliveryDelivered
		case StatusDeliveryResultPermanent:
			result.Status = model.StatusDeliveryDead
			result.LastError = statusDeliveryErrorSummary(statusCode, deliveryErr)
		default:
			result.Status = model.StatusDeliveryPending
			result.NextAttemptAt = now + StatusDeliveryRetryDelay(delivery.Attempts+1, worker.Jitter)
			result.LastError = statusDeliveryErrorSummary(statusCode, deliveryErr)
		}
		updated, resultErr := model.CompleteStatusDeliveryOutbox(result)
		if resultErr != nil || !updated {
			if firstPersistenceError == nil {
				if resultErr != nil {
					firstPersistenceError = resultErr
				} else {
					firstPersistenceError = errors.New("status delivery result lost its claim")
				}
			}
			continue
		}
	}
	return processed, firstPersistenceError
}

func (worker StatusDeliveryWorker) deliveryNow() int64 {
	if worker.Now != nil {
		return worker.Now()
	}
	return time.Now().Unix()
}

func (worker StatusDeliveryWorker) deliverWithLease(ctx context.Context, delivery model.StatusDeliveryOutbox, now int64, leaseTTL int64) (int, error, error) {
	stop := make(chan struct{})
	done := make(chan error, 1)
	renewalInterval := time.Duration(leaseTTL) * time.Second / 3
	if renewalInterval <= 0 {
		renewalInterval = time.Second
	}
	gopool.Go(func() {
		ticker := time.NewTicker(renewalInterval)
		defer ticker.Stop()
		var firstRenewalError error
		for {
			select {
			case <-stop:
				done <- firstRenewalError
				return
			case <-ticker.C:
				renewed, err := model.RenewStatusDeliveryOutboxLease(
					delivery.ID, delivery.LockToken, delivery.Version, worker.deliveryNow(), leaseTTL,
				)
				if err != nil {
					if firstRenewalError == nil {
						firstRenewalError = err
					}
					continue
				}
				if !renewed {
					done <- errors.New("status delivery lost its claim during outbound attempt")
					return
				}
			}
		}
	})
	statusCode, deliveryErr := worker.deliver(ctx, delivery, now)
	close(stop)
	return statusCode, deliveryErr, <-done
}

func (worker StatusDeliveryWorker) deliver(ctx context.Context, delivery model.StatusDeliveryOutbox, now int64) (int, error) {
	switch delivery.DestinationType {
	case model.StatusDestinationEmail:
		subscriber, err := model.GetStatusSubscriber(delivery.DestinationID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return http.StatusNotFound, err
			}
			return 0, err
		}
		if subscriber.Kind != model.StatusSubscriberKindEmail || subscriber.Status != model.StatusSubscriberActive {
			return http.StatusGone, errors.New("status email destination is not active")
		}
		var payload statusPublishedDeliveryPayload
		if err := common.Unmarshal([]byte(delivery.Payload), &payload); err != nil {
			return http.StatusUnprocessableEntity, err
		}
		sender := worker.Email
		if sender == nil {
			sender = statusCommonEmailSender{}
		}
		content := fmt.Sprintf("<p>%s</p><p>Status: %s</p>", html.EscapeString(payload.Body), html.EscapeString(payload.State))
		if err := sender.SendEmail("NewAPI status update", subscriber.DisplayAddress, content); err != nil {
			return 0, err
		}
		return http.StatusNoContent, nil
	case model.StatusDestinationWebhook:
		if worker.Keyring == nil || !worker.Keyring.Enabled() {
			return 0, ErrStatusSecretKeyringDisabled
		}
		if worker.Webhook == nil {
			return 0, errors.New("status webhook sender is not configured")
		}
		subscriber, err := model.GetStatusSubscriber(delivery.DestinationID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return http.StatusNotFound, err
			}
			return 0, err
		}
		if subscriber.Kind != model.StatusSubscriberKindWebhook || subscriber.Status != model.StatusSubscriberActive {
			return http.StatusGone, errors.New("status webhook destination is not active")
		}
		endpoint, err := worker.Keyring.Decrypt(subscriber.EncryptedEndpoint)
		if err != nil {
			return 0, err
		}
		secret, err := worker.Keyring.Decrypt(subscriber.EncryptedSigningSecret)
		if err != nil {
			return 0, err
		}
		response, err := worker.Webhook.Send(ctx, StatusWebhookRequest{
			Endpoint: endpoint, EventID: delivery.EventID, Timestamp: now,
			Body: []byte(delivery.Payload), SigningSecret: secret,
		})
		return response.StatusCode, err
	case model.StatusDestinationDiscord:
		if delivery.DestinationID != statusDeliveryDiscordDestinationID {
			return http.StatusUnprocessableEntity, errors.New("unsupported status Discord destination")
		}
		state, err := model.GetStatusDiscordDeliveryState()
		if err != nil {
			return 0, err
		}
		if state.SuspendedAt > 0 {
			return http.StatusGone, errors.New("status Discord destination is suspended")
		}
		if worker.Webhook == nil {
			return 0, errors.New("status webhook sender is not configured")
		}
		endpoint, err := StatusDiscordEndpoint(worker.Keyring)
		if err != nil {
			return 0, err
		}
		var payload statusPublishedDeliveryPayload
		if err := common.Unmarshal([]byte(delivery.Payload), &payload); err != nil {
			return http.StatusUnprocessableEntity, err
		}
		content := strings.TrimSpace(payload.Body)
		if content == "" {
			content = "NewAPI status update: " + payload.State
		}
		if len(content) > 1900 {
			content = content[:1900]
		}
		discordPayload, err := common.Marshal(struct {
			Content string `json:"content"`
		}{Content: content})
		if err != nil {
			return 0, err
		}
		response, err := worker.Webhook.Send(ctx, StatusWebhookRequest{
			Endpoint: endpoint, EventID: delivery.EventID, Timestamp: now, Body: discordPayload,
		})
		return response.StatusCode, err
	default:
		return http.StatusUnprocessableEntity, errors.New("unsupported status delivery destination")
	}
}

func statusDeliveryCryptoJitter(max int64) int64 {
	if max <= 0 {
		return 0
	}
	value, err := cryptorand.Int(cryptorand.Reader, big.NewInt(max+1))
	if err != nil {
		return 0
	}
	return value.Int64()
}

func statusDeliveryErrorSummary(statusCode int, err error) string {
	if errors.Is(err, ErrStatusSecretKeyringDisabled) {
		return "status secret keyring is disabled"
	}
	if err != nil {
		return "status delivery network or transport failure"
	}
	if statusCode > 0 {
		return fmt.Sprintf("status destination returned HTTP %d", statusCode)
	}
	return "status delivery failed"
}
