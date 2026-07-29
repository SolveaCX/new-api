package service

import (
	"context"
	"sync"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

var (
	supplierAccountingFactRetentionMu     sync.Mutex
	supplierAccountingFactRetentionCursor string
)

// The cursor only avoids rescanning already-empty days. All deletion safety is
// enforced by database predicates, so independent nodes and restarts are safe.
func configuredSupplierAccountingFactRetentionDays() (int, bool, error) {
	settings, err := model.GetSupplierAccountingRuntimeSettings()
	if err != nil {
		return 0, false, err
	}
	return settings.RetentionDays, settings.RetentionDays > 0, nil
}

func RunSupplierAccountingFactRetentionOnce(ctx context.Context, mainDB, logDB *gorm.DB) (int64, error) {
	retentionDays, enabled, err := configuredSupplierAccountingFactRetentionDays()
	if err != nil {
		return 0, err
	}
	supplierAccountingFactRetentionMu.Lock()
	defer supplierAccountingFactRetentionMu.Unlock()
	if !enabled {
		supplierAccountingFactRetentionCursor = ""
		return 0, nil
	}
	if mainDB == nil || logDB == nil {
		return 0, model.ErrDatabase
	}
	batch, err := model.SelectSupplierAccountingFactRetentionBatch(ctx, mainDB, retentionDays, supplierAccountingFactRetentionCursor)
	if err != nil {
		return 0, err
	}
	if batch == nil {
		supplierAccountingFactRetentionCursor = ""
		return 0, nil
	}
	result, err := model.DeleteSupplierAccountingFactRetentionChunk(ctx, logDB, batch.PreparedDay, batch.SourceMaxFactId)
	if err != nil {
		return 0, err
	}
	if result.Selected == 0 {
		supplierAccountingFactRetentionCursor = batch.PreparedDay
	}
	return result.Deleted, nil
}
