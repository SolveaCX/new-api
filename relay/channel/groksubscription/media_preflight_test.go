package groksubscription

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMediaPreflightTestDB(t *testing.T) {
	t.Helper()
	originalDB := model.DB
	originalSQLite := common.UsingSQLite
	common.UsingSQLite = true
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/media-preflight.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.GrokChannelState{}))
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		common.UsingSQLite = originalSQLite
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
}

func seedMediaPreflightChannel(t *testing.T, expiresAt int64) int {
	t.Helper()
	cred := Credential{Version: 1, Type: CredentialType, AccessToken: "old-at", RefreshToken: "rt", TokenType: "Bearer", ExpiresAt: expiresAt}
	key, err := cred.Serialize()
	require.NoError(t, err)
	ch := model.Channel{
		Type:   constant.ChannelTypeGrokSubscription,
		Key:    key,
		Models: "grok-4.6",
		Group:  "default",
		Status: common.ChannelStatusEnabled,
	}
	require.NoError(t, ch.Insert())
	require.NoError(t, model.UpsertGrokChannelState(&model.GrokChannelState{ChannelID: ch.Id, AuthStatus: model.GrokAuthStatusActive}))
	return ch.Id
}

func TestEnsureMediaCredentialRefreshesAndProbesBeforePaidWrite(t *testing.T) {
	setupMediaPreflightTestDB(t)
	channelID := seedMediaPreflightChannel(t, 2000+int64(MediaCredentialExpirySafetyWindow/time.Second)-1)

	var tokenRefreshes, billingCalls int
	restore := SetMediaPreflightHooksForTest(MediaPreflightHooks{
		Now: func(context.Context) int64 { return 2000 },
		HTTPDoer: doerFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case req.URL.String() == OAuthToken:
				tokenRefreshes++
				return jsonResponse(http.StatusOK, `{"access_token":"new-at","refresh_token":"rt2","token_type":"Bearer","expires_in":7200}`), nil
			case strings.HasSuffix(req.URL.Path, BillingMonthlyPath):
				billingCalls++
				require.Equal(t, "Bearer new-at", req.Header.Get("Authorization"))
				return jsonResponse(http.StatusOK, `{"monthlyLimit":15000}`), nil
			case strings.HasSuffix(req.URL.Path, BillingWeeklyCreditsPath):
				billingCalls++
				require.Equal(t, "Bearer new-at", req.Header.Get("Authorization"))
				return jsonResponse(http.StatusServiceUnavailable, `{}`), nil
			default:
				t.Fatalf("unexpected upstream request: %s", req.URL.String())
				return nil, nil
			}
		}),
	})
	defer restore()

	got, err := EnsureMediaCredential(context.Background(), channelID, true)

	require.NoError(t, err)
	require.Equal(t, MediaCredential{ChannelID: channelID, AccessToken: "new-at"}, got)
	require.Equal(t, 1, tokenRefreshes)
	require.Equal(t, 2, billingCalls)
	st, err := model.GetGrokChannelState(channelID)
	require.NoError(t, err)
	require.Equal(t, int64(2000), st.BillingObservedAt)
	require.Contains(t, st.QuotaSnapshot, `"version":1`)
	require.Equal(t, "SuperGrok", st.BillingPlan)
	var mediaCount int64
	require.NoError(t, model.DB.Model(&model.Ability{}).Where("channel_id = ? AND model IN ?", channelID, model.GrokMediaAbilityModels()).Count(&mediaCount).Error)
	require.Equal(t, int64(3), mediaCount)
}

func TestEnsureMediaCredentialReadDoesNotDenyStaleBilling(t *testing.T) {
	setupMediaPreflightTestDB(t)
	channelID := seedMediaPreflightChannel(t, 4000)
	require.NoError(t, model.DB.Model(&model.GrokChannelState{}).Where("channel_id = ?", channelID).Updates(map[string]any{
		"quota_snapshot":      `{"version":1,"plan":"SuperGrok","monthly":{"status_code":200},"weekly":{"status_code":503}}`,
		"billing_observed_at": 1,
	}).Error)
	restore := SetMediaPreflightHooksForTest(MediaPreflightHooks{
		Now: func(context.Context) int64 { return 2000 },
		HTTPDoer: doerFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatalf("read preflight with non-expiring token must not call upstream, got %s", req.URL.String())
			return nil, nil
		}),
	})
	defer restore()

	got, err := EnsureMediaCredential(context.Background(), channelID, false)

	require.NoError(t, err)
	require.Equal(t, "old-at", got.AccessToken)
}

func TestEnsureMediaCredentialProbeFailurePreservesSnapshotAndReturnsUnavailable(t *testing.T) {
	setupMediaPreflightTestDB(t)
	now := int64(2000 + billingMaxEvidenceAge + 1)
	channelID := seedMediaPreflightChannel(t, now+3600)
	require.NoError(t, model.DB.Model(&model.GrokChannelState{}).Where("channel_id = ?", channelID).Updates(map[string]any{
		"quota_snapshot":      `{"version":1,"plan":"SuperGrok","monthly":{"status_code":200},"weekly":{"status_code":503}}`,
		"billing_observed_at": 100,
		"billing_plan":        "SuperGrok",
	}).Error)
	restore := SetMediaPreflightHooksForTest(MediaPreflightHooks{
		Now: func(context.Context) int64 { return now },
		HTTPDoer: doerFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("network down")
		}),
	})
	defer restore()

	_, err := EnsureMediaCredential(context.Background(), channelID, true)

	require.ErrorIs(t, err, ErrBillingProbeFailed)
	st, err := model.GetGrokChannelState(channelID)
	require.NoError(t, err)
	require.Equal(t, int64(100), st.BillingObservedAt)
	require.Equal(t, "SuperGrok", st.BillingPlan)
	require.Equal(t, model.GrokAuthStatusActive, st.AuthStatus)
}
