package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestNotifyPaymentSuccessBestEffortRecoversFromHookPanic(t *testing.T) {
	originalHook := PaymentSuccessHook
	t.Cleanup(func() {
		PaymentSuccessHook = originalHook
	})

	PaymentSuccessHook = func(*TopUp) {
		panic("boom")
	}

	require.NotPanics(t, func() {
		notifyPaymentSuccessBestEffort(&TopUp{
			TradeNo: "trade-panic",
			UserId:  42,
			Status:  common.TopUpStatusSuccess,
		}, true)
	})
}
