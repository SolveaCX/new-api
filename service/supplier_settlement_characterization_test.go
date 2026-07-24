package service

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type supplierSettlementFundingSpy struct {
	settleErr    error
	settleDeltas []int
}

func (f *supplierSettlementFundingSpy) Source() string { return BillingSourceWallet }

func (f *supplierSettlementFundingSpy) PreConsume(int) error { return nil }

func (f *supplierSettlementFundingSpy) Settle(delta int) error {
	f.settleDeltas = append(f.settleDeltas, delta)
	return f.settleErr
}

func (f *supplierSettlementFundingSpy) Refund() error { return nil }

func useSupplierSettlementDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	previousDB := model.DB
	previousRedisEnabled := common.RedisEnabled
	previousBatchUpdateEnabled := common.BatchUpdateEnabled
	model.DB = db
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.RedisEnabled = previousRedisEnabled
		common.BatchUpdateEnabled = previousBatchUpdateEnabled
		_ = sqlDB.Close()
	})
	return db
}

func TestBillingSessionSettleWithZeroDeltaIsFinanciallyCommitted(t *testing.T) {
	funding := &supplierSettlementFundingSpy{}
	session := &BillingSession{
		relayInfo:        &relaycommon.RelayInfo{IsPlayground: true},
		funding:          funding,
		preConsumedQuota: 73,
		tokenConsumed:    73,
	}

	require.NoError(t, session.Settle(73))

	require.False(t, session.NeedsRefund())
	require.Empty(t, funding.settleDeltas)
}

func TestBillingSessionFundingSuccessThenTokenAdjustmentFailureRemainsFinanciallyCommitted(t *testing.T) {
	db := useSupplierSettlementDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	funding := &supplierSettlementFundingSpy{}
	session := &BillingSession{
		relayInfo: &relaycommon.RelayInfo{
			UserId:   201,
			TokenId:  301,
			TokenKey: "closed-db-token",
		},
		funding:          funding,
		preConsumedQuota: 100,
		tokenConsumed:    100,
	}

	err = session.Settle(125)

	require.Error(t, err)
	require.Equal(t, []int{25}, funding.settleDeltas)
	require.False(t, session.NeedsRefund())
}

func TestBillingSessionFundingFailureIsNotFinanciallyCommitted(t *testing.T) {
	settleErr := errors.New("funding mutation failed")
	funding := &supplierSettlementFundingSpy{settleErr: settleErr}
	session := &BillingSession{
		relayInfo:        &relaycommon.RelayInfo{IsPlayground: true},
		funding:          funding,
		preConsumedQuota: 40,
		tokenConsumed:    40,
	}

	err := session.Settle(55)

	require.ErrorIs(t, err, settleErr)
	require.True(t, session.NeedsRefund())
}

func TestBillingSessionDuplicateSettlementDoesNotCommitFundingTwice(t *testing.T) {
	funding := &supplierSettlementFundingSpy{}
	session := &BillingSession{
		relayInfo:        &relaycommon.RelayInfo{IsPlayground: true},
		funding:          funding,
		preConsumedQuota: 40,
		tokenConsumed:    40,
	}

	require.NoError(t, session.Settle(55))
	require.NoError(t, session.Settle(55))

	require.Equal(t, []int{15}, funding.settleDeltas)
}

func TestBillingSessionSettlementPreservesExactFinalSalesQuota(t *testing.T) {
	db := useSupplierSettlementDB(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}))
	require.NoError(t, db.Create(&model.Token{
		Id:          401,
		UserId:      501,
		Key:         "exact-final-sales-quota",
		Name:        "exact quota token",
		Status:      common.TokenStatusEnabled,
		RemainQuota: 9_863,
		UsedQuota:   137,
	}).Error)

	funding := &supplierSettlementFundingSpy{}
	session := &BillingSession{
		relayInfo: &relaycommon.RelayInfo{
			UserId:   501,
			TokenId:  401,
			TokenKey: "exact-final-sales-quota",
		},
		funding:          funding,
		preConsumedQuota: 137,
		tokenConsumed:    137,
	}

	require.NoError(t, session.Settle(1_234))

	var token model.Token
	require.NoError(t, db.Select("remain_quota", "used_quota").First(&token, 401).Error)
	require.Equal(t, 1_234, token.UsedQuota)
	require.Equal(t, 8_766, token.RemainQuota)
}

func TestBillingSessionSettlementDoesNotPerformDBTimeLookup(t *testing.T) {
	db := useSupplierSettlementDB(t)
	var queryCount atomic.Int64
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(
		"supplier-settlement:count-queries",
		func(*gorm.DB) { queryCount.Add(1) },
	))

	funding := &supplierSettlementFundingSpy{}
	session := &BillingSession{
		relayInfo:        &relaycommon.RelayInfo{IsPlayground: true},
		funding:          funding,
		preConsumedQuota: 5,
		tokenConsumed:    5,
	}

	require.NoError(t, session.Settle(9))

	require.Zero(t, queryCount.Load())
}
