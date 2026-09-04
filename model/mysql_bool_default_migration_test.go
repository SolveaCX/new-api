package model

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// mysqlBoolDefaultProbe mirrors the users columns that production keeps
// re-altering on every console start: bool columns with a `default:false`
// tag. Its table name is unique so the test never touches real tables.
type mysqlBoolDefaultProbe struct {
	Id            int    `gorm:"primaryKey"`
	Name          string `gorm:"type:varchar(64);default:''"`
	IsHoneypot    bool   `gorm:"default:false"`
	IsEnterprise  bool   `gorm:"default:false"`
	Enabled       bool   `gorm:"default:true"`
	RequiredFlag  bool   `gorm:"not null;default:false"`
	NullableCount int    `gorm:"default:0"`
}

func (mysqlBoolDefaultProbe) TableName() string {
	return "zz_gorm_bool_default_probe"
}

// alterRecorder captures every ALTER TABLE statement gorm executes so the test
// can assert on migration behaviour instead of trusting return values.
type alterRecorder struct {
	logger.Interface
	mu     sync.Mutex
	alters []string
}

func (r *alterRecorder) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, _ := fc()
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sql)), "ALTER TABLE") {
		r.mu.Lock()
		r.alters = append(r.alters, sql)
		r.mu.Unlock()
	}
	r.Interface.Trace(ctx, begin, fc, err)
}

func (r *alterRecorder) reset() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := append([]string(nil), r.alters...)
	r.alters = nil
	return out
}

func openMySQLBoolDefaultTestDB(t *testing.T) (*gorm.DB, *alterRecorder) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run the MySQL bool default migration test")
	}
	rec := &alterRecorder{Interface: logger.Default.LogMode(logger.Silent)}
	db, err := gorm.Open(newMySQLDialector(ensureMySQLDSNDefaults(dsn)), &gorm.Config{Logger: rec})
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	if db.Migrator().HasTable(&mysqlBoolDefaultProbe{}) {
		require.NoError(t, db.Migrator().DropTable(&mysqlBoolDefaultProbe{}))
	}
	t.Cleanup(func() {
		_ = db.Migrator().DropTable(&mysqlBoolDefaultProbe{})
	})
	return db, rec
}

func TestMySQLAutoMigrateDoesNotReAlterBoolDefaultColumns(t *testing.T) {
	db, rec := openMySQLBoolDefaultTestDB(t)

	require.NoError(t, db.AutoMigrate(&mysqlBoolDefaultProbe{}))
	rec.reset()

	// Second run against an already-migrated table must be a no-op. This is
	// exactly what every newapi-console instance start does in production.
	require.NoError(t, db.AutoMigrate(&mysqlBoolDefaultProbe{}))
	require.Empty(t, rec.reset(), "second AutoMigrate must not emit ALTER TABLE for unchanged bool default columns")
}

func TestMySQLAutoMigrateStillAltersWhenBoolDefaultChanges(t *testing.T) {
	db, rec := openMySQLBoolDefaultTestDB(t)

	require.NoError(t, db.AutoMigrate(&mysqlBoolDefaultProbe{}))
	// Flip the stored default behind gorm's back; the next AutoMigrate must
	// notice and repair it, proving the fix did not just disable the check.
	require.NoError(t, db.Exec("ALTER TABLE zz_gorm_bool_default_probe MODIFY COLUMN is_honeypot tinyint(1) DEFAULT 1").Error)
	rec.reset()

	require.NoError(t, db.AutoMigrate(&mysqlBoolDefaultProbe{}))
	alters := rec.reset()
	require.Len(t, alters, 1, "expected exactly one repair ALTER, got %v", alters)
	require.Contains(t, alters[0], "is_honeypot")

	var def string
	require.NoError(t, db.Raw("SELECT COLUMN_DEFAULT FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'zz_gorm_bool_default_probe' AND COLUMN_NAME = 'is_honeypot'").Scan(&def).Error)
	require.Equal(t, "0", def)
}

func TestMySQLDialectorMigratorWrapsBoolDefaultComparison(t *testing.T) {
	d := newMySQLDialector("root:x@tcp(127.0.0.1:1)/nonexistent")
	if _, ok := d.(*mysql.Dialector); ok {
		t.Fatalf("newMySQLDialector must return a dialector whose Migrator fixes bool default comparison, got the stock mysql.Dialector")
	}
}
