package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
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
	validTopUps := make([]TopUp, 0, len(topUps))
	userIds := make([]int, 0, len(topUps))
	seenUserIds := map[int]struct{}{}
	minCreateTime := int64(0)
	for _, topUp := range topUps {
		if topUp.UserId == 0 || strings.TrimSpace(topUp.TradeNo) == "" {
			continue
		}
		validTopUps = append(validTopUps, topUp)
		if _, ok := seenUserIds[topUp.UserId]; !ok {
			seenUserIds[topUp.UserId] = struct{}{}
			userIds = append(userIds, topUp.UserId)
		}
		if minCreateTime == 0 || topUp.CreateTime < minCreateTime {
			minCreateTime = topUp.CreateTime
		}
	}
	if len(validTopUps) == 0 {
		return candidates, nil
	}

	activeRecallUserIds, err := topUpRecallActiveUserIds(userIds)
	if err != nil {
		return nil, err
	}
	usersById, err := topUpRecallUsersById(userIds)
	if err != nil {
		return nil, err
	}
	successCreateTimesByUserId, err := successfulStripeTopUpCreateTimesByUserId(userIds, minCreateTime)
	if err != nil {
		return nil, err
	}

	for _, topUp := range validTopUps {
		if len(candidates) >= limit {
			break
		}
		if activeRecallUserIds[topUp.UserId] {
			continue
		}
		user, ok := usersById[topUp.UserId]
		if !ok {
			continue
		}
		email := strings.TrimSpace(user.Email)
		if email == "" || isInternalTopUpRecallEmail(email) {
			continue
		}
		if hasSuccessfulStripeTopUpCreateTimeAfter(successCreateTimesByUserId[topUp.UserId], topUp.CreateTime) {
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
	if hasSuccessfulStripeTopUpAfter(candidate.UserId, candidate.CreateTime) {
		return nil, false, nil
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
		return reactivateFailedTopUpRecall(candidate, email, tradeNo)
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

func topUpRecallActiveStatuses() []string {
	return []string{TopUpRecallStatusPending, TopUpRecallStatusSent}
}

func topUpRecallActiveUserIds(userIds []int) (map[int]bool, error) {
	activeUserIds := map[int]bool{}
	if len(userIds) == 0 {
		return activeUserIds, nil
	}
	var recalls []TopUpRecall
	err := DB.Select("user_id").
		Where("user_id IN ? AND status IN ?", userIds, topUpRecallActiveStatuses()).
		Find(&recalls).Error
	if err != nil {
		return nil, err
	}
	for _, recall := range recalls {
		activeUserIds[recall.UserId] = true
	}
	return activeUserIds, nil
}

func topUpRecallUsersById(userIds []int) (map[int]User, error) {
	usersById := map[int]User{}
	if len(userIds) == 0 {
		return usersById, nil
	}
	var users []User
	err := DB.Select("id", "email", "setting").
		Where("id IN ?", userIds).
		Find(&users).Error
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		usersById[user.Id] = user
	}
	return usersById, nil
}

func successfulStripeTopUpCreateTimesByUserId(userIds []int, minCreateTime int64) (map[int][]int64, error) {
	createTimesByUserId := map[int][]int64{}
	if len(userIds) == 0 {
		return createTimesByUserId, nil
	}
	var topUps []TopUp
	err := DB.Select("user_id", "create_time").
		Where("user_id IN ? AND payment_provider = ? AND status = ? AND create_time > ?", userIds, PaymentProviderStripe, common.TopUpStatusSuccess, minCreateTime).
		Find(&topUps).Error
	if err != nil {
		return nil, err
	}
	for _, topUp := range topUps {
		createTimesByUserId[topUp.UserId] = append(createTimesByUserId[topUp.UserId], topUp.CreateTime)
	}
	return createTimesByUserId, nil
}

func hasSuccessfulStripeTopUpCreateTimeAfter(createTimes []int64, createTime int64) bool {
	for _, successCreateTime := range createTimes {
		if successCreateTime > createTime {
			return true
		}
	}
	return false
}

func reactivateFailedTopUpRecall(candidate TopUpRecallCandidate, email string, tradeNo string) (*TopUpRecall, bool, error) {
	recall := TopUpRecall{}
	err := DB.Where("status = ? AND (user_id = ? OR trade_no = ?)", TopUpRecallStatusFailed, candidate.UserId, tradeNo).
		First(&recall).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, err
		}
		return nil, false, nil
	}

	updates := map[string]any{
		"trade_no":                 tradeNo,
		"email":                    email,
		"amount":                   candidate.Amount,
		"status":                   TopUpRecallStatusPending,
		"promotion_code":           "",
		"stripe_promotion_code_id": "",
		"error":                    "",
		"sent_at":                  0,
	}
	result := DB.Model(&TopUpRecall{}).
		Where("id = ? AND status = ?", recall.Id, TopUpRecallStatusFailed).
		Updates(updates)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, false, nil
	}
	recall.TradeNo = tradeNo
	recall.Email = email
	recall.Amount = candidate.Amount
	recall.Status = TopUpRecallStatusPending
	recall.PromotionCode = ""
	recall.StripePromotionCodeId = ""
	recall.Error = ""
	recall.SentAt = 0
	return &recall, true, nil
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
