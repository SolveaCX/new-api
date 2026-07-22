// Package compute is a thin server-side wrapper around the upstream GPU
// marketplace API used to power the "flatkey Compute" rental line.
//
// WHITELABEL (critical): the upstream provider (Vast.ai) MUST NOT be perceived
// by customers or the admin UI. This package is the ONLY place that knows the
// provider's base URL, auth scheme, and wire format. Everything it returns to
// callers is mapped into flatkey-branded structs whose provider-identifying
// fields are tagged `json:"-"` so they can never leak through an API response.
// The API key is read from the VAST_API_KEY environment variable and is NEVER
// hardcoded or logged.
package compute

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	// vastAPIBaseURL is the upstream marketplace API base. Internal only.
	vastAPIBaseURL = "https://console.vast.ai/api/v0"
	// vastAPIKeyEnv is the env var holding the upstream bearer token.
	vastAPIKeyEnv      = "VAST_API_KEY"
	vastRequestTimeout = 20 * time.Second
)

// ErrProviderNotConfigured is returned when VAST_API_KEY is not set. Callers
// should surface a generic "compute provider not configured" message and must
// not echo the provider name to clients.
var ErrProviderNotConfigured = errors.New("compute provider is not configured")

// Offer is the whitelabeled view of an upstream GPU offer.
//
// Only GPU / price / spec fields are serialized. The upstream contract/host/
// machine identifiers are internal-only (`json:"-"`) so a proxied offers
// response can be returned to the admin UI without leaking the marketplace.
type Offer struct {
	// ---- Public / whitelabeled fields ----
	GpuName     string  `json:"gpu_name"`
	NumGpus     int     `json:"num_gpus"`
	GpuRamGB    float64 `json:"gpu_ram_gb"`
	CostPerHour float64 `json:"cost_per_hour"`
	CpuCores    float64 `json:"cpu_cores"`
	RamGB       float64 `json:"ram_gb"`
	DiskGB      float64 `json:"disk_gb"`
	Reliability float64 `json:"reliability"`

	// ---- INTERNAL-ONLY (whitelabel): upstream identifiers, never serialized ----
	ContractID int    `json:"-"`
	HostID     int    `json:"-"`
	MachineID  int    `json:"-"`
	Datacenter string `json:"-"`
}

// RemoteInstance is the whitelabeled view of an upstream running instance.
// The upstream instance/contract id is retained internally for reconciliation
// but is never serialized to clients.
type RemoteInstance struct {
	Status  string `json:"status"`
	GpuName string `json:"gpu_name"`

	ContractID int    `json:"-"`
	HostIP     string `json:"-"`
	HostPort   int    `json:"-"`
}

// --- upstream wire structs (internal only) ---

type vastBundlesResponse struct {
	Offers []vastOffer `json:"offers"`
}

type vastOffer struct {
	ID          int     `json:"id"`
	HostID      int     `json:"host_id"`
	MachineID   int     `json:"machine_id"`
	GpuName     string  `json:"gpu_name"`
	NumGpus     int     `json:"num_gpus"`
	GpuRam      float64 `json:"gpu_ram"` // MB
	DphTotal    float64 `json:"dph_total"`
	CpuCores    float64 `json:"cpu_cores"`
	CpuRam      float64 `json:"cpu_ram"`    // MB
	DiskSpace   float64 `json:"disk_space"` // GB
	Reliability float64 `json:"reliability2"`
	Geolocation string  `json:"geolocation"`
}

type vastInstancesResponse struct {
	Instances []vastInstance `json:"instances"`
}

type vastInstance struct {
	ID           int     `json:"id"`
	ActualStatus string  `json:"actual_status"`
	GpuName      string  `json:"gpu_name"`
	PublicIPAddr string  `json:"public_ipaddr"`
	SSHPort      int     `json:"ssh_port"`
	DphTotal     float64 `json:"dph_total"`
}

func apiKey() (string, error) {
	key := strings.TrimSpace(os.Getenv(vastAPIKeyEnv))
	if key == "" {
		return "", ErrProviderNotConfigured
	}
	return key, nil
}

func httpClient() *http.Client {
	return &http.Client{Timeout: vastRequestTimeout}
}

// doRequest performs an authenticated request against the upstream API and
// decodes the JSON body into out (when non-nil). The bearer token is attached
// here and nowhere else.
func doRequest(method, endpoint string, out any) error {
	key, err := apiKey()
	if err != nil {
		return err
	}
	req, err := http.NewRequest(method, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := httpClient().Do(req)
	if err != nil {
		// Deliberately do not wrap with the provider name.
		return fmt.Errorf("compute upstream request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("compute upstream returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out == nil {
		return nil
	}
	return common.DecodeJson(resp.Body, out)
}

// buildOffersQuery constructs the JSON `q` query for the bundles endpoint.
// Marshaling goes through common.Marshal (Rule 1).
func buildOffersQuery(gpuName string) (string, error) {
	q := map[string]any{
		"rentable": map[string]any{"eq": true},
		"order":    [][]string{{"dph_total", "asc"}},
	}
	if strings.TrimSpace(gpuName) != "" {
		q["gpu_name"] = map[string]any{"eq": gpuName}
	}
	raw, err := common.Marshal(q)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// SearchOffers queries available upstream GPU offers, optionally filtered by
// GPU name. Returned offers are whitelabeled — provider identifiers live only
// on the internal (`json:"-"`) fields.
func SearchOffers(gpuName string) ([]Offer, error) {
	q, err := buildOffersQuery(gpuName)
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("%s/bundles/?q=%s", vastAPIBaseURL, url.QueryEscape(q))

	var resp vastBundlesResponse
	if err := doRequest(http.MethodGet, endpoint, &resp); err != nil {
		return nil, err
	}

	offers := make([]Offer, 0, len(resp.Offers))
	for _, o := range resp.Offers {
		offers = append(offers, Offer{
			GpuName:     o.GpuName,
			NumGpus:     o.NumGpus,
			GpuRamGB:    mbToGB(o.GpuRam),
			CostPerHour: o.DphTotal,
			CpuCores:    o.CpuCores,
			RamGB:       mbToGB(o.CpuRam),
			DiskGB:      o.DiskSpace,
			Reliability: o.Reliability,
			ContractID:  o.ID,
			HostID:      o.HostID,
			MachineID:   o.MachineID,
			Datacenter:  o.Geolocation,
		})
	}
	return offers, nil
}

// ListRemoteInstances returns the caller's currently provisioned upstream
// instances, whitelabeled.
func ListRemoteInstances() ([]RemoteInstance, error) {
	endpoint := fmt.Sprintf("%s/instances/", vastAPIBaseURL)
	var resp vastInstancesResponse
	if err := doRequest(http.MethodGet, endpoint, &resp); err != nil {
		return nil, err
	}
	instances := make([]RemoteInstance, 0, len(resp.Instances))
	for _, in := range resp.Instances {
		instances = append(instances, RemoteInstance{
			Status:     mapUpstreamStatus(in.ActualStatus),
			GpuName:    in.GpuName,
			ContractID: in.ID,
			HostIP:     in.PublicIPAddr,
			HostPort:   in.SSHPort,
		})
	}
	return instances, nil
}

// StopNode tears down (deletes) the upstream instance identified by the
// internal contract id.
func StopNode(contractID string) error {
	contractID = strings.TrimSpace(contractID)
	if contractID == "" {
		return errors.New("compute node has no upstream contract to stop")
	}
	endpoint := fmt.Sprintf("%s/instances/%s/", vastAPIBaseURL, url.PathEscape(contractID))
	return doRequest(http.MethodDelete, endpoint, nil)
}

// ProvisionNode is a STUB for the next PR.
//
// TODO(next-PR): implement GPU auto-provisioning — select a cheap Offer via
// SearchOffers, PUT /asks/{contract_id}/ with a vLLM launch image + onstart
// script serving the open model, poll until running, capture host_ip/host_port,
// then create the fronting flatkey Channel and persist a ComputeNode row with
// the internal provider fields populated. Intentionally not implemented here to
// keep this PR reviewable (admin dashboard + read/stop only).
func ProvisionNode(_ Offer) error {
	return errors.New("compute provisioning is not implemented yet")
}

func mbToGB(mb float64) float64 {
	if mb <= 0 {
		return 0
	}
	return mb / 1024.0
}

// mapUpstreamStatus normalizes an upstream instance status string into the
// flatkey ComputeNode status vocabulary. Kept here so the mapping (and any
// provider-specific status vocabulary) never leaks past this package.
func mapUpstreamStatus(actual string) string {
	switch strings.ToLower(strings.TrimSpace(actual)) {
	case "running":
		return "running"
	case "exited", "stopped", "offline":
		return "stopped"
	case "created", "loading", "starting", "scheduling":
		return "provisioning"
	case "":
		return "provisioning"
	default:
		return "error"
	}
}
