package auto_model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

type Route string

const (
	RouteGeneral     Route = "general"
	RouteCoding      Route = "coding"
	RouteReasoning   Route = "reasoning"
	RouteTranslation Route = "translation"
)

const DefaultClassifierResponseLimit int64 = 8 << 10

const (
	AutoHopHeader = "X-NewAPI-Auto-Hop"
	AutoHopValue  = "1"
)

type ClassifierErrorReason string

const (
	ClassifierErrorTimeout          ClassifierErrorReason = "timeout"
	ClassifierErrorHTTPStatus       ClassifierErrorReason = "http_status"
	ClassifierErrorInvalidJSON      ClassifierErrorReason = "invalid_json"
	ClassifierErrorInvalidRoute     ClassifierErrorReason = "invalid_route"
	ClassifierErrorResponseTooLarge ClassifierErrorReason = "response_too_large"
	ClassifierErrorConfig           ClassifierErrorReason = "config"
)

type ClassifierError struct {
	Reason ClassifierErrorReason
	err    error
}

func (e *ClassifierError) Error() string {
	if e == nil || e.err == nil {
		return "auto model classifier failed"
	}
	return "auto model classifier failed: " + e.err.Error()
}

func (e *ClassifierError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func ClassifierReason(err error) ClassifierErrorReason {
	var classifierErr *ClassifierError
	if errors.As(err, &classifierErr) {
		return classifierErr.Reason
	}
	return ClassifierErrorConfig
}

type Classifier struct {
	client        *http.Client
	responseLimit int64
}

// NewClassifier accepts an explicitly injected client for deterministic tests
// and other trusted internal callers. Production Auto routing must use
// NewProductionClassifier so it cannot inherit general proxy settings.
func NewClassifier(client *http.Client, responseLimit int64) *Classifier {
	return newClassifier(client, responseLimit)
}

// NewProductionClassifier always uses a dedicated direct transport with
// fail-closed target validation and never inherits HTTP_PROXY, HTTPS_PROXY, or
// the application's general HTTP client settings.
func NewProductionClassifier(responseLimit int64) *Classifier {
	return newClassifier(nil, responseLimit)
}

func newClassifier(client *http.Client, responseLimit int64) *Classifier {
	if responseLimit <= 0 {
		responseLimit = DefaultClassifierResponseLimit
	}
	return &Classifier{client: client, responseLimit: responseLimit}
}

// Classify performs exactly one OpenAI-compatible Chat Completions request.
// Any failure is returned for the caller to handle with the configured default
// model; this method never retries and never logs prompt or credential data.
func (c *Classifier) Classify(ctx context.Context, snapshot *model_setting.AutoModelSnapshot, text string) (Route, error) {
	if c == nil || snapshot == nil || !snapshot.Config.Enabled {
		return "", classifierError(ClassifierErrorConfig, "classifier is not configured")
	}
	config := snapshot.Config
	baseURL, err := validateClassifierBaseURL(config.ClassifierBaseURL)
	if err != nil {
		return "", classifierError(ClassifierErrorConfig, "classifier URL is invalid")
	}
	if config.ClassifierModel == "" || config.ClassifierModel == "auto" || snapshot.ClassifierAPIKey == "" || strings.TrimSpace(text) == "" {
		return "", classifierError(ClassifierErrorConfig, "classifier settings are incomplete")
	}
	if config.ClassifierTimeoutMS < 200 || config.ClassifierTimeoutMS > 2000 {
		return "", classifierError(ClassifierErrorConfig, "classifier timeout is invalid")
	}

	payload := classifierRequest{
		Model: config.ClassifierModel,
		Messages: []classifierMessage{
			{Role: "system", Content: classifierSystemPrompt},
			{Role: "user", Content: text},
		},
		Temperature: 0,
		ResponseFormat: classifierResponseFormat{
			Type: "json_schema",
			JSONSchema: classifierJSONSchema{
				Name:   "auto_model_route",
				Strict: true,
				Schema: classifierSchemaBody{
					Type:                 "object",
					Properties:           map[string]classifierSchemaProperty{"route": {Type: "string", Enum: []Route{RouteGeneral, RouteCoding, RouteReasoning, RouteTranslation}}},
					Required:             []string{"route"},
					AdditionalProperties: false,
				},
			},
		},
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return "", classifierError(ClassifierErrorConfig, "cannot encode classifier request")
	}

	requestCtx, cancel := context.WithTimeout(ctx, time.Duration(config.ClassifierTimeoutMS)*time.Millisecond)
	defer cancel()
	endpoint := strings.TrimRight(baseURL.String(), "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", classifierError(ClassifierErrorConfig, "cannot create classifier request")
	}
	req.Header.Set("Authorization", "Bearer "+snapshot.ClassifierAPIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(AutoHopHeader, AutoHopValue)

	client, err := c.clientForRequest(requestCtx, baseURL, time.Duration(config.ClassifierTimeoutMS)*time.Millisecond)
	if err != nil {
		return "", classifierError(ClassifierErrorConfig, "classifier target is not allowed")
	}
	clientCopy := *client
	client = &clientCopy
	if c.client == nil {
		defer client.CloseIdleConnections()
	}
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
			return "", classifierError(ClassifierErrorTimeout, "request timed out")
		}
		return "", classifierError(ClassifierErrorHTTPStatus, "request failed")
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", classifierError(ClassifierErrorHTTPStatus, fmt.Sprintf("unexpected HTTP status %d", response.StatusCode))
	}
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, c.responseLimit+1))
	if err != nil {
		return "", classifierError(ClassifierErrorInvalidJSON, "cannot read response")
	}
	if int64(len(responseBody)) > c.responseLimit {
		return "", classifierError(ClassifierErrorResponseTooLarge, "response exceeds limit")
	}

	var result classifierResponse
	if err := common.Unmarshal(responseBody, &result); err != nil || len(result.Choices) != 1 {
		return "", classifierError(ClassifierErrorInvalidJSON, "response envelope is invalid")
	}
	return parseRoute(result.Choices[0].Message.Content)
}

func (c *Classifier) clientForRequest(ctx context.Context, baseURL *url.URL, timeout time.Duration) (*http.Client, error) {
	if c.client != nil {
		return c.client, nil
	}
	if _, err := resolvePublicClassifierHost(ctx, baseURL.Hostname()); err != nil {
		return nil, err
	}
	directDialer := &classifierDirectDialer{
		resolver: net.DefaultResolver,
		dialer: net.Dialer{
			Timeout:   timeout,
			KeepAlive: 30 * time.Second,
		},
	}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           directDialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          4,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   timeout,
		ExpectContinueTimeout: time.Second,
	}
	return &http.Client{Transport: transport, Timeout: timeout}, nil
}

type classifierDirectDialer struct {
	resolver classifierIPResolver
	dialer   net.Dialer
}

type classifierIPResolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

func (d *classifierDirectDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port != "443" {
		return nil, errors.New("classifier target port is not allowed")
	}
	ips, err := resolvePublicClassifierHostWithResolver(ctx, d.resolver, host)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, ip := range ips {
		conn, dialErr := d.dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr == nil {
		lastErr = errors.New("classifier target has no dialable address")
	}
	return nil, lastErr
}

func validateClassifierBaseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Host == "" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("invalid classifier URL")
	}
	if port := u.Port(); port != "" && port != "443" {
		return nil, errors.New("classifier URL port is not allowed")
	}
	return u, nil
}

func resolvePublicClassifierHost(ctx context.Context, host string) ([]netip.Addr, error) {
	return resolvePublicClassifierHostWithResolver(ctx, net.DefaultResolver, host)
}

func resolvePublicClassifierHostWithResolver(ctx context.Context, resolver classifierIPResolver, host string) ([]netip.Addr, error) {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" || isClassifierMetadataHost(host) {
		return nil, errors.New("classifier target host is not allowed")
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		ip = ip.Unmap()
		if !isPublicClassifierIP(ip) {
			return nil, errors.New("classifier target address is not allowed")
		}
		return []netip.Addr{ip}, nil
	}
	ips, err := resolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return nil, errors.New("classifier target cannot be resolved")
	}
	for i := range ips {
		ips[i] = ips[i].Unmap()
		if !isPublicClassifierIP(ips[i]) {
			return nil, errors.New("classifier target resolved to a disallowed address")
		}
	}
	return ips, nil
}

func isClassifierMetadataHost(host string) bool {
	switch host {
	case "localhost", "metadata", "metadata.google.internal", "metadata.azure.internal", "instance-data", "instance-data.ec2.internal":
		return true
	default:
		return strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal")
	}
}

var classifierDeniedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("168.63.129.16/32"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("2001::/32"),
	netip.MustParsePrefix("2001:10::/28"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
}

func isPublicClassifierIP(ip netip.Addr) bool {
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	for _, prefix := range classifierDeniedPrefixes {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}

func ModelsForRoute(snapshot *model_setting.AutoModelSnapshot, route Route) []string {
	if snapshot == nil {
		return nil
	}
	models := snapshot.Config.Routes[string(route)]
	return append([]string(nil), models...)
}

func DefaultModel(snapshot *model_setting.AutoModelSnapshot) string {
	if snapshot == nil {
		return ""
	}
	return snapshot.Config.DefaultModel
}

func parseRoute(content string) (Route, error) {
	var fields map[string]json.RawMessage
	if err := common.Unmarshal([]byte(strings.TrimSpace(content)), &fields); err != nil || len(fields) != 1 {
		return "", classifierError(ClassifierErrorInvalidJSON, "route response must be a single-field JSON object")
	}
	rawRoute, exists := fields["route"]
	if !exists {
		return "", classifierError(ClassifierErrorInvalidJSON, "route field is missing")
	}
	var route Route
	if err := common.Unmarshal(rawRoute, &route); err != nil {
		return "", classifierError(ClassifierErrorInvalidJSON, "route field is invalid")
	}
	if !validRoute(route) {
		return "", classifierError(ClassifierErrorInvalidRoute, "route is not supported")
	}
	return route, nil
}

func validRoute(route Route) bool {
	switch route {
	case RouteGeneral, RouteCoding, RouteReasoning, RouteTranslation:
		return true
	default:
		return false
	}
}

func classifierError(reason ClassifierErrorReason, message string) error {
	return &ClassifierError{Reason: reason, err: errors.New(message)}
}

const classifierSystemPrompt = `Classify the request into exactly one route: general, coding, reasoning, or translation. Return only JSON matching {"route":"<route>"}. Do not include markdown or explanation.`

type classifierRequest struct {
	Model          string                   `json:"model"`
	Messages       []classifierMessage      `json:"messages"`
	Temperature    int                      `json:"temperature"`
	ResponseFormat classifierResponseFormat `json:"response_format"`
}

type classifierMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type classifierResponseFormat struct {
	Type       string               `json:"type"`
	JSONSchema classifierJSONSchema `json:"json_schema"`
}

type classifierJSONSchema struct {
	Name   string               `json:"name"`
	Strict bool                 `json:"strict"`
	Schema classifierSchemaBody `json:"schema"`
}

type classifierSchemaBody struct {
	Type                 string                              `json:"type"`
	Properties           map[string]classifierSchemaProperty `json:"properties"`
	Required             []string                            `json:"required"`
	AdditionalProperties bool                                `json:"additionalProperties"`
}

type classifierSchemaProperty struct {
	Type string  `json:"type"`
	Enum []Route `json:"enum"`
}

type classifierResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}
