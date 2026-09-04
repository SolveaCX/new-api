package taskcommon

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

func TestShouldProxyResultURL(t *testing.T) {
	tests := []struct {
		name      string
		channelID int
		group     string
		want      bool
	}{
		{name: "channel 106 plg", channelID: 106, group: "plg", want: true},
		{name: "channel 106 other group", channelID: 106, group: "default", want: false},
		{name: "channel 106 group is case sensitive", channelID: 106, group: "PLG", want: false},
		{name: "other channel plg", channelID: 105, group: "plg", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldProxyResultURL(tt.channelID, tt.group); got != tt.want {
				t.Fatalf("ShouldProxyResultURL(%d, %q) = %v, want %v", tt.channelID, tt.group, got, tt.want)
			}
		})
	}
}

func TestPublicResultURL(t *testing.T) {
	oldServerAddress := system_setting.ServerAddress
	system_setting.ServerAddress = "https://gateway.example"
	t.Cleanup(func() { system_setting.ServerAddress = oldServerAddress })

	tests := []struct {
		name string
		task *model.Task
		want string
	}{
		{
			name: "nil task has no public result URL",
			task: nil,
			want: "",
		},
		{
			name: "successful channel 106 plg task uses proxy",
			task: &model.Task{
				TaskID:    "task_public",
				ChannelId: 106,
				Group:     "plg",
				Status:    model.TaskStatusSuccess,
				PrivateData: model.TaskPrivateData{
					ResultURL: "https://upstream.example/video.mp4",
				},
			},
			want: "https://gateway.example/v1/videos/task_public/content",
		},
		{
			name: "unfinished channel 106 plg task does not synthesize proxy or leak fail reason",
			task: &model.Task{
				TaskID:     "task_pending",
				ChannelId:  106,
				Group:      "plg",
				Status:     model.TaskStatusInProgress,
				FailReason: "https://upstream.example/leaked-from-fallback.mp4",
				PrivateData: model.TaskPrivateData{
					ResultURL: "",
				},
			},
			want: "",
		},
		{
			name: "failed channel 106 plg task never returns a stored upstream result",
			task: &model.Task{
				TaskID:     "task_failed",
				ChannelId:  106,
				Group:      "plg",
				Status:     model.TaskStatusFailure,
				FailReason: "https://upstream.example/leaked-from-fallback.mp4",
				PrivateData: model.TaskPrivateData{
					ResultURL: "stored-result",
				},
			},
			want: "",
		},
		{
			name: "channel 106 non-plg uses existing result behavior",
			task: &model.Task{
				ChannelId: 106,
				Group:     "default",
				Status:    model.TaskStatusSuccess,
				PrivateData: model.TaskPrivateData{
					ResultURL: "https://upstream.example/video.mp4",
				},
			},
			want: "https://upstream.example/video.mp4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PublicResultURL(tt.task); got != tt.want {
				t.Fatalf("PublicResultURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShouldWhitelabelPlatform(t *testing.T) {
	tests := []struct {
		name     string
		platform constant.TaskPlatform
		want     bool
	}{
		{"kuaizi (channel 58)", constant.TaskPlatform("58"), true},
		{"jimeng proxy (channel 103)", constant.TaskPlatform("103"), true},
		{"blockrun-video (channel 101)", constant.TaskPlatform("101"), true},
		{"jimeng zhizinan (channel 104)", constant.TaskPlatform("104"), true},
		{"techmobi video (channel 105)", constant.TaskPlatform("105"), true},
		{"byteplus (channel 107)", constant.TaskPlatform("107"), true},
		{"modelapi (channel 111)", constant.TaskPlatform("111"), true},
		{"openai channel type number", constant.TaskPlatform("1"), false},
		{"non-numeric platform suno", constant.TaskPlatformSuno, false},
		{"empty platform", constant.TaskPlatform(""), false},
		{"garbage platform", constant.TaskPlatform("not-a-number"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldWhitelabelPlatform(tt.platform); got != tt.want {
				t.Errorf("ShouldWhitelabelPlatform(%q) = %v, want %v", tt.platform, got, tt.want)
			}
		})
	}
}

func TestShouldWhitelabelChannelType(t *testing.T) {
	if !ShouldWhitelabelChannelType(constant.ChannelTypeKuaiziLizhen) {
		t.Errorf("expected kuaizi channel type %d to be whitelabeled", constant.ChannelTypeKuaiziLizhen)
	}
	if !ShouldWhitelabelChannelType(constant.ChannelTypeBlockRunVideo) {
		t.Errorf("expected blockrun-video channel type %d to be whitelabeled", constant.ChannelTypeBlockRunVideo)
	}
	if !ShouldWhitelabelChannelType(constant.ChannelTypeJimengProxy) {
		t.Errorf("expected jimeng proxy channel type %d to be whitelabeled", constant.ChannelTypeJimengProxy)
	}
	if !ShouldWhitelabelChannelType(constant.ChannelTypeJimengZhizinan) {
		t.Errorf("expected jimeng zhizinan channel type %d to be whitelabeled", constant.ChannelTypeJimengZhizinan)
	}
	if !ShouldWhitelabelChannelType(constant.ChannelTypeTechMobiVideo) {
		t.Errorf("expected techmobi video channel type %d to be whitelabeled", constant.ChannelTypeTechMobiVideo)
	}
	if !ShouldWhitelabelChannelType(constant.ChannelTypeBytePlus) {
		t.Errorf("expected BytePlus channel type %d to be whitelabeled", constant.ChannelTypeBytePlus)
	}
	if !ShouldWhitelabelChannelType(constant.ChannelTypeModelAPISeedance) {
		t.Errorf("expected ModelAPI channel type %d to be whitelabeled", constant.ChannelTypeModelAPISeedance)
	}
	if ShouldWhitelabelChannelType(0) {
		t.Error("zero channel type should not be whitelabeled")
	}
	if ShouldWhitelabelChannelType(constant.ChannelTypeKuaiziLizhen + 9999) {
		t.Error("unknown channel type should not be whitelabeled")
	}
}

func TestScrubBrandedText(t *testing.T) {
	const generic = "task failed at upstream provider"
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty stays empty", "", ""},
		{"plain message unchanged", "prompt rejected by safety filter", "prompt rejected by safety filter"},
		{"contains kuaizi", "kuaizi upstream code=500", generic},
		{"contains lizhen mixed case", "LIZHEN service unavailable", generic},
		{"contains volces host", "fetch https://x.tos-cn-beijing.volces.com/abc failed", generic},
		{"contains bytedance", "bytedance returned 4xx", generic},
		{"contains kz-cgt id", "task id kz-cgt-178100 not found", generic},
		{"contains blockrun host", "fetch https://blockrun.ai/api/media/x.mp4 failed", generic},
		{"contains flatkey", "api2.flatkey.ai gateway error", generic},
		{"contains jimeng host", "jimeng.jianying.com returned 500", generic},
		{"contains dreamina model", "dreamina seedance failed", generic},
		{"contains techmobi host", "chatgpttech.mobi returned 500", generic},
		{"contains techmobi name", "TechMobi task failed", generic},
		{"contains byteplus host", "ark.ap-southeast.bytepluses.com returned 500", generic},
		{"contains byteplus name", "BytePlus task failed", generic},
		{"contains modelapi host", "api.modelapi.co returned 500", generic},
		{"contains modelapi name", "ModelAPI seedance failed", generic},
		{"contains endpoint id", "endpoint ep-test-secret rejected the request", generic},
		{"unrelated word with substring", "kuai noodles", "kuai noodles"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ScrubBrandedText(tt.input); got != tt.want {
				t.Errorf("ScrubBrandedText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
