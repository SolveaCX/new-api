package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func enablePaymentComplianceForSubscriptionControllerTest(t *testing.T) {
	t.Helper()
	paymentSetting := operation_setting.GetPaymentSetting()
	originalConfirmed := paymentSetting.ComplianceConfirmed
	originalVersion := paymentSetting.ComplianceTermsVersion
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	t.Cleanup(func() {
		paymentSetting.ComplianceConfirmed = originalConfirmed
		paymentSetting.ComplianceTermsVersion = originalVersion
	})
}

func setupSubscriptionControllerTestDB(t *testing.T) {
	t.Helper()
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalRedisEnabled := common.RedisEnabled
	originalUsingSQLite := common.UsingSQLite
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalUsingMySQL := common.UsingMySQL

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingPostgreSQL = false
	common.UsingMySQL = false

	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.RedisEnabled = originalRedisEnabled
		common.UsingSQLite = originalUsingSQLite
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.UsingMySQL = originalUsingMySQL
		require.NoError(t, sqlDB.Close())
	})

	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Log{},
		&model.SubscriptionPlan{},
		&model.SubscriptionOrder{},
		&model.UserSubscription{},
		&model.SubscriptionProviderBinding{},
		&model.UserSubscriptionContract{},
		&model.SubscriptionChangeIntent{},
	))
}

func TestChangeSubscriptionPlanRejectsInvalidRequestID(t *testing.T) {
	enablePaymentComplianceForSubscriptionControllerTest(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 901)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/subscription/self/change-plan",
		strings.NewReader(`{"plan_id":1,"payment_mode":"balance_one_period","request_id":"bad id"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	ChangeSubscriptionPlan(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":false`)
	require.Contains(t, recorder.Body.String(), "request_id")
}

func TestSubscriptionStripePayReturnsUnsupportedWithoutLegacyState(t *testing.T) {
	enablePaymentComplianceForSubscriptionControllerTest(t)
	setupSubscriptionControllerTestDB(t)
	rank := 1
	require.NoError(t, model.DB.Create(&model.User{
		Id:       902,
		Username: "stripe_change_user",
		Status:   common.UserStatusEnabled,
		Group:    "plg",
		AffCode:  "stripe_change_aff",
	}).Error)
	require.NoError(t, model.DB.Create(&model.SubscriptionPlan{
		Id:            9902,
		Title:         "Stripe Change Plan",
		PriceAmount:   9.99,
		Currency:      "USD",
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		TierRank:      &rank,
		TotalAmount:   1000,
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 902)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/subscription/stripe/pay",
		strings.NewReader(`{"plan_id":9902,"request_id":"stripe-request-1"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	SubscriptionRequestStripePay(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":false`)
	require.Contains(t, recorder.Body.String(), "pending migration")
	var orderCount int64
	require.NoError(t, model.DB.Model(&model.SubscriptionOrder{}).Where("user_id = ?", 902).Count(&orderCount).Error)
	require.Zero(t, orderCount)
	var entitlementCount int64
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("user_id = ?", 902).Count(&entitlementCount).Error)
	require.Zero(t, entitlementCount)
	var intentCount int64
	require.NoError(t, model.DB.Model(&model.SubscriptionChangeIntent{}).Where("user_id = ?", 902).Count(&intentCount).Error)
	require.Zero(t, intentCount)
}
