package model

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/types"

	"github.com/stretchr/testify/require"
)

func TestDeleteOldLogDoesNotAffectSupplierAccountingFacts(t *testing.T) {
	truncateTables(t)
	require.NoError(t, EnsureSupplierAccountingFactSchema(LOG_DB))
	require.NoError(t, LOG_DB.Where("parent_request_id IN ?", []string{"log-delete-pending", "log-delete-captured"}).Delete(&SupplierAccountingFact{}).Error)

	pending, err := PrepareSupplierAccountingFact(context.Background(), LOG_DB, SupplierAccountingFactPrepare{
		AttemptId:        "018f843e-7e3a-7f61-a0a0-000000000401",
		ParentRequestId:  "log-delete-pending",
		SupplierId:       12,
		ContractId:       13,
		BindingVersionId: 11,
		RateVersionId:    14,
		ChannelId:        15,
		ModelName:        "pending-model",
		CoverageScope:    string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1),
	})
	require.NoError(t, err)

	captured, err := PrepareSupplierAccountingFact(context.Background(), LOG_DB, SupplierAccountingFactPrepare{
		AttemptId:        "018f843e-7e3a-7f61-a0a0-000000000402",
		ParentRequestId:  "log-delete-captured",
		SupplierId:       12,
		ContractId:       13,
		BindingVersionId: 11,
		RateVersionId:    14,
		ChannelId:        15,
		ModelName:        "captured-model",
		CoverageScope:    string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1),
	})
	require.NoError(t, err)
	envelope := supplierAccountingCapturedEnvelope(t)
	require.NoError(t, FinalizeSupplierAccountingFactCaptured(context.Background(), LOG_DB, captured.AttemptId, envelope, time.Now().Unix()))

	require.NoError(t, LOG_DB.Create(&Log{CreatedAt: 100, Type: LogTypeConsume, RequestId: "old-log"}).Error)
	require.NoError(t, LOG_DB.Create(&Log{CreatedAt: 300, Type: LogTypeConsume, RequestId: "new-log"}).Error)
	deleted, err := DeleteOldLog(t.Context(), 200, 100)
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)

	var facts []SupplierAccountingFact
	require.NoError(t, LOG_DB.Where("attempt_id IN ?", []string{pending.AttemptId, captured.AttemptId}).Order("attempt_id ASC").Find(&facts).Error)
	require.Len(t, facts, 2)
	require.Equal(t, SupplierAccountingFactStatusPending, facts[0].Status)
	require.Equal(t, SupplierAccountingFactStatusCaptured, facts[1].Status)
	require.NotEmpty(t, facts[1].Payload)
	require.NotEmpty(t, facts[1].PayloadHash)

	var remainingLogs int64
	require.NoError(t, LOG_DB.Model(&Log{}).Where("request_id IN ?", []string{"old-log", "new-log"}).Count(&remainingLogs).Error)
	require.Equal(t, int64(1), remainingLogs)
}
