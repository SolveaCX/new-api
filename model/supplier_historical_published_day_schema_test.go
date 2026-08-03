package model

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func ptrString(value string) *string {
	return &value
}

type sqlRecorder struct {
	logger.Interface
	sql []string
}

func (recorder *sqlRecorder) LogMode(logger.LogLevel) logger.Interface {
	return recorder
}

func (recorder *sqlRecorder) Info(context.Context, string, ...interface{})  {}
func (recorder *sqlRecorder) Warn(context.Context, string, ...interface{})  {}
func (recorder *sqlRecorder) Error(context.Context, string, ...interface{}) {}

func (recorder *sqlRecorder) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, _ := fc()
	recorder.sql = append(recorder.sql, sql)
}

func TestMigrateSupplierHistoricalPublishedDaySchemaSQLiteIsIdempotentAndUnique(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	require.NoError(t, MigrateSupplierHistoricalPublishedDaySchema(db))
	require.NoError(t, MigrateSupplierHistoricalPublishedDaySchema(db))

	require.True(t, db.Migrator().HasTable(&SupplierHistoricalPublishedDay{}))
	require.NoError(t, db.Create(&SupplierHistoricalPublishedDay{
		Date:        "2026-08-03",
		DayStart:    1785686400,
		ImportId:    1,
		PublishedBy: 7,
		PublishedAt: 1785772800,
	}).Error)
	require.Error(t, db.Create(&SupplierHistoricalPublishedDay{
		Date:        "2026-08-03",
		DayStart:    1785686400,
		ImportId:    2,
		PublishedBy: 7,
		PublishedAt: 1785772801,
	}).Error)

	definition, found, err := supplierHistoricalPublishedDayCanonicalIndex(db)
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, definition.isExactUniqueDateIndex())
}

func TestMigrateSupplierHistoricalPublishedDaySchemaNilDBReturnsErrDatabase(t *testing.T) {
	err := MigrateSupplierHistoricalPublishedDaySchema(nil)

	require.ErrorIs(t, err, ErrDatabase)
}

func TestMigrateSupplierHistoricalPublishedDaySchemaMetadataNormalizationFailsClosed(t *testing.T) {
	tests := []struct {
		name    string
		rows    []supplierHistoricalIndexMetadataRow
		wantErr string
	}{
		{
			name: "inconsistent uniqueness",
			rows: []supplierHistoricalIndexMetadataRow{
				{IndexName: "idx_date", Unique: true, SeqInIndex: 1, ColumnName: ptrString(supplierHistoricalPublishedDayDateColumn)},
				{IndexName: "idx_date", Unique: false, SeqInIndex: 2, ColumnName: ptrString("import_id")},
			},
			wantErr: "inconsistent uniqueness",
		},
		{
			name: "non contiguous sequence",
			rows: []supplierHistoricalIndexMetadataRow{
				{IndexName: "idx_date", Unique: true, SeqInIndex: 1, ColumnName: ptrString(supplierHistoricalPublishedDayDateColumn)},
				{IndexName: "idx_date", Unique: true, SeqInIndex: 3, ColumnName: ptrString("import_id")},
			},
			wantErr: "non-contiguous index sequence",
		},
		{
			name: "inconsistent postgres state",
			rows: []supplierHistoricalIndexMetadataRow{
				{IndexName: "idx_date", Unique: true, SeqInIndex: 1, ColumnName: ptrString(supplierHistoricalPublishedDayDateColumn)},
				{IndexName: "idx_date", Unique: true, SeqInIndex: 2, ColumnName: ptrString("import_id"), Invalid: true},
			},
			wantErr: "inconsistent validity metadata",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := supplierHistoricalIndexDefinitionsFromRows(test.rows)

			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

func TestMigrateSupplierHistoricalPublishedDaySchemaMetadataNormalizationMarksPrefixAndExpression(t *testing.T) {
	subPart := int64(4)
	indexes, err := supplierHistoricalIndexDefinitionsFromRows([]supplierHistoricalIndexMetadataRow{
		{IndexName: "idx_prefix", Unique: true, SeqInIndex: 1, ColumnName: ptrString(supplierHistoricalPublishedDayDateColumn), SubPart: &subPart},
		{IndexName: "idx_expression", Unique: true, SeqInIndex: 1, ColumnName: nil},
	})

	require.NoError(t, err)
	require.Len(t, indexes, 2)
	require.True(t, indexes[0].HasPrefix)
	require.False(t, indexes[0].HasExpression)
	require.False(t, indexes[0].isExactUniqueDateIndex())
	require.False(t, indexes[1].HasPrefix)
	require.True(t, indexes[1].HasExpression)
	require.False(t, indexes[1].isExactUniqueDateIndex())
}

func TestMigrateSupplierHistoricalPublishedDaySchemaMetadataNormalizationRejectsPostgresUnsafeStates(t *testing.T) {
	tests := []struct {
		name  string
		patch func(*supplierHistoricalIndexMetadataRow)
	}{
		{name: "invalid", patch: func(row *supplierHistoricalIndexMetadataRow) { row.Invalid = true }},
		{name: "not ready", patch: func(row *supplierHistoricalIndexMetadataRow) { row.NotReady = true }},
		{name: "not live", patch: func(row *supplierHistoricalIndexMetadataRow) { row.NotLive = true }},
		{name: "not immediate", patch: func(row *supplierHistoricalIndexMetadataRow) { row.NotImmediate = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			row := supplierHistoricalIndexMetadataRow{
				IndexName:  supplierHistoricalPublishedDayDateIndexName,
				Unique:     true,
				SeqInIndex: 1,
				ColumnName: ptrString(supplierHistoricalPublishedDayDateColumn),
			}
			test.patch(&row)

			indexes, err := supplierHistoricalIndexDefinitionsFromRows([]supplierHistoricalIndexMetadataRow{row})

			require.NoError(t, err)
			require.Len(t, indexes, 1)
			require.False(t, indexes[0].isExactUniqueDateIndex())
		})
	}
}

func TestMigrateSupplierHistoricalPublishedDaySchemaMetadataNormalizationRejectsWrongShapes(t *testing.T) {
	tests := []struct {
		name string
		rows []supplierHistoricalIndexMetadataRow
	}{
		{
			name: "partial",
			rows: []supplierHistoricalIndexMetadataRow{{
				IndexName:  supplierHistoricalPublishedDayDateIndexName,
				Unique:     true,
				SeqInIndex: 1,
				ColumnName: ptrString(supplierHistoricalPublishedDayDateColumn),
				HasPartial: true,
			}},
		},
		{
			name: "expression",
			rows: []supplierHistoricalIndexMetadataRow{{
				IndexName:     supplierHistoricalPublishedDayDateIndexName,
				Unique:        true,
				SeqInIndex:    1,
				HasExpression: true,
			}},
		},
		{
			name: "composite",
			rows: []supplierHistoricalIndexMetadataRow{
				{IndexName: supplierHistoricalPublishedDayDateIndexName, Unique: true, SeqInIndex: 1, ColumnName: ptrString(supplierHistoricalPublishedDayDateColumn)},
				{IndexName: supplierHistoricalPublishedDayDateIndexName, Unique: true, SeqInIndex: 2, ColumnName: ptrString("import_id")},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			indexes, err := supplierHistoricalIndexDefinitionsFromRows(test.rows)

			require.NoError(t, err)
			require.Len(t, indexes, 1)
			require.False(t, indexes[0].isExactUniqueDateIndex())
		})
	}
}

func TestMigrateSupplierHistoricalPublishedDaySchemaMySQLRepairUsesQuotedDropBeforeRename(t *testing.T) {
	recorder := &sqlRecorder{}
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "gorm:gorm@tcp(localhost:9910)/gorm?charset=utf8&parseTime=True",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		DisableAutomaticPing: true,
		DryRun:               true,
		Logger:               recorder,
	})
	require.NoError(t, err)

	err = applySupplierHistoricalPublishedDayDateIndexRepair(db, supplierHistoricalPublishedDayIndexRepairPlan{
		RenameFrom: "date_dup_a",
		Drop:       []string{"date_dup_b"},
	})

	require.NoError(t, err)
	require.Equal(t, []string{
		"DROP INDEX `date_dup_b` ON `supplier_historical_published_days`",
		"ALTER TABLE `supplier_historical_published_days` RENAME INDEX `date_dup_a` TO `ux_supplier_historical_published_day_date`",
	}, recorder.sql)
}

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
