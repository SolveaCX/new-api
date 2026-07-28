package model

import (
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSupplierAccountingRuntimeSettingsTest(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := DB
	originalSQLite := common.UsingSQLite
	originalMySQL := common.UsingMySQL
	originalPostgreSQL := common.UsingPostgreSQL
	originalState := supplierAccountingRuntimeSettingsPointer.Load()
	common.OptionMapRWMutex.RLock()
	originalOptionMap := make(map[string]string, len(common.OptionMap))
	for key, value := range common.OptionMap {
		originalOptionMap[key] = value
	}
	common.OptionMapRWMutex.RUnlock()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/runtime-settings.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}))
	DB = db
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()
	supplierAccountingRuntimeSettingsPointer.Store(nil)
	t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", "")
	t.Setenv("SUPPLIER_ACCOUNTING_FACT_RETENTION_DAYS", "")
	t.Cleanup(func() {
		DB = originalDB
		common.UsingSQLite = originalSQLite
		common.UsingMySQL = originalMySQL
		common.UsingPostgreSQL = originalPostgreSQL
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
		supplierAccountingRuntimeSettingsPointer.Store(originalState)
	})
	return db
}

func TestSupplierAccountingRuntimeSettingsMigratesEnvironmentToDatabase(t *testing.T) {
	db := setupSupplierAccountingRuntimeSettingsTest(t)
	location, err := time.LoadLocation(supplierAccountingTimezone)
	require.NoError(t, err)
	cutover := time.Now().In(location).AddDate(0, 0, 2)
	cutover = time.Date(cutover.Year(), cutover.Month(), cutover.Day(), 0, 0, 0, 0, location)
	t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", strconv.FormatInt(cutover.Unix(), 10))
	t.Setenv("SUPPLIER_ACCOUNTING_FACT_RETENTION_DAYS", "45")

	settings, err := GetSupplierAccountingRuntimeSettings()
	require.NoError(t, err)
	require.Equal(t, "environment", settings.Source)
	require.Equal(t, cutover.Unix(), settings.CutoverAt)
	require.Equal(t, 45, settings.RetentionDays)

	settings, err = SetSupplierAccountingRuntimeSettings(SupplierAccountingRuntimeSettingsUpdate{
		ExpectedRevision: settings.Revision,
		CutoverAt:        settings.CutoverAt,
		RetentionDays:    60,
	})
	require.NoError(t, err)
	require.Equal(t, "database", settings.Source)
	require.Equal(t, int64(1), settings.Revision)
	require.Equal(t, 60, settings.RetentionDays)
	t.Setenv("SUPPLIER_ACCOUNTING_FACT_RETENTION_DAYS", "999")
	settings, err = GetSupplierAccountingRuntimeSettings()
	require.NoError(t, err)
	require.Equal(t, 60, settings.RetentionDays, "database settings take precedence after migration")

	var option Option
	require.NoError(t, db.First(&option, "key = ?", OptionKeySupplierAccountingRuntimeSettings).Error)
	require.Contains(t, option.Value, `"retention_days":60`)
}

func TestSupplierAccountingRuntimeSettingsRejectsStaleRevision(t *testing.T) {
	setupSupplierAccountingRuntimeSettingsTest(t)
	settings, err := SetSupplierAccountingRuntimeSettings(SupplierAccountingRuntimeSettingsUpdate{RetentionDays: 30})
	require.NoError(t, err)
	require.Equal(t, int64(1), settings.Revision)

	_, err = SetSupplierAccountingRuntimeSettings(SupplierAccountingRuntimeSettingsUpdate{
		ExpectedRevision: 0,
		RetentionDays:    90,
	})
	require.ErrorIs(t, err, ErrSupplierAccountingRuntimeSettingsConflict)
}

func TestSupplierAccountingRuntimeSettingsLocksActiveCutover(t *testing.T) {
	setupSupplierAccountingRuntimeSettingsTest(t)
	location, err := time.LoadLocation(supplierAccountingTimezone)
	require.NoError(t, err)
	past := time.Now().In(location).AddDate(0, 0, -1)
	past = time.Date(past.Year(), past.Month(), past.Day(), 0, 0, 0, 0, location)
	t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", strconv.FormatInt(past.Unix(), 10))

	settings, err := GetSupplierAccountingRuntimeSettings()
	require.NoError(t, err)
	require.True(t, settings.CutoverLocked)
	_, err = SetSupplierAccountingRuntimeSettings(SupplierAccountingRuntimeSettingsUpdate{
		ExpectedRevision: settings.Revision,
		CutoverAt:        past.AddDate(0, 0, 1).Unix(),
	})
	require.ErrorIs(t, err, ErrSupplierAccountingCutoverLocked)

	settings, err = SetSupplierAccountingRuntimeSettings(SupplierAccountingRuntimeSettingsUpdate{
		ExpectedRevision: settings.Revision,
		CutoverAt:        settings.CutoverAt,
		RetentionDays:    7,
	})
	require.NoError(t, err)
	require.Equal(t, 7, settings.RetentionDays)
}

func TestSupplierAccountingRuntimeSettingsDatabaseSnapshotDoesNotQueryDatabase(t *testing.T) {
	originalDB := DB
	originalState := supplierAccountingRuntimeSettingsPointer.Load()
	DB = nil
	supplierAccountingRuntimeSettingsPointer.Store(&supplierAccountingRuntimeSettingsState{
		ProtocolVersion: SupplierAccountingRuntimeSettingsProtocolVersion,
		Revision:        3,
		CutoverAt:       1_785_254_400,
		RetentionDays:   30,
		Source:          "database",
	})
	t.Cleanup(func() {
		DB = originalDB
		supplierAccountingRuntimeSettingsPointer.Store(originalState)
	})

	settings, err := GetSupplierAccountingRuntimeSettings()
	require.NoError(t, err)
	require.Equal(t, int64(3), settings.Revision)
	require.Equal(t, 30, settings.RetentionDays)
}
