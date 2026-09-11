package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPLGModelCatalogSnapshotTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()

	originalDB := DB
	t.Cleanup(func() {
		DB = originalDB
	})

	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&PLGModelCatalogSnapshot{}))
	DB = db
	return db
}

func TestPLGModelCatalogSnapshotFirstWriteCreatesBaseline(t *testing.T) {
	setupPLGModelCatalogSnapshotTestDB(t, "plg-catalog-baseline")

	previous, err := GetPLGModelCatalogSnapshot("plg")
	require.NoError(t, err)
	require.Nil(t, previous)

	created, err := CreatePLGModelCatalogSnapshotIfMissing("plg", "fp-1", []string{"a", "b"}, 100)
	require.NoError(t, err)
	require.True(t, created)

	created, err = CreatePLGModelCatalogSnapshotIfMissing("plg", "fp-other", []string{"z"}, 101)
	require.NoError(t, err)
	require.False(t, created)

	snapshot, err := GetPLGModelCatalogSnapshot("plg")
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	require.Equal(t, "fp-1", snapshot.Fingerprint)
	require.Equal(t, []string{"a", "b"}, snapshot.ModelNameList())
	require.Equal(t, 2, snapshot.ModelCount)
	require.Equal(t, int64(100), snapshot.UpdatedAt)
}

func TestPLGModelCatalogSnapshotClaimChangeIsCompareAndSwap(t *testing.T) {
	setupPLGModelCatalogSnapshotTestDB(t, "plg-catalog-cas")

	created, err := CreatePLGModelCatalogSnapshotIfMissing("plg", "fp-1", []string{"a"}, 100)
	require.NoError(t, err)
	require.True(t, created)

	claimed, err := ClaimPLGModelCatalogSnapshotChange("plg", "fp-1", "fp-2", []string{"a", "b"}, 200)
	require.NoError(t, err)
	require.True(t, claimed)

	// A second node that still believes the previous fingerprint was fp-1
	// must lose: the row already moved on.
	claimed, err = ClaimPLGModelCatalogSnapshotChange("plg", "fp-1", "fp-3", []string{"c"}, 201)
	require.NoError(t, err)
	require.False(t, claimed)

	snapshot, err := GetPLGModelCatalogSnapshot("plg")
	require.NoError(t, err)
	require.Equal(t, "fp-2", snapshot.Fingerprint)
	require.Equal(t, []string{"a", "b"}, snapshot.ModelNameList())
	require.Equal(t, int64(200), snapshot.UpdatedAt)
}

func TestPLGModelCatalogSnapshotModelNameListToleratesCorruptJSON(t *testing.T) {
	snapshot := &PLGModelCatalogSnapshot{ModelNames: "not json"}
	require.Empty(t, snapshot.ModelNameList())
}
