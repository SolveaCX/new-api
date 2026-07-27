package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	supplierIdempotencyKeyMaxLength = 128
	supplierAdminKeywordMaxLength   = 128
)

func ListSupplyChainSuppliers(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	status := strings.TrimSpace(c.Query("status"))
	if !validSupplierStatusFilter(status) {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return
	}
	keyword, ok := supplyChainAdminKeyword(c)
	if !ok {
		return
	}
	items, total, err := model.ListSupplierAdminRows(model.SupplierAdminListFilter{
		Page: model.SupplierPage{Offset: pageInfo.GetStartIdx(), Limit: pageInfo.GetPageSize(), Status: status}, Keyword: keyword,
	})
	if err != nil {
		supplyChainModelError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetSupplyChainSupplier(c *gin.Context) {
	id, ok := supplyChainPositivePathInt(c, "id")
	if !ok {
		return
	}
	item, err := model.GetUpstreamSupplierByID(id)
	if err != nil {
		supplyChainModelError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func CreateSupplyChainSupplier(c *gin.Context) {
	var request dto.SupplierCreateRequest
	if c.ShouldBindJSON(&request) != nil || strings.TrimSpace(request.Name) == "" {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return
	}
	item := &model.UpstreamSupplier{Name: request.Name, Remark: request.Remark}
	if err := model.CreateUpstreamSupplier(item); err != nil {
		supplyChainModelError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func UpdateSupplyChainSupplier(c *gin.Context) {
	id, ok := supplyChainPositivePathInt(c, "id")
	if !ok {
		return
	}
	var request dto.SupplierUpdateRequest
	if c.ShouldBindJSON(&request) != nil || request.ExpectedVersion == nil || *request.ExpectedVersion <= 0 || (request.Name == nil && request.Remark == nil) || (request.Name != nil && strings.TrimSpace(*request.Name) == "") {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return
	}
	item, err := model.UpdateUpstreamSupplier(id, model.UpdateUpstreamSupplierInput{Name: request.Name, Remark: request.Remark, ExpectedVersion: *request.ExpectedVersion})
	if err != nil {
		supplyChainModelError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func InactivateSupplyChainSupplier(c *gin.Context) {
	id, ok := supplyChainPositivePathInt(c, "id")
	if !ok {
		return
	}
	var request dto.SupplierInactivateRequest
	if c.ShouldBindJSON(&request) != nil || request.ExpectedVersion == nil || *request.ExpectedVersion <= 0 {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return
	}
	if err := model.InactivateUpstreamSupplierCAS(id, *request.ExpectedVersion); err != nil {
		supplyChainModelError(c, err)
		return
	}
	item, err := model.GetUpstreamSupplierByID(id)
	if err != nil {
		supplyChainModelError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func ListSupplyChainContracts(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	supplierID, ok := supplyChainOptionalPositiveQueryInt(c, "supplier_id")
	if !ok {
		return
	}
	status := strings.TrimSpace(c.Query("status"))
	if !validSupplierStatusFilter(status) {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return
	}
	keyword, ok := supplyChainAdminKeyword(c)
	if !ok {
		return
	}
	items, total, err := model.ListSupplierContractAdminRows(model.SupplierContractAdminListFilter{
		SupplierId: supplierID, Page: model.SupplierPage{Offset: pageInfo.GetStartIdx(), Limit: pageInfo.GetPageSize(), Status: status}, Keyword: keyword,
	})
	if err != nil {
		supplyChainModelError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetSupplyChainContract(c *gin.Context) {
	id, ok := supplyChainPositivePathInt(c, "id")
	if !ok {
		return
	}
	item, err := model.GetSupplierContractByID(id)
	if err != nil {
		supplyChainModelError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func CreateSupplyChainContract(c *gin.Context) {
	var request dto.SupplierContractCreateRequest
	if c.ShouldBindJSON(&request) != nil || request.SupplierId <= 0 || strings.TrimSpace(request.Name) == "" || strings.TrimSpace(request.ContractNo) == "" || request.RpmLimit < 0 || request.TpmLimit < 0 || request.MaxConcurrency < 0 {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return
	}
	item := &model.SupplierContract{
		SupplierId: request.SupplierId, Name: request.Name, ContractNo: request.ContractNo, Remark: request.Remark,
		RpmLimit: request.RpmLimit, TpmLimit: request.TpmLimit, MaxConcurrency: request.MaxConcurrency,
	}
	if err := model.CreateSupplierContract(item); err != nil {
		supplyChainModelError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func UpdateSupplyChainContract(c *gin.Context) {
	id, ok := supplyChainPositivePathInt(c, "id")
	if !ok {
		return
	}
	var request dto.SupplierContractUpdateRequest
	if c.ShouldBindJSON(&request) != nil || request.ExpectedVersion == nil || *request.ExpectedVersion <= 0 || supplierContractUpdateEmpty(request) || !validSupplierContractUpdate(request) {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return
	}
	item, err := model.UpdateSupplierContract(id, model.UpdateSupplierContractInput{
		Name: request.Name, ContractNo: request.ContractNo, Remark: request.Remark,
		RpmLimit: request.RpmLimit, TpmLimit: request.TpmLimit, MaxConcurrency: request.MaxConcurrency, ExpectedVersion: *request.ExpectedVersion,
	})
	if err != nil {
		supplyChainModelError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func InactivateSupplyChainContract(c *gin.Context) {
	id, ok := supplyChainPositivePathInt(c, "id")
	if !ok {
		return
	}
	var request dto.SupplierContractInactivateRequest
	if c.ShouldBindJSON(&request) != nil || request.ExpectedVersion == nil || *request.ExpectedVersion <= 0 {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return
	}
	if err := model.InactivateSupplierContractCAS(id, *request.ExpectedVersion); err != nil {
		supplyChainModelError(c, err)
		return
	}
	item, err := model.GetSupplierContractByID(id)
	if err != nil {
		supplyChainModelError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func ListSupplyChainRateVersions(c *gin.Context) {
	contractID, ok := supplyChainPositivePathInt(c, "id")
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListSupplierContractRateVersions(contractID, model.SupplierPage{Offset: pageInfo.GetStartIdx(), Limit: pageInfo.GetPageSize()})
	if err != nil {
		supplyChainModelError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func CreateSupplyChainRateVersion(c *gin.Context) {
	contractID, ok := supplyChainPositivePathInt(c, "id")
	if !ok {
		return
	}
	var request dto.SupplierRateVersionCreateRequest
	if c.ShouldBindJSON(&request) != nil || request.ProcurementMultiplierPpm == nil || *request.ProcurementMultiplierPpm < 0 || *request.ProcurementMultiplierPpm > 1_000_000 {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidRate)
		return
	}
	item, err := model.CreateAndActivateSupplierContractRateVersion(contractID, *request.ProcurementMultiplierPpm, c.GetInt("id"), request.Reason)
	if err != nil {
		supplyChainModelError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func ListSupplyChainInventoryAdjustments(c *gin.Context) {
	contractID, ok := supplyChainPositivePathInt(c, "id")
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListSupplierInventoryAdjustments(contractID, model.SupplierPage{Offset: pageInfo.GetStartIdx(), Limit: pageInfo.GetPageSize()})
	if err != nil {
		supplyChainModelError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func CreateSupplyChainInventoryAdjustment(c *gin.Context) {
	contractID, ok := supplyChainPositivePathInt(c, "id")
	if !ok {
		return
	}
	idempotencyKey, ok := supplyChainIdempotencyKey(c)
	if !ok {
		return
	}
	var request dto.SupplierInventoryAdjustmentCreateRequest
	if c.ShouldBindJSON(&request) != nil || request.DeltaMicroUsd == nil || *request.DeltaMicroUsd == 0 || !validSupplierInventoryType(request.Type) {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidMoney)
		return
	}
	item, err := model.CreateSupplierInventoryAdjustment(&model.SupplierInventoryAdjustment{
		ContractId: contractID, DeltaMicroUsd: *request.DeltaMicroUsd, Type: request.Type,
		Reason: request.Reason, IdempotencyKey: idempotencyKey, CreatedBy: c.GetInt("id"),
	})
	if err != nil {
		supplyChainModelError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func ListSupplyChainExclusionRules(c *gin.Context) {
	userID, ok := supplyChainOptionalPositiveQueryInt(c, "user_id")
	if !ok {
		return
	}
	action := strings.TrimSpace(c.Query("action"))
	if action != "" && action != model.SupplierStatisticsActionExclude && action != model.SupplierStatisticsActionInclude {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return
	}
	currentOnly, ok := supplyChainOptionalBoolQuery(c, "current_only")
	if !ok {
		return
	}
	keyword, ok := supplyChainAdminKeyword(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	filter := model.SupplierExclusionAdminListFilter{
		UserId: userID, Action: action, Keyword: keyword, CurrentOnly: currentOnly,
		Page: model.SupplierPage{Offset: pageInfo.GetStartIdx(), Limit: pageInfo.GetPageSize()},
	}
	var items any
	var total int64
	var err error
	if currentOnly {
		items, total, err = model.ListSupplierEffectiveExclusions(filter)
	} else {
		items, total, err = model.ListSupplierExclusionHistory(filter)
	}
	if err != nil {
		supplyChainModelError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func CreateSupplyChainExclusionRule(c *gin.Context) {
	idempotencyKey, ok := supplyChainIdempotencyKey(c)
	if !ok {
		return
	}
	var request dto.SupplierExclusionRuleCreateRequest
	if c.ShouldBindJSON(&request) != nil || request.UserId <= 0 || (request.Action != model.SupplierStatisticsActionExclude && request.Action != model.SupplierStatisticsActionInclude) {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return
	}
	item, err := model.CreateSupplierStatisticsExclusionRule(request.UserId, request.Action, c.GetInt("id"), request.Reason, idempotencyKey)
	if err != nil {
		supplyChainModelError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func ListSupplyChainChannelBindings(c *gin.Context) {
	contractID, ok := supplyChainOptionalPositiveQueryInt(c, "contract_id")
	if !ok {
		return
	}
	keyword, ok := supplyChainAdminKeyword(c)
	if !ok {
		return
	}
	bound, ok := supplyChainBoundStateQuery(c)
	if !ok {
		return
	}
	channelStatus, ok := supplyChainOptionalNonNegativeQueryInt(c, "channel_status")
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListSupplierChannelBindingAdminRows(model.SupplierChannelBindingAdminListFilter{
		ContractId: contractID, Keyword: keyword, Bound: bound, ChannelStatus: channelStatus,
		Page: model.SupplierPage{Offset: pageInfo.GetStartIdx(), Limit: pageInfo.GetPageSize()},
	})
	if err != nil {
		supplyChainModelError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetSupplyChainChannelBinding(c *gin.Context) {
	channelID, ok := supplyChainPositivePathInt(c, "channel_id")
	if !ok {
		return
	}
	item, err := model.GetChannelSupplierContractBinding(channelID)
	if err != nil {
		supplyChainModelError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func BindSupplyChainChannel(c *gin.Context) {
	channelID, ok := supplyChainPositivePathInt(c, "channel_id")
	if !ok {
		return
	}
	var request dto.SupplierChannelBindingRequest
	if c.ShouldBindJSON(&request) != nil || request.ContractId == nil || *request.ContractId <= 0 || request.ExpectedContractId == nil || *request.ExpectedContractId < 0 {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return
	}
	if err := model.SetChannelSupplierContractCASForActor(channelID, *request.ExpectedContractId, request.ContractId, c.GetInt("id")); err != nil {
		supplyChainModelError(c, err)
		return
	}
	item, err := model.GetChannelSupplierContractBinding(channelID)
	if err != nil {
		supplyChainModelError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func UnbindSupplyChainChannel(c *gin.Context) {
	channelID, ok := supplyChainPositivePathInt(c, "channel_id")
	if !ok {
		return
	}
	expectedContractID, ok := supplyChainRequiredNonNegativeQueryInt(c, "expected_contract_id")
	if !ok {
		return
	}
	if err := model.SetChannelSupplierContractCASForActor(channelID, expectedContractID, nil, c.GetInt("id")); err != nil {
		supplyChainModelError(c, err)
		return
	}
	item, err := model.GetChannelSupplierContractBinding(channelID)
	if err != nil {
		supplyChainModelError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func supplyChainAdminKeyword(c *gin.Context) (string, bool) {
	keyword := strings.TrimSpace(c.Query("keyword"))
	if len(keyword) > supplierAdminKeywordMaxLength {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return "", false
	}
	return keyword, true
}

func supplyChainOptionalBoolQuery(c *gin.Context, name string) (bool, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return false, true
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return false, false
	}
	return value, true
}

func supplyChainBoundStateQuery(c *gin.Context) (*bool, bool) {
	switch strings.TrimSpace(c.Query("bound_state")) {
	case "":
		return nil, true
	case "bound":
		value := true
		return &value, true
	case "unbound":
		value := false
		return &value, true
	default:
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return nil, false
	}
}

func supplyChainOptionalNonNegativeQueryInt(c *gin.Context, name string) (*int, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return nil, false
	}
	return &value, true
}

func supplyChainRequiredNonNegativeQueryInt(c *gin.Context, name string) (int, bool) {
	raw := strings.TrimSpace(c.Query(name))
	value, err := strconv.Atoi(raw)
	if raw == "" || err != nil || value < 0 {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return 0, false
	}
	return value, true
}

func supplyChainModelError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		supplyChainError(c, http.StatusNotFound, i18n.MsgSupplyChainNotFound)
	case errors.Is(err, model.ErrSupplierInvalidStatus), errors.Is(err, model.ErrSupplierInvalidContract),
		errors.Is(err, model.ErrSupplierInvalidRate), errors.Is(err, model.ErrSupplierInvalidInventory),
		errors.Is(err, model.ErrSupplierInvalidStatsRule):
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
	case errors.Is(err, model.ErrSupplierInactive), errors.Is(err, model.ErrSupplierContractInactive),
		errors.Is(err, model.ErrSupplierContractBound), errors.Is(err, model.ErrSupplierHasActiveContracts),
		errors.Is(err, model.ErrSupplierHasChannelBindings), errors.Is(err, model.ErrSupplierCurrentRateRequired),
		errors.Is(err, model.ErrSupplierBindingChanged), errors.Is(err, model.ErrSupplierIdempotencyConflict),
		errors.Is(err, model.ErrSupplierVersionConflict),
		errors.Is(err, model.ErrSupplierImmutableField), errors.Is(err, model.ErrSupplierAppendOnly), errors.Is(err, gorm.ErrDuplicatedKey):
		supplyChainError(c, http.StatusConflict, i18n.MsgSupplyChainConflict)
	default:
		logger.LogError(c.Request.Context(), fmt.Sprintf("supplier admin request failed: %v", err))
		supplyChainError(c, http.StatusInternalServerError, i18n.MsgSupplyChainInternalError)
	}
}

// supplyChainError preserves the standard ApiErrorI18n response shape while
// using semantic HTTP status codes required by the management API contract.
func supplyChainError(c *gin.Context, status int, key string) {
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Status(status)
	c.Writer.WriteHeaderNow()
	common.ApiErrorI18n(c, key)
}

func supplyChainPositivePathInt(c *gin.Context, name string) (int, bool) {
	value, err := strconv.Atoi(c.Param(name))
	if err != nil || value <= 0 {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return 0, false
	}
	return value, true
}

func supplyChainOptionalPositiveQueryInt(c *gin.Context, name string) (int, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return 0, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return 0, false
	}
	return value, true
}

func supplyChainIdempotencyKey(c *gin.Context) (string, bool) {
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" || len(key) > supplierIdempotencyKeyMaxLength {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainIdempotencyKeyRequired)
		return "", false
	}
	return key, true
}

func validSupplierStatusFilter(status string) bool {
	return status == "" || status == model.SupplierStatusActive || status == model.SupplierStatusInactive
}

func validSupplierInventoryType(value string) bool {
	switch strings.TrimSpace(value) {
	case model.SupplierInventoryAdjustmentTypeInitial, model.SupplierInventoryAdjustmentTypeReplenishment,
		model.SupplierInventoryAdjustmentTypeCorrection, model.SupplierInventoryAdjustmentTypeReversal:
		return true
	default:
		return false
	}
}

func supplierContractUpdateEmpty(request dto.SupplierContractUpdateRequest) bool {
	return request.Name == nil && request.ContractNo == nil && request.Remark == nil && request.RpmLimit == nil && request.TpmLimit == nil && request.MaxConcurrency == nil
}

func validSupplierContractUpdate(request dto.SupplierContractUpdateRequest) bool {
	return (request.Name == nil || strings.TrimSpace(*request.Name) != "") &&
		(request.ContractNo == nil || strings.TrimSpace(*request.ContractNo) != "") &&
		(request.RpmLimit == nil || *request.RpmLimit >= 0) &&
		(request.TpmLimit == nil || *request.TpmLimit >= 0) &&
		(request.MaxConcurrency == nil || *request.MaxConcurrency >= 0)
}
