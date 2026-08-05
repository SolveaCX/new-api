package service

import (
	"context"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/bytedance/gopkg/util/gopool"
)

const paymentAnalyticsDeliveryBatchSize = 50

var paymentAnalyticsDeliveryOnce sync.Once

func deliverPaymentAnalyticsEvent(config GAConfig, event model.PaymentAnalyticsOutbox) error {
	occurredAt := event.OccurredAt
	if occurredAt <= 0 {
		occurredAt = common.GetTimestamp()
	}
	items := []map[string]any{{
		"item_id": event.ItemId, "item_name": event.ItemName,
		"item_category": event.ProductType, "price": event.Value, "quantity": 1,
	}}
	return SendGAEventWithConfig(config, GAEvent{
		Name: "purchase", ClientID: event.ClientId, SessionID: event.SessionId,
		Params: map[string]any{
			"transaction_id": event.TransactionId, "value": event.Value, "currency": event.Currency,
			"items": items, "payment_provider": event.PaymentProvider,
			"payment_method": event.PaymentMethod, "product_type": event.ProductType,
			"timestamp_micros": occurredAt * int64(time.Second/time.Microsecond),
		},
	})
}

func runPaymentAnalyticsDeliveryOnce(config GAConfig) {
	events, err := model.ClaimPaymentAnalyticsOutbox(paymentAnalyticsDeliveryBatchSize, common.GetTimestamp())
	if err != nil {
		logger.LogError(context.Background(), "payment analytics outbox claim failed: "+err.Error())
		return
	}
	for _, event := range events {
		if err := deliverPaymentAnalyticsEvent(config, event); err == nil {
			if completeErr := model.CompletePaymentAnalyticsOutbox(event.Id, event.ClaimedAt, common.GetTimestamp()); completeErr != nil {
				logger.LogError(context.Background(), "payment analytics outbox complete failed: "+completeErr.Error())
			}
		} else {
			var failErr error
			if IsGAPermanentDeliveryError(err) {
				failErr = model.DeadPaymentAnalyticsOutbox(event.Id, event.ClaimedAt, err.Error(), common.GetTimestamp())
			} else {
				failErr = model.FailPaymentAnalyticsOutbox(event.Id, event.ClaimedAt, event.Attempts, err.Error(), common.GetTimestamp())
			}
			if failErr != nil {
				logger.LogError(context.Background(), "payment analytics outbox retry scheduling failed: "+failErr.Error())
			}
		}
	}
}

// StartPaymentAnalyticsDeliveryTask is master-only. Conditional claims make
// delivery safe if a master transition briefly overlaps across nodes.
func StartPaymentAnalyticsDeliveryTask() {
	paymentAnalyticsDeliveryOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		config := DefaultGAConfig()
		if config.MeasurementID == "" || config.APISecret == "" {
			logger.LogInfo(context.Background(), "payment analytics delivery disabled: GA configuration incomplete")
			return
		}
		gopool.Go(func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			runPaymentAnalyticsDeliveryOnce(config)
			for range ticker.C {
				runPaymentAnalyticsDeliveryOnce(config)
			}
		})
	})
}
