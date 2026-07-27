package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const SupplierAdminMaxPageSize = 100

type SupplierAdminListFilter struct {
	Page    SupplierPage
	Keyword string
}

type SupplierContractAdminListFilter struct {
	Page       SupplierPage
	SupplierId int
	Keyword    string
}

type SupplierChannelBindingAdminListFilter struct {
	Page          SupplierPage
	ContractId    int
	Keyword       string
	Bound         *bool
	ChannelStatus *int
}

type SupplierExclusionAdminListFilter struct {
	Page        SupplierPage
	UserId      int
	Action      string
	Keyword     string
	CurrentOnly bool
}

type SupplierAdminRow struct {
	Id                     int    `json:"id"`
	Name                   string `json:"name"`
	Status                 string `json:"status"`
	Remark                 string `json:"remark"`
	RowVersion             int64  `json:"row_version"`
	ContractCount          int64  `json:"contract_count"`
	ActiveContractCount    int64  `json:"active_contract_count"`
	LinkedChannelCount     int64  `json:"linked_channel_count"`
	InventoryTotalMicroUsd int64  `json:"inventory_total_micro_usd,string"`
	CreatedAt              int64  `json:"created_at"`
	UpdatedAt              int64  `json:"updated_at"`
}

type SupplierContractAdminRow struct {
	Id                              int    `json:"id"`
	SupplierId                      int    `json:"supplier_id"`
	SupplierName                    string `json:"supplier_name"`
	Name                            string `json:"name"`
	ContractNo                      string `json:"contract_no"`
	Remark                          string `json:"remark"`
	Status                          string `json:"status"`
	RowVersion                      int64  `json:"row_version"`
	CurrentRateVersionId            *int   `json:"current_rate_version_id"`
	CurrentProcurementMultiplierPpm *int64 `json:"current_procurement_multiplier_ppm"`
	CurrentRateEffectiveAt          *int64 `json:"current_rate_effective_at"`
	InventoryTotalMicroUsd          int64  `json:"inventory_total_micro_usd,string"`
	LinkedChannelCount              int64  `json:"linked_channel_count"`
	RpmLimit                        int64  `json:"rpm_limit"`
	TpmLimit                        int64  `json:"tpm_limit"`
	MaxConcurrency                  int    `json:"max_concurrency"`
	CreatedAt                       int64  `json:"created_at"`
	UpdatedAt                       int64  `json:"updated_at"`
}

type SupplierChannelBindingAdminRow struct {
	ChannelId                       int     `json:"channel_id"`
	ChannelName                     string  `json:"channel_name"`
	ChannelStatus                   int     `json:"channel_status"`
	SupplierContractId              *int    `json:"supplier_contract_id"`
	ContractName                    *string `json:"contract_name"`
	ContractNo                      *string `json:"contract_no"`
	SupplierId                      *int    `json:"supplier_id"`
	SupplierName                    *string `json:"supplier_name"`
	CurrentRateVersionId            *int    `json:"current_rate_version_id"`
	CurrentProcurementMultiplierPpm *int64  `json:"current_procurement_multiplier_ppm"`
}

type SupplierEffectiveExclusionRow struct {
	RuleId          int    `json:"rule_id"`
	UserId          int    `json:"user_id"`
	Username        string `json:"username"`
	DisplayName     string `json:"display_name"`
	Role            *int   `json:"role"`
	Status          *int   `json:"status"`
	IdentityPresent bool   `json:"identity_present"`
	Action          string `json:"action"`
	Excluded        bool   `json:"excluded"`
	EffectiveAt     int64  `json:"effective_at"`
	Reason          string `json:"reason"`
	CreatedBy       int    `json:"created_by"`
	CreatedAt       int64  `json:"created_at"`
}

func normalizeSupplierAdminFilter(page SupplierPage, keyword string) (SupplierPage, string) {
	page = normalizeSupplierPage(page)
	if page.Limit > SupplierAdminMaxPageSize {
		page.Limit = SupplierAdminMaxPageSize
	}
	return page, strings.TrimSpace(keyword)
}

func supplierAdminKeywordPattern(keyword string) string {
	return "%" + strings.ToLower(strings.TrimSpace(keyword)) + "%"
}

func ListSupplierAdminRows(filter SupplierAdminListFilter) ([]SupplierAdminRow, int64, error) {
	if DB == nil {
		return nil, 0, fmt.Errorf("list supplier admin rows: %w", ErrDatabase)
	}
	filter.Page, filter.Keyword = normalizeSupplierAdminFilter(filter.Page, filter.Keyword)
	baseQuery := func() *gorm.DB {
		query := DB.Model(&UpstreamSupplier{})
		if filter.Page.Status != "" {
			query = query.Where("status = ?", filter.Page.Status)
		}
		if filter.Keyword != "" {
			query = query.Where("LOWER(name) LIKE ?", supplierAdminKeywordPattern(filter.Keyword))
		}
		return query
	}
	var total int64
	if err := baseQuery().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var suppliers []UpstreamSupplier
	if err := baseQuery().Order("id DESC").Offset(filter.Page.Offset).Limit(filter.Page.Limit).Find(&suppliers).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]SupplierAdminRow, len(suppliers))
	ids := make([]int, len(suppliers))
	byId := make(map[int]int, len(suppliers))
	for index, supplier := range suppliers {
		ids[index] = supplier.Id
		byId[supplier.Id] = index
		rows[index] = SupplierAdminRow{Id: supplier.Id, Name: supplier.Name, Status: supplier.Status, Remark: supplier.Remark, RowVersion: supplier.RowVersion, CreatedAt: supplier.CreatedAt, UpdatedAt: supplier.UpdatedAt}
	}
	if len(ids) == 0 {
		return rows, total, nil
	}
	type contractAggregate struct {
		SupplierId          int
		ContractCount       int64
		ActiveContractCount int64
	}
	var contractAggregates []contractAggregate
	if err := DB.Model(&SupplierContract{}).
		Select("supplier_id, COUNT(*) AS contract_count, COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS active_contract_count", SupplierContractStatusActive).
		Where("supplier_id IN ?", ids).Group("supplier_id").Scan(&contractAggregates).Error; err != nil {
		return nil, 0, err
	}
	for _, aggregate := range contractAggregates {
		index := byId[aggregate.SupplierId]
		rows[index].ContractCount = aggregate.ContractCount
		rows[index].ActiveContractCount = aggregate.ActiveContractCount
	}
	type moneyAggregate struct {
		SupplierId             int
		InventoryTotalMicroUsd int64
	}
	var moneyAggregates []moneyAggregate
	if err := DB.Table("supplier_inventory_adjustments AS adjustment").
		Select("contract.supplier_id AS supplier_id, COALESCE(SUM(adjustment.delta_micro_usd), 0) AS inventory_total_micro_usd").
		Joins("JOIN supplier_contracts AS contract ON contract.id = adjustment.contract_id").
		Where("contract.supplier_id IN ?", ids).Group("contract.supplier_id").Scan(&moneyAggregates).Error; err != nil {
		return nil, 0, err
	}
	for _, aggregate := range moneyAggregates {
		rows[byId[aggregate.SupplierId]].InventoryTotalMicroUsd = aggregate.InventoryTotalMicroUsd
	}
	type bindingAggregate struct {
		SupplierId         int
		LinkedChannelCount int64
	}
	var bindingAggregates []bindingAggregate
	if err := DB.Table("channels AS channel").
		Select("contract.supplier_id AS supplier_id, COUNT(*) AS linked_channel_count").
		Joins("JOIN supplier_contracts AS contract ON contract.id = channel.supplier_contract_id").
		Where("contract.supplier_id IN ?", ids).Group("contract.supplier_id").Scan(&bindingAggregates).Error; err != nil {
		return nil, 0, err
	}
	for _, aggregate := range bindingAggregates {
		rows[byId[aggregate.SupplierId]].LinkedChannelCount = aggregate.LinkedChannelCount
	}
	return rows, total, nil
}

func supplierContractAdminBaseQuery(filter SupplierContractAdminListFilter) *gorm.DB {
	query := DB.Table("supplier_contracts AS contract").Joins("JOIN upstream_suppliers AS supplier ON supplier.id = contract.supplier_id")
	if filter.SupplierId > 0 {
		query = query.Where("contract.supplier_id = ?", filter.SupplierId)
	}
	if filter.Page.Status != "" {
		query = query.Where("contract.status = ?", filter.Page.Status)
	}
	if filter.Keyword != "" {
		pattern := supplierAdminKeywordPattern(filter.Keyword)
		query = query.Where("LOWER(contract.name) LIKE ? OR LOWER(contract.contract_no) LIKE ? OR LOWER(supplier.name) LIKE ?", pattern, pattern, pattern)
	}
	return query
}

func ListSupplierContractAdminRows(filter SupplierContractAdminListFilter) ([]SupplierContractAdminRow, int64, error) {
	if DB == nil {
		return nil, 0, fmt.Errorf("list supplier contract admin rows: %w", ErrDatabase)
	}
	filter.Page, filter.Keyword = normalizeSupplierAdminFilter(filter.Page, filter.Keyword)
	var total int64
	if err := supplierContractAdminBaseQuery(filter).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]SupplierContractAdminRow, 0)
	if err := supplierContractAdminBaseQuery(filter).
		Select(`contract.id, contract.supplier_id, supplier.name AS supplier_name, contract.name, contract.contract_no,
			contract.remark, contract.status, contract.row_version, contract.current_rate_version_id,
			rate.procurement_multiplier_ppm AS current_procurement_multiplier_ppm,
			rate.effective_at AS current_rate_effective_at,
			contract.rpm_limit, contract.tpm_limit, contract.max_concurrency, contract.created_at, contract.updated_at`).
		Joins("LEFT JOIN supplier_contract_rate_versions AS rate ON rate.id = contract.current_rate_version_id").
		Order("contract.id DESC").Offset(filter.Page.Offset).Limit(filter.Page.Limit).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	ids := make([]int, len(rows))
	byId := make(map[int]int, len(rows))
	for index := range rows {
		ids[index] = rows[index].Id
		byId[rows[index].Id] = index
	}
	if len(ids) == 0 {
		return rows, total, nil
	}
	type moneyAggregate struct {
		ContractId             int
		InventoryTotalMicroUsd int64
	}
	var moneyAggregates []moneyAggregate
	if err := DB.Model(&SupplierInventoryAdjustment{}).
		Select("contract_id, COALESCE(SUM(delta_micro_usd), 0) AS inventory_total_micro_usd").
		Where("contract_id IN ?", ids).Group("contract_id").Scan(&moneyAggregates).Error; err != nil {
		return nil, 0, err
	}
	for _, aggregate := range moneyAggregates {
		rows[byId[aggregate.ContractId]].InventoryTotalMicroUsd = aggregate.InventoryTotalMicroUsd
	}
	type bindingAggregate struct {
		ContractId         int
		LinkedChannelCount int64
	}
	var bindingAggregates []bindingAggregate
	if err := DB.Model(&Channel{}).
		Select("supplier_contract_id AS contract_id, COUNT(*) AS linked_channel_count").
		Where("supplier_contract_id IN ?", ids).Group("supplier_contract_id").Scan(&bindingAggregates).Error; err != nil {
		return nil, 0, err
	}
	for _, aggregate := range bindingAggregates {
		rows[byId[aggregate.ContractId]].LinkedChannelCount = aggregate.LinkedChannelCount
	}
	return rows, total, nil
}

func supplierChannelBindingAdminBaseQuery(filter SupplierChannelBindingAdminListFilter) *gorm.DB {
	query := DB.Table("channels AS channel").
		Joins("LEFT JOIN supplier_contracts AS contract ON contract.id = channel.supplier_contract_id").
		Joins("LEFT JOIN upstream_suppliers AS supplier ON supplier.id = contract.supplier_id")
	if filter.ContractId > 0 {
		query = query.Where("channel.supplier_contract_id = ?", filter.ContractId)
	}
	if filter.Bound != nil {
		if *filter.Bound {
			query = query.Where("channel.supplier_contract_id IS NOT NULL")
		} else {
			query = query.Where("channel.supplier_contract_id IS NULL")
		}
	}
	if filter.ChannelStatus != nil {
		query = query.Where("channel.status = ?", *filter.ChannelStatus)
	}
	if filter.Keyword != "" {
		pattern := supplierAdminKeywordPattern(filter.Keyword)
		query = query.Where("LOWER(channel.name) LIKE ? OR LOWER(contract.name) LIKE ? OR LOWER(contract.contract_no) LIKE ? OR LOWER(supplier.name) LIKE ?", pattern, pattern, pattern, pattern)
	}
	return query
}

func ListSupplierChannelBindingAdminRows(filter SupplierChannelBindingAdminListFilter) ([]SupplierChannelBindingAdminRow, int64, error) {
	if DB == nil {
		return nil, 0, fmt.Errorf("list supplier channel binding admin rows: %w", ErrDatabase)
	}
	filter.Page, filter.Keyword = normalizeSupplierAdminFilter(filter.Page, filter.Keyword)
	var total int64
	if err := supplierChannelBindingAdminBaseQuery(filter).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]SupplierChannelBindingAdminRow, 0)
	if err := supplierChannelBindingAdminBaseQuery(filter).
		Select(`channel.id AS channel_id, channel.name AS channel_name, channel.status AS channel_status,
			channel.supplier_contract_id, contract.name AS contract_name, contract.contract_no, contract.supplier_id,
			supplier.name AS supplier_name, contract.current_rate_version_id,
			rate.procurement_multiplier_ppm AS current_procurement_multiplier_ppm`).
		Joins("LEFT JOIN supplier_contract_rate_versions AS rate ON rate.id = contract.current_rate_version_id").
		Order("channel.id DESC").Offset(filter.Page.Offset).Limit(filter.Page.Limit).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func supplierExclusionHistoryBaseQuery(filter SupplierExclusionAdminListFilter) *gorm.DB {
	query := DB.Table("supplier_statistics_exclusion_rules AS rule").
		Joins("LEFT JOIN users AS user_identity ON user_identity.id = rule.user_id AND user_identity.deleted_at IS NULL")
	if filter.UserId > 0 {
		query = query.Where("rule.user_id = ?", filter.UserId)
	}
	if filter.Action != "" {
		query = query.Where("rule.action = ?", filter.Action)
	}
	if filter.Keyword != "" {
		pattern := supplierAdminKeywordPattern(filter.Keyword)
		if userId, err := strconv.Atoi(filter.Keyword); err == nil {
			query = query.Where("rule.user_id = ? OR LOWER(user_identity.username) LIKE ? OR LOWER(user_identity.display_name) LIKE ?", userId, pattern, pattern)
		} else {
			query = query.Where("LOWER(user_identity.username) LIKE ? OR LOWER(user_identity.display_name) LIKE ?", pattern, pattern)
		}
	}
	return query
}

func ListSupplierExclusionHistory(filter SupplierExclusionAdminListFilter) ([]SupplierStatisticsExclusionRule, int64, error) {
	if DB == nil {
		return nil, 0, fmt.Errorf("list supplier exclusion history: %w", ErrDatabase)
	}
	filter.Page, filter.Keyword = normalizeSupplierAdminFilter(filter.Page, filter.Keyword)
	var total int64
	if err := supplierExclusionHistoryBaseQuery(filter).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]SupplierStatisticsExclusionRule, 0)
	if err := supplierExclusionHistoryBaseQuery(filter).Select("rule.*").
		Order("rule.effective_at DESC, rule.id DESC").Offset(filter.Page.Offset).Limit(filter.Page.Limit).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func supplierEffectiveExclusionBaseQuery(filter SupplierExclusionAdminListFilter) *gorm.DB {
	query := DB.Table("supplier_statistics_exclusion_rules AS rule").
		Joins(`LEFT JOIN supplier_statistics_exclusion_rules AS newer
			ON newer.user_id = rule.user_id
			AND (newer.effective_at > rule.effective_at OR (newer.effective_at = rule.effective_at AND newer.id > rule.id))`).
		Joins("LEFT JOIN users AS user_identity ON user_identity.id = rule.user_id AND user_identity.deleted_at IS NULL").
		Where("newer.id IS NULL")
	if filter.UserId > 0 {
		query = query.Where("rule.user_id = ?", filter.UserId)
	}
	if filter.Action != "" {
		query = query.Where("rule.action = ?", filter.Action)
	}
	if filter.Keyword != "" {
		pattern := supplierAdminKeywordPattern(filter.Keyword)
		if userId, err := strconv.Atoi(filter.Keyword); err == nil {
			query = query.Where("rule.user_id = ? OR LOWER(user_identity.username) LIKE ? OR LOWER(user_identity.display_name) LIKE ?", userId, pattern, pattern)
		} else {
			query = query.Where("LOWER(user_identity.username) LIKE ? OR LOWER(user_identity.display_name) LIKE ?", pattern, pattern)
		}
	}
	return query
}

func ListSupplierEffectiveExclusions(filter SupplierExclusionAdminListFilter) ([]SupplierEffectiveExclusionRow, int64, error) {
	if DB == nil {
		return nil, 0, fmt.Errorf("list supplier effective exclusions: %w", ErrDatabase)
	}
	filter.Page, filter.Keyword = normalizeSupplierAdminFilter(filter.Page, filter.Keyword)
	var total int64
	if err := supplierEffectiveExclusionBaseQuery(filter).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]SupplierEffectiveExclusionRow, 0)
	if err := supplierEffectiveExclusionBaseQuery(filter).
		Select(`rule.id AS rule_id, rule.user_id, COALESCE(user_identity.username, '') AS username,
			COALESCE(user_identity.display_name, '') AS display_name, user_identity.role, user_identity.status,
			CASE WHEN user_identity.id IS NULL THEN 0 ELSE 1 END AS identity_present, rule.action,
			CASE WHEN rule.action = ? THEN 1 ELSE 0 END AS excluded,
			rule.effective_at, rule.reason, rule.created_by, rule.created_at`, SupplierStatisticsActionExclude).
		Order("rule.id DESC").Offset(filter.Page.Offset).Limit(filter.Page.Limit).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func SetChannelSupplierContractCAS(channelId int, expectedContractId int, desiredContractId *int) error {
	return SetChannelSupplierContractCASForActor(channelId, expectedContractId, desiredContractId, 0)
}

func SetChannelSupplierContractCASForActor(channelId int, expectedContractId int, desiredContractId *int, createdBy int) error {
	if DB == nil {
		return fmt.Errorf("set channel supplier contract CAS: %w", ErrDatabase)
	}
	if channelId <= 0 || expectedContractId < 0 || createdBy < 0 || (desiredContractId != nil && *desiredContractId <= 0) {
		return ErrSupplierInvalidContract
	}
	changed := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		changed, err = setChannelSupplierContractCASTx(tx, channelId, expectedContractId, desiredContractId, createdBy)
		return err
	})
	if err != nil {
		return err
	}
	if changed {
		refreshLocalChannelCacheAndPublishChanged()
	}
	return nil
}

func setChannelSupplierContractCASTx(tx *gorm.DB, channelId int, expectedContractId int, desiredContractId *int, createdBy int) (bool, error) {
	if tx == nil {
		return false, ErrDatabase
	}
	changed := false
	err := func() error {
		if desiredContractId != nil {
			if _, _, _, err := lockActiveSupplierContractChain(tx, *desiredContractId, true); err != nil {
				return err
			}
		} else if expectedContractId > 0 {
			if err := lockSupplierContractChainForRelease(tx, expectedContractId); err != nil {
				return err
			}
		}
		var channel Channel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "supplier_contract_id").First(&channel, channelId).Error; err != nil {
			return err
		}
		if !supplierContractIdMatchesExpected(channel.SupplierContractId, expectedContractId) {
			return ErrSupplierBindingChanged
		}
		if desiredContractId != nil && channel.SupplierContractId != nil && *channel.SupplierContractId == *desiredContractId {
			return nil
		}
		if desiredContractId == nil && channel.SupplierContractId == nil {
			return nil
		}
		query := tx.Model(&Channel{}).Where("id = ?", channelId)
		if expectedContractId == 0 {
			query = query.Where("supplier_contract_id IS NULL")
		} else {
			query = query.Where("supplier_contract_id = ?", expectedContractId)
		}
		result := query.UpdateColumn("supplier_contract_id", desiredContractId)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrSupplierBindingChanged
		}
		version := SupplierChannelBindingVersion{
			ChannelId:                  channelId,
			PreviousSupplierContractId: channel.SupplierContractId,
			SupplierContractId:         desiredContractId,
			CreatedBy:                  createdBy,
		}
		if err := tx.Create(&version).Error; err != nil {
			return err
		}
		changed = true
		return nil
	}()
	return changed, err
}

func ListSupplierChannelBindingVersions(channelId int, page SupplierPage) ([]SupplierChannelBindingVersion, int64, error) {
	if DB == nil {
		return nil, 0, fmt.Errorf("list supplier channel binding versions: %w", ErrDatabase)
	}
	if channelId <= 0 {
		return nil, 0, ErrSupplierInvalidContract
	}
	page = normalizeSupplierPage(page)
	query := DB.Model(&SupplierChannelBindingVersion{}).Where("channel_id = ?", channelId)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	versions := make([]SupplierChannelBindingVersion, 0)
	if err := query.Order("effective_at DESC, id DESC").Offset(page.Offset).Limit(page.Limit).Find(&versions).Error; err != nil {
		return nil, 0, err
	}
	return versions, total, nil
}

func supplierContractIdMatchesExpected(actual *int, expected int) bool {
	if expected == 0 {
		return actual == nil
	}
	return actual != nil && *actual == expected
}

func lockSupplierContractChainForRelease(tx *gorm.DB, contractId int) error {
	var preliminary SupplierContract
	if err := tx.Select("id", "supplier_id").First(&preliminary, contractId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	var supplier UpstreamSupplier
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&supplier, preliminary.SupplierId).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	var contract SupplierContract
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&contract, contractId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if contract.CurrentRateVersionId != nil {
		var rate SupplierContractRateVersion
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&rate, *contract.CurrentRateVersionId).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	return nil
}
