# Browser QA Two-Phase Secret Bootstrap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make first-time Browser QA provisioning safe when Terraform owns empty Secret containers but an operator owns their first versions.

**Architecture:** Keep the isolated `browser-qa-staging` root/state. Gate exactly three Cloud Run resources and six Run IAM members behind a steady-state-true boolean, validate Phase A and Phase B with separate exact create-only contracts, and place human Secret-version creation between the two saved-plan applies.

**Tech Stack:** Terraform 1.x, hashicorp/google 6.50.x, Python 3 unittest, PowerShell/bash runbook snippets.

---

### Task 1: Lock the phase-aware saved-plan contract with failing tests

**Files:**
- Modify: `scripts/browser_qa/tests/test_terraform_plan_guard.py`

- [ ] **Step 1: Define exact test fixtures**

Define `EXPECTED_INFRA_RESOURCE_ADDRESSES` with the 26 unindexed infra addresses, `EXPECTED_WORKLOAD_RESOURCE_ADDRESSES` with the nine counted `[0]` addresses, and corresponding four-item infra/workload output sets.

- [ ] **Step 2: Write failing phase tests**

Add tests that call:

```python
validate_bootstrap_plan(infra_plan(), phase="infra")
validate_bootstrap_plan(workload_plan(), phase="workloads")
```

Add a regression test that passes the old combined 35-create plan to both phases and requires `PlanValidationError`.

- [ ] **Step 3: Write failing CLI tests**

Require `--phase {infra,workloads}`, exit `2` when missing/invalid, exit `1` with `ABORT:` for a rejected plan, and phase-specific success messages.

- [ ] **Step 4: Verify RED**

Run:

```powershell
python -B -m unittest scripts.browser_qa.tests.test_terraform_plan_guard -v
```

Expected: failures because phase constants, the `phase` parameter, and required CLI flag do not exist.

### Task 2: Lock the Terraform workload gate with failing tests

**Files:**
- Modify: `scripts/browser_qa/tests/test_terraform_contract.py`

- [ ] **Step 1: Test the steady-state variable**

Assert `create_workloads` is boolean, defaults to `true`, and is explicitly `true` in `terraform.tfvars`.

- [ ] **Step 2: Test exact gated resources**

Assert only the three Cloud Run blocks and six Run IAM member blocks contain:

```hcl
count = var.create_workloads ? 1 : 0
```

Assert the other 26 resource blocks do not reference `create_workloads`.

- [ ] **Step 3: Test counted references and outputs**

Require main-job broker references and Run IAM resource names to use counted instances. Require the four workload outputs to use `one(resource[*].field)` while the four infra outputs remain unconditional.

- [ ] **Step 4: Verify RED**

Run:

```powershell
python -B -m unittest scripts.browser_qa.tests.test_terraform_contract -v
```

Expected: failures because the variable, count gates, counted references, and nullable outputs are absent.

### Task 3: Lock the runbook sequence with failing tests

**Files:**
- Modify: `scripts/browser_qa/tests/test_operations_contract.py`

- [ ] **Step 1: Add Phase A assertions**

Require `terraform plan -var='create_workloads=false'`, `--phase infra`, saved-plan confirmation `APPLY_BROWSER_QA_INFRA_SAVED_PLAN`, and the existing empty-state/live-absence checks.

- [ ] **Step 2: Add Phase B readiness assertions**

Require exact 26-address state comparison, read-only `latest`/`ENABLED` checks for all three secrets, absence probes for the three workloads, a new saved plan using steady-state `create_workloads=true`, and `--phase workloads` before `APPLY_BROWSER_QA_WORKLOADS_SAVED_PLAN`.

- [ ] **Step 3: Assert ordering and recovery**

Require:

```text
Phase A apply -> output-backed GitHub variables -> Secret versions -> Phase B apply -> broker IAM verification -> dispatch
```

Require partial apply text to invalidate the saved plan and prohibit automatic retry, `-target`, manual deletion, or import without a separate recovery design.

- [ ] **Step 4: Verify RED**

Run:

```powershell
python -B -m unittest scripts.browser_qa.tests.test_operations_contract -v
```

Expected: failures because the runbook still describes one combined saved plan.

### Task 4: Implement the phase-aware guard

**Files:**
- Modify: `scripts/browser_qa/terraform_plan_guard.py`

- [ ] **Step 1: Add exact phase constants**

Keep a global union for unknown-address rejection, then map each phase to its exact meaningful resource/output create set.

- [ ] **Step 2: Ignore only known no-ops**

Validate a resource/output name against the global union before ignoring `("no-op",)`. This lets Phase B accept Phase A no-ops without allowing an unrelated no-op to bypass the guard.

- [ ] **Step 3: Require the phase in API and CLI**

Implement `validate_bootstrap_plan(plan, phase)` and argparse `--phase` with choices `infra` and `workloads`.

- [ ] **Step 4: Verify GREEN**

Run the Task 1 command. Expected: every guard test passes.

### Task 5: Implement the Terraform gate

**Files:**
- Modify: `deploy/gcp/envs/browser-qa-staging/variables.tf`
- Modify: `deploy/gcp/envs/browser-qa-staging/terraform.tfvars`
- Modify: `deploy/gcp/envs/browser-qa-staging/browser_qa.tf`
- Modify: `deploy/gcp/envs/browser-qa-staging/outputs.tf`

- [ ] **Step 1: Add the steady-state variable**

Add the exact boolean block from the design and `create_workloads = true` to tfvars.

- [ ] **Step 2: Gate exactly nine resources**

Add `count = var.create_workloads ? 1 : 0` only to the three Cloud Run and six Run IAM blocks. Update their cross-references with `[count.index]`.

- [ ] **Step 3: Make workload outputs nullable**

Use forms such as:

```hcl
value = one(google_cloud_run_v2_service.browser_qa_broker[*].uri)
```

- [ ] **Step 4: Verify GREEN and format**

Run:

```powershell
python -B -m unittest scripts.browser_qa.tests.test_terraform_contract -v
terraform -chdir=deploy/gcp/envs/browser-qa-staging fmt -check
terraform -chdir=deploy/gcp/envs/browser-qa-staging init -backend=false
terraform -chdir=deploy/gcp/envs/browser-qa-staging validate
```

Expected: all commands exit `0`.

### Task 6: Implement the human-only two-phase runbook

**Files:**
- Modify: `deploy/gcp/docs/OPERATIONS.md`
- Modify: `docs/superpowers/specs/2026-08-03-browser-qa-terraform-state-isolation-design.md`
- Modify: `docs/superpowers/plans/2026-08-03-browser-qa-terraform-state-isolation.md`

- [ ] **Step 1: Replace the combined plan with Phase A**

Preserve project/region/API/empty-state/live-absence gates, but generate and guard only the 26-resource plan with `create_workloads=false`.

- [ ] **Step 2: Insert Phase B after Secret versions**

Check exact Phase A state addresses, the three enabled `latest` versions without reading payloads, and workload absence. Generate a new steady-state saved plan and require the workload guard.

- [ ] **Step 3: Document fail-closed recovery**

State that any nonzero apply invalidates its saved plan and requires a separate recovery design for partial resources.

- [ ] **Step 4: Verify GREEN**

Run the Task 3 command. Expected: every operations contract test passes.

### Task 7: Verify scope and generate the new Phase A plan

**Files:**
- Modify only files identified by failed verification or review.

- [ ] **Step 1: Run the full Browser QA suite**

```powershell
python -B -m unittest discover -s scripts/browser_qa/tests -v
```

- [ ] **Step 2: Run Terraform validation and safety scans**

Run fmt/init/validate, scan Terraform/docs for secret payload patterns, and verify no production/main workflow or `deploy/gcp/envs/prod` runtime resource changed.

- [ ] **Step 3: Run change impact and review**

Use GitNexus `detect_changes` when the local index is available; otherwise record the analyzer failure and use exact `rg` call-site plus `git diff` review. Resolve every Critical/Important issue.

- [ ] **Step 4: Generate a fresh Phase A saved plan**

From the isolated root, generate a refreshing plan with `create_workloads=false`, export JSON, run `--phase infra`, and inspect the human plan. Expected: 26 create, 0 update/delete/replace, 4 infra outputs. Do not apply it.

- [ ] **Step 5: Commit with Lore trailers**

Commit only the reviewed source/docs changes. Do not add plan files, JSON exports, OAuth material, `tmp_*`, or `__pycache__`.
