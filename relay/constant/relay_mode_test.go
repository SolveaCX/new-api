package constant

import "testing"

func TestPath2RelayModeSupportsPlaygroundMediaAliases(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{path: "/pg/chat/completions", want: RelayModeChatCompletions},
		{path: "/pg/audio/speech", want: RelayModeAudioSpeech},
		{path: "/pg/images/generations", want: RelayModeImagesGenerations},
		{path: "/pg/video-to-music", want: RelayModeVideoSubmit},
		{path: "/pg/video-to-music/task_abc", want: RelayModeVideoFetchByID},
		{path: "/v1/video-to-music/task_abc", want: RelayModeVideoFetchByID},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			if got := Path2RelayMode(test.path); got != test.want {
				t.Fatalf("Path2RelayMode(%q) = %d, want %d", test.path, got, test.want)
			}
		})
	}
}
