package controller

import (
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

func doubaoVideoContentAuthorization(channel *model.Channel, task *model.Task, videoURL *url.URL) (string, bool) {
	if channel == nil || task == nil || videoURL == nil || channel.Type != constant.ChannelTypeDoubaoVideo {
		return "", false
	}
	baseURL, err := url.Parse(strings.TrimSpace(channel.GetBaseURL()))
	if err != nil || !sameHTTPOrigin(baseURL, videoURL) || videoURL.User != nil || videoURL.RawQuery != "" || videoURL.ForceQuery || videoURL.Fragment != "" {
		return "", false
	}
	wantPath := "/v1/videos/" + url.PathEscape(task.GetUpstreamTaskID()) + "/content"
	if videoURL.EscapedPath() != wantPath {
		return "", false
	}
	key := strings.TrimSpace(task.PrivateData.Key)
	if key == "" && !channel.ChannelInfo.IsMultiKey {
		key = strings.TrimSpace(channel.Key)
	}
	return key, key != ""
}

func sameHTTPOrigin(left, right *url.URL) bool {
	if left == nil || right == nil || left.User != nil || right.User != nil {
		return false
	}
	leftScheme := strings.ToLower(left.Scheme)
	rightScheme := strings.ToLower(right.Scheme)
	if (leftScheme != "http" && leftScheme != "https") || leftScheme != rightScheme {
		return false
	}
	return strings.EqualFold(left.Hostname(), right.Hostname()) && effectiveHTTPPort(left) == effectiveHTTPPort(right)
}

func effectiveHTTPPort(parsed *url.URL) string {
	if port := parsed.Port(); port != "" {
		return port
	}
	if strings.EqualFold(parsed.Scheme, "https") {
		return "443"
	}
	return "80"
}
