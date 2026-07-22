package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupDistributorSupplierSnapshotTest(t *testing.T, name string) (*gorm.DB, *model.Channel, *model.Channel) {
	t.Helper()
	originalDB := model.DB
	originalUsingSQLite := common.UsingSQLite
	t.Cleanup(func() {
		model.DB = originalDB
		common.UsingSQLite = originalUsingSQLite
	})

	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&model.Channel{},
		&model.UpstreamSupplier{},
		&model.SupplierContract{},
		&model.SupplierContractRateVersion{},
		&model.SupplierChannelBindingVersion{},
		&model.SupplierStatisticsExclusionRule{},
	))
	model.DB = db
	common.UsingSQLite = true

	supplier := model.UpstreamSupplier{Name: "middleware snapshot supplier"}
	require.NoError(t, db.Create(&supplier).Error)
	contract := model.SupplierContract{SupplierId: supplier.Id, Name: "middleware contract", ContractNo: "middleware-001"}
	require.NoError(t, db.Create(&contract).Error)
	rate := model.SupplierContractRateVersion{
		ContractId:               contract.Id,
		ProcurementMultiplierPpm: 650_000,
		CreatedBy:                1,
	}
	require.NoError(t, db.Create(&rate).Error)
	require.NoError(t, db.Model(&model.SupplierContract{}).Where("id = ?", contract.Id).UpdateColumn("current_rate_version_id", rate.Id).Error)
	bound := &model.Channel{Type: constant.ChannelTypeOpenAI, Name: "bound", Key: "bound-key", Status: common.ChannelStatusEnabled, SupplierContractId: &contract.Id}
	unbound := &model.Channel{Type: constant.ChannelTypeOpenAI, Name: "unbound", Key: "unbound-key", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(bound).Error)
	require.NoError(t, db.Create(unbound).Error)
	require.NoError(t, db.Create(&model.SupplierChannelBindingVersion{
		ChannelId:          bound.Id,
		SupplierContractId: &contract.Id,
	}).Error)
	require.NoError(t, model.RefreshSupplierCache())
	return db, bound, unbound
}

func newDistributorSupplierContext(userId int) *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	common.SetContextKey(c, constant.ContextKeyUserId, userId)
	return c
}

func TestSetupContextProjectsImmutableSupplierSnapshotAndRetryClearsCost(t *testing.T) {
	_, bound, unbound := setupDistributorSupplierSnapshotTest(t, "distributor-supplier-retry")
	c := newDistributorSupplierContext(201)

	// A DB-loaded channel has empty gorm:- fields, so the request path projects
	// the already-built immutable supplier index without supplier I/O.
	cachedSnapshot, ok := model.GetSupplierCostSnapshot(bound.Id)
	require.True(t, ok)
	require.True(t, cachedSnapshot.IsBound())
	require.Nil(t, SetupContextForSelectedChannel(c, bound, "model-a"))
	projectedSnapshot, ok := common.GetContextKeyType[types.SupplierCostSnapshot](c, constant.ContextKeySupplierCostSnapshot)
	require.True(t, ok)
	require.Equal(t, cachedSnapshot, projectedSnapshot)

	bound.SupplierCostSnapshot = cachedSnapshot
	bound.SupplierCostSnapshotLoaded = true
	require.Nil(t, SetupContextForSelectedChannel(c, bound, "model-a"))
	boundSnapshot, ok := common.GetContextKeyType[types.SupplierCostSnapshot](c, constant.ContextKeySupplierCostSnapshot)
	require.True(t, ok)
	require.True(t, boundSnapshot.IsBound())
	require.Equal(t, int64(650_000), boundSnapshot.ProcurementMultiplierPpm)

	// A cache-loaded unbound channel carries an authoritative zero snapshot.
	unbound.SupplierCostSnapshotLoaded = true
	require.Nil(t, SetupContextForSelectedChannel(c, unbound, "model-a"))
	unboundSnapshot, ok := common.GetContextKeyType[types.SupplierCostSnapshot](c, constant.ContextKeySupplierCostSnapshot)
	require.True(t, ok)
	require.False(t, unboundSnapshot.IsBound())
}

func TestSetupContextSupplierStatisticsScopeFreezesAcrossRetryRefresh(t *testing.T) {
	db, bound, unbound := setupDistributorSupplierSnapshotTest(t, "distributor-supplier-scope-freeze")
	exclude := model.SupplierStatisticsExclusionRule{
		UserId: 202, Action: model.SupplierStatisticsActionExclude,
		IdempotencyKey: "exclude-202", CreatedBy: 1,
	}
	require.NoError(t, db.Create(&exclude).Error)
	require.NoError(t, model.RefreshSupplierCache())

	c := newDistributorSupplierContext(202)
	require.Nil(t, SetupContextForSelectedChannel(c, bound, "model-a"))
	frozen, ok := common.GetContextKeyType[types.SupplierStatisticsScopeSnapshot](c, constant.ContextKeySupplierStatsScope)
	require.True(t, ok)
	require.Equal(t, types.SupplierStatisticsScopeInternal, frozen.Scope)
	require.Equal(t, exclude.Id, frozen.ExclusionRuleId)

	include := model.SupplierStatisticsExclusionRule{
		UserId: 202, Action: model.SupplierStatisticsActionInclude,
		IdempotencyKey: "include-202", CreatedBy: 1,
	}
	require.NoError(t, db.Create(&include).Error)
	require.NoError(t, model.RefreshSupplierCache())
	require.Equal(t, types.SupplierStatisticsScopeBusiness, model.GetSupplierStatisticsScopeSnapshot(202).Scope)

	unbound.SupplierCostSnapshotLoaded = true
	require.Nil(t, SetupContextForSelectedChannel(c, unbound, "model-a"))
	afterRetry, ok := common.GetContextKeyType[types.SupplierStatisticsScopeSnapshot](c, constant.ContextKeySupplierStatsScope)
	require.True(t, ok)
	require.Equal(t, frozen, afterRetry)
}

func TestSetupContextSupplierSnapshotsPerformNoDatabaseQueries(t *testing.T) {
	db, bound, _ := setupDistributorSupplierSnapshotTest(t, "distributor-supplier-no-io")
	snapshot, ok := model.GetSupplierCostSnapshot(bound.Id)
	require.True(t, ok)

	queryCount := 0
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("distributor_supplier_query_count", func(*gorm.DB) {
		queryCount++
	}))
	c := newDistributorSupplierContext(203)
	for i := 0; i < 10; i++ {
		require.Nil(t, SetupContextForSelectedChannel(c, bound, "model-a"))
		projectedSnapshot, exists := common.GetContextKeyType[types.SupplierCostSnapshot](c, constant.ContextKeySupplierCostSnapshot)
		require.True(t, exists)
		require.Equal(t, snapshot, projectedSnapshot)
	}
	bound.SupplierCostSnapshot = snapshot
	bound.SupplierCostSnapshotLoaded = true
	require.Nil(t, SetupContextForSelectedChannel(c, bound, "model-a"))
	loadedSnapshot, exists := common.GetContextKeyType[types.SupplierCostSnapshot](c, constant.ContextKeySupplierCostSnapshot)
	require.True(t, exists)
	require.Equal(t, snapshot, loadedSnapshot)
	require.Equal(t, 0, queryCount)
}

func TestSetupContextNoMemoryCacheDBChannelsUseImmutableBoundAndUnboundSnapshots(t *testing.T) {
	db, bound, unbound := setupDistributorSupplierSnapshotTest(t, "distributor-supplier-no-memory-cache")
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = oldMemoryCacheEnabled })

	dbBound, err := model.GetChannelById(bound.Id, true)
	require.NoError(t, err)
	require.False(t, dbBound.SupplierCostSnapshotLoaded)
	require.False(t, dbBound.SupplierCostSnapshot.IsBound())
	dbUnbound, err := model.GetChannelById(unbound.Id, true)
	require.NoError(t, err)
	require.False(t, dbUnbound.SupplierCostSnapshotLoaded)
	queryCount := 0
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("distributor_supplier_no_memory_query_count", func(*gorm.DB) {
		queryCount++
	}))

	c := newDistributorSupplierContext(204)
	require.Nil(t, SetupContextForSelectedChannel(c, dbBound, "model-a"))
	boundSnapshot, ok := common.GetContextKeyType[types.SupplierCostSnapshot](c, constant.ContextKeySupplierCostSnapshot)
	require.True(t, ok)
	require.True(t, boundSnapshot.IsBound())
	require.Equal(t, int64(650_000), boundSnapshot.ProcurementMultiplierPpm)

	require.Nil(t, SetupContextForSelectedChannel(c, dbUnbound, "model-a"))
	unboundSnapshot, ok := common.GetContextKeyType[types.SupplierCostSnapshot](c, constant.ContextKeySupplierCostSnapshot)
	require.True(t, ok)
	require.Equal(t, types.SupplierCostSnapshot{}, unboundSnapshot)
	require.Equal(t, 0, queryCount)
}
