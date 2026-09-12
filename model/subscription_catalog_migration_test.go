package model

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type legacySubscriptionChangeIntentBeforeCatalogMigration struct {
	Id          int64
	ContractId  int64
	UserId      int
	RequestId   string
	Kind        string
	PaymentMode string
	Status      string
}

func (legacySubscriptionChangeIntentBeforeCatalogMigration) TableName() string {
	return "subscription_change_intents"
}

type legacySubscriptionProviderBindingBeforeCurrentSnapshot struct {
	Id                     int64
	UserId                 int
	PlanId                 int
	Provider               string
	ProviderSubscriptionId string
}

func (legacySubscriptionProviderBindingBeforeCurrentSnapshot) TableName() string {
	return "subscription_provider_bindings"
}

func migrateSubscriptionCatalogMigrationTestDB(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(
		&SubscriptionCatalogMigrationBatch{},
		&SubscriptionChangeIntent{},
		&SubscriptionProviderBinding{},
	))
}

func createCatalogMigrationBatchForTest(t *testing.T, id string, requestID string, digestByte string) *SubscriptionCatalogMigrationBatch {
	t.Helper()
	batch := &SubscriptionCatalogMigrationBatch{
		Id:                    id,
		RequestId:             requestID,
		CohortDigest:          strings.Repeat(digestByte, 64),
		Status:                SubscriptionCatalogMigrationBatchStatusApplying,
		ManifestSnapshot:      `{"version":1}`,
		SummarySnapshot:       `{"eligible":1}`,
		RequestedBy:           1,
		DeploymentEnvironment: "staging",
		ServiceName:           "newapi-staging",
		SandboxOnly:           true,
	}
	require.NoError(t, DB.Create(batch).Error)
	return batch
}

func TestSubscriptionCatalogMigrationAutoMigrateCreatesRequiredSchema(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	migrateSubscriptionCatalogMigrationTestDB(t)

	require.True(t, DB.Migrator().HasTable(&SubscriptionCatalogMigrationBatch{}))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionChangeIntent{}, "catalog_migration_batch_id"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionChangeIntent{}, "target_plan_snapshot"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionChangeIntent{}, "provider_schedule_fingerprint"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionProviderBinding{}, "current_plan_snapshot"))
	require.True(t, DB.Migrator().HasIndex(&SubscriptionCatalogMigrationBatch{}, "ux_subscription_catalog_migration_request"))
	require.True(t, DB.Migrator().HasIndex(&SubscriptionCatalogMigrationBatch{}, "ux_subscription_catalog_migration_digest"))
	require.True(t, DB.Migrator().HasIndex(&SubscriptionChangeIntent{}, "ux_catalog_migration_batch_contract"))
}

func TestSubscriptionCatalogMigrationIsRegisteredInStartupMigration(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)

	require.NoError(t, migrateDBFast())
	require.True(t, DB.Migrator().HasTable(&SubscriptionCatalogMigrationBatch{}))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionChangeIntent{}, "target_plan_snapshot"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionProviderBinding{}, "current_plan_snapshot"))
}

func TestSubscriptionCatalogMigrationBatchRejectsLivemode(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	migrateSubscriptionCatalogMigrationTestDB(t)

	batch := &SubscriptionCatalogMigrationBatch{
		Id:                    "batch-live-mode",
		RequestId:             "operator-request-live-mode",
		CohortDigest:          strings.Repeat("f", 64),
		Status:                SubscriptionCatalogMigrationBatchStatusApplying,
		ManifestSnapshot:      `{}`,
		SummarySnapshot:       `{}`,
		RequestedBy:           1,
		DeploymentEnvironment: "staging",
		ServiceName:           "newapi-staging",
		SandboxOnly:           true,
		Livemode:              true,
	}

	require.Error(t, DB.Create(batch).Error)
}

func TestSubscriptionChangeIntentCatalogMigrationKindSurvivesNormalization(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	migrateSubscriptionCatalogMigrationTestDB(t)

	intent := &SubscriptionChangeIntent{
		ContractId:  3001,
		UserId:      3002,
		RequestId:   " catalog-migration-kind ",
		Kind:        " catalog_migration ",
		PaymentMode: SubscriptionPaymentModeStripeRecurring,
		Status:      SubscriptionChangeIntentStatusScheduled,
	}
	require.NoError(t, DB.Create(intent).Error)

	var stored SubscriptionChangeIntent
	require.NoError(t, DB.First(&stored, "id = ?", intent.Id).Error)
	require.Equal(t, SubscriptionChangeIntentKindCatalogMigration, stored.Kind)
}

func TestSubscriptionChangeIntentNeedsAttentionStatusSurvivesNormalization(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	migrateSubscriptionCatalogMigrationTestDB(t)

	intent := &SubscriptionChangeIntent{
		ContractId:  3011,
		UserId:      3012,
		RequestId:   "catalog-migration-needs-attention",
		Kind:        SubscriptionChangeIntentKindCatalogMigration,
		PaymentMode: SubscriptionPaymentModeStripeRecurring,
		Status:      " needs_attention ",
	}
	require.NoError(t, DB.Create(intent).Error)

	var stored SubscriptionChangeIntent
	require.NoError(t, DB.First(&stored, "id = ?", intent.Id).Error)
	require.Equal(t, SubscriptionChangeIntentStatusNeedsAttention, stored.Status)
}

func TestSubscriptionChangeIntentTargetSnapshotRoundTripsWithoutTruncation(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	migrateSubscriptionCatalogMigrationTestDB(t)

	snapshot := `{"version":1,"padding":"` + strings.Repeat("x", 70_000) + `"}`
	intent := &SubscriptionChangeIntent{
		ContractId:         3021,
		UserId:             3022,
		RequestId:          "catalog-migration-long-snapshot",
		Kind:               SubscriptionChangeIntentKindCatalogMigration,
		PaymentMode:        SubscriptionPaymentModeStripeRecurring,
		Status:             SubscriptionChangeIntentStatusCreated,
		TargetPlanSnapshot: snapshot,
	}
	require.NoError(t, DB.Create(intent).Error)

	var stored SubscriptionChangeIntent
	require.NoError(t, DB.First(&stored, "id = ?", intent.Id).Error)
	require.Equal(t, snapshot, stored.TargetPlanSnapshot)
}

func TestSubscriptionProviderBindingCurrentSnapshotRoundTripsWithoutJSONExposure(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	migrateSubscriptionCatalogMigrationTestDB(t)

	snapshot := `{"version":1,"plan_id":5,"base_price_minor":1000}`
	binding := &SubscriptionProviderBinding{
		UserId:                 3031,
		PlanId:                 5,
		Provider:               PaymentProviderStripe,
		ProviderSubscriptionId: "sub_catalog_snapshot",
		CurrentPlanSnapshot:    snapshot,
	}
	require.NoError(t, DB.Create(binding).Error)

	var stored SubscriptionProviderBinding
	require.NoError(t, DB.First(&stored, "id = ?", binding.Id).Error)
	require.Equal(t, snapshot, stored.CurrentPlanSnapshot)

	payload, err := json.Marshal(stored)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "current_plan_snapshot")
	require.NotContains(t, string(payload), "base_price_minor")
}

func TestSubscriptionCatalogMigrationBatchRejectsDuplicateRequestID(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	migrateSubscriptionCatalogMigrationTestDB(t)

	createCatalogMigrationBatchForTest(t, "batch-request-1", "operator-request", "a")
	duplicate := &SubscriptionCatalogMigrationBatch{
		Id:               "batch-request-2",
		RequestId:        "operator-request",
		CohortDigest:     strings.Repeat("b", 64),
		Status:           SubscriptionCatalogMigrationBatchStatusApplying,
		SandboxOnly:      true,
		ManifestSnapshot: `{}`,
		SummarySnapshot:  `{}`,
	}

	require.Error(t, DB.Create(duplicate).Error)
}

func TestSubscriptionCatalogMigrationBatchRejectsDuplicateCohortDigest(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	migrateSubscriptionCatalogMigrationTestDB(t)

	createCatalogMigrationBatchForTest(t, "batch-digest-1", "operator-request-1", "c")
	duplicate := &SubscriptionCatalogMigrationBatch{
		Id:               "batch-digest-2",
		RequestId:        "operator-request-2",
		CohortDigest:     strings.Repeat("c", 64),
		Status:           SubscriptionCatalogMigrationBatchStatusApplying,
		SandboxOnly:      true,
		ManifestSnapshot: `{}`,
		SummarySnapshot:  `{}`,
	}

	require.Error(t, DB.Create(duplicate).Error)
}

func TestSubscriptionCatalogMigrationBatchManifestCannotBeUpdated(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	migrateSubscriptionCatalogMigrationTestDB(t)

	batch := createCatalogMigrationBatchForTest(t, "batch-immutable", "operator-request-immutable", "d")
	require.NoError(t, DB.Model(batch).Updates(map[string]interface{}{
		"manifest_snapshot": `{"version":2}`,
		"summary_snapshot":  `{"eligible":0}`,
	}).Error)

	var stored SubscriptionCatalogMigrationBatch
	require.NoError(t, DB.First(&stored, "id = ?", batch.Id).Error)
	require.Equal(t, `{"version":1}`, stored.ManifestSnapshot)
	require.Equal(t, `{"eligible":0}`, stored.SummarySnapshot)
}

func TestSubscriptionChangeIntentAllowsOnlyOneIntentPerBatchAndContract(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	migrateSubscriptionCatalogMigrationTestDB(t)

	batch := createCatalogMigrationBatchForTest(t, "batch-contract-fence", "operator-request-fence", "e")
	first := &SubscriptionChangeIntent{
		ContractId:              3041,
		UserId:                  3042,
		RequestId:               "catalog-fence-1",
		Kind:                    SubscriptionChangeIntentKindCatalogMigration,
		PaymentMode:             SubscriptionPaymentModeStripeRecurring,
		Status:                  SubscriptionChangeIntentStatusCreated,
		CatalogMigrationBatchId: &batch.Id,
	}
	require.NoError(t, DB.Create(first).Error)

	duplicate := *first
	duplicate.Id = 0
	duplicate.RequestId = "catalog-fence-2"
	require.Error(t, DB.Create(&duplicate).Error)
}

func TestSubscriptionChangeIntentNullableBatchDoesNotConstrainExistingIntents(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	migrateSubscriptionCatalogMigrationTestDB(t)

	for index, kind := range []string{SubscriptionChangeIntentKindUpgrade, SubscriptionChangeIntentKindDowngrade} {
		intent := &SubscriptionChangeIntent{
			ContractId:  3051,
			UserId:      3052,
			RequestId:   "legacy-intent-" + string(rune('a'+index)),
			Kind:        kind,
			PaymentMode: SubscriptionPaymentModeStripeRecurring,
			Status:      SubscriptionChangeIntentStatusCreated,
		}
		require.NoError(t, DB.Create(intent).Error)
		require.Nil(t, intent.CatalogMigrationBatchId)
	}
}

func TestCatalogMigrationAutoMigratePreservesLegacyRowsWithBlankNewFields(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)

	require.NoError(t, DB.AutoMigrate(
		&legacySubscriptionChangeIntentBeforeCatalogMigration{},
		&legacySubscriptionProviderBindingBeforeCurrentSnapshot{},
	))
	require.NoError(t, DB.Create(&legacySubscriptionChangeIntentBeforeCatalogMigration{
		Id:          3061,
		ContractId:  3062,
		UserId:      3063,
		RequestId:   "legacy-before-catalog",
		Kind:        SubscriptionChangeIntentKindUpgrade,
		PaymentMode: SubscriptionPaymentModeStripeRecurring,
		Status:      SubscriptionChangeIntentStatusScheduled,
	}).Error)
	require.NoError(t, DB.Create(&legacySubscriptionProviderBindingBeforeCurrentSnapshot{
		Id:                     3064,
		UserId:                 3063,
		PlanId:                 1,
		Provider:               PaymentProviderStripe,
		ProviderSubscriptionId: "sub_legacy_before_snapshot",
	}).Error)

	migrateSubscriptionCatalogMigrationTestDB(t)

	var intent SubscriptionChangeIntent
	require.NoError(t, DB.First(&intent, "id = ?", 3061).Error)
	require.Equal(t, SubscriptionChangeIntentKindUpgrade, intent.Kind)
	require.Nil(t, intent.CatalogMigrationBatchId)
	require.Empty(t, intent.TargetPlanSnapshot)
	require.Empty(t, intent.ProviderScheduleFingerprint)

	var binding SubscriptionProviderBinding
	require.NoError(t, DB.First(&binding, "id = ?", 3064).Error)
	require.Equal(t, 1, binding.PlanId)
	require.Empty(t, binding.CurrentPlanSnapshot)
}
