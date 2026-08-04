package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

type recordingAssetMaterializer struct {
	createCalls int64
	getCalls    int64
	blockCreate chan struct{}
	createErr   error
	getStatus   string
}

func (m *recordingAssetMaterializer) CreateAsset(ctx context.Context, input AssetMaterializeInput) (AssetMaterializeResult, error) {
	atomic.AddInt64(&m.createCalls, 1)
	if m.blockCreate != nil {
		<-m.blockCreate
	}
	if input.SignSource != nil {
		signedURL, err := input.SignSource(ctx, input.Asset)
		if err != nil {
			return AssetMaterializeResult{}, err
		}
		if signedURL == "" {
			return AssetMaterializeResult{}, errors.New("empty signed source")
		}
	}
	if m.createErr != nil {
		return AssetMaterializeResult{}, m.createErr
	}
	return AssetMaterializeResult{
		UpstreamGroupID: "group-1",
		UpstreamAssetID: "upstream-" + input.Asset.PublicId,
		Status:          model.BytePlusAssetStatusActive,
	}, nil
}

func (m *recordingAssetMaterializer) GetAsset(ctx context.Context, input AssetMaterializeInput, upstreamAssetID string) (AssetMaterializeResult, error) {
	atomic.AddInt64(&m.getCalls, 1)
	status := m.getStatus
	if status == "" {
		status = model.BytePlusAssetStatusActive
	}
	return AssetMaterializeResult{UpstreamAssetID: upstreamAssetID, Status: status}, nil
}

func TestAssetBindingConcurrentClaimersCreateProviderAssetOnce(t *testing.T) {
	newAssetServiceTestDB(t)
	store := installAssetServiceTestDeps(t)
	asset := insertMaterializeAsset(t, "ast_11111111111111111111111111111111")
	channel := insertMaterializeChannel(t, 131)
	materializer := &recordingAssetMaterializer{blockCreate: make(chan struct{})}
	restore := registerAssetMaterializerForTest(t, constant.ChannelTypeBytePlus, materializer)
	defer restore()

	var start sync.WaitGroup
	start.Add(2)
	results := make(chan AssetBindingResult, 2)
	errs := make(chan error, 2)
	for _, owner := range []string{"node-a", "node-b"} {
		owner := owner
		go func() {
			start.Done()
			start.Wait()
			result, err := MaterializeAssetBinding(context.Background(), AssetBindingRequest{
				UserID:       asset.UserId,
				PublicID:     asset.PublicId,
				Channel:      channel,
				LeaseOwner:   owner,
				PollLimit:    5,
				PollDelay:    time.Millisecond,
				LeaseTTL:     time.Minute,
				ExpectedType: "Image",
			})
			if err != nil {
				errs <- err
				return
			}
			results <- result
		}()
	}
	require.Eventually(t, func() bool { return atomic.LoadInt64(&materializer.createCalls) == 1 }, time.Second, time.Millisecond)
	close(materializer.blockCreate)

	require.Len(t, collectBindingErrors(errs, 2, time.Second), 0)
	got := []AssetBindingResult{<-results, <-results}
	require.Equal(t, int64(1), atomic.LoadInt64(&materializer.createCalls))
	require.Equal(t, got[0].RewriteURI, got[1].RewriteURI)
	require.Equal(t, "asset://upstream-"+asset.PublicId, got[0].RewriteURI)
	require.Len(t, store.signed, 1)
	require.Equal(t, http.MethodGet, store.signed[0].Method)
	require.Equal(t, time.Hour, store.signed[0].TTL)

	var binding model.AssetBinding
	require.NoError(t, model.DB.First(&binding, "asset_id = ? AND channel_id = ?", asset.Id, channel.Id).Error)
	require.Equal(t, model.AssetStatusActive, binding.Status)
	require.Equal(t, "group-1", binding.UpstreamGroupId)
	require.Equal(t, "upstream-"+asset.PublicId, binding.UpstreamAssetId)
	require.NotContains(t, binding.UpstreamAssetId, "signed.example")
}

func TestAssetBindingStaleLeaseCanBeTakenOver(t *testing.T) {
	newAssetServiceTestDB(t)
	installAssetServiceTestDeps(t)
	asset := insertMaterializeAsset(t, "ast_22222222222222222222222222222222")
	channel := insertMaterializeChannel(t, 131)
	require.NoError(t, model.DB.Create(&model.AssetBinding{
		AssetId:        asset.Id,
		ChannelId:      channel.Id,
		Status:         model.AssetBindingStatusLeased,
		LeaseOwner:     "dead-node",
		LeaseExpiresAt: 99,
		CreatedAt:      90,
		UpdatedAt:      90,
	}).Error)
	materializer := &recordingAssetMaterializer{}
	restore := registerAssetMaterializerForTest(t, constant.ChannelTypeBytePlus, materializer)
	defer restore()
	assetBindingNow = func() time.Time { return time.Unix(100, 0) }
	t.Cleanup(func() { assetBindingNow = time.Now })

	result, err := MaterializeAssetBinding(context.Background(), AssetBindingRequest{
		UserID:       asset.UserId,
		PublicID:     asset.PublicId,
		Channel:      channel,
		LeaseOwner:   "node-b",
		PollLimit:    1,
		LeaseTTL:     time.Minute,
		ExpectedType: "Image",
	})

	require.NoError(t, err)
	require.Equal(t, "asset://upstream-"+asset.PublicId, result.RewriteURI)
	require.Equal(t, int64(1), atomic.LoadInt64(&materializer.createCalls))
}

func TestAssetBindingReusesActiveBindingWithoutSigningOrProviderCreate(t *testing.T) {
	newAssetServiceTestDB(t)
	store := installAssetServiceTestDeps(t)
	asset := insertMaterializeAsset(t, "ast_33333333333333333333333333333333")
	channel := insertMaterializeChannel(t, 131)
	require.NoError(t, model.DB.Create(&model.AssetBinding{
		AssetId:         asset.Id,
		ChannelId:       channel.Id,
		Status:          model.AssetStatusActive,
		UpstreamGroupId: "group-existing",
		UpstreamAssetId: "upstream-existing",
		CreatedAt:       100,
		UpdatedAt:       100,
	}).Error)
	materializer := &recordingAssetMaterializer{}
	restore := registerAssetMaterializerForTest(t, constant.ChannelTypeBytePlus, materializer)
	defer restore()

	result, err := MaterializeAssetBinding(context.Background(), AssetBindingRequest{
		UserID:       asset.UserId,
		PublicID:     asset.PublicId,
		Channel:      channel,
		LeaseOwner:   "node-a",
		PollLimit:    1,
		LeaseTTL:     time.Minute,
		ExpectedType: "Image",
	})

	require.NoError(t, err)
	require.Equal(t, "asset://upstream-existing", result.RewriteURI)
	require.Zero(t, atomic.LoadInt64(&materializer.createCalls))
	require.Len(t, store.signed, 0)
}

func TestAssetBindingBoundedPollingReturnsSanitizedInitializingError(t *testing.T) {
	newAssetServiceTestDB(t)
	installAssetServiceTestDeps(t)
	asset := insertMaterializeAsset(t, "ast_44444444444444444444444444444444")
	channel := insertMaterializeChannel(t, 131)
	require.NoError(t, model.DB.Create(&model.AssetBinding{
		AssetId:        asset.Id,
		ChannelId:      channel.Id,
		Status:         model.AssetBindingStatusLeased,
		LeaseOwner:     "other-node",
		LeaseExpiresAt: 200,
		CreatedAt:      100,
		UpdatedAt:      100,
	}).Error)
	assetBindingNow = func() time.Time { return time.Unix(100, 0) }
	t.Cleanup(func() { assetBindingNow = time.Now })
	oldSleep := assetBindingPollSleep
	sleepCalls := int64(0)
	assetBindingPollSleep = func(ctx context.Context, delay time.Duration) error {
		atomic.AddInt64(&sleepCalls, 1)
		return nil
	}
	t.Cleanup(func() { assetBindingPollSleep = oldSleep })

	_, err := MaterializeAssetBinding(context.Background(), AssetBindingRequest{
		UserID:       asset.UserId,
		PublicID:     asset.PublicId,
		Channel:      channel,
		LeaseOwner:   "node-a",
		PollLimit:    2,
		PollDelay:    time.Hour,
		LeaseTTL:     time.Minute,
		ExpectedType: "Image",
	})

	require.Error(t, err)
	require.ErrorIs(t, err, ErrAssetBindingInitializing)
	require.Equal(t, int64(1), sleepCalls)
	require.NotContains(t, err.Error(), "other-node")
}

func TestAssetBindingProviderFailureIsSanitizedAndDoesNotPersistSignedURL(t *testing.T) {
	newAssetServiceTestDB(t)
	store := installAssetServiceTestDeps(t)
	asset := insertMaterializeAsset(t, "ast_55555555555555555555555555555555")
	channel := insertMaterializeChannel(t, 131)
	materializer := &recordingAssetMaterializer{createErr: errors.New("BytePlus secret sk-live signed=https://signed.example/assets?X-Goog-Signature=abc")}
	restore := registerAssetMaterializerForTest(t, constant.ChannelTypeBytePlus, materializer)
	defer restore()

	_, err := MaterializeAssetBinding(context.Background(), AssetBindingRequest{
		UserID:       asset.UserId,
		PublicID:     asset.PublicId,
		Channel:      channel,
		LeaseOwner:   "node-a",
		PollLimit:    1,
		LeaseTTL:     time.Minute,
		ExpectedType: "Image",
	})

	require.Error(t, err)
	for _, marker := range []string{"BytePlus", "sk-live", "signed.example", "X-Goog-Signature"} {
		require.NotContains(t, err.Error(), marker)
	}
	var binding model.AssetBinding
	require.NoError(t, model.DB.First(&binding, "asset_id = ? AND channel_id = ?", asset.Id, channel.Id).Error)
	require.Equal(t, model.AssetStatusFailed, binding.Status)
	require.NotContains(t, binding.ErrorCode, "signed.example")
	require.Len(t, store.signed, 1, "create attempt needs exactly one ephemeral signed GET URL")
}

func insertMaterializeAsset(t *testing.T, publicID string) model.Asset {
	t.Helper()
	asset := model.Asset{
		PublicId:         publicID,
		UserId:           7,
		AssetType:        "Image",
		Status:           model.AssetStatusActive,
		SourceStatus:     model.AssetSourceStatusAvailable,
		StorageBackend:   defaultAssetStorageBackend,
		StorageBucket:    "asset-test-bucket",
		ObjectKey:        "assets/" + publicID + ".png",
		ObjectGeneration: 27,
		ContentType:      "image/png",
		SourceExpiresAt:  time.Now().Add(time.Hour).Unix(),
		CreatedAt:        100,
		UpdatedAt:        100,
	}
	require.NoError(t, model.DB.Create(&asset).Error)
	return asset
}

func insertMaterializeChannel(t *testing.T, id int) *model.Channel {
	t.Helper()
	key, err := common.Marshal(BytePlusCredentials{
		APIKey:          "video-key",
		AccessKeyID:     "ak-test",
		SecretAccessKey: "sk-test",
		ProjectName:     "project-test",
	})
	require.NoError(t, err)
	channel := &model.Channel{Id: id, Type: constant.ChannelTypeBytePlus, Key: string(key), Status: common.ChannelStatusEnabled}
	return channel
}

func collectBindingErrors(errs <-chan error, want int, timeout time.Duration) []error {
	deadline := time.After(timeout)
	collected := make([]error, 0)
	for len(collected) < want {
		select {
		case err := <-errs:
			if err != nil {
				collected = append(collected, err)
			}
		case <-deadline:
			return collected
		}
	}
	return collected
}

func assertNoAssetBindingLeak(t *testing.T, err error) {
	t.Helper()
	text := err.Error()
	for _, marker := range []string{"BytePlus", "byteplus", "sk-", "ak-", "signed.example", "X-Goog"} {
		if strings.Contains(text, marker) {
			t.Fatalf("binding error leaked %q in %q", marker, text)
		}
	}
}
