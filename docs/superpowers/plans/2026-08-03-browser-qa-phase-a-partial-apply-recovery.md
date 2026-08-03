# Browser QA Phase A Partial-Apply Recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a fail-closed recovery path that accepts only the three remaining Browser QA report-bucket IAM creates after the interrupted Phase A apply, then produce a freshly reviewed saved plan for manual application.

**Architecture:** Keep the normal `infra` and `workloads` contracts unchanged. Add a third exact `infra-recovery` contract to the existing Python plan guard, lock it with unit tests, and add an incident-specific runbook section whose pre-state, plan, confirmation, apply, and post-state gates are independently tested.

**Tech Stack:** Python 3 `unittest`, Terraform JSON plan inspection, Bash runbook contracts, Google Cloud CLI, PowerShell operator handoff.

---

### Task 1: Lock the exact recovery plan contract with failing tests

**Files:**
- Modify: `scripts/browser_qa/tests/test_terraform_plan_guard.py`
- Test: `scripts/browser_qa/tests/test_terraform_plan_guard.py`

- [ ] **Step 1: Add the expected recovery resource set and import**

Extend the guard import with `INFRA_RECOVERY_RESOURCE_ADDRESSES`, then add this independent expected set:

```python
EXPECTED_INFRA_RECOVERY_RESOURCE_ADDRESSES = frozenset(
    {
        "google_storage_bucket_iam_member.browser_qa_cleanup_report_admin",
        "google_storage_bucket_iam_member.browser_qa_deployer_report_viewer",
        "google_storage_bucket_iam_member.browser_qa_runtime_report_creator",
    }
)
```

Add it to the test phase maps with no meaningful outputs:

```python
PHASE_RESOURCES = {
    "infra": EXPECTED_INFRA_RESOURCE_ADDRESSES,
    "infra-recovery": EXPECTED_INFRA_RECOVERY_RESOURCE_ADDRESSES,
    "workloads": EXPECTED_WORKLOAD_RESOURCE_ADDRESSES,
}
PHASE_OUTPUTS = {
    "infra": EXPECTED_INFRA_OUTPUTS,
    "infra-recovery": frozenset(),
    "workloads": EXPECTED_WORKLOAD_OUTPUTS,
}
```

- [ ] **Step 2: Model the real recovery plan shape**

Add a helper that contains three creates, the other 23 Phase A addresses as no-ops, and the four Phase A outputs as no-ops:

```python
def infra_recovery_plan():
    plan = phase_plan("infra-recovery")
    plan["resource_changes"].extend(
        {"address": address, "change": {"actions": ["no-op"]}}
        for address in sorted(
            EXPECTED_INFRA_RESOURCE_ADDRESSES - EXPECTED_INFRA_RECOVERY_RESOURCE_ADDRESSES
        )
    )
    plan["output_changes"].update(
        {output: {"actions": ["no-op"]}} for output in sorted(EXPECTED_INFRA_OUTPUTS)
    )
    return plan
```

- [ ] **Step 3: Add positive and negative recovery assertions**

Add tests that require the exact three creates and reject an omitted create, an additional Phase A create, and any meaningful output:

```python
def test_infra_recovery_accepts_exact_three_creates_and_known_infra_no_ops(self):
    changes = validate_bootstrap_plan(infra_recovery_plan(), phase="infra-recovery")
    self.assertEqual(set(changes), EXPECTED_INFRA_RECOVERY_RESOURCE_ADDRESSES)

def test_infra_recovery_rejects_missing_or_extra_create(self):
    missing = infra_recovery_plan()
    missing["resource_changes"] = [
        change
        for change in missing["resource_changes"]
        if change["address"] != sorted(EXPECTED_INFRA_RECOVERY_RESOURCE_ADDRESSES)[0]
    ]
    assert_rejected(self, missing, "infra-recovery", "missing resource for infra-recovery phase")

    extra = infra_recovery_plan()
    extra_address = sorted(
        EXPECTED_INFRA_RESOURCE_ADDRESSES - EXPECTED_INFRA_RECOVERY_RESOURCE_ADDRESSES
    )[0]
    for change in extra["resource_changes"]:
        if change["address"] == extra_address:
            change["change"]["actions"] = ["create"]
            break
    assert_rejected(self, extra, "infra-recovery", "unexpected resource for infra-recovery phase")

def test_infra_recovery_rejects_any_meaningful_output(self):
    plan = infra_recovery_plan()
    plan["output_changes"][sorted(EXPECTED_INFRA_OUTPUTS)[0]]["actions"] = ["update"]
    assert_rejected(self, plan, "infra-recovery", "unexpected output for infra-recovery phase")
```

Update generic phase tests so empty expected-output sets do not call `next(iter(...))`, and add the CLI success message:

```python
"infra-recovery": "OK: exact Browser QA infra-recovery create-only bootstrap plan",
```

- [ ] **Step 4: Run the guard tests and verify RED**

Run:

```powershell
python -B -m unittest scripts.browser_qa.tests.test_terraform_plan_guard -v
```

Expected: FAIL because `INFRA_RECOVERY_RESOURCE_ADDRESSES` and the `infra-recovery` phase do not yet exist in production code.

### Task 2: Implement the minimal recovery phase in the guard

**Files:**
- Modify: `scripts/browser_qa/terraform_plan_guard.py`
- Test: `scripts/browser_qa/tests/test_terraform_plan_guard.py`

- [ ] **Step 1: Add the production recovery allowlist**

Add this constant after `INFRA_RESOURCE_ADDRESSES`:

```python
INFRA_RECOVERY_RESOURCE_ADDRESSES = frozenset(
    {
        "google_storage_bucket_iam_member.browser_qa_cleanup_report_admin",
        "google_storage_bucket_iam_member.browser_qa_deployer_report_viewer",
        "google_storage_bucket_iam_member.browser_qa_runtime_report_creator",
    }
)
```

- [ ] **Step 2: Register the exact phase without changing normal contracts**

Change only `_PHASE_CONTRACTS`:

```python
_PHASE_CONTRACTS = {
    "infra": (INFRA_RESOURCE_ADDRESSES, INFRA_OUTPUTS),
    "infra-recovery": (INFRA_RECOVERY_RESOURCE_ADDRESSES, frozenset()),
    "workloads": (WORKLOAD_RESOURCE_ADDRESSES, WORKLOAD_OUTPUTS),
}
```

Do not change `ALLOWED_RESOURCE_ADDRESSES`, `ALLOWED_OUTPUTS`, or `validate_bootstrap_plan`; the existing exact-set and create-only logic already enforces the recovery contract.

- [ ] **Step 3: Run the targeted tests and verify GREEN**

Run:

```powershell
python -B -m unittest scripts.browser_qa.tests.test_terraform_plan_guard -v
```

Expected: all guard tests pass, including the existing 26-resource Phase A and 9-resource Phase B cases.

- [ ] **Step 4: Commit the guard contract**

Stage only the guard and its test, then commit using the repository Lore protocol. The commit must record that arbitrary Phase A subsets remain rejected and that the recovery set is exactly three bucket IAM members.

### Task 3: Lock the operator recovery flow with failing contract tests

**Files:**
- Modify: `scripts/browser_qa/tests/test_operations_contract.py`
- Test: `scripts/browser_qa/tests/test_operations_contract.py`

- [ ] **Step 1: Add a recovery-section extractor**

First make `heading_section()` stop at any following level-three heading so the new `1A` section does not become part of the normal Phase A block:

```python
def heading_section(text, heading):
    match = re.search(
        rf"(?ms)^### {re.escape(heading)}\n(?P<body>.*?)(?=^### [^\n]+\n|\Z)",
        text,
    )
    if not match:
        raise AssertionError(f"section not found: {heading}")
    return match.group("body")
```

Then add the exact extractor:

```python
def phase_a_recovery_section():
    return heading_section(
        browser_qa_section(),
        "1A. Recover the interrupted Phase A bucket IAM apply",
    )


def phase_a_recovery_command_block():
    blocks = fenced_blocks(phase_a_recovery_section())
    if len(blocks) != 1:
        raise AssertionError(f"expected one Phase A recovery command block, found {len(blocks)}")
    return blocks[0]
```

- [ ] **Step 2: Assert the recovery safety gates**

Add contract tests using `phase_a_recovery_command_block()` and this exact recovery set:

```python
RECOVERY_ADDRESSES = {
    "google_storage_bucket_iam_member.browser_qa_cleanup_report_admin",
    "google_storage_bucket_iam_member.browser_qa_deployer_report_viewer",
    "google_storage_bucket_iam_member.browser_qa_runtime_report_creator",
}
```

The tests must prove:

- the old saved plan is called invalid before any new plan is generated;
- active account, project, and region are checked before state or plan operations;
- the expected pre-recovery state contains exactly 23 Phase A addresses and excludes the three recovery addresses;
- `terraform state list` is sorted and compared fail-closed against that expected set;
- the new plan uses `create_workloads=false` and a new `browser-qa-infra-recovery.tfplan` path;
- JSON and text are exported from that same plan;
- `terraform_plan_guard.py --phase infra-recovery` runs before confirmation;
- the human summary must contain `Plan: 3 to add, 0 to change, 0 to destroy.`;
- the token is exactly `APPLY_BROWSER_QA_INFRA_RECOVERY_SAVED_PLAN`;
- `terraform apply` consumes the same recovery plan path;
- a nonzero apply invalidates the plan and stops;
- post-apply state is compared with the exact 26-resource Phase A set;
- the section contains no `-target`, `-refresh=false`, `terraform import`, or `terraform state rm` command.

- [ ] **Step 3: Run the operations tests and verify RED**

Run:

```powershell
python -B -m unittest scripts.browser_qa.tests.test_operations_contract -v
```

Expected: FAIL because the recovery section is not yet present in `OPERATIONS.md`.

### Task 4: Add the incident-specific runbook recovery section

**Files:**
- Modify: `deploy/gcp/docs/OPERATIONS.md`
- Test: `scripts/browser_qa/tests/test_operations_contract.py`

- [ ] **Step 1: Insert the recovery section between Phase A and output-backed variables**

Add `### 1A. Recover the interrupted Phase A bucket IAM apply` after the normal Phase A block. The Bash block must:

```bash
set -euo pipefail
set +x

expected_project="vocai-gemini-prod"
expected_region="us-west1"
repo_root="$(git rev-parse --show-toplevel)"
qa_root="$repo_root/deploy/gcp/envs/browser-qa-staging"
review_dir="$(mktemp -d)"
trap 'rm -rf "$review_dir"' EXIT
```

It must write literal sorted 23-resource and 26-resource expected-state files, validate active configuration, run a refreshing plan, export JSON/text, invoke `--phase infra-recovery`, require the exact summary and confirmation token, apply only the saved plan, and compare post-state with the 26-resource expected file.

- [ ] **Step 2: Document the temporary broad grant cleanup**

After the successful recovery block, state that a project IAM administrator should remove this temporary binding:

```text
user:liu1124789567@gmail.com -> roles/storage.admin on vocai-gemini-prod
```

Do not put automatic project-IAM removal inside the Terraform recovery block because `roles/storage.admin` alone does not grant `resourcemanager.projects.setIamPolicy`.

- [ ] **Step 3: Run the targeted tests and verify GREEN**

Run:

```powershell
python -B -m unittest scripts.browser_qa.tests.test_operations_contract -v
python -B -m unittest scripts.browser_qa.tests.test_terraform_plan_guard -v
```

Expected: both suites pass.

- [ ] **Step 4: Commit the runbook contract**

Stage only `OPERATIONS.md` and `test_operations_contract.py`, then commit using the Lore protocol. Record that cloud mutation still requires the human terminal and that the temporary project-wide role is not silently retained.

### Task 5: Verify implementation and produce a fresh reviewed recovery plan

**Files:**
- Verify only: `scripts/browser_qa/terraform_plan_guard.py`
- Verify only: `scripts/browser_qa/tests/test_terraform_plan_guard.py`
- Verify only: `scripts/browser_qa/tests/test_operations_contract.py`
- Verify only: `deploy/gcp/docs/OPERATIONS.md`

- [ ] **Step 1: Run repository-local verification**

Run:

```powershell
python -B -m unittest discover -s scripts/browser_qa/tests -v
terraform -chdir="deploy/gcp/envs/browser-qa-staging" fmt -check
terraform -chdir="deploy/gcp/envs/browser-qa-staging" validate
git diff --check HEAD
```

Expected: all Browser QA tests pass, Terraform format/validation pass, and no whitespace errors are reported.

- [ ] **Step 2: Generate a new refreshing saved plan in a new review directory**

Run from a manual PowerShell terminal with `create_workloads=false`. Export JSON and text from the same plan and compute its SHA-256. Do not reuse the diagnostic preview plan or the failed Phase A plan.

Expected plan contract:

```text
google_storage_bucket_iam_member.browser_qa_cleanup_report_admin: create
google_storage_bucket_iam_member.browser_qa_deployer_report_viewer: create
google_storage_bucket_iam_member.browser_qa_runtime_report_creator: create
Plan: 3 to add, 0 to change, 0 to destroy.
```

- [ ] **Step 3: Run the new guard and perform human review**

Run:

```powershell
python -B "$repo/scripts/browser_qa/terraform_plan_guard.py" `
  --phase infra-recovery `
  "$planJson"
```

Expected: `OK: exact Browser QA infra-recovery create-only bootstrap plan`, followed by the same three addresses.

- [ ] **Step 4: Apply only from the human terminal**

The operator verifies the path and SHA-256, enters `APPLY_BROWSER_QA_INFRA_RECOVERY_SAVED_PLAN`, and applies the same saved plan. The agent must not execute `terraform apply`.

- [ ] **Step 5: Verify recovery completion**

Expected evidence:

- Terraform apply succeeds;
- state contains exactly 26 Phase A addresses;
- a new refreshing `create_workloads=false` plan has no changes;
- the three service-account bucket IAM bindings are visible without reading any object or secret payload;
- the temporary project-level `roles/storage.admin` grant is removed by a project IAM administrator;
- the workflow can continue with output-backed GitHub Variables and Secret Manager versions.
