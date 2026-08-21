package middleware

import "testing"

func TestIsStaticWebAsset(t *testing.T) {
	for _, test := range []struct {
		path string
		want bool
	}{
		{path: "/static/js/index.js", want: true},
		{path: "/static/css/index.css", want: true},
		{path: "/static/font/public-sans.woff2", want: true},
		{path: "/", want: false},
		{path: "/users", want: false},
		{path: "/api/status", want: false},
	} {
		t.Run(test.path, func(t *testing.T) {
			if got := isStaticWebAsset(test.path); got != test.want {
				t.Fatalf("isStaticWebAsset(%q) = %v, want %v", test.path, got, test.want)
			}
		})
	}
}
