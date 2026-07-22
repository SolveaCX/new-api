package model

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type supplierAdminQueryCounter struct {
	logger.Interface
	count atomic.Int64
}

func setupSupplierAdminProjectionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return setupSupplierTestDB(t, filepath.Join(t.TempDir(), "supplier-admin"))
}

func (c *supplierAdminQueryCounter) Trace(_ context.Context, _ time.Time, sql func() (string, int64), _ error) {
	statement, _ := sql()
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(statement)), "SELECT") {
		c.count.Add(1)
	}
}

func TestSetChannelSupplierContractCASPreventsStaleWritesAndKeepsRetriesIdempotent(t *testing.T) {
	db := setupSupplierAdminProjectionTestDB(t)
	first := createSupplierContractFixture(t, db, "cas-first", "cas-first")
	second := createSupplierContractFixture(t, db, "cas-second", "cas-second")
	_, err := CreateAndActivateSupplierContractRateVersion(first.Id, 650_000, 1, "initial")
	require.NoError(t, err)
	_, err = CreateAndActivateSupplierContractRateVersion(second.Id, 700_000, 1, "initial")
	require.NoError(t, err)
	channel := Channel{Name: "cas channel", Key: "key", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(&channel).Error)

	require.NoError(t, SetChannelSupplierContractCASForActor(channel.Id, 0, &first.Id, 7))
	require.NoError(t, SetChannelSupplierContractCAS(channel.Id, 0, &first.Id), "desired-state retry succeeds even with stale expected state")
	require.ErrorIs(t, SetChannelSupplierContractCAS(channel.Id, 0, &second.Id), ErrSupplierBindingChanged)
	require.NoError(t, SetChannelSupplierContractCASForActor(channel.Id, first.Id, &second.Id, 8))
	require.NoError(t, SetChannelSupplierContractCAS(channel.Id, first.Id, &second.Id), "rebind retry succeeds when desired state already won")
	require.ErrorIs(t, SetChannelSupplierContractCAS(channel.Id, first.Id, nil), ErrSupplierBindingChanged, "stale unbind must not remove a concurrent rebind")
	require.NoError(t, SetChannelSupplierContractCASForActor(channel.Id, second.Id, nil, 9))
	require.NoError(t, SetChannelSupplierContractCAS(channel.Id, second.Id, nil), "delete retry succeeds after the channel is already unbound")

	versions, total, err := ListSupplierChannelBindingVersions(channel.Id, SupplierPage{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, int64(3), total, "retries and rejected stale writes must not create history")
	require.Len(t, versions, 3)
	require.Nil(t, versions[0].SupplierContractId)
	require.NotNil(t, versions[0].PreviousSupplierContractId)
	require.Equal(t, second.Id, *versions[0].PreviousSupplierContractId)
	require.Equal(t, 9, versions[0].CreatedBy)
	require.Equal(t, second.Id, *versions[1].SupplierContractId)
	require.Equal(t, first.Id, *versions[1].PreviousSupplierContractId)
	require.Equal(t, 8, versions[1].CreatedBy)
	require.Equal(t, first.Id, *versions[2].SupplierContractId)
	require.Nil(t, versions[2].PreviousSupplierContractId)
	require.Equal(t, 7, versions[2].CreatedBy)
	for index := range versions {
		require.Positive(t, versions[index].EffectiveAt)
		require.ErrorIs(t, db.Model(&versions[index]).Update("created_by", 99).Error, ErrSupplierAppendOnly)
		require.ErrorIs(t, db.Delete(&versions[index]).Error, ErrSupplierAppendOnly)
	}
}

func TestSetChannelSupplierContractCASAllowsOnlyOneConcurrentUnboundWriter(t *testing.T) {
	db := setupSupplierAdminProjectionTestDB(t)
	first := createSupplierContractFixture(t, db, "cas-concurrent-first", "cas-concurrent-first")
	second := createSupplierContractFixture(t, db, "cas-concurrent-second", "cas-concurrent-second")
	_, err := CreateAndActivateSupplierContractRateVersion(first.Id, 650_000, 1, "initial")
	require.NoError(t, err)
	_, err = CreateAndActivateSupplierContractRateVersion(second.Id, 700_000, 1, "initial")
	require.NoError(t, err)
	channel := Channel{Name: "concurrent cas channel", Key: "key", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(&channel).Error)

	results := make([]error, 2)
	desired := []*int{&first.Id, &second.Id}
	var wait sync.WaitGroup
	for index := range desired {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			results[index] = SetChannelSupplierContractCAS(channel.Id, 0, desired[index])
		}(index)
	}
	wait.Wait()

	successes := 0
	conflicts := 0
	for _, result := range results {
		if result == nil {
			successes++
		} else if result == ErrSupplierBindingChanged {
			conflicts++
		} else {
			t.Fatalf("unexpected concurrent CAS result: %v", result)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
}

func TestSupplierAdminProjectionsAreEnrichedFilteredAndBounded(t *testing.T) {
	db := setupSupplierAdminProjectionTestDB(t)
	contract := createSupplierContractFixture(t, db, "projection supplier", "PROJECTION-1")
	rate, err := CreateAndActivateSupplierContractRateVersion(contract.Id, 650_000, 1, "initial")
	require.NoError(t, err)
	_, err = CreateSupplierInventoryAdjustment(&SupplierInventoryAdjustment{ContractId: contract.Id, DeltaMicroUsd: 200_000_000_000, Type: SupplierInventoryAdjustmentTypeReplenishment, Reason: "stock", IdempotencyKey: "projection-stock", CreatedBy: 1})
	require.NoError(t, err)
	bound := Channel{Name: "projection bound channel", Key: "bound", Status: common.ChannelStatusEnabled, SupplierContractId: &contract.Id}
	unbound := Channel{Name: "projection unbound channel", Key: "unbound", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(&bound).Error)
	require.NoError(t, db.Create(&unbound).Error)

	suppliers, total, err := ListSupplierAdminRows(SupplierAdminListFilter{Page: SupplierPage{Limit: 500}, Keyword: "projection"})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, suppliers, 1)
	require.Equal(t, int64(1), suppliers[0].ContractCount)
	require.Equal(t, int64(1), suppliers[0].LinkedChannelCount)
	require.Equal(t, int64(200_000_000_000), suppliers[0].InventoryTotalMicroUsd)

	contracts, total, err := ListSupplierContractAdminRows(SupplierContractAdminListFilter{Page: SupplierPage{Limit: 500}, Keyword: "PROJECTION-1"})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, contracts, 1)
	require.Equal(t, "projection supplier", contracts[0].SupplierName)
	require.NotNil(t, contracts[0].CurrentProcurementMultiplierPpm)
	require.Equal(t, rate.ProcurementMultiplierPpm, *contracts[0].CurrentProcurementMultiplierPpm)
	require.NotNil(t, contracts[0].CurrentRateEffectiveAt)
	require.Equal(t, rate.EffectiveAt, *contracts[0].CurrentRateEffectiveAt)
	require.Equal(t, int64(1), contracts[0].LinkedChannelCount)

	boundState := true
	bindings, total, err := ListSupplierChannelBindingAdminRows(SupplierChannelBindingAdminListFilter{Page: SupplierPage{Limit: 500}, Bound: &boundState, Keyword: "projection"})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, bindings, 1)
	require.Equal(t, bound.Id, bindings[0].ChannelId)
	require.NotNil(t, bindings[0].SupplierName)
	require.Equal(t, "projection supplier", *bindings[0].SupplierName)
	require.NotNil(t, bindings[0].ContractNo)
	require.Equal(t, "PROJECTION-1", *bindings[0].ContractNo)
	require.NotNil(t, bindings[0].CurrentRateVersionId)
	require.Equal(t, rate.Id, *bindings[0].CurrentRateVersionId)
	require.NotNil(t, bindings[0].CurrentProcurementMultiplierPpm)

	bindings, total, err = ListSupplierChannelBindingAdminRows(SupplierChannelBindingAdminListFilter{Page: SupplierPage{Limit: 100}, Keyword: "PROJECTION-1"})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, bindings, 1)
}

func TestSupplierEffectiveExclusionsSelectLatestBeforeFilteringAndRetainMissingIdentity(t *testing.T) {
	db := setupSupplierAdminProjectionTestDB(t)
	require.NoError(t, db.AutoMigrate(&User{}))
	user := User{Username: "excluded-user", DisplayName: "Excluded User", Password: "not-used", AffCode: "excluded-aff"}
	require.NoError(t, db.Create(&user).Error)

	require.NoError(t, db.Create(&SupplierStatisticsExclusionRule{UserId: user.Id, Action: SupplierStatisticsActionExclude, Reason: "old", IdempotencyKey: "old", CreatedBy: 1}).Error)
	require.NoError(t, db.Create(&SupplierStatisticsExclusionRule{UserId: user.Id, Action: SupplierStatisticsActionInclude, Reason: "latest", IdempotencyKey: "latest", CreatedBy: 1}).Error)
	var rules []SupplierStatisticsExclusionRule
	require.NoError(t, db.Where("user_id = ?", user.Id).Order("id ASC").Find(&rules).Error)
	require.Len(t, rules, 2)
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Model(&SupplierStatisticsExclusionRule{}).Where("id IN ?", []int{rules[0].Id, rules[1].Id}).UpdateColumn("effective_at", rules[0].EffectiveAt).Error)
	require.NoError(t, db.Create(&SupplierStatisticsExclusionRule{UserId: 999_999, Action: SupplierStatisticsActionExclude, Reason: "missing identity", IdempotencyKey: "missing", CreatedBy: 1}).Error)
	deletedUser := User{Username: "deleted-user", DisplayName: "Deleted User", Password: "not-used", AffCode: "deleted-aff"}
	require.NoError(t, db.Create(&deletedUser).Error)
	require.NoError(t, db.Create(&SupplierStatisticsExclusionRule{UserId: deletedUser.Id, Action: SupplierStatisticsActionExclude, Reason: "soft deleted identity", IdempotencyKey: "deleted", CreatedBy: 1}).Error)
	require.NoError(t, db.Delete(&deletedUser).Error)

	includeRows, total, err := ListSupplierEffectiveExclusions(SupplierExclusionAdminListFilter{Page: SupplierPage{Limit: 100}, Action: SupplierStatisticsActionInclude})
	require.NoError(t, err)
	require.Equal(t, int64(1), total, "action filter must apply after latest-by-(effective_at,id) selection")
	require.Len(t, includeRows, 1)
	require.Equal(t, rules[1].Id, includeRows[0].RuleId)
	require.Equal(t, "excluded-user", includeRows[0].Username)
	require.True(t, includeRows[0].IdentityPresent)
	require.NotNil(t, includeRows[0].Role)
	require.Equal(t, user.Role, *includeRows[0].Role)
	require.NotNil(t, includeRows[0].Status)
	require.Equal(t, user.Status, *includeRows[0].Status)
	require.False(t, includeRows[0].Excluded)

	allRows, total, err := ListSupplierEffectiveExclusions(SupplierExclusionAdminListFilter{Page: SupplierPage{Limit: 100}})
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, allRows, 3)
	byUser := make(map[int]SupplierEffectiveExclusionRow, len(allRows))
	for _, row := range allRows {
		byUser[row.UserId] = row
	}
	require.Empty(t, byUser[999_999].Username, "LEFT JOIN must retain a rule whose identity row is missing")
	require.False(t, byUser[999_999].IdentityPresent)
	require.Nil(t, byUser[999_999].Role)
	require.Nil(t, byUser[999_999].Status)
	require.Empty(t, byUser[deletedUser.Id].Username, "soft-deleted identity must not be exposed")
	require.Empty(t, byUser[deletedUser.Id].DisplayName)
	require.False(t, byUser[deletedUser.Id].IdentityPresent)
	require.Nil(t, byUser[deletedUser.Id].Role)
	require.Nil(t, byUser[deletedUser.Id].Status)

	numericRows, total, err := ListSupplierEffectiveExclusions(SupplierExclusionAdminListFilter{Page: SupplierPage{Limit: 100}, Keyword: strconv.Itoa(user.Id)})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, numericRows, 1)
	require.Equal(t, user.Id, numericRows[0].UserId)
}

func TestSupplierAdminProjectionQueryCountsAndPaginationAreBounded(t *testing.T) {
	db := setupSupplierAdminProjectionTestDB(t)
	first := createSupplierContractFixture(t, db, "query-count-first", "QUERY-1")
	second := createSupplierContractFixture(t, db, "query-count-second", "QUERY-2")
	_, err := CreateAndActivateSupplierContractRateVersion(first.Id, 650_000, 1, "initial")
	require.NoError(t, err)
	_, err = CreateAndActivateSupplierContractRateVersion(second.Id, 700_000, 1, "initial")
	require.NoError(t, err)
	_, err = CreateSupplierInventoryAdjustment(&SupplierInventoryAdjustment{ContractId: second.Id, DeltaMicroUsd: 1_000_000, Type: SupplierInventoryAdjustmentTypeInitial, Reason: "initial", IdempotencyKey: "query-count-stock", CreatedBy: 1})
	require.NoError(t, err)
	channel := Channel{Name: "query-count-channel", Key: "key", Status: common.ChannelStatusEnabled, SupplierContractId: &second.Id}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.AutoMigrate(&User{}))
	user := User{Username: "query-count-user", DisplayName: "Query Count User", Password: "not-used", AffCode: "query-count-aff"}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&SupplierStatisticsExclusionRule{UserId: user.Id, Action: SupplierStatisticsActionExclude, Reason: "internal", IdempotencyKey: "query-count-rule", CreatedBy: 1}).Error)

	counter := &supplierAdminQueryCounter{Interface: logger.Discard}
	DB = db.Session(&gorm.Session{Logger: counter})

	suppliers, total, err := ListSupplierAdminRows(SupplierAdminListFilter{Page: SupplierPage{Limit: 1}})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, suppliers, 1)
	require.Equal(t, second.SupplierId, suppliers[0].Id, "page order is stable id DESC")
	require.Equal(t, int64(5), counter.count.Load(), "supplier page uses count, page, and three batch enrich queries")

	counter.count.Store(0)
	contracts, total, err := ListSupplierContractAdminRows(SupplierContractAdminListFilter{Page: SupplierPage{Limit: 1}})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, contracts, 1)
	require.Equal(t, second.Id, contracts[0].Id)
	require.Equal(t, int64(4), counter.count.Load(), "contract page uses count, page, and two batch enrich queries")

	counter.count.Store(0)
	bindings, _, err := ListSupplierChannelBindingAdminRows(SupplierChannelBindingAdminListFilter{Page: SupplierPage{Limit: 100}})
	require.NoError(t, err)
	require.NotEmpty(t, bindings)
	require.Equal(t, int64(2), counter.count.Load())

	counter.count.Store(0)
	effective, _, err := ListSupplierEffectiveExclusions(SupplierExclusionAdminListFilter{Page: SupplierPage{Limit: 100}})
	require.NoError(t, err)
	require.Len(t, effective, 1)
	require.Equal(t, int64(2), counter.count.Load())
}

func TestSupplierAdminEmptyPagesUseNonNilSlices(t *testing.T) {
	db := setupSupplierAdminProjectionTestDB(t)
	require.NoError(t, db.AutoMigrate(&User{}))

	suppliers, _, err := ListSupplierAdminRows(SupplierAdminListFilter{Page: SupplierPage{Limit: 20}})
	require.NoError(t, err)
	require.NotNil(t, suppliers)

	contracts, _, err := ListSupplierContractAdminRows(SupplierContractAdminListFilter{Page: SupplierPage{Limit: 20}})
	require.NoError(t, err)
	require.NotNil(t, contracts)

	rates, _, err := ListSupplierContractRateVersions(1, SupplierPage{Limit: 20})
	require.NoError(t, err)
	require.NotNil(t, rates)

	adjustments, _, err := ListSupplierInventoryAdjustments(1, SupplierPage{Limit: 20})
	require.NoError(t, err)
	require.NotNil(t, adjustments)

	history, _, err := ListSupplierExclusionHistory(SupplierExclusionAdminListFilter{Page: SupplierPage{Limit: 20}})
	require.NoError(t, err)
	require.NotNil(t, history)

	effective, _, err := ListSupplierEffectiveExclusions(SupplierExclusionAdminListFilter{Page: SupplierPage{Limit: 20}})
	require.NoError(t, err)
	require.NotNil(t, effective)

	bindings, _, err := ListSupplierChannelBindingAdminRows(SupplierChannelBindingAdminListFilter{Page: SupplierPage{Limit: 20}})
	require.NoError(t, err)
	require.NotNil(t, bindings)
}

func TestSupplierAdminProjectionQueriesStayMySQL57AndPostgreSQL96Compatible(t *testing.T) {
	sqliteDB := setupSupplierAdminProjectionTestDB(t)
	connection, err := sqliteDB.DB()
	require.NoError(t, err)
	dialectors := map[string]gorm.Dialector{
		"mysql":    mysql.New(mysql.Config{Conn: connection, SkipInitializeWithVersion: true}),
		"postgres": postgres.New(postgres.Config{Conn: connection, WithoutReturning: true}),
	}
	original := DB
	defer func() { DB = original }()
	for name, dialector := range dialectors {
		t.Run(name, func(t *testing.T) {
			dryRun, openErr := gorm.Open(dialector, &gorm.Config{DryRun: true})
			require.NoError(t, openErr)
			DB = dryRun
			var effective []SupplierEffectiveExclusionRow
			statement := supplierEffectiveExclusionBaseQuery(SupplierExclusionAdminListFilter{}).
				Select("rule.id").Find(&effective).Statement.SQL.String()
			require.Contains(t, statement, "LEFT JOIN supplier_statistics_exclusion_rules AS newer")
			require.NotContains(t, strings.ToUpper(statement), "ROW_NUMBER")
			require.NotContains(t, strings.ToUpper(statement), " OVER ")

			var contracts []SupplierContractAdminRow
			statement = supplierContractAdminBaseQuery(SupplierContractAdminListFilter{}).
				Select("contract.id").Find(&contracts).Statement.SQL.String()
			require.Contains(t, statement, "JOIN upstream_suppliers")
		})
	}
}
