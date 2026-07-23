package controller

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// StripeAutoTopUpSettingPayload is the read model for the per-user auto top-up setting,
// including the server-side policy the (future) UI needs to render valid choices.
type StripeAutoTopUpSettingPayload struct {
	Enabled      bool `json:"enabled"`
	ThresholdUSD int  `json:"threshold_usd"`
	AmountUSD    int  `json:"amount_usd"`
	// CardBound reports whether the user currently has a saved card; enabling auto
	// top-up requires it.
	CardBound bool `json:"card_bound"`
	// Server-side policy (informational, enforced on write and on charge).
	DailyMaxCharges  int     `json:"daily_max_charges"`
	ThresholdMinUSD  int     `json:"threshold_min_usd"`
	ThresholdMaxUSD  int     `json:"threshold_max_usd"`
	AmountMinUSD     int     `json:"amount_min_usd"`
	AmountMaxUSD     int     `json:"amount_max_usd"`
	AmountPresetsUSD []int64 `json:"amount_presets_usd,omitempty"`
}

// UpdateStripeAutoTopUpRequest is the write model for the per-user auto top-up setting.
type UpdateStripeAutoTopUpRequest struct {
	Enabled      bool `json:"enabled"`
	ThresholdUSD int  `json:"threshold_usd"`
	AmountUSD    int  `json:"amount_usd"`
}

// validateAutoTopUpParams checks a user-supplied threshold/amount pair against the
// server-side policy: hard min/max bounds for both values, and — when the operator has
// configured top-up preset amounts (and quota is displayed in USD, where those presets
// are USD figures) — the amount must be one of the presets.
func validateAutoTopUpParams(thresholdUSD int, amountUSD int) error {
	if thresholdUSD < setting.StripeAutoTopUpThresholdMinUSD || thresholdUSD > setting.StripeAutoTopUpThresholdMaxUSD {
		return fmt.Errorf("auto top-up threshold must be between %d and %d USD", setting.StripeAutoTopUpThresholdMinUSD, setting.StripeAutoTopUpThresholdMaxUSD)
	}
	if amountUSD < setting.StripeAutoTopUpAmountMinUSD || amountUSD > setting.StripeAutoTopUpAmountMaxUSD {
		return fmt.Errorf("auto top-up amount must be between %d and %d USD", setting.StripeAutoTopUpAmountMinUSD, setting.StripeAutoTopUpAmountMaxUSD)
	}
	if presets := autoTopUpAmountPresets(); len(presets) > 0 {
		for _, preset := range presets {
			if int64(amountUSD) == preset {
				return nil
			}
		}
		return fmt.Errorf("auto top-up amount must be one of the configured preset amounts: %s USD", stripeTopUpPresetAmountLabel())
	}
	return nil
}

// autoTopUpAmountPresets returns the preset amounts the auto top-up amount must match.
// In token display mode the configured AmountOptions are token figures, not USD, so
// preset matching is skipped there and only the min/max bounds apply.
func autoTopUpAmountPresets() []int64 {
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		return nil
	}
	return stripeTopUpPresetAmounts()
}

// GetStripeAutoTopUpSetting returns the current user's auto top-up setting plus the
// server-side policy bounds.
func GetStripeAutoTopUpSetting(c *gin.Context) {
	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil || user == nil {
		common.ApiErrorMsg(c, "User not found")
		return
	}
	userSetting := user.GetSetting()
	common.ApiSuccess(c, buildStripeAutoTopUpPayload(user, userSetting.AutoTopUpEnabled, userSetting.AutoTopUpThresholdUSD, userSetting.AutoTopUpAmountUSD))
}

// UpdateStripeAutoTopUpSetting saves the current user's auto top-up setting. Enabling
// requires a bound card and valid threshold/amount; disabling always succeeds and keeps
// the stored threshold/amount for convenient re-enabling.
func UpdateStripeAutoTopUpSetting(c *gin.Context) {
	var req UpdateStripeAutoTopUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "Invalid request parameters")
		return
	}

	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil || user == nil {
		common.ApiErrorMsg(c, "User not found")
		return
	}

	userSetting := user.GetSetting()
	if req.Enabled {
		if setting.StripeAutoTopUpDailyMaxCharges <= 0 {
			common.ApiErrorMsg(c, "Auto top-up is not available")
			return
		}
		if err := validateAutoTopUpParams(req.ThresholdUSD, req.AmountUSD); err != nil {
			common.ApiErrorMsg(c, err.Error())
			return
		}
		if !user.StripeCardBound || strings.TrimSpace(user.StripeCustomer) == "" {
			common.ApiErrorMsg(c, "A saved card is required to enable auto top-up")
			return
		}
		userSetting.AutoTopUpEnabled = true
		userSetting.AutoTopUpThresholdUSD = req.ThresholdUSD
		userSetting.AutoTopUpAmountUSD = req.AmountUSD
	} else {
		userSetting.AutoTopUpEnabled = false
	}

	if err := model.SaveUserSetting(userId, userSetting); err != nil {
		common.ApiErrorMsg(c, "Failed to save auto top-up setting")
		return
	}
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("Stripe 自动充值设置已更新 user_id=%d enabled=%t threshold_usd=%d amount_usd=%d", userId, userSetting.AutoTopUpEnabled, userSetting.AutoTopUpThresholdUSD, userSetting.AutoTopUpAmountUSD))
	common.ApiSuccess(c, buildStripeAutoTopUpPayload(user, userSetting.AutoTopUpEnabled, userSetting.AutoTopUpThresholdUSD, userSetting.AutoTopUpAmountUSD))
}

func buildStripeAutoTopUpPayload(user *model.User, enabled bool, thresholdUSD int, amountUSD int) StripeAutoTopUpSettingPayload {
	cardBound := user.StripeCardBound && strings.TrimSpace(user.StripeCustomer) != ""
	return StripeAutoTopUpSettingPayload{
		Enabled:          enabled,
		ThresholdUSD:     thresholdUSD,
		AmountUSD:        amountUSD,
		CardBound:        cardBound,
		DailyMaxCharges:  setting.StripeAutoTopUpDailyMaxCharges,
		ThresholdMinUSD:  setting.StripeAutoTopUpThresholdMinUSD,
		ThresholdMaxUSD:  setting.StripeAutoTopUpThresholdMaxUSD,
		AmountMinUSD:     setting.StripeAutoTopUpAmountMinUSD,
		AmountMaxUSD:     setting.StripeAutoTopUpAmountMaxUSD,
		AmountPresetsUSD: autoTopUpAmountPresets(),
	}
}
