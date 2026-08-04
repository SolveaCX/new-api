package service

import (
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

var bytePlusAssetURIPattern = regexp.MustCompile(`^asset://(ast_[A-Za-z0-9]{32})$`)

type BytePlusAssetReferenceResolution struct {
	PinnedChannelID int
	RewriteMap      map[string]string
}

type bytePlusAssetReference struct {
	PublicID          string
	ExpectedAssetType string
}

type bytePlusResolvableAsset struct {
	PublicID   string
	AssetType  string
	Status     string
	Candidates []bytePlusAssetBindingCandidate
}

type bytePlusAssetBindingCandidate struct {
	ChannelID       int
	UpstreamAssetID string
	Status          string
}

func (r BytePlusAssetReferenceResolution) HasReferences() bool {
	return len(r.RewriteMap) > 0
}

func ResolveBytePlusAssetReferences(c *gin.Context, userID int, req *dto.SeedanceVideoRequest) (BytePlusAssetReferenceResolution, *types.NewAPIError) {
	references, apiErr := extractBytePlusAssetPublicIDs(req)
	if apiErr != nil {
		return BytePlusAssetReferenceResolution{}, apiErr
	}
	if len(references) == 0 {
		return BytePlusAssetReferenceResolution{}, nil
	}
	publicIDs := make([]string, 0, len(references))
	for _, reference := range references {
		publicIDs = append(publicIDs, reference.PublicID)
	}

	generalized, err := model.GetAssetsWithBindingsByPublicIDsForUser(userID, publicIDs)
	if err != nil {
		return BytePlusAssetReferenceResolution{}, assetError(err, types.ErrorCodeAssetStorageError, http.StatusInternalServerError)
	}

	missingPublicIDs := make([]string, 0, len(publicIDs))
	for _, publicID := range publicIDs {
		if _, ok := generalized[publicID]; !ok {
			missingPublicIDs = append(missingPublicIDs, publicID)
		}
	}

	legacyAssets, err := model.GetBytePlusAssetsByPublicIDsForUser(userID, missingPublicIDs)
	if err != nil {
		return BytePlusAssetReferenceResolution{}, assetError(err, types.ErrorCodeAssetStorageError, http.StatusInternalServerError)
	}

	byID := make(map[string]bytePlusResolvableAsset, len(generalized)+len(legacyAssets))
	for publicID, item := range generalized {
		resolvable := bytePlusResolvableAsset{
			PublicID:   publicID,
			AssetType:  item.Asset.AssetType,
			Status:     item.Asset.Status,
			Candidates: make([]bytePlusAssetBindingCandidate, 0, len(item.Bindings)),
		}
		for _, binding := range item.Bindings {
			resolvable.Candidates = append(resolvable.Candidates, bytePlusAssetBindingCandidate{
				ChannelID:       binding.ChannelId,
				UpstreamAssetID: binding.UpstreamAssetId,
				Status:          binding.Status,
			})
		}
		if len(resolvable.Candidates) == 0 && item.Binding != nil {
			resolvable.Candidates = append(resolvable.Candidates, bytePlusAssetBindingCandidate{
				ChannelID:       item.Binding.ChannelId,
				UpstreamAssetID: item.Binding.UpstreamAssetId,
				Status:          item.Binding.Status,
			})
		}
		sort.SliceStable(resolvable.Candidates, func(i, j int) bool {
			return resolvable.Candidates[i].ChannelID < resolvable.Candidates[j].ChannelID
		})
		if len(resolvable.Candidates) == 1 && resolvable.Candidates[0].Status != "" {
			resolvable.Status = resolvable.Candidates[0].Status
		}
		byID[publicID] = resolvable
	}
	for _, asset := range legacyAssets {
		byID[asset.PublicId] = bytePlusResolvableAsset{
			PublicID:  asset.PublicId,
			AssetType: asset.AssetType,
			Status:    asset.Status,
			Candidates: []bytePlusAssetBindingCandidate{{
				ChannelID:       asset.ChannelId,
				UpstreamAssetID: asset.UpstreamAssetId,
				Status:          asset.Status,
			}},
		}
	}

	commonChannels := map[int]struct{}{}
	commonChannelsInitialized := false
	for _, reference := range references {
		asset, ok := byID[reference.PublicID]
		if !ok {
			return BytePlusAssetReferenceResolution{}, assetError(errors.New("asset not found"), types.ErrorCodeAssetNotFound, http.StatusNotFound)
		}
		channels := rawPositiveBytePlusAssetChannels(asset.Candidates)
		if len(channels) == 0 {
			continue
		}
		if !commonChannelsInitialized {
			commonChannels = channels
			commonChannelsInitialized = true
			continue
		}
		priorPinnedChannelID := lowestChannelID(commonChannels)
		for channelID := range commonChannels {
			if _, ok := channels[channelID]; !ok {
				delete(commonChannels, channelID)
			}
		}
		if len(commonChannels) == 0 {
			return BytePlusAssetReferenceResolution{PinnedChannelID: priorPinnedChannelID}, assetError(errors.New("asset channels do not match"), types.ErrorCodeAssetChannelConflict, http.StatusConflict)
		}
	}
	pinnedChannelID := lowestChannelID(commonChannels)

	selectedCandidatesByPublicID := make(map[string]bytePlusAssetBindingCandidate, len(byID))
	for _, reference := range references {
		asset := byID[reference.PublicID]
		if asset.AssetType != reference.ExpectedAssetType {
			return BytePlusAssetReferenceResolution{PinnedChannelID: pinnedChannelID}, assetError(errors.New("asset type does not match media type"), types.ErrorCodeInvalidAssetRequest, http.StatusBadRequest)
		}
		if len(asset.Candidates) == 0 {
			return BytePlusAssetReferenceResolution{}, assetError(errors.New("asset channel unavailable"), types.ErrorCodeAssetChannelUnavailable, http.StatusServiceUnavailable)
		}
		candidate := bytePlusAssetCandidateForPinnedChannel(asset.Candidates, pinnedChannelID)
		if apiErr := validateBytePlusAssetLifecycle(asset.Status); apiErr != nil {
			return BytePlusAssetReferenceResolution{PinnedChannelID: pinnedChannelID}, apiErr
		}
		bindingStatus := candidate.Status
		if bindingStatus == "" {
			bindingStatus = asset.Status
		}
		if apiErr := validateBytePlusAssetLifecycle(bindingStatus); apiErr != nil {
			return BytePlusAssetReferenceResolution{PinnedChannelID: pinnedChannelID}, apiErr
		}
		if candidate.ChannelID <= 0 {
			return BytePlusAssetReferenceResolution{PinnedChannelID: pinnedChannelID}, assetError(errors.New("asset channel unavailable"), types.ErrorCodeAssetChannelUnavailable, http.StatusServiceUnavailable)
		}
		if strings.TrimSpace(candidate.UpstreamAssetID) == "" {
			return BytePlusAssetReferenceResolution{PinnedChannelID: pinnedChannelID}, assetError(errors.New("asset is not active"), types.ErrorCodeAssetNotReady, http.StatusConflict)
		}
		selectedCandidatesByPublicID[reference.PublicID] = candidate
	}

	rewriteMap := make(map[string]string, len(publicIDs))
	for _, reference := range references {
		candidate := selectedCandidatesByPublicID[reference.PublicID]
		rewriteMap["asset://"+reference.PublicID] = "asset://" + strings.TrimSpace(candidate.UpstreamAssetID)
	}

	resolution := BytePlusAssetReferenceResolution{
		PinnedChannelID: pinnedChannelID,
		RewriteMap:      rewriteMap,
	}
	if c != nil && resolution.HasReferences() {
		common.SetContextKey(c, constant.ContextKeyBytePlusAssetRewriteMap, rewriteMap)
		common.SetContextKey(c, constant.ContextKeyBytePlusAssetPinnedChannelID, pinnedChannelID)
	}
	return resolution, nil
}

func rawPositiveBytePlusAssetChannels(candidates []bytePlusAssetBindingCandidate) map[int]struct{} {
	channels := make(map[int]struct{}, len(candidates))
	for _, candidate := range candidates {
		if candidate.ChannelID > 0 {
			channels[candidate.ChannelID] = struct{}{}
		}
	}
	return channels
}

func validateBytePlusAssetLifecycle(status string) *types.NewAPIError {
	switch status {
	case model.BytePlusAssetStatusActive:
		return nil
	case model.BytePlusAssetStatusFailed:
		return assetError(errors.New("asset failed"), types.ErrorCodeAssetFailed, http.StatusUnprocessableEntity)
	case model.BytePlusAssetStatusCreating, model.BytePlusAssetStatusProcessing:
		return assetError(errors.New("asset is not active"), types.ErrorCodeAssetNotReady, http.StatusConflict)
	default:
		return assetError(errors.New("asset is not active"), types.ErrorCodeAssetNotReady, http.StatusConflict)
	}
}

func lowestChannelID(channels map[int]struct{}) int {
	lowest := 0
	for channelID := range channels {
		if lowest == 0 || channelID < lowest {
			lowest = channelID
		}
	}
	return lowest
}

func bytePlusAssetCandidateForPinnedChannel(candidates []bytePlusAssetBindingCandidate, pinnedChannelID int) bytePlusAssetBindingCandidate {
	if pinnedChannelID > 0 {
		for _, candidate := range candidates {
			if candidate.ChannelID == pinnedChannelID {
				return candidate
			}
		}
		return bytePlusAssetBindingCandidate{}
	}
	if len(candidates) == 0 {
		return bytePlusAssetBindingCandidate{}
	}
	return candidates[0]
}

func IsStrictBytePlusAssetURI(raw string) bool {
	return bytePlusAssetURIPattern.MatchString(raw)
}

func extractBytePlusAssetPublicIDs(req *dto.SeedanceVideoRequest) ([]bytePlusAssetReference, *types.NewAPIError) {
	if req == nil {
		return nil, nil
	}
	seen := make(map[string]struct{})
	references := make([]bytePlusAssetReference, 0)
	add := func(raw, expectedAssetType string) *types.NewAPIError {
		matches := bytePlusAssetURIPattern.FindStringSubmatch(raw)
		if len(matches) != 2 {
			if isMalformedBytePlusAssetURI(raw) {
				return assetError(errors.New("invalid asset URI"), types.ErrorCodeInvalidAssetRequest, http.StatusBadRequest)
			}
			return nil
		}
		seenKey := matches[1] + "\x00" + expectedAssetType
		if _, ok := seen[seenKey]; ok {
			return nil
		}
		seen[seenKey] = struct{}{}
		references = append(references, bytePlusAssetReference{PublicID: matches[1], ExpectedAssetType: expectedAssetType})
		return nil
	}
	for _, item := range req.Content {
		if item.ImageURL != nil {
			if apiErr := add(item.ImageURL.URL, "Image"); apiErr != nil {
				return nil, apiErr
			}
		}
		if item.VideoURL != nil {
			if apiErr := add(item.VideoURL.URL, "Video"); apiErr != nil {
				return nil, apiErr
			}
		}
		if item.AudioURL != nil {
			if apiErr := add(item.AudioURL.URL, "Audio"); apiErr != nil {
				return nil, apiErr
			}
		}
	}
	return references, nil
}

func isMalformedBytePlusAssetURI(raw string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(raw)), "asset:")
}
