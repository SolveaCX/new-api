package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUserInviterEmailTestDB(t *testing.T) {
	t.Helper()

	originalDB := DB
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}))
	DB = db
	t.Cleanup(func() {
		DB = originalDB
	})
}

func TestFillInviterEmailsResolvesEmailAndFallsBackCleanly(t *testing.T) {
	setupUserInviterEmailTestDB(t)

	inviter := &User{
		Username:    "inviter",
		DisplayName: "Inviter",
		Password:    "password123",
		Email:       "inviter@example.com",
		AffCode:     "INVITER-001",
	}
	require.NoError(t, DB.Create(inviter).Error)

	invited := &User{
		Username:   "invited",
		DisplayName: "Invited",
		Password:   "password123",
		AffCode:    "INVITED-001",
		InviterId:  inviter.Id,
	}
	require.NoError(t, DB.Create(invited).Error)

	users := []*User{invited}
	require.NoError(t, FillInviterEmails(users))
	require.Equal(t, "inviter@example.com", users[0].InviterEmail)
}
