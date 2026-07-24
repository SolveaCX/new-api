package common

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const BuildManifestSchemaVersion = 1

// BuildManifestPayload and BuildManifestSHA256 are injected by the release
// build. They deliberately contain no OCI digest: the registry digest exists
// only after the image has been pushed and is bound by the deployment workflow.
var BuildManifestPayload string
var BuildManifestSHA256 string

type BuildManifest struct {
	SchemaVersion                   int    `json:"schema_version"`
	Repo                            string `json:"repo"`
	BuildCommit                     string `json:"build_commit"`
	GitHubRunID                     string `json:"github_run_id"`
	RunAttempt                      string `json:"run_attempt"`
	BuildJobIdentity                string `json:"build_job_identity"`
	ProducerCapabilities            []int  `json:"producer_capabilities"`
	SupplierAdminSchemaCapabilities []int  `json:"supplier_admin_schema_capabilities"`
	BuildProvenanceID               string `json:"build_provenance_id"`
}

type BuildManifestStatus struct {
	BuildManifest
	ManifestHashPayload string `json:"manifest_hash_payload"`
	ManifestSHA256      string `json:"manifest_sha256"`
}

var currentBuildManifest BuildManifestStatus

func init() {
	if BuildManifestPayload == "" && BuildManifestSHA256 == "" {
		manifest := BuildManifest{
			SchemaVersion:                   BuildManifestSchemaVersion,
			Repo:                            "local",
			BuildCommit:                     "unknown",
			GitHubRunID:                     "0",
			RunAttempt:                      "0",
			BuildJobIdentity:                "local",
			ProducerCapabilities:            []int{1},
			SupplierAdminSchemaCapabilities: []int{1},
		}
		manifest.BuildProvenanceID = BuildProvenanceID(manifest.Repo, manifest.GitHubRunID, manifest.RunAttempt, manifest.BuildJobIdentity)
		payload, err := CanonicalBuildManifestPayload(manifest)
		if err != nil {
			panic(err)
		}
		BuildManifestPayload = payload
		BuildManifestSHA256 = BuildManifestDigest(payload)
	}

	manifest, err := ParseBuildManifest(BuildManifestPayload, BuildManifestSHA256)
	if err != nil {
		panic(fmt.Errorf("invalid embedded build manifest: %w", err))
	}
	currentBuildManifest = manifest
}

func CurrentBuildManifest() BuildManifestStatus {
	manifest := currentBuildManifest
	manifest.ProducerCapabilities = append([]int(nil), currentBuildManifest.ProducerCapabilities...)
	manifest.SupplierAdminSchemaCapabilities = append([]int(nil), currentBuildManifest.SupplierAdminSchemaCapabilities...)
	return manifest
}

func ParseBuildManifest(payload string, expectedSHA256 string) (BuildManifestStatus, error) {
	if payload == "" || len(expectedSHA256) != sha256.Size*2 {
		return BuildManifestStatus{}, errors.New("build manifest payload and SHA-256 are required")
	}
	if expectedSHA256 != strings.ToLower(expectedSHA256) {
		return BuildManifestStatus{}, errors.New("build manifest SHA-256 must be lowercase hexadecimal")
	}
	if _, err := hex.DecodeString(expectedSHA256); err != nil {
		return BuildManifestStatus{}, errors.New("build manifest SHA-256 must be lowercase hexadecimal")
	}
	if BuildManifestDigest(payload) != expectedSHA256 {
		return BuildManifestStatus{}, errors.New("build manifest SHA-256 mismatch")
	}

	var manifest BuildManifest
	if err := Unmarshal([]byte(payload), &manifest); err != nil {
		return BuildManifestStatus{}, fmt.Errorf("decode build manifest: %w", err)
	}
	canonical, err := CanonicalBuildManifestPayload(manifest)
	if err != nil {
		return BuildManifestStatus{}, err
	}
	if canonical != payload {
		return BuildManifestStatus{}, errors.New("build manifest payload is not canonical")
	}
	return BuildManifestStatus{
		BuildManifest:       manifest,
		ManifestHashPayload: payload,
		ManifestSHA256:      expectedSHA256,
	}, nil
}

func CanonicalBuildManifestPayload(manifest BuildManifest) (string, error) {
	if manifest.SchemaVersion != BuildManifestSchemaVersion {
		return "", errors.New("unsupported build manifest schema version")
	}
	for name, value := range map[string]string{
		"repo": manifest.Repo, "build_commit": manifest.BuildCommit,
		"github_run_id": manifest.GitHubRunID, "run_attempt": manifest.RunAttempt,
		"build_job_identity": manifest.BuildJobIdentity,
	} {
		if !validBuildManifestText(value) {
			return "", fmt.Errorf("invalid build manifest %s", name)
		}
	}
	producerCapabilities, err := canonicalBuildCapabilities("producer capabilities", manifest.ProducerCapabilities)
	if err != nil {
		return "", err
	}
	adminSchemaCapabilities, err := canonicalBuildCapabilities("supplier admin schema capabilities", manifest.SupplierAdminSchemaCapabilities)
	if err != nil {
		return "", err
	}
	if adminSchemaCapabilities[0] != 1 {
		return "", errors.New("supplier admin schema capabilities must include current capability 1")
	}
	manifest.ProducerCapabilities = producerCapabilities
	manifest.SupplierAdminSchemaCapabilities = adminSchemaCapabilities
	expectedProvenance := BuildProvenanceID(manifest.Repo, manifest.GitHubRunID, manifest.RunAttempt, manifest.BuildJobIdentity)
	if manifest.BuildProvenanceID != expectedProvenance {
		return "", errors.New("build provenance ID does not match its inputs")
	}
	payload, err := Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("encode build manifest: %w", err)
	}
	return string(payload), nil
}

func canonicalBuildCapabilities(name string, values []int) ([]int, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("%s are required", name)
	}
	capabilities := append([]int(nil), values...)
	sort.Ints(capabilities)
	for i, capability := range capabilities {
		if capability <= 0 || (i > 0 && capability == capabilities[i-1]) {
			return nil, fmt.Errorf("%s must be unique positive integers", name)
		}
	}
	return capabilities, nil
}

func BuildProvenanceID(repo string, runID string, runAttempt string, jobIdentity string) string {
	return repo + "@" + runID + "." + runAttempt + ":" + jobIdentity
}

func BuildManifestDigest(payload string) string {
	digest := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(digest[:])
}

func validBuildManifestText(value string) bool {
	if value == "" || len(value) > 160 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '.' || r == '_' || r == '-' || r == '/' {
			continue
		}
		return false
	}
	return true
}
