package operation_setting

import (
	"math"
	"reflect"
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
)

func TestDefaultLogRequestSamplingSetting(t *testing.T) {
	resetLogRequestSamplingSettingForTest(t)

	setting := GetLogRequestSamplingSetting()
	if setting.Enabled {
		t.Fatal("expected log request sampling to be disabled by default")
	}
	if setting.SampleRate != 0.001 {
		t.Fatalf("sample rate = %v, want 0.001", setting.SampleRate)
	}
	if !reflect.DeepEqual(setting.Groups, []string{"plg"}) {
		t.Fatalf("groups = %v, want [plg]", setting.Groups)
	}
	if setting.MaxBodyBytes != 16384 {
		t.Fatalf("max body bytes = %d, want 16384", setting.MaxBodyBytes)
	}
	if setting.RetentionDays != 14 {
		t.Fatalf("retention days = %d, want 14", setting.RetentionDays)
	}
	if setting.MaxStringBytes != 4096 {
		t.Fatalf("max string bytes = %d, want 4096", setting.MaxStringBytes)
	}
	if setting.MaxJSONDepth != 16 {
		t.Fatalf("max json depth = %d, want 16", setting.MaxJSONDepth)
	}
	if !setting.DropBinaryPayloads {
		t.Fatal("expected binary payloads to be dropped by default")
	}

	wantPaths := []string{"/v1/chat/completions", "/v1/responses", "/v1/responses/compact", "/v1/messages", "/v1/models/*", "/v1beta/models/*"}
	if !reflect.DeepEqual(setting.EligiblePaths, wantPaths) {
		t.Fatalf("eligible paths = %v, want %v", setting.EligiblePaths, wantPaths)
	}
}

func TestLogRequestSamplingSnapshotIsImmutableAndValidated(t *testing.T) {
	resetLogRequestSamplingSettingForTest(t)
	err := UpdateLogRequestSamplingConfigFromMap(map[string]string{
		"enabled":          "true",
		"sample_rate":      "2",
		"groups":           `[" plg ","enterprise",""]`,
		"eligible_paths":   `["/v1/chat/completions","/v1beta/models/*",""]`,
		"max_body_bytes":   "0",
		"retention_days":   "-1",
		"max_string_bytes": "0",
		"max_json_depth":   "0",
	})
	if err != nil {
		t.Fatalf("UpdateConfigFromMap failed: %v", err)
	}

	snapshot := GetLogRequestSamplingSnapshot()
	if !snapshot.Enabled {
		t.Fatal("expected snapshot enabled")
	}
	if snapshot.SampleRate != 1 {
		t.Fatalf("sample rate = %v, want clamped 1", snapshot.SampleRate)
	}
	if !snapshot.Groups["plg"] || !snapshot.Groups["enterprise"] || len(snapshot.Groups) != 2 {
		t.Fatalf("groups snapshot = %v, want plg and enterprise only", snapshot.Groups)
	}
	if snapshot.MaxBodyBytes != 16384 {
		t.Fatalf("max body bytes = %d, want default fallback", snapshot.MaxBodyBytes)
	}
	if snapshot.RetentionDays != 14 {
		t.Fatalf("retention days = %d, want default fallback", snapshot.RetentionDays)
	}
	if snapshot.MaxStringBytes != 4096 {
		t.Fatalf("max string bytes = %d, want default fallback", snapshot.MaxStringBytes)
	}
	if snapshot.MaxJSONDepth != 16 {
		t.Fatalf("max json depth = %d, want default fallback", snapshot.MaxJSONDepth)
	}

	snapshot.Groups["plg"] = false
	again := GetLogRequestSamplingSnapshot()
	if !again.Groups["plg"] {
		t.Fatal("mutating returned snapshot must not affect future snapshots")
	}
}

func TestLogRequestSamplingSnapshotRejectsNonFiniteRateAndClampsLimits(t *testing.T) {
	resetLogRequestSamplingSettingForTest(t)
	UpdateLogRequestSamplingSetting(func(setting *LogRequestSamplingSetting) {
		setting.Enabled = true
		setting.SampleRate = math.Inf(1)
		setting.MaxBodyBytes = MaxLogRequestSamplingBodyBytes + 1
		setting.RetentionDays = MaxLogRequestSamplingRetentionDays + 1
		setting.MaxStringBytes = MaxLogRequestSamplingMaxStringBytes + 1
		setting.MaxJSONDepth = MaxLogRequestSamplingMaxJSONDepth + 1
	})
	snapshot := GetLogRequestSamplingSnapshot()
	if snapshot.SampleRate != DefaultLogRequestSamplingRate {
		t.Fatalf("sample rate = %v, want default fallback", snapshot.SampleRate)
	}
	if snapshot.MaxBodyBytes != MaxLogRequestSamplingBodyBytes {
		t.Fatalf("max body bytes = %d, want cap %d", snapshot.MaxBodyBytes, MaxLogRequestSamplingBodyBytes)
	}
	if snapshot.RetentionDays != MaxLogRequestSamplingRetentionDays {
		t.Fatalf("retention days = %d, want cap %d", snapshot.RetentionDays, MaxLogRequestSamplingRetentionDays)
	}
	if snapshot.MaxStringBytes != MaxLogRequestSamplingMaxStringBytes {
		t.Fatalf("max string bytes = %d, want cap %d", snapshot.MaxStringBytes, MaxLogRequestSamplingMaxStringBytes)
	}
	if snapshot.MaxJSONDepth != MaxLogRequestSamplingMaxJSONDepth {
		t.Fatalf("max json depth = %d, want cap %d", snapshot.MaxJSONDepth, MaxLogRequestSamplingMaxJSONDepth)
	}

	if err := UpdateLogRequestSamplingConfigFromMap(map[string]string{"sample_rate": "NaN"}); err != nil {
		t.Fatalf("UpdateLogRequestSamplingConfigFromMap failed: %v", err)
	}
	snapshot = GetLogRequestSamplingSnapshot()
	if snapshot.SampleRate != DefaultLogRequestSamplingRate {
		t.Fatalf("NaN sample rate = %v, want default fallback", snapshot.SampleRate)
	}
}

func TestLogRequestSamplingGlobalConfigLoadRefreshesSnapshot(t *testing.T) {
	resetLogRequestSamplingSettingForTest(t)
	err := config.GlobalConfig.LoadFromDB(map[string]string{
		"log_request_sampling.enabled":     "true",
		"log_request_sampling.sample_rate": "0.5",
	})
	if err != nil {
		t.Fatalf("LoadFromDB failed: %v", err)
	}

	snapshot := GetLogRequestSamplingSnapshot()
	if !snapshot.Enabled {
		t.Fatal("expected GlobalConfig.LoadFromDB to enable snapshot")
	}
	if snapshot.SampleRate != 0.5 {
		t.Fatalf("sample rate = %v, want 0.5", snapshot.SampleRate)
	}
}

func resetLogRequestSamplingSettingForTest(t *testing.T) {
	t.Helper()
	original := GetLogRequestSamplingSetting()
	UpdateLogRequestSamplingSetting(func(setting *LogRequestSamplingSetting) {
		*setting = cloneLogRequestSamplingSetting(defaultLogRequestSamplingSetting)
	})
	t.Cleanup(func() {
		UpdateLogRequestSamplingSetting(func(setting *LogRequestSamplingSetting) {
			*setting = original
		})
	})
}
