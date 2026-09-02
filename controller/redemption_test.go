package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRedemptionControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	require.NoError(t, i18n.Init())

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db

	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Redemption{}))
	return db
}

func confirmRedemptionPaymentComplianceForTest(t *testing.T) {
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

func TestAddRedemptionAllowsTwoThousandCodes(t *testing.T) {
	setupRedemptionControllerTestDB(t)
	confirmRedemptionPaymentComplianceForTest(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 9101, Username: "redeem-admin", AffCode: "redeem-admin"}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/redemption/", map[string]any{
		"name":         "YCPrompt",
		"quota":        100,
		"expired_time": 0,
		"count":        2000,
	}, 9101)

	AddRedemption(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool     `json:"success"`
		Message string   `json:"message"`
		Data    []string `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data, 2000)

	var count int64
	require.NoError(t, model.DB.Model(&model.Redemption{}).Where("name = ?", "YCPrompt").Count(&count).Error)
	require.EqualValues(t, 2000, count)
}

func TestAddRedemptionRejectsOverTwoThousandCodes(t *testing.T) {
	setupRedemptionControllerTestDB(t)
	confirmRedemptionPaymentComplianceForTest(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 9102, Username: "redeem-admin-2", AffCode: "redeem-admin-2"}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/redemption/", map[string]any{
		"name":         "YCPrompt",
		"quota":        100,
		"expired_time": 0,
		"count":        2001,
	}, 9102)

	AddRedemption(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.False(t, response.Success)
	require.Equal(t, i18n.Translate(i18n.LangEn, i18n.MsgRedemptionCountMax), response.Message)

	var count int64
	require.NoError(t, model.DB.Model(&model.Redemption{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestAddRedemptionUsesProvidedQuotaAndPurpose(t *testing.T) {
	setupRedemptionControllerTestDB(t)
	confirmRedemptionPaymentComplianceForTest(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 9103, Username: "redeem-admin-3", AffCode: "redeem-admin-3"}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/redemption/", map[string]any{
		"name":         "YCPrompt",
		"quota":        777,
		"expired_time": 0,
		"count":        1,
	}, 9103)

	AddRedemption(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool     `json:"success"`
		Message string   `json:"message"`
		Data    []string `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data, 1)

	var stored model.Redemption
	require.NoError(t, model.DB.First(&stored, "name = ?", "YCPrompt").Error)
	require.Equal(t, 777, stored.Quota)
	require.Equal(t, 9103, stored.UserId)
	require.Equal(t, "YCPrompt", stored.Name)
	require.NotEmpty(t, stored.Key)
}
