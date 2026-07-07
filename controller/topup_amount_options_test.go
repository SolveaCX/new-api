package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWalletTopUpAmountOptionsReplaceTenDollarEntryTier(t *testing.T) {
	require.Equal(t, []int{5, 20, 200}, walletTopUpAmountOptions([]int{10, 20, 200}))
	require.Equal(t, []int{5, 20, 200}, walletTopUpAmountOptions([]int{5, 10, 20, 200}))
	require.Equal(t, []int{5}, walletTopUpAmountOptions([]int{10}))
}

func TestWalletTopUpAmountOptionsDropsInvalidValues(t *testing.T) {
	require.Equal(t, []int{5, 20}, walletTopUpAmountOptions([]int{10, 0, -1, 20, 20}))
}
