package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const supplierAccountingAttemptContextKey = "supplier_accounting_attempt"

var (
	ErrSupplierAccountingAttemptBindingInvalid    = errors.New("supplier accounting attempt binding snapshot is invalid")
	ErrSupplierAccountingAttemptTerminalAmbiguous = errors.New("supplier accounting attempt terminal outcome is ambiguous")
)

type supplierAccountingAttemptState struct {
	attemptID   string
	terminalErr error
}

// PrepareSupplierAccountingAttempt creates the durable pending fact before a
// bound synchronous relay attempt reaches the upstream provider.
func PrepareSupplierAccountingAttempt(c *gin.Context, relayInfo *relaycommon.RelayInfo, channelID int) error {
	if c == nil || relayInfo == nil {
		clearSupplierAccountingAttempt(c)
		return nil
	}
	cutover, configured, err := configuredSupplierAccountingCutover()
	if err != nil {
		clearSupplierAccountingAttempt(c)
		return err
	}
	if !configured {
		clearSupplierAccountingAttempt(c)
		return nil
	}
	snapshot := relayInfo.SupplierCostSnapshot
	if snapshot.CacheUnavailable {
		clearSupplierAccountingAttempt(c)
		active, activationErr := supplierAccountingCutoverActive(c, cutover)
		if activationErr != nil {
			return activationErr
		}
		if !active {
			return nil
		}
		return ErrSupplierAccountingAttemptBindingInvalid
	}
	if !supplierAccountingBindingClaimed(snapshot) {
		clearSupplierAccountingAttempt(c)
		return nil
	}
	if snapshot.SupplierId <= 0 || snapshot.ContractId <= 0 || snapshot.BindingVersionId <= 0 || snapshot.RateVersionId <= 0 ||
		channelID <= 0 || strings.TrimSpace(relayInfo.OriginModelName) == "" {
		clearSupplierAccountingAttempt(c)
		active, activationErr := supplierAccountingCutoverActive(c, cutover)
		if activationErr != nil {
			return activationErr
		}
		if !active {
			return nil
		}
		return ErrSupplierAccountingAttemptBindingInvalid
	}
	if snapshot.SkipInternalAccounting && model.IsSupplierSkipInternalAccountingActive() && relayInfo.SupplierStatisticsScopeSnapshot.Scope == types.SupplierStatisticsScopeInternal {
		clearSupplierAccountingAttempt(c)
		return nil
	}

	state := &supplierAccountingAttemptState{attemptID: uuid.NewString()}
	_, err = model.PrepareSupplierAccountingFactFast(c.Request.Context(), model.LOG_DB, model.SupplierAccountingFactPrepare{
		AttemptId:        state.attemptID,
		ParentRequestId:  relayInfo.RequestId,
		RetryIndex:       relayInfo.RetryIndex,
		SupplierId:       snapshot.SupplierId,
		ContractId:       snapshot.ContractId,
		BindingVersionId: snapshot.BindingVersionId,
		RateVersionId:    snapshot.RateVersionId,
		ChannelId:        channelID,
		ModelName:        relayInfo.OriginModelName,
		CoverageScope:    string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1),
		CutoverAt:        cutover,
	})
	if errors.Is(err, model.ErrSupplierAccountingFactBeforeCutover) {
		clearSupplierAccountingAttempt(c)
		return nil
	}
	if err != nil {
		clearSupplierAccountingAttempt(c)
		return err
	}
	c.Set(supplierAccountingAttemptContextKey, state)
	c.Set(types.SupplierAccountingAttemptIDKeyV1, state.attemptID)
	return nil
}

func supplierAccountingCutoverActive(c *gin.Context, cutover int64) (bool, error) {
	if c == nil || c.Request == nil {
		return false, model.ErrDatabase
	}
	return model.IsSupplierAccountingCutoverActive(c.Request.Context(), model.LOG_DB, cutover)
}

func supplierAccountingBindingClaimed(snapshot types.SupplierCostSnapshot) bool {
	return snapshot.SupplierId != 0 || snapshot.ContractId != 0 || snapshot.BindingVersionId != 0 || snapshot.RateVersionId != 0
}

// FinalizeSupplierAccountingAttempt classifies the settlement envelope for a
// known attempt. Captured facts and definitive zero-usage outcomes are terminal;
// producer or coverage inconsistencies stay pending for reconciliation.
func FinalizeSupplierAccountingAttempt(c *gin.Context, relayInfo *relaycommon.RelayInfo, envelope types.SupplierAccountingEnvelopeV1) error {
	state := currentSupplierAccountingAttempt(c)
	if state == nil {
		return nil
	}
	knownComplete := supplierAccountingResponseKnownComplete(relayInfo)
	switch envelope.Disposition {
	case types.SupplierAccountingDispositionCaptured:
		if !knownComplete {
			return nil
		}
		if err := ValidateSupplierAccountingEnvelopeV1(envelope); err != nil {
			return setSupplierAccountingTerminalError(state, err)
		}
		if err := model.FinalizeSupplierAccountingFactCaptured(c.Request.Context(), model.LOG_DB, state.attemptID, envelope, time.Now().Unix()); err != nil {
			return setSupplierAccountingTerminalError(state, err)
		}
		return nil
	case types.SupplierAccountingDispositionZeroUsage:
		if !knownComplete {
			return nil
		}
		return FinalizeSupplierAccountingAttemptVoid(c)
	default:
		return setSupplierAccountingTerminalError(state, fmt.Errorf("%w: disposition %s", ErrSupplierAccountingAttemptTerminalAmbiguous, envelope.Disposition))
	}
}

// FinalizeSupplierAccountingAttemptVoid is safe only for a definitive zero-use
// outcome or a failure known to occur before upstream dispatch. The controller
// owns that classification.
func FinalizeSupplierAccountingAttemptVoid(c *gin.Context) error {
	state := currentSupplierAccountingAttempt(c)
	if state == nil {
		return nil
	}
	if err := model.FinalizeSupplierAccountingFactVoid(c.Request.Context(), model.LOG_DB, state.attemptID, time.Now().Unix()); err != nil {
		state.terminalErr = err
		return err
	}
	return nil
}

func SupplierAccountingAttemptTerminalError(c *gin.Context) error {
	state := currentSupplierAccountingAttempt(c)
	if state == nil {
		return nil
	}
	return state.terminalErr
}

func setSupplierAccountingTerminalError(state *supplierAccountingAttemptState, err error) error {
	state.terminalErr = err
	return err
}

func supplierAccountingResponseKnownComplete(relayInfo *relaycommon.RelayInfo) bool {
	if relayInfo == nil {
		return false
	}
	if !relayInfo.IsStream {
		return true
	}
	return relayInfo.StreamStatus != nil && relayInfo.StreamStatus.IsNormalEnd() && !relayInfo.StreamStatus.HasErrors()
}

func currentSupplierAccountingAttempt(c *gin.Context) *supplierAccountingAttemptState {
	if c == nil {
		return nil
	}
	value, exists := c.Get(supplierAccountingAttemptContextKey)
	if !exists || value == nil {
		return nil
	}
	state, _ := value.(*supplierAccountingAttemptState)
	return state
}

func clearSupplierAccountingAttempt(c *gin.Context) {
	if c != nil {
		c.Set(supplierAccountingAttemptContextKey, nil)
		c.Set(types.SupplierAccountingAttemptIDKeyV1, "")
	}
}
