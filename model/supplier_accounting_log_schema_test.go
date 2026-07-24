package model

import (
	"sort"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSupplierAccountingContractKeepsLogsPhysicalSchemaStable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))

	columnTypes, err := db.Migrator().ColumnTypes(&Log{})
	require.NoError(t, err)
	columns := make([]string, 0, len(columnTypes))
	for _, column := range columnTypes {
		columns = append(columns, column.Name())
	}
	sort.Strings(columns)
	expectedColumns := []string{
		"channel_id", "channel_name", "completion_tokens", "content", "created_at", "group", "id", "ip",
		"is_stream", "model_name", "other", "prompt_tokens", "quota", "request_id", "token_id", "token_name",
		"type", "upstream_request_id", "use_time", "user_id", "username",
	}
	sort.Strings(expectedColumns)
	require.Equal(t, expectedColumns, columns, "supplier accounting must remain embedded only in logs.other")

	var indexRows []struct {
		Name string `gorm:"column:name"`
	}
	require.NoError(t, db.Raw("PRAGMA index_list('logs')").Scan(&indexRows).Error)
	indexes := make([]string, 0, len(indexRows))
	for _, index := range indexRows {
		indexes = append(indexes, index.Name)
		require.False(t, strings.Contains(index.Name, "supplier"), "supplier accounting must not add a logs index")
	}
	sort.Strings(indexes)
	expectedIndexes := []string{
		"idx_created_at_id", "idx_created_at_type", "idx_logs_channel_id", "idx_logs_channel_type_created_id",
		"idx_logs_group", "idx_logs_ip", "idx_logs_model_name", "idx_logs_request_id", "idx_logs_token_id",
		"idx_logs_token_name", "idx_logs_upstream_request_id", "idx_logs_user_id", "idx_logs_username",
		"idx_type_created_at_quota", "idx_user_id_id", "index_username_model_name",
	}
	sort.Strings(expectedIndexes)
	require.Equal(t, expectedIndexes, indexes, "logs indexes changed; supplier accounting V1 permits no request-path schema/index addition")
}
