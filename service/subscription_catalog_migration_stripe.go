package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stripe/stripe-go/v86"
	stripeprice "github.com/stripe/stripe-go/v86/price"
	stripesubscription "github.com/stripe/stripe-go/v86/subscription"
	stripeschedule "github.com/stripe/stripe-go/v86/subscriptionschedule"
	"gorm.io/gorm"
)

const (
	catalogMigrationMetadataBatch       = "newapi_catalog_migration_batch"
	catalogMigrationMetadataContract    = "newapi_catalog_migration_contract"
	catalogMigrationMetadataIntent      = "newapi_catalog_migration_intent"
	catalogMigrationMetadataIdempotency = "newapi_catalog_migration_idempotency"
	catalogMigrationMetadataOwnership   = "newapi_catalog_migration_ownership"
)

type catalogMigrationStripeProvider interface {
	GetSubscription(context.Context, string) (*stripe.Subscription, error)
	GetPrice(context.Context, string) (*stripe.Price, error)
	GetSchedule(context.Context, string) (*stripe.SubscriptionSchedule, error)
	CreateSchedule(context.Context, string, map[string]string, string) (*stripe.SubscriptionSchedule, error)
	UpdateSchedule(context.Context, string, *stripe.SubscriptionScheduleParams, string) (*stripe.SubscriptionSchedule, error)
	ReleaseSchedule(context.Context, string, string) (*stripe.SubscriptionSchedule, error)
}

type stripeCatalogMigrationScheduler struct {
	provider catalogMigrationStripeProvider
	sandbox  func() CatalogMigrationSandboxConfig
}

type stripeCatalogMigrationAPI struct{}

type catalogMigrationUserActionPreemption struct {
	binding     model.SubscriptionProviderBinding
	contract    model.UserSubscriptionContract
	intent      model.SubscriptionChangeIntent
	reservation *model.SubscriptionProviderLifecycleReservation
	request     CatalogMigrationProviderScheduleRequest
}

func init() {
	catalogMigrationRuntimeProviderScheduler = newStripeCatalogMigrationScheduler()
}

func newStripeCatalogMigrationScheduler() CatalogMigrationProviderScheduler {
	return &stripeCatalogMigrationScheduler{provider: stripeCatalogMigrationAPI{}, sandbox: catalogMigrationRuntimeSandboxConfig}
}

// supersedeCatalogMigrationForUserAction gives an explicit cancel or plan
// change precedence over a queued catalog migration. It deliberately recognizes
// only the exact latest catalog intent and preserves all ordinary intent rules.
func supersedeCatalogMigrationForUserAction(ctx context.Context, userID int, targetPlanID int) (int64, bool, error) {
	if userID <= 0 || model.DB == nil {
		return 0, false, nil
	}
	var candidates int64
	if err := model.DB.WithContext(ctx).Model(&model.SubscriptionChangeIntent{}).
		Where("user_id = ? AND kind = ? AND status IN ?", userID, model.SubscriptionChangeIntentKindCatalogMigration, cancellableCatalogMigrationIntentStatuses()).
		Count(&candidates).Error; err != nil {
		return 0, false, err
	}
	if candidates == 0 {
		return 0, false, nil
	}
	var snapshot model.UserSubscriptionContract
	if err := model.DB.WithContext(ctx).Where("user_id = ?", userID).Order("id desc").First(&snapshot).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if snapshot.CurrentProviderBindingId <= 0 || snapshot.LatestChangeIntentId <= 0 {
		return snapshot.ChangeVersion, false, nil
	}
	if targetPlanID > 0 {
		var target model.SubscriptionPlan
		if targetPlanID == snapshot.CurrentPlanId || model.DB.WithContext(ctx).Where("id = ? AND enabled = ?", targetPlanID, true).First(&target).Error != nil {
			return snapshot.ChangeVersion, false, nil
		}
	}

	var prepared catalogMigrationUserActionPreemption
	err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := subscriptionCommandLock(tx).Where("id = ? AND user_id = ?", snapshot.CurrentProviderBindingId, userID).First(&prepared.binding).Error; err != nil {
			return err
		}
		if err := subscriptionCommandLock(tx).Where("id = ? AND user_id = ? AND current_provider_binding_id = ?", prepared.binding.ContractId, userID, prepared.binding.Id).First(&prepared.contract).Error; err != nil {
			return err
		}
		if prepared.contract.LatestChangeIntentId <= 0 {
			return nil
		}
		if err := subscriptionCommandLock(tx).Where("id = ? AND contract_id = ? AND user_id = ?", prepared.contract.LatestChangeIntentId, prepared.contract.Id, userID).First(&prepared.intent).Error; err != nil {
			return err
		}
		if prepared.intent.Kind != model.SubscriptionChangeIntentKindCatalogMigration || !containsString(cancellableCatalogMigrationIntentStatuses(), prepared.intent.Status) {
			prepared.intent = model.SubscriptionChangeIntent{}
			return nil
		}
		if prepared.intent.ProviderBindingId != prepared.binding.Id || prepared.intent.CatalogMigrationBatchId == nil || strings.TrimSpace(*prepared.intent.CatalogMigrationBatchId) == "" ||
			strings.TrimSpace(prepared.intent.ProviderScheduleFingerprint) == "" || !catalogMigrationPendingStateCanClear(prepared.contract, &prepared.intent) ||
			strings.TrimSpace(prepared.binding.ProviderScheduleId) != strings.TrimSpace(prepared.intent.ProviderScheduleId) {
			return errors.New("catalog migration ownership is unclear")
		}
		if strings.TrimSpace(prepared.binding.CurrentPlanSnapshot) == "" || strings.TrimSpace(prepared.intent.TargetPlanSnapshot) == "" {
			return errors.New("catalog migration ownership snapshots are incomplete")
		}
		if strings.TrimSpace(prepared.binding.LifecycleReservationToken) != "" || prepared.binding.LifecycleReservationUntil > 0 {
			return model.ErrSubscriptionProviderLifecycleConflict
		}
		token := stableCatalogMigrationKey("user-action", prepared.contract.Id, prepared.intent.Id, prepared.contract.ChangeVersion)
		reservation, reserved, reserveErr := model.ReserveSubscriptionProviderLifecycleExactTx(tx, &prepared.binding, model.SubscriptionProviderLifecycleActionCatalogMigration, token, catalogMigrationLifecycleReservationTTLSeconds)
		if reserveErr != nil {
			return reserveErr
		}
		prepared.binding = *reserved
		prepared.reservation = reservation
		prepared.request = CatalogMigrationProviderScheduleRequest{
			BatchID: *prepared.intent.CatalogMigrationBatchId, IntentID: prepared.intent.Id, ContractID: prepared.contract.Id,
			UserID: prepared.contract.UserId, ChangeVersion: prepared.contract.ChangeVersion, BindingID: prepared.binding.Id,
			ProviderSubscriptionID: prepared.binding.ProviderSubscriptionId, ProviderSubscriptionItemID: prepared.binding.ProviderSubscriptionItemId,
			ProviderCustomerID: prepared.binding.ProviderCustomerId, CurrentPriceID: prepared.binding.ProviderPriceId,
			CurrentPeriodStart: prepared.binding.CurrentPeriodStart, CurrentPeriodEnd: prepared.binding.CurrentPeriodEnd,
			IdempotencyKey: prepared.intent.ProviderIdempotencyKey, ExpectedOwnershipFingerprint: prepared.intent.ProviderScheduleFingerprint,
			CurrentPlanSnapshot: prepared.binding.CurrentPlanSnapshot, TargetPlanSnapshot: prepared.intent.TargetPlanSnapshot,
			PreviousScheduleSnapshot: prepared.intent.PreviousScheduleSnapshot,
			LifecycleActionSeq:       prepared.reservation.LifecycleActionSeq, LifecycleReservationToken: prepared.reservation.Token,
			LifecycleReservationUntil: prepared.reservation.ExpiresAt,
		}
		target, decodeErr := DecodeRecurringPlanSnapshotV1(prepared.intent.TargetPlanSnapshot)
		if decodeErr != nil {
			return decodeErr
		}
		prepared.request.TargetPriceID = target.StripePriceID
		return nil
	})
	if err != nil {
		if prepared.intent.Id > 0 {
			_ = markCatalogMigrationUserActionAttention(ctx, &prepared, err)
		}
		return snapshot.ChangeVersion, false, err
	}
	if prepared.intent.Id == 0 {
		return prepared.contract.ChangeVersion, false, nil
	}
	if catalogMigrationRuntimeProviderScheduler == nil {
		err = errors.New("catalog migration provider restore is unavailable")
	} else {
		err = catalogMigrationRuntimeProviderScheduler.RestoreCatalogMigration(ctx, prepared.request)
	}
	if err != nil {
		_ = markCatalogMigrationUserActionAttention(ctx, &prepared, err)
		return prepared.contract.ChangeVersion, false, err
	}

	err = model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var binding model.SubscriptionProviderBinding
		if err := subscriptionCommandLock(tx).Where("id = ?", prepared.binding.Id).First(&binding).Error; err != nil {
			return err
		}
		var contract model.UserSubscriptionContract
		if err := subscriptionCommandLock(tx).Where("id = ?", prepared.contract.Id).First(&contract).Error; err != nil {
			return err
		}
		var intent model.SubscriptionChangeIntent
		if err := subscriptionCommandLock(tx).Where("id = ?", prepared.intent.Id).First(&intent).Error; err != nil {
			return err
		}
		if !catalogMigrationReservationMatches(&binding, prepared.reservation) || contract.LatestChangeIntentId != intent.Id ||
			intent.ProviderScheduleFingerprint != prepared.intent.ProviderScheduleFingerprint || binding.ProviderScheduleId != prepared.binding.ProviderScheduleId ||
			!catalogMigrationPendingStateCanClear(contract, &intent) {
			return model.ErrSubscriptionProviderLifecycleConflict
		}
		intentUpdate := tx.Model(&model.SubscriptionChangeIntent{}).Where("id = ? AND status IN ?", intent.Id, cancellableCatalogMigrationIntentStatuses()).Updates(map[string]any{
			"status": model.SubscriptionChangeIntentStatusSuperseded, "provider_schedule_id": "", "last_error": "", "updated_at": time.Now().Unix(),
		})
		if intentUpdate.Error != nil || intentUpdate.RowsAffected != 1 {
			return firstError(intentUpdate.Error, ErrSubscriptionChangeInProgress)
		}
		if err := clearCatalogMigrationPendingTx(tx, &contract, &intent); err != nil {
			return err
		}
		bindingUpdate := tx.Model(&model.SubscriptionProviderBinding{}).Where("id = ? AND lifecycle_action_seq = ? AND lifecycle_reservation_token = ? AND lifecycle_reservation_action = ? AND lifecycle_reservation_until = ?", binding.Id, prepared.reservation.LifecycleActionSeq, prepared.reservation.Token, prepared.reservation.Action, prepared.reservation.ExpiresAt).Updates(map[string]any{
			"provider_schedule_id": "", "lifecycle_action_seq": gorm.Expr("lifecycle_action_seq + 1"), "lifecycle_reservation_token": "", "lifecycle_reservation_action": "", "lifecycle_reservation_until": 0, "updated_at": time.Now().Unix(),
		})
		if bindingUpdate.Error != nil || bindingUpdate.RowsAffected != 1 {
			return firstError(bindingUpdate.Error, model.ErrSubscriptionProviderLifecycleConflict)
		}
		prepared.contract.ChangeVersion = contract.ChangeVersion + 1
		return nil
	})
	if err != nil {
		_ = markCatalogMigrationUserActionAttention(ctx, &prepared, err)
		return prepared.contract.ChangeVersion, false, err
	}
	return prepared.contract.ChangeVersion, true, nil
}

func markCatalogMigrationUserActionAttention(ctx context.Context, prepared *catalogMigrationUserActionPreemption, cause error) error {
	if prepared == nil || prepared.intent.Id <= 0 {
		return nil
	}
	return model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var binding model.SubscriptionProviderBinding
		if err := subscriptionCommandLock(tx).Where("id = ?", prepared.binding.Id).First(&binding).Error; err != nil {
			return err
		}
		var contract model.UserSubscriptionContract
		if err := subscriptionCommandLock(tx).Where("id = ?", prepared.contract.Id).First(&contract).Error; err != nil {
			return err
		}
		var intent model.SubscriptionChangeIntent
		if err := subscriptionCommandLock(tx).Where("id = ?", prepared.intent.Id).First(&intent).Error; err != nil {
			return err
		}
		message := "catalog migration user-action precedence requires attention"
		if cause != nil {
			message = cause.Error()
		}
		if err := tx.Model(&intent).Updates(map[string]any{"status": model.SubscriptionChangeIntentStatusNeedsAttention, "last_error": message, "updated_at": time.Now().Unix()}).Error; err != nil {
			return err
		}
		return tx.Model(&contract).Updates(map[string]any{"status": model.SubscriptionContractStatusNeedsAttention, "updated_at": time.Now().Unix()}).Error
	})
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (s *stripeCatalogMigrationScheduler) ScheduleCatalogMigration(ctx context.Context, request CatalogMigrationProviderScheduleRequest) (CatalogMigrationProviderScheduleResult, error) {
	if err := validateCatalogMigrationScheduleRequest(request); err != nil {
		return CatalogMigrationProviderScheduleResult{}, err
	}
	sub, _, target, err := s.freshAndValidate(ctx, request)
	if err != nil {
		return CatalogMigrationProviderScheduleResult{}, err
	}
	metadata := catalogMigrationScheduleMetadata(request)
	scheduleID := stripeSubscriptionScheduleID(sub)
	if scheduleID != "" {
		schedule, getErr := s.provider.GetSchedule(ctx, scheduleID)
		if getErr != nil {
			return CatalogMigrationProviderScheduleResult{}, errors.New("Stripe subscription has an unrelated schedule")
		}
		if catalogMigrationScheduleExactlyOwned(schedule, request, metadata) {
			return CatalogMigrationProviderScheduleResult{ScheduleID: scheduleID, OwnershipFingerprint: request.ExpectedOwnershipFingerprint}, nil
		}
		if !catalogMigrationScheduleMetadataMatches(schedule, request, metadata) {
			// A schedule created with from_subscription cannot carry metadata in
			// the same request.  If a process lost the response after that create
			// and before the configure update, replay the exact create idempotency
			// key to prove that this is the schedule we just created.  Never adopt
			// an unmarked schedule based only on its phase shape.
			if !catalogMigrationScheduleIsUnconfiguredForRequest(schedule, request) || len(schedule.Metadata) != 0 {
				return CatalogMigrationProviderScheduleResult{}, errors.New("Stripe subscription has an unrelated schedule")
			}
			replayed, replayErr := s.provider.CreateSchedule(ctx, request.ProviderSubscriptionID, metadata, request.IdempotencyKey+":create")
			if replayErr != nil || !catalogMigrationScheduleCreateOutcomeMatches(replayed, request, metadata) || strings.TrimSpace(replayed.ID) != scheduleID {
				return CatalogMigrationProviderScheduleResult{}, errors.New("Stripe schedule create outcome is not safely reclaimable")
			}
			schedule = replayed
		}
		if !catalogMigrationScheduleIsUnconfiguredForRequest(schedule, request) {
			return CatalogMigrationProviderScheduleResult{}, errors.New("Stripe subscription has an unrelated schedule")
		}
		params := catalogMigrationScheduleParams(request, metadata, target)
		updated, updateErr := s.provider.UpdateSchedule(ctx, scheduleID, params, request.IdempotencyKey+":configure")
		if updateErr != nil {
			return CatalogMigrationProviderScheduleResult{}, updateErr
		}
		if !catalogMigrationScheduleExactlyOwned(updated, request, metadata) {
			confirmed, getErr := s.provider.GetSchedule(ctx, scheduleID)
			if getErr != nil || !catalogMigrationScheduleExactlyOwned(confirmed, request, metadata) {
				return CatalogMigrationProviderScheduleResult{}, errors.New("Stripe catalog migration schedule could not be confirmed")
			}
		}
		return CatalogMigrationProviderScheduleResult{ScheduleID: scheduleID, OwnershipFingerprint: request.ExpectedOwnershipFingerprint}, nil
	}

	createKey := request.IdempotencyKey + ":create"
	created, createErr := s.provider.CreateSchedule(ctx, request.ProviderSubscriptionID, metadata, createKey)
	if createErr != nil {
		// A network/response failure can happen after Stripe commits the create.
		// Replaying the same idempotency key is the provider-supported way to
		// recover that response.  A fresh key could create a duplicate schedule.
		retried, retryErr := s.provider.CreateSchedule(ctx, request.ProviderSubscriptionID, metadata, createKey)
		if retryErr != nil {
			return CatalogMigrationProviderScheduleResult{}, fmt.Errorf("Stripe schedule create failed: %v; idempotent retry failed: %w", createErr, retryErr)
		}
		created = retried
	} else if created == nil || strings.TrimSpace(created.ID) == "" {
		return CatalogMigrationProviderScheduleResult{}, errors.New("Stripe schedule create could not be confirmed")
	}
	if !catalogMigrationScheduleCreateOutcomeMatches(created, request, metadata) {
		return CatalogMigrationProviderScheduleResult{}, errors.New("Stripe schedule create outcome is not safely reclaimable")
	}
	scheduleID = strings.TrimSpace(created.ID)

	params := catalogMigrationScheduleParams(request, metadata, target)
	updated, err := s.provider.UpdateSchedule(ctx, scheduleID, params, request.IdempotencyKey+":configure")
	if err != nil {
		return CatalogMigrationProviderScheduleResult{}, err
	}
	if !catalogMigrationScheduleExactlyOwned(updated, request, metadata) {
		confirmed, getErr := s.provider.GetSchedule(ctx, scheduleID)
		if getErr != nil || !catalogMigrationScheduleExactlyOwned(confirmed, request, metadata) {
			return CatalogMigrationProviderScheduleResult{}, errors.New("Stripe catalog migration schedule could not be confirmed")
		}
	}
	return CatalogMigrationProviderScheduleResult{ScheduleID: scheduleID, OwnershipFingerprint: request.ExpectedOwnershipFingerprint}, nil
}

func (s *stripeCatalogMigrationScheduler) RestoreCatalogMigration(ctx context.Context, request CatalogMigrationProviderScheduleRequest) error {
	if err := validateCatalogMigrationScheduleRequest(request); err != nil {
		return err
	}
	sub, _, _, err := s.freshAndValidate(ctx, request)
	if err != nil {
		return err
	}
	if strings.TrimSpace(request.PreviousScheduleSnapshot) != "" {
		return errors.New("catalog migration cannot restore an unknown previous Stripe schedule")
	}
	scheduleID := stripeSubscriptionScheduleID(sub)
	if scheduleID == "" {
		// Provider-success/local-loss replay converges here: absence is safe only
		// after the authoritative subscription still proves the original price.
		return nil
	}
	schedule, err := s.provider.GetSchedule(ctx, scheduleID)
	if err != nil {
		return err
	}
	metadata := catalogMigrationScheduleMetadata(request)
	if !catalogMigrationScheduleExactlyOwned(schedule, request, metadata) {
		return errors.New("refusing to release a Stripe schedule not exactly owned by catalog migration")
	}
	released, err := s.provider.ReleaseSchedule(ctx, scheduleID, request.IdempotencyKey+":restore-release")
	if err != nil {
		return err
	}
	if released == nil || strings.TrimSpace(released.ID) != scheduleID {
		return errors.New("Stripe catalog migration schedule release could not be confirmed")
	}
	return nil
}

func (s *stripeCatalogMigrationScheduler) freshAndValidate(ctx context.Context, request CatalogMigrationProviderScheduleRequest) (*stripe.Subscription, *stripe.Price, *stripe.Price, error) {
	if s == nil || s.provider == nil || s.sandbox == nil {
		return nil, nil, nil, errors.New("Stripe catalog migration provider is unavailable")
	}
	config := s.sandbox()
	if err := ValidateCatalogMigrationCommonSandbox(config, request.ContractID); err != nil {
		return nil, nil, nil, err
	}
	if !catalogMigrationHasTestStripeCredentials(config) {
		return nil, nil, nil, errors.New("catalog migration requires Stripe test credentials")
	}
	var binding model.SubscriptionProviderBinding
	if model.DB == nil || model.DB.WithContext(ctx).Where("id = ?", request.BindingID).First(&binding).Error != nil {
		return nil, nil, nil, errors.New("catalog migration Stripe binding is unavailable")
	}
	if binding.Id != request.BindingID || binding.UserId != request.UserID || binding.ContractId != request.ContractID ||
		binding.Provider != model.PaymentProviderStripe || binding.Livemode || strings.TrimSpace(binding.ProviderStatus) != "active" ||
		binding.EndedAt != 0 || binding.CancelAtPeriodEnd || strings.TrimSpace(binding.ProviderSubscriptionId) != request.ProviderSubscriptionID ||
		strings.TrimSpace(binding.ProviderCustomerId) != request.ProviderCustomerID || strings.TrimSpace(binding.ProviderSubscriptionItemId) != request.ProviderSubscriptionItemID ||
		strings.TrimSpace(binding.ProviderPriceId) != request.CurrentPriceID || binding.CurrentPeriodStart != request.CurrentPeriodStart || binding.CurrentPeriodEnd != request.CurrentPeriodEnd ||
		binding.LifecycleActionSeq != request.LifecycleActionSeq || binding.LifecycleReservationAction != model.SubscriptionProviderLifecycleActionCatalogMigration ||
		strings.TrimSpace(binding.LifecycleReservationToken) != request.LifecycleReservationToken || binding.LifecycleReservationUntil != request.LifecycleReservationUntil || binding.LifecycleReservationUntil <= time.Now().Unix() {
		return nil, nil, nil, errors.New("catalog migration Stripe binding facts drifted")
	}
	sub, err := s.provider.GetSubscription(ctx, request.ProviderSubscriptionID)
	if err != nil {
		return nil, nil, nil, err
	}
	current, err := s.provider.GetPrice(ctx, request.CurrentPriceID)
	if err != nil {
		return nil, nil, nil, err
	}
	target, err := s.provider.GetPrice(ctx, request.TargetPriceID)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := validateFreshCatalogMigrationStripeFacts(config, request, sub, current, target); err != nil {
		return nil, nil, nil, err
	}
	providerScheduleID := stripeSubscriptionScheduleID(sub)
	if strings.TrimSpace(binding.ProviderScheduleId) != providerScheduleID {
		// Provider-success/local-loss can occur in either direction: after create
		// local may be empty, and after release local may still name the schedule.
		// A non-empty disagreement is never safe. The caller proves exact provider
		// ownership (or authoritative absence) before mutation/convergence.
		if strings.TrimSpace(binding.ProviderScheduleId) != "" && providerScheduleID != "" {
			return nil, nil, nil, errors.New("catalog migration Stripe schedule binding drifted")
		}
	}
	return sub, current, target, nil
}

func validateFreshCatalogMigrationStripeFacts(config CatalogMigrationSandboxConfig, request CatalogMigrationProviderScheduleRequest, sub *stripe.Subscription, current, target *stripe.Price) error {
	if sub == nil || current == nil || target == nil {
		return errors.New("Stripe catalog migration facts are incomplete")
	}
	currentSnapshot, err := DecodeRecurringPlanSnapshotV1(request.CurrentPlanSnapshot)
	if err != nil {
		return err
	}
	targetSnapshot, err := DecodeRecurringPlanSnapshotV1(request.TargetPlanSnapshot)
	if err != nil {
		return err
	}
	if err := ValidateRecurringPlanSnapshotV1AgainstStripePrice(currentSnapshot, current); err != nil {
		return err
	}
	if err := ValidateRecurringPlanSnapshotV1AgainstStripePrice(targetSnapshot, target); err != nil {
		return err
	}
	customerID := ""
	if sub.Customer != nil {
		customerID = sub.Customer.ID
	}
	if err := ValidateCatalogMigrationStripeSandbox(config, CatalogMigrationStripeSandboxFacts{
		ContractID: request.ContractID, BindingLivemode: false, SubscriptionLivemode: sub.Livemode,
		CurrentPriceLivemode: current.Livemode, TargetPriceLivemode: target.Livemode,
		BindingSubscriptionID: request.ProviderSubscriptionID, SubscriptionID: sub.ID,
		BindingCustomerID: request.ProviderCustomerID, SubscriptionCustomerID: customerID,
		BindingCurrentPriceID: request.CurrentPriceID, SubscriptionPriceID: stripeSubscriptionFirstPriceID(sub),
		ExpectedTargetPriceID: request.TargetPriceID, TargetPriceID: target.ID,
	}); err != nil {
		return err
	}
	if sub.Status != stripe.SubscriptionStatusActive || sub.CancelAtPeriodEnd {
		return errors.New("Stripe subscription is not active for catalog migration")
	}
	if stripeSubscriptionFirstItemID(sub) != request.ProviderSubscriptionItemID || sub.Items == nil || len(sub.Items.Data) != 1 {
		return errors.New("Stripe subscription item mismatch")
	}
	if stripeSubscriptionCurrentPeriodStart(sub) != request.CurrentPeriodStart || stripeSubscriptionCurrentPeriodEnd(sub) != request.CurrentPeriodEnd || request.CurrentPeriodEnd <= request.CurrentPeriodStart {
		return errors.New("Stripe subscription period mismatch")
	}
	providerView := CatalogMigrationStripeSubscription{ID: sub.ID, CustomerID: customerID, ItemIDs: []string{stripeSubscriptionFirstItemID(sub)}, PriceIDs: []string{stripeSubscriptionFirstPriceID(sub)}, Status: string(sub.Status), CurrentPeriodStart: stripeSubscriptionCurrentPeriodStart(sub), CurrentPeriodEnd: stripeSubscriptionCurrentPeriodEnd(sub), ScheduleID: stripeSubscriptionScheduleID(sub), CancelAtPeriodEnd: sub.CancelAtPeriodEnd, Livemode: sub.Livemode}
	fingerprint := fingerprintJSON(struct {
		Subscription, Customer, Item, Schedule, Status string
		PeriodStart, PeriodEnd                         int64
		Livemode, CancelAtPeriodEnd                    bool
		CurrentPrice, TargetPrice                      any
	}{redactIdentifier(providerView.ID), redactIdentifier(providerView.CustomerID), redactIdentifier(providerView.ItemIDs[0]), redactIdentifier(providerView.ScheduleID), providerView.Status, providerView.CurrentPeriodStart, providerView.CurrentPeriodEnd, providerView.Livemode, providerView.CancelAtPeriodEnd, catalogMigrationPriceFingerprintFacts(current), catalogMigrationPriceFingerprintFacts(target)})
	// On a replay the owned schedule is expected to differ from the preview's
	// empty-schedule fingerprint; all other facts remain independently checked.
	if stripeSubscriptionScheduleID(sub) == "" && strings.TrimSpace(request.ExpectedProviderFingerprint) != "" && fingerprint != request.ExpectedProviderFingerprint {
		return errors.New("Stripe catalog migration provider facts drifted")
	}
	return nil
}

func validateCatalogMigrationScheduleRequest(request CatalogMigrationProviderScheduleRequest) error {
	if request.ContractID <= 0 || request.IntentID <= 0 || request.BindingID <= 0 || request.ChangeVersion <= 0 ||
		strings.TrimSpace(request.BatchID) == "" || strings.TrimSpace(request.ProviderSubscriptionID) == "" ||
		strings.TrimSpace(request.ProviderSubscriptionItemID) == "" || strings.TrimSpace(request.ProviderCustomerID) == "" ||
		strings.TrimSpace(request.CurrentPriceID) == "" || strings.TrimSpace(request.TargetPriceID) == "" ||
		request.CurrentPeriodStart <= 0 || request.CurrentPeriodEnd <= request.CurrentPeriodStart ||
		strings.TrimSpace(request.IdempotencyKey) == "" || strings.TrimSpace(request.ExpectedOwnershipFingerprint) == "" ||
		request.LifecycleActionSeq <= 0 || strings.TrimSpace(request.LifecycleReservationToken) == "" || request.LifecycleReservationUntil <= 0 {
		return errors.New("catalog migration Stripe schedule request is incomplete")
	}
	return nil
}

func catalogMigrationScheduleMetadata(request CatalogMigrationProviderScheduleRequest) map[string]string {
	return map[string]string{
		catalogMigrationMetadataBatch:       strings.TrimSpace(request.BatchID),
		catalogMigrationMetadataContract:    strconv.FormatInt(request.ContractID, 10),
		catalogMigrationMetadataIntent:      strconv.FormatInt(request.IntentID, 10),
		catalogMigrationMetadataIdempotency: strings.TrimSpace(request.IdempotencyKey),
		catalogMigrationMetadataOwnership:   strings.TrimSpace(request.ExpectedOwnershipFingerprint),
	}
}

func catalogMigrationScheduleParams(request CatalogMigrationProviderScheduleRequest, metadata map[string]string, target *stripe.Price) *stripe.SubscriptionScheduleParams {
	targetEnd := time.Unix(request.CurrentPeriodEnd, 0).UTC().AddDate(0, 1, 0).Unix()
	params := &stripe.SubscriptionScheduleParams{EndBehavior: stripe.String(string(stripe.SubscriptionScheduleEndBehaviorRelease)), Metadata: metadata, ProrationBehavior: stripe.String("none")}
	params.Phases = []*stripe.SubscriptionSchedulePhaseParams{
		{StartDate: stripe.Int64(request.CurrentPeriodStart), EndDate: stripe.Int64(request.CurrentPeriodEnd), ProrationBehavior: stripe.String("none"), Items: []*stripe.SubscriptionSchedulePhaseItemParams{{Price: stripe.String(request.CurrentPriceID), Quantity: stripe.Int64(1)}}},
		{StartDate: stripe.Int64(request.CurrentPeriodEnd), EndDate: stripe.Int64(targetEnd), ProrationBehavior: stripe.String("none"), Items: []*stripe.SubscriptionSchedulePhaseItemParams{{Price: stripe.String(target.ID), Quantity: stripe.Int64(1)}}},
	}
	return params
}

func catalogMigrationScheduleMetadataMatches(schedule *stripe.SubscriptionSchedule, request CatalogMigrationProviderScheduleRequest, metadata map[string]string) bool {
	if schedule == nil || schedule.Livemode || strings.TrimSpace(schedule.ID) == "" || schedule.Subscription == nil || strings.TrimSpace(schedule.Subscription.ID) != request.ProviderSubscriptionID || len(schedule.Metadata) != len(metadata) {
		return false
	}
	for key, value := range metadata {
		if schedule.Metadata[key] != value {
			return false
		}
	}
	return true
}

func catalogMigrationScheduleExactlyOwned(schedule *stripe.SubscriptionSchedule, request CatalogMigrationProviderScheduleRequest, metadata map[string]string) bool {
	if !catalogMigrationScheduleMetadataMatches(schedule, request, metadata) || schedule.EndBehavior != stripe.SubscriptionScheduleEndBehaviorRelease || (schedule.Status != stripe.SubscriptionScheduleStatusActive && schedule.Status != stripe.SubscriptionScheduleStatusNotStarted) || len(schedule.Phases) != 2 {
		return false
	}
	targetEnd := time.Unix(request.CurrentPeriodEnd, 0).UTC().AddDate(0, 1, 0).Unix()
	return catalogMigrationSchedulePhaseMatches(schedule.Phases[0], request.CurrentPeriodStart, request.CurrentPeriodEnd, request.CurrentPriceID) &&
		catalogMigrationSchedulePhaseMatches(schedule.Phases[1], request.CurrentPeriodEnd, targetEnd, request.TargetPriceID)
}

func catalogMigrationScheduleIsUnconfigured(schedule *stripe.SubscriptionSchedule) bool {
	// Stripe's from_subscription create returns one phase representing the
	// subscription's current billing period.  The target phase and ownership
	// metadata are added by the subsequent update call.
	return schedule != nil && len(schedule.Phases) == 1
}

func catalogMigrationScheduleIsUnconfiguredForRequest(schedule *stripe.SubscriptionSchedule, request CatalogMigrationProviderScheduleRequest) bool {
	return catalogMigrationScheduleIsUnconfigured(schedule) && catalogMigrationScheduleCurrentPhaseMatches(schedule.Phases[0], request.CurrentPeriodStart, request.CurrentPeriodEnd, request.CurrentPriceID)
}

func catalogMigrationScheduleCreateOutcomeMatches(schedule *stripe.SubscriptionSchedule, request CatalogMigrationProviderScheduleRequest, metadata map[string]string) bool {
	if schedule == nil || schedule.Livemode || strings.TrimSpace(schedule.ID) == "" || schedule.Subscription == nil || strings.TrimSpace(schedule.Subscription.ID) != request.ProviderSubscriptionID || !catalogMigrationScheduleIsUnconfiguredForRequest(schedule, request) {
		return false
	}
	// The create call intentionally sends no metadata because Stripe rejects
	// metadata together with from_subscription.  If a provider returns
	// metadata anyway, only the exact migration marker is acceptable.
	return len(schedule.Metadata) == 0 || catalogMigrationScheduleMetadataMatches(schedule, request, metadata)
}

func catalogMigrationScheduleCurrentPhaseMatches(phase *stripe.SubscriptionSchedulePhase, start, end int64, priceID string) bool {
	return phase != nil && phase.StartDate == start && phase.EndDate == end && len(phase.Items) == 1 && phase.Items[0] != nil && phase.Items[0].Price != nil && phase.Items[0].Price.ID == priceID && phase.Items[0].Quantity == 1
}

func catalogMigrationSchedulePhaseMatches(phase *stripe.SubscriptionSchedulePhase, start, end int64, priceID string) bool {
	return phase != nil && phase.StartDate == start && phase.EndDate == end && phase.ProrationBehavior == stripe.SubscriptionSchedulePhaseProrationBehaviorNone && len(phase.Items) == 1 && phase.Items[0] != nil && phase.Items[0].Price != nil && phase.Items[0].Price.ID == priceID && phase.Items[0].Quantity == 1
}

func (stripeCatalogMigrationAPI) GetSubscription(ctx context.Context, id string) (*stripe.Subscription, error) {
	stripe.Key = setting.StripeApiSecret
	params := &stripe.SubscriptionParams{}
	params.Context = ctx
	params.AddExpand("customer")
	params.AddExpand("items.data.price")
	params.AddExpand("schedule")
	return stripesubscription.Get(strings.TrimSpace(id), params)
}

func (stripeCatalogMigrationAPI) GetPrice(ctx context.Context, id string) (*stripe.Price, error) {
	stripe.Key = setting.StripeApiSecret
	params := &stripe.PriceParams{}
	params.Context = ctx
	return stripeprice.Get(strings.TrimSpace(id), params)
}

func (stripeCatalogMigrationAPI) GetSchedule(ctx context.Context, id string) (*stripe.SubscriptionSchedule, error) {
	stripe.Key = setting.StripeApiSecret
	params := &stripe.SubscriptionScheduleParams{}
	params.Context = ctx
	params.AddExpand("subscription")
	params.AddExpand("phases.items.price")
	return stripeschedule.Get(strings.TrimSpace(id), params)
}

func (stripeCatalogMigrationAPI) CreateSchedule(ctx context.Context, subscriptionID string, _ map[string]string, key string) (*stripe.SubscriptionSchedule, error) {
	stripe.Key = setting.StripeApiSecret
	// Stripe rejects metadata (and phase fields) when from_subscription is
	// supplied.  Ownership metadata and the target phase are applied by the
	// immediately-following UpdateSchedule call.
	params := &stripe.SubscriptionScheduleParams{FromSubscription: stripe.String(strings.TrimSpace(subscriptionID))}
	params.Context = ctx
	params.SetIdempotencyKey(key)
	return stripeschedule.New(params)
}

func (stripeCatalogMigrationAPI) UpdateSchedule(ctx context.Context, id string, params *stripe.SubscriptionScheduleParams, key string) (*stripe.SubscriptionSchedule, error) {
	stripe.Key = setting.StripeApiSecret
	params.Context = ctx
	params.SetIdempotencyKey(key)
	return stripeschedule.Update(strings.TrimSpace(id), params)
}

func (stripeCatalogMigrationAPI) ReleaseSchedule(ctx context.Context, id, key string) (*stripe.SubscriptionSchedule, error) {
	stripe.Key = setting.StripeApiSecret
	params := &stripe.SubscriptionScheduleReleaseParams{PreserveCancelDate: stripe.Bool(true)}
	params.Context = ctx
	params.SetIdempotencyKey(key)
	return stripeschedule.Release(strings.TrimSpace(id), params)
}
