package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestUserTopUpHistoryHidesUnsuccessfulOrders(t *testing.T) {
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
	require.EqualValues(t, 2, total)
	require.Len(t, topUps, 2)
	require.Equal(t, common.TopUpStatusSuccess, topUps[0].Status)
	require.Equal(t, common.TopUpStatusPending, topUps[1].Status)
}

func TestSearchUserTopUpHistoryHidesUnsuccessfulOrders(t *testing.T) {
	setupTopUpLifecycleTestDB(t, 1)
	require.NoError(t, DB.AutoMigrate(&PaymentInvoice{}))

	user := createLifecycleQuotaTestUser(t, "topup-history-search-filter", 0, 100)
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
	require.EqualValues(t, 2, total)
	require.Len(t, topUps, 2)
	require.Equal(t, common.TopUpStatusSuccess, topUps[0].Status)
	require.Equal(t, common.TopUpStatusPending, topUps[1].Status)
}

func TestAllTopUpHistoryCanFilterByStatus(t *testing.T) {
	setupTopUpLifecycleTestDB(t, 1)
	require.NoError(t, DB.AutoMigrate(&PaymentInvoice{}))

	user := createLifecycleQuotaTestUser(t, "topup-history-admin-filter", 0, 100)
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
			"admin-filter-history-"+status,
			PaymentProviderStripe,
			status,
			now+int64(index),
			0,
		)
	}

	pageInfo := &common.PageInfo{Page: 1, PageSize: 10}
	topUps, total, err := GetAllTopUps(pageInfo, common.TopUpStatusExpired)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, topUps, 1)
	require.Equal(t, common.TopUpStatusExpired, topUps[0].Status)
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

func TestTopUpHistoryExcludesFailedBeforePagination(t *testing.T) {
	setupTopUpLifecycleTestDB(t, 1)
	require.NoError(t, DB.AutoMigrate(&PaymentInvoice{}))
	user := createLifecycleQuotaTestUser(t, "history-pagination", 0, 100)
	for _, status := range []string{common.TopUpStatusSuccess, common.TopUpStatusPending, common.TopUpStatusFailed} {
		insertTopUpLifecycleOrder(t, user.Id, "pagination-"+status, PaymentProviderStripe, status, common.GetTimestamp(), 0)
	}
	for _, search := range []bool{false, true} {
		for page := 1; page <= 2; page++ {
			pageInfo := &common.PageInfo{Page: page, PageSize: 1}
			var rows []*TopUp
			var total int64
			var err error
			if search {
				rows, total, err = SearchAllTopUps("%pagination%", pageInfo, "", true)
			} else {
				rows, total, err = GetAllTopUps(pageInfo, "", true)
			}
			require.NoError(t, err)
			require.EqualValues(t, 2, total)
			require.Len(t, rows, 1)
			require.NotEqual(t, common.TopUpStatusFailed, rows[0].Status)
		}
	}
	rows, total, err := GetAllTopUps(&common.PageInfo{Page: 1, PageSize: 10}, common.TopUpStatusFailed)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, rows, 1)
}
