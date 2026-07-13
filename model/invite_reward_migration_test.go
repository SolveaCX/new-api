package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupInviteRewardMigrationTest(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := DB
	originalRedisEnabled := common.RedisEnabled
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL

	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/invite-reward-migration.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}))

	DB = db
	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false

	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
		DB = originalDB
		common.RedisEnabled = originalRedisEnabled
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
	})

	return db
}

func TestMigrateLegacyAffQuotaToQuotaIsIdempotent(t *testing.T) {
	db := setupInviteRewardMigrationTest(t)
	users := []User{
		{Id: 101, Username: "legacy-reward", Password: "password123", AffCode: "legacy-reward-code", Quota: 300, AffQuota: 200},
		{Id: 102, Username: "no-legacy-reward", Password: "password123", AffCode: "no-legacy-reward-code", Quota: 500},
	}
	require.NoError(t, db.Create(&users).Error)

	require.NoError(t, MigrateLegacyAffQuotaToQuota())
	require.NoError(t, MigrateLegacyAffQuotaToQuota())

	var migrated User
	require.NoError(t, db.First(&migrated, users[0].Id).Error)
	require.Equal(t, 500, migrated.Quota)
	require.Zero(t, migrated.AffQuota)

	var unchanged User
	require.NoError(t, db.First(&unchanged, users[1].Id).Error)
	require.Equal(t, 500, unchanged.Quota)
	require.Zero(t, unchanged.AffQuota)
}

func TestMigrateLegacyAffQuotaBatchOnlyMigratesSelectedUsers(t *testing.T) {
	db := setupInviteRewardMigrationTest(t)
	users := []User{
		{Id: 151, Username: "selected-legacy-reward", Password: "password123", AffCode: "selected-legacy-reward-code", Quota: 100, AffQuota: 200},
		{Id: 152, Username: "late-legacy-reward", Password: "password123", AffCode: "late-legacy-reward-code", Quota: 300, AffQuota: 400},
	}
	require.NoError(t, db.Create(&users).Error)

	require.NoError(t, migrateLegacyAffQuotaBatch([]int{users[0].Id}))

	var selected User
	require.NoError(t, db.First(&selected, users[0].Id).Error)
	require.Equal(t, 300, selected.Quota)
	require.Zero(t, selected.AffQuota)

	var late User
	require.NoError(t, db.First(&late, users[1].Id).Error)
	require.Equal(t, 300, late.Quota)
	require.Equal(t, 400, late.AffQuota)
}

func TestMigrateUserLegacyAffQuotaToQuotaScopesTheReconciliation(t *testing.T) {
	db := setupInviteRewardMigrationTest(t)
	users := []User{
		{Id: 201, Username: "selected-user", Password: "password123", AffCode: "selected-user-code", Quota: 100, AffQuota: 250},
		{Id: 202, Username: "other-user", Password: "password123", AffCode: "other-user-code", Quota: 400, AffQuota: 600},
	}
	require.NoError(t, db.Create(&users).Error)

	require.NoError(t, MigrateUserLegacyAffQuotaToQuota(users[0].Id))
	require.NoError(t, MigrateUserLegacyAffQuotaToQuota(users[0].Id))

	var selected User
	require.NoError(t, db.First(&selected, users[0].Id).Error)
	require.Equal(t, 350, selected.Quota)
	require.Zero(t, selected.AffQuota)

	var other User
	require.NoError(t, db.First(&other, users[1].Id).Error)
	require.Equal(t, 400, other.Quota)
	require.Equal(t, 600, other.AffQuota)
}
