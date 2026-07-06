package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm/clause"
)

const (
	TopUpRecallStatusPending = "pending"
	TopUpRecallStatusSent    = "sent"
	TopUpRecallStatusFailed  = "failed"

	TopUpRecallDelaySeconds int64 = 60 * 60
)

var internalTopUpRecallEmailDomains = []string{"voc.ai", "solvea.cx", "qq.com"}

type TopUpRecall struct {
	Id                    int    `json:"id"`
	UserId                int    `json:"user_id" gorm:"not null;uniqueIndex"`
	TradeNo               string `json:"trade_no" gorm:"type:varchar(255);not null;uniqueIndex"`
	Email                 string `json:"email" gorm:"type:varchar(255);not null;default:''"`
	Amount                int64  `json:"amount" gorm:"not null;default:0"`
	Status                string `json:"status" gorm:"type:varchar(32);not null;default:'pending';index"`
	PromotionCode         string `json:"promotion_code" gorm:"type:varchar(64);not null;default:''"`
	StripePromotionCodeId string `json:"stripe_promotion_code_id" gorm:"type:varchar(128);not null;default:''"`
	Error                 string `json:"error" gorm:"type:text"`
	SentAt                int64  `json:"sent_at" gorm:"not null;default:0"`
	CreatedAt             int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt             int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

type TopUpRecallCandidate struct {
	TopUpId    int
	UserId     int
	TradeNo    string
	Email      string
	Language   string
	Amount     int64
	CreateTime int64
}

func GetEligibleTopUpRecallCandidates(now int64, limit int) ([]TopUpRecallCandidate, error) {
	if DB == nil {
		return nil, errors.New("database is not initialized")
	}
	if now <= 0 {
		now = common.GetTimestamp()
	}
	if limit <= 0 {
		limit = 50
	}

	var topUps []TopUp
	fetchLimit := limit * 10
	if fetchLimit < limit {
		fetchLimit = limit
	}
	cutoff := now - TopUpRecallDelaySeconds
	err := DB.Where("payment_provider = ? AND status = ?", PaymentProviderStripe, common.TopUpStatusExpired).
		Where("(complete_time > 0 AND complete_time <= ?) OR (complete_time = 0 AND create_time <= ?)", cutoff, cutoff).
		Order("id asc").
		Limit(fetchLimit).
		Find(&topUps).Error
	if err != nil {
		return nil, err
	}

	candidates := make([]TopUpRecallCandidate, 0, limit)
	for _, topUp := range topUps {
		if len(candidates) >= limit {
			break
		}
		if topUp.UserId == 0 || strings.TrimSpace(topUp.TradeNo) == "" {
			continue
		}
		if hasTopUpRecallForUser(topUp.UserId) {
			continue
		}
		user := User{}
		if err := DB.Select("id", "email", "setting").First(&user, "id = ?", topUp.UserId).Error; err != nil {
			continue
		}
		email := strings.TrimSpace(user.Email)
		if email == "" || isInternalTopUpRecallEmail(email) {
			continue
		}
		if hasSuccessfulStripeTopUpAfter(topUp.UserId, topUp.CreateTime) {
			continue
		}
		candidates = append(candidates, TopUpRecallCandidate{
			TopUpId:    topUp.Id,
			UserId:     topUp.UserId,
			TradeNo:    topUp.TradeNo,
			Email:      email,
			Language:   user.GetSetting().Language,
			Amount:     topUp.Amount,
			CreateTime: topUp.CreateTime,
		})
	}

	return candidates, nil
}

func ReserveTopUpRecall(candidate TopUpRecallCandidate) (*TopUpRecall, bool, error) {
	if DB == nil {
		return nil, false, errors.New("database is not initialized")
	}
	email := strings.TrimSpace(candidate.Email)
	tradeNo := strings.TrimSpace(candidate.TradeNo)
	if candidate.UserId == 0 || tradeNo == "" || email == "" {
		return nil, false, errors.New("invalid top-up recall candidate")
	}

	recall := &TopUpRecall{
		UserId:  candidate.UserId,
		TradeNo: tradeNo,
		Email:   email,
		Amount:  candidate.Amount,
		Status:  TopUpRecallStatusPending,
	}

	result := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(recall)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, false, nil
	}
	return recall, true, nil
}

func MarkTopUpRecallSent(id int, promotionCode string, stripePromotionCodeId string) error {
	if DB == nil || id == 0 {
		return nil
	}
	return DB.Model(&TopUpRecall{}).
		Where("id = ? AND status = ?", id, TopUpRecallStatusPending).
		Updates(map[string]any{
			"status":                   TopUpRecallStatusSent,
			"promotion_code":           strings.TrimSpace(promotionCode),
			"stripe_promotion_code_id": strings.TrimSpace(stripePromotionCodeId),
			"sent_at":                  common.GetTimestamp(),
			"error":                    "",
		}).Error
}

func MarkTopUpRecallFailed(id int, err error) error {
	if DB == nil || id == 0 || err == nil {
		return nil
	}
	return DB.Model(&TopUpRecall{}).
		Where("id = ? AND status = ?", id, TopUpRecallStatusPending).
		Updates(map[string]any{
			"status": TopUpRecallStatusFailed,
			"error":  err.Error(),
		}).Error
}

func hasTopUpRecallForUser(userId int) bool {
	var count int64
	if err := DB.Model(&TopUpRecall{}).Where("user_id = ?", userId).Limit(1).Count(&count).Error; err != nil {
		return true
	}
	return count > 0
}

func hasSuccessfulStripeTopUpAfter(userId int, createTime int64) bool {
	var count int64
	err := DB.Model(&TopUp{}).
		Where("user_id = ? AND payment_provider = ? AND status = ? AND create_time > ?", userId, PaymentProviderStripe, common.TopUpStatusSuccess, createTime).
		Limit(1).
		Count(&count).Error
	if err != nil {
		return true
	}
	return count > 0
}

func isInternalTopUpRecallEmail(email string) bool {
	domain := strings.ToLower(strings.TrimSpace(email))
	at := strings.LastIndex(domain, "@")
	if at >= 0 {
		domain = domain[at+1:]
	}
	for _, internalDomain := range internalTopUpRecallEmailDomains {
		if domain == internalDomain || strings.HasSuffix(domain, "."+internalDomain) {
			return true
		}
	}
	return false
}
