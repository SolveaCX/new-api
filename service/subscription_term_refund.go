package service

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const subscriptionTermStatusCompleted = "completed"

type SubscriptionTermRefundResult struct {
	TermSegmentID int64
	RefundedQuota int64
	RefundedMoney float64
	RefundKey     string
}

func RefundSubscriptionTermSegment(userID int, termSegmentID int64) (*SubscriptionTermRefundResult, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	if termSegmentID <= 0 {
		return nil, errors.New("invalid term segment id")
	}

	var result *SubscriptionTermRefundResult
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var term model.SubscriptionTermSegment
		if err := subscriptionCommandLock(tx).Where("id = ?", termSegmentID).First(&term).Error; err != nil {
			return err
		}
		if term.Status != model.SubscriptionTermStatusNotStarted {
			return fmt.Errorf("subscription term must be not_started, got %s", term.Status)
		}
		var order model.SubscriptionOrder
		if err := subscriptionCommandLock(tx).
			Where("id = ? AND user_id = ?", term.OrderId, userID).
			First(&order).Error; err != nil {
			return err
		}
		if order.PaymentProvider == model.PaymentProviderStripe && order.PaymentMethod == model.PaymentMethodStripe {
			return errors.New("subscription term refund only supports one-time terms")
		}
		refundQuota, err := subscriptionMoneyQuota(term.AllocatedMoney)
		if err != nil {
			return err
		}
		refundKey := fmt.Sprintf("subscription:term:refund:%d", term.Id)
		if err := tx.Create(&model.WalletLedgerEntry{
			UserId:        userID,
			EntryKey:      refundKey,
			QuotaDelta:    int64(refundQuota),
			MoneyAmount:   term.AllocatedMoney,
			EntryType:     model.WalletLedgerEntryTypePrepaidRefund,
			OrderId:       term.OrderId,
			TermSegmentId: term.Id,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.SubscriptionTermSegment{}).
			Where("id = ? AND status = ?", term.Id, model.SubscriptionTermStatusNotStarted).
			Updates(map[string]interface{}{
				"status":     model.SubscriptionTermStatusRefunded,
				"refund_key": refundKey,
			}).Error; err != nil {
			return err
		}
		if refundQuota > 0 {
			if err := tx.Model(&model.User{}).Where("id = ?", userID).
				Update("quota", gorm.Expr("quota + ?", refundQuota)).Error; err != nil {
				return err
			}
		}
		result = &SubscriptionTermRefundResult{
			TermSegmentID: term.Id,
			RefundedQuota: int64(refundQuota),
			RefundedMoney: term.AllocatedMoney,
			RefundKey:     refundKey,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func RunSubscriptionTermSegmentAdvanceOnce(limit int) (int, error) {
	if limit <= 0 {
		limit = subscriptionResetBatchSize
	}
	now := common.GetTimestamp()
	advanced := 0
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var completeIDs []int64
		if err := tx.Model(&model.SubscriptionTermSegment{}).
			Where("status = ? AND end_time > ? AND end_time <= ?", model.SubscriptionTermStatusActive, 0, now).
			Order("end_time asc, id asc").
			Limit(limit).
			Pluck("id", &completeIDs).Error; err != nil {
			return err
		}
		if len(completeIDs) > 0 {
			res := tx.Model(&model.SubscriptionTermSegment{}).
				Where("id IN ? AND status = ?", completeIDs, model.SubscriptionTermStatusActive).
				Update("status", subscriptionTermStatusCompleted)
			if res.Error != nil {
				return res.Error
			}
			advanced += int(res.RowsAffected)
		}

		remaining := limit - advanced
		if remaining <= 0 {
			return nil
		}
		var activateIDs []int64
		if err := tx.Model(&model.SubscriptionTermSegment{}).
			Where("status = ? AND start_time <= ? AND end_time > ?", model.SubscriptionTermStatusNotStarted, now, now).
			Order("start_time asc, id asc").
			Limit(remaining).
			Pluck("id", &activateIDs).Error; err != nil {
			return err
		}
		if len(activateIDs) == 0 {
			return nil
		}
		res := tx.Model(&model.SubscriptionTermSegment{}).
			Where("id IN ? AND status = ?", activateIDs, model.SubscriptionTermStatusNotStarted).
			Update("status", model.SubscriptionTermStatusActive)
		if res.Error != nil {
			return res.Error
		}
		advanced += int(res.RowsAffected)
		return nil
	})
	if err != nil {
		return advanced, err
	}
	return advanced, nil
}
