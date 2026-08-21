package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestRegistrationRiskHashesAreStableAndDoNotExposeRawSignals(t *testing.T) {
	oldSecret := common.CryptoSecret
	common.CryptoSecret = "registration-risk-test-secret"
	t.Cleanup(func() { common.CryptoSecret = oldSecret })

	deviceHash := HashRegistrationDeviceID("device-123")
	require.Len(t, deviceHash, 64)
	require.NotContains(t, deviceHash, "device-123")
	require.Equal(t, deviceHash, HashRegistrationDeviceID("device-123"))
	require.Equal(t, HashRegistrationEmail("USER@Example.com"), HashRegistrationEmail("user@example.com"))
	require.Equal(t, HashRegistrationIP("::ffff:192.0.2.1"), HashRegistrationIP("192.0.2.1"))
}

func TestAssessRegistrationRiskUsesConfiguredBenefitThresholds(t *testing.T) {
	oldDB := DB
	oldRedisEnabled := common.RedisEnabled
	oldRDB := common.RDB
	DB = nil
	common.RedisEnabled = false
	common.RDB = nil
	t.Cleanup(func() {
		DB = oldDB
		common.RedisEnabled = oldRedisEnabled
		common.RDB = oldRDB
	})

	decision := AssessRegistrationRisk("device-123", "user@example.com", "192.0.2.1")
	require.Equal(t, 1, decision.Level)
	require.False(t, decision.BlockBenefits)
	require.False(t, decision.BlockTokens)
}

func TestRiskBlockedOnlyAffectsCommonUsers(t *testing.T) {
	require.True(t, IsUserRegistrationRiskBenefitsBlocked(&User{Role: common.RoleCommonUser, RegistrationRiskLevel: RegistrationRiskBenefits}))
	require.False(t, IsUserRegistrationRiskBenefitsBlocked(&User{Role: common.RoleAdminUser, RegistrationRiskLevel: RegistrationRiskBenefits}))
	require.True(t, IsUserRegistrationRiskTokenBlocked(&User{Role: common.RoleCommonUser, RegistrationRiskLevel: RegistrationRiskTokens}))
}
