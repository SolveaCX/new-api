package common

import (
	"strings"
	"testing"
)

func testBuildManifest(runID string, attempt string, job string, capabilities []int) BuildManifest {
	manifest := BuildManifest{
		SchemaVersion:                   BuildManifestSchemaVersion,
		Repo:                            "SolveaCX/new-api",
		BuildCommit:                     strings.Repeat("a", 40),
		GitHubRunID:                     runID,
		RunAttempt:                      attempt,
		BuildJobIdentity:                job,
		ProducerCapabilities:            capabilities,
		SupplierAdminSchemaCapabilities: []int{1},
	}
	manifest.BuildProvenanceID = BuildProvenanceID(manifest.Repo, manifest.GitHubRunID, manifest.RunAttempt, manifest.BuildJobIdentity)
	return manifest
}

func TestCanonicalBuildManifestIsReproducibleAndSorted(t *testing.T) {
	manifest := testBuildManifest("123", "2", "gcp-deploy-build", []int{3, 1, 2})
	first, err := CanonicalBuildManifestPayload(manifest)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CanonicalBuildManifestPayload(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || BuildManifestDigest(first) != BuildManifestDigest(second) {
		t.Fatal("same manifest input must produce identical canonical bytes and digest")
	}
	want := `{"schema_version":1,"repo":"SolveaCX/new-api","build_commit":"` + strings.Repeat("a", 40) + `","github_run_id":"123","run_attempt":"2","build_job_identity":"gcp-deploy-build","producer_capabilities":[1,2,3],"supplier_admin_schema_capabilities":[1],"build_provenance_id":"SolveaCX/new-api@123.2:gcp-deploy-build"}`
	if first != want {
		t.Fatalf("canonical payload\n got: %s\nwant: %s", first, want)
	}
	parsed, err := ParseBuildManifest(first, BuildManifestDigest(first))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.ManifestHashPayload != first || parsed.ManifestSHA256 != BuildManifestDigest(first) {
		t.Fatal("parsed status must preserve the exact payload and digest")
	}
}

func TestCanonicalBuildManifestAdminSchemaCapabilitiesAreSortedAndStrict(t *testing.T) {
	manifest := testBuildManifest("123", "1", "gcp-deploy-build", []int{1})
	manifest.SupplierAdminSchemaCapabilities = []int{3, 1, 2}
	payload, err := CanonicalBuildManifestPayload(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(payload, `"supplier_admin_schema_capabilities":[1,2,3]`) {
		t.Fatalf("admin schema capabilities were not canonicalized: %s", payload)
	}
	for _, invalid := range [][]int{nil, {}, {0}, {-1}, {1, 1}, {2}} {
		candidate := manifest
		candidate.SupplierAdminSchemaCapabilities = invalid
		if _, err := CanonicalBuildManifestPayload(candidate); err == nil {
			t.Fatalf("invalid admin schema capabilities unexpectedly accepted: %v", invalid)
		}
	}
}

func TestBuildManifestProvenanceChangesAcrossRunCoordinates(t *testing.T) {
	base := testBuildManifest("123", "1", "gcp-deploy-build", []int{1})
	basePayload, err := CanonicalBuildManifestPayload(base)
	if err != nil {
		t.Fatal(err)
	}
	cases := []BuildManifest{
		testBuildManifest("124", "1", "gcp-deploy-build", []int{1}),
		testBuildManifest("123", "2", "gcp-deploy-build", []int{1}),
		testBuildManifest("123", "1", "gcp-deploy-router", []int{1}),
	}
	for _, candidate := range cases {
		payload, err := CanonicalBuildManifestPayload(candidate)
		if err != nil {
			t.Fatal(err)
		}
		if candidate.BuildProvenanceID == base.BuildProvenanceID || BuildManifestDigest(payload) == BuildManifestDigest(basePayload) {
			t.Fatal("run, attempt, and job identity must each change provenance and manifest digest")
		}
	}
}

func TestParseBuildManifestFailsClosed(t *testing.T) {
	manifest := testBuildManifest("123", "1", "gcp-deploy-build", []int{1})
	payload, err := CanonicalBuildManifestPayload(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseBuildManifest(payload, strings.Repeat("0", 64)); err == nil {
		t.Fatal("tampered manifest binding must fail")
	}
	if _, err := ParseBuildManifest(payload+"\n", BuildManifestDigest(payload+"\n")); err == nil {
		t.Fatal("non-canonical payload must fail")
	}
}
