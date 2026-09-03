package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestUserTopUpHistoryIncludesAllOrderStatuses(t *testing.T) {
	setupTopUpLifecycleTestDB(t, 1)
	require.NoError(t, DB.AutoMigrate(&PaymentInvoice{}))

	user := createLifecycleQuotaTestUser(t, "topup-history-status-filter", 0, 100)
	now := common.GetTimestamp()
	for index, status := range []string{
		common.TopUpStatusPending,
		common.TopUpStatusExpired,
		common.TopUpStatusFailed,
		common.TopUpStatusSuccess,
	} {
		insertTopUpLifecycleOrder(
			t,
			user.Id,
			"history-status-"+status,
			PaymentProviderStripe,
			status,
			now+int64(index),
			0,
		)
	}

	pageInfo := &common.PageInfo{Page: 1, PageSize: 10}
	topUps, total, err := GetUserTopUps(user.Id, pageInfo)
	require.NoError(t, err)
	require.EqualValues(t, 4, total)
	require.Len(t, topUps, 4)
	require.Equal(t, common.TopUpStatusSuccess, topUps[0].Status)
	require.Equal(t, common.TopUpStatusFailed, topUps[1].Status)
	require.Equal(t, common.TopUpStatusExpired, topUps[2].Status)
	require.Equal(t, common.TopUpStatusPending, topUps[3].Status)
}

func TestSearchUserTopUpHistoryIncludesAllOrderStatuses(t *testing.T) {
	setupTopUpLifecycleTestDB(t, 1)
	require.NoError(t, DB.AutoMigrate(&PaymentInvoice{}))

	user := createLifecycleQuotaTestUser(t, "topup-history-search-filter", 0, 100)
	now := common.GetTimestamp()
	for index, status := range []string{
		common.TopUpStatusPending,
		common.TopUpStatusExpired,
		common.TopUpStatusSuccess,
	} {
		insertTopUpLifecycleOrder(
			t,
			user.Id,
			"searchable-history-"+status,
			PaymentProviderStripe,
			status,
			now+int64(index),
			0,
		)
	}

	pageInfo := &common.PageInfo{Page: 1, PageSize: 10}
	topUps, total, err := SearchUserTopUps(user.Id, "%searchable-history%", pageInfo)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, topUps, 3)
	require.Equal(t, common.TopUpStatusSuccess, topUps[0].Status)
	require.Equal(t, common.TopUpStatusExpired, topUps[1].Status)
	require.Equal(t, common.TopUpStatusPending, topUps[2].Status)
}

func TestUserTopUpHistoryIncludesRecordsOlderThanThirtyDays(t *testing.T) {
	setupTopUpLifecycleTestDB(t, 1)
	require.NoError(t, DB.AutoMigrate(&PaymentInvoice{}))

	user := createLifecycleQuotaTestUser(t, "topup-history-long-term", 0, 100)
	oldCreateTime := common.GetTimestamp() - 31*24*60*60
	insertTopUpLifecycleOrder(
		t,
		user.Id,
		"history-old-success",
		PaymentProviderStripe,
		common.TopUpStatusSuccess,
		oldCreateTime,
		oldCreateTime,
	)

	pageInfo := &common.PageInfo{Page: 1, PageSize: 10}
	topUps, total, err := GetUserTopUps(user.Id, pageInfo)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, topUps, 1)
	require.Equal(t, "history-old-success", topUps[0].TradeNo)

	searchResults, searchTotal, err := SearchUserTopUps(
		user.Id,
		"history-old-success",
		pageInfo,
	)
	require.NoError(t, err)
	require.EqualValues(t, 1, searchTotal)
	require.Len(t, searchResults, 1)
	require.Equal(t, "history-old-success", searchResults[0].TradeNo)
}
