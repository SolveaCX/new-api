package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlanSupplierHistoricalPublishedDayDateIndexRepairKeepsCanonicalAndDropsExactDuplicates(t *testing.T) {
	plan, err := planSupplierHistoricalPublishedDayDateIndexRepair([]supplierHistoricalIndexDefinition{
		{Name: "PRIMARY", Unique: true, Primary: true, Columns: []string{supplierHistoricalPublishedDayDateColumn}},
		{Name: "daily_nonunique", Unique: false, Columns: []string{supplierHistoricalPublishedDayDateColumn}},
		{Name: "daily_composite", Unique: true, Columns: []string{supplierHistoricalPublishedDayDateColumn, "import_id"}},
		{Name: "daily_other", Unique: true, Columns: []string{"import_id"}},
		{Name: "daily_prefix", Unique: true, Columns: []string{supplierHistoricalPublishedDayDateColumn}, HasPrefix: true},
		{Name: "daily_expression", Unique: true, Columns: []string{supplierHistoricalPublishedDayDateColumn}, HasExpression: true},
		{Name: "date_dup_b", Unique: true, Columns: []string{supplierHistoricalPublishedDayDateColumn}},
		{Name: supplierHistoricalPublishedDayDateIndexName, Unique: true, Columns: []string{supplierHistoricalPublishedDayDateColumn}},
		{Name: "date_dup_a", Unique: true, Columns: []string{supplierHistoricalPublishedDayDateColumn}},
	})

	require.NoError(t, err)
	require.Equal(t, supplierHistoricalPublishedDayIndexRepairPlan{
		Drop: []string{"date_dup_a", "date_dup_b"},
	}, plan)
}

func TestPlanSupplierHistoricalPublishedDayDateIndexRepairRenamesSmallestCandidateAndDropsRemaining(t *testing.T) {
	plan, err := planSupplierHistoricalPublishedDayDateIndexRepair([]supplierHistoricalIndexDefinition{
		{Name: "date_9", Unique: true, Columns: []string{supplierHistoricalPublishedDayDateColumn}},
		{Name: "date_2", Unique: true, Columns: []string{supplierHistoricalPublishedDayDateColumn}},
		{Name: "date_3", Unique: true, Columns: []string{supplierHistoricalPublishedDayDateColumn}},
	})

	require.NoError(t, err)
	require.Equal(t, supplierHistoricalPublishedDayIndexRepairPlan{
		RenameFrom: "date_2",
		Drop:       []string{"date_3", "date_9"},
	}, plan)
}

func TestPlanSupplierHistoricalPublishedDayDateIndexRepairCanonicalOnlyIsIdempotent(t *testing.T) {
	plan, err := planSupplierHistoricalPublishedDayDateIndexRepair([]supplierHistoricalIndexDefinition{
		{Name: supplierHistoricalPublishedDayDateIndexName, Unique: true, Columns: []string{supplierHistoricalPublishedDayDateColumn}},
	})

	require.NoError(t, err)
	require.Empty(t, plan.RenameFrom)
	require.Empty(t, plan.Drop)
}

func TestPlanSupplierHistoricalPublishedDayDateIndexRepairUnsafeExactCandidateNameErrors(t *testing.T) {
	_, err := planSupplierHistoricalPublishedDayDateIndexRepair([]supplierHistoricalIndexDefinition{
		{Name: "bad-name", Unique: true, Columns: []string{supplierHistoricalPublishedDayDateColumn}},
	})

	require.ErrorContains(t, err, "unsafe index name")
}

func TestPlanSupplierHistoricalPublishedDayDateIndexRepairEmptyExactCandidateNameQuotesUnsafeName(t *testing.T) {
	_, err := planSupplierHistoricalPublishedDayDateIndexRepair([]supplierHistoricalIndexDefinition{
		{Name: "", Unique: true, Columns: []string{supplierHistoricalPublishedDayDateColumn}},
	})

	require.ErrorContains(t, err, `unsafe index name ""`)
}

func TestPlanSupplierHistoricalPublishedDayDateIndexRepairOverlongExactCandidateNameErrors(t *testing.T) {
	_, err := planSupplierHistoricalPublishedDayDateIndexRepair([]supplierHistoricalIndexDefinition{
		{Name: strings.Repeat("a", 65), Unique: true, Columns: []string{supplierHistoricalPublishedDayDateColumn}},
	})

	require.ErrorContains(t, err, "unsafe index name")
}

func TestPlanSupplierHistoricalPublishedDayDateIndexRepairCanonicalWrongShapeErrors(t *testing.T) {
	_, err := planSupplierHistoricalPublishedDayDateIndexRepair([]supplierHistoricalIndexDefinition{
		{Name: "UX_SUPPLIER_HISTORICAL_PUBLISHED_DAY_DATE", Unique: true, Columns: []string{supplierHistoricalPublishedDayDateColumn, "import_id"}},
	})

	require.ErrorContains(t, err, "canonical index has unexpected definition")
}
