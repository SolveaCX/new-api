package model

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type supplierUpdateCommandPayload struct {
	SupplierID      int     `json:"supplier_id"`
	ExpectedVersion int64   `json:"expected_version"`
	Name            *string `json:"name,omitempty"`
	Remark          *string `json:"remark,omitempty"`
}

func UpdateUpstreamSupplierIdempotentForActor(id int, input UpdateUpstreamSupplierInput, idempotencyKey string, actorID int) (*UpstreamSupplier, bool, error) {
	if DB == nil {
		return nil, false, fmt.Errorf("update upstream supplier idempotently: %w", ErrDatabase)
	}
	payload := supplierUpdateCommandPayload{SupplierID: id, ExpectedVersion: input.ExpectedVersion, Name: trimmedStringPointer(input.Name), Remark: trimmedStringPointer(input.Remark)}
	if id <= 0 || actorID <= 0 || input.ExpectedVersion <= 0 || (payload.Name == nil && payload.Remark == nil) || (payload.Name != nil && *payload.Name == "") {
		return nil, false, ErrSupplierInvalidStatus
	}
	var result UpstreamSupplier
	replayed := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		claim, err := ClaimSupplierAdminCommandTx(tx, actorID, SupplierUpdateCommandScope(id), idempotencyKey, payload, supplierAdminCommandResourceSupplier)
		if err != nil {
			return err
		}
		replayed = claim.Replayed
		if replayed {
			return claim.DecodeResult(&result)
		}
		updates := map[string]any{"row_version": gorm.Expr("row_version + ?", 1)}
		if payload.Name != nil {
			updates["name"] = *payload.Name
		}
		if payload.Remark != nil {
			updates["remark"] = *payload.Remark
		}
		updated := tx.Session(&gorm.Session{SkipHooks: true}).Model(&UpstreamSupplier{}).Where("id = ? AND row_version = ?", id, input.ExpectedVersion).Updates(updates)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return supplierRowVersionMiss(tx, &UpstreamSupplier{}, id)
		}
		if err := tx.First(&result, id).Error; err != nil {
			return err
		}
		return CompleteSupplierAdminCommandTx(tx, claim, result.Id, result)
	})
	if err != nil {
		return nil, false, err
	}
	if !replayed {
		refreshLocalChannelCacheAndPublishChanged()
	}
	return &result, replayed, nil
}

type supplierInactivateCommandPayload struct {
	SupplierID      int   `json:"supplier_id"`
	ExpectedVersion int64 `json:"expected_version"`
}

func InactivateUpstreamSupplierIdempotentForActor(id int, expectedVersion int64, idempotencyKey string, actorID int) (*UpstreamSupplier, bool, error) {
	if DB == nil {
		return nil, false, fmt.Errorf("inactivate upstream supplier idempotently: %w", ErrDatabase)
	}
	payload := supplierInactivateCommandPayload{SupplierID: id, ExpectedVersion: expectedVersion}
	if id <= 0 || expectedVersion <= 0 || actorID <= 0 {
		return nil, false, ErrSupplierInvalidStatus
	}
	var result UpstreamSupplier
	replayed := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		claim, err := ClaimSupplierAdminCommandTx(tx, actorID, SupplierInactivateCommandScope(id), idempotencyKey, payload, supplierAdminCommandResourceSupplier)
		if err != nil {
			return err
		}
		replayed = claim.Replayed
		if replayed {
			return claim.DecodeResult(&result)
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&result, id).Error; err != nil {
			return err
		}
		if result.RowVersion != expectedVersion {
			return ErrSupplierVersionConflict
		}
		var contracts []SupplierContract
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("supplier_id = ?", id).Order("id ASC").Find(&contracts).Error; err != nil {
			return err
		}
		contractIDs := make([]int, 0, len(contracts))
		for _, contract := range contracts {
			contractIDs = append(contractIDs, contract.Id)
			if contract.Status == SupplierContractStatusActive {
				return ErrSupplierHasActiveContracts
			}
		}
		if len(contractIDs) > 0 {
			var bindingCount int64
			if err := tx.Model(&Channel{}).Where("supplier_contract_id IN ?", contractIDs).Count(&bindingCount).Error; err != nil {
				return err
			}
			if bindingCount > 0 {
				return ErrSupplierHasChannelBindings
			}
		}
		if result.Status == SupplierStatusInactive {
			return ErrSupplierVersionConflict
		}
		if result.Status != SupplierStatusInactive {
			updated := tx.Session(&gorm.Session{SkipHooks: true}).Model(&UpstreamSupplier{}).Where("id = ? AND row_version = ? AND status = ?", id, expectedVersion, SupplierStatusActive).
				Updates(map[string]any{"status": SupplierStatusInactive, "row_version": gorm.Expr("row_version + ?", 1)})
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return ErrSupplierVersionConflict
			}
			if err := tx.First(&result, id).Error; err != nil {
				return err
			}
		}
		return CompleteSupplierAdminCommandTx(tx, claim, result.Id, result)
	})
	if err != nil {
		return nil, false, err
	}
	if !replayed {
		refreshLocalChannelCacheAndPublishChanged()
	}
	return &result, replayed, nil
}

type contractUpdateCommandPayload struct {
	ContractID      int     `json:"contract_id"`
	ExpectedVersion int64   `json:"expected_version"`
	Name            *string `json:"name,omitempty"`
	ContractNo      *string `json:"contract_no,omitempty"`
	Remark          *string `json:"remark,omitempty"`
	RpmLimit        *int64  `json:"rpm_limit,omitempty"`
	TpmLimit        *int64  `json:"tpm_limit,omitempty"`
	MaxConcurrency  *int    `json:"max_concurrency,omitempty"`
}

func UpdateSupplierContractIdempotentForActor(id int, input UpdateSupplierContractInput, idempotencyKey string, actorID int) (*SupplierContract, bool, error) {
	if DB == nil {
		return nil, false, fmt.Errorf("update supplier contract idempotently: %w", ErrDatabase)
	}
	payload := contractUpdateCommandPayload{ContractID: id, ExpectedVersion: input.ExpectedVersion, Name: trimmedStringPointer(input.Name), ContractNo: trimmedStringPointer(input.ContractNo), Remark: trimmedStringPointer(input.Remark), RpmLimit: input.RpmLimit, TpmLimit: input.TpmLimit, MaxConcurrency: input.MaxConcurrency}
	if id <= 0 || actorID <= 0 || input.ExpectedVersion <= 0 || invalidContractUpdatePayload(payload) {
		return nil, false, ErrSupplierInvalidContract
	}
	var result SupplierContract
	replayed := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		claim, err := ClaimSupplierAdminCommandTx(tx, actorID, SupplierContractUpdateCommandScope(id), idempotencyKey, payload, supplierAdminCommandResourceContract)
		if err != nil {
			return err
		}
		replayed = claim.Replayed
		if replayed {
			return claim.DecodeResult(&result)
		}
		updates := map[string]any{"row_version": gorm.Expr("row_version + ?", 1)}
		for key, value := range map[string]any{"name": payload.Name, "contract_no": payload.ContractNo, "remark": payload.Remark, "rpm_limit": payload.RpmLimit, "tpm_limit": payload.TpmLimit, "max_concurrency": payload.MaxConcurrency} {
			switch pointer := value.(type) {
			case *string:
				if pointer != nil {
					updates[key] = *pointer
				}
			case *int64:
				if pointer != nil {
					updates[key] = *pointer
				}
			case *int:
				if pointer != nil {
					updates[key] = *pointer
				}
			}
		}
		updated := tx.Session(&gorm.Session{SkipHooks: true}).Model(&SupplierContract{}).Where("id = ? AND row_version = ?", id, input.ExpectedVersion).Updates(updates)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return supplierRowVersionMiss(tx, &SupplierContract{}, id)
		}
		if err := tx.First(&result, id).Error; err != nil {
			return err
		}
		return CompleteSupplierAdminCommandTx(tx, claim, result.Id, result)
	})
	if err != nil {
		return nil, false, err
	}
	if !replayed {
		refreshLocalChannelCacheAndPublishChanged()
	}
	return &result, replayed, nil
}

func InactivateSupplierContractIdempotentForActor(id int, expectedVersion int64, idempotencyKey string, actorID int) (*SupplierContract, bool, error) {
	if DB == nil {
		return nil, false, fmt.Errorf("inactivate supplier contract idempotently: %w", ErrDatabase)
	}
	payload := struct {
		ContractID      int   `json:"contract_id"`
		ExpectedVersion int64 `json:"expected_version"`
	}{id, expectedVersion}
	if id <= 0 || expectedVersion <= 0 || actorID <= 0 {
		return nil, false, ErrSupplierInvalidContract
	}
	var result SupplierContract
	replayed := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		claim, err := ClaimSupplierAdminCommandTx(tx, actorID, SupplierContractInactivateCommandScope(id), idempotencyKey, payload, supplierAdminCommandResourceContract)
		if err != nil {
			return err
		}
		replayed = claim.Replayed
		if replayed {
			return claim.DecodeResult(&result)
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&result, id).Error; err != nil {
			return err
		}
		if result.RowVersion != expectedVersion {
			return ErrSupplierVersionConflict
		}
		var bindingCount int64
		if err := tx.Model(&Channel{}).Where("supplier_contract_id = ?", id).Count(&bindingCount).Error; err != nil {
			return err
		}
		if bindingCount > 0 {
			return ErrSupplierContractBound
		}
		if result.Status == SupplierContractStatusInactive {
			return ErrSupplierVersionConflict
		}
		if result.Status != SupplierContractStatusInactive {
			updated := tx.Session(&gorm.Session{SkipHooks: true}).Model(&SupplierContract{}).Where("id = ? AND row_version = ? AND status = ?", id, expectedVersion, SupplierContractStatusActive).Updates(map[string]any{"status": SupplierContractStatusInactive, "row_version": gorm.Expr("row_version + ?", 1)})
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return ErrSupplierVersionConflict
			}
			if err := tx.First(&result, id).Error; err != nil {
				return err
			}
		}
		return CompleteSupplierAdminCommandTx(tx, claim, result.Id, result)
	})
	if err != nil {
		return nil, false, err
	}
	if !replayed {
		refreshLocalChannelCacheAndPublishChanged()
	}
	return &result, replayed, nil
}

type channelBindingCommandPayload struct {
	ChannelID          int  `json:"channel_id"`
	ExpectedContractID int  `json:"expected_contract_id"`
	DesiredContractID  *int `json:"desired_contract_id"`
}

func SetChannelSupplierContractCASIdempotentForActor(channelID, expectedContractID int, desiredContractID *int, idempotencyKey string, actorID int) (*SupplierChannelBinding, bool, error) {
	if DB == nil {
		return nil, false, fmt.Errorf("set channel supplier contract idempotently: %w", ErrDatabase)
	}
	payload := channelBindingCommandPayload{channelID, expectedContractID, desiredContractID}
	if channelID <= 0 || expectedContractID < 0 || actorID <= 0 || (desiredContractID != nil && *desiredContractID <= 0) {
		return nil, false, ErrSupplierInvalidContract
	}
	scope := SupplierChannelUnbindCommandScope(channelID)
	if desiredContractID != nil {
		scope = SupplierChannelBindCommandScope(channelID)
	}
	var result SupplierChannelBinding
	replayed := false
	changed := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		claim, err := ClaimSupplierAdminCommandTx(tx, actorID, scope, idempotencyKey, payload, supplierAdminCommandResourceBinding)
		if err != nil {
			return err
		}
		replayed = claim.Replayed
		if replayed {
			return claim.DecodeResult(&result)
		}
		changed, err = setChannelSupplierContractCASTx(tx, channelID, expectedContractID, desiredContractID, actorID)
		if err != nil {
			return err
		}
		if err := tx.Model(&Channel{}).Select("id AS channel_id", "name AS channel_name", "status AS channel_status", "supplier_contract_id").Where("id = ?", channelID).First(&result).Error; err != nil {
			return err
		}
		return CompleteSupplierAdminCommandTx(tx, claim, channelID, result)
	})
	if err != nil {
		return nil, false, err
	}
	if changed {
		refreshLocalChannelCacheAndPublishChanged()
	}
	return &result, replayed, nil
}

func CreateSupplierInventoryAdjustmentIdempotentForActor(adjustment *SupplierInventoryAdjustment, idempotencyKey string, actorID int) (*SupplierInventoryAdjustment, bool, error) {
	if DB == nil {
		return nil, false, fmt.Errorf("create supplier inventory adjustment idempotently: %w", ErrDatabase)
	}
	if adjustment == nil || actorID <= 0 {
		return nil, false, ErrSupplierInvalidInventory
	}
	payload := struct {
		ContractID    int    `json:"contract_id"`
		DeltaMicroUsd int64  `json:"delta_micro_usd"`
		Type          string `json:"type"`
		Reason        string `json:"reason"`
	}{adjustment.ContractId, adjustment.DeltaMicroUsd, strings.TrimSpace(adjustment.Type), strings.TrimSpace(adjustment.Reason)}
	var result SupplierInventoryAdjustment
	replayed := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		claim, err := ClaimSupplierAdminCommandTx(tx, actorID, SupplierInventoryCommandScope(adjustment.ContractId), idempotencyKey, payload, supplierAdminCommandResourceInventory)
		if err != nil {
			return err
		}
		replayed = claim.Replayed
		if replayed {
			return claim.DecodeResult(&result)
		}
		if _, _, _, err := lockActiveSupplierContractChain(tx, adjustment.ContractId, true); err != nil {
			return err
		}
		legacy := SupplierInventoryAdjustment{}
		legacyErr := tx.Where("contract_id = ? AND created_by = ? AND idempotency_key = ?", adjustment.ContractId, actorID, strings.TrimSpace(idempotencyKey)).Take(&legacy).Error
		if legacyErr == nil {
			if legacy.DeltaMicroUsd != adjustment.DeltaMicroUsd || legacy.Type != payload.Type || legacy.Reason != payload.Reason {
				return ErrSupplierAdminIdempotencyConflict
			}
			result = legacy
			replayed = true
			return CompleteSupplierAdminCommandTx(tx, claim, result.Id, result)
		}
		if legacyErr != gorm.ErrRecordNotFound {
			return legacyErr
		}
		result = SupplierInventoryAdjustment{ContractId: adjustment.ContractId, DeltaMicroUsd: adjustment.DeltaMicroUsd, Type: payload.Type, Reason: payload.Reason, IdempotencyKey: strings.TrimSpace(idempotencyKey), CreatedBy: actorID}
		if err := tx.Create(&result).Error; err != nil {
			return err
		}
		return CompleteSupplierAdminCommandTx(tx, claim, result.Id, result)
	})
	if err != nil {
		return nil, false, err
	}
	if !replayed {
		refreshLocalChannelCacheAndPublishChanged()
	}
	return &result, replayed, nil
}

func CreateSupplierStatisticsExclusionRuleIdempotentForActor(userID int, action string, actorID int, reason, idempotencyKey string) (*SupplierStatisticsExclusionRule, bool, error) {
	if DB == nil {
		return nil, false, fmt.Errorf("create supplier exclusion rule idempotently: %w", ErrDatabase)
	}
	payload := struct {
		UserID int    `json:"user_id"`
		Action string `json:"action"`
		Reason string `json:"reason"`
	}{userID, strings.TrimSpace(action), strings.TrimSpace(reason)}
	if userID <= 0 || actorID <= 0 || !isSupplierStatisticsAction(payload.Action) {
		return nil, false, ErrSupplierInvalidStatsRule
	}
	var result SupplierStatisticsExclusionRule
	replayed := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		claim, err := ClaimSupplierAdminCommandTx(tx, actorID, SupplierAdminCommandScopeCreateExclusion, idempotencyKey, payload, supplierAdminCommandResourceExclusion)
		if err != nil {
			return err
		}
		replayed = claim.Replayed
		if replayed {
			return claim.DecodeResult(&result)
		}
		legacy := SupplierStatisticsExclusionRule{}
		legacyErr := tx.Where("created_by = ? AND idempotency_key = ?", actorID, strings.TrimSpace(idempotencyKey)).Take(&legacy).Error
		if legacyErr == nil {
			if legacy.UserId != userID || legacy.Action != payload.Action || legacy.Reason != payload.Reason {
				return ErrSupplierAdminIdempotencyConflict
			}
			result = legacy
			replayed = true
			return CompleteSupplierAdminCommandTx(tx, claim, result.Id, result)
		}
		if legacyErr != gorm.ErrRecordNotFound {
			return legacyErr
		}
		result = SupplierStatisticsExclusionRule{UserId: userID, Action: payload.Action, Reason: payload.Reason, IdempotencyKey: strings.TrimSpace(idempotencyKey), CreatedBy: actorID}
		if err := tx.Create(&result).Error; err != nil {
			return err
		}
		return CompleteSupplierAdminCommandTx(tx, claim, result.Id, result)
	})
	if err != nil {
		return nil, false, err
	}
	if !replayed {
		refreshLocalChannelCacheAndPublishChanged()
	}
	return &result, replayed, nil
}

func trimmedStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}
func supplierRowVersionMiss(tx *gorm.DB, entity any, id int) error {
	var count int64
	if err := tx.Model(entity).Where("id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return gorm.ErrRecordNotFound
	}
	return ErrSupplierVersionConflict
}
func invalidContractUpdatePayload(p contractUpdateCommandPayload) bool {
	if p.Name == nil && p.ContractNo == nil && p.Remark == nil && p.RpmLimit == nil && p.TpmLimit == nil && p.MaxConcurrency == nil {
		return true
	}
	return (p.Name != nil && *p.Name == "") || (p.ContractNo != nil && *p.ContractNo == "") || (p.RpmLimit != nil && *p.RpmLimit < 0) || (p.TpmLimit != nil && *p.TpmLimit < 0) || (p.MaxConcurrency != nil && *p.MaxConcurrency < 0)
}
