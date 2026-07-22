package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/stripe/stripe-go/v81"
	stripesubscription "github.com/stripe/stripe-go/v81/subscription"
)

const (
	stripeSubscriptionReconciliationTickInterval = 15 * time.Minute
	stripeSubscriptionReconciliationBatchSize    = 100
)

var (
	stripeSubscriptionReconciliationOnce               sync.Once
	stripeSubscriptionReconciliationRunning            atomic.Bool
	stripeSubscriptionSnapshotForReconciliation        = getStripeSubscriptionSnapshotForReconciliation
	reconcileStripeInvoiceCollectionForCanceledBinding = reconcileStripeInvoiceCollectionForCanceledBindingNoop
)

func StartStripeSubscriptionReconciliationTask() {
	stripeSubscriptionReconciliationOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("Stripe subscription reconciliation task started: tick=%s", stripeSubscriptionReconciliationTickInterval))
			ticker := time.NewTicker(stripeSubscriptionReconciliationTickInterval)
			defer ticker.Stop()

			runStripeSubscriptionReconciliationOnceLogged()
			for range ticker.C {
				runStripeSubscriptionReconciliationOnceLogged()
			}
		})
	})
}

func runStripeSubscriptionReconciliationOnceLogged() {
	count, err := RunStripeSubscriptionReconciliationOnce()
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("Stripe subscription reconciliation failed after %d binding(s): %v", count, err))
		return
	}
	if common.DebugEnabled && count > 0 {
		logger.LogDebug(context.Background(), "Stripe subscription reconciliation processed_count=%d", count)
	}
}

func RunStripeSubscriptionReconciliationOnce() (int, error) {
	if !common.IsMasterNode {
		return 0, nil
	}
	if !stripeSubscriptionReconciliationRunning.CompareAndSwap(false, true) {
		return 0, nil
	}
	defer stripeSubscriptionReconciliationRunning.Store(false)

	var bindings []model.SubscriptionProviderBinding
	if err := model.DB.Where("provider = ? AND provider_subscription_id <> ? AND ended_at = ?",
		model.PaymentProviderStripe, "", 0).
		Where("provider_status NOT IN ?", []string{"canceled", "incomplete_expired", "unpaid"}).
		Order("id asc").
		Limit(stripeSubscriptionReconciliationBatchSize).
		Find(&bindings).Error; err != nil {
		return 0, err
	}

	processed := 0
	for _, binding := range bindings {
		snapshot, err := stripeSubscriptionSnapshotForReconciliation(binding.ProviderSubscriptionId)
		if err != nil {
			return processed, err
		}
		if strings.TrimSpace(snapshot.ProviderSubscriptionId) == "" {
			snapshot.ProviderSubscriptionId = binding.ProviderSubscriptionId
		}
		if snapshot.EndedAt > 0 || isTerminalStripeSubscriptionStatus(snapshot.ProviderStatus) {
			updated, err := model.ApplyProviderSubscriptionTermination(binding.Id, snapshot)
			if err != nil {
				return processed, err
			}
			if err := reconcileStripeInvoiceCollectionForCanceledBinding(*updated); err != nil {
				return processed, err
			}
		} else {
			if _, err := model.ApplyProviderSubscriptionSnapshot(binding.Id, snapshot); err != nil {
				return processed, err
			}
		}
		processed++
	}
	return processed, nil
}

func getStripeSubscriptionSnapshotForReconciliation(providerSubscriptionID string) (model.ProviderSubscriptionSnapshot, error) {
	if err := ensureStripeLifecycleKey(); err != nil {
		return model.ProviderSubscriptionSnapshot{}, err
	}
	params := &stripe.SubscriptionParams{}
	params.AddExpand("latest_invoice")
	params.AddExpand("items.data.price")
	sub, err := stripesubscription.Get(strings.TrimSpace(providerSubscriptionID), params)
	if err != nil {
		return model.ProviderSubscriptionSnapshot{}, err
	}
	return providerSubscriptionSnapshotFromStripe(sub), nil
}

func reconcileStripeInvoiceCollectionForCanceledBindingNoop(binding model.SubscriptionProviderBinding) error {
	return nil
}
