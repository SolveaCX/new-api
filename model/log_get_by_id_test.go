package model

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetLogByIDReadsExactSourceRowAndReportsMissing(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	require.NoError(t, db.Create(&Log{RequestId: "req-source", Other: `{"supplier_accounting_v1":{"disposition":"captured"}}`}).Error)
	var expected Log
	require.NoError(t, db.Where("request_id = ?", "req-source").First(&expected).Error)

	actual, err := GetLogByID(context.Background(), db, expected.Id)
	require.NoError(t, err)
	require.Equal(t, expected.Id, actual.Id)
	require.Equal(t, expected.RequestId, actual.RequestId)
	require.Equal(t, expected.Other, actual.Other)

	_, err = GetLogByID(context.Background(), db, expected.Id+1)
	require.ErrorIs(t, err, ErrLogNotFound)
}
