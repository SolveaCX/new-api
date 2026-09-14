package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestIsPhoneAlreadyTakenUsesNormalizedE164Value(t *testing.T) {
	originalDB := DB
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	t.Cleanup(func() { DB = originalDB })
	require.NoError(t, DB.AutoMigrate(&User{}, &UserPhoneBinding{}))
	require.NoError(t, DB.Create(&User{
		Username:    "phone-owner",
		Password:    "hashed-password",
		PhoneNumber: "+8613800138000",
	}).Error)

	require.NoError(t, DB.Create(&UserPhoneBinding{PhoneNumber: "+8613800138000", UserID: 1, VerifiedAt: 1}).Error)
	taken, err := IsPhoneAlreadyTaken("+86 138-0013-8000")
	require.NoError(t, err)
	require.True(t, taken)
	taken, err = IsPhoneAlreadyTaken("+14155552671")
	require.NoError(t, err)
	require.False(t, taken)
	_, err = common.NormalizePhoneNumber("+8613800138000")
	require.NoError(t, err)
}

func TestReserveUserPhoneRejectsDuplicateAcrossUsers(t *testing.T) {
	originalDB := DB
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	t.Cleanup(func() { DB = originalDB })
	require.NoError(t, DB.AutoMigrate(&User{}, &UserPhoneBinding{}))
	require.NoError(t, DB.Create(&User{Username: "phone-owner", Password: "hashed-password", AffCode: "owner"}).Error)
	require.NoError(t, DB.Create(&User{Username: "phone-other", Password: "hashed-password", AffCode: "other"}).Error)
	require.NoError(t, ReserveUserPhone("+14155552671", 1, 1))
	require.ErrorIs(t, ReserveUserPhone("+14155552671", 2, 2), ErrPhoneAlreadyTaken)
}

func TestInsertUserWithPhoneRollsBackOnDuplicateBinding(t *testing.T) {
	originalDB := DB
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	t.Cleanup(func() { DB = originalDB })
	require.NoError(t, DB.AutoMigrate(&User{}, &UserPhoneBinding{}))
	first := &User{Username: "phone-first", Password: "hashed-password", PhoneNumber: "+14155552671", AffCode: "first"}
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		require.NoError(t, tx.Create(first).Error)
		return reserveUserPhoneTx(tx, first.PhoneNumber, first.Id, 1)
	}))
	second := &User{Username: "phone-second", Password: "hashed-password", PhoneNumber: "+14155552671", AffCode: "second"}
	require.ErrorIs(t, DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(second).Error; err != nil {
			return err
		}
		return reserveUserPhoneTx(tx, second.PhoneNumber, second.Id, 1)
	}), ErrPhoneAlreadyTaken)
	var count int64
	require.NoError(t, DB.Model(&User{}).Where("username = ?", "phone-second").Count(&count).Error)
	require.Zero(t, count)
}
