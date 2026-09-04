package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBytePlusLegacyAndBindingGroupsUseSeparateTables(t *testing.T) {
	newAssetTestDB(t, &BytePlusAssetGroup{}, &BytePlusAssetBindingGroup{})

	require.True(t, DB.Migrator().HasTable("byte_plus_asset_groups"))
	require.True(t, DB.Migrator().HasTable("byteplus_asset_binding_groups"))
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
