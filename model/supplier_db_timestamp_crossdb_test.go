package model

import (
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSupplierDBTimestampCrossDBUsesFloorSemantics(t *testing.T) {
	testCases := []struct {
		name             string
		dsnEnv           string
		expectedDatabase string
		open             func(string) gorm.Dialector
		preciseEpochSQL  string
	}{
		{
			name: "sqlite",
			open: func(_ string) gorm.Dialector {
				return sqlite.Open("file:supplier-db-timestamp-floor?mode=memory&cache=shared")
			},
			preciseEpochSQL: "SELECT (julianday('now') - 2440587.5) * 86400.0",
		},
		{
			name:             "mysql",
			dsnEnv:           "TEST_MYSQL_DSN",
			expectedDatabase: "supplier_g009_mysql",
			open:             func(dsn string) gorm.Dialector { return mysql.Open(dsn) },
			preciseEpochSQL:  "SELECT UNIX_TIMESTAMP(NOW(6))",
		},
		{
			name:             "postgres",
			dsnEnv:           "TEST_POSTGRES_DSN",
			expectedDatabase: "supplier_g009_postgres",
			open:             func(dsn string) gorm.Dialector { return postgres.Open(dsn) },
			preciseEpochSQL:  "SELECT EXTRACT(EPOCH FROM clock_timestamp())",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(testCase.dsnEnv))
			if testCase.dsnEnv != "" && dsn == "" {
				t.Skipf("set %s to run the isolated %s timestamp test", testCase.dsnEnv, testCase.name)
			}
			db, err := gorm.Open(testCase.open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			if testCase.expectedDatabase != "" {
				query := "SELECT DATABASE()"
				if testCase.name == "postgres" {
					query = "SELECT current_database()"
				}
				var current string
				require.NoError(t, db.Raw(query).Scan(&current).Error)
				require.Equal(t, testCase.expectedDatabase, current, "integration test refuses to query a non-isolated database")
			}

			deadline := time.Now().Add(4 * time.Second)
			for time.Now().Before(deadline) {
				var before float64
				require.NoError(t, db.Raw(testCase.preciseEpochSQL).Scan(&before).Error)
				floorBefore := int64(math.Floor(before))
				fraction := before - float64(floorBefore)
				if fraction < 0.60 || fraction > 0.85 {
					time.Sleep(5 * time.Millisecond)
					continue
				}

				timestamp, timestampErr := getSupplierDBTimestamp(db)
				require.NoError(t, timestampErr)
				var after float64
				require.NoError(t, db.Raw(testCase.preciseEpochSQL).Scan(&after).Error)
				if int64(math.Floor(after)) != floorBefore {
					continue
				}
				require.Equal(t, floorBefore, timestamp, "database epoch must truncate fractional seconds instead of rounding into the future")
				return
			}
			t.Fatal("did not observe a stable fractional-second verification window")
		})
	}
}
