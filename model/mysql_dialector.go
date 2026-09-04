package model

import (
	"reflect"
	"strconv"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// newMySQLDialector builds the gorm dialector used for every MySQL connection.
//
// It wraps the stock mysql driver so that AutoMigrate stops re-issuing
// `ALTER TABLE ... MODIFY COLUMN` for bool columns whose default is already
// correct. gorm v1.25.2 compares the stored default ("0"/"1") with the tag
// default ("false"/"true") as raw strings, so every startup re-alters every
// bool column with a default tag. On a busy production table that ALTER waits
// on the metadata lock and stalls all traffic. Upstream fixed it in gorm
// v1.25.9 (commit 1b0aa802); this shim carries the same comparison until the
// dependency can be upgraded.
func newMySQLDialector(dsn string) gorm.Dialector {
	return &boolDefaultAwareMySQLDialector{Dialector: mysql.Open(dsn).(*mysql.Dialector)}
}

type boolDefaultAwareMySQLDialector struct {
	*mysql.Dialector
}

func (d *boolDefaultAwareMySQLDialector) Migrator(db *gorm.DB) gorm.Migrator {
	return boolDefaultAwareMySQLMigrator{Migrator: d.Dialector.Migrator(db).(mysql.Migrator)}
}

type boolDefaultAwareMySQLMigrator struct {
	mysql.Migrator
}

func (m boolDefaultAwareMySQLMigrator) MigrateColumn(value interface{}, field *schema.Field, columnType gorm.ColumnType) error {
	if boolDefaultMatches(field, columnType) {
		columnType = boolDefaultNormalizedColumnType{inner: columnType, defaultValue: field.DefaultValue}
	}
	return m.Migrator.MigrateColumn(value, field, columnType)
}

// boolDefaultMatches reports whether a bool field's tag default and the
// column's stored default describe the same value (e.g. "false" vs "0").
func boolDefaultMatches(field *schema.Field, columnType gorm.ColumnType) bool {
	if field == nil || field.PrimaryKey || field.GORMDataType != schema.Bool || !field.HasDefaultValue {
		return false
	}
	stored, ok := columnType.DefaultValue()
	if !ok {
		return false
	}
	storedBool, err := strconv.ParseBool(stored)
	if err != nil {
		return false
	}
	wantBool, err := strconv.ParseBool(field.DefaultValue)
	if err != nil {
		return false
	}
	return storedBool == wantBool
}

// boolDefaultNormalizedColumnType reports the default value in the same
// spelling as the model tag so the stock string comparison sees no drift.
type boolDefaultNormalizedColumnType struct {
	inner        gorm.ColumnType
	defaultValue string
}

func (c boolDefaultNormalizedColumnType) Name() string                { return c.inner.Name() }
func (c boolDefaultNormalizedColumnType) DatabaseTypeName() string    { return c.inner.DatabaseTypeName() }
func (c boolDefaultNormalizedColumnType) ColumnType() (string, bool)  { return c.inner.ColumnType() }
func (c boolDefaultNormalizedColumnType) PrimaryKey() (bool, bool)    { return c.inner.PrimaryKey() }
func (c boolDefaultNormalizedColumnType) AutoIncrement() (bool, bool) { return c.inner.AutoIncrement() }
func (c boolDefaultNormalizedColumnType) Length() (int64, bool)       { return c.inner.Length() }
func (c boolDefaultNormalizedColumnType) DecimalSize() (int64, int64, bool) {
	return c.inner.DecimalSize()
}
func (c boolDefaultNormalizedColumnType) Nullable() (bool, bool)       { return c.inner.Nullable() }
func (c boolDefaultNormalizedColumnType) Unique() (bool, bool)         { return c.inner.Unique() }
func (c boolDefaultNormalizedColumnType) ScanType() reflect.Type       { return c.inner.ScanType() }
func (c boolDefaultNormalizedColumnType) Comment() (string, bool)      { return c.inner.Comment() }
func (c boolDefaultNormalizedColumnType) DefaultValue() (string, bool) { return c.defaultValue, true }
