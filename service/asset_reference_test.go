package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAssetReferenceSetRejectsOwnershipMalformedTypeMismatchAndExpiredSources(t *testing.T) {
	tests := []struct {
		name       string
		seed       func(t *testing.T)
		userID     int
		item       dto.SeedanceContentItem
		wantCode   types.ErrorCode
		wantStatus int
	}{
		{
			name: "ownership hidden as not found",
			seed: func(t *testing.T) {
				insertAssetReferenceAsset(t, assetReferenceSeed{UserID: 8, PublicID: "ast_1234567890abcdefABCDEF1234567890", AssetType: "Image", SourceStatus: model.AssetSourceStatusAvailable, SourceExpiresAt: 200})
			},
			userID:     7,
			item:       imageAssetItem("ast_1234567890abcdefABCDEF1234567890"),
			wantCode:   types.ErrorCodeAssetNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "malformed media URI",
			seed:       func(t *testing.T) {},
			userID:     7,
			item:       dto.SeedanceContentItem{Type: dto.SeedanceContentImage, ImageURL: &dto.SeedanceURLObject{URL: "asset://ast_short"}},
			wantCode:   types.ErrorCodeInvalidAssetRequest,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "type mismatch",
			seed: func(t *testing.T) {
				insertAssetReferenceAsset(t, assetReferenceSeed{UserID: 7, PublicID: "ast_1234567890abcdefABCDEF1234567890", AssetType: "Video", SourceStatus: model.AssetSourceStatusAvailable, SourceExpiresAt: 200})
			},
			userID:     7,
			item:       imageAssetItem("ast_1234567890abcdefABCDEF1234567890"),
			wantCode:   types.ErrorCodeInvalidAssetRequest,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "expired source without active binding",
			seed: func(t *testing.T) {
				insertAssetReferenceAsset(t, assetReferenceSeed{UserID: 7, PublicID: "ast_1234567890abcdefABCDEF1234567890", AssetType: "Image", SourceStatus: model.AssetSourceStatusExpired, SourceExpiresAt: 10})
			},
			userID:     7,
			item:       imageAssetItem("ast_1234567890abcdefABCDEF1234567890"),
			wantCode:   types.ErrorCodeAssetExpired,
			wantStatus: http.StatusGone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newAssetReferenceDB(t)
			assetNow = func() time.Time { return time.Unix(100, 0) }
			t.Cleanup(func() { assetNow = time.Now })
			tt.seed(t)

			refs, apiErr := ResolveAssetReferences(newAssetReferenceContext(), tt.userID, &dto.SeedanceVideoRequest{Content: []dto.SeedanceContentItem{tt.item}})
			require.NotNil(t, apiErr)
			require.Equal(t, tt.wantCode, apiErr.GetErrorCode())
			require.Equal(t, tt.wantStatus, apiErr.StatusCode)
			require.False(t, refs.HasReferences())
		})
	}
}

func TestAssetReferenceSetRanksAllActivePartialAndNoBindingReadiness(t *testing.T) {
	newAssetReferenceDB(t)
	assetNow = func() time.Time { return time.Unix(100, 0) }
	t.Cleanup(func() { assetNow = time.Now })
	bytePlus := &model.Channel{Id: 131, Type: constant.ChannelTypeBytePlus, Key: structuredBytePlusKey()}
	bindingScope, err := assetBindingScopeForChannel(bytePlus, AssetMaterializeOptions{APIKey: bytePlus.Key})
	require.NoError(t, err)

	allBound := insertAssetReferenceAsset(t, assetReferenceSeed{UserID: 7, PublicID: "ast_11111111111111111111111111111111", AssetType: "Image", SourceStatus: model.AssetSourceStatusAvailable, SourceExpiresAt: 200, BindingChannelID: 131, BindingScope: bindingScope, UpstreamID: "upstream-all", BindingStatus: model.AssetStatusActive})
	partial := insertAssetReferenceAsset(t, assetReferenceSeed{UserID: 7, PublicID: "ast_22222222222222222222222222222222", AssetType: "Image", SourceStatus: model.AssetSourceStatusAvailable, SourceExpiresAt: 200, BindingChannelID: 131, BindingScope: bindingScope, UpstreamID: "upstream-partial", BindingStatus: model.AssetStatusActive})
	insertAssetReferenceAsset(t, assetReferenceSeed{UserID: 7, PublicID: "ast_33333333333333333333333333333333", AssetType: "Image", SourceStatus: model.AssetSourceStatusAvailable, SourceExpiresAt: 200})

	allSet, apiErr := ResolveAssetReferences(newAssetReferenceContext(), 7, &dto.SeedanceVideoRequest{Content: []dto.SeedanceContentItem{imageAssetItem(allBound.PublicId)}})
	require.Nil(t, apiErr)
	partialSet, apiErr := ResolveAssetReferences(newAssetReferenceContext(), 7, &dto.SeedanceVideoRequest{Content: []dto.SeedanceContentItem{imageAssetItem(partial.PublicId), imageAssetItem("ast_33333333333333333333333333333333")}})
	require.Nil(t, apiErr)
	noneSet, apiErr := ResolveAssetReferences(newAssetReferenceContext(), 7, &dto.SeedanceVideoRequest{Content: []dto.SeedanceContentItem{imageAssetItem("ast_33333333333333333333333333333333")}})
	require.Nil(t, apiErr)

	allReadiness, ok := allSet.ReadinessForChannel(bytePlus)
	require.True(t, ok)
	require.Equal(t, AssetReadinessAllBound, allReadiness)
	partialReadiness, ok := partialSet.ReadinessForChannel(bytePlus)
	require.True(t, ok)
	require.Equal(t, AssetReadinessPartialBound, partialReadiness)
	noneReadiness, ok := noneSet.ReadinessForChannel(bytePlus)
	require.True(t, ok)
	require.Equal(t, AssetReadinessRecoverable, noneReadiness)
}

func TestLoadAssetModelTargetReadinessReopensTargetlessFailureForDirectUse(t *testing.T) {
	db, _ := setupServiceModelAccessDB(t)
	require.NoError(t, db.AutoMigrate(&model.Asset{}, &model.AssetBinding{}, &model.AssetModelCoverageTarget{}, &model.AssetModelReadiness{}))
	seedModelAccessScope(t, db, 180, "default", constant.ChannelTypeTechMobiVideo, "seedance-2.0")
	setModelAccessBilling(t, map[string]float64{"seedance-2.0": 1}, nil, nil)
	registerAssetMaterializerForTest(t, constant.ChannelTypeTechMobiVideo, &recordingAssetMaterializer{})

	asset := insertAssetReferenceAsset(t, assetReferenceSeed{
		UserID:          7,
		PublicID:        "ast_direct_targetless_recovery_1234",
		AssetType:       "Image",
		SourceStatus:    model.AssetSourceStatusAvailable,
		SourceExpiresAt: time.Now().Add(time.Hour).Unix(),
	})
	ctx := assetModelScopeGinContext(t, nil)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	scope, err := ResolveAssetModelScopeForContext(ctx, asset.UserId)
	require.NoError(t, err)
	target, err := ensureAssetModelCoverageTargetAt(scope, "seedance-2.0", "owner", 100)
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.AssetModelReadiness{
		AssetId:          asset.Id,
		ScopeKey:         scope.ScopeKey,
		ModelName:        target.ModelName,
		Status:           model.AssetModelReadinessStatusFailed,
		ErrorClass:       "target_unavailable",
		AttemptCount:     1,
		AttemptStartedAt: 90,
		CreatedAt:        90,
		UpdatedAt:        95,
	}).Error)
	refs := AssetReferenceSet{
		references: []assetReference{{PublicID: asset.PublicId, ExpectedAssetType: "Image"}},
		assets: map[string]assetReferenceAsset{
			asset.PublicId: {
				ID:              asset.Id,
				PublicID:        asset.PublicId,
				AssetType:       asset.AssetType,
				Status:          asset.Status,
				SourceStatus:    asset.SourceStatus,
				StorageBackend:  asset.StorageBackend,
				StorageBucket:   asset.StorageBucket,
				ObjectKey:       asset.ObjectKey,
				SourceExpiresAt: asset.SourceExpiresAt,
			},
		},
	}

	err = refs.loadAssetModelTargetReadiness(ctx, asset.UserId, &dto.SeedanceVideoRequest{
		Model:   "seedance-2.0",
		Content: []dto.SeedanceContentItem{imageAssetItem(asset.PublicId)},
	})

	require.NoError(t, err)
	row := requireAssetModelReadinessRow(t, asset.Id, scope, target.ModelName)
	require.Equal(t, model.AssetModelReadinessStatusPending, row.Status)
	require.Empty(t, row.ErrorClass)
	require.Equal(t, target.Generation, row.TargetGeneration)
	require.Equal(t, target.ChannelId, row.ChannelId)
	require.Equal(t, target.BindingScope, row.BindingScope)
	require.Equal(t, row, refs.readinessByPublicID[asset.PublicId])
}

func TestAssetReferenceSetAllowsLegacyAndSourceUnavailableOnlyOnActiveOriginalBinding(t *testing.T) {
	newAssetReferenceDB(t)
	insertLegacyAssetReferenceAsset(t, 7, 131, "ast_1234567890abcdefABCDEF1234567890", "legacy-upstream", model.BytePlusAssetStatusActive)
	insertAssetReferenceAsset(t, assetReferenceSeed{UserID: 7, PublicID: "ast_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", AssetType: "Image", SourceStatus: model.AssetSourceStatusUnavailable, BindingChannelID: 132, UpstreamID: "migrated-upstream", BindingStatus: model.AssetStatusActive})

	refs, apiErr := ResolveAssetReferences(newAssetReferenceContext(), 7, &dto.SeedanceVideoRequest{Content: []dto.SeedanceContentItem{
		imageAssetItem("ast_1234567890abcdefABCDEF1234567890"),
		imageAssetItem("ast_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
	}})
	require.Nil(t, apiErr)

	_, ok := refs.ReadinessForChannel(&model.Channel{Id: 131, Type: constant.ChannelTypeBytePlus})
	require.False(t, ok, "legacy and source-unavailable assets must require one common active binding")
	readiness, ok := refs.ReadinessForChannel(&model.Channel{Id: 132, Type: constant.ChannelTypeBytePlus})
	require.False(t, ok)
	require.Equal(t, AssetReadinessIneligible, readiness)
}

func TestResolveAssetReferencesRejectsInvalidLegacyRealPersonProfile(t *testing.T) {
	tests := []struct {
		name           string
		profileUserID  int
		profileChannel int
		profileStatus  string
		wantCode       types.ErrorCode
	}{
		{
			name:           "inactive profile",
			profileUserID:  7,
			profileChannel: 106,
			profileStatus:  model.BytePlusRealPersonProfileStatusExpired,
			wantCode:       types.ErrorCodeRealPersonNotActive,
		},
		{
			name:           "profile belongs to another user",
			profileUserID:  8,
			profileChannel: 106,
			profileStatus:  model.BytePlusRealPersonProfileStatusActive,
			wantCode:       types.ErrorCodeAssetNotFound,
		},
		{
			name:           "profile belongs to another channel",
			profileUserID:  7,
			profileChannel: 107,
			profileStatus:  model.BytePlusRealPersonProfileStatusActive,
			wantCode:       types.ErrorCodeAssetChannelConflict,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			newAssetReferenceDB(t)
			require.NoError(t, model.DB.AutoMigrate(&model.BytePlusRealPersonProfile{}))
			profile := model.BytePlusRealPersonProfile{
				PublicId:    "rph_invalid_worker_profile",
				UserId:      test.profileUserID,
				ChannelId:   test.profileChannel,
				Status:      test.profileStatus,
				CreatedTime: 100,
				UpdatedTime: 100,
			}
			require.NoError(t, model.DB.Create(&profile).Error)
			publicID := "ast_1234567890abcdefABCDEF1234567890"
			require.NoError(t, model.DB.Create(&model.BytePlusAsset{
				PublicId:            publicID,
				UserId:              7,
				RealPersonProfileId: &profile.Id,
				ChannelId:           106,
				UpstreamAssetId:     "tokenspace-person-asset",
				AssetType:           "Image",
				Status:              model.BytePlusAssetStatusActive,
			}).Error)

			refs, apiErr := ResolveAssetReferences(newAssetReferenceContext(), 7, &dto.SeedanceVideoRequest{
				Content: []dto.SeedanceContentItem{imageAssetItem(publicID)},
			})

			require.NotNil(t, apiErr)
			require.Equal(t, test.wantCode, apiErr.GetErrorCode())
			require.False(t, refs.HasReferences())
		})
	}
}

func TestAssetReferenceSetDoesNotTreatUnscopedLegacyBindingAsCredentialVerified(t *testing.T) {
	newAssetReferenceDB(t)
	assetNow = func() time.Time { return time.Unix(100, 0) }
	t.Cleanup(func() { assetNow = time.Now })
	insertAssetReferenceAsset(t, assetReferenceSeed{UserID: 7, PublicID: "ast_1234567890abcdefABCDEF1234567890", AssetType: "Image", SourceStatus: model.AssetSourceStatusAvailable, SourceExpiresAt: 200})
	insertLegacyAssetReferenceAsset(t, 7, 131, "ast_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "legacy-upstream", model.BytePlusAssetStatusActive)

	refs, apiErr := ResolveAssetReferences(newAssetReferenceContext(), 7, &dto.SeedanceVideoRequest{Content: []dto.SeedanceContentItem{
		imageAssetItem("ast_1234567890abcdefABCDEF1234567890"),
		imageAssetItem("ast_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
	}})
	require.Nil(t, apiErr)

	readiness, ok := refs.ReadinessForChannel(&model.Channel{Id: 131, Type: constant.ChannelTypeBytePlus, Key: structuredBytePlusKey()})
	require.False(t, ok)
	require.Equal(t, AssetReadinessIneligible, readiness)
	require.Equal(t, map[string]string{
		"asset://ast_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": "asset://legacy-upstream",
	}, refs.RewriteMapForChannel(131))

	readiness, ok = refs.ReadinessForChannel(&model.Channel{Id: 132, Type: constant.ChannelTypeBytePlus})
	require.False(t, ok)
	require.Equal(t, AssetReadinessIneligible, readiness)
	require.Nil(t, refs.RewriteMapForChannel(132))
}

func TestAssetReferenceRewriteMapPreservesOpaqueUpstreamURI(t *testing.T) {
	refs := AssetReferenceSet{
		references: []assetReference{{PublicID: "ast_opaque", ExpectedAssetType: "Image"}},
		assets: map[string]assetReferenceAsset{
			"ast_opaque": {
				PublicID:  "ast_opaque",
				AssetType: "Image",
				Status:    model.AssetStatusActive,
				Bindings: []assetReferenceBinding{{
					ChannelID:       106,
					Status:          model.AssetStatusActive,
					UpstreamAssetID: "asset://asset-opaque-123",
				}},
			},
		},
	}

	require.Equal(t, map[string]string{
		"asset://ast_opaque": "asset://asset-opaque-123",
	}, refs.RewriteMapForChannel(106))
}

func TestTechMobiReadinessRequiresOneKeyScopeToCoverEveryAsset(t *testing.T) {
	mapping := `{"seedance2.0-pro":"doubao/seedance-pro"}`
	channel := &model.Channel{
		Id:           106,
		Type:         constant.ChannelTypeTechMobiVideo,
		Key:          "techmobi-key-a\ntechmobi-key-b",
		ModelMapping: &mapping,
		ChannelInfo: model.ChannelInfo{
			IsMultiKey:   true,
			MultiKeySize: 2,
		},
	}
	scopeA, err := assetBindingScope(channel.Type, AssetMaterializeOptions{Model: "doubao/seedance-pro", APIKey: "techmobi-key-a"})
	require.NoError(t, err)
	scopeB, err := assetBindingScope(channel.Type, AssetMaterializeOptions{Model: "doubao/seedance-pro", APIKey: "techmobi-key-b"})
	require.NoError(t, err)

	refs := AssetReferenceSet{
		references: []assetReference{
			{PublicID: "ast_scope_a", ExpectedAssetType: "Image"},
			{PublicID: "ast_scope_b", ExpectedAssetType: "Image"},
		},
		assets: map[string]assetReferenceAsset{
			"ast_scope_a": {
				PublicID:     "ast_scope_a",
				AssetType:    "Image",
				Status:       model.AssetStatusActive,
				SourceStatus: model.AssetSourceStatusUnavailable,
				Bindings: []assetReferenceBinding{{
					ChannelID:       channel.Id,
					BindingScope:    scopeA,
					UpstreamAssetID: "asset-a",
					Status:          model.AssetStatusActive,
				}},
			},
			"ast_scope_b": {
				PublicID:     "ast_scope_b",
				AssetType:    "Image",
				Status:       model.AssetStatusActive,
				SourceStatus: model.AssetSourceStatusUnavailable,
				Bindings: []assetReferenceBinding{{
					ChannelID:       channel.Id,
					BindingScope:    scopeB,
					UpstreamAssetID: "asset-b",
					Status:          model.AssetStatusActive,
				}},
			},
		},
	}

	readiness, ok := refs.ReadinessForChannel(channel, "seedance2.0-pro")
	require.False(t, ok)
	require.Equal(t, AssetReadinessIneligible, readiness)
}

func TestResolveAssetMaterializeOptionsAllowsTokenSpaceLegacyRealPersonBindingWithoutStrictCoverage(t *testing.T) {
	channel := &model.Channel{
		Id:            106,
		Type:          constant.ChannelTypeDoubaoVideo,
		Key:           "tokenspace-key",
		Status:        common.ChannelStatusEnabled,
		OtherSettings: `{"asset_materialization":{"provider":"tokenspace_material","gateway_base_url":"https://materials.example.invalid","group_id":"group-aigc"}}`,
	}
	refs := AssetReferenceSet{
		references: []assetReference{{PublicID: "ast_real_person", ExpectedAssetType: "Image"}},
		assets: map[string]assetReferenceAsset{
			"ast_real_person": {
				PublicID:         "ast_real_person",
				AssetType:        "Image",
				Status:           model.AssetStatusActive,
				SourceStatus:     model.AssetSourceStatusUnavailable,
				LegacyBytePlus:   true,
				LegacyRealPerson: true,
				Bindings: []assetReferenceBinding{{
					ChannelID:       channel.Id,
					UpstreamAssetID: "tokenspace-person-asset",
					Status:          model.AssetStatusActive,
				}},
			},
		},
	}

	options, keyIndex, err := ResolveAssetMaterializeOptions(refs, channel, AssetMaterializeOptions{
		Model:  "seedance-2.0",
		APIKey: channel.Key,
	})

	require.NoError(t, err)
	require.Equal(t, AssetMaterializeOptions{Model: "seedance-2.0", APIKey: channel.Key}, options)
	require.Equal(t, 0, keyIndex)
}

func TestExplicitCredentialScopedProviderReadinessRequiresOneKeyScopeToCoverEveryAsset(t *testing.T) {
	mapping := `{"seedance-2.0-fast":"seedance-2.0-fast"}`
	channel := &model.Channel{
		Id:            156,
		Type:          constant.ChannelTypeBytePlus,
		Key:           "seedance-key-a\nseedance-key-b",
		ModelMapping:  &mapping,
		OtherSettings: `{"asset_materialization":{"provider":"seedance_proxy","gateway_base_url":"https://asset-gateway.example.invalid/v1","group_id":"grp_shared_aigc"}}`,
		ChannelInfo: model.ChannelInfo{
			IsMultiKey:   true,
			MultiKeySize: 2,
		},
	}
	scopeA, err := assetBindingScopeForChannel(channel, AssetMaterializeOptions{Model: "seedance-2.0-fast", APIKey: "seedance-key-a"})
	require.NoError(t, err)
	scopeB, err := assetBindingScopeForChannel(channel, AssetMaterializeOptions{Model: "seedance-2.0-fast", APIKey: "seedance-key-b"})
	require.NoError(t, err)

	refs := AssetReferenceSet{
		references: []assetReference{
			{PublicID: "ast_scope_a", ExpectedAssetType: "Image"},
			{PublicID: "ast_scope_b", ExpectedAssetType: "Image"},
		},
		assets: map[string]assetReferenceAsset{
			"ast_scope_a": {
				PublicID:     "ast_scope_a",
				AssetType:    "Image",
				Status:       model.AssetStatusActive,
				SourceStatus: model.AssetSourceStatusUnavailable,
				Bindings: []assetReferenceBinding{{
					ChannelID:       channel.Id,
					BindingScope:    scopeA,
					UpstreamAssetID: "asset-a",
					Status:          model.AssetStatusActive,
				}},
			},
			"ast_scope_b": {
				PublicID:     "ast_scope_b",
				AssetType:    "Image",
				Status:       model.AssetStatusActive,
				SourceStatus: model.AssetSourceStatusUnavailable,
				Bindings: []assetReferenceBinding{{
					ChannelID:       channel.Id,
					BindingScope:    scopeB,
					UpstreamAssetID: "asset-b",
					Status:          model.AssetStatusActive,
				}},
			},
		},
	}

	readiness, ok := refs.ReadinessForChannel(channel, "seedance-2.0-fast")
	require.False(t, ok)
	require.Equal(t, AssetReadinessIneligible, readiness)
	require.Equal(t, map[string]string{
		"asset://ast_scope_a": "asset://asset-a",
	}, refs.RewriteMapForSelectedChannel(channel, "seedance-2.0-fast", "seedance-key-a"))
	require.Equal(t, map[string]string{
		"asset://ast_scope_b": "asset://asset-b",
	}, refs.RewriteMapForSelectedChannel(channel, "seedance-2.0-fast", "seedance-key-b"))
}

func TestNativeBytePlusCredentialScopeControlsReadinessResolutionAndRewrite(t *testing.T) {
	keyA := structuredBytePlusKeyWith("video-a", "access-a", "secret-a", "project-a")
	keyB := structuredBytePlusKeyWith("video-b", "access-b", "secret-b", "project-b")
	channel := &model.Channel{
		Id:     157,
		Type:   constant.ChannelTypeBytePlus,
		Key:    "[" + keyA + "," + keyB + "]",
		Status: common.ChannelStatusEnabled,
		ChannelInfo: model.ChannelInfo{
			IsMultiKey:   true,
			MultiKeySize: 2,
		},
	}
	scopeA, err := assetBindingScopeForChannel(channel, AssetMaterializeOptions{Model: "seedance-2.0-fast", APIKey: keyA})
	require.NoError(t, err)
	scopeB, err := assetBindingScopeForChannel(channel, AssetMaterializeOptions{Model: "seedance-2.0-fast", APIKey: keyB})
	require.NoError(t, err)
	require.NotEqual(t, scopeA, scopeB)

	refs := AssetReferenceSet{
		references: []assetReference{
			{PublicID: "ast_byteplus_one", ExpectedAssetType: "Image"},
			{PublicID: "ast_byteplus_two", ExpectedAssetType: "Video"},
		},
		assets: map[string]assetReferenceAsset{
			"ast_byteplus_one": {
				PublicID:     "ast_byteplus_one",
				AssetType:    "Image",
				Status:       model.AssetStatusActive,
				SourceStatus: model.AssetSourceStatusUnavailable,
				Bindings: []assetReferenceBinding{{
					ChannelID:       channel.Id,
					BindingScope:    scopeB,
					UpstreamAssetID: "byteplus-one",
					Status:          model.AssetStatusActive,
				}},
			},
			"ast_byteplus_two": {
				PublicID:     "ast_byteplus_two",
				AssetType:    "Video",
				Status:       model.AssetStatusActive,
				SourceStatus: model.AssetSourceStatusUnavailable,
				Bindings: []assetReferenceBinding{{
					ChannelID:       channel.Id,
					BindingScope:    scopeB,
					UpstreamAssetID: "byteplus-two",
					Status:          model.AssetStatusActive,
				}},
			},
		},
	}

	readiness, ok := refs.ReadinessForChannel(channel, "seedance-2.0-fast")
	require.True(t, ok)
	require.Equal(t, AssetReadinessAllBound, readiness)
	require.Nil(t, refs.RewriteMapForSelectedChannel(channel, "seedance-2.0-fast", keyA))

	options, keyIndex, err := ResolveAssetMaterializeOptions(refs, channel, AssetMaterializeOptions{
		Model:  "seedance-2.0-fast",
		APIKey: keyA,
	})
	require.NoError(t, err)
	require.Equal(t, keyB, options.APIKey)
	require.Equal(t, 1, keyIndex)
	require.Equal(t, map[string]string{
		"asset://ast_byteplus_one": "asset://byteplus-one",
		"asset://ast_byteplus_two": "asset://byteplus-two",
	}, refs.RewriteMapForSelectedChannel(channel, "seedance-2.0-fast", options.APIKey))

	refs.assets["ast_byteplus_one"] = assetReferenceAsset{
		PublicID:     "ast_byteplus_one",
		AssetType:    "Image",
		Status:       model.AssetStatusActive,
		SourceStatus: model.AssetSourceStatusUnavailable,
		Bindings: []assetReferenceBinding{{
			ChannelID:       channel.Id,
			BindingScope:    scopeA,
			UpstreamAssetID: "byteplus-one-a",
			Status:          model.AssetStatusActive,
		}},
	}
	readiness, ok = refs.ReadinessForChannel(channel, "seedance-2.0-fast")
	require.False(t, ok)
	require.Equal(t, AssetReadinessIneligible, readiness)
}

func TestTokenSpaceMaterialReadyChannelResolvesSelectedCredentialAndRewriteMap(t *testing.T) {
	mapping := `{"seedance-2.0-fast":"doubao/seedance-pro"}`
	channel := &model.Channel{
		Id:            160,
		Type:          constant.ChannelTypeTechMobiVideo,
		Key:           "tokenspace-key-a\ntokenspace-key-b",
		ModelMapping:  &mapping,
		OtherSettings: `{"asset_materialization":{"provider":"tokenspace_material","gateway_base_url":"https://materials.example.invalid","group_id":"group-internal"}}`,
		ChannelInfo: model.ChannelInfo{
			IsMultiKey:   true,
			MultiKeySize: 2,
		},
	}
	scopeB, err := assetBindingScopeForChannel(channel, AssetMaterializeOptions{Model: "doubao/seedance-pro", APIKey: "tokenspace-key-b"})
	require.NoError(t, err)
	refs := AssetReferenceSet{
		references: []assetReference{
			{PublicID: "ast_tokenspace_one", ExpectedAssetType: "Image"},
			{PublicID: "ast_tokenspace_two", ExpectedAssetType: "Video"},
		},
		assets: map[string]assetReferenceAsset{
			"ast_tokenspace_one": {
				PublicID:     "ast_tokenspace_one",
				AssetType:    "Image",
				Status:       model.AssetStatusActive,
				SourceStatus: model.AssetSourceStatusUnavailable,
				Bindings: []assetReferenceBinding{{
					ChannelID:       channel.Id,
					BindingScope:    scopeB,
					UpstreamAssetID: "asset-one",
					Status:          model.AssetStatusActive,
				}},
			},
			"ast_tokenspace_two": {
				PublicID:     "ast_tokenspace_two",
				AssetType:    "Video",
				Status:       model.AssetStatusActive,
				SourceStatus: model.AssetSourceStatusUnavailable,
				Bindings: []assetReferenceBinding{{
					ChannelID:       channel.Id,
					BindingScope:    scopeB,
					UpstreamAssetID: "asset-two",
					Status:          model.AssetStatusActive,
				}},
			},
		},
	}

	readiness, ok := refs.ReadinessForChannel(channel, "seedance-2.0-fast")
	require.True(t, ok)
	require.Equal(t, AssetReadinessAllBound, readiness)

	options, keyIndex, err := ResolveAssetMaterializeOptions(refs, channel, AssetMaterializeOptions{
		Model:  "doubao/seedance-pro",
		APIKey: "tokenspace-key-a",
	})
	require.NoError(t, err)
	require.Equal(t, AssetMaterializeOptions{Model: "doubao/seedance-pro", APIKey: "tokenspace-key-b"}, options)
	require.Equal(t, 1, keyIndex)
	require.Equal(t, map[string]string{
		"asset://ast_tokenspace_one": "asset://asset-one",
		"asset://ast_tokenspace_two": "asset://asset-two",
	}, refs.RewriteMapForSelectedChannel(channel, "seedance-2.0-fast", options.APIKey))
}

// TechMobi readiness promises "some enabled key covers every reference". This
// locks in that the promise is actually redeemable: resolving the scope from an
// arbitrarily routed key must land on the key that holds the bindings, so the
// rewrite map is never silently dropped for a channel we already ranked ready.
func TestTechMobiReadyChannelRewritesEveryAssetForSomeEnabledKey(t *testing.T) {
	mapping := `{"seedance2.0-pro":"doubao/seedance-pro"}`
	channel := &model.Channel{
		Id:           106,
		Type:         constant.ChannelTypeTechMobiVideo,
		Key:          "techmobi-key-a\ntechmobi-key-b",
		ModelMapping: &mapping,
		ChannelInfo: model.ChannelInfo{
			IsMultiKey:   true,
			MultiKeySize: 2,
		},
	}
	scopeB, err := assetBindingScope(channel.Type, AssetMaterializeOptions{Model: "doubao/seedance-pro", APIKey: "techmobi-key-b"})
	require.NoError(t, err)

	// Both assets are bound only under key B, so the channel is legitimately ready.
	refs := AssetReferenceSet{
		references: []assetReference{
			{PublicID: "ast_only_b_one", ExpectedAssetType: "Image"},
			{PublicID: "ast_only_b_two", ExpectedAssetType: "Image"},
		},
		assets: map[string]assetReferenceAsset{
			"ast_only_b_one": {
				PublicID:     "ast_only_b_one",
				AssetType:    "Image",
				Status:       model.AssetStatusActive,
				SourceStatus: model.AssetSourceStatusUnavailable,
				Bindings: []assetReferenceBinding{{
					ChannelID:       channel.Id,
					BindingScope:    scopeB,
					UpstreamAssetID: "asset-b-one",
					Status:          model.AssetStatusActive,
				}},
			},
			"ast_only_b_two": {
				PublicID:     "ast_only_b_two",
				AssetType:    "Image",
				Status:       model.AssetStatusActive,
				SourceStatus: model.AssetSourceStatusUnavailable,
				Bindings: []assetReferenceBinding{{
					ChannelID:       channel.Id,
					BindingScope:    scopeB,
					UpstreamAssetID: "asset-b-two",
					Status:          model.AssetStatusActive,
				}},
			},
		},
	}

	readiness, ok := refs.ReadinessForChannel(channel, "seedance2.0-pro")
	require.True(t, ok)
	require.Equal(t, AssetReadinessAllBound, readiness)

	// Routing may land on key A, which holds no binding at all. Readiness promised
	// the channel can serve this request, so resolving the scope must land on the
	// key that actually covers every reference instead of silently dropping it.
	options, keyIndex, err := ResolveAssetMaterializeOptions(refs, channel, AssetMaterializeOptions{
		Model:  "doubao/seedance-pro",
		APIKey: "techmobi-key-a",
	})
	require.NoError(t, err)
	require.Equal(t, "techmobi-key-b", options.APIKey)
	require.Equal(t, 1, keyIndex)

	require.Equal(t, map[string]string{
		"asset://ast_only_b_one": "asset://asset-b-one",
		"asset://ast_only_b_two": "asset://asset-b-two",
	}, refs.RewriteMapForSelectedChannel(channel, "seedance2.0-pro", options.APIKey))
}

func TestAssetReferenceVerifiedTargetOutranksOrdinaryBindings(t *testing.T) {
	targetChannel := &model.Channel{Id: 120, Type: constant.ChannelTypeTechMobiVideo}
	otherChannel := &model.Channel{Id: 106, Type: constant.ChannelTypeBytePlus}
	scope := AssetModelScope{ScopeKey: "scope-a", Groups: []string{"default"}, ModelNames: []string{"seedance-2.0-fast"}}
	target := model.AssetModelCoverageTarget{
		ScopeKey:      scope.ScopeKey,
		ModelName:     "seedance-2.0-fast",
		RoutingGroups: assetModelRoutingGroups(scope.Groups),
		ChannelId:     targetChannel.Id,
		MappedModel:   "seedance-2.0-fast",
		BindingScope:  "target-scope",
		Generation:    3,
		Status:        model.AssetModelTargetStatusActive,
	}
	refs := AssetReferenceSet{
		strictCoverage: true,
		scope:          scope,
		target:         &target,
		readinessByPublicID: map[string]model.AssetModelReadiness{
			"ast_asset_a": {
				AssetId:          11,
				ScopeKey:         scope.ScopeKey,
				ModelName:        target.ModelName,
				TargetGeneration: target.Generation,
				ChannelId:        target.ChannelId,
				BindingScope:     target.BindingScope,
				Status:           model.AssetModelReadinessStatusActive,
			},
			"ast_asset_b": {
				AssetId:          12,
				ScopeKey:         scope.ScopeKey,
				ModelName:        target.ModelName,
				TargetGeneration: target.Generation,
				ChannelId:        target.ChannelId,
				BindingScope:     target.BindingScope,
				Status:           model.AssetModelReadinessStatusActive,
			},
		},
		references: []assetReference{
			{PublicID: "ast_asset_a", ExpectedAssetType: "Image"},
			{PublicID: "ast_asset_b", ExpectedAssetType: "Image"},
		},
		assets: map[string]assetReferenceAsset{
			"ast_asset_a": {
				ID:        11,
				PublicID:  "ast_asset_a",
				AssetType: "Image",
				Status:    model.AssetStatusActive,
				Bindings: []assetReferenceBinding{
					{ChannelID: otherChannel.Id, UpstreamAssetID: "other-a", Status: model.AssetStatusActive},
					{ChannelID: targetChannel.Id, BindingScope: target.BindingScope, UpstreamAssetID: "upstream-a-target", Status: model.AssetStatusActive},
				},
			},
			"ast_asset_b": {
				ID:        12,
				PublicID:  "ast_asset_b",
				AssetType: "Image",
				Status:    model.AssetStatusActive,
				Bindings: []assetReferenceBinding{
					{ChannelID: otherChannel.Id, UpstreamAssetID: "other-b", Status: model.AssetStatusActive},
					{ChannelID: targetChannel.Id, BindingScope: target.BindingScope, UpstreamAssetID: "upstream-b-target", Status: model.AssetStatusActive},
				},
			},
		},
	}

	readiness, eligible := refs.ReadinessForChannel(targetChannel, "seedance-2.0-fast")
	require.True(t, eligible)
	require.Equal(t, AssetReadinessVerifiedTarget, readiness)

	readiness, eligible = refs.ReadinessForChannel(otherChannel, "seedance-2.0-fast")
	require.False(t, eligible)
	require.Equal(t, AssetReadinessIneligible, readiness)

	rewrite := refs.RewriteMapForSelectedChannel(targetChannel, "seedance-2.0-fast", "ignored-key")
	require.Equal(t, map[string]string{
		"asset://ast_asset_a": "asset://upstream-a-target",
		"asset://ast_asset_b": "asset://upstream-b-target",
	}, rewrite)
}

func TestAssetReferenceStaleVerifiedTargetIsRecoverableAndPinsTargetChannel(t *testing.T) {
	scope := AssetModelScope{ScopeKey: "scope-a", Groups: []string{"default"}, ModelNames: []string{"seedance-2.0-fast"}}
	target := model.AssetModelCoverageTarget{
		ScopeKey:      scope.ScopeKey,
		ModelName:     "seedance-2.0-fast",
		RoutingGroups: assetModelRoutingGroups(scope.Groups),
		ChannelId:     120,
		MappedModel:   "seedance-2.0-fast",
		BindingScope:  "target-scope",
		Generation:    4,
		Status:        model.AssetModelTargetStatusActive,
	}
	refs := AssetReferenceSet{
		strictCoverage: true,
		scope:          scope,
		target:         &target,
		readinessByPublicID: map[string]model.AssetModelReadiness{
			"ast_asset_a": {
				AssetId:          11,
				ScopeKey:         scope.ScopeKey,
				ModelName:        target.ModelName,
				TargetGeneration: 3,
				ChannelId:        target.ChannelId,
				BindingScope:     target.BindingScope,
				Status:           model.AssetModelReadinessStatusActive,
			},
		},
		references: []assetReference{{PublicID: "ast_asset_a", ExpectedAssetType: "Image"}},
		assets: map[string]assetReferenceAsset{
			"ast_asset_a": {
				ID:              11,
				PublicID:        "ast_asset_a",
				AssetType:       "Image",
				Status:          model.AssetStatusActive,
				SourceStatus:    model.AssetSourceStatusAvailable,
				StorageBackend:  "gcs",
				StorageBucket:   "bucket",
				ObjectKey:       "assets/ast_asset_a",
				SourceExpiresAt: time.Now().Add(time.Hour).Unix(),
				Bindings: []assetReferenceBinding{{
					ChannelID:       106,
					UpstreamAssetID: "ordinary-binding",
					Status:          model.AssetStatusActive,
				}},
			},
		},
	}

	readiness, eligible := refs.ReadinessForChannel(&model.Channel{Id: 120, Type: constant.ChannelTypeTechMobiVideo}, "seedance-2.0-fast")
	require.True(t, eligible)
	require.Equal(t, AssetReadinessRecoverable, readiness)

	readiness, eligible = refs.ReadinessForChannel(&model.Channel{Id: 106, Type: constant.ChannelTypeBytePlus}, "seedance-2.0-fast")
	require.False(t, eligible)
	require.Equal(t, AssetReadinessIneligible, readiness)
}

func TestAssetReferencePinnedTargetOptionsUseCredentialIndexAndMappedModel(t *testing.T) {
	channel := &model.Channel{
		Id:     120,
		Type:   constant.ChannelTypeTechMobiVideo,
		Key:    "techmobi-key-a\ntechmobi-key-b",
		Status: common.ChannelStatusEnabled,
		ChannelInfo: model.ChannelInfo{
			IsMultiKey:   true,
			MultiKeySize: 2,
		},
	}
	targetScope, err := assetBindingScope(channel.Type, AssetMaterializeOptions{Model: "doubao/seedance-pro", APIKey: "techmobi-key-b"})
	require.NoError(t, err)
	scope := AssetModelScope{ScopeKey: "scope-a", Groups: []string{"default"}, ModelNames: []string{"seedance-2.0-fast"}}
	target := model.AssetModelCoverageTarget{
		ScopeKey:        scope.ScopeKey,
		ModelName:       "seedance-2.0-fast",
		RoutingGroups:   assetModelRoutingGroups(scope.Groups),
		ChannelId:       channel.Id,
		MappedModel:     "doubao/seedance-pro",
		BindingScope:    targetScope,
		CredentialIndex: 1,
		Generation:      3,
		Status:          model.AssetModelTargetStatusActive,
	}
	refs := AssetReferenceSet{
		strictCoverage: true,
		scope:          scope,
		target:         &target,
		references:     []assetReference{{PublicID: "ast_asset_a", ExpectedAssetType: "Image"}},
		assets: map[string]assetReferenceAsset{
			"ast_asset_a": {ID: 11, PublicID: "ast_asset_a", AssetType: "Image", Status: model.AssetStatusActive},
		},
	}

	options, keyIndex, err := ResolveAssetMaterializeOptions(refs, channel, AssetMaterializeOptions{
		Model:  "wrong-model",
		APIKey: "techmobi-key-a",
	})
	require.NoError(t, err)
	require.Equal(t, AssetMaterializeOptions{Model: "doubao/seedance-pro", APIKey: "techmobi-key-b"}, options)
	require.Equal(t, 1, keyIndex)

	channel.Key = "techmobi-key-a\nrotated-key-b"
	_, _, err = ResolveAssetMaterializeOptions(refs, channel, AssetMaterializeOptions{
		Model:  "doubao/seedance-pro",
		APIKey: "rotated-key-b",
	})
	require.ErrorIs(t, err, ErrAssetBindingUnavailable)
}

func TestAssetReferenceStrictCoverageWithoutTargetDoesNotFallBackToLegacyBindings(t *testing.T) {
	channel := &model.Channel{
		Id:     120,
		Type:   constant.ChannelTypeTechMobiVideo,
		Key:    "techmobi-key-a",
		Status: common.ChannelStatusEnabled,
	}
	refs := AssetReferenceSet{
		strictCoverage: true,
		references:     []assetReference{{PublicID: "ast_asset_a", ExpectedAssetType: "Image"}},
		assets: map[string]assetReferenceAsset{
			"ast_asset_a": {
				ID:        11,
				PublicID:  "ast_asset_a",
				AssetType: "Image",
				Status:    model.AssetStatusActive,
				Bindings: []assetReferenceBinding{{
					ChannelID:       channel.Id,
					BindingScope:    "legacy-scope",
					UpstreamAssetID: "legacy-upstream",
					Status:          model.AssetStatusActive,
				}},
			},
		},
	}

	readiness, eligible := refs.ReadinessForChannel(channel, "seedance-2.0-fast")
	require.False(t, eligible)
	require.Equal(t, AssetReadinessIneligible, readiness)
	require.Nil(t, refs.RewriteMapForSelectedChannel(channel, "seedance-2.0-fast", channel.Key))
	_, _, err := ResolveAssetMaterializeOptions(refs, channel, AssetMaterializeOptions{Model: "seedance-2.0-fast", APIKey: channel.Key})
	require.ErrorIs(t, err, ErrAssetBindingInitializing)
}

func TestAssetReferenceSetRejectsMixedSourceUnavailableBindingsOnDifferentChannels(t *testing.T) {
	newAssetReferenceDB(t)
	insertAssetReferenceAsset(t, assetReferenceSeed{UserID: 7, PublicID: "ast_1234567890abcdefABCDEF1234567890", AssetType: "Image", SourceStatus: model.AssetSourceStatusUnavailable, BindingChannelID: 131, UpstreamID: "generalized-upstream", BindingStatus: model.AssetStatusActive})
	insertLegacyAssetReferenceAsset(t, 7, 132, "ast_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "legacy-upstream", model.BytePlusAssetStatusActive)

	refs, apiErr := ResolveAssetReferences(newAssetReferenceContext(), 7, &dto.SeedanceVideoRequest{Content: []dto.SeedanceContentItem{
		imageAssetItem("ast_1234567890abcdefABCDEF1234567890"),
		imageAssetItem("ast_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
	}})
	require.Nil(t, apiErr)

	readiness, ok := refs.ReadinessForChannel(&model.Channel{Id: 131, Type: constant.ChannelTypeBytePlus})
	require.False(t, ok)
	require.Equal(t, AssetReadinessIneligible, readiness)
	readiness, ok = refs.ReadinessForChannel(&model.Channel{Id: 132, Type: constant.ChannelTypeBytePlus})
	require.False(t, ok)
	require.Equal(t, AssetReadinessIneligible, readiness)
}

func TestAssetReferenceSetRequiresOneChannelToConsumeEveryReferencedAsset(t *testing.T) {
	newAssetReferenceDB(t)
	assetNow = func() time.Time { return time.Unix(100, 0) }
	t.Cleanup(func() { assetNow = time.Now })
	bytePlus := &model.Channel{Id: 131, Type: constant.ChannelTypeBytePlus, Key: structuredBytePlusKey()}
	bindingScope, err := assetBindingScopeForChannel(bytePlus, AssetMaterializeOptions{APIKey: bytePlus.Key})
	require.NoError(t, err)
	insertAssetReferenceAsset(t, assetReferenceSeed{UserID: 7, PublicID: "ast_1234567890abcdefABCDEF1234567890", AssetType: "Image", SourceStatus: model.AssetSourceStatusAvailable, SourceExpiresAt: 200, BindingChannelID: 131, BindingScope: bindingScope, UpstreamID: "image-upstream", BindingStatus: model.AssetStatusActive})
	insertAssetReferenceAsset(t, assetReferenceSeed{UserID: 7, PublicID: "ast_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", AssetType: "Audio", SourceStatus: model.AssetSourceStatusAvailable, SourceExpiresAt: 200})

	refs, apiErr := ResolveAssetReferences(newAssetReferenceContext(), 7, &dto.SeedanceVideoRequest{Content: []dto.SeedanceContentItem{
		imageAssetItem("ast_1234567890abcdefABCDEF1234567890"),
		audioAssetItem("ast_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
	}})
	require.Nil(t, apiErr)

	_, ok := refs.ReadinessForChannel(&model.Channel{Id: 131, Type: constant.ChannelTypeOpenAI})
	require.False(t, ok, "OpenAI video route must not be eligible for audio assets")
	readiness, ok := refs.ReadinessForChannel(bytePlus)
	require.True(t, ok)
	require.Equal(t, AssetReadinessPartialBound, readiness)
}

func TestExplicitUnknownAssetMaterializationProviderFailsClosedForAllAssetTypes(t *testing.T) {
	channel := &model.Channel{
		Type:          constant.ChannelTypeBytePlus,
		OtherSettings: `{"asset_materialization":{"provider":"future_provider","gateway_base_url":"https://gateway.example.invalid","group_id":"group-1"}}`,
	}

	require.False(t, channelCanConsumeAssetType(channel, "Image"))
	require.False(t, channelCanConsumeAssetType(channel, "Video"))
	require.False(t, channelCanConsumeAssetType(channel, "Audio"))
}

func TestTokenSpaceMaterialConfiguredAssetTypeAllowsImageVideoAndAudio(t *testing.T) {
	tokenSpaceChannel := &model.Channel{
		Type:          constant.ChannelTypeTechMobiVideo,
		OtherSettings: `{"asset_materialization":{"provider":"tokenspace_material","gateway_base_url":"https://materials.example.invalid","group_id":"group-internal"}}`,
	}

	require.True(t, channelCanConsumeAssetType(tokenSpaceChannel, "Image"))
	require.True(t, channelCanConsumeAssetType(tokenSpaceChannel, "Video"))
	require.True(t, channelCanConsumeAssetType(tokenSpaceChannel, "Audio"))
}

func TestSeedanceProxyCapabilitySupportsAudio(t *testing.T) {
	seedanceProxyChannel := &model.Channel{
		Type:          constant.ChannelTypeBytePlus,
		OtherSettings: `{"asset_materialization":{"provider":"seedance_proxy","gateway_base_url":"https://asset-gateway.example.invalid","group_id":"grp_shared_aigc"}}`,
	}

	require.True(t, channelCanConsumeAssetType(seedanceProxyChannel, "Image"))
	require.True(t, channelCanConsumeAssetType(seedanceProxyChannel, "Video"))
	require.True(t, channelCanConsumeAssetType(seedanceProxyChannel, "Audio"))
}

func TestTokenSpaceMaterialIncompleteConfigurationFailsClosedForAllAssetTypes(t *testing.T) {
	channel := &model.Channel{
		Type:          constant.ChannelTypeTechMobiVideo,
		OtherSettings: `{"asset_materialization":{"provider":"tokenspace_material","gateway_base_url":"https://materials.example.invalid"}}`,
	}

	require.False(t, channelCanConsumeAssetType(channel, "Image"))
	require.False(t, channelCanConsumeAssetType(channel, "Video"))
	require.False(t, channelCanConsumeAssetType(channel, "Audio"))
}

func TestExplicitInvalidSeedanceProxyConfigurationFailsClosedForAllAssetTypes(t *testing.T) {
	channel := &model.Channel{
		Type:          constant.ChannelTypeModelAPISeedance,
		OtherSettings: `{"asset_materialization":{"provider":"seedance_proxy","gateway_base_url":"http://gateway.example.invalid","group_id":"group-1"}}`,
	}

	require.False(t, channelCanConsumeAssetType(channel, "Image"))
	require.False(t, channelCanConsumeAssetType(channel, "Video"))
	require.False(t, channelCanConsumeAssetType(channel, "Audio"))
}

func TestCacheGetRandomSatisfiedChannelAssetRankerReadinessOutranksPriority(t *testing.T) {
	newAssetReferenceDB(t)
	assetNow = func() time.Time { return time.Unix(100, 0) }
	t.Cleanup(func() { assetNow = time.Now })
	restoreConcurrency := useMemoryChannelConcurrencyForTest(t)
	defer restoreConcurrency()

	insertAssetReferenceChannel(t, 131, constant.ChannelTypeBytePlus, "default", "seedance-2.0", 10, 1)
	insertAssetReferenceChannel(t, 132, constant.ChannelTypeBytePlus, "default", "seedance-2.0", 100, 1000)
	insertAssetReferenceAsset(t, assetReferenceSeed{UserID: 7, PublicID: "ast_1234567890abcdefABCDEF1234567890", AssetType: "Image", SourceStatus: model.AssetSourceStatusAvailable, SourceExpiresAt: 200, BindingChannelID: 131, UpstreamID: "bound-upstream", BindingStatus: model.AssetStatusActive})
	model.InitChannelCache()

	refs, apiErr := ResolveAssetReferences(newAssetReferenceContext(), 7, &dto.SeedanceVideoRequest{Content: []dto.SeedanceContentItem{imageAssetItem("ast_1234567890abcdefABCDEF1234567890")}})
	require.Nil(t, apiErr)
	c := newAssetReferenceContext()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	retry := 0

	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:           c,
		TokenGroup:    "default",
		ModelName:     "seedance-2.0",
		Retry:         &retry,
		ChannelRanker: refs.ChannelRanker(),
	})
	defer ReleaseChannelConcurrencyForContext(c)

	require.NoError(t, err)
	require.Equal(t, "default", selectedGroup)
	require.NotNil(t, channel)
	require.Equal(t, 131, channel.Id)
}

type assetReferenceSeed struct {
	UserID           int
	PublicID         string
	AssetType        string
	SourceStatus     string
	SourceExpiresAt  int64
	BindingChannelID int
	BindingScope     string
	UpstreamID       string
	BindingStatus    string
}

func newAssetReferenceDB(t *testing.T) {
	t.Helper()
	oldDB := model.DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Asset{}, &model.AssetBinding{}, &model.BytePlusAsset{}, &model.Channel{}, &model.Ability{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	model.DB = db
	common.MemoryCacheEnabled = true
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	t.Cleanup(func() {
		model.DB = oldDB
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		model.InitChannelCache()
		_ = sqlDB.Close()
	})
}

func insertAssetReferenceAsset(t *testing.T, seed assetReferenceSeed) model.Asset {
	t.Helper()
	asset := model.Asset{
		PublicId:        seed.PublicID,
		UserId:          seed.UserID,
		AssetType:       seed.AssetType,
		Status:          model.AssetStatusActive,
		SourceStatus:    seed.SourceStatus,
		StorageBackend:  "gcs",
		StorageBucket:   "bucket",
		ObjectKey:       "assets/" + seed.PublicID,
		SourceExpiresAt: seed.SourceExpiresAt,
		CreatedAt:       100,
		UpdatedAt:       100,
	}
	require.NoError(t, model.DB.Create(&asset).Error)
	if seed.BindingChannelID > 0 {
		status := seed.BindingStatus
		if status == "" {
			status = model.AssetStatusActive
		}
		require.NoError(t, model.DB.Create(&model.AssetBinding{
			AssetId:         asset.Id,
			ChannelId:       seed.BindingChannelID,
			BindingScope:    seed.BindingScope,
			UpstreamAssetId: seed.UpstreamID,
			Status:          status,
			CreatedAt:       100,
			UpdatedAt:       100,
		}).Error)
	}
	return asset
}

func insertLegacyAssetReferenceAsset(t *testing.T, userID, channelID int, publicID, upstreamID, status string) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.BytePlusAsset{
		PublicId:        publicID,
		UserId:          userID,
		ChannelId:       channelID,
		UpstreamAssetId: upstreamID,
		AssetType:       "Image",
		Status:          status,
	}).Error)
}

func insertAssetReferenceChannel(t *testing.T, id int, channelType int, group string, modelName string, priority int64, weight uint) {
	t.Helper()
	ch := &model.Channel{
		Id:       id,
		Type:     channelType,
		Key:      fmt.Sprintf("sk-%d", id),
		Status:   common.ChannelStatusEnabled,
		Name:     fmt.Sprintf("asset-channel-%d", id),
		Group:    group,
		Models:   modelName,
		Priority: &priority,
		Weight:   &weight,
	}
	require.NoError(t, model.DB.Create(ch).Error)
	require.NoError(t, ch.AddAbilities(nil))
}

func imageAssetItem(publicID string) dto.SeedanceContentItem {
	return dto.SeedanceContentItem{Type: dto.SeedanceContentImage, ImageURL: &dto.SeedanceURLObject{URL: "asset://" + publicID}}
}

func audioAssetItem(publicID string) dto.SeedanceContentItem {
	return dto.SeedanceContentItem{Type: dto.SeedanceContentAudio, AudioURL: &dto.SeedanceURLObject{URL: "asset://" + publicID}}
}
