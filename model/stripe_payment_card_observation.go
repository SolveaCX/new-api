package model

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	StripeCardRewardDecisionUnknown            = "unknown"
	StripeCardRewardDecisionNotCard            = "not_card"
	StripeCardRewardDecisionNonCard            = StripeCardRewardDecisionNotCard
	StripeCardRewardDecisionFirstExactCard     = "first_exact_fingerprint"
	StripeCardRewardDecisionSameUserExactCard  = "same_user_exact_fingerprint"
	StripeCardRewardDecisionDuplicateExactCard = "duplicate_exact_fingerprint"

	StripeCardRiskMatchExactFingerprint    = "exact_fingerprint"
	StripeCardRiskMatchSuspectedAttributes = "suspected_card_attributes"
)

// StripePaymentCardObservation is an immutable snapshot of the card details Stripe returned
// for one successful Charge. It deliberately stores only Stripe identifiers and display/risk
// attributes; PAN, CVC and full billing details must never be persisted here.
//
// ChargeID is the multi-node idempotency key. Stripe can redeliver webhook events and multiple
// application instances can process the same event, but the database accepts only one row.
type StripePaymentCardObservation struct {
	Id                int    `json:"id"`
	UserId            int    `json:"user_id" gorm:"index"`
	TradeNo           string `json:"trade_no" gorm:"type:varchar(128);index"`
	CheckoutSessionId string `json:"checkout_session_id" gorm:"type:varchar(128);index"`
	PaymentIntentId   string `json:"payment_intent_id" gorm:"type:varchar(128);index"`
	ChargeId          string `json:"charge_id" gorm:"type:varchar(128);uniqueIndex"`
	StripeCustomerId  string `json:"stripe_customer_id" gorm:"type:varchar(128);index"`
	PaymentMethodId   string `json:"payment_method_id" gorm:"type:varchar(128);index"`
	PaymentMethodType string `json:"payment_method_type" gorm:"type:varchar(32);index"`
	Fingerprint       string `json:"fingerprint" gorm:"type:varchar(128);index"`
	Brand             string `json:"brand" gorm:"type:varchar(32)"`
	Last4             string `json:"last4" gorm:"type:varchar(4)"`
	ExpMonth          int64  `json:"exp_month"`
	ExpYear           int64  `json:"exp_year"`
	Country           string `json:"country" gorm:"type:varchar(8)"`
	Funding           string `json:"funding" gorm:"type:varchar(32)"`
	WalletType        string `json:"wallet_type" gorm:"type:varchar(32)"`
	DynamicLast4      string `json:"dynamic_last4" gorm:"type:varchar(4)"`
	RewardDecision    string `json:"reward_decision" gorm:"type:varchar(48);index"`
	RewardClaimUserId int    `json:"reward_claim_user_id" gorm:"index"`
	ObservedAt        int64  `json:"observed_at" gorm:"bigint;index"`
}

type StripePaymentCardBackfillLease struct {
	Key        string `json:"key" gorm:"column:lease_key;type:varchar(64);primaryKey"`
	LeaseUntil int64  `json:"lease_until" gorm:"bigint;index"`
	OwnerToken string `json:"owner_token" gorm:"type:varchar(96);index"`
	UpdatedAt  int64  `json:"updated_at" gorm:"bigint"`
}

type StripeCardRiskUser struct {
	UserId            int    `json:"user_id"`
	Email             string `json:"email"`
	TradeNo           string `json:"trade_no"`
	FingerprintDigest string `json:"fingerprint_digest,omitempty"`
	ObservedAt        int64  `json:"observed_at"`
	RewardDecision    string `json:"reward_decision"`
}

type StripeCardRiskGroup struct {
	MatchBasis        string               `json:"match_basis"`
	FingerprintDigest string               `json:"fingerprint_digest,omitempty"`
	FingerprintCount  int                  `json:"fingerprint_count"`
	UserCount         int                  `json:"user_count"`
	ChargeCount       int                  `json:"charge_count"`
	Brand             string               `json:"brand"`
	Last4             string               `json:"last4"`
	ExpMonth          int64                `json:"exp_month"`
	ExpYear           int64                `json:"exp_year"`
	Country           string               `json:"country"`
	Funding           string               `json:"funding"`
	WalletType        string               `json:"wallet_type"`
	DynamicLast4      string               `json:"dynamic_last4"`
	Users             []StripeCardRiskUser `json:"users"`
}

func normalizeStripeCardObservation(observation *StripePaymentCardObservation) {
	observation.TradeNo = strings.TrimSpace(observation.TradeNo)
	observation.CheckoutSessionId = strings.TrimSpace(observation.CheckoutSessionId)
	observation.PaymentIntentId = strings.TrimSpace(observation.PaymentIntentId)
	observation.ChargeId = strings.TrimSpace(observation.ChargeId)
	observation.StripeCustomerId = strings.TrimSpace(observation.StripeCustomerId)
	observation.PaymentMethodId = strings.TrimSpace(observation.PaymentMethodId)
	observation.PaymentMethodType = strings.ToLower(strings.TrimSpace(observation.PaymentMethodType))
	observation.Fingerprint = strings.TrimSpace(observation.Fingerprint)
	observation.Brand = strings.ToLower(strings.TrimSpace(observation.Brand))
	observation.Last4 = strings.TrimSpace(observation.Last4)
	observation.Country = strings.ToUpper(strings.TrimSpace(observation.Country))
	observation.Funding = strings.ToLower(strings.TrimSpace(observation.Funding))
	observation.WalletType = strings.ToLower(strings.TrimSpace(observation.WalletType))
	observation.DynamicLast4 = strings.TrimSpace(observation.DynamicLast4)
	if observation.ObservedAt <= 0 {
		observation.ObservedAt = common.GetTimestamp()
	}
	if observation.RewardDecision == "" {
		observation.RewardDecision = StripeCardRewardDecisionUnknown
	}
}

// ObserveStripePaymentCard persists one successful card Charge and atomically records the
// exact-fingerprint reward decision. The decision is an audit/enforcement input for future
// rewards; this function never deducts already-granted quota or purchased balance.
func ObserveStripePaymentCard(observation *StripePaymentCardObservation) (bool, error) {
	if observation == nil {
		return false, nil
	}
	normalizeStripeCardObservation(observation)
	if observation.ChargeId == "" {
		return false, nil
	}
	if observation.PaymentMethodType == "" && observation.Fingerprint != "" {
		observation.PaymentMethodType = "card"
	}
	// Reward decisions are derived from durable data inside the transaction. Callers may
	// supply attribution and Stripe snapshots, but must never be able to choose a decision.
	observation.RewardDecision = StripeCardRewardDecisionUnknown
	observation.RewardClaimUserId = 0

	created := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		insert := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "charge_id"}},
			DoNothing: true,
		}).Create(observation)
		if insert.Error != nil {
			return insert.Error
		}
		created = insert.RowsAffected > 0
		if !created {
			// A charge.succeeded event can win the race before checkout.session.completed.
			// Preserve the immutable card snapshot, but fill attribution fields that were
			// genuinely missing so the later top-up transaction can enforce reward policy.
			if err := tx.Model(&StripePaymentCardObservation{}).
				Where("charge_id = ?", observation.ChargeId).
				Updates(map[string]any{
					"user_id":             gorm.Expr("CASE WHEN user_id = 0 THEN ? ELSE user_id END", observation.UserId),
					"trade_no":            gorm.Expr("CASE WHEN trade_no = '' THEN ? ELSE trade_no END", observation.TradeNo),
					"checkout_session_id": gorm.Expr("CASE WHEN checkout_session_id = '' THEN ? ELSE checkout_session_id END", observation.CheckoutSessionId),
					"payment_intent_id":   gorm.Expr("CASE WHEN payment_intent_id = '' THEN ? ELSE payment_intent_id END", observation.PaymentIntentId),
					"stripe_customer_id":  gorm.Expr("CASE WHEN stripe_customer_id = '' THEN ? ELSE stripe_customer_id END", observation.StripeCustomerId),
					"payment_method_id":   gorm.Expr("CASE WHEN payment_method_id = '' THEN ? ELSE payment_method_id END", observation.PaymentMethodId),
					"payment_method_type": gorm.Expr("CASE WHEN payment_method_type = '' THEN ? ELSE payment_method_type END", observation.PaymentMethodType),
					"fingerprint":         gorm.Expr("CASE WHEN fingerprint = '' THEN ? ELSE fingerprint END", observation.Fingerprint),
					"brand":               gorm.Expr("CASE WHEN brand = '' THEN ? ELSE brand END", observation.Brand),
					"last4":               gorm.Expr("CASE WHEN last4 = '' THEN ? ELSE last4 END", observation.Last4),
					"exp_month":           gorm.Expr("CASE WHEN exp_month = 0 THEN ? ELSE exp_month END", observation.ExpMonth),
					"exp_year":            gorm.Expr("CASE WHEN exp_year = 0 THEN ? ELSE exp_year END", observation.ExpYear),
					"country":             gorm.Expr("CASE WHEN country = '' THEN ? ELSE country END", observation.Country),
					"funding":             gorm.Expr("CASE WHEN funding = '' THEN ? ELSE funding END", observation.Funding),
					"wallet_type":         gorm.Expr("CASE WHEN wallet_type = '' THEN ? ELSE wallet_type END", observation.WalletType),
					"dynamic_last4":       gorm.Expr("CASE WHEN dynamic_last4 = '' THEN ? ELSE dynamic_last4 END", observation.DynamicLast4),
				}).Error; err != nil {
				return err
			}
		}

		var stored StripePaymentCardObservation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("charge_id = ?", observation.ChargeId).First(&stored).Error; err != nil {
			return err
		}
		if stored.UserId <= 0 || stored.RewardDecision != StripeCardRewardDecisionUnknown {
			return nil
		}

		decision := StripeCardRewardDecisionUnknown
		claimUserId := 0
		switch stored.PaymentMethodType {
		case "":
			return nil
		case "card":
			if stored.Fingerprint == "" {
				return nil
			}
			won, err := claimBonusForFingerprint(tx, stored.UserId, stored.Fingerprint)
			if err != nil {
				return err
			}
			var claim StripeBonusClaim
			if err := tx.Where("card_fingerprint = ?", stored.Fingerprint).First(&claim).Error; err != nil {
				return err
			}
			claimUserId = claim.UserId
			switch {
			case won:
				decision = StripeCardRewardDecisionFirstExactCard
			case claim.UserId == stored.UserId:
				decision = StripeCardRewardDecisionSameUserExactCard
			default:
				decision = StripeCardRewardDecisionDuplicateExactCard
			}
		default:
			decision = StripeCardRewardDecisionNotCard
		}
		return tx.Model(&StripePaymentCardObservation{}).
			Where("id = ? AND reward_decision = ?", stored.Id, StripeCardRewardDecisionUnknown).
			Updates(map[string]any{
				"reward_decision":      decision,
				"reward_claim_user_id": claimUserId,
			}).Error
	})
	return created, err
}

func AcquireStripeCardBackfillLease(key, ownerToken string, now, leaseUntil int64) (bool, error) {
	key = strings.TrimSpace(key)
	ownerToken = strings.TrimSpace(ownerToken)
	if key == "" || ownerToken == "" || leaseUntil <= now {
		return false, nil
	}
	acquired := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		seed := StripePaymentCardBackfillLease{Key: key}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
			return err
		}
		result := tx.Model(&StripePaymentCardBackfillLease{}).
			Where("lease_key = ? AND (owner_token = ? OR owner_token = '' OR lease_until <= ?)", key, ownerToken, now).
			Updates(map[string]any{"owner_token": ownerToken, "lease_until": leaseUntil, "updated_at": now})
		acquired = result.RowsAffected == 1
		return result.Error
	})
	return acquired, err
}

func ReleaseStripeCardBackfillLease(key, ownerToken string) error {
	return DB.Model(&StripePaymentCardBackfillLease{}).
		Where("lease_key = ? AND owner_token = ?", strings.TrimSpace(key), strings.TrimSpace(ownerToken)).
		Updates(map[string]any{"owner_token": "", "lease_until": int64(0), "updated_at": common.GetTimestamp()}).Error
}

func stripeFingerprintDigest(fingerprint string) string {
	fingerprint = strings.TrimSpace(fingerprint)
	if fingerprint == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(fingerprint))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func MaskStripeCardRiskEmail(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return "***"
	}
	return "***" + email[at:]
}

// GetStripeCardRiskGroups returns exact fingerprint collisions and secondary suspected
// attribute groups. Only exact fingerprint decisions are enforcement inputs; suspected
// brand/last4/expiry/country groups are data for human review and never create claims.
func GetStripeCardRiskGroups(limit int) ([]StripeCardRiskGroup, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	type fingerprintCount struct {
		Fingerprint string
		UserCount   int
		ChargeCount int
	}
	type attributeCount struct {
		Brand            string
		Last4            string
		ExpMonth         int64
		ExpYear          int64
		Country          string
		UserCount        int
		ChargeCount      int
		FingerprintCount int
	}
	type riskGroupRows struct {
		group        StripeCardRiskGroup
		observations []StripePaymentCardObservation
	}

	var exactCounts []fingerprintCount
	if err := DB.Model(&StripePaymentCardObservation{}).
		Select("fingerprint", "COUNT(DISTINCT user_id) AS user_count", "COUNT(*) AS charge_count").
		Where("fingerprint <> ? AND user_id > 0", "").
		Group("fingerprint").
		Having("COUNT(DISTINCT user_id) > 1").
		Order("user_count DESC, charge_count DESC").
		Limit(limit).
		Scan(&exactCounts).Error; err != nil {
		return nil, err
	}

	groupRows := make([]riskGroupRows, 0, limit)
	if len(exactCounts) > 0 {
		fingerprints := make([]string, 0, len(exactCounts))
		groupIndexes := make(map[string]int, len(exactCounts))
		for _, count := range exactCounts {
			fingerprints = append(fingerprints, count.Fingerprint)
			groupRows = append(groupRows, riskGroupRows{group: StripeCardRiskGroup{
				MatchBasis:        StripeCardRiskMatchExactFingerprint,
				FingerprintDigest: stripeFingerprintDigest(count.Fingerprint),
				FingerprintCount:  1,
				UserCount:         count.UserCount,
				ChargeCount:       count.ChargeCount,
				Users:             []StripeCardRiskUser{},
			}})
			groupIndexes[count.Fingerprint] = len(groupRows) - 1
		}
		var exactObservations []StripePaymentCardObservation
		if err := DB.Where("fingerprint IN ?", fingerprints).
			Order("observed_at DESC, id DESC").
			Find(&exactObservations).Error; err != nil {
			return nil, err
		}
		for _, observation := range exactObservations {
			index, ok := groupIndexes[observation.Fingerprint]
			if !ok {
				continue
			}
			groupRows[index].observations = append(groupRows[index].observations, observation)
		}
	}

	remaining := limit - len(groupRows)
	if remaining > 0 {
		var suspectedCounts []attributeCount
		if err := DB.Model(&StripePaymentCardObservation{}).
			Select("brand", "last4", "exp_month", "exp_year", "country", "COUNT(DISTINCT user_id) AS user_count", "COUNT(*) AS charge_count", "COUNT(DISTINCT fingerprint) AS fingerprint_count").
			Where("user_id > 0 AND fingerprint <> ? AND brand <> ? AND last4 <> ? AND exp_month > 0 AND exp_year > 0 AND country <> ?", "", "", "", "").
			Group("brand, last4, exp_month, exp_year, country").
			Having("COUNT(DISTINCT fingerprint) > 1 AND COUNT(DISTINCT user_id) > 1").
			Order("user_count DESC, fingerprint_count DESC, charge_count DESC").
			Limit(remaining).
			Scan(&suspectedCounts).Error; err != nil {
			return nil, err
		}
		for _, count := range suspectedCounts {
			var suspectedObservations []StripePaymentCardObservation
			if err := DB.Where("user_id > 0 AND fingerprint <> ? AND brand = ? AND last4 = ? AND exp_month = ? AND exp_year = ? AND country = ?",
				"", count.Brand, count.Last4, count.ExpMonth, count.ExpYear, count.Country).
				Order("observed_at DESC, id DESC").
				Find(&suspectedObservations).Error; err != nil {
				return nil, err
			}
			groupRows = append(groupRows, riskGroupRows{
				group: StripeCardRiskGroup{
					MatchBasis:       StripeCardRiskMatchSuspectedAttributes,
					FingerprintCount: count.FingerprintCount,
					UserCount:        count.UserCount,
					ChargeCount:      count.ChargeCount,
					Brand:            count.Brand,
					Last4:            count.Last4,
					ExpMonth:         count.ExpMonth,
					ExpYear:          count.ExpYear,
					Country:          count.Country,
					Users:            []StripeCardRiskUser{},
				},
				observations: suspectedObservations,
			})
		}
	}

	userIds := make([]int, 0)
	seenUser := map[int]bool{}
	for _, rows := range groupRows {
		for _, observation := range rows.observations {
			if observation.UserId > 0 && !seenUser[observation.UserId] {
				seenUser[observation.UserId] = true
				userIds = append(userIds, observation.UserId)
			}
		}
	}
	var users []User
	if len(userIds) > 0 {
		if err := DB.Select("id", "email").Where("id IN ?", userIds).Find(&users).Error; err != nil {
			return nil, err
		}
	}
	emails := make(map[int]string, len(users))
	for _, user := range users {
		emails[user.Id] = MaskStripeCardRiskEmail(user.Email)
	}

	groups := make([]StripeCardRiskGroup, 0, len(groupRows))
	for _, rows := range groupRows {
		group := rows.group
		for _, observation := range rows.observations {
			if group.Brand == "" {
				group.Brand = observation.Brand
				group.Last4 = observation.Last4
				group.ExpMonth = observation.ExpMonth
				group.ExpYear = observation.ExpYear
				group.Country = observation.Country
				group.Funding = observation.Funding
				group.WalletType = observation.WalletType
				group.DynamicLast4 = observation.DynamicLast4
			}
			group.Users = append(group.Users, StripeCardRiskUser{
				UserId:            observation.UserId,
				Email:             emails[observation.UserId],
				TradeNo:           observation.TradeNo,
				FingerprintDigest: stripeFingerprintDigest(observation.Fingerprint),
				ObservedAt:        observation.ObservedAt,
				RewardDecision:    observation.RewardDecision,
			})
		}
		groups = append(groups, group)
	}
	return groups, nil
}

// ShouldWithholdRewardForStripeFingerprint is the enforcement seam for reward flows that know
// the actual paid-card fingerprint before granting a benefit. Unknown/empty fingerprints are
// always allowed; callers must never treat a Stripe lookup failure as fraud. A duplicate exact
// fingerprint owned by another user withholds only the reward, never payment or account access.
func ShouldWithholdRewardForStripeFingerprint(userId int, fingerprint string) (bool, int, error) {
	fingerprint = strings.TrimSpace(fingerprint)
	if userId <= 0 || fingerprint == "" {
		return false, 0, nil
	}
	var claim StripeBonusClaim
	if err := DB.Select("user_id").Where("card_fingerprint = ?", fingerprint).First(&claim).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, 0, nil
		}
		return false, 0, err
	}
	return claim.UserId != userId, claim.UserId, nil
}

func GetUserIDByStripeCustomer(customerId string) (int, error) {
	customerId = strings.TrimSpace(customerId)
	if customerId == "" {
		return 0, nil
	}
	var user User
	if err := DB.Select("id").Where("stripe_customer = ?", customerId).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, nil
		}
		return 0, err
	}
	return user.Id, nil
}
