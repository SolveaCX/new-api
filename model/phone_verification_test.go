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
	require.NoError(t, DB.AutoMigrate(&User{}))
	require.NoError(t, DB.Create(&User{
		Username:    "phone-owner",
		Password:    "hashed-password",
		PhoneNumber: "+8613800138000",
	}).Error)

	require.True(t, IsPhoneAlreadyTaken("+86 138-0013-8000"))
	require.False(t, IsPhoneAlreadyTaken("+14155552671"))
	_, err = common.NormalizePhoneNumber("+8613800138000")
	require.NoError(t, err)
}
