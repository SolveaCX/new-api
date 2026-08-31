package model

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCustomerReferralOutboxDB(t *testing.T) {
	t.Helper()
	originalDB := DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &CustomerReferralOutbox{}))
	DB = db
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	t.Cleanup(func() {
		DB = originalDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
	})
}

func TestEnqueueCustomerReferralIncludesCustomerCreatedAt(t *testing.T) {
	setupCustomerReferralOutboxDB(t)
	const userCreatedAt = int64(1736899200)
	user := &User{
		Id:                             7844,
		Username:                       "jilin@example.com",
		DisplayName:                    "jilin",
		Status:                         common.UserStatusEnabled,
		CreatedAt:                      userCreatedAt,
		CustomerReferralInviteCode:     "invite-code",
		CustomerReferralSourcePlatform: "fluere",
	}
	require.NoError(t, DB.Create(user).Error)

	require.NoError(t, EnqueueCustomerReferralInTx(DB, user))
	var event CustomerReferralOutbox
	require.NoError(t, DB.First(&event, "event_id = ?", "flatkey-customer-created-7844").Error)
	var payload customerReferralPayload
	require.NoError(t, common.Unmarshal([]byte(event.Payload), &payload))

	require.Equal(t, "7844", payload.Customer.CustomerId)
	require.Equal(t, "2025-01-15T00:00:00.000Z", payload.Customer.CreatedAt)
	parsedCustomerCreatedAt, err := time.Parse(time.RFC3339Nano, payload.Customer.CreatedAt)
	require.NoError(t, err)
	require.Equal(t, time.UTC, parsedCustomerCreatedAt.Location())
	require.NotEqual(t, payload.Customer.CreatedAt, payload.CreatedAt, "event time must remain distinct from customer registration time")
	require.Equal(t, "flatkey-customer-created-7844", payload.EventId)
	require.NotContains(t, event.Payload, "jilin@example.com")
	require.NotContains(t, event.Payload, "sk-jilin-secret")
}

func TestEnqueueCustomerReferralRejectsMissingCustomerCreatedAt(t *testing.T) {
	setupCustomerReferralOutboxDB(t)
	user := &User{
		Id:                             7845,
		Username:                       "missing-created-at@example.com",
		DisplayName:                    "missing",
		Status:                         common.UserStatusEnabled,
		CustomerReferralInviteCode:     "invite-code",
		CustomerReferralSourcePlatform: "fluere",
	}
	err := EnqueueCustomerReferralInTx(DB, user)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrCustomerReferralCreatedAtMissing))
	var count int64
	require.NoError(t, DB.Model(&CustomerReferralOutbox{}).Where("user_id = ?", user.Id).Count(&count).Error)
	require.Zero(t, count)
	require.True(t, strings.Contains(ErrCustomerReferralCreatedAtMissing.Error(), "created_at"))
}
