package model

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	InviteBenefitRiskRuleVersion = "invite_benefit_risk_v1"
	InviteBenefitRiskRuleR1      = "R1"
	InviteBenefitRiskRuleR2      = "R2"

	InviteBenefitRiskReasonR1 = "minimum_payment_ratio"
	InviteBenefitRiskReasonR2 = "uniform_minimum_payment_fast_cluster"

	InviteBenefitRiskMinimumPaidInvitees = 3
	InviteBenefitRiskMinimumUSDMinor     = int64(500)
	InviteBenefitRiskR1RatioBPS          = 8000
	InviteBenefitRiskR2RatioBPS          = 10000
	InviteBenefitRiskR2MaxSpanSeconds    = int64(24 * 60 * 60)
)

// InviteBenefitBlacklist is the durable, inviter-scoped decision produced by the
// invitation payment-cluster rules. InviterID is unique so concurrent application
// nodes can only create one immutable decision for an inviter.
type InviteBenefitBlacklist struct {
	Id                        int    `json:"id"`
	InviterId                 int    `json:"inviter_id" gorm:"uniqueIndex"`
	Rule                      string `json:"rule" gorm:"type:varchar(8);index"`
	Reason                    string `json:"reason" gorm:"type:varchar(64);index"`
	RuleVersion               string `json:"rule_version" gorm:"type:varchar(64);index"`
	PaidInviteeCount          int    `json:"paid_invitee_count"`
	MinimumAmountInviteeCount int    `json:"minimum_amount_invitee_count"`
	MinimumAmountRatioBPS     int    `json:"minimum_amount_ratio_bps"`
	MinimumUSDMinor           int64  `json:"minimum_usd_minor" gorm:"type:bigint"`
	FirstPaymentAt            int64  `json:"first_payment_at" gorm:"type:bigint"`
	LastFirstPaymentAt        int64  `json:"last_first_payment_at" gorm:"type:bigint"`
	FirstPaymentSpanSeconds   int64  `json:"first_payment_span_seconds" gorm:"type:bigint"`
	CreatedAt                 int64  `json:"created_at" gorm:"autoCreateTime;index"`
}

type inviteBenefitRiskMetrics struct {
	PaidInviteeCount          int
	MinimumAmountInviteeCount int
	MinimumAmountRatioBPS     int
	FirstPaymentAt            int64
	LastFirstPaymentAt        int64
	FirstPaymentSpanSeconds   int64
}

type inviteBenefitPaymentRow struct {
	ID               int
	UserID           int
	TradeNo          string
	PaymentCurrency  string
	PaymentProvider  string
	Money            float64
	CreateTime       int64
	CompleteTime     int64
	fromSubscription bool
}

type inviteBenefitInviteePayment struct {
	totalUSDMinor  int64
	firstPaymentAt int64
}

func classifyInviteBenefitRisk(metrics inviteBenefitRiskMetrics) (string, string, bool) {
	if metrics.PaidInviteeCount < InviteBenefitRiskMinimumPaidInvitees {
		return "", "", false
	}
	if metrics.MinimumAmountRatioBPS >= InviteBenefitRiskR2RatioBPS &&
		metrics.FirstPaymentSpanSeconds <= InviteBenefitRiskR2MaxSpanSeconds {
		return InviteBenefitRiskRuleR2, InviteBenefitRiskReasonR2, true
	}
	if metrics.MinimumAmountRatioBPS >= InviteBenefitRiskR1RatioBPS {
		return InviteBenefitRiskRuleR1, InviteBenefitRiskReasonR1, true
	}
	return "", "", false
}

func ensureInviteBenefitBlacklistForInviterTx(tx *gorm.DB, inviterId int) (*InviteBenefitBlacklist, bool, error) {
	if tx == nil {
		return nil, false, errors.New("tx is nil")
	}
	if inviterId <= 0 {
		return nil, false, nil
	}

	var existing InviteBenefitBlacklist
	err := tx.Where("inviter_id = ?", inviterId).First(&existing).Error
	if err == nil {
		return &existing, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	metrics, err := evaluateInviteBenefitRiskMetricsTx(tx, inviterId)
	if err != nil {
		return nil, false, err
	}
	rule, reason, matched := classifyInviteBenefitRisk(metrics)
	if !matched {
		return nil, false, nil
	}
	blacklist := InviteBenefitBlacklist{
		InviterId:                 inviterId,
		Rule:                      rule,
		Reason:                    reason,
		RuleVersion:               InviteBenefitRiskRuleVersion,
		PaidInviteeCount:          metrics.PaidInviteeCount,
		MinimumAmountInviteeCount: metrics.MinimumAmountInviteeCount,
		MinimumAmountRatioBPS:     metrics.MinimumAmountRatioBPS,
		MinimumUSDMinor:           InviteBenefitRiskMinimumUSDMinor,
		FirstPaymentAt:            metrics.FirstPaymentAt,
		LastFirstPaymentAt:        metrics.LastFirstPaymentAt,
		FirstPaymentSpanSeconds:   metrics.FirstPaymentSpanSeconds,
		CreatedAt:                 common.GetTimestamp(),
	}
	insert := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&blacklist)
	if insert.Error != nil {
		return nil, false, insert.Error
	}
	if insert.RowsAffected == 1 {
		return &blacklist, true, nil
	}
	if err := tx.Where("inviter_id = ?", inviterId).First(&blacklist).Error; err != nil {
		return nil, false, err
	}
	return &blacklist, false, nil
}

func evaluateInviteBenefitRiskMetricsTx(tx *gorm.DB, inviterId int) (inviteBenefitRiskMetrics, error) {
	var inviteeIds []int
	if err := tx.Model(&User{}).Where("inviter_id = ?", inviterId).Pluck("id", &inviteeIds).Error; err != nil {
		return inviteBenefitRiskMetrics{}, err
	}
	if len(inviteeIds) == 0 {
		return inviteBenefitRiskMetrics{}, nil
	}

	type subscriptionTradeRef struct {
		UserID  int
		TradeNo string
	}
	var subscriptionTradeRefs []subscriptionTradeRef
	if err := tx.Model(&SubscriptionOrder{}).Select("user_id", "trade_no").
		Where("user_id IN ?", inviteeIds).Find(&subscriptionTradeRefs).Error; err != nil {
		return inviteBenefitRiskMetrics{}, err
	}
	subscriptionMirrorKeys := make(map[string]struct{}, len(subscriptionTradeRefs))
	for _, ref := range subscriptionTradeRefs {
		if strings.TrimSpace(ref.TradeNo) != "" {
			subscriptionMirrorKeys[inviteBenefitPaymentTradeKey(ref.UserID, ref.TradeNo)] = struct{}{}
		}
	}

	var orders []SubscriptionOrder
	if err := tx.Select("id", "user_id", "trade_no", "payment_currency", "payment_provider", "money", "create_time", "complete_time").
		Where("user_id IN ? AND status = ?", inviteeIds, common.TopUpStatusSuccess).
		Find(&orders).Error; err != nil {
		return inviteBenefitRiskMetrics{}, err
	}
	var topUps []TopUp
	if err := tx.Select("id", "user_id", "trade_no", "payment_currency", "payment_provider", "money", "create_time", "complete_time").
		Where("user_id IN ? AND status = ?", inviteeIds, common.TopUpStatusSuccess).
		Find(&topUps).Error; err != nil {
		return inviteBenefitRiskMetrics{}, err
	}

	paymentsByTrade := make(map[string]inviteBenefitPaymentRow, len(orders)+len(topUps))
	for _, order := range orders {
		row := inviteBenefitPaymentRow{
			ID: order.Id, UserID: order.UserId, TradeNo: order.TradeNo,
			PaymentCurrency: order.PaymentCurrency, PaymentProvider: order.PaymentProvider,
			Money: order.Money, CreateTime: order.CreateTime, CompleteTime: order.CompleteTime,
			fromSubscription: true,
		}
		paymentsByTrade[inviteBenefitPaymentKey("subscription", row)] = row
	}
	for _, topUp := range topUps {
		row := inviteBenefitPaymentRow{
			ID: topUp.Id, UserID: topUp.UserId, TradeNo: topUp.TradeNo,
			PaymentCurrency: topUp.PaymentCurrency, PaymentProvider: topUp.PaymentProvider,
			Money: topUp.Money, CreateTime: topUp.CreateTime, CompleteTime: topUp.CompleteTime,
		}
		key := inviteBenefitPaymentKey("topup", row)
		if _, ok := subscriptionMirrorKeys[inviteBenefitPaymentTradeKey(row.UserID, row.TradeNo)]; ok {
			continue
		}
		if existing, ok := paymentsByTrade[key]; ok && existing.fromSubscription {
			continue
		}
		paymentsByTrade[key] = row
	}

	inviteePayments := make(map[int]inviteBenefitInviteePayment)
	for _, row := range paymentsByTrade {
		amountMinor, ok := inviteBenefitUSDMinor(row.PaymentCurrency, row.PaymentProvider, row.Money)
		if !ok {
			continue
		}
		paidAt := row.CompleteTime
		if paidAt <= 0 {
			paidAt = row.CreateTime
		}
		if paidAt <= 0 {
			continue
		}
		payment := inviteePayments[row.UserID]
		payment.totalUSDMinor += amountMinor
		if payment.firstPaymentAt == 0 || paidAt < payment.firstPaymentAt {
			payment.firstPaymentAt = paidAt
		}
		inviteePayments[row.UserID] = payment
	}

	metrics := inviteBenefitRiskMetrics{}
	for _, payment := range inviteePayments {
		if payment.totalUSDMinor <= 0 || payment.firstPaymentAt <= 0 {
			continue
		}
		metrics.PaidInviteeCount++
		if payment.totalUSDMinor == InviteBenefitRiskMinimumUSDMinor {
			metrics.MinimumAmountInviteeCount++
		}
		if metrics.FirstPaymentAt == 0 || payment.firstPaymentAt < metrics.FirstPaymentAt {
			metrics.FirstPaymentAt = payment.firstPaymentAt
		}
		if payment.firstPaymentAt > metrics.LastFirstPaymentAt {
			metrics.LastFirstPaymentAt = payment.firstPaymentAt
		}
	}
	if metrics.PaidInviteeCount > 0 {
		metrics.MinimumAmountRatioBPS = metrics.MinimumAmountInviteeCount * 10000 / metrics.PaidInviteeCount
		metrics.FirstPaymentSpanSeconds = metrics.LastFirstPaymentAt - metrics.FirstPaymentAt
	}
	return metrics, nil
}

func inviteBenefitPaymentKey(source string, row inviteBenefitPaymentRow) string {
	tradeNo := strings.TrimSpace(row.TradeNo)
	if tradeNo != "" {
		return inviteBenefitPaymentTradeKey(row.UserID, tradeNo)
	}
	return source + ":" + decimal.NewFromInt(int64(row.ID)).String()
}

func inviteBenefitPaymentTradeKey(userId int, tradeNo string) string {
	return "trade:" + strconv.Itoa(userId) + ":" + strings.TrimSpace(tradeNo)
}

func inviteBenefitUSDMinor(currency string, paymentProvider string, money float64) (int64, bool) {
	if !strings.EqualFold(strings.TrimSpace(currency), "USD") {
		return 0, false
	}
	if strings.EqualFold(strings.TrimSpace(paymentProvider), PaymentProviderBalance) {
		return 0, false
	}
	// Money is the successful settlement's actual paid amount. Do not fall back to
	// PaymentAmountMinor: for top-ups that field may be the pre-discount checkout
	// price and would overstate real payment after a promotion.
	amountMinor := decimal.NewFromFloat(money).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
	if amountMinor <= 0 {
		return 0, false
	}
	return amountMinor, true
}
