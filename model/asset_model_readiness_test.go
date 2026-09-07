package model

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openAssetModelReadinessTestDB(t *testing.T) {
	t.Helper()

	previous := DB
	dsn := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
		DB = previous
	})
	DB = db
	require.NoError(t, DB.AutoMigrate(&AssetModelCoverageTarget{}, &AssetModelReadiness{}, &AssetBinding{}))
}

func TestAssetModelReadinessAutoMigrateAndCompositeUniqueness(t *testing.T) {
	openAssetModelReadinessTestDB(t)

	require.NoError(t, DB.Create(&AssetModelCoverageTarget{
		ScopeKey:   "scope-a",
		ModelName:  "model-a",
		Status:     AssetModelTargetStatusSelecting,
		CreatedAt:  10,
		UpdatedAt:  10,
		Generation: 0,
	}).Error)
	require.Error(t, DB.Create(&AssetModelCoverageTarget{
		ScopeKey:  "scope-a",
		ModelName: "model-a",
		Status:    AssetModelTargetStatusSelecting,
		CreatedAt: 11,
		UpdatedAt: 11,
	}).Error)

	require.NoError(t, DB.Create(&AssetModelReadiness{
		AssetId:   7,
		ScopeKey:  "scope-a",
		ModelName: "model-a",
		Status:    AssetModelReadinessStatusPending,
	}).Error)
	require.Error(t, DB.Create(&AssetModelReadiness{
		AssetId:   7,
		ScopeKey:  "scope-a",
		ModelName: "model-a",
		Status:    AssetModelReadinessStatusPending,
	}).Error)
}

func TestAssetModelEnsureReadinessCanonicalIdempotency(t *testing.T) {
	openAssetModelReadinessTestDB(t)

	require.NoError(t, EnsureAssetModelReadiness(42, " scope-a ", []string{" zeta ", "", "alpha", "zeta", " alpha "}, 100))
	require.NoError(t, EnsureAssetModelReadiness(42, "scope-a", []string{"alpha", "zeta"}, 200))
	require.NoError(t, EnsureAssetModelReadiness(0, "scope-a", []string{"ignored"}, 200))
	require.NoError(t, EnsureAssetModelReadiness(42, "", []string{"ignored"}, 200))
	require.NoError(t, EnsureAssetModelReadiness(42, "scope-a", []string{" ", ""}, 200))

	rows, err := ListAssetModelReadiness(42, "scope-a", []string{"zeta", "alpha", "alpha"})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, "alpha", rows[0].ModelName)
	require.Equal(t, "zeta", rows[1].ModelName)
	for _, row := range rows {
		require.Equal(t, AssetModelReadinessStatusPending, row.Status)
		require.Equal(t, int64(100), row.CreatedAt)
		require.Equal(t, int64(100), row.UpdatedAt)
	}
}

func TestAssetModelReadinessDueFilteringAndOrder(t *testing.T) {
	openAssetModelReadinessTestDB(t)

	seed := []AssetModelReadiness{
		{AssetId: 1, ScopeKey: "s", ModelName: "pending-leased", Status: AssetModelReadinessStatusPending, LeaseOwner: "other", LeaseExpiresAt: 1000, UpdatedAt: 1},
		{AssetId: 1, ScopeKey: "s", ModelName: "retry-future", Status: AssetModelReadinessStatusRetryWaiting, NextRetryAt: 600, UpdatedAt: 2},
		{AssetId: 1, ScopeKey: "s", ModelName: "processing-live", Status: AssetModelReadinessStatusProcessing, LeaseOwner: "w", LeaseExpiresAt: 600, UpdatedAt: 3},
		{AssetId: 1, ScopeKey: "s", ModelName: "failed", Status: AssetModelReadinessStatusFailed, UpdatedAt: 4},
		{AssetId: 1, ScopeKey: "s", ModelName: "retry-due", Status: AssetModelReadinessStatusRetryWaiting, NextRetryAt: 20, UpdatedAt: 8},
		{AssetId: 1, ScopeKey: "s", ModelName: "pending", Status: AssetModelReadinessStatusPending, UpdatedAt: 5},
		{AssetId: 1, ScopeKey: "s", ModelName: "processing-expired", Status: AssetModelReadinessStatusProcessing, LeaseOwner: "w", LeaseExpiresAt: 40, UpdatedAt: 7},
	}
	require.NoError(t, DB.Create(&seed).Error)

	rows, err := ListDueAssetModelReadiness(50, 10)
	require.NoError(t, err)
	require.Equal(t, []string{"pending", "processing-expired", "retry-due"}, readinessModelNames(rows))
}

func TestAssetModelReadinessLeaseSingleWinnerAndAttemptStart(t *testing.T) {
	openAssetModelReadinessTestDB(t)
	require.NoError(t, EnsureAssetModelReadiness(7, "scope", []string{"model"}, 10))
	row := requireOneReadiness(t, 7, "scope", "model")

	won, err := ClaimAssetModelReadinessLease(row.Id, "owner-a", 20, 80)
	require.NoError(t, err)
	require.True(t, won)
	won, err = ClaimAssetModelReadinessLease(row.Id, "owner-b", 21, 90)
	require.NoError(t, err)
	require.False(t, won)

	row = requireOneReadiness(t, 7, "scope", "model")
	require.Equal(t, AssetModelReadinessStatusProcessing, row.Status)
	require.Equal(t, "owner-a", row.LeaseOwner)
	require.Equal(t, int64(80), row.LeaseExpiresAt)
	require.Equal(t, int64(20), row.AttemptStartedAt)
	require.Equal(t, 1, row.AttemptCount)

	won, err = ClaimAssetModelReadinessLease(row.Id, "owner-b", 81, 120)
	require.NoError(t, err)
	require.True(t, won)
	row = requireOneReadiness(t, 7, "scope", "model")
	require.Equal(t, "owner-b", row.LeaseOwner)
	require.Equal(t, int64(20), row.AttemptStartedAt)
	require.Equal(t, 2, row.AttemptCount)
}

func TestAssetModelReadinessCASFencingRetryActiveFailedAndReset(t *testing.T) {
	openAssetModelReadinessTestDB(t)
	require.NoError(t, EnsureAssetModelReadiness(9, "scope", []string{"model"}, 10))
	row := requireOneReadiness(t, 9, "scope", "model")
	won, err := ClaimAssetModelReadinessLease(row.Id, "worker", 20, 100)
	require.NoError(t, err)
	require.True(t, won)
	row = requireOneReadiness(t, 9, "scope", "model")

	target := AssetModelCoverageTarget{
		ScopeKey:          "scope",
		ModelName:         "model",
		ChannelId:         101,
		BindingScope:      "tenant-a",
		Generation:        2,
		Status:            AssetModelTargetStatusActive,
		SpecificChannelId: 101,
		CandidateIndex:    3,
		CredentialIndex:   4,
	}
	reset, err := ResetAssetModelReadinessForTargetCAS(row.Id, "worker", row.AttemptCount, row.LeaseExpiresAt, target, 30)
	require.NoError(t, err)
	require.True(t, reset)
	row = requireOneReadiness(t, 9, "scope", "model")
	require.Equal(t, AssetModelReadinessStatusPending, row.Status)
	require.Equal(t, int64(2), row.TargetGeneration)
	require.Equal(t, 101, row.ChannelId)
	require.Equal(t, "tenant-a", row.BindingScope)
	require.Equal(t, 0, row.AttemptCount)
	require.Equal(t, int64(0), row.AttemptStartedAt)

	won, err = ClaimAssetModelReadinessLease(row.Id, "worker", 40, 120)
	require.NoError(t, err)
	require.True(t, won)
	row = requireOneReadiness(t, 9, "scope", "model")
	transition := AssetModelReadinessTransition{
		AssetId:                9,
		ScopeKey:               "scope",
		ModelName:              "model",
		TargetGeneration:       2,
		ChannelId:              101,
		BindingScope:           "tenant-a",
		LeaseOwner:             "worker",
		ExpectedAttemptCount:   row.AttemptCount,
		ExpectedLeaseExpiresAt: row.LeaseExpiresAt,
		Now:                    50,
	}
	ok, err := ScheduleAssetModelReadinessRetryCAS(transition, " provider/time out! ", 90)
	require.NoError(t, err)
	require.True(t, ok)
	row = requireOneReadiness(t, 9, "scope", "model")
	require.Equal(t, AssetModelReadinessStatusRetryWaiting, row.Status)
	require.Equal(t, "provider_time_out", row.ErrorClass)
	require.Equal(t, int64(90), row.NextRetryAt)
	require.Equal(t, "", row.LeaseOwner)

	won, err = ClaimAssetModelReadinessLease(row.Id, "worker", 91, 140)
	require.NoError(t, err)
	require.True(t, won)
	row = requireOneReadiness(t, 9, "scope", "model")
	transition.ExpectedAttemptCount = row.AttemptCount
	transition.ExpectedLeaseExpiresAt = row.LeaseExpiresAt
	stale := transition
	stale.TargetGeneration = 1
	stale.Now = 92
	ok, err = ActivateAssetModelReadinessCAS(stale)
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = ActivateAssetModelReadinessCAS(transition)
	require.NoError(t, err)
	require.True(t, ok)
	row = requireOneReadiness(t, 9, "scope", "model")
	require.Equal(t, AssetModelReadinessStatusActive, row.Status)
	require.Equal(t, "", row.ErrorClass)
	require.Equal(t, "", row.LeaseOwner)

	row.Status = AssetModelReadinessStatusProcessing
	row.LeaseOwner = "worker"
	row.LeaseExpiresAt = 200
	require.NoError(t, DB.Save(&row).Error)
	transition.ExpectedLeaseExpiresAt = 200
	ok, err = FailAssetModelReadinessCAS(transition, "fatal/provider")
	require.NoError(t, err)
	require.True(t, ok)
	row = requireOneReadiness(t, 9, "scope", "model")
	require.Equal(t, AssetModelReadinessStatusFailed, row.Status)
	require.Equal(t, "fatal_provider", row.ErrorClass)
}

func TestReopenFailedAssetModelReadinessForTargetCASResetsExactSnapshot(t *testing.T) {
	openAssetModelReadinessTestDB(t)

	expected := AssetModelReadiness{
		AssetId:          71,
		ScopeKey:         "scope-a",
		ModelName:        "seedance-2.0-pro",
		TargetGeneration: 3,
		ChannelId:        120,
		BindingScope:     "byteplus:v1:old",
		Status:           AssetModelReadinessStatusFailed,
		ErrorClass:       "target_unavailable",
		AttemptCount:     4,
		AttemptStartedAt: 101,
		NextRetryAt:      202,
		LeaseOwner:       "stale-worker",
		LeaseExpiresAt:   303,
		CreatedAt:        10,
		UpdatedAt:        20,
	}
	require.NoError(t, DB.Create(&expected).Error)

	target := AssetModelCoverageTarget{
		ScopeKey:     "scope-a",
		ModelName:    "seedance-2.0-pro",
		Generation:   4,
		ChannelId:    121,
		BindingScope: "byteplus:v1:new",
		Status:       AssetModelTargetStatusActive,
	}
	require.NoError(t, DB.Create(&target).Error)
	reopened, err := ReopenFailedAssetModelReadinessForTargetCAS(expected, target, false, 400)
	require.NoError(t, err)
	require.True(t, reopened)

	row := requireOneReadiness(t, 71, "scope-a", "seedance-2.0-pro")
	require.Equal(t, AssetModelReadinessStatusPending, row.Status)
	require.Empty(t, row.ErrorClass)
	require.Equal(t, int64(4), row.TargetGeneration)
	require.Equal(t, 121, row.ChannelId)
	require.Equal(t, "byteplus:v1:new", row.BindingScope)
	require.Zero(t, row.AttemptCount)
	require.Zero(t, row.AttemptStartedAt)
	require.Zero(t, row.NextRetryAt)
	require.Empty(t, row.LeaseOwner)
	require.Zero(t, row.LeaseExpiresAt)
	require.Equal(t, int64(400), row.UpdatedAt)
}

func TestReopenFailedAssetModelReadinessForTargetCASRejectsTargetChangedAfterCapture(t *testing.T) {
	tests := []struct {
		name   string
		change func(t *testing.T, target AssetModelCoverageTarget)
	}{
		{
			name: "rotated",
			change: func(t *testing.T, target AssetModelCoverageTarget) {
				require.NoError(t, DB.Model(&AssetModelCoverageTarget{}).Where("id = ?", target.Id).UpdateColumn("generation", target.Generation+1).Error)
			},
		},
		{
			name: "became unavailable",
			change: func(t *testing.T, target AssetModelCoverageTarget) {
				require.NoError(t, DB.Model(&AssetModelCoverageTarget{}).Where("id = ?", target.Id).UpdateColumn("status", AssetModelTargetStatusUnavailable).Error)
			},
		},
		{
			name: "deleted",
			change: func(t *testing.T, target AssetModelCoverageTarget) {
				require.NoError(t, DB.Delete(&AssetModelCoverageTarget{}, target.Id).Error)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			openAssetModelReadinessTestDB(t)

			expected := AssetModelReadiness{
				AssetId:          74,
				ScopeKey:         "scope-a",
				ModelName:        "seedance-2.0-pro",
				TargetGeneration: 3,
				ChannelId:        120,
				BindingScope:     "byteplus:v1:old",
				Status:           AssetModelReadinessStatusFailed,
				ErrorClass:       "target_unavailable",
				CreatedAt:        10,
				UpdatedAt:        20,
			}
			require.NoError(t, DB.Create(&expected).Error)
			target := AssetModelCoverageTarget{
				ScopeKey:     "scope-a",
				ModelName:    "seedance-2.0-pro",
				Generation:   4,
				ChannelId:    121,
				BindingScope: "byteplus:v1:new",
				Status:       AssetModelTargetStatusActive,
				CreatedAt:    10,
				UpdatedAt:    20,
			}
			require.NoError(t, DB.Create(&target).Error)
			captured := target
			tt.change(t, target)

			reopened, err := ReopenFailedAssetModelReadinessForTargetCAS(expected, captured, false, 400)
			require.NoError(t, err)
			require.False(t, reopened)
			persisted := requireOneReadiness(t, 74, "scope-a", "seedance-2.0-pro")
			require.Equal(t, AssetModelReadinessStatusFailed, persisted.Status)
			require.Equal(t, int64(3), persisted.TargetGeneration)
		})
	}
}

func TestReopenFailedAssetModelReadinessForTargetCASRequiresExactActiveBinding(t *testing.T) {
	tests := []struct {
		name   string
		change func(t *testing.T, binding AssetBinding)
		want   bool
	}{
		{name: "active binding", want: true},
		{
			name: "binding deactivated",
			change: func(t *testing.T, binding AssetBinding) {
				require.NoError(t, DB.Model(&AssetBinding{}).Where("id = ?", binding.Id).UpdateColumn("status", AssetStatusFailed).Error)
			},
		},
		{
			name: "binding missing",
			change: func(t *testing.T, binding AssetBinding) {
				require.NoError(t, DB.Delete(&AssetBinding{}, binding.Id).Error)
			},
		},
		{
			name: "binding upstream asset cleared",
			change: func(t *testing.T, binding AssetBinding) {
				require.NoError(t, DB.Model(&AssetBinding{}).Where("id = ?", binding.Id).UpdateColumn("upstream_asset_id", "").Error)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			openAssetModelReadinessTestDB(t)

			expected := AssetModelReadiness{
				AssetId:          75,
				ScopeKey:         "scope-a",
				ModelName:        "seedance-2.0-pro",
				TargetGeneration: 3,
				ChannelId:        120,
				BindingScope:     "byteplus:v1:old",
				Status:           AssetModelReadinessStatusFailed,
				ErrorClass:       "target_unavailable",
				CreatedAt:        10,
				UpdatedAt:        20,
			}
			require.NoError(t, DB.Create(&expected).Error)
			target := AssetModelCoverageTarget{
				ScopeKey:     "scope-a",
				ModelName:    "seedance-2.0-pro",
				Generation:   4,
				ChannelId:    121,
				BindingScope: "byteplus:v1:new",
				Status:       AssetModelTargetStatusActive,
				CreatedAt:    10,
				UpdatedAt:    20,
			}
			require.NoError(t, DB.Create(&target).Error)
			binding := AssetBinding{
				AssetId:         expected.AssetId,
				ChannelId:       target.ChannelId,
				BindingScope:    target.BindingScope,
				UpstreamAssetId: "upstream-asset",
				Status:          AssetStatusActive,
				CreatedAt:       10,
				UpdatedAt:       20,
			}
			require.NoError(t, DB.Create(&binding).Error)
			if tt.change != nil {
				tt.change(t, binding)
			}

			reopened, err := ReopenFailedAssetModelReadinessForTargetCAS(expected, target, true, 400)
			require.NoError(t, err)
			require.Equal(t, tt.want, reopened)
			persisted := requireOneReadiness(t, 75, "scope-a", "seedance-2.0-pro")
			if tt.want {
				require.Equal(t, AssetModelReadinessStatusPending, persisted.Status)
			} else {
				require.Equal(t, AssetModelReadinessStatusFailed, persisted.Status)
				require.Equal(t, int64(3), persisted.TargetGeneration)
			}
		})
	}
}

func TestReopenFailedAssetModelReadinessForTargetCASRejectsChangedSnapshot(t *testing.T) {
	tests := []struct {
		name   string
		column string
		value  any
	}{
		{name: "status", column: "status", value: AssetModelReadinessStatusProcessing},
		{name: "error class", column: "error_class", value: "source_unavailable"},
		{name: "target generation", column: "target_generation", value: int64(8)},
		{name: "channel", column: "channel_id", value: 121},
		{name: "binding scope", column: "binding_scope", value: "byteplus:v1:other"},
		{name: "attempt count", column: "attempt_count", value: 5},
		{name: "attempt start", column: "attempt_started_at", value: int64(102)},
		{name: "next retry", column: "next_retry_at", value: int64(203)},
		{name: "lease owner", column: "lease_owner", value: "new-worker"},
		{name: "lease expiry", column: "lease_expires_at", value: int64(304)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			openAssetModelReadinessTestDB(t)

			expected := AssetModelReadiness{
				AssetId:          72,
				ScopeKey:         "scope-a",
				ModelName:        "seedance-2.0-pro",
				TargetGeneration: 3,
				ChannelId:        120,
				BindingScope:     "byteplus:v1:old",
				Status:           AssetModelReadinessStatusFailed,
				ErrorClass:       "target_unavailable",
				AttemptCount:     4,
				AttemptStartedAt: 101,
				NextRetryAt:      202,
				LeaseOwner:       "stale-worker",
				LeaseExpiresAt:   303,
				CreatedAt:        10,
				UpdatedAt:        20,
			}
			require.NoError(t, DB.Create(&expected).Error)
			require.NoError(t, DB.Model(&AssetModelReadiness{}).Where("id = ?", expected.Id).UpdateColumn(tt.column, tt.value).Error)

			target := AssetModelCoverageTarget{
				ScopeKey:     "scope-a",
				ModelName:    "seedance-2.0-pro",
				Generation:   4,
				ChannelId:    121,
				BindingScope: "byteplus:v1:new",
				Status:       AssetModelTargetStatusActive,
			}
			require.NoError(t, DB.Create(&target).Error)
			reopened, err := ReopenFailedAssetModelReadinessForTargetCAS(expected, target, false, 400)
			require.NoError(t, err)
			require.False(t, reopened)
			persisted := requireOneReadiness(t, 72, "scope-a", "seedance-2.0-pro")
			require.NotEqual(t, AssetModelReadinessStatusPending, persisted.Status)
			require.NotEqual(t, target.Generation, persisted.TargetGeneration)
		})
	}
}

func TestReopenFailedAssetModelReadinessForTargetCASRejectsUnsafeInputs(t *testing.T) {
	tests := []struct {
		name           string
		mutateExpected func(*AssetModelReadiness)
		mutateTarget   func(*AssetModelCoverageTarget)
	}{
		{name: "non failed snapshot", mutateExpected: func(row *AssetModelReadiness) { row.Status = AssetModelReadinessStatusRetryWaiting }},
		{name: "non target error", mutateExpected: func(row *AssetModelReadiness) { row.ErrorClass = "source_unavailable" }},
		{name: "inactive new target", mutateTarget: func(target *AssetModelCoverageTarget) { target.Status = AssetModelTargetStatusUnavailable }},
		{name: "different target scope", mutateTarget: func(target *AssetModelCoverageTarget) { target.ScopeKey = "scope-b" }},
		{name: "different target model", mutateTarget: func(target *AssetModelCoverageTarget) { target.ModelName = "seedance-2.5" }},
		{name: "non positive target generation", mutateTarget: func(target *AssetModelCoverageTarget) { target.Generation = 0 }},
		{name: "non positive target channel", mutateTarget: func(target *AssetModelCoverageTarget) { target.ChannelId = 0 }},
		{name: "empty target binding scope", mutateTarget: func(target *AssetModelCoverageTarget) { target.BindingScope = "" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			openAssetModelReadinessTestDB(t)

			expected := AssetModelReadiness{
				AssetId:          73,
				ScopeKey:         "scope-a",
				ModelName:        "seedance-2.0-pro",
				TargetGeneration: 3,
				ChannelId:        120,
				BindingScope:     "byteplus:v1:old",
				Status:           AssetModelReadinessStatusFailed,
				ErrorClass:       "target_unavailable",
				AttemptCount:     4,
				AttemptStartedAt: 101,
				NextRetryAt:      202,
				CreatedAt:        10,
				UpdatedAt:        20,
			}
			if tt.mutateExpected != nil {
				tt.mutateExpected(&expected)
			}
			require.NoError(t, DB.Create(&expected).Error)

			target := AssetModelCoverageTarget{
				ScopeKey:     "scope-a",
				ModelName:    "seedance-2.0-pro",
				Generation:   4,
				ChannelId:    121,
				BindingScope: "byteplus:v1:new",
				Status:       AssetModelTargetStatusActive,
			}
			if tt.mutateTarget != nil {
				tt.mutateTarget(&target)
			}

			reopened, err := ReopenFailedAssetModelReadinessForTargetCAS(expected, target, false, 400)
			require.NoError(t, err)
			require.False(t, reopened)
			persisted := requireOneReadiness(t, 73, "scope-a", "seedance-2.0-pro")
			require.Equal(t, int64(3), persisted.TargetGeneration)
		})
	}
}

func TestActivateAssetModelReadinessBindingSetCASActivatesExactCurrentBindingSet(t *testing.T) {
	openAssetModelReadinessTestDB(t)

	seedAssetModelBindingSetReadiness(t, 41, "scope-a", "driver", 11, 131, "binding-a", AssetModelReadinessStatusProcessing, func(row *AssetModelReadiness) {
		row.LeaseOwner = "worker-a"
		row.AttemptCount = 1
		row.LeaseExpiresAt = 200
		row.ErrorClass = "stale_error"
		row.NextRetryAt = 150
	})
	seedAssetModelBindingSetReadiness(t, 41, "scope-a", "pending-sibling", 11, 131, "binding-a", AssetModelReadinessStatusPending, func(row *AssetModelReadiness) {
		row.ErrorClass = "pending_error"
		row.NextRetryAt = 170
	})
	seedAssetModelBindingSetReadiness(t, 41, "scope-a", "retry-sibling", 11, 131, "binding-a", AssetModelReadinessStatusRetryWaiting, func(row *AssetModelReadiness) {
		row.ErrorClass = "retry_error"
		row.NextRetryAt = 180
	})
	seedAssetModelBindingSetReadiness(t, 41, "scope-a", "processing-sibling", 11, 131, "binding-a", AssetModelReadinessStatusProcessing, func(row *AssetModelReadiness) {
		row.LeaseOwner = "worker-b"
		row.AttemptCount = 2
		row.AttemptStartedAt = 90
		row.LeaseExpiresAt = 250
		row.ErrorClass = "processing_error"
		row.NextRetryAt = 190
	})
	seedAssetModelBindingSetReadiness(t, 41, "scope-a", "failed-sibling", 11, 131, "binding-a", AssetModelReadinessStatusFailed, func(row *AssetModelReadiness) {
		row.ErrorClass = "fatal"
	})
	seedAssetModelBindingSetReadiness(t, 41, "scope-a", "other-channel", 11, 132, "binding-a", AssetModelReadinessStatusPending, nil)
	seedAssetModelBindingSetReadiness(t, 41, "scope-a", "other-binding", 11, 131, "binding-b", AssetModelReadinessStatusPending, nil)
	seedAssetModelBindingSetReadiness(t, 41, "scope-a", "stale-generation", 11, 131, "binding-a", AssetModelReadinessStatusPending, nil)
	require.NoError(t, DB.Model(&AssetModelReadiness{}).
		Where("asset_id = ? AND scope_key = ? AND model_name = ?", 41, "scope-a", "stale-generation").
		UpdateColumn("target_generation", int64(10)).Error)
	seedAssetModelBindingSetReadiness(t, 41, "scope-b", "other-scope", 11, 131, "binding-a", AssetModelReadinessStatusPending, nil)
	seedAssetModelBindingSetReadiness(t, 42, "scope-a", "other-asset", 11, 131, "binding-a", AssetModelReadinessStatusPending, nil)

	count, err := ActivateAssetModelReadinessBindingSetCAS(AssetModelReadinessTransition{
		AssetId:                41,
		ScopeKey:               " scope-a ",
		ModelName:              " driver ",
		TargetGeneration:       11,
		ChannelId:              131,
		BindingScope:           "binding-a",
		LeaseOwner:             " worker-a ",
		ExpectedAttemptCount:   1,
		ExpectedLeaseExpiresAt: 200,
		Now:                    100,
	})
	require.NoError(t, err)
	require.Equal(t, int64(4), count)

	for _, modelName := range []string{"driver", "pending-sibling", "retry-sibling", "processing-sibling"} {
		row := requireOneReadiness(t, 41, "scope-a", modelName)
		require.Equal(t, AssetModelReadinessStatusActive, row.Status, modelName)
		require.Equal(t, "", row.ErrorClass, modelName)
		require.Equal(t, int64(0), row.NextRetryAt, modelName)
		require.Equal(t, "", row.LeaseOwner, modelName)
		require.Equal(t, int64(0), row.LeaseExpiresAt, modelName)
		require.Equal(t, int64(100), row.UpdatedAt, modelName)
	}
	processingSibling := requireOneReadiness(t, 41, "scope-a", "processing-sibling")
	require.Equal(t, 2, processingSibling.AttemptCount)
	require.Equal(t, int64(90), processingSibling.AttemptStartedAt)

	failed := requireOneReadiness(t, 41, "scope-a", "failed-sibling")
	require.Equal(t, AssetModelReadinessStatusFailed, failed.Status)
	require.Equal(t, "fatal", failed.ErrorClass)
	for _, control := range []struct {
		assetID   int64
		scopeKey  string
		modelName string
	}{
		{41, "scope-a", "other-channel"},
		{41, "scope-a", "other-binding"},
		{41, "scope-a", "stale-generation"},
		{41, "scope-b", "other-scope"},
		{42, "scope-a", "other-asset"},
	} {
		row := requireOneReadiness(t, control.assetID, control.scopeKey, control.modelName)
		require.Equal(t, AssetModelReadinessStatusPending, row.Status, control.modelName)
		require.Equal(t, int64(10), row.UpdatedAt, control.modelName)
	}
}

func TestActivateAssetModelReadinessBindingSetCASStaleDriverChangesNothing(t *testing.T) {
	tests := []struct {
		name                           string
		mutate                         func(*AssetModelReadinessTransition)
		targetHook                     func(*AssetModelCoverageTarget)
		driverHook                     func(*AssetModelReadiness)
		expectedDriverTargetGeneration int64
	}{
		{
			name: "lease owner mismatch",
			mutate: func(transition *AssetModelReadinessTransition) {
				transition.LeaseOwner = "worker-b"
			},
		},
		{
			name: "attempt mismatch",
			mutate: func(transition *AssetModelReadinessTransition) {
				transition.ExpectedAttemptCount = 2
			},
		},
		{
			name: "expected lease expiry mismatch",
			mutate: func(transition *AssetModelReadinessTransition) {
				transition.ExpectedLeaseExpiresAt = 201
			},
		},
		{
			name: "expired equal lease",
			mutate: func(transition *AssetModelReadinessTransition) {
				transition.Now = transition.ExpectedLeaseExpiresAt
			},
		},
		{
			name: "transition target generation mismatch",
			mutate: func(transition *AssetModelReadinessTransition) {
				transition.TargetGeneration = 10
			},
		},
		{
			name: "stored driver target generation mismatch",
			driverHook: func(row *AssetModelReadiness) {
				row.TargetGeneration = 10
			},
			expectedDriverTargetGeneration: 10,
		},
		{
			name: "current driving target not active",
			targetHook: func(target *AssetModelCoverageTarget) {
				target.Status = AssetModelTargetStatusRotating
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			openAssetModelReadinessTestDB(t)
			expectedDriverTargetGeneration := tt.expectedDriverTargetGeneration
			if expectedDriverTargetGeneration == 0 {
				expectedDriverTargetGeneration = 11
			}
			seedAssetModelBindingSetReadiness(t, 41, "scope-a", "driver", 11, 131, "binding-a", AssetModelReadinessStatusProcessing, func(row *AssetModelReadiness) {
				row.LeaseOwner = "worker-a"
				row.AttemptCount = 1
				row.LeaseExpiresAt = 200
				if tt.driverHook != nil {
					tt.driverHook(row)
				}
			}, tt.targetHook)
			seedAssetModelBindingSetReadiness(t, 41, "scope-a", "sibling", 11, 131, "binding-a", AssetModelReadinessStatusPending, nil)

			transition := AssetModelReadinessTransition{
				AssetId:                41,
				ScopeKey:               "scope-a",
				ModelName:              "driver",
				TargetGeneration:       11,
				ChannelId:              131,
				BindingScope:           "binding-a",
				LeaseOwner:             "worker-a",
				ExpectedAttemptCount:   1,
				ExpectedLeaseExpiresAt: 200,
				Now:                    100,
			}
			if tt.mutate != nil {
				tt.mutate(&transition)
			}

			count, err := ActivateAssetModelReadinessBindingSetCAS(transition)
			require.NoError(t, err)
			require.Equal(t, int64(0), count)

			driver := requireOneReadiness(t, 41, "scope-a", "driver")
			require.Equal(t, AssetModelReadinessStatusProcessing, driver.Status)
			require.Equal(t, "worker-a", driver.LeaseOwner)
			require.Equal(t, 1, driver.AttemptCount)
			require.Equal(t, int64(200), driver.LeaseExpiresAt)
			require.Equal(t, expectedDriverTargetGeneration, driver.TargetGeneration)
			sibling := requireOneReadiness(t, 41, "scope-a", "sibling")
			require.Equal(t, AssetModelReadinessStatusPending, sibling.Status)
			require.Equal(t, "", sibling.LeaseOwner)
			require.Equal(t, int64(0), sibling.LeaseExpiresAt)
		})
	}
}

func TestActivateAssetModelReadinessBindingSetCASInvalidInputNoOp(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*AssetModelReadinessTransition)
	}{
		{
			name: "asset id not positive",
			mutate: func(transition *AssetModelReadinessTransition) {
				transition.AssetId = 0
			},
		},
		{
			name: "blank scope after trim",
			mutate: func(transition *AssetModelReadinessTransition) {
				transition.ScopeKey = " \t "
			},
		},
		{
			name: "blank model after trim",
			mutate: func(transition *AssetModelReadinessTransition) {
				transition.ModelName = " \n "
			},
		},
		{
			name: "blank owner after trim",
			mutate: func(transition *AssetModelReadinessTransition) {
				transition.LeaseOwner = "   "
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			openAssetModelReadinessTestDB(t)
			seedAssetModelBindingSetReadiness(t, 41, "scope-a", "driver", 11, 131, "binding-a", AssetModelReadinessStatusProcessing, func(row *AssetModelReadiness) {
				row.LeaseOwner = "worker-a"
				row.AttemptCount = 1
				row.LeaseExpiresAt = 200
			})
			seedAssetModelBindingSetReadiness(t, 41, "scope-a", "sibling", 11, 131, "binding-a", AssetModelReadinessStatusPending, nil)

			transition := AssetModelReadinessTransition{
				AssetId:                41,
				ScopeKey:               "scope-a",
				ModelName:              "driver",
				TargetGeneration:       11,
				ChannelId:              131,
				BindingScope:           "binding-a",
				LeaseOwner:             "worker-a",
				ExpectedAttemptCount:   1,
				ExpectedLeaseExpiresAt: 200,
				Now:                    100,
			}
			tt.mutate(&transition)

			count, err := ActivateAssetModelReadinessBindingSetCAS(transition)
			require.NoError(t, err)
			require.Equal(t, int64(0), count)

			driver := requireOneReadiness(t, 41, "scope-a", "driver")
			require.Equal(t, AssetModelReadinessStatusProcessing, driver.Status)
			require.Equal(t, "worker-a", driver.LeaseOwner)
			require.Equal(t, 1, driver.AttemptCount)
			require.Equal(t, int64(200), driver.LeaseExpiresAt)
			sibling := requireOneReadiness(t, 41, "scope-a", "sibling")
			require.Equal(t, AssetModelReadinessStatusPending, sibling.Status)
			require.Equal(t, "", sibling.LeaseOwner)
			require.Equal(t, int64(0), sibling.LeaseExpiresAt)
		})
	}
}

func TestActivateAssetModelReadinessBindingSetCASRollsBackOnSiblingUpdateError(t *testing.T) {
	openAssetModelReadinessTestDB(t)
	seedAssetModelBindingSetReadiness(t, 41, "scope-a", "driver", 11, 131, "binding-a", AssetModelReadinessStatusProcessing, func(row *AssetModelReadiness) {
		row.LeaseOwner = "worker-a"
		row.AttemptCount = 1
		row.LeaseExpiresAt = 200
	})
	seedAssetModelBindingSetReadiness(t, 41, "scope-a", "sibling", 11, 131, "binding-a", AssetModelReadinessStatusPending, nil)

	sentinel := errors.New("sentinel sibling update")
	updateCount := 0
	callbackName := "asset_model_readiness_sibling_update_error"
	require.NoError(t, DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil || tx.Statement.Schema.Name != "AssetModelReadiness" {
			return
		}
		updateCount++
		if updateCount == 2 {
			tx.AddError(sentinel)
		}
	}))
	t.Cleanup(func() {
		require.NoError(t, DB.Callback().Update().Remove(callbackName))
	})

	count, err := ActivateAssetModelReadinessBindingSetCAS(AssetModelReadinessTransition{
		AssetId:                41,
		ScopeKey:               "scope-a",
		ModelName:              "driver",
		TargetGeneration:       11,
		ChannelId:              131,
		BindingScope:           "binding-a",
		LeaseOwner:             "worker-a",
		ExpectedAttemptCount:   1,
		ExpectedLeaseExpiresAt: 200,
		Now:                    100,
	})
	require.ErrorIs(t, err, sentinel)
	require.Equal(t, int64(0), count)

	driver := requireOneReadiness(t, 41, "scope-a", "driver")
	require.Equal(t, AssetModelReadinessStatusProcessing, driver.Status)
	require.Equal(t, "worker-a", driver.LeaseOwner)
	require.Equal(t, int64(200), driver.LeaseExpiresAt)
	sibling := requireOneReadiness(t, 41, "scope-a", "sibling")
	require.Equal(t, AssetModelReadinessStatusPending, sibling.Status)
	require.Equal(t, "", sibling.LeaseOwner)
}

func TestAssetModelReadinessSameOwnerReclaimRequiresFreshAttemptFence(t *testing.T) {
	openAssetModelReadinessTestDB(t)
	require.NoError(t, EnsureAssetModelReadiness(11, "scope", []string{"model"}, 1))
	row := requireOneReadiness(t, 11, "scope", "model")
	won, err := ClaimAssetModelReadinessLease(row.Id, "node-a", 10, 20)
	require.NoError(t, err)
	require.True(t, won)
	attemptOne := requireOneReadiness(t, 11, "scope", "model")

	won, err = ClaimAssetModelReadinessLease(row.Id, "node-a", 21, 60)
	require.NoError(t, err)
	require.True(t, won)
	attemptTwo := requireOneReadiness(t, 11, "scope", "model")
	require.Equal(t, 2, attemptTwo.AttemptCount)
	require.Equal(t, int64(60), attemptTwo.LeaseExpiresAt)

	staleTransition := AssetModelReadinessTransition{
		AssetId:                11,
		ScopeKey:               "scope",
		ModelName:              "model",
		LeaseOwner:             "node-a",
		ExpectedAttemptCount:   attemptOne.AttemptCount,
		ExpectedLeaseExpiresAt: attemptOne.LeaseExpiresAt,
		Now:                    22,
	}
	ok, err := ActivateAssetModelReadinessCAS(staleTransition)
	require.NoError(t, err)
	require.False(t, ok)

	freshTransition := staleTransition
	freshTransition.ExpectedAttemptCount = attemptTwo.AttemptCount
	freshTransition.ExpectedLeaseExpiresAt = attemptTwo.LeaseExpiresAt
	ok, err = ActivateAssetModelReadinessCAS(freshTransition)
	require.NoError(t, err)
	require.True(t, ok)

	require.NoError(t, DB.Model(&AssetModelReadiness{}).Where("id = ?", row.Id).Updates(map[string]any{
		"status":           AssetModelReadinessStatusProcessing,
		"lease_owner":      "node-a",
		"lease_expires_at": int64(90),
		"attempt_count":    3,
	}).Error)
	target := AssetModelCoverageTarget{
		ScopeKey:     "scope",
		ModelName:    "model",
		Generation:   1,
		ChannelId:    77,
		BindingScope: "tenant",
	}
	reset, err := ResetAssetModelReadinessForTargetCAS(row.Id, "node-a", attemptOne.AttemptCount, attemptOne.LeaseExpiresAt, target, 30)
	require.NoError(t, err)
	require.False(t, reset)
	mismatchedTarget := target
	mismatchedTarget.ScopeKey = "other-scope"
	reset, err = ResetAssetModelReadinessForTargetCAS(row.Id, "node-a", 3, 90, mismatchedTarget, 30)
	require.NoError(t, err)
	require.False(t, reset)
	reset, err = ResetAssetModelReadinessForTargetCAS(row.Id, "node-a", 3, 90, target, 30)
	require.NoError(t, err)
	require.True(t, reset)
}

func TestAssetModelTargetLeasePublishAndRotateCAS(t *testing.T) {
	openAssetModelReadinessTestDB(t)

	won, err := ClaimAssetModelTargetLease("scope", "model", "owner-a", 10, 100)
	require.NoError(t, err)
	require.True(t, won)
	target, err := GetAssetModelCoverageTarget("scope", "model")
	require.NoError(t, err)
	require.Equal(t, AssetModelTargetStatusSelecting, target.Status)
	require.Equal(t, int64(0), target.Generation)
	require.Equal(t, "owner-a", target.LeaseOwner)

	won, err = ClaimAssetModelTargetLease("scope", "model", "owner-b", 11, 120)
	require.NoError(t, err)
	require.False(t, won)
	won, err = ClaimAssetModelTargetLease("scope", "model", "owner-a", 12, 130)
	require.NoError(t, err)
	require.True(t, won)

	candidate := AssetModelCoverageTarget{
		RoutingGroups:     "vip,default",
		SpecificChannelId: 77,
		ChannelId:         88,
		MappedModel:       "mapped-model",
		BindingScope:      "tenant",
		CredentialIndex:   2,
		CandidateIndex:    5,
	}
	published, err := PublishAssetModelTargetCAS("scope", "model", "owner-b", 0, 130, candidate, 20)
	require.NoError(t, err)
	require.False(t, published)
	published, err = PublishAssetModelTargetCAS("scope", "model", "owner-a", 0, 130, candidate, 21)
	require.NoError(t, err)
	require.True(t, published)
	published, err = PublishAssetModelTargetCAS("scope", "model", "owner-a", 0, 130, candidate, 22)
	require.NoError(t, err)
	require.False(t, published)
	target, err = GetAssetModelCoverageTarget("scope", "model")
	require.NoError(t, err)
	require.Equal(t, AssetModelTargetStatusActive, target.Status)
	require.Equal(t, int64(1), target.Generation)
	require.Equal(t, 88, target.ChannelId)
	require.Equal(t, "", target.LeaseOwner)

	rotated, err := RotateAssetModelTargetCAS("scope", "model", 0, "ignored", 30)
	require.NoError(t, err)
	require.False(t, rotated)
	rotated, err = RotateAssetModelTargetCAS("scope", "model", 1, "ignored", 31)
	require.NoError(t, err)
	require.True(t, rotated)
	target, err = GetAssetModelCoverageTarget("scope", "model")
	require.NoError(t, err)
	require.Equal(t, AssetModelTargetStatusRotating, target.Status)
	require.Equal(t, int64(2), target.Generation)
	require.Equal(t, 0, target.ChannelId)
	require.Equal(t, "", target.MappedModel)
	require.Equal(t, -1, target.CandidateIndex)
	require.Equal(t, -1, target.CredentialIndex)
}

func TestAssetModelTargetPublishSameOwnerReclaimRequiresCurrentLeaseExpiry(t *testing.T) {
	openAssetModelReadinessTestDB(t)

	won, err := ClaimAssetModelTargetLease("scope", "model", "owner-a", 10, 100)
	require.NoError(t, err)
	require.True(t, won)
	won, err = ClaimAssetModelTargetLease("scope", "model", "owner-a", 11, 150)
	require.NoError(t, err)
	require.True(t, won)

	candidate := AssetModelCoverageTarget{ChannelId: 88, BindingScope: "tenant"}
	published, err := PublishAssetModelTargetCAS("scope", "model", "owner-a", 0, 100, candidate, 20)
	require.NoError(t, err)
	require.False(t, published)
	published, err = PublishAssetModelTargetCAS("scope", "model", "owner-a", 0, 150, candidate, 21)
	require.NoError(t, err)
	require.True(t, published)
	published, err = PublishAssetModelTargetCAS("scope", "model", "owner-a", 0, 150, candidate, 22)
	require.NoError(t, err)
	require.False(t, published)

	target, err := GetAssetModelCoverageTarget("scope", "model")
	require.NoError(t, err)
	require.Equal(t, int64(1), target.Generation)
}

func TestAssetModelTargetReleaseLeaseCASRequiresCurrentOwnerExpiryAndGeneration(t *testing.T) {
	openAssetModelReadinessTestDB(t)

	require.NoError(t, DB.Create(&AssetModelCoverageTarget{
		ScopeKey:       "scope",
		ModelName:      "model",
		Status:         AssetModelTargetStatusActive,
		Generation:     3,
		LeaseOwner:     "owner-a",
		LeaseExpiresAt: 100,
		ChannelId:      88,
		UpdatedAt:      10,
	}).Error)

	released, err := ReleaseAssetModelTargetLeaseCAS("scope", "model", "owner-b", 3, 100, 20)
	require.NoError(t, err)
	require.False(t, released)
	released, err = ReleaseAssetModelTargetLeaseCAS("scope", "model", "owner-a", 2, 100, 20)
	require.NoError(t, err)
	require.False(t, released)
	released, err = ReleaseAssetModelTargetLeaseCAS("scope", "model", "owner-a", 3, 90, 20)
	require.NoError(t, err)
	require.False(t, released)
	released, err = ReleaseAssetModelTargetLeaseCAS("scope", "model", "owner-a", 3, 100, 100)
	require.NoError(t, err)
	require.False(t, released)

	target, err := GetAssetModelCoverageTarget("scope", "model")
	require.NoError(t, err)
	require.Equal(t, "owner-a", target.LeaseOwner)
	require.Equal(t, int64(100), target.LeaseExpiresAt)

	released, err = ReleaseAssetModelTargetLeaseCAS("scope", "model", "owner-a", 3, 100, 20)
	require.NoError(t, err)
	require.True(t, released)
	target, err = GetAssetModelCoverageTarget("scope", "model")
	require.NoError(t, err)
	require.Equal(t, int64(3), target.Generation)
	require.Equal(t, AssetModelTargetStatusActive, target.Status)
	require.Equal(t, "", target.LeaseOwner)
	require.Equal(t, int64(0), target.LeaseExpiresAt)
	require.Equal(t, int64(20), target.UpdatedAt)
}

func TestAssetModelTargetReleaseLeaseCASRequiresActiveStatus(t *testing.T) {
	openAssetModelReadinessTestDB(t)

	require.NoError(t, DB.Create(&AssetModelCoverageTarget{
		ScopeKey:       "scope",
		ModelName:      "model",
		Status:         AssetModelTargetStatusSelecting,
		Generation:     3,
		LeaseOwner:     "owner-a",
		LeaseExpiresAt: 100,
		UpdatedAt:      10,
	}).Error)

	released, err := ReleaseAssetModelTargetLeaseCAS("scope", "model", "owner-a", 3, 100, 20)
	require.NoError(t, err)
	require.False(t, released)
	target, err := GetAssetModelCoverageTarget("scope", "model")
	require.NoError(t, err)
	require.Equal(t, "owner-a", target.LeaseOwner)
	require.Equal(t, int64(100), target.LeaseExpiresAt)
}

func TestAssetModelTargetRotateDoesNotStealLiveLease(t *testing.T) {
	openAssetModelReadinessTestDB(t)

	require.NoError(t, DB.Create(&AssetModelCoverageTarget{
		ScopeKey:       "scope",
		ModelName:      "model",
		Status:         AssetModelTargetStatusActive,
		Generation:     7,
		LeaseOwner:     "publisher",
		LeaseExpiresAt: 100,
		ChannelId:      88,
	}).Error)

	rotated, err := RotateAssetModelTargetCAS("scope", "model", 7, "ignored", 50)
	require.NoError(t, err)
	require.False(t, rotated)
	rotated, err = RotateAssetModelTargetCAS("scope", "model", 7, "ignored", 101)
	require.NoError(t, err)
	require.True(t, rotated)
}

func readinessModelNames(rows []AssetModelReadiness) []string {
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.ModelName)
	}
	return names
}

func requireOneReadiness(t *testing.T, assetID int64, scopeKey string, modelName string) AssetModelReadiness {
	t.Helper()

	rows, err := ListAssetModelReadiness(assetID, scopeKey, []string{modelName})
	require.NoError(t, err)
	require.Len(t, rows, 1, fmt.Sprintf("readiness row %d/%s/%s", assetID, scopeKey, modelName))
	return rows[0]
}

func seedAssetModelBindingSetReadiness(t *testing.T, assetID int64, scopeKey string, modelName string, generation int64, channelID int, bindingScope string, readinessStatus string, readinessHook func(*AssetModelReadiness), targetHooks ...func(*AssetModelCoverageTarget)) {
	t.Helper()

	target := AssetModelCoverageTarget{
		ScopeKey:     scopeKey,
		ModelName:    modelName,
		Generation:   generation,
		Status:       AssetModelTargetStatusActive,
		ChannelId:    channelID,
		BindingScope: bindingScope,
		CreatedAt:    10,
		UpdatedAt:    10,
	}
	for _, hook := range targetHooks {
		if hook != nil {
			hook(&target)
		}
	}
	require.NoError(t, DB.Create(&target).Error)

	readiness := AssetModelReadiness{
		AssetId:          assetID,
		ScopeKey:         scopeKey,
		ModelName:        modelName,
		TargetGeneration: generation,
		ChannelId:        channelID,
		BindingScope:     bindingScope,
		Status:           readinessStatus,
		CreatedAt:        10,
		UpdatedAt:        10,
	}
	if readinessHook != nil {
		readinessHook(&readiness)
	}
	require.NoError(t, DB.Create(&readiness).Error)
}
