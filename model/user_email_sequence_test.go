package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupEmailSeqTestDB(t *testing.T) {
	t.Helper()

	originalDB := DB
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&User{}, &UserEmailSequence{}, &TopUp{}))

	DB = db
	t.Cleanup(func() {
		DB = originalDB
	})
}

func TestRecordEmailSequenceSent_Idempotent(t *testing.T) {
	setupEmailSeqTestDB(t)

	// 首次记录成功
	ok, err := RecordEmailSequenceSent(1001, 1)
	require.NoError(t, err)
	require.True(t, ok, "首次记录应返回 true")

	// 同 (user,step) 再记录:唯一约束挡住,返回 false
	ok, err = RecordEmailSequenceSent(1001, 1)
	require.NoError(t, err)
	require.False(t, ok, "重复记录应返回 false,绝不重发")
}

func TestGetSentSteps(t *testing.T) {
	setupEmailSeqTestDB(t)
	_, _ = RecordEmailSequenceSent(2001, 1)
	_, _ = RecordEmailSequenceSent(2001, 3)

	steps, err := GetSentSteps(2001)
	require.NoError(t, err)
	require.ElementsMatch(t, []int{1, 3}, steps)
}

func TestHasSentStepWithinWindow(t *testing.T) {
	setupEmailSeqTestDB(t)
	_, _ = RecordEmailSequenceSent(3001, 3)

	// step 3 在窗口内(刚发)
	within, sentAt, err := HasSentStepWithinWindow(3001, 3, 7*24*3600)
	require.NoError(t, err)
	require.True(t, within)
	require.Greater(t, sentAt, int64(0))

	// step 4 没发过
	within, _, err = HasSentStepWithinWindow(3001, 4, 7*24*3600)
	require.NoError(t, err)
	require.False(t, within)
}

func TestSetUserEmailOptOut(t *testing.T) {
	setupEmailSeqTestDB(t)

	u := &User{Username: "optouttest", Email: "opt@example.com", Password: "x"}
	require.NoError(t, DB.Create(u).Error)
	require.False(t, u.EmailOptOut)

	require.NoError(t, SetUserEmailOptOut(u.Id))

	var reloaded User
	require.NoError(t, DB.First(&reloaded, u.Id).Error)
	require.True(t, reloaded.EmailOptOut, "退订后该列应为 true")
}
