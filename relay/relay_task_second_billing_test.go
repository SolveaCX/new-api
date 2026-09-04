package relay

import (
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/task/suno"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

// fakeSecondBillingAdaptor stands in for a task adaptor that reports a
// per-second billing failure.
type fakeSecondBillingAdaptor struct {
	err   error
	units map[string]float64
}

func (f *fakeSecondBillingAdaptor) SecondBillingRatios() (map[string]float64, error) {
	return f.units, f.err
}

func TestResolveSecondBillingRatios_PropagatesError(t *testing.T) {
	want := errors.New("no matching rule")
	got, err := resolveSecondBillingRatios(&fakeSecondBillingAdaptor{err: want})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if got != nil {
		t.Fatalf("ratios must be nil on error, got %v", got)
	}
}

func TestResolveSecondBillingRatios_ReturnsUnits(t *testing.T) {
	units := map[string]float64{"video_billing_units": 11.2}
	got, err := resolveSecondBillingRatios(&fakeSecondBillingAdaptor{units: units})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["video_billing_units"] != 11.2 {
		t.Fatalf("units = %v, want 11.2", got)
	}
}

func TestResolveSecondBillingRatios_IgnoresNonImplementers(t *testing.T) {
	got, err := resolveSecondBillingRatios(struct{}{})
	if err != nil {
		t.Fatalf("a non-implementing adaptor must not error: %v", err)
	}
	if got != nil {
		t.Fatalf("ratios must be nil, got %v", got)
	}
}

func TestResolveSecondBillingRatios_NilAdaptor(t *testing.T) {
	got, err := resolveSecondBillingRatios(nil)
	if err != nil {
		t.Fatalf("a nil adaptor must not error: %v", err)
	}
	if got != nil {
		t.Fatalf("ratios must be nil, got %v", got)
	}
}

func TestHasValidSecondBillingUnits(t *testing.T) {
	tests := []struct {
		name   string
		ratios map[string]float64
		valid  bool
	}{
		{"nil", nil, false},
		{"empty", map[string]float64{}, false},
		{"wrong key", map[string]float64{"seconds": 1}, false},
		{"zero", map[string]float64{taskcommon.BillingUnitsKey: 0}, false},
		{"negative", map[string]float64{taskcommon.BillingUnitsKey: -1}, false},
		{"nan", map[string]float64{taskcommon.BillingUnitsKey: math.NaN()}, false},
		{"positive infinity", map[string]float64{taskcommon.BillingUnitsKey: math.Inf(1)}, false},
		{"negative infinity", map[string]float64{taskcommon.BillingUnitsKey: math.Inf(-1)}, false},
		{"valid", map[string]float64{taskcommon.BillingUnitsKey: 0.001}, true},
		{"valid with extras", map[string]float64{taskcommon.BillingUnitsKey: 8, "region": 2}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasValidSecondBillingUnits(tt.ratios); got != tt.valid {
				t.Fatalf("hasValidSecondBillingUnits(%v) = %v, want %v", tt.ratios, got, tt.valid)
			}
		})
	}
}

// An unpriceable request is a local configuration fault, not a channel fault.
// prepareTaskSubmit must therefore wrap it as a local TaskError: the controller
// hands every non-local TaskError to processChannelError, which logs a channel
// error and can auto-disable the channel on a keyword match ("permission
// denied", "operation not allowed", ...). A missing price rule would then take
// a healthy channel offline, and the same missing rule would do it again on
// every channel the retry loop tried.
func TestSecondBillingRejectionIsLocalNotChannelFault(t *testing.T) {
	taskErr := service.TaskErrorWrapperLocal(
		errors.New("no matching rule"), "video_price_not_configured", http.StatusBadRequest)

	if !taskErr.LocalError {
		t.Fatal("price-table rejection must be a local error, or it is blamed on the channel")
	}
	if taskErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", taskErr.StatusCode, http.StatusBadRequest)
	}
	if taskErr.Code != "video_price_not_configured" {
		t.Fatalf("code = %q, want video_price_not_configured", taskErr.Code)
	}
}

// A model configured only in the video /s price table must reach the
// per-second billing path even when it has no legacy fixed ModelPrice or
// ModelRatio entry. The baseline fails earlier in ModelPriceHelperPerCall with
// model_price_not_configured, before BytePlus can calculate video_billing_units.
func TestPrepareTaskAttempt_AllowsVideoOnlyPriceConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldConfig := map[string]string{}
	oldModelPrices := ratio_setting.ModelPrice2JSONString()
	oldModelRatios := ratio_setting.ModelRatio2JSONString()
	if err := config.GlobalConfig.SaveToDB(func(key, value string) error {
		oldConfig[key] = value
		return nil
	}); err != nil {
		t.Fatalf("snapshot config: %v", err)
	}
	t.Cleanup(func() {
		if err := config.GlobalConfig.LoadFromDB(oldConfig); err != nil {
			t.Errorf("restore config: %v", err)
		}
		if err := ratio_setting.UpdateModelPriceByJSONString(oldModelPrices); err != nil {
			t.Errorf("restore model prices: %v", err)
		}
		if err := ratio_setting.UpdateModelRatioByJSONString(oldModelRatios); err != nil {
			t.Errorf("restore model ratios: %v", err)
		}
	})
	if err := config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting_video.video_price_rules": `[{"model":"seedance-2.0-mini","match":{"resolution":"1080p"},"price_per_second":0.5,"basis":"output_duration"}]`,
	}); err != nil {
		t.Fatalf("install video rules: %v", err)
	}
	if err := ratio_setting.UpdateModelPriceByJSONString(`{}`); err != nil {
		t.Fatalf("clear model prices: %v", err)
	}
	if err := ratio_setting.UpdateModelRatioByJSONString(`{}`); err != nil {
		t.Fatalf("clear model ratios: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(
		`{"model":"seedance-2.0-mini","content":[{"type":"text","text":"a cat"}],"duration":8,"resolution":"1080p"}`,
	))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("platform", strconv.Itoa(constant.ChannelTypeBytePlus))
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeBytePlus)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "https://ark.example")

	info := &relaycommon.RelayInfo{
		OriginModelName: "seedance-2.0-mini",
		UsingGroup:      "default",
		UserGroup:       "default",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
	}
	preflight, taskErr := PrepareTaskAttempt(c, info)
	if taskErr != nil {
		t.Fatalf("video-only model rejected before per-second billing: %+v", taskErr)
	}
	if preflight == nil {
		t.Fatal("missing task preflight")
	}
	if !info.PriceData.UsePrice || info.PriceData.ModelPrice != 1 || info.PriceData.ModelRatio != 0 {
		t.Fatalf("video-only request did not get fallback fixed basis: %+v", info.PriceData)
	}
	if got := info.PriceData.OtherRatios[taskcommon.BillingUnitsKey]; got != 4 {
		t.Fatalf("video_billing_units = %v, want 4", got)
	}
	if want := int(4 * common.QuotaPerUnit * info.PriceData.GroupRatioInfo.GroupRatio); preflight.Quota != want {
		t.Fatalf("quota = %d, want %d (units*QuotaPerUnit*group ratio)", preflight.Quota, want)
	}
}

func snapshotSecondBillingGlobals(t *testing.T, rules string) {
	t.Helper()
	oldConfig := map[string]string{}
	if err := config.GlobalConfig.SaveToDB(func(key, value string) error { oldConfig[key] = value; return nil }); err != nil {
		t.Fatalf("snapshot config: %v", err)
	}
	oldPrices, oldRatios := ratio_setting.ModelPrice2JSONString(), ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() {
		if err := config.GlobalConfig.LoadFromDB(oldConfig); err != nil {
			t.Errorf("restore config: %v", err)
		}
		if err := ratio_setting.UpdateModelPriceByJSONString(oldPrices); err != nil {
			t.Errorf("restore prices: %v", err)
		}
		if err := ratio_setting.UpdateModelRatioByJSONString(oldRatios); err != nil {
			t.Errorf("restore ratios: %v", err)
		}
	})
	if err := config.GlobalConfig.LoadFromDB(map[string]string{"billing_setting_video.video_price_rules": rules}); err != nil {
		t.Fatalf("install rules: %v", err)
	}
}

func bytePlusPrepare(t *testing.T, model string) (*relaycommon.RelayInfo, *TaskPreflightResult, *dto.TaskError) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{"model":"`+model+`","content":[{"type":"text","text":"cat"}],"duration":8,"resolution":"1080p"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("platform", strconv.Itoa(constant.ChannelTypeBytePlus))
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeBytePlus)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "https://ark.example")
	info := &relaycommon.RelayInfo{OriginModelName: model, UsingGroup: "default", UserGroup: "default", TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
	pre, err := PrepareTaskAttempt(c, info)
	return info, pre, err
}

func TestPrepareTaskAttempt_PreservesPositiveModelPriceButBillsPerSecond(t *testing.T) {
	snapshotSecondBillingGlobals(t, `[{"model":"price-kept","match":{"resolution":"1080p"},"price_per_second":0.5,"basis":"output_duration"}]`)
	if err := ratio_setting.UpdateModelPriceByJSONString(`{"price-kept":0.25}`); err != nil {
		t.Fatal(err)
	}
	if err := ratio_setting.UpdateModelRatioByJSONString(`{}`); err != nil {
		t.Fatal(err)
	}
	info, pre, taskErr := bytePlusPrepare(t, "price-kept")
	if taskErr != nil {
		t.Fatalf("prepare: %+v", taskErr)
	}
	if info.PriceData.ModelPrice != 0.25 || !info.PriceData.UsePrice {
		t.Fatalf("price basis changed: %+v", info.PriceData)
	}
	if info.PriceData.OtherRatios[taskcommon.BillingUnitsKey] != 16 {
		t.Fatalf("units = %v, want 16", info.PriceData.OtherRatios[taskcommon.BillingUnitsKey])
	}
	if want := int(0.5 * 8 * common.QuotaPerUnit * info.PriceData.GroupRatioInfo.GroupRatio); pre.Quota != want {
		t.Fatalf("quota = %d, want %d", pre.Quota, want)
	}
}

func TestPrepareTaskAttempt_ZeroModelPriceFallsBack(t *testing.T) {
	snapshotSecondBillingGlobals(t, `[{"model":"zero-price","match":{"resolution":"1080p"},"price_per_second":0.5,"basis":"output_duration"}]`)
	if err := ratio_setting.UpdateModelPriceByJSONString(`{"zero-price":0}`); err != nil {
		t.Fatal(err)
	}
	if err := ratio_setting.UpdateModelRatioByJSONString(`{}`); err != nil {
		t.Fatal(err)
	}
	info, _, taskErr := bytePlusPrepare(t, "zero-price")
	if taskErr != nil {
		t.Fatalf("prepare: %+v", taskErr)
	}
	if info.PriceData.ModelPrice != 1 || !info.PriceData.UsePrice || info.PriceData.ModelRatio != 0 {
		t.Fatalf("fallback price data = %+v", info.PriceData)
	}
	if info.PriceData.GroupRatioInfo.GroupRatio > 0 && info.PriceData.FreeModel {
		t.Fatal("configured per-second price must not be marked free for a positive group ratio")
	}
}

func TestPrepareTaskAttempt_ModelRatioOnlyVideoFallsBack(t *testing.T) {
	snapshotSecondBillingGlobals(t, `[{"model":"ratio-video","match":{"resolution":"1080p"},"price_per_second":0.5,"basis":"output_duration"}]`)
	if err := ratio_setting.UpdateModelPriceByJSONString(`{}`); err != nil {
		t.Fatal(err)
	}
	if err := ratio_setting.UpdateModelRatioByJSONString(`{"ratio-video":2}`); err != nil {
		t.Fatal(err)
	}
	info, _, taskErr := bytePlusPrepare(t, "ratio-video")
	if taskErr != nil {
		t.Fatalf("prepare: %+v", taskErr)
	}
	if info.PriceData.ModelPrice != 1 || !info.PriceData.UsePrice {
		t.Fatalf("ratio fallback = %+v", info.PriceData)
	}
}

func TestPrepareTaskAttempt_UnconfiguredSecondBillingAdaptorPreservesLegacyRatios(t *testing.T) {
	snapshotSecondBillingGlobals(t, `[]`)
	if err := ratio_setting.UpdateModelPriceByJSONString(`{}`); err != nil {
		t.Fatal(err)
	}
	if err := ratio_setting.UpdateModelRatioByJSONString(`{"seedance-2.0":2}`); err != nil {
		t.Fatal(err)
	}

	info, pre, taskErr := bytePlusPrepare(t, "seedance-2.0")
	if taskErr != nil {
		t.Fatalf("prepare: %+v", taskErr)
	}
	if info.PriceData.UsePrice || info.PriceData.ModelRatio != 2 {
		t.Fatalf("legacy ModelRatio billing was not restored: %+v", info.PriceData)
	}
	wantRatio := 77.0 / 70.0
	if got := info.PriceData.OtherRatios["video_input"]; got != wantRatio {
		t.Fatalf("video_input ratio = %v, want %v", got, wantRatio)
	}
	baseQuota := int(common.QuotaPerUnit * info.PriceData.GroupRatioInfo.GroupRatio)
	wantQuota := int(float64(baseQuota) * wantRatio)
	if pre == nil || pre.Quota != wantQuota {
		t.Fatalf("quota = %v, want %d", pre, wantQuota)
	}
}

func TestPrepareTaskAttempt_NonVideoMissingPriceStillRejects(t *testing.T) {
	snapshotSecondBillingGlobals(t, `[]`)
	if err := ratio_setting.UpdateModelPriceByJSONString(`{}`); err != nil {
		t.Fatal(err)
	}
	if err := ratio_setting.UpdateModelRatioByJSONString(`{}`); err != nil {
		t.Fatal(err)
	}
	_, _, taskErr := bytePlusPrepare(t, "ordinary-unpriced")
	if taskErr == nil || taskErr.Code != "model_price_error" {
		t.Fatalf("error = %+v, want model_price_error", taskErr)
	}
}

func TestPrepareTaskAttempt_VideoRuleRequiresSecondBillingAdaptor(t *testing.T) {
	snapshotSecondBillingGlobals(t, `[{"model":"suno-video","match":{"resolution":"1080p"},"price_per_second":0.5,"basis":"output_duration"}]`)
	undo := registerTaskAdaptorForTest(constant.TaskPlatformSuno, &suno.TaskAdaptor{})
	defer undo()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "action", Value: "MUSIC"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/suno/submit/music", strings.NewReader(`{"prompt":"x"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("platform", string(constant.TaskPlatformSuno))
	common.SetContextKey(c, constant.ContextKeyChannelType, 0)
	info := &relaycommon.RelayInfo{OriginModelName: "suno-video", UsingGroup: "default", UserGroup: "default", TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
	_, taskErr := PrepareTaskAttempt(c, info)
	if taskErr == nil || taskErr.Code != "model_price_error" {
		t.Fatalf("error = %+v, want model_price_error", taskErr)
	}
}

func TestPrepareTaskAttempt_UnmatchedVideoRuleFailsClosed(t *testing.T) {
	snapshotSecondBillingGlobals(t, `[{"model":"unmatched-video","match":{"resolution":"720p"},"price_per_second":0.5,"basis":"output_duration"}]`)
	if err := ratio_setting.UpdateModelPriceByJSONString(`{"unmatched-video":0.25}`); err != nil {
		t.Fatal(err)
	}
	if err := ratio_setting.UpdateModelRatioByJSONString(`{}`); err != nil {
		t.Fatal(err)
	}
	_, _, taskErr := bytePlusPrepare(t, "unmatched-video")
	if taskErr == nil || taskErr.Code != "video_price_not_configured" {
		t.Fatalf("error = %+v, want video_price_not_configured", taskErr)
	}
}

// TASK_PRICE_PATCH is a temporary escape hatch meaning "force pure per-call,
// skip every OtherRatios multiplier". A configured per-second price is a
// deliberate administrator decision and must outrank it: otherwise the
// multiplier is computed, stored in the billing snapshot, and then silently
// never applied, so a 30-second video bills at the base quota.
func TestShouldApplyOtherRatios(t *testing.T) {
	tests := []struct {
		name         string
		patched      bool
		secondRatios map[string]float64
		want         bool
	}{
		{"not patched, no per-second", false, nil, true},
		{"not patched, per-second", false, map[string]float64{"video_billing_units": 11.2}, true},
		{"patched, no per-second", true, nil, false},
		{"patched, per-second wins", true, map[string]float64{"video_billing_units": 11.2}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldApplyOtherRatios(tc.patched, tc.secondRatios); got != tc.want {
				t.Fatalf("shouldApplyOtherRatios(%v, %v) = %v, want %v",
					tc.patched, tc.secondRatios, got, tc.want)
			}
		})
	}
}
