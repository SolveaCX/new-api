package auto_model

import (
	"context"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/stretchr/testify/require"
)

type classifierRoundTripFunc func(*http.Request) (*http.Response, error)

func (f classifierRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestClassifierAcceptsOnlyFixedRoutes(t *testing.T) {
	for _, expected := range []Route{RouteGeneral, RouteCoding, RouteReasoning, RouteTranslation} {
		t.Run(string(expected), func(t *testing.T) {
			var calls atomic.Int32
			client := &http.Client{Transport: classifierRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls.Add(1)
				require.Equal(t, "https://classifier.example/v1/chat/completions", req.URL.String())
				require.Equal(t, "Bearer secret-key", req.Header.Get("Authorization"))
				require.Equal(t, AutoHopValue, req.Header.Get(AutoHopHeader))
				requestBody, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				var payload map[string]any
				require.NoError(t, common.Unmarshal(requestBody, &payload))
				require.Equal(t, "router-mini", payload["model"])
				require.NotNil(t, payload["response_format"])

				body, err := common.Marshal(map[string]any{
					"choices": []any{map[string]any{"message": map[string]any{"content": `{"route":"` + string(expected) + `"}`}}},
				})
				require.NoError(t, err)
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header)}, nil
			})}

			route, err := NewClassifier(client, 0).Classify(context.Background(), classifierSnapshot(), "[user]\nclassify this")
			require.NoError(t, err)
			require.Equal(t, expected, route)
			require.Equal(t, int32(1), calls.Load())
		})
	}
}

type classifierResolverFunc func(context.Context, string, string) ([]netip.Addr, error)

func (f classifierResolverFunc) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	return f(ctx, network, host)
}

func TestClassifierProductionTargetPolicy(t *testing.T) {
	for _, raw := range []string{
		"http://classifier.example/v1",
		"https://user:pass@classifier.example/v1",
		"https://classifier.example:8443/v1",
		"https://classifier.example/v1?token=secret",
	} {
		_, err := validateClassifierBaseURL(raw)
		require.Error(t, err, raw)
	}
	for _, host := range []string{
		"127.0.0.1", "10.0.0.1", "169.254.169.254", "168.63.129.16", "100.64.0.1", "100.100.100.200",
		"::1", "fc00::1", "fd00:ec2::254", "fe80::1", "2001:db8::1", "metadata.google.internal",
	} {
		_, err := resolvePublicClassifierHost(context.Background(), host)
		require.Error(t, err, host)
	}
	_, err := resolvePublicClassifierHost(context.Background(), "8.8.8.8")
	require.NoError(t, err)

	resolver := classifierResolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("93.184.216.34"), netip.MustParseAddr("127.0.0.1")}, nil
	})
	_, err = resolvePublicClassifierHostWithResolver(context.Background(), resolver, "classifier.example")
	require.Error(t, err, "all DNS answers must pass the public-address policy")

	baseURL, err := validateClassifierBaseURL("https://8.8.8.8/v1")
	require.NoError(t, err)
	client, err := NewProductionClassifier(0).clientForRequest(context.Background(), baseURL, time.Second)
	require.NoError(t, err)
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	require.Nil(t, transport.Proxy, "production classifier must not inherit environment proxies")
	require.NotNil(t, transport.DialContext)
}

func TestClassifierRejectsRedirectWithoutFollowing(t *testing.T) {
	var calls atomic.Int32
	client := &http.Client{Transport: classifierRoundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{"https://redirect.example/v1"}},
			Body:       io.NopCloser(strings.NewReader("redirect")),
		}, nil
	})}
	_, err := NewClassifier(client, 0).Classify(context.Background(), classifierSnapshot(), "hello")
	require.Equal(t, ClassifierErrorHTTPStatus, ClassifierReason(err))
	require.Equal(t, int32(1), calls.Load())
}

func TestClassifierRejectsVirtualClassifierModelBeforeNetwork(t *testing.T) {
	var calls atomic.Int32
	client := &http.Client{Transport: classifierRoundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, nil
	})}
	snapshot := classifierSnapshot()
	snapshot.Config.ClassifierModel = "auto"
	_, err := NewClassifier(client, 0).Classify(context.Background(), snapshot, "hello")
	require.Equal(t, ClassifierErrorConfig, ClassifierReason(err))
	require.Zero(t, calls.Load())
}

func TestClassifierFailuresAreTypedAndDoNotRetry(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		content    string
		limit      int64
		wantReason ClassifierErrorReason
	}{
		{"http status", 429, "{}", 0, ClassifierErrorHTTPStatus},
		{"invalid envelope", 200, `{"choices":[]}`, 0, ClassifierErrorInvalidJSON},
		{"markdown", 200, responseEnvelope("```json\n{\"route\":\"coding\"}\n```"), 0, ClassifierErrorInvalidJSON},
		{"extra field", 200, responseEnvelope(`{"route":"coding","model":"secret"}`), 0, ClassifierErrorInvalidJSON},
		{"unknown route", 200, responseEnvelope(`{"route":"vision"}`), 0, ClassifierErrorInvalidRoute},
		{"too large", 200, strings.Repeat("x", 128), 32, ClassifierErrorResponseTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			client := &http.Client{Transport: classifierRoundTripFunc(func(*http.Request) (*http.Response, error) {
				calls.Add(1)
				return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(test.content)), Header: make(http.Header)}, nil
			})}
			_, err := NewClassifier(client, test.limit).Classify(context.Background(), classifierSnapshot(), "hello")
			require.Error(t, err)
			require.Equal(t, test.wantReason, ClassifierReason(err))
			require.Equal(t, int32(1), calls.Load())
			require.NotContains(t, err.Error(), "secret-key")
		})
	}
}

func TestClassifierTimeout(t *testing.T) {
	client := &http.Client{Transport: classifierRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})}
	snapshot := classifierSnapshot()
	snapshot.Config.ClassifierTimeoutMS = 200
	started := time.Now()
	_, err := NewClassifier(client, 0).Classify(context.Background(), snapshot, "hello")
	require.Equal(t, ClassifierErrorTimeout, ClassifierReason(err))
	require.Less(t, time.Since(started), time.Second)
}

func TestRouteModelAccessorsReturnCopies(t *testing.T) {
	snapshot := classifierSnapshot()
	snapshot.Config.Routes["coding"] = []string{"coder-a", "coder-b"}
	models := ModelsForRoute(snapshot, RouteCoding)
	models[0] = "mutated"
	require.Equal(t, "coder-a", snapshot.Config.Routes["coding"][0])
	require.Equal(t, "fallback", DefaultModel(snapshot))
}

func classifierSnapshot() *model_setting.AutoModelSnapshot {
	return &model_setting.AutoModelSnapshot{
		Config: model_setting.AutoModelConfig{
			Version:                 model_setting.AutoModelConfigVersion,
			Enabled:                 true,
			ClassifierBaseURL:       "https://classifier.example/v1",
			ClassifierModel:         "router-mini",
			ClassifierTimeoutMS:     800,
			ClassifierInputMaxChars: 8000,
			DefaultModel:            "fallback",
			Routes:                  map[string][]string{},
		},
		ClassifierAPIKey: "secret-key",
	}
}

func responseEnvelope(content string) string {
	body, err := common.Marshal(map[string]any{
		"choices": []any{map[string]any{"message": map[string]any{"content": content}}},
	})
	if err != nil {
		panic(err)
	}
	return string(body)
}
