package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCliDeviceAuthorizationTestDB(t *testing.T) {
	t.Helper()
	previousDB := DB
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/cli-device-authorization.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &Token{}, &CliDeviceAuthorization{}))
	DB = db
	t.Cleanup(func() {
		DB = previousDB
	})
}

func TestApproveCliDeviceAuthorizationWithTokenReusesSameDeviceToken(t *testing.T) {
	setupCliDeviceAuthorizationTestDB(t)
	user := User{Username: "cli-user", Password: "password"}
	require.NoError(t, DB.Create(&user).Error)
	auth := CliDeviceAuthorization{
		DeviceCodeHash: "device-code-hash",
		UserCodeHash:   "user-code-hash",
		Status:         CliDeviceAuthorizationStatusPending,
		ClientName:     "flatkey-cli",
		ClientVersion:  "0.1.0",
		DeviceIdHash:   "device-id-hash",
		CreatedTime:    100,
		ExpiresAt:      700,
	}
	require.NoError(t, CreateCliDeviceAuthorization(&auth))

	first, err := ApproveCliDeviceAuthorizationWithToken(
		auth.Id,
		user.Id,
		Token{
			Name:             "Flatkey CLI",
			Key:              "first-key",
			Status:           common.TokenStatusEnabled,
			CreatedTime:      101,
			AccessedTime:     101,
			ExpiredTime:      -1,
			UnlimitedQuota:   true,
			Source:           TokenSourceCLI,
			DeviceIdHash:     "device-id-hash",
			ClientName:       "flatkey-cli",
			ClientVersion:    "0.1.0",
			LastUsedClientAt: 101,
		},
		10,
		InviteRewardTriggerManualTokenCreate,
		101,
	)
	require.NoError(t, err)
	require.True(t, first.AuthorizationUpdated)
	require.True(t, first.TokenCreated)
	require.Equal(t, CliDeviceAuthorizationStatusApproved, first.Authorization.Status)
	require.Equal(t, first.Token.Id, first.Authorization.TokenId)

	secondAuth := CliDeviceAuthorization{
		DeviceCodeHash: "device-code-hash-2",
		UserCodeHash:   "user-code-hash-2",
		Status:         CliDeviceAuthorizationStatusPending,
		ClientName:     "flatkey-cli",
		ClientVersion:  "0.2.0",
		DeviceIdHash:   "device-id-hash",
		CreatedTime:    200,
		ExpiresAt:      800,
	}
	require.NoError(t, CreateCliDeviceAuthorization(&secondAuth))
	second, err := ApproveCliDeviceAuthorizationWithToken(
		secondAuth.Id,
		user.Id,
		Token{
			Name:             "Flatkey CLI",
			Key:              "second-key",
			Status:           common.TokenStatusEnabled,
			CreatedTime:      201,
			AccessedTime:     201,
			ExpiredTime:      -1,
			UnlimitedQuota:   true,
			Source:           TokenSourceCLI,
			DeviceIdHash:     "device-id-hash",
			ClientName:       "flatkey-cli",
			ClientVersion:    "0.2.0",
			LastUsedClientAt: 201,
		},
		10,
		InviteRewardTriggerManualTokenCreate,
		201,
	)
	require.NoError(t, err)
	require.True(t, second.AuthorizationUpdated)
	require.False(t, second.TokenCreated)
	require.Equal(t, first.Token.Id, second.Token.Id)
	require.Equal(t, "first-key", second.Token.Key)
	require.Equal(t, "0.2.0", second.Token.ClientVersion)
	require.Equal(t, int64(201), second.Token.LastUsedClientAt)

	var tokenCount int64
	require.NoError(t, DB.Model(&Token{}).Count(&tokenCount).Error)
	require.Equal(t, int64(1), tokenCount)
}
