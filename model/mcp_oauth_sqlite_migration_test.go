package model

import (
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type legacySQLiteTokenWithoutOAuthGrantID struct {
	Id                    int    `gorm:"primaryKey"`
	UserId                int    `gorm:"index"`
	Key                   string `gorm:"column:key;type:varchar(128);uniqueIndex"`
	Status                int    `gorm:"default:1"`
	Name                  string `gorm:"index"`
	CreatedTime           int64  `gorm:"bigint"`
	AccessedTime          int64  `gorm:"bigint"`
	ExpiredTime           int64  `gorm:"bigint;default:-1"`
	RemainQuota           int    `gorm:"default:0"`
	UnlimitedQuota        bool
	ModelLimitsEnabled    bool
	ModelLimits           string `gorm:"type:text"`
	ModelBlacklistEnabled bool
	ModelBlacklist        string  `gorm:"type:text"`
	AllowIps              *string `gorm:"default:''"`
	UsedQuota             int     `gorm:"default:0"`
	Group                 string  `gorm:"column:group;default:''"`
	CrossGroupRetry       bool
	Source                string         `gorm:"index;default:''"`
	DeletedAt             gorm.DeletedAt `gorm:"index"`
}

func (legacySQLiteTokenWithoutOAuthGrantID) TableName() string {
	return "tokens"
}

func TestSQLiteMigrateDBAddsTokenOAuthGrantIDWithoutUniqueAddColumn(t *testing.T) {
	originalDB := DB
	originalLogDB := LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalRedisEnabled := common.RedisEnabled
	t.Cleanup(func() {
		DB = originalDB
		LOG_DB = originalLogDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.RedisEnabled = originalRedisEnabled
	})

	dbPath := filepath.Join(t.TempDir(), "legacy-token-oauth.db") + "?_pragma=busy_timeout(5000)"
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	sqlDB.SetMaxOpenConns(1)

	DB = db
	LOG_DB = db
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	require.NoError(t, db.AutoMigrate(&legacySQLiteTokenWithoutOAuthGrantID{}))
	require.NoError(t, db.Create(&legacySQLiteTokenWithoutOAuthGrantID{
		UserId:      10,
		Key:         "legacy-token-key",
		Status:      common.TokenStatusEnabled,
		Name:        "legacy",
		ExpiredTime: -1,
		Group:       "default",
		Source:      "",
	}).Error)

	require.NoError(t, migrateDB())

	require.True(t, db.Migrator().HasTable(&McpOAuthClient{}))
	require.True(t, db.Migrator().HasTable(&McpOAuthGrant{}))
	require.True(t, db.Migrator().HasTable(&McpOAuthAuthorizationCode{}))
	require.True(t, db.Migrator().HasTable(&McpOAuthRefreshToken{}))
	require.True(t, db.Migrator().HasColumn(&Token{}, "oauth_grant_id"))
	require.True(t, db.Migrator().HasIndex(&Token{}, "idx_tokens_o_auth_grant_id"))

	var migrated Token
	require.NoError(t, db.First(&migrated, "`key` = ?", "legacy-token-key").Error)
	require.Equal(t, 10, migrated.UserId)
	require.Nil(t, migrated.OAuthGrantId)

	firstGrantID := "grant_sqlite_unique"
	require.NoError(t, db.Create(&Token{
		UserId:       11,
		Key:          "oauth-token-one",
		Status:       common.TokenStatusEnabled,
		Name:         "oauth one",
		ExpiredTime:  -1,
		Group:        "default",
		OAuthGrantId: &firstGrantID,
	}).Error)
	duplicateGrantID := firstGrantID
	err = db.Create(&Token{
		UserId:       12,
		Key:          "oauth-token-two",
		Status:       common.TokenStatusEnabled,
		Name:         "oauth two",
		ExpiredTime:  -1,
		Group:        "default",
		OAuthGrantId: &duplicateGrantID,
	}).Error
	require.Error(t, err)

	require.NoError(t, db.Create(&Token{UserId: 13, Key: "null-grant-one", Status: common.TokenStatusEnabled, Name: "null one", ExpiredTime: -1, Group: "default"}).Error)
	require.NoError(t, db.Create(&Token{UserId: 14, Key: "null-grant-two", Status: common.TokenStatusEnabled, Name: "null two", ExpiredTime: -1, Group: "default"}).Error)
}

func TestSQLiteMigrateDBReplacesNonUniqueTokenOAuthGrantIDIndex(t *testing.T) {
	originalDB := DB
	originalLogDB := LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalRedisEnabled := common.RedisEnabled
	t.Cleanup(func() {
		DB = originalDB
		LOG_DB = originalLogDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.RedisEnabled = originalRedisEnabled
	})

	dbPath := filepath.Join(t.TempDir(), "legacy-token-oauth-nonunique.db") + "?_pragma=busy_timeout(5000)"
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	sqlDB.SetMaxOpenConns(1)

	DB = db
	LOG_DB = db
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	require.NoError(t, db.AutoMigrate(&legacySQLiteTokenWithoutOAuthGrantID{}))
	require.NoError(t, db.Exec("ALTER TABLE `tokens` ADD COLUMN `oauth_grant_id` varchar(64)").Error)
	require.NoError(t, db.Exec("CREATE INDEX `idx_tokens_o_auth_grant_id` ON `tokens`(`oauth_grant_id`)").Error)

	require.NoError(t, migrateDB())

	require.True(t, db.Migrator().HasColumn(&Token{}, "oauth_grant_id"))
	require.True(t, db.Migrator().HasIndex(&Token{}, "idx_tokens_o_auth_grant_id"))

	grantID := "grant_sqlite_repaired_unique"
	require.NoError(t, db.Create(&Token{
		UserId:       21,
		Key:          "oauth-token-repair-one",
		Status:       common.TokenStatusEnabled,
		Name:         "oauth repair one",
		ExpiredTime:  -1,
		Group:        "default",
		OAuthGrantId: &grantID,
	}).Error)
	duplicateGrantID := grantID
	err = db.Create(&Token{
		UserId:       22,
		Key:          "oauth-token-repair-two",
		Status:       common.TokenStatusEnabled,
		Name:         "oauth repair two",
		ExpiredTime:  -1,
		Group:        "default",
		OAuthGrantId: &duplicateGrantID,
	}).Error
	require.Error(t, err)
}

func TestSQLiteMigrateDBAddsTokenOAuthGrantIDWhenGlobalSQLiteFlagIsStale(t *testing.T) {
	originalDB := DB
	originalLogDB := LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalRedisEnabled := common.RedisEnabled
	t.Cleanup(func() {
		DB = originalDB
		LOG_DB = originalLogDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.RedisEnabled = originalRedisEnabled
	})

	dbPath := filepath.Join(t.TempDir(), "legacy-token-oauth-stale-flag.db") + "?_pragma=busy_timeout(5000)"
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	sqlDB.SetMaxOpenConns(1)

	DB = db
	LOG_DB = db
	common.UsingSQLite = false
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	require.NoError(t, db.AutoMigrate(&legacySQLiteTokenWithoutOAuthGrantID{}))

	require.NoError(t, migrateDB())

	require.True(t, db.Migrator().HasColumn(&Token{}, "oauth_grant_id"))
	require.True(t, db.Migrator().HasIndex(&Token{}, "idx_tokens_o_auth_grant_id"))
}
