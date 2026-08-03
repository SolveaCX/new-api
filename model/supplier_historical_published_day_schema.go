package model

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	supplierHistoricalPublishedDayTableName     = "supplier_historical_published_days"
	supplierHistoricalPublishedDayDateIndexName = "ux_supplier_historical_published_day_date"
	supplierHistoricalPublishedDayDateColumn    = "date"
	supplierHistoricalPublishedDayMySQLLockName = "newapi:supplier_historical_published_days:date_unique"
	supplierHistoricalPublishedDayMySQLLockWait = 30
)

var supplierHistoricalPublishedDayIndexNamePattern = regexp.MustCompile(`^[A-Za-z0-9_$]+$`)

type supplierHistoricalIndexDefinition struct {
	Name          string
	Unique        bool
	Primary       bool
	Columns       []string
	HasPrefix     bool
	HasExpression bool
	HasPartial    bool
	Invalid       bool
	NotReady      bool
	NotLive       bool
	NotImmediate  bool
}

func (index supplierHistoricalIndexDefinition) isExactUniqueDateIndex() bool {
	return index.Unique &&
		!index.Primary &&
		!index.HasPrefix &&
		!index.HasExpression &&
		!index.HasPartial &&
		!index.Invalid &&
		!index.NotReady &&
		!index.NotLive &&
		!index.NotImmediate &&
		len(index.Columns) == 1 &&
		index.Columns[0] == supplierHistoricalPublishedDayDateColumn
}

type supplierHistoricalIndexMetadataRow struct {
	IndexName     string
	Unique        bool
	Primary       bool
	SeqInIndex    int
	ColumnName    *string
	SubPart       *int64
	HasExpression bool
	HasPartial    bool
	Invalid       bool
	NotReady      bool
	NotLive       bool
	NotImmediate  bool
}

func supplierHistoricalIndexDefinitionsFromRows(rows []supplierHistoricalIndexMetadataRow) ([]supplierHistoricalIndexDefinition, error) {
	type builder struct {
		def     supplierHistoricalIndexDefinition
		nextSeq int
	}
	byName := make(map[string]*builder)
	order := make([]string, 0)

	for _, row := range rows {
		if row.IndexName == "" {
			return nil, fmt.Errorf("index metadata has empty index name")
		}
		if row.SeqInIndex < 1 {
			return nil, fmt.Errorf("index %q has invalid sequence %d", row.IndexName, row.SeqInIndex)
		}
		current, ok := byName[row.IndexName]
		rowPrimary := row.Primary || strings.EqualFold(row.IndexName, "PRIMARY")
		if !ok {
			current = &builder{
				def: supplierHistoricalIndexDefinition{
					Name:         row.IndexName,
					Unique:       row.Unique,
					Primary:      rowPrimary,
					HasPartial:   row.HasPartial,
					Invalid:      row.Invalid,
					NotReady:     row.NotReady,
					NotLive:      row.NotLive,
					NotImmediate: row.NotImmediate,
				},
				nextSeq: 1,
			}
			byName[row.IndexName] = current
			order = append(order, row.IndexName)
		}
		if current.def.Unique != row.Unique {
			return nil, fmt.Errorf("index %q has inconsistent uniqueness metadata", row.IndexName)
		}
		if current.def.Primary != rowPrimary {
			return nil, fmt.Errorf("index %q has inconsistent primary metadata", row.IndexName)
		}
		if current.def.HasPartial != row.HasPartial {
			return nil, fmt.Errorf("index %q has inconsistent partial metadata", row.IndexName)
		}
		if current.def.Invalid != row.Invalid {
			return nil, fmt.Errorf("index %q has inconsistent validity metadata", row.IndexName)
		}
		if current.def.NotReady != row.NotReady {
			return nil, fmt.Errorf("index %q has inconsistent readiness metadata", row.IndexName)
		}
		if current.def.NotLive != row.NotLive {
			return nil, fmt.Errorf("index %q has inconsistent liveness metadata", row.IndexName)
		}
		if current.def.NotImmediate != row.NotImmediate {
			return nil, fmt.Errorf("index %q has inconsistent immediate uniqueness metadata", row.IndexName)
		}
		if row.SeqInIndex != current.nextSeq {
			return nil, fmt.Errorf("index %q has non-contiguous index sequence: got %d want %d", row.IndexName, row.SeqInIndex, current.nextSeq)
		}
		current.nextSeq++
		if row.ColumnName == nil || row.HasExpression {
			current.def.HasExpression = true
			continue
		}
		if row.SubPart != nil {
			current.def.HasPrefix = true
		}
		current.def.Columns = append(current.def.Columns, *row.ColumnName)
	}

	indexes := make([]supplierHistoricalIndexDefinition, 0, len(order))
	for _, name := range order {
		indexes = append(indexes, byName[name].def)
	}
	return indexes, nil
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
				return plan, fmt.Errorf("canonical index has unexpected definition: %q", index.Name)
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

func MigrateSupplierHistoricalPublishedDaySchema(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate supplier historical published day schema: %w", ErrDatabase)
	}

	switch db.Dialector.Name() {
	case "mysql":
		if err := migrateSupplierHistoricalPublishedDaySchemaMySQL(db); err != nil {
			return fmt.Errorf("migrate supplier historical published day schema on mysql: %w", err)
		}
	case "sqlite", "postgres":
		if err := db.AutoMigrate(&SupplierHistoricalPublishedDay{}); err != nil {
			return fmt.Errorf("auto migrate supplier historical published days: %w", err)
		}
		if err := ensureSupplierHistoricalPublishedDayDateIndex(db); err != nil {
			return fmt.Errorf("ensure supplier historical published day date index: %w", err)
		}
	default:
		return fmt.Errorf("migrate supplier historical published day schema: unsupported database dialect %q", db.Dialector.Name())
	}
	return nil
}

func migrateSupplierHistoricalPublishedDaySchemaMySQL(db *gorm.DB) error {
	return db.Connection(func(tx *gorm.DB) (err error) {
		var locked sql.NullInt64
		if lockErr := tx.Raw("SELECT GET_LOCK(?, ?)", supplierHistoricalPublishedDayMySQLLockName, supplierHistoricalPublishedDayMySQLLockWait).Scan(&locked).Error; lockErr != nil {
			return fmt.Errorf("acquire supplier historical published day schema lock: %w", lockErr)
		}
		if !locked.Valid || locked.Int64 != 1 {
			return fmt.Errorf("acquire supplier historical published day schema lock: lock not acquired")
		}
		defer func() {
			var released sql.NullInt64
			releaseErr := tx.Raw("SELECT RELEASE_LOCK(?)", supplierHistoricalPublishedDayMySQLLockName).Scan(&released).Error
			if releaseErr == nil && (!released.Valid || released.Int64 != 1) {
				releaseErr = fmt.Errorf("release supplier historical published day schema lock: lock was not released")
			}
			if releaseErr != nil {
				err = errors.Join(err, releaseErr)
			}
		}()

		if tx.Migrator().HasTable(&SupplierHistoricalPublishedDay{}) {
			indexes, loadErr := supplierHistoricalPublishedDayAllIndexes(tx)
			if loadErr != nil {
				return fmt.Errorf("load supplier historical published day indexes before repair: %w", loadErr)
			}
			plan, planErr := planSupplierHistoricalPublishedDayDateIndexRepair(indexes)
			if planErr != nil {
				return fmt.Errorf("plan supplier historical published day index repair: %w", planErr)
			}
			if applyErr := applySupplierHistoricalPublishedDayDateIndexRepair(tx, plan); applyErr != nil {
				return fmt.Errorf("apply supplier historical published day index repair: %w", applyErr)
			}
			repairedIndexes, reloadErr := supplierHistoricalPublishedDayAllIndexes(tx)
			if reloadErr != nil {
				return fmt.Errorf("reload supplier historical published day indexes after repair: %w", reloadErr)
			}
			repairedPlan, repairedPlanErr := planSupplierHistoricalPublishedDayDateIndexRepair(repairedIndexes)
			if repairedPlanErr != nil {
				return fmt.Errorf("validate supplier historical published day index repair: %w", repairedPlanErr)
			}
			if repairedPlan.RenameFrom != "" || len(repairedPlan.Drop) > 0 {
				return fmt.Errorf("validate supplier historical published day index repair: repair plan remains")
			}
		}

		if migrateErr := tx.AutoMigrate(&SupplierHistoricalPublishedDay{}); migrateErr != nil {
			return fmt.Errorf("auto migrate supplier historical published days: %w", migrateErr)
		}
		if ensureErr := ensureSupplierHistoricalPublishedDayDateIndex(tx); ensureErr != nil {
			return fmt.Errorf("ensure supplier historical published day date index: %w", ensureErr)
		}
		finalIndexes, loadErr := supplierHistoricalPublishedDayAllIndexes(tx)
		if loadErr != nil {
			return fmt.Errorf("load supplier historical published day indexes after migration: %w", loadErr)
		}
		finalPlan, planErr := planSupplierHistoricalPublishedDayDateIndexRepair(finalIndexes)
		if planErr != nil {
			return fmt.Errorf("validate final supplier historical published day indexes: %w", planErr)
		}
		if finalPlan.RenameFrom != "" || len(finalPlan.Drop) > 0 {
			return fmt.Errorf("validate final supplier historical published day indexes: repair plan remains")
		}
		return nil
	})
}

func applySupplierHistoricalPublishedDayDateIndexRepair(db *gorm.DB, plan supplierHistoricalPublishedDayIndexRepairPlan) error {
	for _, indexName := range plan.Drop {
		if err := db.Exec(
			"DROP INDEX ? ON ?",
			clause.Column{Name: indexName},
			clause.Table{Name: supplierHistoricalPublishedDayTableName},
		).Error; err != nil {
			return fmt.Errorf("drop duplicate supplier historical published day date index %q: %w", indexName, err)
		}
	}
	if plan.RenameFrom != "" {
		if err := db.Exec(
			"ALTER TABLE ? RENAME INDEX ? TO ?",
			clause.Table{Name: supplierHistoricalPublishedDayTableName},
			clause.Column{Name: plan.RenameFrom},
			clause.Column{Name: supplierHistoricalPublishedDayDateIndexName},
		).Error; err != nil {
			return fmt.Errorf("rename supplier historical published day date index %q: %w", plan.RenameFrom, err)
		}
	}
	return nil
}

func ensureSupplierHistoricalPublishedDayDateIndex(db *gorm.DB) error {
	definition, found, err := supplierHistoricalPublishedDayCanonicalIndex(db)
	if err != nil {
		return fmt.Errorf("load canonical supplier historical published day date index: %w", err)
	}
	if found {
		if !definition.isExactUniqueDateIndex() {
			return fmt.Errorf("canonical supplier historical published day date index has unexpected definition")
		}
		return nil
	}
	if err := createSupplierHistoricalPublishedDayDateIndex(db); err != nil {
		return fmt.Errorf("create canonical supplier historical published day date index: %w", err)
	}

	definition, found, err = supplierHistoricalPublishedDayCanonicalIndex(db)
	if err != nil {
		return fmt.Errorf("reload canonical supplier historical published day date index: %w", err)
	}
	if !found {
		return fmt.Errorf("canonical supplier historical published day date index was not created")
	}
	if !definition.isExactUniqueDateIndex() {
		return fmt.Errorf("canonical supplier historical published day date index has unexpected definition after create")
	}
	return nil
}

func createSupplierHistoricalPublishedDayDateIndex(db *gorm.DB) error {
	createStatement := "CREATE UNIQUE INDEX ? ON ? (?)"
	switch db.Dialector.Name() {
	case "sqlite", "postgres":
		createStatement = "CREATE UNIQUE INDEX IF NOT EXISTS ? ON ? (?)"
	case "mysql":
	default:
		return fmt.Errorf("unsupported database dialect %q", db.Dialector.Name())
	}
	return db.Exec(
		createStatement,
		clause.Column{Name: supplierHistoricalPublishedDayDateIndexName},
		clause.Table{Name: supplierHistoricalPublishedDayTableName},
		clause.Column{Name: supplierHistoricalPublishedDayDateColumn},
	).Error
}

func supplierHistoricalPublishedDayCanonicalIndex(db *gorm.DB) (supplierHistoricalIndexDefinition, bool, error) {
	if db == nil {
		return supplierHistoricalIndexDefinition{}, false, fmt.Errorf("load supplier historical published day canonical index: %w", ErrDatabase)
	}
	switch db.Dialector.Name() {
	case "mysql":
		indexes, err := supplierHistoricalPublishedDayAllIndexes(db)
		if err != nil {
			return supplierHistoricalIndexDefinition{}, false, err
		}
		for _, index := range indexes {
			if strings.EqualFold(index.Name, supplierHistoricalPublishedDayDateIndexName) {
				return index, true, nil
			}
		}
		return supplierHistoricalIndexDefinition{}, false, nil
	case "sqlite":
		return supplierHistoricalPublishedDayCanonicalSQLiteIndex(db)
	case "postgres":
		indexes, err := supplierHistoricalPublishedDayPostgresIndexes(db)
		if err != nil {
			return supplierHistoricalIndexDefinition{}, false, err
		}
		if len(indexes) == 0 {
			return supplierHistoricalIndexDefinition{}, false, nil
		}
		return indexes[0], true, nil
	default:
		return supplierHistoricalIndexDefinition{}, false, fmt.Errorf("unsupported database dialect %q", db.Dialector.Name())
	}
}

func supplierHistoricalPublishedDayAllIndexes(db *gorm.DB) ([]supplierHistoricalIndexDefinition, error) {
	switch db.Dialector.Name() {
	case "mysql":
		return supplierHistoricalPublishedDayMySQLIndexes(db)
	default:
		return nil, fmt.Errorf("unsupported database dialect %q", db.Dialector.Name())
	}
}

func supplierHistoricalPublishedDayMySQLIndexes(db *gorm.DB) ([]supplierHistoricalIndexDefinition, error) {
	rows, err := db.Raw(`
SELECT index_name, non_unique, seq_in_index, column_name, sub_part
FROM information_schema.statistics
WHERE table_schema = DATABASE() AND table_name = ?
ORDER BY index_name, seq_in_index
`, supplierHistoricalPublishedDayTableName).Rows()
	if err != nil {
		return nil, fmt.Errorf("query mysql supplier historical published day index metadata: %w", err)
	}
	defer rows.Close()

	metadataRows := make([]supplierHistoricalIndexMetadataRow, 0)
	for rows.Next() {
		var indexName string
		var nonUnique int
		var seqInIndex int
		var columnName sql.NullString
		var subPart sql.NullInt64
		if err := rows.Scan(&indexName, &nonUnique, &seqInIndex, &columnName, &subPart); err != nil {
			return nil, fmt.Errorf("scan mysql supplier historical published day index metadata: %w", err)
		}
		var columnNamePtr *string
		if columnName.Valid {
			column := columnName.String
			columnNamePtr = &column
		}
		var subPartPtr *int64
		if subPart.Valid {
			value := subPart.Int64
			subPartPtr = &value
		}
		metadataRows = append(metadataRows, supplierHistoricalIndexMetadataRow{
			IndexName:  indexName,
			Unique:     nonUnique == 0,
			Primary:    strings.EqualFold(indexName, "PRIMARY"),
			SeqInIndex: seqInIndex,
			ColumnName: columnNamePtr,
			SubPart:    subPartPtr,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read mysql supplier historical published day index metadata: %w", err)
	}
	indexes, err := supplierHistoricalIndexDefinitionsFromRows(metadataRows)
	if err != nil {
		return nil, fmt.Errorf("normalize mysql supplier historical published day index metadata: %w", err)
	}
	return indexes, nil
}

func supplierHistoricalPublishedDayCanonicalSQLiteIndex(db *gorm.DB) (supplierHistoricalIndexDefinition, bool, error) {
	type sqliteIndexListRow struct {
		Name    string `gorm:"column:name"`
		Unique  int    `gorm:"column:unique"`
		Partial int    `gorm:"column:partial"`
	}
	var indexRows []sqliteIndexListRow
	if err := db.Raw("PRAGMA index_list('supplier_historical_published_days')").Scan(&indexRows).Error; err != nil {
		return supplierHistoricalIndexDefinition{}, false, fmt.Errorf("query sqlite supplier historical published day index list: %w", err)
	}
	var canonical *sqliteIndexListRow
	for index := range indexRows {
		if strings.EqualFold(indexRows[index].Name, supplierHistoricalPublishedDayDateIndexName) {
			canonical = &indexRows[index]
			break
		}
	}
	if canonical == nil {
		return supplierHistoricalIndexDefinition{}, false, nil
	}

	type sqliteIndexInfoRow struct {
		SeqNo int    `gorm:"column:seqno"`
		CID   int    `gorm:"column:cid"`
		Name  string `gorm:"column:name"`
	}
	var infoRows []sqliteIndexInfoRow
	if err := db.Raw("PRAGMA index_info('ux_supplier_historical_published_day_date')").Scan(&infoRows).Error; err != nil {
		return supplierHistoricalIndexDefinition{}, false, fmt.Errorf("query sqlite supplier historical published day canonical index info: %w", err)
	}
	sort.Slice(infoRows, func(i, j int) bool {
		return infoRows[i].SeqNo < infoRows[j].SeqNo
	})
	definition := supplierHistoricalIndexDefinition{
		Name:       canonical.Name,
		Unique:     canonical.Unique == 1,
		HasPartial: canonical.Partial != 0,
	}
	for _, row := range infoRows {
		if row.CID < 0 || row.Name == "" {
			definition.HasExpression = true
			continue
		}
		definition.Columns = append(definition.Columns, row.Name)
	}
	return definition, true, nil
}

func supplierHistoricalPublishedDayPostgresIndexes(db *gorm.DB) ([]supplierHistoricalIndexDefinition, error) {
	rows, err := db.Raw(`
SELECT idx.relname AS index_name,
       pg_index.indisunique AS is_unique,
       pg_index.indisprimary AS is_primary,
       key.ordinality::int AS seq_in_index,
       attribute.attname AS column_name,
       pg_index.indpred IS NOT NULL AS has_partial,
       pg_index.indexprs IS NOT NULL OR key.attnum = 0 AS has_expression,
       pg_index.indisvalid AS is_valid,
       pg_index.indisready AS is_ready,
       pg_index.indislive AS is_live,
       pg_index.indimmediate AS is_immediate
FROM pg_class table_class
JOIN pg_namespace namespace ON namespace.oid = table_class.relnamespace
JOIN pg_index ON pg_index.indrelid = table_class.oid
JOIN pg_class idx ON idx.oid = pg_index.indexrelid
LEFT JOIN LATERAL unnest(pg_index.indkey) WITH ORDINALITY AS key(attnum, ordinality) ON true
LEFT JOIN pg_attribute attribute ON attribute.attrelid = table_class.oid AND attribute.attnum = key.attnum
WHERE namespace.nspname = current_schema()
  AND table_class.relname = ?
  AND idx.relname = ?
ORDER BY idx.relname, key.ordinality
`, supplierHistoricalPublishedDayTableName, supplierHistoricalPublishedDayDateIndexName).Rows()
	if err != nil {
		return nil, fmt.Errorf("query postgres supplier historical published day index metadata: %w", err)
	}
	defer rows.Close()

	metadataRows := make([]supplierHistoricalIndexMetadataRow, 0)
	for rows.Next() {
		var indexName string
		var unique bool
		var primary bool
		var seqInIndex sql.NullInt64
		var columnName sql.NullString
		var hasPartial bool
		var hasExpression bool
		var valid bool
		var ready bool
		var live bool
		var immediate bool
		if err := rows.Scan(&indexName, &unique, &primary, &seqInIndex, &columnName, &hasPartial, &hasExpression, &valid, &ready, &live, &immediate); err != nil {
			return nil, fmt.Errorf("scan postgres supplier historical published day index metadata: %w", err)
		}
		var columnNamePtr *string
		if columnName.Valid {
			column := columnName.String
			columnNamePtr = &column
		}
		seq := 1
		if seqInIndex.Valid {
			seq = int(seqInIndex.Int64)
		}
		metadataRows = append(metadataRows, supplierHistoricalIndexMetadataRow{
			IndexName:     indexName,
			Unique:        unique,
			Primary:       primary,
			SeqInIndex:    seq,
			ColumnName:    columnNamePtr,
			HasPartial:    hasPartial,
			HasExpression: hasExpression,
			Invalid:       !valid,
			NotReady:      !ready,
			NotLive:       !live,
			NotImmediate:  !immediate,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read postgres supplier historical published day index metadata: %w", err)
	}
	indexes, err := supplierHistoricalIndexDefinitionsFromRows(metadataRows)
	if err != nil {
		return nil, fmt.Errorf("normalize postgres supplier historical published day index metadata: %w", err)
	}
	return indexes, nil
}
