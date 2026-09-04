package model

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// fakeColumnType is a hand-rolled gorm.ColumnType so the shim's comparison can
// be exercised without a database.
type fakeColumnType struct {
	name         string
	dbType       string
	defaultValue string
	hasDefault   bool
	nullable     bool
}

func (c fakeColumnType) Name() string                      { return c.name }
func (c fakeColumnType) DatabaseTypeName() string          { return c.dbType }
func (c fakeColumnType) ColumnType() (string, bool)        { return c.dbType, true }
func (c fakeColumnType) PrimaryKey() (bool, bool)          { return false, true }
func (c fakeColumnType) AutoIncrement() (bool, bool)       { return false, true }
func (c fakeColumnType) Length() (int64, bool)             { return 0, false }
func (c fakeColumnType) DecimalSize() (int64, int64, bool) { return 0, 0, false }
func (c fakeColumnType) Nullable() (bool, bool)            { return c.nullable, true }
func (c fakeColumnType) Unique() (bool, bool)              { return false, true }
func (c fakeColumnType) ScanType() reflect.Type            { return reflect.TypeOf(int64(0)) }
func (c fakeColumnType) Comment() (string, bool)           { return "", false }
func (c fakeColumnType) DefaultValue() (string, bool)      { return c.defaultValue, c.hasDefault }

// shimProbeModel carries one field per case the shim has to distinguish.
type shimProbeModel struct {
	Id         int    `gorm:"primaryKey"`
	IsHoneypot bool   `gorm:"default:false"`
	Enabled    bool   `gorm:"default:true"`
	NoDefault  bool   ``
	Label      string `gorm:"type:varchar(32);default:placeholder"`
	Count      int    `gorm:"default:0"`
	PtrFlag    *bool  `gorm:"default:false"`
}

func (shimProbeModel) TableName() string { return "zz_shim_probe" }

func shimProbeField(t *testing.T, name string) *schema.Field {
	t.Helper()
	parsed, err := schema.Parse(&shimProbeModel{}, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)
	field := parsed.LookUpField(name)
	require.NotNil(t, field, "field %s not found on probe model", name)
	return field
}

func TestBoolDefaultMatchesTreatsZeroOneAsFalseTrue(t *testing.T) {
	cases := []struct {
		name   string
		field  string
		stored string
		has    bool
		want   bool
	}{
		{"tag false vs stored 0 is the same value", "IsHoneypot", "0", true, true},
		{"tag false vs stored false is the same value", "IsHoneypot", "false", true, true},
		{"tag false vs stored 1 is real drift", "IsHoneypot", "1", true, false},
		{"tag true vs stored 1 is the same value", "Enabled", "1", true, true},
		{"tag true vs stored 0 is real drift", "Enabled", "0", true, false},
		{"pointer bool is handled like a bool", "PtrFlag", "0", true, true},
		{"column without a stored default is left alone", "IsHoneypot", "", false, false},
		{"unparsable stored default is left alone", "IsHoneypot", "b'0'", true, false},
		{"field without a default tag is left alone", "NoDefault", "0", true, false},
		{"string column is left alone", "Label", "placeholder", true, false},
		{"int column is left alone", "Count", "0", true, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			field := shimProbeField(t, testCase.field)
			column := fakeColumnType{
				name:         field.DBName,
				dbType:       "tinyint",
				defaultValue: testCase.stored,
				hasDefault:   testCase.has,
			}
			require.Equal(t, testCase.want, boolDefaultMatches(field, column))
		})
	}
}

func TestBoolDefaultMatchesIgnoresNilFieldAndPrimaryKey(t *testing.T) {
	column := fakeColumnType{dbType: "tinyint", defaultValue: "0", hasDefault: true}
	require.False(t, boolDefaultMatches(nil, column))
	require.False(t, boolDefaultMatches(shimProbeField(t, "Id"), column))
}

func TestBoolDefaultNormalizedColumnTypeOnlyRewritesTheDefault(t *testing.T) {
	inner := fakeColumnType{
		name:         "is_honeypot",
		dbType:       "tinyint",
		defaultValue: "0",
		hasDefault:   true,
		nullable:     true,
	}
	wrapped := boolDefaultNormalizedColumnType{inner: inner, defaultValue: "false"}
	var _ gorm.ColumnType = wrapped

	rewritten, ok := wrapped.DefaultValue()
	require.True(t, ok)
	require.Equal(t, "false", rewritten)

	require.Equal(t, inner.Name(), wrapped.Name())
	require.Equal(t, inner.DatabaseTypeName(), wrapped.DatabaseTypeName())
	require.Equal(t, inner.ScanType(), wrapped.ScanType())

	columnType, ok := wrapped.ColumnType()
	require.True(t, ok)
	require.Equal(t, "tinyint", columnType)

	nullable, ok := wrapped.Nullable()
	require.True(t, ok)
	require.True(t, nullable)

	unique, ok := wrapped.Unique()
	require.True(t, ok)
	require.False(t, unique)
}

// execRecorderConnPool is a minimal gorm.ConnPool that records statements and
// returns nothing, making the migrator's ALTER-or-not decision observable
// without a MySQL server.
type execRecorderConnPool struct {
	statements []string
}

func (p *execRecorderConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, driver.ErrSkip
}

func (p *execRecorderConnPool) ExecContext(_ context.Context, query string, _ ...interface{}) (sql.Result, error) {
	p.statements = append(p.statements, query)
	return driver.RowsAffected(0), nil
}

func (p *execRecorderConnPool) QueryContext(_ context.Context, query string, _ ...interface{}) (*sql.Rows, error) {
	p.statements = append(p.statements, query)
	return nil, sql.ErrNoRows
}

func (p *execRecorderConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return &sql.Row{}
}

func (p *execRecorderConnPool) alters() []string {
	var out []string
	for _, statement := range p.statements {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(statement)), "ALTER TABLE") {
			out = append(out, statement)
		}
	}
	return out
}

// openShimTestDB opens gorm against a recording pool, with or without the shim.
func openShimTestDB(t *testing.T, wrapped bool) (*gorm.DB, *execRecorderConnPool) {
	t.Helper()
	pool := &execRecorderConnPool{}
	stock := mysql.New(mysql.Config{
		Conn:                      pool,
		SkipInitializeWithVersion: true,
		ServerVersion:             "8.0.36",
	})
	dialector := stock
	if wrapped {
		dialector = &boolDefaultAwareMySQLDialector{Dialector: stock.(*mysql.Dialector)}
	}
	db, err := gorm.Open(dialector, &gorm.Config{
		DisableAutomaticPing: true,
		Logger:               logger.Discard,
	})
	require.NoError(t, err)
	return db, pool
}

func TestShimSkipsAlterWhenBoolDefaultOnlyDiffersInSpelling(t *testing.T) {
	column := fakeColumnType{name: "is_honeypot", dbType: "tinyint", defaultValue: "0", hasDefault: true}

	stockDB, stockPool := openShimTestDB(t, false)
	require.NoError(t, stockDB.Migrator().MigrateColumn(&shimProbeModel{}, shimProbeField(t, "IsHoneypot"), column))
	require.Len(t, stockPool.alters(), 1, "stock migrator was expected to re-alter, this is the production bug")
	require.Contains(t, stockPool.alters()[0], "MODIFY COLUMN `is_honeypot`")

	shimDB, shimPool := openShimTestDB(t, true)
	require.NoError(t, shimDB.Migrator().MigrateColumn(&shimProbeModel{}, shimProbeField(t, "IsHoneypot"), column))
	require.Empty(t, shimPool.alters(), "shim must not re-alter when only the spelling differs")
}

func TestShimStillAltersOnRealBoolDefaultDrift(t *testing.T) {
	drifted := fakeColumnType{name: "is_honeypot", dbType: "tinyint", defaultValue: "1", hasDefault: true}

	db, pool := openShimTestDB(t, true)
	require.NoError(t, db.Migrator().MigrateColumn(&shimProbeModel{}, shimProbeField(t, "IsHoneypot"), drifted))
	require.Len(t, pool.alters(), 1, "a genuinely wrong default must still be repaired")
	require.Contains(t, pool.alters()[0], "MODIFY COLUMN `is_honeypot`")
	require.Contains(t, pool.alters()[0], "DEFAULT false")
}

func TestShimStillAltersWhenBoolDefaultDisappears(t *testing.T) {
	missing := fakeColumnType{name: "is_honeypot", dbType: "tinyint", hasDefault: false}

	db, pool := openShimTestDB(t, true)
	require.NoError(t, db.Migrator().MigrateColumn(&shimProbeModel{}, shimProbeField(t, "IsHoneypot"), missing))
	require.Len(t, pool.alters(), 1, "a dropped default must still be restored")
	require.Contains(t, pool.alters()[0], "DEFAULT false")
}

func TestShimLeavesNonBoolColumnsToStockLogic(t *testing.T) {
	column := fakeColumnType{name: "label", dbType: "varchar", defaultValue: "placeholder", hasDefault: true}

	for _, wrapped := range []bool{false, true} {
		db, pool := openShimTestDB(t, wrapped)
		require.NoError(t, db.Migrator().MigrateColumn(&shimProbeModel{}, shimProbeField(t, "Label"), column))
		require.Empty(t, pool.alters(), "wrapped=%v: matching string default needs no ALTER", wrapped)
	}
}

func TestNewMySQLDialectorPreservesDriverBehaviour(t *testing.T) {
	dialector := newMySQLDialector("root:x@tcp(127.0.0.1:1)/nonexistent")

	require.Equal(t, "mysql", dialector.Name(), "dialect switches across model/ key off Name()")

	_, isStock := dialector.(*mysql.Dialector)
	require.False(t, isStock, "chooseDB must get the wrapping dialector")

	_, ok := dialector.(gorm.SavePointerDialectorInterface)
	require.True(t, ok, "nested transactions rely on SAVEPOINT support")

	_, ok = dialector.(interface{ Apply(*gorm.Config) error })
	require.True(t, ok, "gorm applies driver defaults through Apply")

	_, ok = dialector.Migrator(&gorm.DB{Config: &gorm.Config{}}).(boolDefaultAwareMySQLMigrator)
	require.True(t, ok, "Migrator() must return the wrapping migrator")
}
