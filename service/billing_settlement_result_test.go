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

type billingSettlementResultFundingSpy struct {
	settleErr    error
	settleDeltas []int
}

func (f *billingSettlementResultFundingSpy) Source() string       { return BillingSourceWallet }
func (f *billingSettlementResultFundingSpy) PreConsume(int) error { return nil }
func (f *billingSettlementResultFundingSpy) Refund() error        { return nil }

func (f *billingSettlementResultFundingSpy) Settle(delta int) error {
	f.settleDeltas = append(f.settleDeltas, delta)
	return f.settleErr
}

func useBillingSettlementResultDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	previousDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })
	return db
}

func newBillingSettlementResultSession(funding FundingSource, preConsumedQuota int, playground bool) *BillingSession {
	return &BillingSession{
		relayInfo: &relaycommon.RelayInfo{
			UserId:       901,
			TokenId:      902,
			TokenKey:     "billing-settlement-result-token",
			IsPlayground: playground,
		},
		funding:          funding,
		preConsumedQuota: preConsumedQuota,
		tokenConsumed:    preConsumedQuota,
	}
}

func TestBillingSettlementResultZeroDeltaCommitsWithoutFundingMutation(t *testing.T) {
	funding := &billingSettlementResultFundingSpy{}
	session := newBillingSettlementResultSession(funding, 73, true)

	result := session.SettleWithResult(73)

	require.True(t, result.FinanciallyCommitted)
	require.NotZero(t, result.FinanciallyCommittedAt)
	require.Equal(t, 73, result.FinalSalesQuota)
	require.NoError(t, result.Err)
	require.Empty(t, funding.settleDeltas)
}

func TestBillingSettlementResultFundingFailureIsNotCommitted(t *testing.T) {
	settleErr := errors.New("funding mutation failed")
	funding := &billingSettlementResultFundingSpy{settleErr: settleErr}
	session := newBillingSettlementResultSession(funding, 40, true)

	result := session.SettleWithResult(55)

	require.False(t, result.FinanciallyCommitted)
	require.Zero(t, result.FinanciallyCommittedAt)
	require.Equal(t, 55, result.FinalSalesQuota)
	require.ErrorIs(t, result.Err, settleErr)
}

func TestBillingSettlementResultFundingCommitSurvivesTokenAdjustmentFailure(t *testing.T) {
	db := useBillingSettlementResultDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	funding := &billingSettlementResultFundingSpy{}
	session := newBillingSettlementResultSession(funding, 100, false)

	result := session.SettleWithResult(125)
	duplicate := session.SettleWithResult(999)

	require.True(t, result.FinanciallyCommitted)
	require.NotZero(t, result.FinanciallyCommittedAt)
	require.Equal(t, 125, result.FinalSalesQuota)
	require.Error(t, result.Err)
	require.Equal(t, result, duplicate)
	require.Equal(t, []int{25}, funding.settleDeltas)
}

func TestLegacyBillingFallbackReportsFundingCommitWhenTokenAdjustmentFails(t *testing.T) {
	db := useBillingSettlementResultDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	user := &model.User{Username: "legacy-settlement", Password: "password", AffCode: "legacy", Quota: 100}
	require.NoError(t, db.Create(user).Error)

	previousBatchUpdateEnabled := common.BatchUpdateEnabled
	common.BatchUpdateEnabled = false
	t.Cleanup(func() { common.BatchUpdateEnabled = previousBatchUpdateEnabled })

	relayInfo := &relaycommon.RelayInfo{
		UserId:        user.Id,
		TokenId:       404,
		TokenKey:      "missing-token-table",
		BillingSource: BillingSourceWallet,
	}
	result := SettleBillingResult(nil, relayInfo, 25)

	require.True(t, result.FinanciallyCommitted)
	require.NotZero(t, result.FinanciallyCommittedAt)
	require.Equal(t, 25, result.FinalSalesQuota)
	require.Error(t, result.Err)

	var updated model.User
	require.NoError(t, db.First(&updated, user.Id).Error)
	require.Equal(t, 75, updated.Quota)
}

func TestBillingSettlementResultDuplicateReturnsFirstCommit(t *testing.T) {
	funding := &billingSettlementResultFundingSpy{}
	session := newBillingSettlementResultSession(funding, 40, true)

	first := session.SettleWithResult(1_234)
	second := session.SettleWithResult(9_999)

	require.True(t, first.FinanciallyCommitted)
	require.Equal(t, first, second)
	require.Equal(t, 1_234, first.FinalSalesQuota)
	require.Equal(t, []int{1_194}, funding.settleDeltas)
}

func TestBillingSettlementResultCommitTimeDoesNotQueryDatabase(t *testing.T) {
	db := useBillingSettlementResultDB(t)
	var queryCount atomic.Int64
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(
		"billing-settlement-result:count-queries",
		func(*gorm.DB) { queryCount.Add(1) },
	))

	funding := &billingSettlementResultFundingSpy{}
	session := newBillingSettlementResultSession(funding, 5, true)

	before := common.GetTimestamp()
	result := session.SettleWithResult(9)
	after := common.GetTimestamp()

	require.True(t, result.FinanciallyCommitted)
	require.NotZero(t, result.FinanciallyCommittedAt)
	require.Zero(t, queryCount.Load())
	require.GreaterOrEqual(t, result.FinanciallyCommittedAt, before)
	require.LessOrEqual(t, result.FinanciallyCommittedAt, after)
}
