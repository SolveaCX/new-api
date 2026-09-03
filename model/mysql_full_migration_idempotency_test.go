package model

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestMySQLFullSchemaAutoMigrateIsIdempotent runs the real production model
// list twice against an empty MySQL database. The second pass is what every
// console instance start executes; it must not emit a single ALTER TABLE.
func TestMySQLFullSchemaAutoMigrateIsIdempotent(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run the MySQL full schema idempotency test")
	}
	rec := &alterRecorder{Interface: logger.Default.LogMode(logger.Silent)}
	db, err := gorm.Open(newMySQLDialector(ensureMySQLDSNDefaults(dsn)), &gorm.Config{Logger: rec})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	models := migrationModelValues(orderedMigrationModels())
	for _, m := range models {
		if db.Migrator().HasTable(m) {
			t.Skipf("refusing to run against a non-empty database: table for %T already exists", m)
		}
	}
	t.Cleanup(func() { _ = db.Migrator().DropTable(models...) })

	require.NoError(t, db.AutoMigrate(models...))
	firstPass := rec.reset()

	require.NoError(t, db.AutoMigrate(models...))
	secondPass := rec.reset()
	require.Empty(t, secondPass, "second full AutoMigrate emitted ALTER TABLE:\n%s", strings.Join(secondPass, "\n"))
	t.Logf("first pass emitted %d ALTER TABLE statements (fresh schema), second pass emitted %d", len(firstPass), len(secondPass))
}

// TestMySQLFullSchemaStockDialectorReAltersBoolDefaults is the control for the
// test above: with the unpatched driver the second pass still emits ALTER
// TABLE for every bool column carrying a default tag. It exists so a future
// gorm upgrade that silently changes the behaviour is visible, and to prove the
// recorder actually sees migration DDL.
func TestMySQLFullSchemaStockDialectorReAltersBoolDefaults(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run the MySQL full schema control test")
	}
	rec := &alterRecorder{Interface: logger.Default.LogMode(logger.Silent)}
	db, err := gorm.Open(mysql.Open(ensureMySQLDSNDefaults(dsn)), &gorm.Config{Logger: rec})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	models := migrationModelValues(orderedMigrationModels())
	for _, m := range models {
		if db.Migrator().HasTable(m) {
			t.Skipf("refusing to run against a non-empty database: table for %T already exists", m)
		}
	}
	t.Cleanup(func() { _ = db.Migrator().DropTable(models...) })

	require.NoError(t, db.AutoMigrate(models...))
	rec.reset()
	require.NoError(t, db.AutoMigrate(models...))
	secondPass := rec.reset()
	require.NotEmpty(t, secondPass, "stock mysql dialector was expected to re-alter bool default columns on the second pass")
	for _, stmt := range secondPass {
		require.Contains(t, strings.ToLower(stmt), "boolean", "unexpected non-bool re-alter with stock dialector: %s", stmt)
	}
	t.Logf("stock dialector second pass re-altered %d bool columns:\n%s", len(secondPass), strings.Join(secondPass, "\n"))
}
