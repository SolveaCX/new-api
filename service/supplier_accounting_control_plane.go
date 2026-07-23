package service

import (
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

func GetSupplierAccountingControlStatus() (*model.SupplierAccountingControlStatus, error) {
	return model.GetSupplierAccountingControlStatus()
}

func PrepareSupplierAccounting(actorID int, idempotencyKey string, request dto.SupplierAccountingPrepareRequest) (*model.SupplierAccountingControlResult, error) {
	return model.PrepareSupplierAccounting(model.SupplierAccountingPrepareInput{
		SupplierAccountingControlCommand: supplierAccountingControlCommand(actorID, idempotencyKey, request.SupplierAccountingCommandRequest),
		AcceptedCapabilityVersions:       request.AcceptedCapabilityVersions,
	})
}

func ArmSupplierAccounting(actorID int, idempotencyKey string, request dto.SupplierAccountingArmRequest) (*model.SupplierAccountingControlResult, error) {
	return model.ArmSupplierAccounting(model.SupplierAccountingArmInput{
		SupplierAccountingControlCommand: supplierAccountingControlCommand(actorID, idempotencyKey, request.SupplierAccountingCommandRequest),
		CutoverAt:                        request.CutoverAt,
		AcceptedCapabilityVersions:       request.AcceptedCapabilityVersions,
	})
}

func ActivateSupplierAccounting(actorID int, idempotencyKey string, request dto.SupplierAccountingCommandRequest) (*model.SupplierAccountingControlResult, error) {
	return model.ActivateSupplierAccounting(supplierAccountingControlCommand(actorID, idempotencyKey, request))
}

func DisableSupplierAccountingBeforeCutover(actorID int, idempotencyKey string, request dto.SupplierAccountingCommandRequest) (*model.SupplierAccountingControlResult, error) {
	return model.DisableSupplierAccountingBeforeCutover(supplierAccountingControlCommand(actorID, idempotencyKey, request))
}

func DegradeSupplierAccounting(actorID int, idempotencyKey string, request dto.SupplierAccountingDegradeRequest) (*model.SupplierAccountingControlResult, error) {
	return model.DegradeSupplierAccounting(model.SupplierAccountingDegradeInput{
		SupplierAccountingControlCommand: supplierAccountingControlCommand(actorID, idempotencyKey, request.SupplierAccountingCommandRequest),
		StartAt:                          request.StartAt,
		ReasonCategory:                   request.ReasonCategory,
		ExpectedCapabilityVersion:        request.ExpectedCapabilityVersion,
		AffectedCapabilityVersion:        request.AffectedCapabilityVersion,
		AffectedOCIDigest:                request.AffectedOCIDigest,
		AffectedBuildCommit:              request.AffectedBuildCommit,
		EvidenceRefs:                     request.EvidenceRefs,
	})
}

func ResolveSupplierAccountingGap(actorID int, idempotencyKey string, request dto.SupplierAccountingResolveGapRequest) (*model.SupplierAccountingControlResult, error) {
	return model.ResolveSupplierAccountingGap(model.SupplierAccountingResolveGapInput{
		SupplierAccountingControlCommand: supplierAccountingControlCommand(actorID, idempotencyKey, request.SupplierAccountingCommandRequest),
		GapID:                            request.GapId,
		ExpectedGapVersion:               request.ExpectedGapVersion,
		EndAt:                            request.EndAt,
		FinanceDisposition:               request.FinanceDisposition,
	})
}

func ReactivateSupplierAccounting(actorID int, idempotencyKey string, request dto.SupplierAccountingCommandRequest) (*model.SupplierAccountingControlResult, error) {
	return model.ReactivateSupplierAccounting(supplierAccountingControlCommand(actorID, idempotencyKey, request))
}

func ToggleSupplierAccountingMutationGate(actorID int, idempotencyKey string, request dto.SupplierAccountingMutationGateRequest) (*model.SupplierAccountingControlResult, error) {
	if request.Enabled == nil {
		return nil, model.ErrSupplierAccountingCommandInvalid
	}
	return model.ToggleSupplierAccountingMutationGate(model.SupplierAccountingMutationGateInput{
		SupplierAccountingControlCommand: supplierAccountingControlCommand(actorID, idempotencyKey, request.SupplierAccountingCommandRequest),
		Enabled:                          *request.Enabled,
	})
}

func AdoptLegacySupplierAccounting(actorID int, idempotencyKey string, request dto.SupplierAccountingLegacyAdoptionRequest) (*model.SupplierAccountingControlResult, error) {
	return model.AdoptLegacySupplierAccounting(model.SupplierAccountingLegacyAdoptionInput{
		SupplierAccountingControlCommand: supplierAccountingControlCommand(actorID, idempotencyKey, request.SupplierAccountingCommandRequest),
		AcceptedCapabilityVersions:       request.AcceptedCapabilityVersions,
	})
}

func supplierAccountingControlCommand(actorID int, idempotencyKey string, request dto.SupplierAccountingCommandRequest) model.SupplierAccountingControlCommand {
	expectedStateVersion := int64(-1)
	if request.ExpectedStateVersion != nil {
		expectedStateVersion = *request.ExpectedStateVersion
	}
	return model.SupplierAccountingControlCommand{
		ActorID: actorID, IdempotencyKey: idempotencyKey,
		ExpectedStateVersion: expectedStateVersion, Reason: request.Reason,
	}
}
