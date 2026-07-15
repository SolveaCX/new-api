package model

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StripeBonusClaim records that a physical card (identified by its stable Stripe fingerprint)
// has already earned the one-time new-user bonus. The UNIQUE index on CardFingerprint lets
// the database atomically enforce "one bonus per card" — concurrent binds of the same card
// across different accounts race on the insert, and only the winner grants the bonus. This
// avoids the read-then-write TOCTOU of a count-based check.
type StripeBonusClaim struct {
	Id              int    `json:"id"`
	CardFingerprint string `json:"card_fingerprint" gorm:"type:varchar(64);uniqueIndex"`
	UserId          int    `json:"user_id" gorm:"index"`
	CreatedTime     int64  `json:"created_time" gorm:"bigint"`
}

// claimBonusForFingerprint atomically claims the one-time bonus for a card fingerprint within
// the given transaction. Returns true if THIS user won the claim (and should be granted the
// bonus), false if the card already claimed it (insert lost on the unique index).
func claimBonusForFingerprint(tx *gorm.DB, userId int, cardFingerprint string) (bool, error) {
	claim := &StripeBonusClaim{
		CardFingerprint: cardFingerprint,
		UserId:          userId,
		CreatedTime:     common.GetTimestamp(),
	}
	// Insert; on unique-key conflict the card already claimed the bonus -> we lost the race.
	res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(claim)
	if res.Error != nil {
		return false, res.Error
	}
	// RowsAffected == 0 means ON CONFLICT DO NOTHING skipped the insert (already claimed).
	return res.RowsAffected > 0, nil
}

// PaymentProviderStripeAuto marks top-up orders created by the threshold-triggered
// automatic off-session charge, so they can be distinguished from manual top-ups.
const PaymentProviderStripeAuto = "stripe_auto"

// SetStripeCardUnbound clears the bound-card flag for a user (used when a card is detached).
func SetStripeCardUnbound(userId int) error {
	if userId <= 0 {
		return errors.New("invalid user id")
	}
	return DB.Model(&User{}).Where("id = ?", userId).Update("stripe_card_bound", false).Error
}

// SetStripeCardBound marks a user's card as bound (and records customer + fingerprint),
// without granting any bonus. Used by the recharge-with-save-card flow, where the card is
// saved during a paid Checkout via setup_future_usage.
func SetStripeCardBound(userId int, customerId string, cardFingerprint string) error {
	if userId <= 0 {
		return errors.New("invalid user id")
	}
	fields := map[string]interface{}{"stripe_card_bound": true}
	if strings.TrimSpace(customerId) != "" {
		fields["stripe_customer"] = strings.TrimSpace(customerId)
	}
	if strings.TrimSpace(cardFingerprint) != "" {
		fields["stripe_card_fingerprint"] = strings.TrimSpace(cardFingerprint)
	}
	return DB.Model(&User{}).Where("id = ?", userId).Updates(fields).Error
}

// ClaimStripeCardFingerprint atomically consumes the one-time new-user-bonus eligibility for a
// physical card (Stripe fingerprint) without granting any bonus. It is used by the paid
// recharge-with-save-card flow: that flow gives the user a (purchased) deposit bonus instead of
// the free new-user bonus, so the card must still "use up" its one bonus slot to stop the same
// physical card from later earning the free new-user bonus on another account via the setup-mode
// bind path (which guards on the same StripeBonusClaim unique index). No-op when fingerprint is
// empty. Idempotent: a card already claimed (by this or another user) is a harmless no-op.
func ClaimStripeCardFingerprint(userId int, cardFingerprint string) error {
	cardFingerprint = strings.TrimSpace(cardFingerprint)
	if userId <= 0 || cardFingerprint == "" {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		_, err := claimBonusForFingerprint(tx, userId, cardFingerprint)
		return err
	})
}

// HasRecentStripeAutoCharge reports whether the user already has an automatic off-session
// charge recorded within the last windowSeconds. This is a persistent (cross-instance,
// restart-safe) cooldown guard that complements the in-memory dedup in the controller —
// it prevents charging the same user again from another replica or after a restart.
func HasRecentStripeAutoCharge(userId int, windowSeconds int64) (bool, error) {
	if userId <= 0 {
		return false, errors.New("invalid user id")
	}
	cutoff := common.GetTimestamp() - windowSeconds
	var count int64
	err := DB.Model(&TopUp{}).
		Where("user_id = ? AND payment_provider = ? AND create_time >= ?", userId, PaymentProviderStripeAuto, cutoff).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// stripeAutoTopUpTradeNo returns the deterministic trade_no for the seq-th automatic
// top-up slot of a user on a given UTC day (day formatted as YYYYMMDD). Determinism
// across nodes is the idempotency mechanism: concurrent nodes observing the same
// exhaustion episode compute the same key and race on the trade_no unique index, so
// exactly one insert — and therefore exactly one charge — wins (Rule 11).
func stripeAutoTopUpTradeNo(userId int, day string, seq int) string {
	return fmt.Sprintf("sauto_%d_%s_%d", userId, day, seq)
}

// ClaimStripeAutoTopUpEpisode atomically claims the next automatic top-up slot for a
// user on the given UTC day, bounded by dailyCap slots per day and a minimum gap of
// cooldownSeconds between consecutive slots. It returns (order, true) when THIS caller
// won the claim and must perform the charge, and (nil, false) when the daily cap is
// reached, the newest slot is too fresh (another charge just happened / is in flight),
// or a concurrent node won the insert race.
//
// Multi-node safety: the slot count and the newest slot's create_time are read in one
// statement, and the insert targets slot[count] under the trade_no unique index. A
// concurrent claimer either saw the same snapshot (same slot key → unique index picks
// one winner) or a snapshot that already includes the winner's row (→ stopped by the
// cooldown or the cap). There is no interleaving that yields two winners.
//
// NOTE for revenue/top-up reporting: claimed rows start as status=pending and may end
// failed. Any report aggregating TopUp by payment_provider MUST filter on
// status = success (the stripe_auto provider also carries pending/failed claim rows).
func ClaimStripeAutoTopUpEpisode(userId int, day string, dailyCap int, cooldownSeconds int64, amountUnits int, money float64) (*TopUp, bool, error) {
	day = strings.TrimSpace(day)
	if userId <= 0 || dailyCap <= 0 || day == "" || amountUnits <= 0 {
		return nil, false, nil
	}
	// Enumerate the (small) set of possible slot keys for the day. Exact-match IN keeps
	// the query cross-DB safe and avoids LIKE wildcard pitfalls ("_" matches any char).
	candidates := make([]string, 0, dailyCap)
	for seq := 0; seq < dailyCap; seq++ {
		candidates = append(candidates, stripeAutoTopUpTradeNo(userId, day, seq))
	}
	var snapshot struct {
		Taken  int64
		Latest sql.NullInt64
	}
	if err := DB.Model(&TopUp{}).
		Select("COUNT(*) AS taken, MAX(create_time) AS latest").
		Where("trade_no IN ?", candidates).
		Scan(&snapshot).Error; err != nil {
		return nil, false, err
	}
	if snapshot.Taken >= int64(dailyCap) {
		return nil, false, nil
	}
	now := common.GetTimestamp()
	if snapshot.Latest.Valid && now-snapshot.Latest.Int64 < cooldownSeconds {
		return nil, false, nil
	}
	topUp := &TopUp{
		UserId:          userId,
		Amount:          int64(amountUnits),
		Money:           money,
		TradeNo:         stripeAutoTopUpTradeNo(userId, day, int(snapshot.Taken)),
		PaymentMethod:   PaymentMethodStripe,
		PaymentProvider: PaymentProviderStripeAuto,
		CreateTime:      now,
		Status:          common.TopUpStatusPending,
	}
	res := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(topUp)
	if res.Error != nil {
		return nil, false, res.Error
	}
	if res.RowsAffected == 0 {
		// Another node claimed this slot concurrently — it owns the charge.
		return nil, false, nil
	}
	return topUp, true, nil
}

// CompleteStripeAutoTopUpOrder marks a claimed auto top-up order as paid and credits
// the user's quota, all within one transaction. Idempotent: completing an order that is
// already successful is a no-op, so a duplicate call cannot credit quota twice.
func CompleteStripeAutoTopUpOrder(tradeNo string, gatewayTradeNo string, callerIp string) error {
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo == "" {
		return errors.New("missing auto top-up trade no")
	}

	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}

	var creditedQuota int
	var creditedUserId int
	var creditedMoney float64
	err := DB.Transaction(func(tx *gorm.DB) error {
		topUp := &TopUp{}
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", tradeNo).First(topUp).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTopUpNotFound
			}
			return err
		}
		if topUp.PaymentProvider != PaymentProviderStripeAuto {
			return ErrPaymentMethodMismatch
		}
		if topUp.Status == common.TopUpStatusSuccess {
			return nil
		}
		if topUp.Status != common.TopUpStatusPending {
			return ErrTopUpStatusInvalid
		}
		quotaToAdd := int(topUp.Amount) * int(common.QuotaPerUnit)
		if quotaToAdd <= 0 {
			return errors.New("invalid auto top-up amount")
		}
		topUp.Status = common.TopUpStatusSuccess
		topUp.CompleteTime = common.GetTimestamp()
		if strings.TrimSpace(gatewayTradeNo) != "" {
			topUp.GatewayTradeNo = strings.TrimSpace(gatewayTradeNo)
		}
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}
		if err := tx.Model(&User{}).Where("id = ?", topUp.UserId).
			Update("quota", gorm.Expr("quota + ?", quotaToAdd)).Error; err != nil {
			return err
		}
		creditedQuota = quotaToAdd
		creditedUserId = topUp.UserId
		creditedMoney = topUp.Money
		return nil
	})
	if err != nil {
		return err
	}
	if creditedQuota > 0 {
		if cacheErr := cacheIncrUserQuota(creditedUserId, int64(creditedQuota)); cacheErr != nil {
			common.SysLog("failed to increase user quota cache after stripe auto top-up: " + cacheErr.Error())
		}
		RecordTopupLog(creditedUserId, fmt.Sprintf("自动充值成功，充值额度: %s，支付金额：%.2f", logger.FormatQuota(creditedQuota), creditedMoney), callerIp, PaymentMethodStripe, PaymentProviderStripeAuto)
	}
	return nil
}

// MarkStripeAutoTopUpOrderFailed transitions a pending auto top-up claim to failed.
// The failed row deliberately keeps occupying its daily slot and remains the newest
// slot for the cooldown check, so a declined card cannot be retried in a tight loop.
func MarkStripeAutoTopUpOrderFailed(tradeNo string, gatewayTradeNo string) error {
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo == "" {
		return errors.New("missing auto top-up trade no")
	}
	updates := map[string]interface{}{
		"status":        common.TopUpStatusFailed,
		"complete_time": common.GetTimestamp(),
	}
	if strings.TrimSpace(gatewayTradeNo) != "" {
		updates["gateway_trade_no"] = strings.TrimSpace(gatewayTradeNo)
	}
	return DB.Model(&TopUp{}).
		Where("trade_no = ? AND payment_provider = ? AND status = ?", tradeNo, PaymentProviderStripeAuto, common.TopUpStatusPending).
		Updates(updates).Error
}

// DisableUserAutoTopUpSetting turns off the user's opt-in auto top-up flag (used after
// a definitive card failure so a broken card is never retried against Stripe forever).
// Returns true when the flag was actually flipped by this call.
func DisableUserAutoTopUpSetting(userId int) (bool, error) {
	if userId <= 0 {
		return false, errors.New("invalid user id")
	}
	user, err := GetUserById(userId, true)
	if err != nil {
		return false, err
	}
	userSetting := user.GetSetting()
	if !userSetting.AutoTopUpEnabled {
		return false, nil
	}
	userSetting.AutoTopUpEnabled = false
	if err := SaveUserSetting(userId, userSetting); err != nil {
		return false, err
	}
	return true, nil
}

// RecordStripeAutoChargeFailure writes a user-visible system log entry when an automatic
// off-session charge fails, so the user (and admins) can see that their bound card needs
// attention. reason is a short human-readable cause (e.g. "card declined").
func RecordStripeAutoChargeFailure(userId int, amountUnits int, reason string) {
	if userId <= 0 {
		return
	}
	RecordLog(userId, LogTypeSystem, fmt.Sprintf(
		"自动扣费失败：尝试为您的绑定卡扣款 $%d 失败（%s），请检查或更新您的支付方式以免影响使用。",
		amountUnits, reason,
	))
}

