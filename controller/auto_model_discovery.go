package controller

import (
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

const autoModelID = "auto"

var autoModelSupportedEndpointTypes = []constant.EndpointType{
	constant.EndpointTypeOpenAI,
	constant.EndpointTypeOpenAIResponse,
	constant.EndpointTypeAnthropic,
}

type autoModelDiscoveryState struct {
	snapshot         *model_setting.AutoModelSnapshot
	realNameConflict bool
}

func loadAutoModelDiscoveryState() autoModelDiscoveryState {
	conflict, err := model.HasCachedRealAutoModelConflict()
	if err != nil {
		return autoModelDiscoveryState{}
	}
	state := autoModelDiscoveryState{realNameConflict: conflict}
	if conflict {
		return state
	}

	snapshot := model_setting.GetAutoModelSnapshot()
	if !isCompleteAutoModelSnapshot(snapshot) {
		return state
	}
	state.snapshot = snapshot
	return state
}

func isCompleteAutoModelSnapshot(snapshot *model_setting.AutoModelSnapshot) bool {
	if snapshot == nil || !snapshot.Initialized || snapshot.Invalid || !snapshot.Config.Enabled || strings.TrimSpace(snapshot.ClassifierAPIKey) == "" || strings.TrimSpace(snapshot.Config.CredentialVersion) == "" {
		return false
	}
	config := snapshot.Config
	config.Routes = make(map[string][]string, len(snapshot.Config.Routes))
	for route, models := range snapshot.Config.Routes {
		config.Routes[route] = append([]string(nil), models...)
	}
	return config.NormalizeAndValidate() == nil
}

func autoModelTokenVisible(candidates []service.ModelAccessModel, state autoModelDiscoveryState, modelLimitsEnabled bool, modelLimits map[string]bool) bool {
	if state.snapshot == nil {
		return false
	}
	if modelLimitsEnabled && !service.TokenAllowsModel(modelLimits, autoModelID) {
		return false
	}
	return hasEligibleAutoModelCandidate(state.snapshot, candidates)
}

func autoModelUserVisible(candidates []service.ModelAccessModel, state autoModelDiscoveryState) bool {
	return state.snapshot != nil && hasEligibleAutoModelCandidate(state.snapshot, candidates)
}

func hasEligibleAutoModelCandidate(snapshot *model_setting.AutoModelSnapshot, candidates []service.ModelAccessModel) bool {
	configured := configuredAutoModelCandidates(snapshot)
	for _, candidate := range candidates {
		if _, ok := configured[candidate.ID]; !ok || candidate.AvailabilityStatus == model.ModelAvailabilityOfficialUnsupported {
			continue
		}
		if supportsAnyAutoModelEndpoint(candidate.SupportedEndpointTypes) {
			return true
		}
	}
	return false
}

func configuredAutoModelCandidates(snapshot *model_setting.AutoModelSnapshot) map[string]struct{} {
	configured := make(map[string]struct{})
	if snapshot == nil {
		return configured
	}
	for _, models := range snapshot.Config.Routes {
		for _, modelName := range models {
			configured[modelName] = struct{}{}
		}
	}
	return configured
}

func supportsAnyAutoModelEndpoint(endpoints []constant.EndpointType) bool {
	for _, endpoint := range endpoints {
		for _, supported := range autoModelSupportedEndpointTypes {
			if endpoint == supported {
				return true
			}
		}
	}
	return false
}

func autoModelAccessMetadata() service.ModelAccessModel {
	return service.ModelAccessModel{
		ID:                     autoModelID,
		AllowlistMatchKey:      service.AllowlistMatchKey(autoModelID),
		SupportedEndpointTypes: append([]constant.EndpointType(nil), autoModelSupportedEndpointTypes...),
		AvailabilityStatus:     service.ModelAvailabilityUnknown,
	}
}

func appendAutoModelToUserAccess(access *service.UserModelAccess, state autoModelDiscoveryState) {
	if access == nil || state.snapshot == nil {
		return
	}
	metadataByID := make(map[string]service.ModelAccessModel, len(access.Models))
	for _, item := range access.Models {
		metadataByID[item.ID] = item
	}

	added := false
	access.IdentityModelIDs, added = appendAutoModelToScope(access.IdentityModelIDs, metadataByID, state.snapshot, added)
	access.AccountModelIDs, added = appendAutoModelToScope(access.AccountModelIDs, metadataByID, state.snapshot, added)
	for i := range access.Groups {
		access.Groups[i].ModelIDs, added = appendAutoModelToScope(access.Groups[i].ModelIDs, metadataByID, state.snapshot, added)
	}
	if !added {
		return
	}
	access.Models = append(access.Models, autoModelAccessMetadata())
	sort.Slice(access.Models, func(i, j int) bool { return access.Models[i].ID < access.Models[j].ID })
}

func appendAutoModelToUserModels(user *model.UserBase, selectedGroup string, modelIDs []string, state autoModelDiscoveryState) []string {
	if user == nil || state.snapshot == nil {
		return modelIDs
	}
	var candidates []service.ModelAccessModel
	if selectedGroup == "" {
		access, err := service.ResolveUserModelAccess(user)
		if err != nil {
			return modelIDs
		}
		candidates = access.Models
	} else {
		access, err := service.ResolveTokenModelAccess(service.TokenModelAccessInput{
			IdentityGroup:  user.Group,
			TokenGroup:     selectedGroup,
			AcceptUnpriced: service.UserAcceptsUnpricedModels(user),
		})
		if err != nil {
			return modelIDs
		}
		candidates = access.Models
	}
	if !autoModelUserVisible(candidates, state) {
		return modelIDs
	}
	for _, modelID := range modelIDs {
		if modelID == autoModelID {
			return modelIDs
		}
	}
	return append(modelIDs, autoModelID)
}

func appendAutoModelToScope(ids []string, metadataByID map[string]service.ModelAccessModel, snapshot *model_setting.AutoModelSnapshot, alreadyAdded bool) ([]string, bool) {
	candidates := make([]service.ModelAccessModel, 0, len(ids))
	for _, id := range ids {
		if item, ok := metadataByID[id]; ok {
			candidates = append(candidates, item)
		}
	}
	if !hasEligibleAutoModelCandidate(snapshot, candidates) {
		return ids, alreadyAdded
	}
	for _, id := range ids {
		if id == autoModelID {
			return ids, true
		}
	}
	ids = append(ids, autoModelID)
	sort.Strings(ids)
	return ids, true
}
