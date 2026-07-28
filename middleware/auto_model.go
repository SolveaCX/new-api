package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/service"
	automodel "github.com/QuantumNous/new-api/service/auto_model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type autoModelClassifier interface {
	Classify(ctx context.Context, snapshot *model_setting.AutoModelSnapshot, text string) (automodel.Route, error)
}

type autoModelProtocol struct {
	extract  automodel.Protocol
	metric   string
	endpoint constant.EndpointType
	messages bool
}

type autoModelPair struct {
	model      string
	group      string
	groupIndex int
}

type autoModelResolution struct {
	pair           autoModelPair
	protocol       autoModelProtocol
	route          automodel.Route
	decisionSource string
	classifierMS   int
	fixedChannel   bool
}

var defaultAutoModelClassifier autoModelClassifier = automodel.NewProductionClassifier(automodel.DefaultClassifierResponseLimit)

func resolveAutoModel(c *gin.Context, raw []byte, fixedChannel *model.Channel) (*autoModelResolution, bool, *types.NewAPIError) {
	conflict, err := model.HasCachedRealAutoModelConflict()
	if err != nil {
		return nil, true, types.NewErrorWithStatusCode(err, types.ErrorCodeGetChannelFailed, http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
	}
	if conflict {
		snapshot := model_setting.GetAutoModelSnapshot()
		if snapshot != nil && snapshot.Config.Enabled {
			return nil, true, automodel.ConfigInvalidError(c)
		}
		return nil, false, nil
	}

	snapshot := model_setting.GetAutoModelSnapshot()
	if snapshot == nil || !snapshot.Initialized || snapshot.Invalid {
		return nil, true, automodel.ConfigInvalidError(c)
	}
	if !snapshot.Config.Enabled {
		return nil, true, automodel.DisabledError(c)
	}
	protocol, ok := autoModelProtocolForRequest(c.Request)
	if !ok || c.GetHeader(automodel.AutoHopHeader) == automodel.AutoHopValue {
		return nil, true, automodel.UnsupportedRequestError(c)
	}
	if !autoModelSnapshotReady(snapshot) {
		return nil, true, automodel.ConfigInvalidError(c)
	}
	if err := validateAutoModelTopLevel(protocol.extract, raw); err != nil {
		return nil, true, automodel.UnsupportedRequestError(c)
	}

	modelLimitsEnabled := common.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled)
	modelLimits := autoModelTokenLimits(c)
	if modelLimitsEnabled && !service.TokenAllowsModel(modelLimits, "auto") {
		return nil, true, autoModelAccessDenied(c, "auto")
	}

	staticPairs, readyPairs, err := eligibleAutoModelPairs(c, snapshot, protocol.endpoint, fixedChannel, modelLimitsEnabled, modelLimits)
	if err != nil {
		return nil, true, types.NewErrorWithStatusCode(err, types.ErrorCodeGetChannelFailed, http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
	}
	if len(staticPairs) == 0 {
		perfmetrics.RecordAutoModelNoEligibleCandidate(protocol.metric)
		return nil, true, automodel.NoEligibleCandidateError(c)
	}
	if len(readyPairs) == 0 {
		return nil, true, types.NewErrorWithStatusCode(errors.New("no channel is currently available for an eligible auto model candidate"), types.ErrorCodeModelNotFound, http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
	}

	_, err = automodel.ExtractText(protocol.extract, raw, int(^uint(0)>>1))
	if err != nil {
		return nil, true, automodel.UnsupportedRequestError(c)
	}
	if setting.ShouldCheckPromptSensitive() {
		sensitiveText, err := autoModelSensitiveText(protocol.extract, raw)
		if err != nil {
			return nil, true, automodel.UnsupportedRequestError(c)
		}
		if contains, _ := service.CheckSensitiveText(sensitiveText); contains {
			return nil, true, types.NewErrorWithStatusCode(errors.New("sensitive words detected"), types.ErrorCodeSensitiveWordsDetected, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
	}
	classifierText, err := automodel.ExtractText(protocol.extract, raw, snapshot.Config.ClassifierInputMaxChars)
	if err != nil {
		return nil, true, automodel.UnsupportedRequestError(c)
	}

	started := time.Now()
	route, classifyErr := defaultAutoModelClassifier.Classify(c.Request.Context(), snapshot, classifierText)
	elapsed := time.Since(started)
	perfmetrics.ObserveAutoModelClassifierDuration(elapsed)
	resolution := &autoModelResolution{
		protocol:     protocol,
		route:        route,
		classifierMS: int(elapsed.Milliseconds()),
		fixedChannel: fixedChannel != nil,
	}
	if classifyErr == nil {
		if pair, found := firstAutoModelPair(automodel.ModelsForRoute(snapshot, route), readyPairs); found {
			resolution.pair = pair
			resolution.decisionSource = "classifier"
			return resolution, true, nil
		}
	} else {
		perfmetrics.RecordAutoModelClassifierError(string(automodel.ClassifierReason(classifyErr)))
	}

	if pair, found := firstAutoModelPair([]string{automodel.DefaultModel(snapshot)}, readyPairs); found {
		resolution.pair = pair
		resolution.decisionSource = "default"
		return resolution, true, nil
	}
	perfmetrics.RecordAutoModelNoEligibleCandidate(protocol.metric)
	return nil, true, automodel.NoEligibleCandidateError(c)
}

func applyAutoModelResolution(c *gin.Context, modelRequest *ModelRequest, raw []byte, resolution *autoModelResolution) *types.NewAPIError {
	rewritten, err := automodel.RewriteModel(raw, resolution.pair.model)
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeBadRequestBody, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	if err := common.ReplaceRequestBody(c, rewritten); err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusInternalServerError, types.ErrOptionWithSkipRetry())
	}
	modelRequest.Model = resolution.pair.model
	common.SetContextKey(c, constant.ContextKeyAutoModelProtocol, resolution.protocol.metric)
	common.SetContextKey(c, constant.ContextKeyAutoModelRoute, string(resolution.route))
	common.SetContextKey(c, constant.ContextKeyAutoModelDecisionSource, resolution.decisionSource)
	common.SetContextKey(c, constant.ContextKeyAutoModelClassifierLatencyMS, resolution.classifierMS)
	common.SetContextKey(c, constant.ContextKeyAutoModelSelectedModel, resolution.pair.model)
	if common.GetContextKeyString(c, constant.ContextKeyTokenGroup) == "auto" {
		common.SetContextKey(c, constant.ContextKeyAutoGroupIndex, resolution.pair.groupIndex)
	}
	if resolution.fixedChannel {
		common.SetContextKey(c, constant.ContextKeyAutoGroup, resolution.pair.group)
	}
	outcome := "selected"
	if resolution.decisionSource == "default" {
		outcome = "fallback"
	}
	perfmetrics.RecordAutoModelRequest(resolution.protocol.metric, string(resolution.route), resolution.pair.model, outcome)
	return nil
}

func autoModelProtocolForRequest(request *http.Request) (autoModelProtocol, bool) {
	if request == nil || request.Method != http.MethodPost || request.URL == nil {
		return autoModelProtocol{}, false
	}
	switch request.URL.Path {
	case "/v1/chat/completions":
		return autoModelProtocol{extract: automodel.ProtocolChatCompletions, metric: "chat", endpoint: constant.EndpointTypeOpenAI}, true
	case "/v1/responses":
		return autoModelProtocol{extract: automodel.ProtocolResponses, metric: "responses", endpoint: constant.EndpointTypeOpenAIResponse}, true
	case "/v1/messages":
		return autoModelProtocol{extract: automodel.ProtocolMessages, metric: "messages", endpoint: constant.EndpointTypeAnthropic, messages: true}, true
	default:
		return autoModelProtocol{}, false
	}
}

func autoModelProtocolForError(request *http.Request) autoModelProtocol {
	if protocol, ok := autoModelProtocolForRequest(request); ok {
		return protocol
	}
	if request != nil && request.URL != nil {
		if request.URL.Path == "/v1/messages" {
			return autoModelProtocol{metric: "messages", messages: true}
		}
		if strings.HasPrefix(request.URL.Path, "/v1/responses") {
			return autoModelProtocol{metric: "responses"}
		}
	}
	return autoModelProtocol{metric: "unknown"}
}

func validateAutoModelTopLevel(protocol automodel.Protocol, raw []byte) error {
	var fields map[string]json.RawMessage
	if err := common.Unmarshal(raw, &fields); err != nil || fields == nil {
		return errors.New("request body must be a JSON object")
	}
	allowed := map[string]struct{}{}
	switch protocol {
	case automodel.ProtocolChatCompletions:
		allowed = autoModelFieldSet("model", "messages", "stream", "stream_options", "max_tokens", "max_completion_tokens", "temperature", "top_p", "stop", "n", "frequency_penalty", "presence_penalty", "seed", "logit_bias", "metadata", "user")
	case automodel.ProtocolResponses:
		allowed = autoModelFieldSet("model", "input", "instructions", "stream", "stream_options", "max_output_tokens", "temperature", "top_p", "truncation", "metadata", "user", "store")
	case automodel.ProtocolMessages:
		allowed = autoModelFieldSet("model", "system", "messages", "max_tokens", "max_tokens_to_sample", "stop_sequences", "temperature", "top_p", "top_k", "stream", "metadata")
	default:
		return errors.New("unsupported protocol")
	}
	for key := range fields {
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("unsupported top-level field %q", key)
		}
	}
	if protocol == automodel.ProtocolResponses {
		if rawStore, exists := fields["store"]; exists {
			var store *bool
			if err := common.Unmarshal(rawStore, &store); err != nil || store == nil || *store {
				return errors.New("responses store must be false")
			}
		}
	}
	return nil
}

func autoModelFieldSet(fields ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		set[field] = struct{}{}
	}
	return set
}

func autoModelSnapshotReady(snapshot *model_setting.AutoModelSnapshot) bool {
	return snapshot != nil && snapshot.Config.Enabled && snapshot.Config.ClassifierBaseURL != "" &&
		snapshot.Config.ClassifierModel != "" && snapshot.Config.ClassifierModel != "auto" &&
		snapshot.Config.DefaultModel != "" && snapshot.ClassifierAPIKey != "" &&
		snapshot.Config.ClassifierInputMaxChars > 0
}

func autoModelTokenLimits(c *gin.Context) map[string]bool {
	value, ok := common.GetContextKey(c, constant.ContextKeyTokenModelLimit)
	if !ok {
		return nil
	}
	limits, _ := value.(map[string]bool)
	return limits
}

func eligibleAutoModelPairs(c *gin.Context, snapshot *model_setting.AutoModelSnapshot, endpoint constant.EndpointType, fixedChannel *model.Channel, modelLimitsEnabled bool, modelLimits map[string]bool) ([]autoModelPair, []autoModelPair, error) {
	identityGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	tokenGroup := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
	if tokenGroup == "" {
		tokenGroup = common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	}
	groups := []string{common.GetContextKeyString(c, constant.ContextKeyUsingGroup)}
	if tokenGroup == "auto" {
		groups = service.GetUserAutoGroup(identityGroup)
	}
	userSetting, _ := common.GetContextKeyType[dto.UserSetting](c, constant.ContextKeyUserSetting)
	models := configuredAutoModelCandidates(snapshot)
	access, err := service.ResolveTokenModelAccessByGroup(service.TokenModelAccessInput{
		IdentityGroup:      identityGroup,
		AcceptUnpriced:     operation_setting.SelfUseModeEnabled || userSetting.AcceptUnsetRatioModel,
		ModelLimitsEnabled: modelLimitsEnabled,
		ModelLimits:        modelLimits,
	}, groups)
	if err != nil {
		return nil, nil, err
	}
	staticPairs := make([]autoModelPair, 0, len(models)*len(groups))
	readyPairs := make([]autoModelPair, 0, len(models)*len(groups))
	for groupIndex, group := range groups {
		if group == "" {
			continue
		}
		metadata := make(map[string]service.ModelAccessModel, len(access.ModelsByGroup[group]))
		for _, item := range access.ModelsByGroup[group] {
			metadata[item.ID] = item
		}
		for _, modelName := range models {
			item, ok := metadata[modelName]
			if !ok || item.AvailabilityStatus == model.ModelAvailabilityOfficialUnsupported || !autoModelDeclaredEndpointSupported(access.ChannelTypesByGroupAndModel[group][modelName], modelName, endpoint) {
				continue
			}
			pair := autoModelPair{model: modelName, group: group, groupIndex: groupIndex}
			if fixedChannel != nil {
				if !model.IsChannelEnabledForGroupModel(group, modelName, fixedChannel.Id) || !service.ChannelSupportsEndpointType(fixedChannel, modelName, endpoint) {
					continue
				}
				staticPairs = append(staticPairs, pair)
				readyPairs = append(readyPairs, pair)
				continue
			}
			staticPairs = append(staticPairs, pair)
			channels, err := model.GetSatisfiedChannelCandidatesWithFilter(group, modelName, 0, func(channel *model.Channel) bool {
				return service.ChannelSupportsEndpointType(channel, modelName, endpoint)
			})
			if err != nil {
				return nil, nil, err
			}
			if len(channels) > 0 {
				readyPairs = append(readyPairs, pair)
			}
		}
	}
	return staticPairs, readyPairs, nil
}

func autoModelDeclaredEndpointSupported(channelTypes []int, modelName string, endpoint constant.EndpointType) bool {
	for _, channelType := range channelTypes {
		if service.ChannelSupportsEndpointType(&model.Channel{Type: channelType}, modelName, endpoint) {
			return true
		}
	}
	return false
}

func autoModelSensitiveText(protocol automodel.Protocol, raw []byte) (string, error) {
	var meta *types.TokenCountMeta
	switch protocol {
	case automodel.ProtocolChatCompletions:
		var request dto.GeneralOpenAIRequest
		if err := common.Unmarshal(raw, &request); err != nil {
			return "", err
		}
		meta = request.GetTokenCountMeta()
	case automodel.ProtocolResponses:
		var request dto.OpenAIResponsesRequest
		if err := common.Unmarshal(raw, &request); err != nil {
			return "", err
		}
		meta = request.GetTokenCountMeta()
	case automodel.ProtocolMessages:
		var request dto.ClaudeRequest
		if err := common.Unmarshal(raw, &request); err != nil {
			return "", err
		}
		meta = request.GetTokenCountMeta()
	default:
		return "", errors.New("unsupported protocol")
	}
	if meta == nil {
		return "", nil
	}
	return meta.CombineText, nil
}

func configuredAutoModelCandidates(snapshot *model_setting.AutoModelSnapshot) []string {
	if snapshot == nil {
		return nil
	}
	seen := make(map[string]struct{})
	models := make([]string, 0, 10)
	for _, route := range []automodel.Route{automodel.RouteGeneral, automodel.RouteCoding, automodel.RouteReasoning, automodel.RouteTranslation} {
		for _, modelName := range automodel.ModelsForRoute(snapshot, route) {
			if _, exists := seen[modelName]; exists {
				continue
			}
			seen[modelName] = struct{}{}
			models = append(models, modelName)
		}
	}
	return models
}

func firstAutoModelPair(models []string, pairs []autoModelPair) (autoModelPair, bool) {
	for _, modelName := range models {
		for _, pair := range pairs {
			if pair.model == modelName {
				return pair, true
			}
		}
	}
	return autoModelPair{}, false
}

func autoModelAccessDenied(c *gin.Context, modelName string) *types.NewAPIError {
	return types.NewErrorWithStatusCode(
		errors.New(i18n.T(c, i18n.MsgDistributorTokenModelForbidden, map[string]any{"Model": modelName})),
		types.ErrorCodeAccessDenied,
		http.StatusForbidden,
		types.ErrOptionWithSkipRetry(),
	)
}

func abortWithAutoModelError(c *gin.Context, protocol autoModelProtocol, apiErr *types.NewAPIError) {
	if apiErr == nil {
		return
	}
	perfmetrics.RecordAutoModelRequest(protocol.metric, "", "", "rejected")
	message := common.MessageWithRequestId(apiErr.Error(), c.GetString(common.RequestIdKey))
	if protocol.messages {
		c.JSON(apiErr.StatusCode, gin.H{
			"type": "error",
			"error": gin.H{
				"type":    "invalid_request_error",
				"message": message,
				"code":    string(apiErr.GetErrorCode()),
			},
		})
		c.Abort()
		return
	}
	abortWithOpenAiMessage(c, apiErr.StatusCode, apiErr.Error(), apiErr.GetErrorCode())
}
