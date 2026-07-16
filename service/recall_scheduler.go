package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/bytedance/gopkg/util/gopool"
)

type RecallRuntime struct {
	Campaigns  *RecallCampaignService
	Claims     *RecallClaimService
	Recipients *RecallRecipientWorker
}

var (
	recallRuntimeOnce   sync.Once
	recallRuntime       *RecallRuntime
	recallSchedulerOnce sync.Once
)

func GetRecallRuntime() *RecallRuntime {
	recallRuntimeOnce.Do(func() {
		stripeService := NewRecallStripeService(nil)
		claims := NewRecallClaimService()
		recallRuntime = &RecallRuntime{
			Campaigns: NewRecallCampaignService(
				NewRecallAudienceSelector(),
				stripeService,
			),
			Claims:     claims,
			Recipients: NewRecallRecipientWorker(stripeService, claims, common.GetReplicaID()),
		}
	})
	return recallRuntime
}

func StartRecallCampaignTasks() {
	recallSchedulerOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			setting := operation_setting.GetRecallCampaignSetting()
			ticker := time.NewTicker(time.Duration(setting.TickSeconds) * time.Second)
			defer ticker.Stop()
			RunRecallMaintenanceTick(context.Background())
			for range ticker.C {
				RunRecallMaintenanceTick(context.Background())
			}
		})
	})
}

func RunRecallMaintenanceTick(ctx context.Context) {
	if !operation_setting.IsRecallCampaignEnabled() {
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.LogWarn(ctx, fmt.Sprintf("recall campaign maintenance panic: %v", recovered))
		}
	}()
	setting := operation_setting.GetRecallCampaignSetting()
	runtime := GetRecallRuntime()
	if _, err := runtime.Campaigns.RunDueCampaigns(ctx, time.Now(), setting.BatchSize); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("recall campaign maintenance failed: %v", err))
	}
	if _, err := runtime.Recipients.RunBatch(ctx, setting.BatchSize); err != nil {
		logger.LogWarn(ctx, "recall recipient maintenance failed")
	}
}
