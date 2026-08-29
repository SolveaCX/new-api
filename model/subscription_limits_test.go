package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestStandardSubscriptionPlanLimitsMatchProductContract(t *testing.T) {
	got := StandardSubscriptionPlanLimits()
	require.Equal(t, []StandardSubscriptionPlanLimit{
		{Title: "Go", PriceUSD: 10, Window5hUSD: 8, WindowWeekUSD: 12, MonthlyUSD: 25},
		{Title: "Pro", PriceUSD: 30, Window5hUSD: 18, WindowWeekUSD: 45, MonthlyUSD: 90},
		{Title: "Max", PriceUSD: 100, Window5hUSD: 78, WindowWeekUSD: 220, MonthlyUSD: 450},
	}, got)
}

func TestMigrateStandardSubscriptionPlanLimitsRestoresExistingRows(t *testing.T) {
	originalDB := DB
	originalUsingSQLite := common.UsingSQLite
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		DB = originalDB
		common.UsingSQLite = originalUsingSQLite
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&Option{}, &SubscriptionPlan{}))
	DB = db
	common.UsingSQLite = true
	common.QuotaPerUnit = 1000

	goPlan := &SubscriptionPlan{Title: "Go", PriceAmount: 10, Currency: "USD", TotalAmount: 1}
	proPlan := &SubscriptionPlan{Title: " pro ", PriceAmount: 30, Currency: "usd", TotalAmount: 2}
	maxPlan := &SubscriptionPlan{Title: "Max", PriceAmount: 100, Currency: "USD", TotalAmount: 3}
	customPlan := &SubscriptionPlan{Title: "Custom", PriceAmount: 10, Currency: "USD", TotalAmount: 123}
	for _, plan := range []*SubscriptionPlan{goPlan, proPlan, maxPlan, customPlan} {
		require.NoError(t, db.Create(plan).Error)
	}

	require.NoError(t, migrateStandardSubscriptionPlanLimits())
	var gotGo, gotPro, gotCustom SubscriptionPlan
	require.NoError(t, db.First(&gotGo, goPlan.Id).Error)
	require.Equal(t, int64(8000), gotGo.Window5hAmount)
	require.Equal(t, int64(12000), gotGo.WindowWeekAmount)
	require.Equal(t, int64(25000), gotGo.TotalAmount)
	require.Equal(t, SubscriptionResetMonthly, gotGo.QuotaResetPeriod)
	require.NoError(t, db.First(&gotPro, proPlan.Id).Error)
	require.Equal(t, int64(18000), gotPro.Window5hAmount)
	require.Equal(t, int64(45000), gotPro.WindowWeekAmount)
	require.Equal(t, int64(90000), gotPro.TotalAmount)
	var gotMax SubscriptionPlan
	require.NoError(t, db.First(&gotMax, maxPlan.Id).Error)
	require.Equal(t, int64(78000), gotMax.Window5hAmount)
	require.Equal(t, int64(220000), gotMax.WindowWeekAmount)
	require.Equal(t, int64(450000), gotMax.TotalAmount)
	require.NoError(t, db.First(&gotCustom, customPlan.Id).Error)
	require.Equal(t, int64(123), gotCustom.TotalAmount)
	require.Zero(t, gotCustom.Window5hAmount)

	var marker Option
	require.NoError(t, db.Where(&Option{Key: subscriptionStandardLimitsMigrationKey}).First(&marker).Error)
	// The marker makes a restart idempotent and preserves later operator edits.
	require.NoError(t, db.Model(&SubscriptionPlan{}).Where("id = ?", goPlan.Id).Update("window_5h_amount", 7).Error)
	require.NoError(t, migrateStandardSubscriptionPlanLimits())
	require.NoError(t, db.First(&gotGo, goPlan.Id).Error)
	require.Equal(t, int64(7), gotGo.Window5hAmount)
}

func TestMigrateStandardSubscriptionPlanLimitsUsesPersistedQuotaUnit(t *testing.T) {
	originalDB := DB
	originalUsingSQLite := common.UsingSQLite
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		DB = originalDB
		common.UsingSQLite = originalUsingSQLite
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&Option{}, &SubscriptionPlan{}))
	DB = db
	common.UsingSQLite = true
	common.QuotaPerUnit = 500000
	require.NoError(t, db.Create(&Option{Key: "QuotaPerUnit", Value: "1000"}).Error)
	plan := &SubscriptionPlan{Title: "Go", PriceAmount: 10, Currency: "USD"}
	require.NoError(t, db.Create(plan).Error)

	require.NoError(t, migrateStandardSubscriptionPlanLimits())
	var got SubscriptionPlan
	require.NoError(t, db.First(&got, plan.Id).Error)
	require.Equal(t, int64(8000), got.Window5hAmount)
	require.Equal(t, int64(12000), got.WindowWeekAmount)
	require.Equal(t, int64(25000), got.TotalAmount)
}

func TestMigrateStandardSubscriptionPlanLimitsRecognizesStagingTestPrefix(t *testing.T) {
	originalDB := DB
	originalUsingSQLite := common.UsingSQLite
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		DB = originalDB
		common.UsingSQLite = originalUsingSQLite
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&Option{}, &SubscriptionPlan{}))
	DB = db
	common.UsingSQLite = true
	common.QuotaPerUnit = 1000

	// A staging database may already have applied the previous contract marker
	// before its test plans were created or renamed with the [TEST] prefix.
	require.NoError(t, db.Create(&Option{Key: "subscription_standard_limits_v2", Value: "applied"}).Error)
	plan := &SubscriptionPlan{
		Title:       "[TEST] Go",
		PriceAmount: 10,
		Currency:    "USD",
		TotalAmount: 45000,
	}
	require.NoError(t, db.Create(plan).Error)

	require.NoError(t, migrateStandardSubscriptionPlanLimits())
	var got SubscriptionPlan
	require.NoError(t, db.First(&got, plan.Id).Error)
	require.Equal(t, int64(8000), got.Window5hAmount)
	require.Equal(t, int64(12000), got.WindowWeekAmount)
	require.Equal(t, int64(25000), got.TotalAmount)

	var marker Option
	require.NoError(t, db.Where(&Option{Key: "subscription_standard_limits_v3"}).First(&marker).Error)
}
