package model

import (
	"context"
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// GetDBTimestamp returns a UNIX timestamp from database time.
// Falls back to application time on error.
func GetDBTimestamp() int64 {
	return getDBTimestampTx(DB)
}

func getDBTimestampTx(tx *gorm.DB) int64 {
	ts, err := getDBTimestampTxStrict(tx)
	if err != nil {
		return common.GetTimestamp()
	}
	return ts
}

// GetDBTimestampWithContext returns database time without falling back to a
// process-local clock. Distributed correctness boundaries should use this
// variant and fail closed when database time is unavailable.
func GetDBTimestampWithContext(ctx context.Context) (int64, error) {
	if ctx == nil {
		return 0, fmt.Errorf("context is nil")
	}
	if DB == nil {
		return 0, fmt.Errorf("database is not initialized")
	}
	return getDBTimestamp(DB.WithContext(ctx))
}

func getDBTimestamp(db *gorm.DB) (int64, error) {
	if db == nil || db.Dialector == nil {
		return 0, fmt.Errorf("database is not initialized")
	}
	return scanDBTimestamp(db, dbTimestampQueryForDB(db))
}

func getDBTimestampTxStrict(tx *gorm.DB) (int64, error) {
	if tx == nil {
		return 0, errors.New("database handle is nil")
	}
	return scanDBTimestamp(tx, dbTimestampQuery())
}

func scanDBTimestamp(tx *gorm.DB, query string) (int64, error) {
	var ts int64
	err := tx.Raw(query).Scan(&ts).Error
	if err != nil {
		return 0, err
	}
	if ts <= 0 {
		return 0, fmt.Errorf("database timestamp query returned non-positive timestamp: %d", ts)
	}
	return ts, nil
}

func dbTimestampQuery() string {
	switch {
	case common.UsingPostgreSQL:
		return "SELECT FLOOR(EXTRACT(EPOCH FROM clock_timestamp()))::bigint"
	case common.UsingSQLite:
		return "SELECT strftime('%s','now')"
	default:
		return "SELECT UNIX_TIMESTAMP()"
	}
}

func dbTimestampQueryForDB(db *gorm.DB) string {
	switch db.Dialector.Name() {
	case "postgres":
		return "SELECT FLOOR(EXTRACT(EPOCH FROM clock_timestamp()))::bigint"
	case "sqlite":
		return "SELECT strftime('%s','now')"
	case "mysql":
		return "SELECT UNIX_TIMESTAMP()"
	default:
		return dbTimestampQuery()
	}
}
