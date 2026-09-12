package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	SubscriptionCatalogMigrationBatchStatusApplying       = "applying"
	SubscriptionCatalogMigrationBatchStatusScheduled      = "scheduled"
	SubscriptionCatalogMigrationBatchStatusPartial        = "partial"
	SubscriptionCatalogMigrationBatchStatusApplied        = "applied"
	SubscriptionCatalogMigrationBatchStatusCancelled      = "cancelled"
	SubscriptionCatalogMigrationBatchStatusNeedsAttention = "needs_attention"
)

var ErrSubscriptionCatalogMigrationBatchImmutable = errors.New("subscription catalog migration batch manifest is immutable")

// SubscriptionCatalogMigrationBatch is the immutable audit envelope for one
// explicitly selected catalog-migration cohort. ManifestSnapshot is write-once;
// only the operational status and aggregate summary may change after creation.
type SubscriptionCatalogMigrationBatch struct {
	Id string `json:"id" gorm:"type:varchar(64);primaryKey"`

	RequestId        string `json:"request_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_subscription_catalog_migration_request"`
	CohortDigest     string `json:"cohort_digest" gorm:"type:char(64);not null;uniqueIndex:ux_subscription_catalog_migration_digest"`
	Status           string `json:"status" gorm:"type:varchar(32);not null;default:'applying';index"`
	ManifestSnapshot string `json:"-" gorm:"type:longtext;not null;<-:create"`
	SummarySnapshot  string `json:"-" gorm:"type:longtext"`
	RequestedBy      int    `json:"requested_by" gorm:"not null;index"`

	// These immutable facts make it possible to prove that a stored batch was
	// prepared by the staging-only implementation. Livemode must remain false.
	DeploymentEnvironment string `json:"deployment_environment" gorm:"type:varchar(32);not null;<-:create"`
	ServiceName           string `json:"service_name" gorm:"type:varchar(128);not null;<-:create"`
	SandboxOnly           bool   `json:"sandbox_only" gorm:"not null;default:true;<-:create"`
	Livemode              bool   `json:"livemode" gorm:"not null;default:false;<-:create"`

	CreatedAt int64 `json:"created_at" gorm:"type:bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"type:bigint"`
}

func (b *SubscriptionCatalogMigrationBatch) BeforeCreate(tx *gorm.DB) error {
	if b == nil {
		return errors.New("subscription catalog migration batch is nil")
	}
	if b.Livemode {
		return errors.New("live-mode catalog migration batches are not supported")
	}
	b.Id = strings.TrimSpace(b.Id)
	if b.Id == "" {
		b.Id = "catmig_" + uuid.NewString()
	}
	b.RequestId = strings.TrimSpace(b.RequestId)
	b.CohortDigest = strings.ToLower(strings.TrimSpace(b.CohortDigest))
	b.Status = normalizeSubscriptionCatalogMigrationBatchStatus(b.Status)
	b.DeploymentEnvironment = strings.TrimSpace(b.DeploymentEnvironment)
	b.ServiceName = strings.TrimSpace(b.ServiceName)
	b.SandboxOnly = true
	now := common.GetTimestamp()
	b.CreatedAt = now
	b.UpdatedAt = now
	return nil
}

func (b *SubscriptionCatalogMigrationBatch) BeforeUpdate(tx *gorm.DB) error {
	if tx.Statement.Changed(
		"Id",
		"RequestId",
		"CohortDigest",
		"ManifestSnapshot",
		"RequestedBy",
		"DeploymentEnvironment",
		"ServiceName",
		"SandboxOnly",
		"Livemode",
	) {
		return ErrSubscriptionCatalogMigrationBatchImmutable
	}
	b.Status = normalizeSubscriptionCatalogMigrationBatchStatus(b.Status)
	b.UpdatedAt = common.GetTimestamp()
	return nil
}

func normalizeSubscriptionCatalogMigrationBatchStatus(status string) string {
	status = strings.TrimSpace(status)
	switch status {
	case SubscriptionCatalogMigrationBatchStatusScheduled,
		SubscriptionCatalogMigrationBatchStatusPartial,
		SubscriptionCatalogMigrationBatchStatusApplied,
		SubscriptionCatalogMigrationBatchStatusCancelled,
		SubscriptionCatalogMigrationBatchStatusNeedsAttention:
		return status
	default:
		return SubscriptionCatalogMigrationBatchStatusApplying
	}
}
