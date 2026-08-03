# Browser QA Terraform State Isolation Implementation Plan

> **Safety addendum:** The original single-phase 35-resource bootstrap steps are superseded by [Browser QA Two-Phase Secret Bootstrap Implementation Plan](./2026-08-03-browser-qa-two-phase-secret-bootstrap.md). State isolation remains valid; do not apply an old combined saved plan.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create a dedicated Terraform root/state that can bootstrap staging Browser QA without planning or changing production-managed resources, then prepare the existing staging auto-`core` workflow for live acceptance.

**Architecture:** `deploy/gcp/envs/browser-qa-staging/` owns only the 35 Browser QA resources and eight `browser_qa_*` outputs under backend prefix `envs/browser-qa-staging`. The production root relinquishes Browser QA ownership, project API enablement remains in the production state, and a versioned Python guard accepts only the exact create-only bootstrap plan. Human operators retain all apply, Secret version, GitHub Variable, and workflow-dispatch authority.

**Tech Stack:** Terraform >= 1.5, HashiCorp Google provider `~> 6.13`, Python 3 standard library/unittest, Google Cloud CLI, GitHub CLI, GitHub Actions.

**Approved design:** [2026-08-03 Browser QA Terraform State Isolation Design](../specs/2026-08-03-browser-qa-terraform-state-isolation-design.md)

---

## Scope and stop conditions

- Implementation work happens only in the isolated worktree `C:\Users\11247\.config\superpowers\worktrees\new-api\browser-qa-live-acceptance`.
- No task applies `deploy/gcp/envs/prod`, writes a Secret version, writes a GitHub Variable, pushes `staging`, or dispatches a workflow from an agent session.
- Human bootstrap stops if the independent state is non-empty, any same-name live QA resource exists, or the saved plan contains anything other than the exact 35 QA `create` actions and eight QA outputs.
- Staging is pushed only after infrastructure, three Secret versions, and five repository Variables pass a fresh readiness audit.
- Main and production release workflows remain out of scope.

## File responsibility map

| Path | Responsibility |
|---|---|
| `deploy/gcp/envs/browser-qa-staging/backend.tf` | Dedicated GCS backend prefix only |
| `deploy/gcp/envs/browser-qa-staging/versions.tf` | Terraform/provider contract and Google provider configuration |
| `deploy/gcp/envs/browser-qa-staging/variables.tf` | Non-sensitive, environment-pinned `project_id` and `region` inputs |
| `deploy/gcp/envs/browser-qa-staging/terraform.tfvars` | Checked-in non-secret staging QA project/region values |
| `deploy/gcp/envs/browser-qa-staging/browser_qa.tf` | Unconditional Browser QA resources; no shared API ownership |
| `deploy/gcp/envs/browser-qa-staging/outputs.tf` | Stable eight-output GitHub/runbook contract |
| `deploy/gcp/envs/browser-qa-staging/.terraform.lock.hcl` | Reproducible Google provider checksums |
| `scripts/browser_qa/terraform_plan_guard.py` | Exact bootstrap resource/output/action whitelist |
| `scripts/browser_qa/tests/test_terraform_state_isolation_contract.py` | Root/backend/ownership/API boundary tests |
| `scripts/browser_qa/tests/test_terraform_plan_guard.py` | Positive and adversarial saved-plan tests |
| `scripts/browser_qa/tests/test_terraform_contract.py` | Existing QA resource/security contract retargeted to the new root |
| `scripts/browser_qa/tests/test_operations_contract.py` | Runbook path, guard, preflight, and safe variable-write contract |
| `deploy/gcp/docs/OPERATIONS.md` | Human-only bootstrap/recovery procedure for the new root |
| `deploy/gcp/envs/prod/browser_qa.tf` | Deleted after ownership moves |
| `deploy/gcp/envs/prod/variables.tf` | Remove `enable_browser_qa` only |
| `deploy/gcp/envs/prod/terraform.tfvars` | Remove `enable_browser_qa = true` only |
| `deploy/gcp/envs/prod/outputs.tf` | Remove the eight `browser_qa_*` outputs only |

### Task 1: Add and validate the independent Terraform root shell

**Files:**
- Create: `scripts/browser_qa/tests/test_terraform_state_isolation_contract.py`
- Create: `deploy/gcp/envs/browser-qa-staging/backend.tf`
- Create: `deploy/gcp/envs/browser-qa-staging/versions.tf`
- Create: `deploy/gcp/envs/browser-qa-staging/variables.tf`
- Create: `deploy/gcp/envs/browser-qa-staging/terraform.tfvars`
- Create: `deploy/gcp/envs/browser-qa-staging/.terraform.lock.hcl`

- [ ] **Step 1: Write the failing root-boundary test**

Create `scripts/browser_qa/tests/test_terraform_state_isolation_contract.py` with these contracts:

```python
import re
import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[3]
QA_ROOT = REPO_ROOT / "deploy" / "gcp" / "envs" / "browser-qa-staging"
PROD_ROOT = REPO_ROOT / "deploy" / "gcp" / "envs" / "prod"


class BrowserQaTerraformStateIsolationContractTest(unittest.TestCase):
    def test_dedicated_backend_uses_exact_prefix(self):
        backend = (QA_ROOT / "backend.tf").read_text(encoding="utf-8")
        self.assertRegex(backend, r'bucket\s*=\s*"vocai-gemini-prod-newapi-tfstate"')
        self.assertRegex(backend, r'prefix\s*=\s*"envs/browser-qa-staging"')
        self.assertNotIn('prefix = "envs/prod"', backend)

    def test_root_accepts_only_pinned_non_secret_inputs(self):
        variables = (QA_ROOT / "variables.tf").read_text(encoding="utf-8")
        names = set(re.findall(r'variable\s+"([^"]+)"', variables))
        self.assertEqual(names, {"project_id", "region"})
        self.assertIn('var.project_id == "vocai-gemini-prod"', variables)
        self.assertIn('var.region == "us-west1"', variables)
        self.assertNotRegex(variables, r"(?i)secret|token|oauth|gmail|api[_-]?key")

    def test_tfvars_contains_only_expected_environment_values(self):
        tfvars = (QA_ROOT / "terraform.tfvars").read_text(encoding="utf-8")
        assignments = dict(re.findall(r'(?m)^\s*([a-z0-9_]+)\s*=\s*"([^"]+)"\s*$', tfvars))
        self.assertEqual(assignments, {"project_id": "vocai-gemini-prod", "region": "us-west1"})

    def test_shared_api_ownership_is_not_declared_in_qa_root(self):
        terraform = "\n".join(path.read_text(encoding="utf-8") for path in QA_ROOT.glob("*.tf"))
        self.assertNotIn('resource "google_project_service"', terraform)
        self.assertNotIn('module "apis"', terraform)


if __name__ == "__main__":
    unittest.main()
```

- [ ] **Step 2: Run the test and verify it fails because the root does not exist**

Run:

```bash
python -B -m unittest scripts.browser_qa.tests.test_terraform_state_isolation_contract -v
```

Expected: `ERROR` from reading `deploy/gcp/envs/browser-qa-staging/backend.tf` or `variables.tf`.

- [ ] **Step 3: Create the backend and provider files**

`deploy/gcp/envs/browser-qa-staging/backend.tf`:

```hcl
terraform {
  backend "gcs" {
    bucket = "vocai-gemini-prod-newapi-tfstate"
    prefix = "envs/browser-qa-staging"
  }
}
```

`deploy/gcp/envs/browser-qa-staging/versions.tf`:

```hcl
terraform {
  required_version = ">= 1.5.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.13"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}
```

- [ ] **Step 4: Create pinned non-secret inputs**

`deploy/gcp/envs/browser-qa-staging/variables.tf`:

```hcl
variable "project_id" {
  type        = string
  description = "GCP project that hosts the isolated staging Browser QA resources"

  validation {
    condition     = var.project_id == "vocai-gemini-prod"
    error_message = "Browser QA staging must target vocai-gemini-prod."
  }
}

variable "region" {
  type        = string
  description = "GCP region for isolated staging Browser QA resources"

  validation {
    condition     = var.region == "us-west1"
    error_message = "Browser QA staging must target us-west1."
  }
}
```

`deploy/gcp/envs/browser-qa-staging/terraform.tfvars`:

```hcl
project_id = "vocai-gemini-prod"
region     = "us-west1"
```

- [ ] **Step 5: Initialize without the remote backend and generate the provider lock**

Run:

```bash
terraform -chdir=deploy/gcp/envs/browser-qa-staging init -backend=false
terraform -chdir=deploy/gcp/envs/browser-qa-staging validate
```

Expected: `Terraform has been successfully initialized!` followed by `Success! The configuration is valid.` The generated `.terraform/` remains untracked; `.terraform.lock.hcl` is staged.

- [ ] **Step 6: Run the root-boundary test and verify it passes**

Run:

```bash
python -B -m unittest scripts.browser_qa.tests.test_terraform_state_isolation_contract -v
```

Expected: four tests pass.

- [ ] **Step 7: Commit the independent root shell**

```bash
git add scripts/browser_qa/tests/test_terraform_state_isolation_contract.py deploy/gcp/envs/browser-qa-staging
git commit -m "Give staging Browser QA its own Terraform state boundary" \
  -m "Constraint: The root accepts only the pinned non-secret project and region and does not own shared project APIs.
Rejected: Reusing envs/prod | Production drift would continue contaminating QA plans.
Confidence: high
Scope-risk: narrow
Directive: Keep backend prefix envs/browser-qa-staging distinct from envs/prod.
Tested: Terraform init -backend=false; terraform validate; state-isolation contract tests.
Not-tested: Remote backend initialization and live GCP plan remain human-operator steps."
```

### Task 2: Transfer only Browser QA resources and outputs out of the production root

**Files:**
- Move: `deploy/gcp/envs/prod/browser_qa.tf` → `deploy/gcp/envs/browser-qa-staging/browser_qa.tf`
- Create: `deploy/gcp/envs/browser-qa-staging/outputs.tf`
- Modify: `deploy/gcp/envs/prod/variables.tf`
- Modify: `deploy/gcp/envs/prod/terraform.tfvars`
- Modify: `deploy/gcp/envs/prod/outputs.tf`
- Modify: `scripts/browser_qa/tests/test_terraform_state_isolation_contract.py`
- Modify: `scripts/browser_qa/tests/test_terraform_contract.py`

- [ ] **Step 1: Extend tests to require exclusive ownership in the new root**

Add to `test_terraform_state_isolation_contract.py`:

```python
    def test_prod_root_relinquishes_browser_qa_ownership(self):
        self.assertFalse((PROD_ROOT / "browser_qa.tf").exists())
        prod_text = "\n".join(
            (PROD_ROOT / name).read_text(encoding="utf-8")
            for name in ("variables.tf", "terraform.tfvars", "outputs.tf")
        )
        self.assertNotIn("enable_browser_qa", prod_text)
        self.assertNotRegex(prod_text, r'output\s+"browser_qa_')

    def test_qa_resources_are_unconditional_and_do_not_depend_on_prod_modules(self):
        browser_qa = (QA_ROOT / "browser_qa.tf").read_text(encoding="utf-8")
        self.assertNotIn("enable_browser_qa", browser_qa)
        self.assertNotRegex(browser_qa, r"\bcount\s*=")
        self.assertNotIn("module.apis", browser_qa)
        self.assertNotRegex(browser_qa, r"browser_qa_[a-z0-9_]+\[0\]")
```

Retarget the constants in `test_terraform_contract.py`:

```python
QA_DIR = REPO_ROOT / "deploy" / "gcp" / "envs" / "browser-qa-staging"
BROWSER_QA_TF = QA_DIR / "browser_qa.tf"
VARIABLES_TF = QA_DIR / "variables.tf"
TFVARS = QA_DIR / "terraform.tfvars"
OUTPUTS_TF = QA_DIR / "outputs.tf"
```

Delete `_assert_count_gated` and `test_opt_in_variable_and_enabled_tfvars`. Change every expected Terraform reference from `resource.name[0].attribute` to `resource.name.attribute`, and change output assertions from conditional `var.enable_browser_qa ? ... : null` values to direct resource values.

- [ ] **Step 2: Run the two Terraform contract modules and verify the ownership tests fail**

```bash
python -B -m unittest \
  scripts.browser_qa.tests.test_terraform_state_isolation_contract \
  scripts.browser_qa.tests.test_terraform_contract -v
```

Expected: failures report the existing prod file/toggle and the missing new-root `browser_qa.tf`/`outputs.tf`.

- [ ] **Step 3: Move and ungate the Browser QA resources**

Move the file, preserving every resource body, then apply these exact transformations:

1. Delete every line `count = var.enable_browser_qa ? 1 : 0`.
2. Replace every QA resource reference `...[0].<attribute>` with `....<attribute>`.
3. Delete the four `depends_on = [module.apis]` clauses attached to Artifact Registry, the three Secret containers, and the WIF pool.
4. Preserve all `lifecycle.ignore_changes` image paths, IAM roles, names, service accounts, timeouts, bucket lifecycle, WIF conditions, staging origins, and placeholder image.

Verify the transformation mechanically:

```bash
rg -n 'enable_browser_qa|module\.apis|browser_qa_[a-z0-9_]+\[0\]' deploy/gcp/envs/browser-qa-staging/browser_qa.tf
```

Expected: no output.

- [ ] **Step 4: Create the stable unconditional outputs**

`deploy/gcp/envs/browser-qa-staging/outputs.tf`:

```hcl
output "browser_qa_artifact_registry_url" {
  description = "Set this as the GH Actions variable GCP_BROWSER_QA_AR_REPO_URL"
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.browser_qa.repository_id}"
}

output "browser_qa_wif_provider" {
  description = "Set this as the GH Actions variable GCP_BROWSER_QA_WIF_PROVIDER"
  value       = google_iam_workload_identity_pool_provider.browser_qa_github.name
}

output "browser_qa_deployer_sa_email" {
  description = "Set this as the GH Actions variable GCP_BROWSER_QA_DEPLOYER_SA"
  value       = google_service_account.browser_qa_deployer.email
}

output "browser_qa_report_bucket" {
  description = "Set this as the GH Actions variable GCP_BROWSER_QA_GCS_BUCKET"
  value       = google_storage_bucket.browser_qa_reports.name
}

output "browser_qa_broker_uri" {
  description = "Private broker service URI used by the browser QA runtime"
  value       = google_cloud_run_v2_service.browser_qa_broker.uri
}

output "browser_qa_broker_service_name" {
  description = "Cloud Run service name matching the GCP Browser QA workflow QA_BROKER_SERVICE env"
  value       = google_cloud_run_v2_service.browser_qa_broker.name
}

output "browser_qa_main_job_name" {
  description = "Cloud Run job name matching the GCP Browser QA workflow QA_MAIN_JOB env"
  value       = google_cloud_run_v2_job.browser_qa_main.name
}

output "browser_qa_cleanup_job_name" {
  description = "Cloud Run job name matching the GCP Browser QA workflow QA_CLEANUP_JOB env"
  value       = google_cloud_run_v2_job.browser_qa_cleanup.name
}
```

- [ ] **Step 5: Remove only the old production declarations**

- Delete the `enable_browser_qa` variable block from `deploy/gcp/envs/prod/variables.tf`.
- Delete only `enable_browser_qa = true` from `deploy/gcp/envs/prod/terraform.tfvars`.
- Delete only the eight output blocks whose names begin with `browser_qa_` from `deploy/gcp/envs/prod/outputs.tf`.
- Do not change any production module, Cloud SQL setting, staging service, Secret version, monitoring rule, or unrelated tfvar.

- [ ] **Step 6: Format, validate, and run the two contract modules**

```bash
terraform fmt -recursive deploy/gcp/envs/browser-qa-staging deploy/gcp/envs/prod
terraform -chdir=deploy/gcp/envs/browser-qa-staging init -backend=false
terraform -chdir=deploy/gcp/envs/browser-qa-staging validate
python -B -m unittest \
  scripts.browser_qa.tests.test_terraform_state_isolation_contract \
  scripts.browser_qa.tests.test_terraform_contract -v
```

Expected: Terraform validation succeeds and all targeted tests pass.

- [ ] **Step 7: Commit the ownership transfer**

```bash
git add deploy/gcp/envs/prod deploy/gcp/envs/browser-qa-staging scripts/browser_qa/tests/test_terraform_contract.py scripts/browser_qa/tests/test_terraform_state_isolation_contract.py
git commit -m "Prevent Browser QA plans from inheriting production drift" \
  -m "Constraint: No live Browser QA resources or old-state addresses exist, so the ownership transfer requires neither destroy nor state migration.
Rejected: Leaving disabled duplicate declarations in prod | Future toggles could recreate ownership collisions.
Confidence: high
Scope-risk: moderate
Directive: Stop and design an import if any same-name live resource or browser_qa state address appears before bootstrap.
Tested: Terraform fmt/validate and Browser QA Terraform ownership/security contract tests.
Not-tested: Remote-state refresh and live plan remain human-operator steps."
```

### Task 3: Add an exact create-only saved-plan guard

**Files:**
- Create: `scripts/browser_qa/terraform_plan_guard.py`
- Create: `scripts/browser_qa/tests/test_terraform_plan_guard.py`

- [ ] **Step 1: Write adversarial unit tests before the guard**

The test module imports `ALLOWED_RESOURCE_ADDRESSES`, `ALLOWED_OUTPUTS`, `PlanValidationError`, and `validate_bootstrap_plan`. It constructs a passing plan from every allowed address/output, then verifies these mutations fail:

```python
import unittest

from scripts.browser_qa.terraform_plan_guard import (
    ALLOWED_OUTPUTS,
    ALLOWED_RESOURCE_ADDRESSES,
    PlanValidationError,
    validate_bootstrap_plan,
)


def valid_plan():
    return {
        "resource_changes": [
            {"address": address, "change": {"actions": ["create"]}}
            for address in sorted(ALLOWED_RESOURCE_ADDRESSES)
        ],
        "output_changes": {
            name: {"actions": ["create"]}
            for name in sorted(ALLOWED_OUTPUTS)
        },
    }


class TerraformPlanGuardTest(unittest.TestCase):
    def test_accepts_complete_exact_create_only_plan(self):
        validate_bootstrap_plan(valid_plan())

    def test_rejects_non_qa_address(self):
        plan = valid_plan()
        plan["resource_changes"].append(
            {"address": "module.cloud_sql.google_sql_database_instance.main", "change": {"actions": ["create"]}}
        )
        with self.assertRaisesRegex(PlanValidationError, "unexpected resource"):
            validate_bootstrap_plan(plan)

    def test_rejects_update_delete_and_replace(self):
        for actions in (["update"], ["delete"], ["create", "delete"], ["delete", "create"]):
            with self.subTest(actions=actions):
                plan = valid_plan()
                plan["resource_changes"][0]["change"]["actions"] = actions
                with self.assertRaisesRegex(PlanValidationError, "create-only"):
                    validate_bootstrap_plan(plan)

    def test_rejects_missing_expected_resource(self):
        plan = valid_plan()
        plan["resource_changes"].pop()
        with self.assertRaisesRegex(PlanValidationError, "missing resource"):
            validate_bootstrap_plan(plan)

    def test_rejects_unrelated_output(self):
        plan = valid_plan()
        plan["output_changes"]["cloudsql_connection_name"] = {"actions": ["create"]}
        with self.assertRaisesRegex(PlanValidationError, "unexpected output"):
            validate_bootstrap_plan(plan)

    def test_rejects_output_update(self):
        plan = valid_plan()
        output_name = next(iter(plan["output_changes"]))
        plan["output_changes"][output_name]["actions"] = ["update"]
        with self.assertRaisesRegex(PlanValidationError, "output changes are not create-only"):
            validate_bootstrap_plan(plan)

    def test_rejects_empty_plan(self):
        with self.assertRaisesRegex(PlanValidationError, "no resource changes"):
            validate_bootstrap_plan({"resource_changes": [], "output_changes": {}})
```

- [ ] **Step 2: Run the guard test and verify import failure**

```bash
python -B -m unittest scripts.browser_qa.tests.test_terraform_plan_guard -v
```

Expected: `ModuleNotFoundError` for `scripts.browser_qa.terraform_plan_guard`.

- [ ] **Step 3: Implement the guard with exact constants and deterministic errors**

`scripts/browser_qa/terraform_plan_guard.py` uses these complete constants and no substring/regex allow rule:

```python
import argparse
import json
import sys
from pathlib import Path


ALLOWED_RESOURCE_ADDRESSES = frozenset(
    {
        "google_artifact_registry_repository.browser_qa",
        "google_artifact_registry_repository_iam_member.browser_qa_deployer_writer",
        "google_cloud_run_v2_job.browser_qa_cleanup",
        "google_cloud_run_v2_job.browser_qa_main",
        "google_cloud_run_v2_job_iam_member.browser_qa_cleanup_deployer_developer",
        "google_cloud_run_v2_job_iam_member.browser_qa_cleanup_deployer_invoker",
        "google_cloud_run_v2_job_iam_member.browser_qa_main_deployer_developer",
        "google_cloud_run_v2_job_iam_member.browser_qa_main_deployer_invoker",
        "google_cloud_run_v2_service.browser_qa_broker",
        "google_cloud_run_v2_service_iam_member.browser_qa_broker_deployer_developer",
        "google_cloud_run_v2_service_iam_member.browser_qa_broker_invoker",
        "google_iam_workload_identity_pool.browser_qa_github",
        "google_iam_workload_identity_pool_provider.browser_qa_github",
        "google_project_iam_member.browser_qa_broker_log_writer",
        "google_project_iam_member.browser_qa_cleanup_log_writer",
        "google_project_iam_member.browser_qa_runtime_log_writer",
        "google_secret_manager_secret.browser_qa_codex_api_key",
        "google_secret_manager_secret.browser_qa_gmail_oauth",
        "google_secret_manager_secret.browser_qa_identity_seed",
        "google_secret_manager_secret_iam_member.browser_qa_broker_gmail_oauth",
        "google_secret_manager_secret_iam_member.browser_qa_cleanup_identity_seed",
        "google_secret_manager_secret_iam_member.browser_qa_runtime_codex_api_key",
        "google_secret_manager_secret_iam_member.browser_qa_runtime_identity_seed",
        "google_service_account.browser_qa_broker",
        "google_service_account.browser_qa_cleanup",
        "google_service_account.browser_qa_deployer",
        "google_service_account.browser_qa_runtime",
        "google_service_account_iam_member.browser_qa_broker_user",
        "google_service_account_iam_member.browser_qa_cleanup_user",
        "google_service_account_iam_member.browser_qa_runtime_user",
        "google_service_account_iam_member.browser_qa_wif_deployer",
        "google_storage_bucket.browser_qa_reports",
        "google_storage_bucket_iam_member.browser_qa_cleanup_report_admin",
        "google_storage_bucket_iam_member.browser_qa_deployer_report_viewer",
        "google_storage_bucket_iam_member.browser_qa_runtime_report_creator",
    }
)

ALLOWED_OUTPUTS = frozenset(
    {
        "browser_qa_artifact_registry_url",
        "browser_qa_broker_service_name",
        "browser_qa_broker_uri",
        "browser_qa_cleanup_job_name",
        "browser_qa_deployer_sa_email",
        "browser_qa_main_job_name",
        "browser_qa_report_bucket",
        "browser_qa_wif_provider",
    }
)


class PlanValidationError(ValueError):
    pass


def _meaningful_resource_changes(plan):
    return {
        change.get("address", ""): tuple(change.get("change", {}).get("actions", ()))
        for change in plan.get("resource_changes", [])
        if change.get("change", {}).get("actions") not in (None, [], ["no-op"])
    }


def validate_bootstrap_plan(plan):
    changes = _meaningful_resource_changes(plan)
    if not changes:
        raise PlanValidationError("saved plan has no resource changes")

    unexpected = sorted(set(changes) - ALLOWED_RESOURCE_ADDRESSES)
    if unexpected:
        raise PlanValidationError(f"unexpected resource addresses: {', '.join(unexpected)}")

    missing = sorted(ALLOWED_RESOURCE_ADDRESSES - set(changes))
    if missing:
        raise PlanValidationError(f"missing resource addresses: {', '.join(missing)}")

    non_create = sorted(address for address, actions in changes.items() if actions != ("create",))
    if non_create:
        raise PlanValidationError(f"bootstrap plan is not create-only: {', '.join(non_create)}")

    changed_outputs = {
        name: tuple(change.get("actions", ()))
        for name, change in plan.get("output_changes", {}).items()
        if change.get("actions") not in (None, [], ["no-op"])
    }
    unexpected_outputs = sorted(set(changed_outputs) - ALLOWED_OUTPUTS)
    if unexpected_outputs:
        raise PlanValidationError(f"unexpected output changes: {', '.join(unexpected_outputs)}")
    missing_outputs = sorted(ALLOWED_OUTPUTS - set(changed_outputs))
    if missing_outputs:
        raise PlanValidationError(f"missing output changes: {', '.join(missing_outputs)}")
    non_create_outputs = sorted(
        name for name, actions in changed_outputs.items() if actions != ("create",)
    )
    if non_create_outputs:
        raise PlanValidationError(
            f"output changes are not create-only: {', '.join(non_create_outputs)}"
        )

    return changes


def main(argv=None):
    parser = argparse.ArgumentParser(
        description="Validate the exact first Browser QA Terraform bootstrap plan."
    )
    parser.add_argument("plan_json", type=Path, help="Path produced by terraform show -json")
    args = parser.parse_args(argv)

    try:
        with args.plan_json.open("r", encoding="utf-8") as handle:
            plan = json.load(handle)
        changes = validate_bootstrap_plan(plan)
    except (OSError, json.JSONDecodeError, PlanValidationError) as exc:
        print(f"ABORT: {exc}", file=sys.stderr)
        return 1

    print("OK: exact Browser QA create-only bootstrap plan")
    for address in sorted(changes):
        print(f"  {address}: create")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
```

- [ ] **Step 4: Run unit tests and a CLI smoke test**

```bash
python -B -m unittest scripts.browser_qa.tests.test_terraform_plan_guard -v
python -B scripts/browser_qa/terraform_plan_guard.py --help
```

Expected: seven tests pass and help shows the required plan JSON positional argument.

- [ ] **Step 5: Commit the guard**

```bash
git add scripts/browser_qa/terraform_plan_guard.py scripts/browser_qa/tests/test_terraform_plan_guard.py
git commit -m "Reject any Browser QA bootstrap plan that can alter existing resources" \
  -m "Constraint: First bootstrap must contain exactly the dedicated QA resources as create-only actions and only QA outputs.
Rejected: Address substring checks | A future unrelated resource could be named to bypass them.
Confidence: high
Scope-risk: narrow
Directive: Keep the exact whitelist synchronized with reviewed browser_qa.tf changes and require a new design for delete or replace actions.
Tested: Positive, non-QA, missing-resource, update, delete, replace, output, empty-plan, and CLI tests.
Not-tested: Real terraform show JSON is validated during the human bootstrap."
```

### Task 4: Migrate the human runbook and lock it with operations tests

**Files:**
- Modify: `scripts/browser_qa/tests/test_operations_contract.py`
- Modify: `deploy/gcp/docs/OPERATIONS.md`

- [ ] **Step 1: Write failing runbook-path and guard tests**

Change the existing anchor expectation to:

```python
self.assertRegex(block, r'cd "\$repo_root/deploy/gcp/envs/browser-qa-staging"')
```

Add tests that isolate the `## Flatkey staging browser QA first-run and recovery runbook` section and assert:

```python
def browser_qa_section():
    match = re.search(
        r"(?ms)^## Flatkey staging browser QA first-run and recovery runbook\n"
        r"(?P<body>.*?)(?=^---\n\n## |\Z)",
        operations_text(),
    )
    if not match:
        raise AssertionError("Browser QA runbook section not found")
    return match.group("body")


    def test_browser_qa_runbook_uses_only_the_independent_root(self):
        section = browser_qa_section()
        self.assertNotIn("deploy/gcp/envs/prod", section)
        self.assertIn("deploy/gcp/envs/browser-qa-staging", section)

    def test_bootstrap_uses_versioned_guard_and_fail_closed_preflight(self):
        section = browser_qa_section()
        self.assertIn(
            'python3 "$repo_root/scripts/browser_qa/terraform_plan_guard.py" "$plan_json"',
            section,
        )
        self.assertNotIn('if "browser_qa" not in address', section)
        self.assertIn("gcloud services list --enabled", section)
        self.assertIn("terraform state list", section)
        self.assertIn("ABORT: independent Browser QA state is not empty", section)
```

Place the two `test_*` methods on `BrowserQaOperationsContractTests`; keep `browser_qa_section` as a module helper.

- [ ] **Step 2: Run operations tests and verify old-root failures**

```bash
python -B -m unittest scripts.browser_qa.tests.test_operations_contract -v
```

Expected: failures identify `deploy/gcp/envs/prod`, the inline substring guard, and missing API/state preflight.

- [ ] **Step 3: Update the Browser QA resource table and every Terraform command path**

- Change the managed root to `deploy/gcp/envs/browser-qa-staging/` and backend prefix to `envs/browser-qa-staging`.
- Explain that `deploy/gcp/envs/prod/` still owns shared project APIs and production/staging application infrastructure.
- Change all Browser QA `cd` and `terraform -chdir` commands, including output reads, broker validation, and GCS lookup, to the new root.
- Remove `enable_browser_qa = true` from the resource table and instructions.
- Leave Gmail, Secret stdin, IAM negative probe, dispatch, cleanup-only, and report triage semantics unchanged.

- [ ] **Step 4: Replace the inline guard with API/state/live-resource preflight plus the versioned guard**

The saved-plan block must:

1. Anchor `repo_root` and `qa_root`.
2. Verify active project `vocai-gemini-prod` and region `us-west1`.
3. Compare these enabled services using exact lines from `gcloud services list --enabled`: `artifactregistry.googleapis.com`, `cloudresourcemanager.googleapis.com`, `iam.googleapis.com`, `iamcredentials.googleapis.com`, `logging.googleapis.com`, `run.googleapis.com`, `secretmanager.googleapis.com`, `serviceusage.googleapis.com`, `sts.googleapis.com`, and `storage.googleapis.com`.
4. Run `terraform init -reconfigure` only in the new root.
5. Abort if `terraform state list` emits any address.
6. Abort if describe probes find the QA Artifact Registry repository, broker service, either job, any of four service accounts, any of three Secret containers, the report bucket, or WIF pool/provider already live.
7. Generate one refreshing saved plan and JSON/text render.
8. Call `python3 "$repo_root/scripts/browser_qa/terraform_plan_guard.py" "$plan_json"`.
9. Preserve human-readable review and the exact saved-plan confirmation phrase.

The text must say that any state/live-resource hit requires an import/migration design; operators must not continue with create or `-target`.

- [ ] **Step 5: Run operations and Terraform contract tests**

```bash
python -B -m unittest \
  scripts.browser_qa.tests.test_operations_contract \
  scripts.browser_qa.tests.test_terraform_state_isolation_contract \
  scripts.browser_qa.tests.test_terraform_contract \
  scripts.browser_qa.tests.test_terraform_plan_guard -v
```

Expected: all targeted tests pass.

- [ ] **Step 6: Commit the runbook migration**

```bash
git add deploy/gcp/docs/OPERATIONS.md scripts/browser_qa/tests/test_operations_contract.py
git commit -m "Keep Browser QA operators out of the production Terraform root" \
  -m "Constraint: Applies, Secret writes, GitHub Variable writes, and dispatches remain human-only authenticated operations.
Rejected: Duplicated inline plan logic | A versioned tested guard is safer and reusable.
Confidence: high
Scope-risk: narrow
Directive: Abort on non-empty state, same-name live resources, missing APIs, or any guard failure.
Tested: Operations, state-isolation, Terraform resource, and plan-guard contract tests.
Not-tested: Mutating operator commands and live workflow dispatch are intentionally not run by agents."
```

### Task 5: Run full local verification and produce the operator readiness boundary

**Files:**
- Modify only if verification exposes a defect in the files owned by Tasks 1–4.

- [ ] **Step 1: Verify formatting and both Terraform roots' static integrity**

```bash
terraform fmt -check -recursive deploy/gcp/envs/browser-qa-staging deploy/gcp/envs/prod
terraform -chdir=deploy/gcp/envs/browser-qa-staging init -backend=false
terraform -chdir=deploy/gcp/envs/browser-qa-staging validate
git diff --check
```

Expected: every command exits 0. Do not run a production plan.

- [ ] **Step 2: Run the full Browser QA test suite**

```bash
python -B -m unittest discover -s scripts/browser_qa/tests -p "test_*.py" -v
```

Expected: all tests pass; environment-specific skips remain explicitly reported as skips.

- [ ] **Step 3: Verify secrets and ownership cannot leak back into Terraform**

```bash
if rg -n 'secret_data|refresh_token|client_secret|@gmail\.com|GCP_BROWSER_QA_GMAIL_BASE' deploy/gcp/envs/browser-qa-staging; then exit 1; fi
if rg -n 'browser_qa|enable_browser_qa' deploy/gcp/envs/prod; then exit 1; fi
if rg -n 'google_project_service|module\.apis|google_secret_manager_secret_version' deploy/gcp/envs/browser-qa-staging; then exit 1; fi
```

Expected: all three commands return no matches. Secret resource names such as `browser_qa_gmail_oauth` are allowed in `browser_qa.tf`; Secret values and Gmail addresses are not.

- [ ] **Step 4: Review the exact implementation diff**

```bash
git diff --stat 2879c78e8..HEAD
git diff --name-status 2879c78e8..HEAD
git log --oneline 2879c78e8..HEAD
```

Expected: only files listed in the file responsibility map changed; no application runtime, main/production workflow, Cloud SQL module, existing Cloud Run module, monitoring module, or Secret value file changed.

- [ ] **Step 5: Run completion review**

Request a code-review agent to verify state ownership, exact whitelist synchronization, Terraform security, runbook correctness, and test adequacy. Approval is required. For each blocking finding, add a regression assertion to the owning test module, reproduce the failure, patch only the files in the responsibility map, rerun Steps 1–4, and make a Lore-format correction commit.

- [ ] **Step 6: Perform a fresh read-only readiness audit**

Read-only checks may confirm that the 35 QA resources and five GitHub Variables are still absent. Do not read Secret values. Report the next human action as the exact new-root saved-plan bootstrap; do not push `staging` while readiness is incomplete.

### Task 6: Human bootstrap and live staging acceptance

**Files:**
- No repository file changes unless live evidence exposes a defect.

- [ ] **Step 1: Human operator applies only the guarded independent plan**

The operator follows the committed `OPERATIONS.md`, uses Owner-capable ADC, verifies empty new state and absent live QA resources, reviews the human-readable plan, and applies the exact saved create-only plan. Expected: only the 35 Browser QA resources are created; zero update/delete/replace actions occur.

- [ ] **Step 2: Human operator writes runtime material without exposing it**

Using the committed stdin-only commands, add one version to each of:

- `flatkey-browser-qa-codex-api-key`
- `flatkey-browser-qa-identity-seed`
- `flatkey-browser-qa-gmail-oauth`

Then set the four output-backed GitHub Variables and `GCP_BROWSER_QA_GMAIL_BASE`. Do not paste values into chat or argv.

- [ ] **Step 3: Re-run readiness audit**

Expected: all QA resources exist, each Secret container has an enabled version, all five GitHub Variables exist, broker/IAM negative probes pass, and no staging deployment is in progress.

- [ ] **Step 4: Fast-forward the expected commit chain to `staging`**

First verify `git ls-remote --heads origin staging` still matches the expected ancestor. If it moved, fetch and integrate without overwriting others. Push the reviewed commit chain only after readiness is green.

- [ ] **Step 5: Monitor the automatic staging `core` execution**

Expected job order: backend `build` → `deploy` and health check → `browser-qa-core` reusable workflow → main execution → cleanup execution. QA failure marks Actions red but does not roll back staging.

- [ ] **Step 6: Verify the live acceptance evidence**

Confirm the recorded onboarding replay completes or produces a localized failure, GCS artifacts are private and redacted, the test account and all of its API Keys are removed, and cleanup status is verified. If cleanup fails, run manual `cleanup-only` with the original staging workflow run id.

- [ ] **Step 7: Stop before main/production work**

Record the staging result. Do not connect Browser QA to main or production release flows in this plan.
