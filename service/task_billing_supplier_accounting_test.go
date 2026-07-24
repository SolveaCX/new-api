package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestRecalculateTaskQuotaSupplierEnvelopeOnlyOnPositiveDelta(t *testing.T) {
	t.Run("positive consume delta", func(t *testing.T) {
		truncate(t)
		const userID, channelID = 71, 71
		seedUser(t, userID, 10_000)
		seedChannel(t, channelID)
		task := makeTask(userID, channelID, 1_000, 0, BillingSourceWallet, 0)

		RecalculateTaskQuota(context.Background(), task, 1_500, "supplier accounting contract")

		log := getLastLog(t)
		require.NotNil(t, log)
		require.Equal(t, model.LogTypeConsume, log.Type)
		require.Nil(t, taskBillingSupplierEnvelope(t, log.Other), "unsupported task accounting must not persist a supplier marker")
	})

	t.Run("negative refund delta", func(t *testing.T) {
		truncate(t)
		const userID, channelID = 72, 72
		seedUser(t, userID, 10_000)
		seedChannel(t, channelID)
		task := makeTask(userID, channelID, 1_500, 0, BillingSourceWallet, 0)

		RecalculateTaskQuota(context.Background(), task, 1_000, "supplier accounting contract")

		log := getLastLog(t)
		require.NotNil(t, log)
		require.Equal(t, model.LogTypeRefund, log.Type)
		require.Nil(t, taskBillingSupplierEnvelope(t, log.Other), "refund rows must remain unchanged")
	})
}

func taskBillingSupplierEnvelope(t *testing.T, other string) *types.SupplierAccountingEnvelopeV1 {
	t.Helper()
	var payload struct {
		Envelope *types.SupplierAccountingEnvelopeV1 `json:"supplier_accounting_v1"`
	}
	require.NoError(t, common.UnmarshalJsonStr(other, &payload))
	return payload.Envelope
}
