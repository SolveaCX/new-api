package service

import (
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

// TriggerStripeAutoCharge is the threshold-triggered automatic off-session charge entry point.
// The real implementation lives in the controller package (where the Stripe helpers are) and is
// registered here at init time to avoid a circular import. It is nil until registered.
var TriggerStripeAutoCharge func(userId int)

// autoChargeEvalAt throttles per-user trigger evaluation on this node. This is pure load
// shedding — cross-node correctness is enforced by the DB-side episode claim in the
// trigger implementation (Rule 11), never by this map.
var autoChargeEvalAt sync.Map

const autoChargeEvalIntervalSeconds int64 = 10

// MaybeTriggerStripeAutoCharge fires an asynchronous auto-charge when the user's balance has
// dropped below the applicable threshold. It never blocks the caller (relay hot path): the
// current request proceeds under its normal quota rules regardless of the charge outcome.
//
// Two configurations can arm the trigger:
//   - the user's own opt-in auto top-up setting (read from the request context, where
//     TokenAuth cached it — zero extra IO on the hot path); the authoritative re-check
//     happens inside the trigger implementation with fresh data;
//   - the legacy operator-level StripeAutoCharge* options.
func MaybeTriggerStripeAutoCharge(c *gin.Context, userId int, userQuota int) {
	if TriggerStripeAutoCharge == nil || userId <= 0 {
		return
	}
	perUserActive := setting.StripeAutoTopUpDailyMaxCharges > 0
	globalActive := setting.StripeAutoChargeEnabled
	if !perUserActive && !globalActive {
		return
	}

	thresholdUSD := 0
	if globalActive {
		thresholdUSD = setting.StripeAutoChargeThreshold
	}
	if perUserActive {
		if userSetting, ok := contextUserSetting(c); ok {
			if userSetting.AutoTopUpEnabled {
				userThreshold := userSetting.AutoTopUpThresholdUSD
				if userThreshold > setting.StripeAutoTopUpThresholdMaxUSD {
					userThreshold = setting.StripeAutoTopUpThresholdMaxUSD
				}
				if userThreshold > thresholdUSD {
					thresholdUSD = userThreshold
				}
			}
		} else {
			// No cached setting on this request (non-token callers): fall back to the widest
			// permitted threshold; the trigger re-checks the real opt-in before charging.
			if setting.StripeAutoTopUpThresholdMaxUSD > thresholdUSD {
				thresholdUSD = setting.StripeAutoTopUpThresholdMaxUSD
			}
		}
	}
	if thresholdUSD <= 0 || userQuota >= thresholdUSD*int(common.QuotaPerUnit) {
		return
	}

	now := time.Now().Unix()
	if v, ok := autoChargeEvalAt.Load(userId); ok {
		if at, ok2 := v.(int64); ok2 && now-at < autoChargeEvalIntervalSeconds {
			return
		}
	}
	autoChargeEvalAt.Store(userId, now)

	gopool.Go(func() {
		TriggerStripeAutoCharge(userId)
	})
}

func contextUserSetting(c *gin.Context) (dto.UserSetting, bool) {
	if c == nil {
		return dto.UserSetting{}, false
	}
	return common.GetContextKeyType[dto.UserSetting](c, constant.ContextKeyUserSetting)
}
