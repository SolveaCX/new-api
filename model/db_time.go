package model

import (
	"context"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// GetDBTimestamp returns a UNIX timestamp from database time.
// Falls back to application time on error.
func GetDBTimestamp() int64 {
	ts, err := GetDBTimestampWithContext(context.Background())
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
	var ts int64
	var err error
	dialect := db.Dialector.Name()
	switch dialect {
	case "postgres":
		err = db.Raw("SELECT EXTRACT(EPOCH FROM NOW())::bigint").Scan(&ts).Error
	case "sqlite":
		err = db.Raw("SELECT strftime('%s','now')").Scan(&ts).Error
	case "mysql":
		err = db.Raw("SELECT UNIX_TIMESTAMP()").Scan(&ts).Error
	default:
		switch {
		case common.UsingPostgreSQL:
			err = db.Raw("SELECT EXTRACT(EPOCH FROM NOW())::bigint").Scan(&ts).Error
		case common.UsingSQLite:
			err = db.Raw("SELECT strftime('%s','now')").Scan(&ts).Error
		case common.UsingMySQL:
			err = db.Raw("SELECT UNIX_TIMESTAMP()").Scan(&ts).Error
		default:
			return 0, fmt.Errorf("unsupported database dialect %q", dialect)
		}
	}
	if err != nil {
		return 0, err
	}
	if ts <= 0 {
		return 0, fmt.Errorf("database returned invalid timestamp %d", ts)
	}
	return ts, nil
}
