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
		{Title: "Go", PriceUSD: 10, Window5hUSD: 10, WindowWeekUSD: 18, MonthlyUSD: 45},
		{Title: "Pro", PriceUSD: 30, Window5hUSD: 30, WindowWeekUSD: 60, MonthlyUSD: 90},
		{Title: "Max", PriceUSD: 100, Window5hUSD: 80, WindowWeekUSD: 240, MonthlyUSD: 300},
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
	// Existing installations may still have the previous migration marker; the
	// bumped key must allow the corrected contract to run once more.
	require.NoError(t, db.Create(&Option{Key: "subscription_standard_limits_v5", Value: "applied"}).Error)

	require.NoError(t, migrateStandardSubscriptionPlanLimits())
	var gotGo, gotPro, gotCustom SubscriptionPlan
	require.NoError(t, db.First(&gotGo, goPlan.Id).Error)
	require.Equal(t, int64(10000), gotGo.Window5hAmount)
	require.Equal(t, int64(18000), gotGo.WindowWeekAmount)
	require.Equal(t, int64(45000), gotGo.TotalAmount)
	require.Equal(t, SubscriptionResetMonthly, gotGo.QuotaResetPeriod)
	require.NoError(t, db.First(&gotPro, proPlan.Id).Error)
	require.Equal(t, int64(30000), gotPro.Window5hAmount)
	require.Equal(t, int64(60000), gotPro.WindowWeekAmount)
	require.Equal(t, int64(90000), gotPro.TotalAmount)
	var gotMax SubscriptionPlan
	require.NoError(t, db.First(&gotMax, maxPlan.Id).Error)
	require.Equal(t, int64(80000), gotMax.Window5hAmount)
	require.Equal(t, int64(240000), gotMax.WindowWeekAmount)
	require.Equal(t, int64(300000), gotMax.TotalAmount)
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
	require.Equal(t, int64(10000), got.Window5hAmount)
	require.Equal(t, int64(18000), got.WindowWeekAmount)
	require.Equal(t, int64(45000), got.TotalAmount)
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
	require.NoError(t, db.Create(&Option{Key: "subscription_standard_limits_v5", Value: "applied"}).Error)
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
	require.Equal(t, int64(10000), got.Window5hAmount)
	require.Equal(t, int64(18000), got.WindowWeekAmount)
	require.Equal(t, int64(45000), got.TotalAmount)

	var marker Option
	require.NoError(t, db.Where(&Option{Key: subscriptionStandardLimitsMigrationKey}).First(&marker).Error)
}

func TestMigrateStandardSubscriptionPlanLimitsContinuesPastDuplicateTier(t *testing.T) {
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
	require.NoError(t, db.AutoMigrate(&Option{}, &SubscriptionPlan{}, &UserSubscription{}))
	DB = db
	common.UsingSQLite = true
	common.QuotaPerUnit = 1000

	goPlan := &SubscriptionPlan{
		Title:       "[TEST] Go",
		PriceAmount: 10,
		Currency:    "USD",
		TotalAmount: 45000,
		Enabled:     true,
	}
	duplicateProA := &SubscriptionPlan{
		Title:       "[TEST] Pro",
		PriceAmount: 30,
		Currency:    "USD",
		TotalAmount: 90000,
		Enabled:     true,
	}
	duplicateProB := &SubscriptionPlan{
		Title:       "[TEST] Pro",
		PriceAmount: 30,
		Currency:    "USD",
		TotalAmount: 90000,
		Enabled:     true,
	}
	maxPlan := &SubscriptionPlan{
		Title:       "[TEST] Max",
		PriceAmount: 100,
		Currency:    "USD",
		TotalAmount: 300000,
		Enabled:     true,
	}
	for _, plan := range []*SubscriptionPlan{goPlan, duplicateProA, duplicateProB, maxPlan} {
		require.NoError(t, db.Create(plan).Error)
	}
	activeGo := &UserSubscription{
		PlanId:      goPlan.Id,
		AmountTotal: goPlan.TotalAmount,
		Status:      SubscriptionEntitlementStatusActive,
		EndTime:     9_999_999_999,
	}
	require.NoError(t, db.Create(activeGo).Error)

	require.NoError(t, migrateStandardSubscriptionPlanLimits())
	var gotGo, gotMax, gotPro SubscriptionPlan
	require.NoError(t, db.First(&gotGo, goPlan.Id).Error)
	require.NoError(t, db.First(&gotMax, maxPlan.Id).Error)
	require.NoError(t, db.First(&gotPro, duplicateProA.Id).Error)
	require.Equal(t, int64(10000), gotGo.Window5hAmount)
	require.Equal(t, int64(18000), gotGo.WindowWeekAmount)
	require.Equal(t, int64(45000), gotGo.TotalAmount)
	require.Equal(t, int64(80000), gotMax.Window5hAmount)
	require.Equal(t, int64(240000), gotMax.WindowWeekAmount)
	require.Equal(t, int64(300000), gotMax.TotalAmount)
	require.Equal(t, int64(90000), gotPro.TotalAmount)
	require.Zero(t, gotPro.Window5hAmount)
	require.Zero(t, gotPro.WindowWeekAmount)

	var updatedActiveGo UserSubscription
	require.NoError(t, db.First(&updatedActiveGo, activeGo.Id).Error)
	require.Equal(t, int64(45000), updatedActiveGo.AmountTotal)
	require.NotNil(t, updatedActiveGo.Window5hAmount)
	require.NotNil(t, updatedActiveGo.WindowWeekAmount)
	require.Equal(t, int64(10000), *updatedActiveGo.Window5hAmount)
	require.Equal(t, int64(18000), *updatedActiveGo.WindowWeekAmount)

	var marker Option
	require.ErrorIs(t, db.Where(&Option{Key: subscriptionStandardLimitsMigrationKey}).First(&marker).Error, gorm.ErrRecordNotFound)
}

func TestMigrateStandardSubscriptionPlanLimitsSelectsEnabledDuplicateTier(t *testing.T) {
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
	require.NoError(t, db.AutoMigrate(&Option{}, &SubscriptionPlan{}, &UserSubscription{}))
	DB = db
	common.UsingSQLite = true
	common.QuotaPerUnit = 1000

	require.NoError(t, db.Create(&Option{Key: "subscription_standard_limits_v5", Value: "applied"}).Error)
	duplicateGoA := &SubscriptionPlan{Title: "[TEST] Go", PriceAmount: 10, Currency: "USD", TotalAmount: 45000}
	duplicateGoB := &SubscriptionPlan{Title: "[TEST] Go", PriceAmount: 10, Currency: "USD", TotalAmount: 47000}
	require.NoError(t, db.Create(duplicateGoA).Error)
	require.NoError(t, db.Create(duplicateGoB).Error)
	require.NoError(t, db.Model(&SubscriptionPlan{}).Where("id = ?", duplicateGoA.Id).Update("enabled", false).Error)
	require.NoError(t, db.Model(&SubscriptionPlan{}).Where("id = ?", duplicateGoB.Id).Update("enabled", true).Error)
	activeGo := &UserSubscription{
		PlanId:      duplicateGoB.Id,
		AmountTotal: duplicateGoB.TotalAmount,
		Status:      SubscriptionEntitlementStatusActive,
		EndTime:     9_999_999_999,
	}
	require.NoError(t, db.Create(activeGo).Error)

	require.NoError(t, migrateStandardSubscriptionPlanLimits())

	var gotA, gotB SubscriptionPlan
	require.NoError(t, db.First(&gotA, duplicateGoA.Id).Error)
	require.NoError(t, db.First(&gotB, duplicateGoB.Id).Error)
	// The disabled historical duplicate is not the canonical tier row and is
	// intentionally left untouched.
	require.Zero(t, gotA.Window5hAmount)
	require.Zero(t, gotA.WindowWeekAmount)
	require.Equal(t, int64(45000), gotA.TotalAmount)
	require.Equal(t, int64(10000), gotB.Window5hAmount)
	require.Equal(t, int64(18000), gotB.WindowWeekAmount)
	require.Equal(t, int64(45000), gotB.TotalAmount)

	var updatedSubscription UserSubscription
	require.NoError(t, db.First(&updatedSubscription, activeGo.Id).Error)
	require.Equal(t, int64(45000), updatedSubscription.AmountTotal)
	require.NotNil(t, updatedSubscription.Window5hAmount)
	require.NotNil(t, updatedSubscription.WindowWeekAmount)
	require.Equal(t, int64(10000), *updatedSubscription.Window5hAmount)
	require.Equal(t, int64(18000), *updatedSubscription.WindowWeekAmount)

	var marker Option
	require.NoError(t, db.Where(&Option{Key: subscriptionStandardLimitsMigrationKey}).First(&marker).Error)
}
