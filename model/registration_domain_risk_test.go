package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRegistrationDomainRiskTest(t *testing.T) {
	t.Helper()
	originalDB := DB
	originalLogDB := LOG_DB
	originalRedis := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/registration-risk.db?_pragma=busy_timeout(5000)"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	LOG_DB = db
	common.RedisEnabled = false
	require.NoError(t, db.AutoMigrate(&User{}, &RegistrationDomainState{}, &RegistrationDomainBlock{}, &RegistrationDomainBlockUser{}))
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
		DB = originalDB
		LOG_DB = originalLogDB
		common.RedisEnabled = originalRedis
	})
}

func seedDomainRiskUsers(t *testing.T, domain string, enabled int, disabled int, createdAt int64) []User {
	t.Helper()
	users := make([]User, 0, enabled+disabled)
	for i := 0; i < enabled+disabled; i++ {
		status := common.UserStatusEnabled
		if i >= enabled {
			status = common.UserStatusDisabled
		}
		user := User{
			Username:    fmt.Sprintf("risk-user-%d", i),
			AffCode:     fmt.Sprintf("r%03d", i),
			Password:    "hashed",
			Email:       fmt.Sprintf("user-%d@%s", i, domain),
			EmailDomain: domain,
			Status:      status,
			Role:        common.RoleCommonUser,
			CreatedAt:   createdAt,
		}
		require.NoError(t, DB.Create(&user).Error)
		users = append(users, user)
	}
	return users
}

func TestRegisterUserWithDomainRiskThresholdBlocksAndDisables(t *testing.T) {
	setupRegistrationDomainRiskTest(t)
	now := time.Now().Unix()
	seedDomainRiskUsers(t, "farm.example", 9, 0, now-60)
	candidate := User{Username: "trigger", Password: "password123", Email: "trigger@farm.example", EmailDomain: "farm.example", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}

	result, err := RegisterUserWithDomainRisk(&candidate, 0, "203.0.113.10", RegistrationDomainRiskPolicy{
		Enabled: true, Window: 24 * time.Hour, Threshold: 10, Now: now,
	}, nil)

	require.ErrorIs(t, err, ErrRegistrationDomainBlocked)
	require.True(t, result.Triggered)
	require.Zero(t, candidate.Id)
	var enabledCount int64
	require.NoError(t, DB.Model(&User{}).Where("email_domain = ? AND status = ?", "farm.example", common.UserStatusEnabled).Count(&enabledCount).Error)
	require.Zero(t, enabledCount)
	var block RegistrationDomainBlock
	require.NoError(t, DB.Where("domain = ?", "farm.example").First(&block).Error)
	require.Equal(t, 10, block.ObservedCount)
	var affected int64
	require.NoError(t, DB.Model(&RegistrationDomainBlockUser{}).Where("block_id = ?", block.Id).Count(&affected).Error)
	require.EqualValues(t, 9, affected)
}

func TestRegisterUserWithDomainRiskPersistsNormalizedDomainBelowThreshold(t *testing.T) {
	setupRegistrationDomainRiskTest(t)
	now := time.Now().Unix()
	candidate := User{Username: "allowed", Password: "password123", Email: "Allowed@Example.COM", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}

	result, err := RegisterUserWithDomainRisk(&candidate, 0, "203.0.113.11", RegistrationDomainRiskPolicy{
		Enabled: true, Window: 24 * time.Hour, Threshold: 10, Now: now,
	}, nil)

	require.NoError(t, err)
	require.False(t, result.Triggered)
	require.NotZero(t, candidate.Id)
	var stored User
	require.NoError(t, DB.First(&stored, candidate.Id).Error)
	require.Equal(t, "example.com", stored.EmailDomain)
	require.Equal(t, "203.0.113.11", stored.RegistrationIP)
}

func TestReleaseRegistrationDomainBlockRestoresOnlyAutomatedDisables(t *testing.T) {
	setupRegistrationDomainRiskTest(t)
	now := time.Now().Unix()
	users := seedDomainRiskUsers(t, "restore.example", 2, 1, now-60)
	candidate := User{Username: "restore-trigger", Password: "password123", Email: "trigger@restore.example", EmailDomain: "restore.example", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	result, err := RegisterUserWithDomainRisk(&candidate, 0, "203.0.113.12", RegistrationDomainRiskPolicy{
		Enabled: true, Window: 24 * time.Hour, Threshold: 4, Now: now,
	}, nil)
	require.ErrorIs(t, err, ErrRegistrationDomainBlocked)

	release, err := ReleaseRegistrationDomainBlock(result.BlockID, 99, true, now+10)
	require.NoError(t, err)
	require.Equal(t, int64(2), release.RestoredUsers)
	var restored []User
	require.NoError(t, DB.Order("id asc").Find(&restored).Error)
	require.Equal(t, common.UserStatusEnabled, restored[0].Status)
	require.Equal(t, common.UserStatusEnabled, restored[1].Status)
	require.Equal(t, common.UserStatusDisabled, restored[2].Status)
	require.Equal(t, users[2].Id, restored[2].Id)

	repeated, err := ReleaseRegistrationDomainBlock(result.BlockID, 99, true, now+20)
	require.NoError(t, err)
	require.Zero(t, repeated.RestoredUsers)
	var state RegistrationDomainState
	require.NoError(t, DB.Where("domain = ?", "restore.example").First(&state).Error)
	require.Zero(t, state.ActiveBlockID)
	require.Equal(t, now+10, state.CountingSince)
}
