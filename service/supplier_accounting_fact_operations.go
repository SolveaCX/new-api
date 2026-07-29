package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
)

const (
	SupplierAccountingFactResolveVoid           = "void"
	SupplierAccountingFactResolveCaptureFromLog = "capture_from_log"
)

var (
	ErrSupplierAccountingFactOperationInvalid  = errors.New("supplier accounting fact operation is invalid")
	ErrSupplierAccountingFactSourceLogMismatch = errors.New("supplier accounting fact source log does not match")
	ErrSupplierAccountingFactSourceLogInvalid  = errors.New("supplier accounting fact source log is invalid")
)

type SupplierAccountingFactResolveInput struct {
	AttemptId   string
	Action      string
	SourceLogId int
	Actor       string
	Reason      string
	Evidence    string
}

func ListPendingSupplierAccountingFacts(ctx context.Context, preparedDay string, cursorID int64, limit int) ([]model.SupplierAccountingFact, error) {
	return model.ListPendingSupplierAccountingFacts(ctx, model.LOG_DB, preparedDay, cursorID, limit)
}

func ResolvePendingSupplierAccountingFact(ctx context.Context, input SupplierAccountingFactResolveInput) (model.SupplierAccountingFact, error) {
	input.AttemptId = strings.TrimSpace(input.AttemptId)
	input.Action = strings.TrimSpace(input.Action)
	input.Actor = strings.TrimSpace(input.Actor)
	input.Reason = strings.TrimSpace(input.Reason)
	input.Evidence = strings.TrimSpace(input.Evidence)
	if input.AttemptId == "" || input.Actor == "" || input.Reason == "" || input.Evidence == "" {
		return model.SupplierAccountingFact{}, ErrSupplierAccountingFactOperationInvalid
	}

	resolution := model.SupplierAccountingFactResolution{
		AttemptId: input.AttemptId, Actor: input.Actor, Reason: input.Reason,
		Evidence: input.Evidence, TerminalAt: time.Now().Unix(),
	}
	switch input.Action {
	case SupplierAccountingFactResolveVoid:
		if input.SourceLogId != 0 {
			return model.SupplierAccountingFact{}, ErrSupplierAccountingFactOperationInvalid
		}
		resolution.Status = model.SupplierAccountingFactStatusVoid
	case SupplierAccountingFactResolveCaptureFromLog:
		if input.SourceLogId <= 0 {
			return model.SupplierAccountingFact{}, ErrSupplierAccountingFactOperationInvalid
		}
		fact, err := model.GetSupplierAccountingFactByAttemptID(ctx, model.LOG_DB, input.AttemptId)
		if err != nil {
			return model.SupplierAccountingFact{}, err
		}
		sourceLog, err := model.GetLogByID(ctx, model.LOG_DB, input.SourceLogId)
		if err != nil {
			return model.SupplierAccountingFact{}, err
		}
		if sourceLog.Type != model.LogTypeConsume || sourceLog.RequestId != fact.ParentRequestId {
			return model.SupplierAccountingFact{}, ErrSupplierAccountingFactSourceLogMismatch
		}
		envelope, sourceAttemptID, err := supplierAccountingEnvelopeFromSourceLog(sourceLog.Other)
		if err != nil {
			return model.SupplierAccountingFact{}, err
		}
		if sourceAttemptID != fact.AttemptId {
			return model.SupplierAccountingFact{}, ErrSupplierAccountingFactSourceLogMismatch
		}
		if envelope.Captured.SupplierId != fact.SupplierId || envelope.Captured.ContractId != fact.ContractId ||
			envelope.Captured.BindingVersionId != fact.BindingVersionId || envelope.Captured.RateVersionId != fact.RateVersionId {
			return model.SupplierAccountingFact{}, ErrSupplierAccountingFactSourceLogMismatch
		}
		resolution.Status = model.SupplierAccountingFactStatusCaptured
		resolution.Envelope = &envelope
		resolution.Evidence = fmt.Sprintf("source_log_id=%d; %s", input.SourceLogId, input.Evidence)
	default:
		return model.SupplierAccountingFact{}, ErrSupplierAccountingFactOperationInvalid
	}

	if err := model.ResolveSupplierAccountingFact(ctx, model.LOG_DB, resolution); err != nil {
		return model.SupplierAccountingFact{}, err
	}
	return model.GetSupplierAccountingFactByAttemptID(ctx, model.LOG_DB, input.AttemptId)
}

func supplierAccountingEnvelopeFromSourceLog(other string) (types.SupplierAccountingEnvelopeV1, string, error) {
	var payload struct {
		Envelope  *types.SupplierAccountingEnvelopeV1 `json:"supplier_accounting_v1"`
		AttemptId string                              `json:"supplier_accounting_attempt_id"`
	}
	if err := common.UnmarshalJsonStr(other, &payload); err != nil || payload.Envelope == nil ||
		payload.Envelope.Disposition != types.SupplierAccountingDispositionCaptured || payload.Envelope.Captured == nil {
		return types.SupplierAccountingEnvelopeV1{}, "", ErrSupplierAccountingFactSourceLogInvalid
	}
	if err := ValidateSupplierAccountingEnvelopeV1(*payload.Envelope); err != nil {
		return types.SupplierAccountingEnvelopeV1{}, "", fmt.Errorf("%w: %v", ErrSupplierAccountingFactSourceLogInvalid, err)
	}
	return *payload.Envelope, strings.TrimSpace(payload.AttemptId), nil
}
