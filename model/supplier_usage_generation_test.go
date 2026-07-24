package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestEnsureSupplierUsageGenerationSchemaUpgradesDraftUniqueIndex(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE supplier_usage_daily_summaries (
		id integer PRIMARY KEY AUTOINCREMENT,
		batch_date varchar(10) NOT NULL,
		batch_fence_token integer NOT NULL DEFAULT 0,
		dimension_key varchar(64) NOT NULL
	)`).Error)
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX ux_supplier_daily_dimension ON supplier_usage_daily_summaries(batch_date, dimension_key)`).Error)

	require.NoError(t, EnsureSupplierUsageGenerationSchema(db))
	columns, err := supplierUsageIndexColumns(db, "ux_supplier_daily_dimension")
	require.NoError(t, err)
	require.Equal(t, []string{"batch_date", "batch_fence_token", "dimension_key"}, columns)
}
