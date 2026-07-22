package model

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const supplierAdminDefaultPageSize = 50

var ErrSupplierBindingChanged = errors.New("supplier channel binding changed concurrently")

type SupplierPage struct {
	Offset int
	Limit  int
	Status string
}

type UpdateUpstreamSupplierInput struct {
	Name   *string
	Remark *string
}

type UpdateSupplierContractInput struct {
	Name           *string
	ContractNo     *string
	Remark         *string
	RpmLimit       *int64
	TpmLimit       *int64
	MaxConcurrency *int
}

type SupplierChannelBinding struct {
	ChannelId          int    `json:"channel_id"`
	ChannelName        string `json:"channel_name"`
	ChannelStatus      int    `json:"channel_status"`
	SupplierContractId *int   `json:"supplier_contract_id"`
}

func normalizeSupplierPage(page SupplierPage) SupplierPage {
	if page.Offset < 0 {
		page.Offset = 0
	}
	if page.Limit <= 0 {
		page.Limit = supplierAdminDefaultPageSize
	}
	if page.Limit > 500 {
		page.Limit = 500
	}
	page.Status = strings.TrimSpace(page.Status)
	return page
}

func CreateUpstreamSupplier(supplier *UpstreamSupplier) error {
	if DB == nil {
		return fmt.Errorf("create upstream supplier: %w", ErrDatabase)
	}
	if supplier == nil {
		return ErrSupplierInvalidStatus
	}
	created := UpstreamSupplier{Name: supplier.Name, Remark: supplier.Remark, Status: SupplierStatusActive}
	if err := DB.Create(&created).Error; err != nil {
		return err
	}
	*supplier = created
	refreshLocalChannelCacheAndPublishChanged()
	return nil
}

func GetUpstreamSupplierByID(id int) (*UpstreamSupplier, error) {
	if DB == nil {
		return nil, fmt.Errorf("get upstream supplier: %w", ErrDatabase)
	}
	var supplier UpstreamSupplier
	if err := DB.First(&supplier, id).Error; err != nil {
		return nil, err
	}
	return &supplier, nil
}

func ListUpstreamSuppliers(page SupplierPage) ([]UpstreamSupplier, int64, error) {
	if DB == nil {
		return nil, 0, fmt.Errorf("list upstream suppliers: %w", ErrDatabase)
	}
	page = normalizeSupplierPage(page)
	query := DB.Model(&UpstreamSupplier{})
	if page.Status != "" {
		query = query.Where("status = ?", page.Status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var suppliers []UpstreamSupplier
	if err := query.Order("id DESC").Offset(page.Offset).Limit(page.Limit).Find(&suppliers).Error; err != nil {
		return nil, 0, err
	}
	return suppliers, total, nil
}

func UpdateUpstreamSupplier(id int, input UpdateUpstreamSupplierInput) (*UpstreamSupplier, error) {
	if DB == nil {
		return nil, fmt.Errorf("update upstream supplier: %w", ErrDatabase)
	}
	updates := make(map[string]any, 2)
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, ErrSupplierInvalidStatus
		}
		updates["name"] = name
	}
	if input.Remark != nil {
		updates["remark"] = strings.TrimSpace(*input.Remark)
	}
	var supplier UpstreamSupplier
	if err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&supplier, id).Error; err != nil {
			return err
		}
		if len(updates) > 0 {
			if err := tx.Model(&supplier).Updates(updates).Error; err != nil {
				return err
			}
		}
		return tx.First(&supplier, id).Error
	}); err != nil {
		return nil, err
	}
	refreshLocalChannelCacheAndPublishChanged()
	return &supplier, nil
}

func InactivateUpstreamSupplier(id int) error {
	if DB == nil {
		return fmt.Errorf("inactivate upstream supplier: %w", ErrDatabase)
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var supplier UpstreamSupplier
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&supplier, id).Error; err != nil {
			return err
		}
		alreadyInactive := supplier.Status == SupplierStatusInactive
		var contracts []SupplierContract
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("supplier_id = ?", id).Order("id ASC").Find(&contracts).Error; err != nil {
			return err
		}
		contractIds := make([]int, 0, len(contracts))
		for _, contract := range contracts {
			contractIds = append(contractIds, contract.Id)
			if contract.Status == SupplierContractStatusActive {
				return ErrSupplierHasActiveContracts
			}
		}
		if len(contractIds) > 0 {
			var bindingCount int64
			if err := tx.Model(&Channel{}).Where("supplier_contract_id IN ?", contractIds).Count(&bindingCount).Error; err != nil {
				return err
			}
			if bindingCount > 0 {
				return ErrSupplierHasChannelBindings
			}
		}
		if alreadyInactive {
			return nil
		}
		updatedAt, err := getSupplierDBTimestamp(tx)
		if err != nil {
			return err
		}
		return tx.Model(&UpstreamSupplier{}).Where("id = ? AND status = ?", id, SupplierStatusActive).UpdateColumns(map[string]any{
			"status":     SupplierStatusInactive,
			"updated_at": updatedAt,
		}).Error
	})
	if err != nil {
		return err
	}
	refreshLocalChannelCacheAndPublishChanged()
	return nil
}

func CreateSupplierContract(contract *SupplierContract) error {
	if DB == nil {
		return fmt.Errorf("create supplier contract: %w", ErrDatabase)
	}
	if contract == nil {
		return ErrSupplierInvalidContract
	}
	created := SupplierContract{
		SupplierId:     contract.SupplierId,
		Name:           contract.Name,
		ContractNo:     contract.ContractNo,
		Remark:         contract.Remark,
		Status:         SupplierContractStatusActive,
		RpmLimit:       contract.RpmLimit,
		TpmLimit:       contract.TpmLimit,
		MaxConcurrency: contract.MaxConcurrency,
	}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		var supplier UpstreamSupplier
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&supplier, created.SupplierId).Error; err != nil {
			return err
		}
		if supplier.Status != SupplierStatusActive {
			return ErrSupplierInactive
		}
		return tx.Create(&created).Error
	}); err != nil {
		return err
	}
	*contract = created
	refreshLocalChannelCacheAndPublishChanged()
	return nil
}

func GetSupplierContractByID(id int) (*SupplierContract, error) {
	if DB == nil {
		return nil, fmt.Errorf("get supplier contract: %w", ErrDatabase)
	}
	var contract SupplierContract
	if err := DB.First(&contract, id).Error; err != nil {
		return nil, err
	}
	return &contract, nil
}

func ListSupplierContracts(supplierId int, page SupplierPage) ([]SupplierContract, int64, error) {
	if DB == nil {
		return nil, 0, fmt.Errorf("list supplier contracts: %w", ErrDatabase)
	}
	page = normalizeSupplierPage(page)
	query := DB.Model(&SupplierContract{})
	if supplierId > 0 {
		query = query.Where("supplier_id = ?", supplierId)
	}
	if page.Status != "" {
		query = query.Where("status = ?", page.Status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var contracts []SupplierContract
	if err := query.Order("id DESC").Offset(page.Offset).Limit(page.Limit).Find(&contracts).Error; err != nil {
		return nil, 0, err
	}
	return contracts, total, nil
}

func UpdateSupplierContract(id int, input UpdateSupplierContractInput) (*SupplierContract, error) {
	if DB == nil {
		return nil, fmt.Errorf("update supplier contract: %w", ErrDatabase)
	}
	updates := make(map[string]any, 6)
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, ErrSupplierInvalidContract
		}
		updates["name"] = name
	}
	if input.ContractNo != nil {
		contractNo := strings.TrimSpace(*input.ContractNo)
		if contractNo == "" {
			return nil, ErrSupplierInvalidContract
		}
		updates["contract_no"] = contractNo
	}
	if input.Remark != nil {
		updates["remark"] = strings.TrimSpace(*input.Remark)
	}
	if input.RpmLimit != nil {
		if *input.RpmLimit < 0 {
			return nil, ErrSupplierInvalidContract
		}
		updates["rpm_limit"] = *input.RpmLimit
	}
	if input.TpmLimit != nil {
		if *input.TpmLimit < 0 {
			return nil, ErrSupplierInvalidContract
		}
		updates["tpm_limit"] = *input.TpmLimit
	}
	if input.MaxConcurrency != nil {
		if *input.MaxConcurrency < 0 {
			return nil, ErrSupplierInvalidContract
		}
		updates["max_concurrency"] = *input.MaxConcurrency
	}
	var contract SupplierContract
	if err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&contract, id).Error; err != nil {
			return err
		}
		if len(updates) > 0 {
			if err := tx.Model(&contract).Updates(updates).Error; err != nil {
				return err
			}
		}
		return tx.First(&contract, id).Error
	}); err != nil {
		return nil, err
	}
	refreshLocalChannelCacheAndPublishChanged()
	return &contract, nil
}

func InactivateSupplierContract(id int) error {
	if DB == nil {
		return fmt.Errorf("inactivate supplier contract: %w", ErrDatabase)
	}
	var preliminary SupplierContract
	if err := DB.Select("id", "supplier_id").First(&preliminary, id).Error; err != nil {
		return err
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var supplier UpstreamSupplier
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&supplier, preliminary.SupplierId).Error; err != nil {
			return err
		}
		var contract SupplierContract
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&contract, id).Error; err != nil {
			return err
		}
		if contract.SupplierId != supplier.Id {
			return ErrSupplierImmutableField
		}
		alreadyInactive := contract.Status == SupplierContractStatusInactive
		var bindingCount int64
		if err := tx.Model(&Channel{}).Where("supplier_contract_id = ?", id).Count(&bindingCount).Error; err != nil {
			return err
		}
		if bindingCount > 0 {
			return ErrSupplierContractBound
		}
		if alreadyInactive {
			return nil
		}
		updatedAt, err := getSupplierDBTimestamp(tx)
		if err != nil {
			return err
		}
		return tx.Model(&SupplierContract{}).Where("id = ? AND status = ?", id, SupplierContractStatusActive).UpdateColumns(map[string]any{
			"status":     SupplierContractStatusInactive,
			"updated_at": updatedAt,
		}).Error
	})
	if err != nil {
		return err
	}
	refreshLocalChannelCacheAndPublishChanged()
	return nil
}

func ListSupplierContractRateVersions(contractId int, page SupplierPage) ([]SupplierContractRateVersion, int64, error) {
	if DB == nil {
		return nil, 0, fmt.Errorf("list supplier contract rates: %w", ErrDatabase)
	}
	page = normalizeSupplierPage(page)
	query := DB.Model(&SupplierContractRateVersion{}).Where("contract_id = ?", contractId)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	versions := make([]SupplierContractRateVersion, 0)
	if err := query.Order("effective_at DESC, id DESC").Offset(page.Offset).Limit(page.Limit).Find(&versions).Error; err != nil {
		return nil, 0, err
	}
	return versions, total, nil
}

func CreateSupplierInventoryAdjustment(adjustment *SupplierInventoryAdjustment) (*SupplierInventoryAdjustment, error) {
	if DB == nil {
		return nil, fmt.Errorf("create supplier inventory adjustment: %w", ErrDatabase)
	}
	if adjustment == nil {
		return nil, ErrSupplierInvalidInventory
	}
	adjustment.Id = 0
	adjustment.CreatedAt = 0
	created := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		if _, _, _, err := lockActiveSupplierContractChain(tx, adjustment.ContractId, true); err != nil {
			return err
		}
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "contract_id"}, {Name: "idempotency_key"}},
			DoNothing: true,
		}).Create(adjustment)
		if result.Error != nil {
			return result.Error
		}
		created = result.RowsAffected == 1 && adjustment.Id > 0
		var existing SupplierInventoryAdjustment
		if err := tx.Where("contract_id = ? AND idempotency_key = ?", adjustment.ContractId, strings.TrimSpace(adjustment.IdempotencyKey)).First(&existing).Error; err != nil {
			return err
		}
		if existing.DeltaMicroUsd != adjustment.DeltaMicroUsd || existing.Type != strings.TrimSpace(adjustment.Type) || existing.Reason != strings.TrimSpace(adjustment.Reason) || existing.CreatedBy != adjustment.CreatedBy {
			return ErrSupplierIdempotencyConflict
		}
		*adjustment = existing
		return nil
	})
	if err != nil {
		return nil, err
	}
	if created {
		refreshLocalChannelCacheAndPublishChanged()
	}
	return adjustment, nil
}

func ListSupplierInventoryAdjustments(contractId int, page SupplierPage) ([]SupplierInventoryAdjustment, int64, error) {
	if DB == nil {
		return nil, 0, fmt.Errorf("list supplier inventory adjustments: %w", ErrDatabase)
	}
	page = normalizeSupplierPage(page)
	query := DB.Model(&SupplierInventoryAdjustment{}).Where("contract_id = ?", contractId)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	adjustments := make([]SupplierInventoryAdjustment, 0)
	if err := query.Order("id DESC").Offset(page.Offset).Limit(page.Limit).Find(&adjustments).Error; err != nil {
		return nil, 0, err
	}
	return adjustments, total, nil
}

func ListSupplierStatisticsExclusionRules(userId int, page SupplierPage) ([]SupplierStatisticsExclusionRule, int64, error) {
	if DB == nil {
		return nil, 0, fmt.Errorf("list supplier statistics exclusions: %w", ErrDatabase)
	}
	page = normalizeSupplierPage(page)
	query := DB.Model(&SupplierStatisticsExclusionRule{})
	if userId > 0 {
		query = query.Where("user_id = ?", userId)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rules []SupplierStatisticsExclusionRule
	if err := query.Order("effective_at DESC, id DESC").Offset(page.Offset).Limit(page.Limit).Find(&rules).Error; err != nil {
		return nil, 0, err
	}
	return rules, total, nil
}

func BindChannelSupplierContract(channelId int, contractId int) error {
	if DB == nil {
		return fmt.Errorf("bind channel supplier contract: %w", ErrDatabase)
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if _, _, _, err := lockActiveSupplierContractChain(tx, contractId, true); err != nil {
			return err
		}
		var channel Channel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "supplier_contract_id").First(&channel, channelId).Error; err != nil {
			return err
		}
		if channel.SupplierContractId != nil && *channel.SupplierContractId == contractId {
			return nil
		}
		if err := tx.Model(&Channel{}).Where("id = ?", channelId).UpdateColumn("supplier_contract_id", contractId).Error; err != nil {
			return err
		}
		return tx.Create(&SupplierChannelBindingVersion{
			ChannelId:                  channelId,
			PreviousSupplierContractId: channel.SupplierContractId,
			SupplierContractId:         &contractId,
		}).Error
	})
	if err != nil {
		return err
	}
	refreshLocalChannelCacheAndPublishChanged()
	return nil
}

func GetChannelSupplierContractBinding(channelId int) (*SupplierChannelBinding, error) {
	if DB == nil {
		return nil, fmt.Errorf("get channel supplier binding: %w", ErrDatabase)
	}
	var binding SupplierChannelBinding
	if err := DB.Model(&Channel{}).
		Select("id AS channel_id", "name AS channel_name", "status AS channel_status", "supplier_contract_id").
		Where("id = ?", channelId).
		First(&binding).Error; err != nil {
		return nil, err
	}
	return &binding, nil
}

func ListSupplierChannelBindings(contractId int, page SupplierPage) ([]SupplierChannelBinding, int64, error) {
	if DB == nil {
		return nil, 0, fmt.Errorf("list channel supplier bindings: %w", ErrDatabase)
	}
	page = normalizeSupplierPage(page)
	query := DB.Model(&Channel{})
	if contractId > 0 {
		query = query.Where("supplier_contract_id = ?", contractId)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var bindings []SupplierChannelBinding
	if err := query.
		Select("id AS channel_id", "name AS channel_name", "status AS channel_status", "supplier_contract_id").
		Order("id DESC").Offset(page.Offset).Limit(page.Limit).Find(&bindings).Error; err != nil {
		return nil, 0, err
	}
	return bindings, total, nil
}

func UnbindChannelSupplierContract(channelId int) error {
	if DB == nil {
		return fmt.Errorf("unbind channel supplier contract: %w", ErrDatabase)
	}
	var preliminary supplierChannelBindingRow
	if err := DB.Model(&Channel{}).Select("id", "supplier_contract_id").First(&preliminary, channelId).Error; err != nil {
		return err
	}
	if preliminary.SupplierContractId == nil {
		return nil
	}
	contractId := *preliminary.SupplierContractId
	var preliminaryContract SupplierContract
	if err := DB.Select("id", "supplier_id").First(&preliminaryContract, contractId).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if preliminaryContract.SupplierId > 0 {
			var supplier UpstreamSupplier
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&supplier, preliminaryContract.SupplierId).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			var contract SupplierContract
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&contract, contractId).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if contract.CurrentRateVersionId != nil {
				var rate SupplierContractRateVersion
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&rate, *contract.CurrentRateVersionId).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
			}
		}
		var channel Channel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "supplier_contract_id").First(&channel, channelId).Error; err != nil {
			return err
		}
		if channel.SupplierContractId == nil {
			return nil
		}
		if *channel.SupplierContractId != contractId {
			return ErrSupplierBindingChanged
		}
		if err := tx.Model(&Channel{}).Where("id = ? AND supplier_contract_id = ?", channelId, contractId).UpdateColumn("supplier_contract_id", nil).Error; err != nil {
			return err
		}
		return tx.Create(&SupplierChannelBindingVersion{
			ChannelId:                  channelId,
			PreviousSupplierContractId: &contractId,
			SupplierContractId:         nil,
		}).Error
	})
	if err != nil {
		return err
	}
	refreshLocalChannelCacheAndPublishChanged()
	return nil
}

func lockActiveSupplierContractChain(tx *gorm.DB, contractId int, requireCurrentRate bool) (SupplierContract, UpstreamSupplier, *SupplierContractRateVersion, error) {
	var preliminary SupplierContract
	if err := tx.Select("id", "supplier_id").First(&preliminary, contractId).Error; err != nil {
		return SupplierContract{}, UpstreamSupplier{}, nil, err
	}
	var supplier UpstreamSupplier
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&supplier, preliminary.SupplierId).Error; err != nil {
		return SupplierContract{}, UpstreamSupplier{}, nil, err
	}
	if supplier.Status != SupplierStatusActive {
		return SupplierContract{}, UpstreamSupplier{}, nil, ErrSupplierInactive
	}
	var contract SupplierContract
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&contract, contractId).Error; err != nil {
		return SupplierContract{}, UpstreamSupplier{}, nil, err
	}
	if contract.SupplierId != supplier.Id {
		return SupplierContract{}, UpstreamSupplier{}, nil, ErrSupplierImmutableField
	}
	if contract.Status != SupplierContractStatusActive {
		return SupplierContract{}, UpstreamSupplier{}, nil, ErrSupplierContractInactive
	}
	if contract.CurrentRateVersionId == nil {
		if requireCurrentRate {
			return SupplierContract{}, UpstreamSupplier{}, nil, ErrSupplierCurrentRateRequired
		}
		return contract, supplier, nil, nil
	}
	var rate SupplierContractRateVersion
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&rate, *contract.CurrentRateVersionId).Error; err != nil {
		return SupplierContract{}, UpstreamSupplier{}, nil, err
	}
	if rate.ContractId != contract.Id || rate.ProcurementMultiplierPpm < 0 || rate.ProcurementMultiplierPpm > 1_000_000 {
		return SupplierContract{}, UpstreamSupplier{}, nil, ErrSupplierInvalidRate
	}
	return contract, supplier, &rate, nil
}
