package service

import (
	"regexp"
	"strings"
	"testing"
)

func TestConsoleRobotsCrawlBoundaries(t *testing.T) {
	// This policy has one catch-all Allow and literal Disallow prefixes with
	// optional end anchors. Apply those robots semantics to real request URIs.
	var blocked []*regexp.Regexp
	for _, line := range strings.Split(BuildConsoleRobotsTxt(), "\n") {
		if !strings.HasPrefix(line, "Disallow: ") {
			continue
		}
		path := strings.TrimPrefix(line, "Disallow: ")
		pattern := "^" + regexp.QuoteMeta(strings.TrimSuffix(path, "$"))
		if strings.HasSuffix(path, "$") {
			pattern += "$"
		}
		blocked = append(blocked, regexp.MustCompile(pattern))
	}
	for _, tc := range []struct {
		uri   string
		allow bool
	}{
		{"/api-marketplace", true},
		{"/api-marketplace/", true},
		{"/api-marketplace?from=search", true},
		{"/playground", true},
		{"/available-models", true},
		{"/dashboard/overview", true},
		{"/sign-up?aff=CS47", true},
		{"/sign-in?redirect=/redeem?from%3DX_SG", true},
		{"/redeem?from=X_SG", true},
		{"/keys", true},
		{"/profile", true},
		{"/dashboard", true},
		{"/api", false},
		{"/api?", false},
		{"/api?from=search", false},
		{"/api/", false},
		{"/api/user/self", false},
		{"/api/user/self?from=search", false},
		{"/v1", false},
		{"/v1?from=search", false},
		{"/v1/chat/completions", false},
		{"/v1beta", false},
		{"/v1beta/models", false},
		{"/assets", false},
		{"/assets/app.js", false},
		{"/_next/static/app.js", false},
		{"/cdn-cgi/l/email-protection", false},
	} {
		t.Run(tc.uri, func(t *testing.T) {
			allow := true
			for _, rule := range blocked {
				if rule.MatchString(tc.uri) {
					allow = false
					break
				}
			}
			if allow != tc.allow {
				t.Fatalf("crawl allowed for %q = %v, want %v", tc.uri, allow, tc.allow)
			}
		})
	}
}
