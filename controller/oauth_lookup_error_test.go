package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestFindOrCreateOAuthUserStopsWhenProviderIdentityLookupFails(t *testing.T) {
	db := setupModelListControllerTestDB(t)

	originalRegisterEnabled := common.RegisterEnabled
	t.Cleanup(func() { common.RegisterEnabled = originalRegisterEnabled })
	common.RegisterEnabled = true

	expectedErr := errors.New("forced provider identity lookup failure")
	callbackName := "force_oauth_identity_lookup_failure"
	injected := false
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if !injected && tx.Statement.Table == "users" {
			injected = true
			tx.AddError(expectedErr)
		}
	}))
	t.Cleanup(func() {
		require.NoError(t, db.Callback().Query().Remove(callbackName))
	})

	var resultUser *model.User
	var resultIsNew bool
	var resultErr error

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("oauth-lookup-error"))))
	router.GET("/oauth-lookup-error", func(c *gin.Context) {
		resultUser, resultIsNew, resultErr = findOrCreateOAuthUser(c, &oauth.GoogleProvider{}, &oauth.OAuthUser{
			ProviderUserID: "google-sub-query-error",
			Username:       "google-query-error",
			Email:          "google-query-error@example.com",
			Extra:          map[string]any{},
		}, sessions.Default(c))
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/oauth-lookup-error", nil))

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.True(t, injected)
	require.Nil(t, resultUser)
	require.False(t, resultIsNew)
	require.ErrorIs(t, resultErr, expectedErr)

	var users int64
	require.NoError(t, db.Model(&model.User{}).Where("google_id = ?", "google-sub-query-error").Count(&users).Error)
	require.Zero(t, users)
}

func TestFindOrCreateOAuthUserRejectsSoftDeletedLegacyIdentity(t *testing.T) {
	db := setupModelListControllerTestDB(t)

	originalRegisterEnabled := common.RegisterEnabled
	t.Cleanup(func() { common.RegisterEnabled = originalRegisterEnabled })
	common.RegisterEnabled = true

	deletedUser := &model.User{
		Username: "github-soft-deleted",
		GitHubId: "github-legacy-login",
		AffCode:  "github-soft-deleted",
	}
	require.NoError(t, db.Create(deletedUser).Error)
	require.NoError(t, db.Delete(deletedUser).Error)

	var resultUser *model.User
	var resultIsNew bool
	var resultErr error

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("oauth-legacy-soft-delete"))))
	router.GET("/oauth-legacy-soft-delete", func(c *gin.Context) {
		resultUser, resultIsNew, resultErr = findOrCreateOAuthUser(c, oauthLanguageTestProvider{}, &oauth.OAuthUser{
			ProviderUserID: "github-numeric-id",
			Username:       "github-new-user",
			Email:          "github-new-user@example.com",
			Extra: map[string]any{
				"legacy_id": "github-legacy-login",
			},
		}, sessions.Default(c))
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/oauth-legacy-soft-delete", nil))

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Nil(t, resultUser)
	require.False(t, resultIsNew)
	var deletedErr *OAuthUserDeletedError
	require.ErrorAs(t, resultErr, &deletedErr)

	var newUsers int64
	require.NoError(t, db.Unscoped().Model(&model.User{}).Where("github_id = ?", "github-numeric-id").Count(&newUsers).Error)
	require.Zero(t, newUsers)
}
