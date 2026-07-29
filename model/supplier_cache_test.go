package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSupplierCacheTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	originalDB := DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalUsingSQLite := common.UsingSQLite
	originalGeneration := supplierRuntimeGenerationPointer.Load()
	originalPolicyState := supplierAccountingPolicyStatePointer.Load()
	common.OptionMapRWMutex.Lock()
	originalPolicyActivation, hadPolicyActivation := common.OptionMap[OptionKeySupplierSkipInternalAccountingActive]
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMap[OptionKeySupplierSkipInternalAccountingActive] = "false"
	common.OptionMapRWMutex.Unlock()
	RefreshSupplierAccountingPolicyCapability()
	t.Cleanup(func() {
		DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.UsingSQLite = originalUsingSQLite
		supplierRuntimeGenerationPointer.Store(originalGeneration)
		supplierAccountingPolicyStatePointer.Store(originalPolicyState)
		common.OptionMapRWMutex.Lock()
		if hadPolicyActivation {
			common.OptionMap[OptionKeySupplierSkipInternalAccountingActive] = originalPolicyActivation
		} else {
			delete(common.OptionMap, OptionKeySupplierSkipInternalAccountingActive)
		}
		common.OptionMapRWMutex.Unlock()
	})

	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&Option{},
		&Channel{},
		&Ability{},
		&UpstreamSupplier{},
		&SupplierContract{},
		&SupplierContractRateVersion{},
		&SupplierChannelBindingVersion{},
		&SupplierStatisticsExclusionRule{},
	))
	DB = db
	common.UsingSQLite = true
	common.MemoryCacheEnabled = false
	supplierRuntimeGenerationPointer.Store(nil)
	return db
}

func createSupplierCacheFixture(t *testing.T, db *gorm.DB) (SupplierContract, SupplierContractRateVersion, []Channel) {
	t.Helper()
	supplier := UpstreamSupplier{Name: "cache supplier"}
	require.NoError(t, db.Create(&supplier).Error)
	contract := SupplierContract{SupplierId: supplier.Id, Name: "cache contract", ContractNo: "cache-001"}
	require.NoError(t, db.Create(&contract).Error)
	rate := SupplierContractRateVersion{
		ContractId:               contract.Id,
		ProcurementMultiplierPpm: 650_000,
		CreatedBy:                1,
	}
	require.NoError(t, db.Create(&rate).Error)
	require.NoError(t, db.Model(&SupplierContract{}).Where("id = ?", contract.Id).UpdateColumn("current_rate_version_id", rate.Id).Error)
	contract.CurrentRateVersionId = &rate.Id

	channels := []Channel{
		{Name: "bound one", Key: "key-1", Status: common.ChannelStatusEnabled},
		{Name: "bound two", Key: "key-2", Status: common.ChannelStatusEnabled},
		{Name: "unbound", Key: "key-3", Status: common.ChannelStatusEnabled},
	}
	for i := range channels {
		require.NoError(t, db.Create(&channels[i]).Error)
	}
	for i := range channels[:2] {
		require.NoError(t, SetChannelSupplierContractCASForActor(channels[i].Id, 0, &contract.Id, 1))
	}
	return contract, rate, channels
}

func TestSupplierCacheMultipleChannelsUnboundAndLatestExclusion(t *testing.T) {
	db := setupSupplierCacheTestDB(t, "supplier-cache-bindings")
	require.NoError(t, SetSupplierSkipInternalAccountingActive(true))
	contract, rate, channels := createSupplierCacheFixture(t, db)
	require.NoError(t, SetChannelSupplierContractPolicyCASForActor(channels[0].Id, contract.Id, false, &contract.Id, true, 2))
	rules := []SupplierStatisticsExclusionRule{
		{UserId: 101, Action: SupplierStatisticsActionExclude, IdempotencyKey: "101-exclude", CreatedBy: 1},
		{UserId: 101, Action: SupplierStatisticsActionInclude, IdempotencyKey: "101-include", CreatedBy: 1},
		{UserId: 102, Action: SupplierStatisticsActionInclude, IdempotencyKey: "102-include", CreatedBy: 1},
		{UserId: 102, Action: SupplierStatisticsActionExclude, IdempotencyKey: "102-exclude", CreatedBy: 1},
		{UserId: 103, Action: SupplierStatisticsActionExclude, IdempotencyKey: "103-exclude", CreatedBy: 1},
		{UserId: 103, Action: SupplierStatisticsActionInclude, IdempotencyKey: "103-include", CreatedBy: 1},
	}
	for i := range rules {
		require.NoError(t, db.Create(&rules[i]).Error)
	}

	require.NoError(t, RefreshSupplierCache())
	require.False(t, GetSupplierCacheHealth().Blocking)
	for index, channel := range channels[:2] {
		snapshot, ok := GetSupplierCostSnapshot(channel.Id)
		require.True(t, ok)
		var bindingVersion SupplierChannelBindingVersion
		require.NoError(t, db.Where("channel_id = ?", channel.Id).Order("id DESC").First(&bindingVersion).Error)
		require.Equal(t, bindingVersion.Id, snapshot.BindingVersionId)
		require.Equal(t, contract.Id, snapshot.ContractId)
		require.Equal(t, rate.Id, snapshot.RateVersionId)
		require.Equal(t, int64(650_000), snapshot.ProcurementMultiplierPpm)
		require.Equal(t, index == 0, snapshot.SkipInternalAccounting)
	}
	_, ok := GetSupplierCostSnapshot(channels[2].Id)
	require.False(t, ok)

	require.Equal(t, "business", string(GetSupplierStatisticsScopeSnapshot(101).Scope))
	user102 := GetSupplierStatisticsScopeSnapshot(102)
	require.Equal(t, "internal", string(user102.Scope))
	require.Equal(t, rules[3].Id, user102.ExclusionRuleId)
	require.Equal(t, "business", string(GetSupplierStatisticsScopeSnapshot(103).Scope))
}

func TestSupplierCacheRefreshIsImmutableAndFailureRetainsPreviousIndex(t *testing.T) {
	db := setupSupplierCacheTestDB(t, "supplier-cache-refresh")
	require.True(t, IsSupplierCacheBlocking(), "the request path must fail closed before the initial load")
	contract, firstRate, channels := createSupplierCacheFixture(t, db)
	require.NoError(t, RefreshSupplierCache())
	require.False(t, IsSupplierCacheBlocking())
	inFlight, ok := GetSupplierCostSnapshot(channels[0].Id)
	require.True(t, ok)
	require.Equal(t, firstRate.Id, inFlight.RateVersionId)

	secondRate := SupplierContractRateVersion{
		ContractId:               contract.Id,
		ProcurementMultiplierPpm: 700_000,
		CreatedBy:                1,
	}
	require.NoError(t, db.Create(&secondRate).Error)
	require.NoError(t, db.Model(&SupplierContract{}).Where("id = ?", contract.Id).UpdateColumn("current_rate_version_id", secondRate.Id).Error)
	require.NoError(t, RefreshSupplierCache())
	refreshed, ok := GetSupplierCostSnapshot(channels[0].Id)
	require.True(t, ok)
	require.Equal(t, secondRate.Id, refreshed.RateVersionId)
	require.Equal(t, int64(700_000), refreshed.ProcurementMultiplierPpm)
	require.Equal(t, firstRate.Id, inFlight.RateVersionId)
	require.Equal(t, int64(650_000), inFlight.ProcurementMultiplierPpm)

	failedDB, err := gorm.Open(sqlite.Open("file:supplier-cache-failed?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	failedSQLDB, err := failedDB.DB()
	require.NoError(t, err)
	require.NoError(t, failedSQLDB.Close())
	DB = failedDB
	require.Error(t, RefreshSupplierCache())
	retained, ok := GetSupplierCostSnapshot(channels[0].Id)
	require.True(t, ok)
	require.Equal(t, refreshed, retained)
	health := GetSupplierCacheHealth()
	require.True(t, health.Blocking)
	require.True(t, IsSupplierCacheBlocking())
	require.NotEmpty(t, health.RefreshError)
	DB = db
	require.NoError(t, RefreshSupplierCache())
	require.False(t, IsSupplierCacheBlocking())
}

func TestSupplierAccountingRuntimeSnapshotFreezesOneGeneration(t *testing.T) {
	setupSupplierCacheTestDB(t, "supplier-cache-request-generation")
	supplierRuntimeGenerationPointer.Store(&supplierRuntimeGeneration{
		index: &supplierRuntimeIndex{
			channelCosts: map[int]types.SupplierCostSnapshot{
				1: {BindingVersionId: 11, SkipInternalAccounting: true},
			},
			excludedUserRuleId: map[int]int{99: 7},
		},
		health: &SupplierCacheHealth{LoadedAt: 1},
	})

	requestSnapshot := CaptureSupplierAccountingRuntimeSnapshot()
	supplierRuntimeGenerationPointer.Store(&supplierRuntimeGeneration{
		index: &supplierRuntimeIndex{
			channelCosts: map[int]types.SupplierCostSnapshot{
				1: {BindingVersionId: 22},
			},
			excludedUserRuleId: map[int]int{},
		},
		health: &SupplierCacheHealth{LoadedAt: 2},
	})

	frozenCost, ok := requestSnapshot.CostForChannel(1)
	require.True(t, ok)
	require.Equal(t, 11, frozenCost.BindingVersionId)
	require.True(t, frozenCost.SkipInternalAccounting)
	require.Equal(t, types.SupplierStatisticsScopeInternal, requestSnapshot.StatisticsScopeForUser(99).Scope)
	require.Equal(t, 7, requestSnapshot.StatisticsScopeForUser(99).ExclusionRuleId)

	current := CaptureSupplierAccountingRuntimeSnapshot()
	currentCost, ok := current.CostForChannel(1)
	require.True(t, ok)
	require.Equal(t, 22, currentCost.BindingVersionId)
	require.False(t, currentCost.SkipInternalAccounting)
	require.Equal(t, types.SupplierStatisticsScopeBusiness, current.StatisticsScopeForUser(99).Scope)
}

func TestSupplierCacheRefreshSerializesOverlappingPublications(t *testing.T) {
	setupSupplierCacheTestDB(t, "supplier-cache-serialized-refresh")
	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondAttempted := make(chan struct{})
	secondEntered := make(chan struct{})
	results := make(chan error, 2)
	health := func() *SupplierCacheHealth {
		return &SupplierCacheHealth{LoadedAt: time.Now().UTC().Unix(), Issues: []SupplierCacheIssue{}}
	}
	go func() {
		results <- refreshSupplierCache(DB, func(*gorm.DB) (*supplierRuntimeIndex, *SupplierCacheHealth, error) {
			close(firstEntered)
			<-releaseFirst
			return &supplierRuntimeIndex{
				channelCosts:       map[int]types.SupplierCostSnapshot{1: {BindingVersionId: 1}},
				excludedUserRuleId: map[int]int{},
			}, health(), nil
		})
	}()
	<-firstEntered
	go func() {
		close(secondAttempted)
		results <- refreshSupplierCache(DB, func(*gorm.DB) (*supplierRuntimeIndex, *SupplierCacheHealth, error) {
			close(secondEntered)
			return &supplierRuntimeIndex{
				channelCosts:       map[int]types.SupplierCostSnapshot{1: {BindingVersionId: 2}},
				excludedUserRuleId: map[int]int{},
			}, health(), nil
		})
	}()
	<-secondAttempted
	select {
	case <-secondEntered:
		t.Fatal("second refresh entered before the first refresh completed")
	case <-time.After(25 * time.Millisecond):
	}
	close(releaseFirst)
	require.NoError(t, <-results)
	require.NoError(t, <-results)
	<-secondEntered
	snapshot, ok := GetSupplierCostSnapshot(1)
	require.True(t, ok)
	require.Equal(t, 2, snapshot.BindingVersionId)
}

func TestSupplierCacheGettersPerformNoDatabaseWorkAfterRefresh(t *testing.T) {
	db := setupSupplierCacheTestDB(t, "supplier-cache-no-request-io")
	_, _, channels := createSupplierCacheFixture(t, db)
	require.NoError(t, RefreshSupplierCache())

	queryCount := 0
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("supplier_cache_test_query_count", func(*gorm.DB) {
		queryCount++
	}))
	for i := 0; i < 100; i++ {
		_, _ = GetSupplierCostSnapshot(channels[0].Id)
		_ = GetSupplierStatisticsScopeSnapshot(999)
	}
	require.Equal(t, 0, queryCount)
}

func TestInitChannelCacheAttachesBoundAndUnboundSnapshots(t *testing.T) {
	db := setupSupplierCacheTestDB(t, "supplier-cache-channel-attachment")
	_, _, channels := createSupplierCacheFixture(t, db)
	common.MemoryCacheEnabled = true
	InitChannelCache()

	bound, err := CacheGetChannel(channels[0].Id)
	require.NoError(t, err)
	require.True(t, bound.SupplierCostSnapshotLoaded)
	require.True(t, bound.SupplierCostSnapshot.IsBound())
	unbound, err := CacheGetChannel(channels[2].Id)
	require.NoError(t, err)
	require.True(t, unbound.SupplierCostSnapshotLoaded)
	require.False(t, unbound.SupplierCostSnapshot.IsBound())
}
