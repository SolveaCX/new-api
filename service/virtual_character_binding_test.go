package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

type virtualCharacterBindingRecorder struct {
	creates, gets int
	ready         bool
}

func (r *virtualCharacterBindingRecorder) CreateAsset(context.Context, AssetMaterializeInput) (AssetMaterializeResult, error) {
	r.creates++
	return AssetMaterializeResult{UpstreamAssetID: "42", Status: model.AssetStatusProcessing}, nil
}

func (r *virtualCharacterBindingRecorder) GetAsset(_ context.Context, _ AssetMaterializeInput, id string) (AssetMaterializeResult, error) {
	r.gets++
	result := AssetMaterializeResult{UpstreamAssetID: id, Status: model.AssetStatusProcessing}
	if r.ready {
		result.Status = model.AssetStatusActive
		result.UpstreamGroupID = "aigc-generation-reference"
	}
	return result, nil
}

func virtualCharacterBindingTestChannel() *model.Channel {
	return &model.Channel{
		Id: 272, Type: constant.ChannelTypeDoubaoVideo, Status: common.ChannelStatusEnabled,
		Key: "material-key-a", Models: "seedance-2.0,seedance-2.5", Group: "default",
		OtherSettings: `{"asset_materialization":{"provider":"virtual_character","gateway_base_url":"https://materials.example.invalid"}}`,
	}
}

func TestVirtualCharacterBindingPersistsManagementIDAndResumesWithoutUpload(t *testing.T) {
	newAssetServiceTestDB(t)
	installAssetServiceTestDeps(t)
	oldStrict := AssetModelCoverageStrictEnabled
	AssetModelCoverageStrictEnabled = false
	t.Cleanup(func() { AssetModelCoverageStrictEnabled = oldStrict })
	asset := insertMaterializeAsset(t, "ast_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	channel := virtualCharacterBindingTestChannel()
	recorder := &virtualCharacterBindingRecorder{}
	original := assetMaterializationProviderDescriptors[assetMaterializationProviderVirtualCharacter]
	replacement := original
	replacement.MaterializerFactory = func(assetMaterializationChannelConfig) AssetMaterializer { return recorder }
	assetMaterializationProviderDescriptors[assetMaterializationProviderVirtualCharacter] = replacement
	t.Cleanup(func() {
		assetMaterializationProviderDescriptors[assetMaterializationProviderVirtualCharacter] = original
	})
	request := AssetBindingRequest{
		UserID: asset.UserId, PublicID: asset.PublicId, Channel: channel, LeaseOwner: "first-node",
		PollLimit: 1, LeaseTTL: time.Minute, ExpectedType: "Image", Model: "seedance-2.0", APIKey: channel.Key,
	}
	_, err := MaterializeAssetBinding(context.Background(), request)
	require.ErrorIs(t, err, ErrAssetBindingInitializing)
	scope, err := assetBindingScopeForChannel(channel, AssetMaterializeOptions{APIKey: channel.Key, Model: request.Model})
	require.NoError(t, err)
	stored, err := model.GetAssetBindingForScope(asset.Id, channel.Id, scope)
	require.NoError(t, err)
	require.Equal(t, "42", stored.UpstreamAssetId)
	require.Empty(t, stored.UpstreamGroupId)
	require.Equal(t, model.AssetStatusProcessing, stored.Status)
	require.Empty(t, assetBindingResult(asset.PublicId, *stored).RewriteURI)

	// A new worker invocation can resume using the persisted management ID.
	recorder.ready = true
	request.LeaseOwner = "second-node"
	result, err := MaterializeAssetBinding(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, "42", result.Binding.UpstreamAssetId)
	require.Equal(t, "aigc-generation-reference", result.Binding.UpstreamGroupId)
	require.Equal(t, "asset://aigc-generation-reference", result.RewriteURI)
	require.Equal(t, 1, recorder.creates)
	require.Equal(t, 2, recorder.gets)

	// Different Seedance models using this credential reuse the same binding.
	request.Model = "seedance-2.5"
	result, err = MaterializeAssetBinding(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, "asset://aigc-generation-reference", result.RewriteURI)
	require.Equal(t, 1, recorder.creates)
	require.Equal(t, 2, recorder.gets)

	// The DB-to-reference projection must preserve the separate generation ID.
	refs, apiErr := ResolveAssetReferences(nil, asset.UserId, &dto.SeedanceVideoRequest{
		Model: "seedance-2.0", Content: []dto.SeedanceContentItem{imageAssetItem(asset.PublicId)},
	})
	require.Nil(t, apiErr)
	expected := map[string]string{"asset://" + asset.PublicId: "asset://aigc-generation-reference"}
	require.Equal(t, expected, refs.RewriteMapForSelectedChannel(channel, request.Model, channel.Key))
	rewritten, err := MaterializeAssetBindingsForChannel(context.Background(), asset.UserId, refs, channel,
		AssetMaterializeOptions{APIKey: channel.Key, Model: request.Model})
	require.NoError(t, err)
	require.Equal(t, expected, rewritten)
}

func TestVirtualCharacterBindingRejectsIncompleteActiveReference(t *testing.T) {
	channel := virtualCharacterBindingTestChannel()
	scope, err := assetBindingScopeForChannel(channel, AssetMaterializeOptions{APIKey: channel.Key})
	require.NoError(t, err)
	for _, providerID := range []string{"", "asset://unsafe", "https://upstream.invalid/key", "../other", "bad?secret"} {
		t.Run(providerID, func(t *testing.T) {
			binding := model.AssetBinding{ChannelId: 272, BindingScope: scope, UpstreamAssetId: "42", UpstreamGroupId: providerID, Status: model.AssetStatusActive}
			require.False(t, activeAssetBinding(&binding))
			require.Empty(t, assetBindingResult("ast_example", binding).RewriteURI)
			require.False(t, isActiveAssetReferenceBinding(assetReferenceBinding{
				ChannelID: 272, BindingScope: scope, UpstreamAssetID: "42", UpstreamGroupID: providerID, Status: model.AssetStatusActive,
			}))
		})
	}
	// Existing providers keep interpreting the private group field as a group.
	require.Equal(t, "asset://original-asset", assetBindingRewriteURIForScope(tokenSpaceMaterialBindingScopePrefix+"scope", "original-asset", "original-group"))
}

func TestVirtualCharacterBindingScopesAndMediaCapability(t *testing.T) {
	channel := virtualCharacterBindingTestChannel()
	one, err := assetBindingScopeForChannel(channel, AssetMaterializeOptions{APIKey: channel.Key, Model: "seedance-2.0"})
	require.NoError(t, err)
	two, err := assetBindingScopeForChannel(channel, AssetMaterializeOptions{APIKey: channel.Key, Model: "seedance-2.5"})
	require.NoError(t, err)
	require.Equal(t, one, two)
	rotated, err := assetBindingScopeForChannel(channel, AssetMaterializeOptions{APIKey: "material-key-b", Model: "seedance-2.0"})
	require.NoError(t, err)
	require.NotEqual(t, one, rotated)
	require.NotContains(t, one, channel.Key)
	for _, mediaType := range []string{"Image", "Video", "Audio"} {
		require.True(t, channelCanConsumeAssetType(channel, mediaType))
	}
	require.False(t, channelCanConsumeAssetType(channel, "Document"))
	channel.OtherSettings = `{"asset_materialization":{"provider":"virtual_character","gateway_base_url":"https://user:secret@materials.example.invalid"}}`
	require.False(t, channelCanConsumeAssetType(channel, "Image"))
	_, err = assetMaterializerForChannel(channel)
	require.Error(t, err)
}

func TestVirtualCharacterStrictReadinessPreparesAndRewritesReference(t *testing.T) {
	db, _ := setupServiceModelAccessDB(t)
	require.NoError(t, db.AutoMigrate(&model.Asset{}, &model.AssetBinding{}, &model.AssetModelCoverageTarget{}, &model.AssetModelReadiness{}))
	installAssetServiceTestDeps(t)
	oldStrict := AssetModelCoverageStrictEnabled
	AssetModelCoverageStrictEnabled = true
	t.Cleanup(func() { AssetModelCoverageStrictEnabled = oldStrict })
	seedModelAccessScope(t, db, 272, "default", constant.ChannelTypeDoubaoVideo, "seedance-2.0")
	channel := virtualCharacterBindingTestChannel()
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", channel.Id).Updates(map[string]any{
		"key": channel.Key, "other_settings": channel.OtherSettings,
	}).Error)
	setModelAccessBilling(t, map[string]float64{"seedance-2.0": 1}, nil, nil)
	recorder := &virtualCharacterBindingRecorder{}
	original := assetMaterializationProviderDescriptors[assetMaterializationProviderVirtualCharacter]
	replacement := original
	replacement.MaterializerFactory = func(assetMaterializationChannelConfig) AssetMaterializer { return recorder }
	assetMaterializationProviderDescriptors[assetMaterializationProviderVirtualCharacter] = replacement
	t.Cleanup(func() {
		assetMaterializationProviderDescriptors[assetMaterializationProviderVirtualCharacter] = original
	})
	asset := insertMaterializeAsset(t, "ast_dddddddddddddddddddddddddddddddd")
	ctx := assetModelScopeGinContext(t, nil)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	scope, err := ResolveAssetModelScopeForContext(ctx, asset.UserId)
	require.NoError(t, err)
	require.Equal(t, []string{"seedance-2.0"}, scope.ModelNames)
	candidates, err := AssetModelTargetCandidates(scope, "seedance-2.0")
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, 272, candidates[0].ChannelID)
	now := time.Now().Unix()
	target, err := ensureAssetModelCoverageTargetAt(scope, "seedance-2.0", "owner", now)
	require.NoError(t, err)
	require.NoError(t, model.EnsureAssetModelReadiness(asset.Id, scope.ScopeKey, scope.ModelNames, now))
	for step := int64(0); step < 2; step++ {
		row := requireAssetModelReadinessRow(t, asset.Id, scope, target.ModelName)
		claimed, err := model.ClaimAssetModelReadinessLease(row.Id, "node", now+step*5, now+step*5+60)
		require.NoError(t, err)
		require.True(t, claimed)
		row = requireAssetModelReadinessRow(t, asset.Id, scope, target.ModelName)
		require.NoError(t, PrepareAssetModelReadiness(context.Background(), row, "node", time.Unix(now+step*5, 0)))
		recorder.ready = true
	}
	row := requireAssetModelReadinessRow(t, asset.Id, scope, target.ModelName)
	require.Equal(t, model.AssetModelReadinessStatusActive, row.Status)
	require.Equal(t, model.AssetStatusActive, ProjectAssetStatusForScope(asset, scope, []model.AssetModelReadiness{row}, map[string]model.AssetModelCoverageTarget{target.ModelName: *target}))
	refs, apiErr := ResolveAssetReferences(ctx, asset.UserId, &dto.SeedanceVideoRequest{
		Model: "seedance-2.0", Content: []dto.SeedanceContentItem{imageAssetItem(asset.PublicId)},
	})
	require.Nil(t, apiErr)
	rewritten, err := MaterializeAssetBindingsForChannel(context.Background(), asset.UserId, refs, channel, AssetMaterializeOptions{APIKey: channel.Key, Model: "seedance-2.0"})
	require.NoError(t, err)
	require.Equal(t, map[string]string{"asset://" + asset.PublicId: "asset://aigc-generation-reference"}, rewritten)
	require.Equal(t, 1, recorder.creates)
}
