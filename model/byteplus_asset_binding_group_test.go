package model

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBytePlusLegacyAndBindingGroupsUseSeparateTables(t *testing.T) {
	newAssetTestDB(t, &BytePlusAssetGroup{}, &BytePlusAssetBindingGroup{})

	require.True(t, DB.Migrator().HasTable("byte_plus_asset_groups"))
	require.True(t, DB.Migrator().HasTable("byte_plus_asset_binding_groups"))
}

func TestBytePlusAssetBindingGroupScopesAreIndependent(t *testing.T) {
	newBytePlusAssetBindingGroupTestDB(t)

	first, owner, err := ClaimBytePlusAssetBindingGroup(7, 120, "byteplus:v1:scope-a", 1000, 900)
	require.NoError(t, err)
	require.True(t, owner)
	require.Equal(t, "byteplus:v1:scope-a", first.BindingScope)

	updated, err := ActivateBytePlusAssetBindingGroup(first.Id, first.LeaseUpdatedTime, "group-a", "req-a", 1010)
	require.NoError(t, err)
	require.True(t, updated)

	second, owner, err := ClaimBytePlusAssetBindingGroup(7, 120, "byteplus:v1:scope-b", 1020, 900)
	require.NoError(t, err)
	require.True(t, owner)
	require.Equal(t, "byteplus:v1:scope-b", second.BindingScope)
	require.NotEqual(t, first.Id, second.Id)

	reloaded, owner, err := ClaimBytePlusAssetBindingGroup(7, 120, "byteplus:v1:scope-a", 1030, 900)
	require.NoError(t, err)
	require.False(t, owner)
	require.Equal(t, first.Id, reloaded.Id)
	require.Equal(t, "group-a", reloaded.UpstreamGroupId)
}

func TestBytePlusAssetBindingGroupStaleTakeoverIsScoped(t *testing.T) {
	newBytePlusAssetBindingGroupTestDB(t)

	stale, owner, err := ClaimBytePlusAssetBindingGroup(8, 120, "byteplus:v1:scope-a", 1000, 900)
	require.NoError(t, err)
	require.True(t, owner)

	sibling, owner, err := ClaimBytePlusAssetBindingGroup(8, 120, "byteplus:v1:scope-b", 1010, 900)
	require.NoError(t, err)
	require.True(t, owner)
	updated, err := ActivateBytePlusAssetBindingGroup(sibling.Id, sibling.LeaseUpdatedTime, "group-b", "req-b", 1020)
	require.NoError(t, err)
	require.True(t, updated)

	reclaimed, owner, err := ClaimBytePlusAssetBindingGroup(8, 120, "byteplus:v1:scope-a", 1200, 1100)
	require.NoError(t, err)
	require.True(t, owner)
	require.Equal(t, stale.Id, reclaimed.Id)
	require.Equal(t, BytePlusAssetGroupStatusCreating, reclaimed.Status)

	storedSibling, err := GetBytePlusAssetBindingGroup(8, 120, "byteplus:v1:scope-b")
	require.NoError(t, err)
	require.Equal(t, BytePlusAssetGroupStatusActive, storedSibling.Status)
	require.Equal(t, "group-b", storedSibling.UpstreamGroupId)
}

func TestBytePlusAssetBindingGroupInvalidationRejectsStaleObserver(t *testing.T) {
	newBytePlusAssetBindingGroupTestDB(t)

	group, owner, err := ClaimBytePlusAssetBindingGroup(9, 120, "byteplus:v1:scope-a", 1000, 900)
	require.NoError(t, err)
	require.True(t, owner)
	updated, err := ActivateBytePlusAssetBindingGroup(group.Id, group.LeaseUpdatedTime, "old-group", "req-old", 1010)
	require.NoError(t, err)
	require.True(t, updated)

	updated, err = InvalidateActiveBytePlusAssetBindingGroup(group.Id, "old-group", "req-missing", "upstream group not found", 1020)
	require.NoError(t, err)
	require.True(t, updated)

	replacement, owner, err := ClaimBytePlusAssetBindingGroup(9, 120, "byteplus:v1:scope-a", 1030, 900)
	require.NoError(t, err)
	require.True(t, owner)
	updated, err = ActivateBytePlusAssetBindingGroup(replacement.Id, replacement.LeaseUpdatedTime, "replacement-group", "req-replacement", 1040)
	require.NoError(t, err)
	require.True(t, updated)

	updated, err = InvalidateActiveBytePlusAssetBindingGroup(group.Id, "old-group", "req-late", "late observer", 1050)
	require.NoError(t, err)
	require.False(t, updated)

	stored, err := GetBytePlusAssetBindingGroup(9, 120, "byteplus:v1:scope-a")
	require.NoError(t, err)
	require.Equal(t, BytePlusAssetGroupStatusActive, stored.Status)
	require.Equal(t, "replacement-group", stored.UpstreamGroupId)
}

func TestActivateBytePlusAssetBindingGroupRejectsBlankUpstreamGroupID(t *testing.T) {
	newBytePlusAssetBindingGroupTestDB(t)

	group, owner, err := ClaimBytePlusAssetBindingGroup(10, 120, "byteplus:v1:scope-a", 1000, 900)
	require.NoError(t, err)
	require.True(t, owner)

	updated, err := ActivateBytePlusAssetBindingGroup(group.Id, group.LeaseUpdatedTime, " \t ", "req-empty", 1010)
	require.ErrorIs(t, err, ErrBytePlusAssetBindingGroupUpstreamIDRequired)
	require.False(t, updated)

	stored, err := GetBytePlusAssetBindingGroup(10, 120, "byteplus:v1:scope-a")
	require.NoError(t, err)
	require.Equal(t, BytePlusAssetGroupStatusCreating, stored.Status)
	require.Empty(t, stored.UpstreamGroupId)
}

func TestClaimBytePlusAssetBindingGroupRecoversActiveRowWithBlankUpstreamGroupID(t *testing.T) {
	newBytePlusAssetBindingGroupTestDB(t)
	require.NoError(t, DB.Create(&BytePlusAssetBindingGroup{
		UserId:           11,
		ChannelId:        120,
		BindingScope:     "byteplus:v1:scope-a",
		UpstreamGroupId:  " \t ",
		Status:           BytePlusAssetGroupStatusActive,
		LeaseUpdatedTime: 1000,
		CreatedTime:      1000,
		UpdatedTime:      1000,
	}).Error)

	group, owner, err := ClaimBytePlusAssetBindingGroup(11, 120, "byteplus:v1:scope-a", 1100, 800)
	require.NoError(t, err)
	require.True(t, owner)
	require.Equal(t, BytePlusAssetGroupStatusCreating, group.Status)
	require.Empty(t, group.UpstreamGroupId)
	require.Equal(t, int64(1100), group.LeaseUpdatedTime)
}

func TestClaimBytePlusAssetBindingGroupConcurrentSameScopeHasOneRowAndOneOwner(t *testing.T) {
	newBytePlusAssetBindingGroupTestDB(t)
	sqlDB, err := DB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(2)
	sqlDB.SetMaxIdleConns(2)
	firstConn, err := sqlDB.Conn(context.Background())
	require.NoError(t, err)
	secondConn, err := sqlDB.Conn(context.Background())
	require.NoError(t, err)
	require.NoError(t, firstConn.PingContext(context.Background()))
	require.NoError(t, secondConn.PingContext(context.Background()))
	require.NoError(t, firstConn.Close())
	require.NoError(t, secondConn.Close())
	require.GreaterOrEqual(t, sqlDB.Stats().OpenConnections, 2)

	type claimResult struct {
		group *BytePlusAssetBindingGroup
		owner bool
		err   error
	}
	start := make(chan struct{})
	ready := make(chan struct{}, 2)
	results := make(chan claimResult, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	for range 2 {
		go func() {
			defer workers.Done()
			ready <- struct{}{}
			<-start
			group, owner, claimErr := ClaimBytePlusAssetBindingGroup(12, 120, "byteplus:v1:scope-a", 1000, 700)
			results <- claimResult{group: group, owner: owner, err: claimErr}
		}()
	}
	<-ready
	<-ready
	close(start)

	done := make(chan struct{})
	go func() {
		workers.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent scoped claims did not complete within 5 seconds")
	}
	close(results)

	owners := 0
	var rowID int64
	for result := range results {
		require.NoError(t, result.err)
		require.NotNil(t, result.group)
		if rowID == 0 {
			rowID = result.group.Id
		} else {
			require.Equal(t, rowID, result.group.Id)
		}
		if result.owner {
			owners++
		}
	}
	require.Equal(t, 1, owners)
	var rows int64
	require.NoError(t, DB.Model(&BytePlusAssetBindingGroup{}).
		Where("user_id = ? AND channel_id = ? AND binding_scope = ?", 12, 120, "byteplus:v1:scope-a").
		Count(&rows).Error)
	require.Equal(t, int64(1), rows)
	// SQLite validates the uniqueness/CAS outcome here, but this test does not
	// exercise MySQL or PostgreSQL row-lock behavior.
}

func TestBytePlusAssetBindingGroupIsRegisteredForMigration(t *testing.T) {
	for _, migration := range orderedMigrationModels() {
		if _, ok := migration.model.(*BytePlusAssetBindingGroup); ok {
			return
		}
	}
	t.Fatal("BytePlusAssetBindingGroup is not registered for migration")
}

func newBytePlusAssetBindingGroupTestDB(t *testing.T) {
	t.Helper()
	newBytePlusAssetTestDB(t)
	require.NoError(t, DB.AutoMigrate(&BytePlusAssetBindingGroup{}))
}
