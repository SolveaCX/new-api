package service

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPrepareSupplierAccountingAttemptUsesSingleDatabaseStatement(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.EnsureSupplierAccountingFactSchema(db))
	previous := model.LOG_DB
	model.LOG_DB = db
	t.Cleanup(func() { model.LOG_DB = previous })

	cutover := time.Date(2020, 1, 1, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60)).Unix()
	t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", strconv.FormatInt(cutover, 10))
	rawCount, queryCount := 0, 0
	require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register("test:count_prepare_raw", func(tx *gorm.DB) {
		if strings.Contains(tx.Statement.SQL.String(), "supplier_accounting_facts") {
			rawCount++
		}
	}))
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test:count_prepare_query", func(tx *gorm.DB) {
		if tx.Statement.Table == "supplier_accounting_facts" {
			queryCount++
		}
	}))

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	relayInfo := &relaycommon.RelayInfo{
		RequestId:       "req-single-prepare",
		RetryIndex:      1,
		OriginModelName: "gpt-test",
		SupplierCostSnapshot: types.SupplierCostSnapshot{
			SupplierId: 12, ContractId: 13, BindingVersionId: 11, RateVersionId: 14,
		},
	}
	require.NoError(t, PrepareSupplierAccountingAttempt(c, relayInfo, 15))
	require.Equal(t, 1, rawCount)
	require.Zero(t, queryCount)
	require.NotNil(t, currentSupplierAccountingAttempt(c))

	var count int64
	require.NoError(t, db.Model(&model.SupplierAccountingFact{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}
