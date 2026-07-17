package service

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type StatusDeliveryRetryInput struct {
	DeliveryID      int64
	ExpectedVersion int64
	Actor           StatusMutationActor
	Reason          string
	Now             int64
}

func RetryStatusDelivery(input StatusDeliveryRetryInput) (model.StatusDeliveryOutbox, error) {
	actor, err := requireStatusAdmin(input.Actor)
	if err != nil {
		return model.StatusDeliveryOutbox{}, err
	}
	if actor.Role < common.RoleRootUser {
		return model.StatusDeliveryOutbox{}, ErrStatusRootRequired
	}
	if !actor.SecureVerified {
		return model.StatusDeliveryOutbox{}, ErrStatusSecureVerificationRequired
	}
	reason := strings.TrimSpace(input.Reason)
	if input.DeliveryID <= 0 || input.ExpectedVersion <= 0 || input.Now <= 0 || reason == "" {
		return model.StatusDeliveryOutbox{}, ErrStatusInvalidMutation
	}
	delivery, err := model.RetryDeadStatusDelivery(model.StatusDeliveryRetryMutation{
		ID: input.DeliveryID, ExpectedVersion: input.ExpectedVersion, Now: input.Now,
		Audit: model.StatusAuditMutation{
			ActorID: actor.ID, ActorType: actor.ActorType, Action: "status.delivery.retry",
			Reason: sanitizeStatusEvidence(reason), CreatedAt: input.Now,
		},
	})
	if errors.Is(err, model.ErrStatusInvalidDeliveryMutation) {
		return model.StatusDeliveryOutbox{}, ErrStatusInvalidMutation
	}
	return delivery, err
}
