package model

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestFillUserByGoogleId(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Create(&User{
		Username: "google_tester",
		GoogleId: "google-sub-123",
	}).Error)

	u := &User{GoogleId: "google-sub-123"}
	require.NoError(t, u.FillUserByGoogleId())
	require.Equal(t, "google_tester", u.Username)
}

func TestFillUserByGoogleId_EmptyId(t *testing.T) {
	u := &User{}
	require.Error(t, u.FillUserByGoogleId())
}

func TestFillUserByGoogleId_NotFound(t *testing.T) {
	truncateTables(t)
	// Mirrors OIDC behavior: a non-empty id with no matching row returns no error
	// and leaves the user as zero-value. Callers must gate with IsGoogleIdAlreadyTaken.
	u := &User{GoogleId: "does-not-exist"}
	require.NoError(t, u.FillUserByGoogleId())
	require.Zero(t, u.Id)
}

func TestFillUserByGoogleId_PropagatesDatabaseError(t *testing.T) {
	expectedErr := errors.New("forced google user lookup failure")
	callbackName := "force_google_user_lookup_failure"
	require.NoError(t, DB.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		tx.AddError(expectedErr)
	}))
	t.Cleanup(func() {
		require.NoError(t, DB.Callback().Query().Remove(callbackName))
	})

	u := &User{GoogleId: "google-sub-error"}
	require.ErrorIs(t, u.FillUserByGoogleId(), expectedErr)
}

func TestIsGoogleIdAlreadyTaken_SoftDeletedStillTaken(t *testing.T) {
	truncateTables(t)
	user := &User{Username: "g_soft", GoogleId: "google-sub-soft"}
	require.NoError(t, DB.Create(user).Error)
	// Soft-delete the user; the google_id must remain reserved (Unscoped).
	require.NoError(t, DB.Delete(user).Error)
	isTaken, err := IsGoogleIdAlreadyTaken("google-sub-soft")
	require.NoError(t, err)
	require.True(t, isTaken)
}

func TestIsGoogleIdAlreadyTaken(t *testing.T) {
	truncateTables(t)
	isTaken, err := IsGoogleIdAlreadyTaken("google-sub-456")
	require.NoError(t, err)
	require.False(t, isTaken)
	require.NoError(t, DB.Create(&User{Username: "g2", GoogleId: "google-sub-456"}).Error)
	isTaken, err = IsGoogleIdAlreadyTaken("google-sub-456")
	require.NoError(t, err)
	require.True(t, isTaken)
}

func TestIsGoogleIdAlreadyTaken_MultipleMatchesStillTaken(t *testing.T) {
	truncateTables(t)
	for _, username := range []string{"google_duplicate_1", "google_duplicate_2", "google_duplicate_3"} {
		require.NoError(t, DB.Create(&User{Username: username, GoogleId: "google-sub-duplicate", AffCode: username}).Error)
	}

	isTaken, err := IsGoogleIdAlreadyTaken("google-sub-duplicate")
	require.NoError(t, err)
	require.True(t, isTaken)
}

func TestIsOidcIdAlreadyTaken_SoftDeletedStillTaken(t *testing.T) {
	truncateTables(t)
	user := &User{Username: "oidc_soft_deleted", OidcId: "oidc-sub-soft", AffCode: "oidc-soft"}
	require.NoError(t, DB.Create(user).Error)
	require.NoError(t, DB.Delete(user).Error)

	isTaken, err := IsOidcIdAlreadyTaken("oidc-sub-soft")
	require.NoError(t, err)
	require.True(t, isTaken)
}
