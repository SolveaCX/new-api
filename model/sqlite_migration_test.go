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

func TestSQLiteMigrateDBCanRunTwiceOnSameDatabase(t *testing.T) {
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

	dbPath := filepath.Join(t.TempDir(), "startup.db") + "?_pragma=busy_timeout(5000)"
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

	require.NoError(t, migrateDB())
	require.NoError(t, migrateDB())

	require.True(t, db.Migrator().HasTable(&StripeBonusClaim{}))
	require.True(t, db.Migrator().HasIndex(&StripeBonusClaim{}, "idx_stripe_bonus_claims_card_fingerprint"))
	require.True(t, db.Migrator().HasTable(&TopUpBonusClaim{}))
	require.True(t, db.Migrator().HasIndex(&TopUpBonusClaim{}, "idx_topup_bonus_user_tier_seq"))

	require.True(t, db.Migrator().HasIndex(&SubscriptionTermSegment{}, "idx_subscription_term_order_segment"))
	assertSQLiteSubscriptionTermConstraints(t, db)
	assertSQLiteWalletLedgerConstraints(t, db)
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

func TestMigrationModelDescriptorsIncludeCriticalSQLiteModels(t *testing.T) {
	models := orderedMigrationModels()
	require.NotEmpty(t, models)
	require.Len(t, migrationModelValues(models), len(models))

	seen := map[string]bool{}
	for _, m := range models {
		require.NotEmpty(t, m.name)
		require.NotNil(t, m.model)
		require.False(t, seen[m.name], "duplicate migration model %s", m.name)
		seen[m.name] = true
	}

	require.True(t, seen["StripeBonusClaim"])
	require.True(t, seen["TopUpBonusClaim"])
	require.True(t, seen["SubscriptionOrder"])
	require.True(t, seen["SubscriptionTermSegment"])
	require.True(t, seen["WalletLedgerEntry"])
}

func assertSQLiteSubscriptionTermConstraints(t *testing.T, db *gorm.DB) {
	t.Helper()

	refundKey := "sqlite-refund-key-1"
	require.NoError(t, db.Create(&SubscriptionTermSegment{
		ContractId:     1001,
		OrderId:        2001,
		PlanId:         3001,
		SegmentIndex:   0,
		StartTime:      100,
		EndTime:        200,
		AllocatedMoney: 30,
		Status:         SubscriptionTermStatusActive,
		RefundKey:      &refundKey,
	}).Error)

	otherRefundKey := "sqlite-refund-key-2"
	err := db.Create(&SubscriptionTermSegment{
		ContractId:     1002,
		OrderId:        2001,
		PlanId:         3002,
		SegmentIndex:   0,
		StartTime:      200,
		EndTime:        300,
		AllocatedMoney: 30,
		Status:         SubscriptionTermStatusActive,
		RefundKey:      &otherRefundKey,
	}).Error
	require.Error(t, err)

	duplicateRefundKey := refundKey
	err = db.Create(&SubscriptionTermSegment{
		ContractId:     1003,
		OrderId:        2003,
		PlanId:         3003,
		SegmentIndex:   0,
		StartTime:      300,
		EndTime:        400,
		AllocatedMoney: 30,
		Status:         SubscriptionTermStatusActive,
		RefundKey:      &duplicateRefundKey,
	}).Error
	require.Error(t, err)
}

func assertSQLiteWalletLedgerConstraints(t *testing.T, db *gorm.DB) {
	t.Helper()

	require.NoError(t, db.Create(&WalletLedgerEntry{
		UserId:        10,
		EntryKey:      "sqlite-wallet-entry-1",
		QuotaDelta:    500,
		MoneyAmount:   15,
		EntryType:     WalletLedgerEntryTypePrepaidDebit,
		OrderId:       2001,
		TermSegmentId: 3001,
	}).Error)

	err := db.Create(&WalletLedgerEntry{
		UserId:        11,
		EntryKey:      "sqlite-wallet-entry-1",
		QuotaDelta:    500,
		MoneyAmount:   15,
		EntryType:     WalletLedgerEntryTypePrepaidDebit,
		OrderId:       2002,
		TermSegmentId: 3002,
	}).Error
	require.Error(t, err)
}
