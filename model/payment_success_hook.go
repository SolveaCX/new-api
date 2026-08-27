package model

import "github.com/QuantumNous/new-api/common"

// PaymentSuccessHook is registered by service/ so payment completion code can
// emit post-commit side effects without introducing a model -> service import cycle.
var PaymentSuccessHook func(*TopUp)

func notifyPaymentSuccessBestEffort(topUp *TopUp, newlyCompleted bool) {
	if !newlyCompleted || topUp == nil || topUp.Status != common.TopUpStatusSuccess || PaymentSuccessHook == nil {
		return
	}
	PaymentSuccessHook(topUp)
}
