package model

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	supplierHistoricalPublishedDayTableName     = "supplier_historical_published_days"
	supplierHistoricalPublishedDayDateIndexName = "ux_supplier_historical_published_day_date"
	supplierHistoricalPublishedDayDateColumn    = "date"
)

var supplierHistoricalPublishedDayIndexNamePattern = regexp.MustCompile(`^[A-Za-z0-9_$]+$`)

type supplierHistoricalIndexDefinition struct {
	Name          string
	Unique        bool
	Primary       bool
	Columns       []string
	HasPrefix     bool
	HasExpression bool
}

func (index supplierHistoricalIndexDefinition) isExactUniqueDateIndex() bool {
	return index.Unique &&
		!index.Primary &&
		!index.HasPrefix &&
		!index.HasExpression &&
		len(index.Columns) == 1 &&
		index.Columns[0] == supplierHistoricalPublishedDayDateColumn
}

type supplierHistoricalPublishedDayIndexRepairPlan struct {
	RenameFrom string
	Drop       []string
}

func planSupplierHistoricalPublishedDayDateIndexRepair(indexes []supplierHistoricalIndexDefinition) (supplierHistoricalPublishedDayIndexRepairPlan, error) {
	var plan supplierHistoricalPublishedDayIndexRepairPlan
	var hasCanonical bool
	var candidates []string

	for _, index := range indexes {
		isCanonical := strings.EqualFold(index.Name, supplierHistoricalPublishedDayDateIndexName)
		if isCanonical {
			if !index.isExactUniqueDateIndex() {
				return plan, fmt.Errorf("canonical index has unexpected definition: %s", index.Name)
			}
			hasCanonical = true
			continue
		}
		if !index.isExactUniqueDateIndex() {
			continue
		}
		if !supplierHistoricalPublishedDayIndexNamePattern.MatchString(index.Name) || len(index.Name) > 64 {
			return plan, fmt.Errorf("unsafe index name %q", index.Name)
		}
		candidates = append(candidates, index.Name)
	}

	sort.Strings(candidates)
	if len(candidates) == 0 {
		return plan, nil
	}
	if hasCanonical {
		plan.Drop = candidates
		return plan, nil
	}
	plan.RenameFrom = candidates[0]
	plan.Drop = candidates[1:]
	return plan, nil
}
