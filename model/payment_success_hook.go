package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

// PaymentSuccessHook is registered by service/ so payment completion code can
// emit post-commit side effects without introducing a model -> service import cycle.
var PaymentSuccessHook func(*TopUp)

func notifyPaymentSuccessBestEffort(topUp *TopUp, newlyCompleted bool) {
	if !newlyCompleted || topUp == nil || topUp.Status != common.TopUpStatusSuccess || PaymentSuccessHook == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			common.SysError(fmt.Sprintf("panic while handling payment success hook trade_no=%s user_id=%d panic=%v", topUp.TradeNo, topUp.UserId, r))
		}
	}()
	PaymentSuccessHook(topUp)
}
