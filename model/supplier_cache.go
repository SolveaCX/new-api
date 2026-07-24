package model

import (
	"fmt"
	"sort"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"

	"gorm.io/gorm"
)

const supplierCacheLoadBatchSize = 500
const supplierCacheMaxReportedIssues = 100

type supplierRuntimeIndex struct {
	channelCosts       map[int]types.SupplierCostSnapshot
	excludedUserRuleId map[int]int
}

var supplierRuntimeIndexPointer atomic.Pointer[supplierRuntimeIndex]
var supplierCacheHealthPointer atomic.Pointer[SupplierCacheHealth]

type SupplierCacheIssue struct {
	ChannelId  int    `json:"channel_id"`
	ContractId int    `json:"contract_id"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

type SupplierCacheHealth struct {
	Blocking            bool                 `json:"blocking"`
	LoadedAt            int64                `json:"loaded_at"`
	IllegalBindingCount int                  `json:"illegal_binding_count"`
	RefreshError        string               `json:"refresh_error,omitempty"`
	Issues              []SupplierCacheIssue `json:"issues"`
}

func emptySupplierRuntimeIndex() *supplierRuntimeIndex {
	return &supplierRuntimeIndex{
		channelCosts:       map[int]types.SupplierCostSnapshot{},
		excludedUserRuleId: map[int]int{},
	}
}

func currentSupplierRuntimeIndex() *supplierRuntimeIndex {
	index := supplierRuntimeIndexPointer.Load()
	if index == nil {
		return emptySupplierRuntimeIndex()
	}
	return index
}

// RefreshSupplierCache builds a complete immutable index and publishes it with
// one atomic pointer swap. On any query or consistency error the previous index
// remains authoritative.
func RefreshSupplierCache() error {
	index, health, err := loadSupplierRuntimeIndex(DB)
	if err != nil {
		previous := GetSupplierCacheHealth()
		previous.Blocking = true
		previous.RefreshError = err.Error()
		supplierCacheHealthPointer.Store(&previous)
		return err
	}
	if health.Blocking {
		err = fmt.Errorf("supplier cache integrity check failed: %d illegal binding(s)", health.IllegalBindingCount)
		health.RefreshError = err.Error()
		supplierCacheHealthPointer.Store(health)
		return err
	}
	supplierRuntimeIndexPointer.Store(index)
	supplierCacheHealthPointer.Store(health)
	return nil
}

func SyncSupplierCache(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		if err := RefreshSupplierCache(); err != nil {
			common.SysError("supplier cache refresh failed; retaining previous snapshot: " + err.Error())
		}
	}
}

func GetSupplierCostSnapshot(channelId int) (types.SupplierCostSnapshot, bool) {
	snapshot, ok := currentSupplierRuntimeIndex().channelCosts[channelId]
	return snapshot, ok
}

func GetSupplierStatisticsScopeSnapshot(userId int) types.SupplierStatisticsScopeSnapshot {
	if ruleId, ok := currentSupplierRuntimeIndex().excludedUserRuleId[userId]; ok {
		return types.SupplierStatisticsScopeSnapshot{
			Scope:           types.SupplierStatisticsScopeInternal,
			ExclusionRuleId: ruleId,
		}
	}
	return types.BusinessSupplierStatisticsScopeSnapshot()
}

func GetSupplierCacheHealth() SupplierCacheHealth {
	health := supplierCacheHealthPointer.Load()
	if health == nil {
		return SupplierCacheHealth{Blocking: true, RefreshError: "supplier cache has not completed an initial load"}
	}
	copyHealth := *health
	copyHealth.Issues = append([]SupplierCacheIssue(nil), health.Issues...)
	return copyHealth
}

// IsSupplierCacheBlocking is the request-path health check. It is an O(1)
// atomic read and fails closed until the first successful refresh.
func IsSupplierCacheBlocking() bool {
	health := supplierCacheHealthPointer.Load()
	return health == nil || health.Blocking
}

type supplierChannelBindingRow struct {
	Id                 int
	SupplierContractId *int
	BindingVersionId   int
}

func loadSupplierRuntimeIndex(db *gorm.DB) (*supplierRuntimeIndex, *SupplierCacheHealth, error) {
	if db == nil {
		return nil, nil, fmt.Errorf("load supplier cache: %w", ErrDatabase)
	}

	bindings, contractIds, err := loadSupplierChannelBindings(db)
	if err != nil {
		return nil, nil, err
	}
	contracts, err := loadSupplierContractsByIds(db, contractIds)
	if err != nil {
		return nil, nil, err
	}
	suppliers, err := loadUpstreamSuppliersByContracts(db, contracts)
	if err != nil {
		return nil, nil, err
	}
	rates, err := loadSupplierRatesByContracts(db, contracts)
	if err != nil {
		return nil, nil, err
	}
	excludedUsers, err := loadLatestSupplierStatisticsExclusions(db)
	if err != nil {
		return nil, nil, err
	}

	health := &SupplierCacheHealth{LoadedAt: time.Now().UTC().Unix(), Issues: make([]SupplierCacheIssue, 0)}
	reportIllegalBinding := func(binding supplierChannelBindingRow, code string, message string) {
		health.Blocking = true
		health.IllegalBindingCount++
		if len(health.Issues) < supplierCacheMaxReportedIssues {
			contractId := 0
			if binding.SupplierContractId != nil {
				contractId = *binding.SupplierContractId
			}
			health.Issues = append(health.Issues, SupplierCacheIssue{ChannelId: binding.Id, ContractId: contractId, Code: code, Message: message})
		}
	}
	channelCosts := make(map[int]types.SupplierCostSnapshot, len(bindings))
	for _, binding := range bindings {
		if binding.SupplierContractId == nil || *binding.SupplierContractId <= 0 {
			continue
		}
		if binding.BindingVersionId <= 0 {
			reportIllegalBinding(binding, "binding_version_missing", "current channel binding has no matching append-only version")
			continue
		}
		contract, ok := contracts[*binding.SupplierContractId]
		if !ok {
			reportIllegalBinding(binding, "contract_not_found", fmt.Sprintf("contract %d does not exist", *binding.SupplierContractId))
			continue
		}
		if contract.Status != SupplierContractStatusActive {
			reportIllegalBinding(binding, "contract_inactive", fmt.Sprintf("contract %d is not active", contract.Id))
			continue
		}
		if contract.CurrentRateVersionId == nil || *contract.CurrentRateVersionId <= 0 {
			reportIllegalBinding(binding, "current_rate_missing", fmt.Sprintf("contract %d has no current rate", contract.Id))
			continue
		}
		supplier, ok := suppliers[contract.SupplierId]
		if !ok {
			reportIllegalBinding(binding, "supplier_not_found", fmt.Sprintf("supplier %d does not exist", contract.SupplierId))
			continue
		}
		if supplier.Status != SupplierStatusActive {
			reportIllegalBinding(binding, "supplier_inactive", fmt.Sprintf("supplier %d is not active", supplier.Id))
			continue
		}
		rate, ok := rates[*contract.CurrentRateVersionId]
		if !ok {
			reportIllegalBinding(binding, "current_rate_not_found", fmt.Sprintf("rate version %d does not exist", *contract.CurrentRateVersionId))
			continue
		}
		if rate.ContractId != contract.Id {
			reportIllegalBinding(binding, "current_rate_wrong_contract", fmt.Sprintf("rate version %d does not belong to contract %d", rate.Id, contract.Id))
			continue
		}
		if rate.ProcurementMultiplierPpm < 0 || rate.ProcurementMultiplierPpm > 1_000_000 {
			reportIllegalBinding(binding, "current_rate_invalid", fmt.Sprintf("rate version %d has an invalid multiplier", rate.Id))
			continue
		}
		channelCosts[binding.Id] = types.SupplierCostSnapshot{
			BindingVersionId:         binding.BindingVersionId,
			SupplierId:               supplier.Id,
			SupplierName:             supplier.Name,
			ContractId:               contract.Id,
			ContractName:             contract.Name,
			RateVersionId:            rate.Id,
			ProcurementMultiplierPpm: rate.ProcurementMultiplierPpm,
		}
	}

	return &supplierRuntimeIndex{
		channelCosts:       channelCosts,
		excludedUserRuleId: excludedUsers,
	}, health, nil
}

func loadSupplierChannelBindings(db *gorm.DB) ([]supplierChannelBindingRow, []int, error) {
	bindings := make([]supplierChannelBindingRow, 0)
	contractIdSet := make(map[int]struct{})
	lastId := 0
	for {
		var batch []supplierChannelBindingRow
		err := db.Model(&Channel{}).
			Select("id", "supplier_contract_id").
			Where("id > ? AND supplier_contract_id IS NOT NULL", lastId).
			Order("id ASC").
			Limit(supplierCacheLoadBatchSize).
			Find(&batch).Error
		if err != nil {
			return nil, nil, fmt.Errorf("load supplier channel bindings: %w", err)
		}
		if len(batch) == 0 {
			break
		}
		for _, binding := range batch {
			bindings = append(bindings, binding)
			if binding.SupplierContractId != nil && *binding.SupplierContractId > 0 {
				contractIdSet[*binding.SupplierContractId] = struct{}{}
			}
		}
		lastId = batch[len(batch)-1].Id
	}
	if err := attachLatestSupplierChannelBindingVersions(db, bindings); err != nil {
		return nil, nil, err
	}
	return bindings, sortedSupplierIds(contractIdSet), nil
}

func attachLatestSupplierChannelBindingVersions(db *gorm.DB, bindings []supplierChannelBindingRow) error {
	for start := 0; start < len(bindings); start += supplierCacheLoadBatchSize {
		end := min(start+supplierCacheLoadBatchSize, len(bindings))
		channelIds := make([]int, 0, end-start)
		bindingIndexes := make(map[int]int, end-start)
		for index := start; index < end; index++ {
			channelIds = append(channelIds, bindings[index].Id)
			bindingIndexes[bindings[index].Id] = index
		}
		var versions []SupplierChannelBindingVersion
		if err := db.Table("supplier_channel_binding_versions AS version").
			Select("version.*").
			Joins(`LEFT JOIN supplier_channel_binding_versions AS newer
				ON newer.channel_id = version.channel_id
				AND (newer.effective_at > version.effective_at
					OR (newer.effective_at = version.effective_at AND newer.id > version.id))`).
			Where("version.channel_id IN ? AND newer.id IS NULL", channelIds).
			Scan(&versions).Error; err != nil {
			return fmt.Errorf("load latest supplier channel binding versions: %w", err)
		}
		for _, version := range versions {
			index, ok := bindingIndexes[version.ChannelId]
			if ok && supplierContractIdsEqual(bindings[index].SupplierContractId, version.SupplierContractId) {
				bindings[index].BindingVersionId = version.Id
			}
		}
	}
	return nil
}

func loadSupplierContractsByIds(db *gorm.DB, ids []int) (map[int]SupplierContract, error) {
	contracts := make(map[int]SupplierContract, len(ids))
	for start := 0; start < len(ids); start += supplierCacheLoadBatchSize {
		end := min(start+supplierCacheLoadBatchSize, len(ids))
		var batch []SupplierContract
		if err := db.Where("id IN ?", ids[start:end]).Find(&batch).Error; err != nil {
			return nil, fmt.Errorf("load supplier contracts: %w", err)
		}
		for _, contract := range batch {
			contracts[contract.Id] = contract
		}
	}
	return contracts, nil
}

func loadUpstreamSuppliersByContracts(db *gorm.DB, contracts map[int]SupplierContract) (map[int]UpstreamSupplier, error) {
	idSet := make(map[int]struct{}, len(contracts))
	for _, contract := range contracts {
		idSet[contract.SupplierId] = struct{}{}
	}
	ids := sortedSupplierIds(idSet)
	suppliers := make(map[int]UpstreamSupplier, len(ids))
	for start := 0; start < len(ids); start += supplierCacheLoadBatchSize {
		end := min(start+supplierCacheLoadBatchSize, len(ids))
		var batch []UpstreamSupplier
		if err := db.Where("id IN ?", ids[start:end]).Find(&batch).Error; err != nil {
			return nil, fmt.Errorf("load upstream suppliers: %w", err)
		}
		for _, supplier := range batch {
			suppliers[supplier.Id] = supplier
		}
	}
	return suppliers, nil
}

func loadSupplierRatesByContracts(db *gorm.DB, contracts map[int]SupplierContract) (map[int]SupplierContractRateVersion, error) {
	idSet := make(map[int]struct{}, len(contracts))
	for _, contract := range contracts {
		if contract.CurrentRateVersionId != nil && *contract.CurrentRateVersionId > 0 {
			idSet[*contract.CurrentRateVersionId] = struct{}{}
		}
	}
	ids := sortedSupplierIds(idSet)
	rates := make(map[int]SupplierContractRateVersion, len(ids))
	for start := 0; start < len(ids); start += supplierCacheLoadBatchSize {
		end := min(start+supplierCacheLoadBatchSize, len(ids))
		var batch []SupplierContractRateVersion
		if err := db.Where("id IN ?", ids[start:end]).Find(&batch).Error; err != nil {
			return nil, fmt.Errorf("load supplier rate versions: %w", err)
		}
		for _, rate := range batch {
			rates[rate.Id] = rate
		}
	}
	return rates, nil
}

func loadLatestSupplierStatisticsExclusions(db *gorm.DB) (map[int]int, error) {
	latestByUser := make(map[int]SupplierStatisticsExclusionRule)
	lastId := 0
	for {
		var batch []SupplierStatisticsExclusionRule
		err := db.Where("id > ?", lastId).
			Order("id ASC").
			Limit(supplierCacheLoadBatchSize).
			Find(&batch).Error
		if err != nil {
			return nil, fmt.Errorf("load supplier statistics exclusions: %w", err)
		}
		if len(batch) == 0 {
			break
		}
		for _, rule := range batch {
			current, ok := latestByUser[rule.UserId]
			if !ok || rule.EffectiveAt > current.EffectiveAt || (rule.EffectiveAt == current.EffectiveAt && rule.Id > current.Id) {
				latestByUser[rule.UserId] = rule
			}
		}
		lastId = batch[len(batch)-1].Id
	}

	excludedUsers := make(map[int]int, len(latestByUser))
	for userId, rule := range latestByUser {
		if rule.Action == SupplierStatisticsActionExclude {
			excludedUsers[userId] = rule.Id
		}
	}
	return excludedUsers, nil
}

func sortedSupplierIds(idSet map[int]struct{}) []int {
	ids := make([]int, 0, len(idSet))
	for id := range idSet {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	return ids
}
