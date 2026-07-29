package model

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	SupplierAccountingPolicyProtocolVersion       = 1
	OptionKeySupplierSkipInternalAccountingActive = "supplier_accounting.skip_internal_accounting_active"
	supplierAccountingPolicyMinimumPropagation    = 2 * time.Minute
)

type supplierAccountingPolicyState struct {
	ProtocolVersion int   `json:"protocol_version"`
	PreviousActive  bool  `json:"previous_active"`
	Activated       bool  `json:"activated"`
	EffectiveAt     int64 `json:"effective_at"`
}

type SupplierAccountingPolicyCapability struct {
	ProtocolVersion int   `json:"protocol_version"`
	Activated       bool  `json:"activated"`
	Active          bool  `json:"active"`
	EffectiveAt     int64 `json:"effective_at"`
}

var supplierAccountingPolicyStatePointer atomic.Pointer[supplierAccountingPolicyState]

func init() {
	RegisterOptionReloadHook(RefreshSupplierAccountingPolicyCapability)
}

func disabledSupplierAccountingPolicyState() *supplierAccountingPolicyState {
	return &supplierAccountingPolicyState{ProtocolVersion: SupplierAccountingPolicyProtocolVersion}
}

func RefreshSupplierAccountingPolicyCapability() {
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[OptionKeySupplierSkipInternalAccountingActive]
	common.OptionMapRWMutex.RUnlock()
	state := disabledSupplierAccountingPolicyState()
	if raw != "" && raw != "false" {
		var configured supplierAccountingPolicyState
		if err := common.UnmarshalJsonStr(raw, &configured); err != nil {
			common.SysError("invalid supplier accounting policy activation state; disabling capability: " + err.Error())
		} else if configured.ProtocolVersion != SupplierAccountingPolicyProtocolVersion || configured.EffectiveAt <= 0 {
			common.SysError(fmt.Sprintf("invalid supplier accounting policy activation state; disabling capability: protocol_version=%d effective_at=%d", configured.ProtocolVersion, configured.EffectiveAt))
		} else {
			state = &configured
		}
	}
	supplierAccountingPolicyStatePointer.Store(state)
}

func currentSupplierAccountingPolicyState() *supplierAccountingPolicyState {
	state := supplierAccountingPolicyStatePointer.Load()
	if state == nil {
		RefreshSupplierAccountingPolicyCapability()
		state = supplierAccountingPolicyStatePointer.Load()
	}
	return state
}

func GetSupplierAccountingPolicyCapability() SupplierAccountingPolicyCapability {
	state := currentSupplierAccountingPolicyState()
	now := common.GetTimestamp()
	active := state.Activated
	if now < state.EffectiveAt {
		active = state.PreviousActive
	}
	return SupplierAccountingPolicyCapability{
		ProtocolVersion: state.ProtocolVersion,
		Activated:       state.Activated,
		Active:          active,
		EffectiveAt:     state.EffectiveAt,
	}
}

func IsSupplierSkipInternalAccountingActive() bool {
	return GetSupplierAccountingPolicyCapability().Active
}

func CanConfigureSupplierSkipInternalAccounting() bool {
	capability := GetSupplierAccountingPolicyCapability()
	return capability.Active || capability.Activated
}

func SetSupplierSkipInternalAccountingActive(activated bool) error {
	current := GetSupplierAccountingPolicyCapability()
	delay := 2 * time.Duration(common.SyncFrequency) * time.Second
	if delay < supplierAccountingPolicyMinimumPropagation {
		delay = supplierAccountingPolicyMinimumPropagation
	}
	state := supplierAccountingPolicyState{
		ProtocolVersion: SupplierAccountingPolicyProtocolVersion,
		PreviousActive:  current.Active,
		Activated:       activated,
		EffectiveAt:     GetDBTimestamp() + int64(delay/time.Second),
	}
	value, err := common.Marshal(state)
	if err != nil {
		return err
	}
	if err := UpdateOptionsBulk(map[string]string{OptionKeySupplierSkipInternalAccountingActive: string(value)}); err != nil {
		return err
	}
	RefreshSupplierAccountingPolicyCapability()
	return nil
}
