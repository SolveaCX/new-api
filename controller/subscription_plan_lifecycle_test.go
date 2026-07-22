package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func setupSubscriptionPlanControllerLifecycleTestDB(t *testing.T) {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalUsingMySQL := common.UsingMySQL

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(5)

	model.DB = db
	model.LOG_DB = db
	common.UsingSQLite = true
	common.UsingPostgreSQL = false
	common.UsingMySQL = false
	require.NoError(t, model.DB.AutoMigrate(
		&model.User{},
		&model.Log{},
		&model.TopUp{},
		&model.SubscriptionPlan{},
		&model.SubscriptionOrder{},
		&model.UserSubscription{},
		&model.SubscriptionProviderBinding{},
		&model.PaymentWebhookEvent{},
		&model.UserSubscriptionContract{},
		&model.SubscriptionChangeIntent{},
		&model.SubscriptionTierRankReservation{},
	))

	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.UsingMySQL = originalUsingMySQL
		require.NoError(t, sqlDB.Close())
	})
}

func confirmSubscriptionPlanPaymentComplianceForTest(t *testing.T) {
	t.Helper()
	paymentSetting := operation_setting.GetPaymentSetting()
	originalConfirmed := paymentSetting.ComplianceConfirmed
	originalTermsVersion := paymentSetting.ComplianceTermsVersion
	t.Cleanup(func() {
		paymentSetting.ComplianceConfirmed = originalConfirmed
		paymentSetting.ComplianceTermsVersion = originalTermsVersion
	})
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
}

func TestAdminUpdateSubscriptionPlanStatusUsesLifecycleValidation(t *testing.T) {
	setupSubscriptionPlanControllerLifecycleTestDB(t)
	confirmSubscriptionPlanPaymentComplianceForTest(t)
	gin.SetMode(gin.TestMode)

	legacy := &model.SubscriptionPlan{
		Title:         "Legacy nil rank",
		PriceAmount:   9.99,
		Currency:      "USD",
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       false,
		TotalAmount:   1000,
	}
	require.NoError(t, model.DB.Select("*").Create(legacy).Error)
	require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", legacy.Id).Update("enabled", false).Error)

	body, err := json.Marshal(AdminUpdateSubscriptionPlanStatusRequest{Enabled: common.GetPointer(true)})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(legacy.Id)}}
	ctx.Request = httptest.NewRequest(http.MethodPost, "/subscription/plans/"+strconv.Itoa(legacy.Id)+"/status", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	AdminUpdateSubscriptionPlanStatus(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var stored model.SubscriptionPlan
	require.NoError(t, model.DB.First(&stored, "id = ?", legacy.Id).Error)
	require.False(t, stored.Enabled)
	var reservations int64
	require.NoError(t, model.DB.Model(&model.SubscriptionTierRankReservation{}).Count(&reservations).Error)
	require.Zero(t, reservations)
}
