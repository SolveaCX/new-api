package common_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/types"
)

func TestSupplierReleaseManifestCapabilitiesTrackSourceV1(t *testing.T) {
	if types.SupplierAccountingProducerCapabilityV1 != 1 {
		t.Fatalf("release manifest producer capability is fixed at 1; update every release source when capability advances (source=%d)", types.SupplierAccountingProducerCapabilityV1)
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve source contract test path")
	}
	root := filepath.Dir(filepath.Dir(filename))
	assertReleaseSource := func(relative string, required ...string) {
		t.Helper()
		contents, err := os.ReadFile(filepath.Join(root, relative))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		source := string(contents)
		for _, forbidden := range []string{"SUPPLIER_PRODUCER_CAPABILITIES", "SUPPLIER_ADMIN_SCHEMA_CAPABILITIES"} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s permits release capability override through %s", relative, forbidden)
			}
		}
		for _, contract := range required {
			if !strings.Contains(source, contract) {
				t.Fatalf("%s does not bind the current producer/admin capabilities: missing %q", relative, contract)
			}
		}
	}

	assertReleaseSource(".github/workflows/gcp-deploy.yml", `"gcp-deploy-build" "1" "1"`)
	assertReleaseSource(".github/workflows/gcp-deploy-staging.yml",
		`"gcp-deploy-staging-build" "1" "1"`,
		`supplier-deploy-verify.sh capabilities /tmp/status.json 1`)
	staging, err := os.ReadFile(filepath.Join(root, ".github/workflows/gcp-deploy-staging.yml"))
	if err != nil {
		t.Fatalf("read staging workflow: %v", err)
	}
	if strings.Contains(string(staging), "SUPPLIER_ACCEPTED_CAPABILITIES") {
		t.Fatal("staging capability preflight must remain fixed to source capability V1")
	}
	assertReleaseSource("Dockerfile", `"${BUILD_JOB_IDENTITY}" "1" "1"`)
}
