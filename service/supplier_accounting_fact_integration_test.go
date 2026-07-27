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

func TestPrepareSupplierAccountingAttemptUsesChannelPolicyForInternalTraffic(t *testing.T) {
	tests := []struct {
		name                   string
		skipInternalAccounting bool
		scope                  types.SupplierStatisticsScopeSnapshot
		wantFacts              int64
	}{
		{
			name:                   "internal traffic with skip policy",
			skipInternalAccounting: true,
			scope: types.SupplierStatisticsScopeSnapshot{
				Scope:           types.SupplierStatisticsScopeInternal,
				ExclusionRuleId: 91,
			},
			wantFacts: 0,
		},
		{
			name:                   "business traffic with skip policy",
			skipInternalAccounting: true,
			scope:                  types.BusinessSupplierStatisticsScopeSnapshot(),
			wantFacts:              1,
		},
		{
			name:                   "internal traffic with record policy",
			skipInternalAccounting: false,
			scope: types.SupplierStatisticsScopeSnapshot{
				Scope:           types.SupplierStatisticsScopeInternal,
				ExclusionRuleId: 92,
			},
			wantFacts: 1,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "-")+"?mode=memory&cache=shared"), &gorm.Config{})
			require.NoError(t, err)
			require.NoError(t, model.EnsureSupplierAccountingFactSchema(db))
			previous := model.LOG_DB
			model.LOG_DB = db
			t.Cleanup(func() { model.LOG_DB = previous })

			cutover := time.Date(2020, 1, 1, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60)).Unix()
			t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", strconv.FormatInt(cutover, 10))
			prepareStatements := 0
			require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register("test:count_policy_prepare", func(tx *gorm.DB) {
				if strings.Contains(tx.Statement.SQL.String(), "supplier_accounting_facts") {
					prepareStatements++
				}
			}))
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
			relayInfo := &relaycommon.RelayInfo{
				RequestId:                       "req-" + strings.ReplaceAll(testCase.name, " ", "-"),
				OriginModelName:                 "gpt-test",
				SupplierStatisticsScopeSnapshot: testCase.scope,
				SupplierCostSnapshot: types.SupplierCostSnapshot{
					SupplierId: 12, ContractId: 13, BindingVersionId: 11, RateVersionId: 14,
					SkipInternalAccounting: testCase.skipInternalAccounting,
				},
			}

			require.NoError(t, PrepareSupplierAccountingAttempt(c, relayInfo, 15))
			require.Equal(t, int(testCase.wantFacts), prepareStatements)
			var count int64
			require.NoError(t, db.Model(&model.SupplierAccountingFact{}).Count(&count).Error)
			require.Equal(t, testCase.wantFacts, count)
			if testCase.wantFacts == 0 {
				require.Nil(t, currentSupplierAccountingAttempt(c))
			} else {
				require.NotNil(t, currentSupplierAccountingAttempt(c))
			}
		})
	}
}

func TestPrepareSupplierAccountingAttemptDoesNotSkipInvalidPolicySnapshot(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.EnsureSupplierAccountingFactSchema(db))
	previous := model.LOG_DB
	model.LOG_DB = db
	t.Cleanup(func() { model.LOG_DB = previous })

	cutover := time.Date(2020, 1, 1, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60)).Unix()
	t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", strconv.FormatInt(cutover, 10))
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	relayInfo := &relaycommon.RelayInfo{
		RequestId:       "req-invalid-skip-policy",
		OriginModelName: "gpt-test",
		SupplierStatisticsScopeSnapshot: types.SupplierStatisticsScopeSnapshot{
			Scope:           types.SupplierStatisticsScopeInternal,
			ExclusionRuleId: 91,
		},
		SupplierCostSnapshot: types.SupplierCostSnapshot{
			SupplierId: 12, ContractId: 13, BindingVersionId: 0, RateVersionId: 14,
			SkipInternalAccounting: true,
		},
	}

	require.ErrorIs(t, PrepareSupplierAccountingAttempt(c, relayInfo, 15), ErrSupplierAccountingAttemptBindingInvalid)
	require.Nil(t, currentSupplierAccountingAttempt(c))
	var count int64
	require.NoError(t, db.Model(&model.SupplierAccountingFact{}).Count(&count).Error)
	require.Zero(t, count)
}
