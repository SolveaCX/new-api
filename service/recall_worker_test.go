package service

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
)

const recallWorkerTestNow = int64(1_800_000_000)

func TestRecallWorkerRuntimeUsesReplicaOwnerAndMaintenanceRunsRecipients(t *testing.T) {
	setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	campaign := createRecallWorkerCampaign(t, model.RecallCampaignRunning)
	user := model.User{Username: "recall-worker-runtime", Password: "password", Email: "runtime@example.com", StripeCustomer: "cus_runtime"}
	require.NoError(t, model.DB.Create(&user).Error)
	recipient := createRecallWorkerRecipient(t, campaign.Id, user.Id, model.RecallRecipientQueued)
	client := &recallStripeFakeClient{
		getCustomerFn: func(_ context.Context, id string) (*stripe.Customer, error) { return &stripe.Customer{ID: id}, nil },
		updateCustomerFn: func(_ context.Context, id string, params *stripe.CustomerParams) (*stripe.Customer, error) {
			return &stripe.Customer{ID: id, Email: *params.Email}, nil
		},
		createPromotionCodeFn: func(_ context.Context, params *stripe.PromotionCodeParams) (*stripe.PromotionCode, error) {
			return recallWorkerPromotionFromParams("promo_runtime", params), nil
		},
	}
	worker := newRecallWorkerForTest(client, common.GetReplicaID())
	originalRuntime := recallRuntime
	recallRuntime = &RecallRuntime{
		Campaigns:  NewRecallCampaignService(NewRecallAudienceSelector(), NewRecallStripeService(client)),
		Claims:     NewRecallClaimService(),
		Recipients: worker,
	}
	recallRuntimeOnce = sync.Once{}
	recallRuntimeOnce.Do(func() {})
	t.Cleanup(func() {
		recallRuntime = originalRuntime
		recallRuntimeOnce = sync.Once{}
		if originalRuntime != nil {
			recallRuntimeOnce.Do(func() {})
		}
	})

	RunRecallMaintenanceTick(context.Background())
	require.Equal(t, common.GetReplicaID(), recallRuntime.Recipients.owner)
	var stored model.RecallRecipient
	require.NoError(t, model.DB.First(&stored, recipient.Id).Error)
	require.Equal(t, model.RecallRecipientContacting, stored.State)
}

func TestRecallWorkerCampaignEnrollmentDefersStageOneUntilCodeReady(t *testing.T) {
	db := setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	now := time.Date(2026, 7, 16, 9, 0, 0, 0, time.UTC)
	createRecallCampaignEligibleUser(t, db, now, "worker-deferred-message")
	draft := validRecallCampaignDraft(now)
	draft.Audience.LastAPICallAgeDays = 0
	campaignService := NewRecallCampaignService(NewRecallAudienceSelector(), newRecallCampaignStripeService(t, &recallCampaignStripeCalls{}))
	campaignService.now = func() time.Time { return now }
	campaign, err := campaignService.SaveDraft(context.Background(), 7, draft)
	require.NoError(t, err)

	require.NoError(t, campaignService.Activate(context.Background(), 7, campaign.Id))
	var recipientCount, messageCount int64
	require.NoError(t, db.Model(&model.RecallRecipient{}).Count(&recipientCount).Error)
	require.NoError(t, db.Model(&model.RecallMessage{}).Count(&messageCount).Error)
	require.EqualValues(t, 1, recipientCount)
	require.Zero(t, messageCount)
}

func TestRecallWorkerReusesCustomerCreatesBoundPromotionAndSchedulesStageOne(t *testing.T) {
	setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	campaign := createRecallWorkerCampaign(t, model.RecallCampaignRunning)
	user := model.User{Username: "recall-worker-reuse", Password: "password", Email: " Current@Example.com ", StripeCustomer: "cus_existing"}
	require.NoError(t, model.DB.Create(&user).Error)
	recipient := createRecallWorkerRecipient(t, campaign.Id, user.Id, model.RecallRecipientQueued)

	createCustomerCalls := 0
	updateEmails := make([]string, 0, 1)
	var promotionParams *stripe.PromotionCodeParams
	client := &recallStripeFakeClient{
		getCustomerFn: func(_ context.Context, id string) (*stripe.Customer, error) {
			require.Equal(t, "cus_existing", id)
			return &stripe.Customer{ID: id}, nil
		},
		createCustomerFn: func(context.Context, *stripe.CustomerParams) (*stripe.Customer, error) {
			createCustomerCalls++
			return nil, nil
		},
		updateCustomerFn: func(_ context.Context, id string, params *stripe.CustomerParams) (*stripe.Customer, error) {
			updateEmails = append(updateEmails, *params.Email)
			return &stripe.Customer{ID: id, Email: *params.Email}, nil
		},
		createPromotionCodeFn: func(_ context.Context, params *stripe.PromotionCodeParams) (*stripe.PromotionCode, error) {
			promotionParams = params
			return recallWorkerPromotionFromParams("promo_reuse", params), nil
		},
	}
	stripeService := NewRecallStripeService(client)
	stripeService.codeGenerator = func(int) (string, error) { return "WORKER234", nil }
	worker := NewRecallRecipientWorker(stripeService, NewRecallClaimService(), "node-a")
	worker.now = func() time.Time { return time.Unix(recallWorkerTestNow, 0).UTC() }

	processed, err := worker.RunBatch(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, processed)
	require.Zero(t, createCustomerCalls)
	require.Equal(t, []string{"current@example.com"}, updateEmails)
	require.NotNil(t, promotionParams)
	require.Equal(t, "cus_existing", *promotionParams.Customer)
	require.Equal(t, "coupon_worker", *promotionParams.Coupon)
	require.Equal(t, int64(1), *promotionParams.MaxRedemptions)
	require.Equal(t, int64(2500), *promotionParams.Restrictions.MinimumAmount)
	require.Equal(t, "usd", *promotionParams.Restrictions.MinimumAmountCurrency)

	var stored model.RecallRecipient
	require.NoError(t, model.DB.First(&stored, recipient.Id).Error)
	require.Equal(t, model.RecallRecipientContacting, stored.State)
	require.Equal(t, "cus_existing", stored.StripeCustomerId)
	require.NotNil(t, stored.StripePromotionCodeId)
	require.Equal(t, "promo_reuse", *stored.StripePromotionCodeId)
	require.Equal(t, "FKWXRKER234", stored.PromotionCode)
	var message model.RecallMessage
	require.NoError(t, model.DB.Where("recipient_id = ? AND stage_no = 1", stored.Id).First(&message).Error)
	require.Nil(t, message.ClaimTokenHash)
	require.Equal(t, model.RecallMessageScheduled, message.State)
}

func TestRecallWorkerResolvesCompetingCustomerWinnerByReloadAndValidation(t *testing.T) {
	setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	campaign := createRecallWorkerCampaign(t, model.RecallCampaignRunning)
	user := model.User{Username: "recall-worker-winner", Password: "password", Email: "winner@example.com", StripeCustomer: "cus_deleted"}
	require.NoError(t, model.DB.Create(&user).Error)
	recipient := createRecallWorkerRecipient(t, campaign.Id, user.Id, model.RecallRecipientQueued)

	var getIDs []string
	createParams := make([]*stripe.CustomerParams, 0, 1)
	var promotionCustomer string
	injectedWinner := false
	client := &recallStripeFakeClient{
		getCustomerFn: func(_ context.Context, id string) (*stripe.Customer, error) {
			getIDs = append(getIDs, id)
			if id == "cus_deleted" {
				return &stripe.Customer{ID: id, Deleted: true}, nil
			}
			require.Equal(t, "cus_winner", id)
			return &stripe.Customer{ID: id}, nil
		},
		createCustomerFn: func(_ context.Context, params *stripe.CustomerParams) (*stripe.Customer, error) {
			createParams = append(createParams, params)
			return &stripe.Customer{ID: "cus_created"}, nil
		},
		updateCustomerFn: func(_ context.Context, id string, params *stripe.CustomerParams) (*stripe.Customer, error) {
			if id == "cus_created" && !injectedWinner {
				injectedWinner = true
				require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("stripe_customer", "cus_winner").Error)
			}
			return &stripe.Customer{ID: id, Email: *params.Email}, nil
		},
		createPromotionCodeFn: func(_ context.Context, params *stripe.PromotionCodeParams) (*stripe.PromotionCode, error) {
			promotionCustomer = *params.Customer
			return recallWorkerPromotionFromParams("promo_winner", params), nil
		},
	}
	worker := newRecallWorkerForTest(client, "node-a")

	processed, err := worker.RunBatch(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, processed)
	require.Equal(t, []string{"cus_deleted", "cus_winner"}, getIDs)
	require.Len(t, createParams, 1)
	require.Nil(t, createParams[0].Email)
	require.Nil(t, createParams[0].Name)
	require.Nil(t, createParams[0].Description)
	require.Equal(t, map[string]string{"flatkey_user_id": fmt.Sprintf("%d", user.Id)}, createParams[0].Metadata)
	require.Equal(t, "recall_customer:"+fmt.Sprintf("%d", user.Id), *createParams[0].IdempotencyKey)
	require.Equal(t, "cus_winner", promotionCustomer)

	storedUser, err := model.GetUserByIdWithContext(context.Background(), user.Id)
	require.NoError(t, err)
	require.Equal(t, "cus_winner", storedUser.StripeCustomer)
	var storedRecipient model.RecallRecipient
	require.NoError(t, model.DB.First(&storedRecipient, recipient.Id).Error)
	require.Equal(t, "cus_winner", storedRecipient.StripeCustomerId)
	require.Equal(t, model.RecallRecipientContacting, storedRecipient.State)
}

func TestRecallWorkerReconcilesExistingPromotionWithoutCreate(t *testing.T) {
	setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	campaign := createRecallWorkerCampaign(t, model.RecallCampaignRunning)
	user := model.User{Username: "recall-worker-existing-promo", Password: "password", Email: "existing@example.com", StripeCustomer: "cus_existing"}
	require.NoError(t, model.DB.Create(&user).Error)
	recipient := createRecallWorkerRecipient(t, campaign.Id, user.Id, model.RecallRecipientCustomerReady)
	promotionID := "promo_existing"
	require.NoError(t, model.DB.Model(&model.RecallRecipient{}).Where("id = ?", recipient.Id).Updates(map[string]any{
		"stripe_customer_id":       "cus_existing",
		"stripe_promotion_code_id": promotionID,
		"promotion_code":           "FKEXXST234",
	}).Error)
	createCalls := 0
	getCalls := 0
	client := &recallStripeFakeClient{
		getPromotionCodeFn: func(_ context.Context, id string) (*stripe.PromotionCode, error) {
			getCalls++
			require.Equal(t, promotionID, id)
			return &stripe.PromotionCode{
				ID: id, Active: true, Code: "FKEXXST234", Coupon: &stripe.Coupon{ID: "coupon_worker"}, Customer: &stripe.Customer{ID: "cus_existing"},
				ExpiresAt: 1_900_000_000, MaxRedemptions: 1,
				Restrictions: &stripe.PromotionCodeRestrictions{MinimumAmount: 2500, MinimumAmountCurrency: stripe.CurrencyUSD},
			}, nil
		},
		createPromotionCodeFn: func(context.Context, *stripe.PromotionCodeParams) (*stripe.PromotionCode, error) {
			createCalls++
			return nil, nil
		},
	}
	worker := newRecallWorkerForTest(client, "node-a")

	_, err := worker.RunBatch(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, getCalls)
	require.Zero(t, createCalls)
	var stored model.RecallRecipient
	require.NoError(t, model.DB.First(&stored, recipient.Id).Error)
	require.Equal(t, model.RecallRecipientContacting, stored.State)
}

func TestRecallWorkerTwoWorkersCreateOnePromotion(t *testing.T) {
	setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	campaign := createRecallWorkerCampaign(t, model.RecallCampaignRunning)
	user := model.User{Username: "recall-worker-race", Password: "password", Email: "race@example.com", StripeCustomer: "cus_race"}
	require.NoError(t, model.DB.Create(&user).Error)
	recipient := createRecallWorkerRecipient(t, campaign.Id, user.Id, model.RecallRecipientCustomerReady)
	require.NoError(t, model.DB.Model(&model.RecallRecipient{}).Where("id = ?", recipient.Id).Updates(map[string]any{
		"stripe_customer_id": "cus_race", "promotion_code": "FKRACE234",
	}).Error)
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var createCalls atomic.Int32
	client := &recallStripeFakeClient{createPromotionCodeFn: func(_ context.Context, params *stripe.PromotionCodeParams) (*stripe.PromotionCode, error) {
		createCalls.Add(1)
		started <- struct{}{}
		<-release
		return recallWorkerPromotionFromParams("promo_race", params), nil
	}}
	workerA := newRecallWorkerForTest(client, "node-a")
	workerB := newRecallWorkerForTest(client, "node-b")
	errCh := make(chan error, 1)
	go func() {
		_, err := workerA.RunBatch(context.Background(), 10)
		errCh <- err
	}()
	<-started

	processedB, err := workerB.RunBatch(context.Background(), 10)
	require.NoError(t, err)
	require.Zero(t, processedB)
	close(release)
	require.NoError(t, <-errCh)
	require.Equal(t, int32(1), createCalls.Load())
}

func TestRecallWorkerRunBatchHonorsPerCampaignConcurrency(t *testing.T) {
	setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	campaignA := createRecallWorkerCampaign(t, model.RecallCampaignRunning)
	campaignB := createRecallWorkerCampaign(t, model.RecallCampaignRunning)
	require.NoError(t, model.DB.Model(&model.RecallCampaign{}).Where("id = ?", campaignA.Id).Update("worker_concurrency", 2).Error)
	require.NoError(t, model.DB.Model(&model.RecallCampaign{}).Where("id = ?", campaignB.Id).Update("worker_concurrency", 1).Error)

	codes := []string{"FKAAAA234", "FKBBBB234", "FKCCCC234", "FKDDDD234", "FKEEEE234", "FKFFFF234"}
	for i := range codes {
		campaignID := campaignA.Id
		if i >= 4 {
			campaignID = campaignB.Id
		}
		user := model.User{
			Username:       fmt.Sprintf("recall-worker-concurrency-%d", i),
			Password:       "password",
			Email:          fmt.Sprintf("concurrency-%d@example.com", i),
			AffCode:        fmt.Sprintf("worker-concurrency-%d", i),
			StripeCustomer: fmt.Sprintf("cus_concurrency_%d", i),
		}
		require.NoError(t, model.DB.Create(&user).Error)
		recipient := createRecallWorkerRecipient(t, campaignID, user.Id, model.RecallRecipientCustomerReady)
		require.NoError(t, model.DB.Model(&model.RecallRecipient{}).Where("id = ?", recipient.Id).Updates(map[string]any{
			"stripe_customer_id": user.StripeCustomer,
			"promotion_code":     codes[i],
		}).Error)
	}

	started := make(chan int64, len(codes))
	release := make(chan struct{})
	active := map[int64]int{}
	peak := map[int64]int{}
	var concurrencyMu sync.Mutex
	client := &recallStripeFakeClient{createPromotionCodeFn: func(_ context.Context, params *stripe.PromotionCodeParams) (*stripe.PromotionCode, error) {
		var campaignID int64
		_, err := fmt.Sscan(params.Metadata["recall_campaign_id"], &campaignID)
		require.NoError(t, err)
		concurrencyMu.Lock()
		active[campaignID]++
		if active[campaignID] > peak[campaignID] {
			peak[campaignID] = active[campaignID]
		}
		concurrencyMu.Unlock()
		started <- campaignID
		<-release
		concurrencyMu.Lock()
		active[campaignID]--
		concurrencyMu.Unlock()
		return recallWorkerPromotionFromParams("promo_"+params.Metadata["recall_recipient_id"], params), nil
	}}
	worker := newRecallWorkerForTest(client, "node-a")
	type runResult struct {
		processed int
		err       error
	}
	done := make(chan runResult, 1)
	go func() {
		processed, err := worker.RunBatch(context.Background(), len(codes))
		done <- runResult{processed: processed, err: err}
	}()

	startedCount := 0
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for startedCount < 3 {
		select {
		case <-started:
			startedCount++
		case <-timer.C:
			close(release)
			result := <-done
			require.NoError(t, result.err)
			t.Fatalf("expected three concurrent Stripe calls across campaign caps, got %d", startedCount)
		}
	}
	close(release)
	result := <-done
	require.NoError(t, result.err)
	require.Equal(t, len(codes), result.processed)
	concurrencyMu.Lock()
	defer concurrencyMu.Unlock()
	require.Equal(t, 2, peak[campaignA.Id])
	require.Equal(t, 1, peak[campaignB.Id])
}

func TestRecallWorkerPersistsPromotionAfterLeaseLossWithoutAdvancing(t *testing.T) {
	setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	campaign := createRecallWorkerCampaign(t, model.RecallCampaignRunning)
	user := model.User{Username: "recall-worker-promo-lease", Password: "password", Email: "promo-lease@example.com", StripeCustomer: "cus_lease"}
	require.NoError(t, model.DB.Create(&user).Error)
	recipient := createRecallWorkerRecipient(t, campaign.Id, user.Id, model.RecallRecipientCustomerReady)
	require.NoError(t, model.DB.Model(&model.RecallRecipient{}).Where("id = ?", recipient.Id).Updates(map[string]any{
		"stripe_customer_id": "cus_lease", "promotion_code": "FKSXFE234",
	}).Error)
	client := &recallStripeFakeClient{createPromotionCodeFn: func(_ context.Context, params *stripe.PromotionCodeParams) (*stripe.PromotionCode, error) {
		require.NoError(t, model.DB.Model(&model.RecallRecipient{}).Where("id = ?", recipient.Id).Updates(map[string]any{
			"lease_owner": "node-b", "lease_expires_at": recallWorkerTestNow + 120,
		}).Error)
		promotion := recallWorkerPromotionFromParams("promo_after_lease_loss", params)
		promotion.Code = "FKNEW234"
		return promotion, nil
	}}
	worker := newRecallWorkerForTest(client, "node-a")

	_, err := worker.RunBatch(context.Background(), 10)
	require.NoError(t, err)
	var stored model.RecallRecipient
	require.NoError(t, model.DB.First(&stored, recipient.Id).Error)
	require.Equal(t, model.RecallRecipientCustomerReady, stored.State)
	require.Equal(t, "node-b", stored.LeaseOwner)
	require.NotNil(t, stored.StripePromotionCodeId)
	require.Equal(t, "promo_after_lease_loss", *stored.StripePromotionCodeId)
	require.Equal(t, "FKNEW234", stored.PromotionCode)
	var messageCount int64
	require.NoError(t, model.DB.Model(&model.RecallMessage{}).Count(&messageCount).Error)
	require.Zero(t, messageCount)
}

func TestRecallWorkerPersistsCustomerAfterLeaseLossWithoutAdvancing(t *testing.T) {
	setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	campaign := createRecallWorkerCampaign(t, model.RecallCampaignRunning)
	user := model.User{Username: "recall-worker-customer-lease", Password: "password", Email: "customer-lease@example.com"}
	require.NoError(t, model.DB.Create(&user).Error)
	recipient := createRecallWorkerRecipient(t, campaign.Id, user.Id, model.RecallRecipientQueued)
	client := &recallStripeFakeClient{
		createCustomerFn: func(context.Context, *stripe.CustomerParams) (*stripe.Customer, error) {
			require.NoError(t, model.DB.Model(&model.RecallRecipient{}).Where("id = ?", recipient.Id).Updates(map[string]any{
				"lease_owner": "node-b", "lease_expires_at": recallWorkerTestNow + 120,
			}).Error)
			return &stripe.Customer{ID: "cus_after_lease_loss"}, nil
		},
		updateCustomerFn: func(_ context.Context, id string, params *stripe.CustomerParams) (*stripe.Customer, error) {
			return &stripe.Customer{ID: id, Email: *params.Email}, nil
		},
	}
	worker := newRecallWorkerForTest(client, "node-a")

	_, err := worker.RunBatch(context.Background(), 10)
	require.NoError(t, err)
	var stored model.RecallRecipient
	require.NoError(t, model.DB.First(&stored, recipient.Id).Error)
	require.Equal(t, model.RecallRecipientQueued, stored.State)
	require.Equal(t, "node-b", stored.LeaseOwner)
	require.Equal(t, "cus_after_lease_loss", stored.StripeCustomerId)
	storedUser, err := model.GetUserByIdWithContext(context.Background(), user.Id)
	require.NoError(t, err)
	require.Equal(t, "cus_after_lease_loss", storedUser.StripeCustomer)
}

func TestRecallWorkerTransientStripeErrorDefersOnlyRecipient(t *testing.T) {
	setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	campaign := createRecallWorkerCampaign(t, model.RecallCampaignRunning)
	user := model.User{Username: "recall-worker-transient", Password: "password", Email: "transient@example.com", StripeCustomer: "cus_transient"}
	require.NoError(t, model.DB.Create(&user).Error)
	recipient := createRecallWorkerRecipient(t, campaign.Id, user.Id, model.RecallRecipientQueued)
	client := &recallStripeFakeClient{getCustomerFn: func(context.Context, string) (*stripe.Customer, error) {
		return nil, recallStripeTimeout{}
	}}
	worker := newRecallWorkerForTest(client, "node-a")

	_, err := worker.RunBatch(context.Background(), 10)
	require.NoError(t, err)
	var stored model.RecallRecipient
	require.NoError(t, model.DB.First(&stored, recipient.Id).Error)
	require.Equal(t, model.RecallRecipientQueued, stored.State)
	require.Empty(t, stored.LeaseOwner)
	require.Equal(t, recallWorkerTestNow+recallRecipientRetrySeconds, stored.LeaseExpiresAt)
	require.Equal(t, "stripe_retryable", stored.LastErrorCode)
}

func TestRecallWorkerPermanentCustomerErrorFailsOnlyOneRecipient(t *testing.T) {
	setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	campaign := createRecallWorkerCampaign(t, model.RecallCampaignRunning)
	badUser := model.User{Username: "recall-worker-bad", Password: "password", Email: "bad@example.com", StripeCustomer: "cus_bad", AffCode: "worker-bad"}
	goodUser := model.User{Username: "recall-worker-good", Password: "password", Email: "good@example.com", StripeCustomer: "cus_good", AffCode: "worker-good"}
	require.NoError(t, model.DB.Create(&badUser).Error)
	require.NoError(t, model.DB.Create(&goodUser).Error)
	badRecipient := createRecallWorkerRecipient(t, campaign.Id, badUser.Id, model.RecallRecipientQueued)
	goodRecipient := createRecallWorkerRecipient(t, campaign.Id, goodUser.Id, model.RecallRecipientQueued)
	client := &recallStripeFakeClient{
		getCustomerFn: func(_ context.Context, id string) (*stripe.Customer, error) {
			if id == "cus_bad" {
				return nil, &stripe.Error{Type: stripe.ErrorTypeInvalidRequest, Param: "customer", Msg: "invalid customer"}
			}
			return &stripe.Customer{ID: id}, nil
		},
		updateCustomerFn: func(_ context.Context, id string, params *stripe.CustomerParams) (*stripe.Customer, error) {
			return &stripe.Customer{ID: id, Email: *params.Email}, nil
		},
		createPromotionCodeFn: func(_ context.Context, params *stripe.PromotionCodeParams) (*stripe.PromotionCode, error) {
			return recallWorkerPromotionFromParams("promo_good", params), nil
		},
	}
	worker := newRecallWorkerForTest(client, "node-a")

	processed, err := worker.RunBatch(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 2, processed)
	var badStored, goodStored model.RecallRecipient
	require.NoError(t, model.DB.First(&badStored, badRecipient.Id).Error)
	require.NoError(t, model.DB.First(&goodStored, goodRecipient.Id).Error)
	require.Equal(t, model.RecallRecipientFailed, badStored.State)
	require.Equal(t, model.RecallRecipientContacting, goodStored.State)
}

func TestRecallWorkerPermanentPromotionErrorFailsOnlyOneRecipient(t *testing.T) {
	setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	campaign := createRecallWorkerCampaign(t, model.RecallCampaignRunning)
	badUser := model.User{Username: "recall-worker-promo-bad", Password: "password", Email: "promo-bad@example.com", StripeCustomer: "cus_promo_bad", AffCode: "worker-promo-bad"}
	goodUser := model.User{Username: "recall-worker-promo-good", Password: "password", Email: "promo-good@example.com", StripeCustomer: "cus_promo_good", AffCode: "worker-promo-good"}
	require.NoError(t, model.DB.Create(&badUser).Error)
	require.NoError(t, model.DB.Create(&goodUser).Error)
	badRecipient := createRecallWorkerRecipient(t, campaign.Id, badUser.Id, model.RecallRecipientCustomerReady)
	goodRecipient := createRecallWorkerRecipient(t, campaign.Id, goodUser.Id, model.RecallRecipientCustomerReady)
	require.NoError(t, model.DB.Model(&model.RecallRecipient{}).Where("id = ?", badRecipient.Id).Updates(map[string]any{
		"stripe_customer_id": badUser.StripeCustomer,
		"promotion_code":     "FKBADP234",
	}).Error)
	require.NoError(t, model.DB.Model(&model.RecallRecipient{}).Where("id = ?", goodRecipient.Id).Updates(map[string]any{
		"stripe_customer_id": goodUser.StripeCustomer,
		"promotion_code":     "FKGXXD234",
	}).Error)
	client := &recallStripeFakeClient{createPromotionCodeFn: func(_ context.Context, params *stripe.PromotionCodeParams) (*stripe.PromotionCode, error) {
		if *params.Customer == badUser.StripeCustomer {
			return nil, &stripe.Error{Type: stripe.ErrorTypeInvalidRequest, Param: "customer", Msg: "invalid customer"}
		}
		return recallWorkerPromotionFromParams("promo_good_sibling", params), nil
	}}
	worker := newRecallWorkerForTest(client, "node-a")

	processed, err := worker.RunBatch(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 2, processed)
	var badStored, goodStored model.RecallRecipient
	require.NoError(t, model.DB.First(&badStored, badRecipient.Id).Error)
	require.NoError(t, model.DB.First(&goodStored, goodRecipient.Id).Error)
	require.Equal(t, model.RecallRecipientFailed, badStored.State)
	require.Equal(t, model.RecallRecipientContacting, goodStored.State)
}

func TestRecallWorkerPromotionCollisionStopsAfterFiveAndFailsRecipient(t *testing.T) {
	setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	campaign := createRecallWorkerCampaign(t, model.RecallCampaignRunning)
	user := model.User{Username: "recall-worker-collision", Password: "password", Email: "collision@example.com", StripeCustomer: "cus_collision"}
	require.NoError(t, model.DB.Create(&user).Error)
	recipient := createRecallWorkerRecipient(t, campaign.Id, user.Id, model.RecallRecipientCustomerReady)
	require.NoError(t, model.DB.Model(&model.RecallRecipient{}).Where("id = ?", recipient.Id).Updates(map[string]any{
		"stripe_customer_id": "cus_collision", "promotion_code": "FKBASE234",
	}).Error)
	var calls int
	client := &recallStripeFakeClient{createPromotionCodeFn: func(context.Context, *stripe.PromotionCodeParams) (*stripe.PromotionCode, error) {
		calls++
		return nil, &stripe.Error{Type: stripe.ErrorTypeInvalidRequest, Param: "code", Msg: "code is already active"}
	}}
	worker := newRecallWorkerForTest(client, "node-a")

	_, err := worker.RunBatch(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 5, calls)
	var stored model.RecallRecipient
	require.NoError(t, model.DB.First(&stored, recipient.Id).Error)
	require.Equal(t, model.RecallRecipientFailed, stored.State)
}

func TestRecallWorkerPauseBetweenStripeCallsPreventsNextExternalCall(t *testing.T) {
	setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	campaign := createRecallWorkerCampaign(t, model.RecallCampaignRunning)
	user := model.User{Username: "recall-worker-pause", Password: "password", Email: "pause@example.com", StripeCustomer: "cus_deleted"}
	require.NoError(t, model.DB.Create(&user).Error)
	recipient := createRecallWorkerRecipient(t, campaign.Id, user.Id, model.RecallRecipientQueued)
	createCalls := 0
	client := &recallStripeFakeClient{
		getCustomerFn: func(context.Context, string) (*stripe.Customer, error) {
			require.NoError(t, model.DB.Model(&model.RecallCampaign{}).Where("id = ?", campaign.Id).Update("status", model.RecallCampaignPaused).Error)
			return &stripe.Customer{ID: "cus_deleted", Deleted: true}, nil
		},
		createCustomerFn: func(context.Context, *stripe.CustomerParams) (*stripe.Customer, error) {
			createCalls++
			return nil, nil
		},
	}
	worker := newRecallWorkerForTest(client, "node-a")

	_, err := worker.RunBatch(context.Background(), 10)
	require.NoError(t, err)
	require.Zero(t, createCalls)
	var stored model.RecallRecipient
	require.NoError(t, model.DB.First(&stored, recipient.Id).Error)
	require.Equal(t, model.RecallRecipientQueued, stored.State)
	require.Empty(t, stored.LeaseOwner)
}

func TestRecallWorkerCancelAfterLeasePreventsExternalCall(t *testing.T) {
	setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	campaign := createRecallWorkerCampaign(t, model.RecallCampaignRunning)
	user := model.User{Username: "recall-worker-cancel", Password: "password", Email: "cancel@example.com", StripeCustomer: "cus_cancel"}
	require.NoError(t, model.DB.Create(&user).Error)
	recipient := createRecallWorkerRecipient(t, campaign.Id, user.Id, model.RecallRecipientQueued)
	won, err := model.LeaseRecallRecipient(recipient.Id, "node-a", recallWorkerTestNow, recallWorkerTestNow+60)
	require.NoError(t, err)
	require.True(t, won)
	require.NoError(t, model.DB.Model(&model.RecallCampaign{}).Where("id = ?", campaign.Id).Update("status", model.RecallCampaignCancelled).Error)

	externalCalls := 0
	client := &recallStripeFakeClient{getCustomerFn: func(context.Context, string) (*stripe.Customer, error) {
		externalCalls++
		return nil, &stripe.Error{Type: stripe.ErrorTypeInvalidRequest, Msg: "must not be called"}
	}}
	worker := newRecallWorkerForTest(client, "node-a")

	require.NoError(t, worker.ProcessLeased(context.Background(), recipient.Id))
	require.Zero(t, externalCalls)
	var stored model.RecallRecipient
	require.NoError(t, model.DB.First(&stored, recipient.Id).Error)
	require.Equal(t, model.RecallRecipientQueued, stored.State)
	require.Empty(t, stored.LeaseOwner)
}

func TestRecallWorkerFeatureDisableAfterLeasePreventsExternalCall(t *testing.T) {
	setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	campaign := createRecallWorkerCampaign(t, model.RecallCampaignRunning)
	user := model.User{Username: "recall-worker-disabled", Password: "password", Email: "disabled@example.com", StripeCustomer: "cus_disabled"}
	require.NoError(t, model.DB.Create(&user).Error)
	recipient := createRecallWorkerRecipient(t, campaign.Id, user.Id, model.RecallRecipientQueued)
	won, err := model.LeaseRecallRecipient(recipient.Id, "node-a", recallWorkerTestNow, recallWorkerTestNow+60)
	require.NoError(t, err)
	require.True(t, won)
	loadRecallCampaignEnabled(t, false)
	externalCalls := 0
	client := &recallStripeFakeClient{getCustomerFn: func(context.Context, string) (*stripe.Customer, error) {
		externalCalls++
		return nil, nil
	}}
	worker := newRecallWorkerForTest(client, "node-a")

	require.NoError(t, worker.ProcessLeased(context.Background(), recipient.Id))
	require.Zero(t, externalCalls)
}

func TestRecallWorkerCompletedCampaignFinishesEnrolledRecipient(t *testing.T) {
	setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	campaign := createRecallWorkerCampaign(t, model.RecallCampaignCompleted)
	user := model.User{Username: "recall-worker-completed", Password: "password", Email: "completed@example.com", StripeCustomer: "cus_completed"}
	require.NoError(t, model.DB.Create(&user).Error)
	recipient := createRecallWorkerRecipient(t, campaign.Id, user.Id, model.RecallRecipientQueued)
	client := &recallStripeFakeClient{
		getCustomerFn: func(_ context.Context, id string) (*stripe.Customer, error) { return &stripe.Customer{ID: id}, nil },
		updateCustomerFn: func(_ context.Context, id string, params *stripe.CustomerParams) (*stripe.Customer, error) {
			return &stripe.Customer{ID: id, Email: *params.Email}, nil
		},
		createPromotionCodeFn: func(_ context.Context, params *stripe.PromotionCodeParams) (*stripe.PromotionCode, error) {
			return recallWorkerPromotionFromParams("promo_completed", params), nil
		},
	}
	worker := newRecallWorkerForTest(client, "node-a")

	_, err := worker.RunBatch(context.Background(), 10)
	require.NoError(t, err)
	var stored model.RecallRecipient
	require.NoError(t, model.DB.First(&stored, recipient.Id).Error)
	require.Equal(t, model.RecallRecipientContacting, stored.State)
}

func TestRecallWorkerLogsOnlyIDsAndErrorClass(t *testing.T) {
	setupRecallCampaignTestDB(t)
	setRecallCampaignEnabled(t, true)
	campaign := createRecallWorkerCampaign(t, model.RecallCampaignRunning)
	user := model.User{Username: "recall-worker-log", Password: "password", Email: "secret-email@example.com", StripeCustomer: "cus_log"}
	require.NoError(t, model.DB.Create(&user).Error)
	recipient := createRecallWorkerRecipient(t, campaign.Id, user.Id, model.RecallRecipientQueued)
	secretError := workerSecretTimeout("FKSECRETCODE claim-secret email-body secret-email@example.com")
	client := &recallStripeFakeClient{getCustomerFn: func(context.Context, string) (*stripe.Customer, error) { return nil, secretError }}
	worker := newRecallWorkerForTest(client, "node-a")
	var logBuffer bytes.Buffer
	common.LogWriterMu.Lock()
	originalWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = originalWriter
		common.LogWriterMu.Unlock()
	})

	_, err := worker.RunBatch(context.Background(), 10)
	require.NoError(t, err)
	logged := logBuffer.String()
	require.Contains(t, logged, fmt.Sprintf("recipient_id=%d", recipient.Id))
	require.Contains(t, logged, "error_class=retryable")
	for _, secret := range []string{"FKSECRETCODE", "claim-secret", "email-body", "secret-email@example.com"} {
		require.NotContains(t, logged, secret)
	}
}

func newRecallWorkerForTest(client RecallStripeClient, owner string) *RecallRecipientWorker {
	stripeService := NewRecallStripeService(client)
	stripeService.codeGenerator = func(int) (string, error) { return "WORKER234", nil }
	worker := NewRecallRecipientWorker(stripeService, NewRecallClaimService(), owner)
	worker.now = func() time.Time { return time.Unix(recallWorkerTestNow, 0).UTC() }
	return worker
}

type workerSecretTimeout string

func (e workerSecretTimeout) Error() string { return string(e) }
func (workerSecretTimeout) Timeout() bool   { return true }
func (workerSecretTimeout) Temporary() bool { return true }

func createRecallWorkerCampaign(t *testing.T, status string) model.RecallCampaign {
	t.Helper()
	discountJSON, err := common.Marshal(RecallDiscountConfig{MinimumAmount: 2500, MinimumAmountCurrency: "usd"})
	require.NoError(t, err)
	emailJSON, err := common.Marshal([]RecallEmailStage{{
		StageNo: 1, DelaySeconds: 60, TemplateVersion: 1,
		Templates: map[string]RecallEmailTemplate{"en": {Subject: "Return", BodyText: "Offer body"}},
	}})
	require.NoError(t, err)
	campaign := model.RecallCampaign{
		Name: "worker campaign", Status: status, AudienceTemplate: "inactive_users", AudienceConfig: `{}`,
		ExecutionMode: "manual", CouponSource: "existing", StripeCouponId: "coupon_worker",
		DiscountConfig: string(discountJSON), ProductScope: `{}`, PromotionValidSeconds: 3600,
		EmailSequenceConfig: string(emailJSON), EnrollmentLimit: 100, WorkerConcurrency: 2,
	}
	require.NoError(t, model.DB.Create(&campaign).Error)
	return campaign
}

func createRecallWorkerRecipient(t *testing.T, campaignID int64, userID int, state string) model.RecallRecipient {
	t.Helper()
	recipient := model.RecallRecipient{
		CampaignId: campaignID, UserId: userID, EligibilitySnapshot: `{}`, EmailSnapshot: "snapshot@example.com",
		LanguageSnapshot: "en", State: state, PromotionExpiresAt: 1_900_000_000,
	}
	require.NoError(t, model.DB.Create(&recipient).Error)
	return recipient
}

func recallWorkerPromotionFromParams(id string, params *stripe.PromotionCodeParams) *stripe.PromotionCode {
	promotion := &stripe.PromotionCode{
		ID: id, Active: true, Code: *params.Code, Coupon: &stripe.Coupon{ID: *params.Coupon}, Customer: &stripe.Customer{ID: *params.Customer},
		ExpiresAt: *params.ExpiresAt, MaxRedemptions: *params.MaxRedemptions,
	}
	if params.Restrictions != nil {
		promotion.Restrictions = &stripe.PromotionCodeRestrictions{
			MinimumAmount:         *params.Restrictions.MinimumAmount,
			MinimumAmountCurrency: stripe.Currency(*params.Restrictions.MinimumAmountCurrency),
		}
	}
	return promotion
}
