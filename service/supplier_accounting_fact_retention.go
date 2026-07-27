package service

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
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
	raw := strings.TrimSpace(os.Getenv("SUPPLIER_ACCOUNTING_FACT_RETENTION_DAYS"))
	if raw == "" {
		return 0, false, nil
	}
	days, err := strconv.Atoi(raw)
	if err != nil || days < 0 {
		return 0, false, fmt.Errorf("invalid SUPPLIER_ACCOUNTING_FACT_RETENTION_DAYS %q: must be a non-negative integer", raw)
	}
	return days, days > 0, nil
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
