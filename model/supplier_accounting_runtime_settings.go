package model

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	SupplierAccountingRuntimeSettingsProtocolVersion = 1
	OptionKeySupplierAccountingRuntimeSettings       = "supplier_accounting.runtime_settings_v1"
	supplierAccountingTimezone                       = "Asia/Shanghai"
	supplierAccountingMaxRetentionDays               = 36500
)

var (
	ErrSupplierAccountingRuntimeSettingsInvalid  = errors.New("invalid supplier accounting runtime settings")
	ErrSupplierAccountingRuntimeSettingsConflict = errors.New("supplier accounting runtime settings changed")
	ErrSupplierAccountingCutoverLocked           = errors.New("supplier accounting cutover is already active")
)

type supplierAccountingRuntimeSettingsState struct {
	ProtocolVersion int    `json:"protocol_version"`
	Revision        int64  `json:"revision"`
	CutoverAt       int64  `json:"cutover_at"`
	RetentionDays   int    `json:"retention_days"`
	Source          string `json:"-"`
	ConfigError     string `json:"-"`
}

type SupplierAccountingRuntimeSettings struct {
	ProtocolVersion int    `json:"protocol_version"`
	Revision        int64  `json:"revision"`
	CutoverAt       int64  `json:"cutover_at"`
	RetentionDays   int    `json:"retention_days"`
	Source          string `json:"source"`
	CutoverLocked   bool   `json:"cutover_locked"`
}

type SupplierAccountingRuntimeSettingsUpdate struct {
	ExpectedRevision int64
	CutoverAt        int64
	RetentionDays    int
}

var supplierAccountingRuntimeSettingsPointer atomic.Pointer[supplierAccountingRuntimeSettingsState]

func init() {
	RegisterOptionReloadHook(RefreshSupplierAccountingRuntimeSettings)
}

func RefreshSupplierAccountingRuntimeSettings() {
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[OptionKeySupplierAccountingRuntimeSettings]
	common.OptionMapRWMutex.RUnlock()
	supplierAccountingRuntimeSettingsPointer.Store(parseSupplierAccountingRuntimeSettings(raw))
}

func parseSupplierAccountingRuntimeSettings(raw string) *supplierAccountingRuntimeSettingsState {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return supplierAccountingRuntimeSettingsFromEnvironment()
	}
	var state supplierAccountingRuntimeSettingsState
	if err := common.UnmarshalJsonStr(raw, &state); err != nil {
		return &supplierAccountingRuntimeSettingsState{Source: "database", ConfigError: err.Error()}
	}
	state.Source = "database"
	if err := validateSupplierAccountingRuntimeSettings(state); err != nil {
		state.ConfigError = err.Error()
	}
	return &state
}

func supplierAccountingRuntimeSettingsFromEnvironment() *supplierAccountingRuntimeSettingsState {
	state := &supplierAccountingRuntimeSettingsState{
		ProtocolVersion: SupplierAccountingRuntimeSettingsProtocolVersion,
		Source:          "default",
	}
	cutoverRaw := strings.TrimSpace(os.Getenv("SUPPLIER_ACCOUNTING_CUTOVER_AT"))
	retentionRaw := strings.TrimSpace(os.Getenv("SUPPLIER_ACCOUNTING_FACT_RETENTION_DAYS"))
	if cutoverRaw != "" || retentionRaw != "" {
		state.Source = "environment"
	}
	if cutoverRaw != "" {
		value, err := strconv.ParseInt(cutoverRaw, 10, 64)
		if err != nil || value <= 0 {
			state.ConfigError = fmt.Sprintf("invalid SUPPLIER_ACCOUNTING_CUTOVER_AT %q", cutoverRaw)
			return state
		}
		state.CutoverAt = value
	}
	if retentionRaw != "" {
		value, err := strconv.Atoi(retentionRaw)
		if err != nil {
			state.ConfigError = fmt.Sprintf("invalid SUPPLIER_ACCOUNTING_FACT_RETENTION_DAYS %q: must be a non-negative integer", retentionRaw)
			return state
		}
		state.RetentionDays = value
	}
	if err := validateSupplierAccountingRuntimeSettings(*state); err != nil {
		state.ConfigError = err.Error()
	}
	return state
}

func validateSupplierAccountingRuntimeSettings(state supplierAccountingRuntimeSettingsState) error {
	if state.ProtocolVersion != SupplierAccountingRuntimeSettingsProtocolVersion || state.Revision < 0 || state.CutoverAt < 0 || state.RetentionDays < 0 || state.RetentionDays > supplierAccountingMaxRetentionDays {
		return ErrSupplierAccountingRuntimeSettingsInvalid
	}
	if state.CutoverAt == 0 {
		return nil
	}
	location, err := time.LoadLocation(supplierAccountingTimezone)
	if err != nil {
		return err
	}
	local := time.Unix(state.CutoverAt, 0).In(location)
	if local.Hour() != 0 || local.Minute() != 0 || local.Second() != 0 {
		return fmt.Errorf("%w: cutover must be Asia/Shanghai 00:00:00", ErrSupplierAccountingRuntimeSettingsInvalid)
	}
	return nil
}

func currentSupplierAccountingRuntimeSettingsState() *supplierAccountingRuntimeSettingsState {
	state := supplierAccountingRuntimeSettingsPointer.Load()
	if state == nil || state.Source != "database" {
		RefreshSupplierAccountingRuntimeSettings()
		state = supplierAccountingRuntimeSettingsPointer.Load()
	}
	return state
}

func GetSupplierAccountingRuntimeSettings() (SupplierAccountingRuntimeSettings, error) {
	state := currentSupplierAccountingRuntimeSettingsState()
	settings := SupplierAccountingRuntimeSettings{
		ProtocolVersion: state.ProtocolVersion,
		Revision:        state.Revision,
		CutoverAt:       state.CutoverAt,
		RetentionDays:   state.RetentionDays,
		Source:          state.Source,
		CutoverLocked:   state.CutoverAt > 0 && state.CutoverAt <= common.GetTimestamp(),
	}
	if state.ConfigError != "" {
		return settings, errors.New(state.ConfigError)
	}
	return settings, nil
}

func SetSupplierAccountingRuntimeSettings(input SupplierAccountingRuntimeSettingsUpdate) (SupplierAccountingRuntimeSettings, error) {
	current, err := GetSupplierAccountingRuntimeSettings()
	if err != nil {
		return current, err
	}
	next := supplierAccountingRuntimeSettingsState{
		ProtocolVersion: SupplierAccountingRuntimeSettingsProtocolVersion,
		Revision:        current.Revision + 1,
		CutoverAt:       input.CutoverAt,
		RetentionDays:   input.RetentionDays,
		Source:          "database",
	}
	if input.ExpectedRevision != current.Revision {
		return current, ErrSupplierAccountingRuntimeSettingsConflict
	}
	now := GetDBTimestamp()
	if current.CutoverAt > 0 && current.CutoverAt <= now && input.CutoverAt != current.CutoverAt {
		return current, ErrSupplierAccountingCutoverLocked
	}
	if input.CutoverAt != current.CutoverAt && input.CutoverAt > 0 && input.CutoverAt <= now {
		return current, ErrSupplierAccountingRuntimeSettingsInvalid
	}
	if err := validateSupplierAccountingRuntimeSettings(next); err != nil {
		return current, err
	}
	value, err := common.Marshal(next)
	if err != nil {
		return current, err
	}
	var persisted Option
	dbResult := DB.Where(commonKeyCol+" = ?", OptionKeySupplierAccountingRuntimeSettings).First(&persisted)
	if dbResult.Error == nil {
		var stored supplierAccountingRuntimeSettingsState
		if err := common.UnmarshalJsonStr(persisted.Value, &stored); err != nil || stored.Revision != input.ExpectedRevision {
			return current, ErrSupplierAccountingRuntimeSettingsConflict
		}
		result := DB.Model(&Option{}).
			Where(commonKeyCol+" = ? AND value = ?", OptionKeySupplierAccountingRuntimeSettings, persisted.Value).
			Update("value", string(value))
		if result.Error != nil {
			return current, result.Error
		}
		if result.RowsAffected != 1 {
			return current, ErrSupplierAccountingRuntimeSettingsConflict
		}
	} else if errors.Is(dbResult.Error, gorm.ErrRecordNotFound) {
		if input.ExpectedRevision != 0 {
			return current, ErrSupplierAccountingRuntimeSettingsConflict
		}
		result := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&Option{
			Key:   OptionKeySupplierAccountingRuntimeSettings,
			Value: string(value),
		})
		if result.Error != nil {
			return current, result.Error
		}
		if result.RowsAffected != 1 {
			return current, ErrSupplierAccountingRuntimeSettingsConflict
		}
	} else {
		return current, dbResult.Error
	}
	if err := updateOptionMap(OptionKeySupplierAccountingRuntimeSettings, string(value)); err != nil {
		return current, err
	}
	RefreshSupplierAccountingRuntimeSettings()
	if err := common.PublishConfigChanged(context.Background(), common.ConfigScopeOptions); err != nil {
		common.SysError("pubsub: failed to publish supplier accounting runtime settings: " + err.Error())
	}
	return GetSupplierAccountingRuntimeSettings()
}
