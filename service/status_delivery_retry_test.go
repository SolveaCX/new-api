package service

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestRetryStatusDeliveryRequiresVerifiedRootAndRequeues(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	delivery := model.StatusDeliveryOutbox{
		PublishedUpdateID: 21, DestinationType: model.StatusDestinationDiscord,
		EventID: "service-retry", Payload: `{"event":"preserved"}`, Status: model.StatusDeliveryDead,
		LastError: "failed", Version: 3, CreatedAt: 1_000, UpdatedAt: 1_900,
	}
	require.NoError(t, db.Create(&delivery).Error)

	_, err := RetryStatusDelivery(StatusDeliveryRetryInput{
		DeliveryID: delivery.ID, ExpectedVersion: 3, Actor: statusAdminActor(), Reason: "operator retry", Now: 2_000,
	})
	require.True(t, errors.Is(err, ErrStatusRootRequired))

	_, err = RetryStatusDelivery(StatusDeliveryRetryInput{
		DeliveryID: delivery.ID, ExpectedVersion: 3, Actor: statusRootActor(false), Reason: "operator retry", Now: 2_000,
	})
	require.True(t, errors.Is(err, ErrStatusSecureVerificationRequired))

	retried, err := RetryStatusDelivery(StatusDeliveryRetryInput{
		DeliveryID: delivery.ID, ExpectedVersion: 3, Actor: statusRootActor(true),
		Reason: "operator retry api_key=secret-value", Now: 2_000,
	})
	require.NoError(t, err)
	require.Equal(t, model.StatusDeliveryPending, retried.Status)
	require.EqualValues(t, 4, retried.Version)

	var audit model.StatusAuditEvent
	require.NoError(t, db.Where("action = ?", "status.delivery.retry").First(&audit).Error)
	require.Contains(t, audit.Reason, "[REDACTED]")
	require.NotContains(t, audit.Reason, "secret-value")
}
