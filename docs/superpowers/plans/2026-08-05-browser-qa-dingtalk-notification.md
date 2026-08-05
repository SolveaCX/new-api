# Browser QA DingTalk Notification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every staging Browser QA terminal outcome send one sanitized DingTalk report and prove the complete replay/cleanup/reporting path with a real run.

**Architecture:** The Browser QA reusable workflow exports a closed set of safe manifest fields and invokes a tested Python notification module from an `if: always()` step before its final failure gate. GitHub Actions supplies the webhook through one explicitly declared reusable-workflow secret, while the Python module validates inputs, retries transient delivery errors, and requires DingTalk `errcode == 0`.

**Tech Stack:** GitHub Actions YAML, Python 3 standard library, `unittest`, GitHub CLI, Google Cloud CLI.

---

### Task 1: Lock the notification contract with failing tests

**Files:**
- Create: `scripts/browser_qa/tests/test_dingtalk.py`
- Modify: `scripts/browser_qa/tests/test_workflow_contract.py`

- [ ] **Step 1: Write the failing sender tests**

Add tests that import `scripts.browser_qa.flatkey_browser_qa.dingtalk`, construct a report with `passed`, `not_started`, zero actions/findings, and assert the rendered Markdown includes only the safe fields. Add transport tests using an injected opener and sleeper: retry two transient failures before success, reject a nonzero DingTalk `errcode`, and assert exception text never includes the webhook.

- [ ] **Step 2: Write the failing workflow tests**

Require `workflow_call.secrets.STAGING_BROWSER_QA_DINGTALK_WEBHOOK`, explicit pass-through from the staging deploy caller, an `if: always()` notification step before `Fail standalone workflow for actionable QA states`, and step-level secret injection only. Require the manifest step to export final/replay/exploration/action/finding/cleanup/GCS outputs in both success and fallback paths.

- [ ] **Step 3: Verify RED**

Run:

```powershell
python -B -m unittest scripts.browser_qa.tests.test_dingtalk scripts.browser_qa.tests.test_workflow_contract -v
```

Expected: failure because the DingTalk module, workflow notification step, reusable secret contract, and safe outputs do not yet exist.

### Task 2: Implement the minimal tested sender

**Files:**
- Create: `scripts/browser_qa/flatkey_browser_qa/dingtalk.py`
- Test: `scripts/browser_qa/tests/test_dingtalk.py`

- [ ] **Step 1: Define the closed report model**

Implement `DingTalkReport` with validated final/replay/exploration/cleanup status values, non-negative integer counts, a GitHub Actions HTTPS URL, and a `gs://` manifest URI. Render a Markdown title beginning with `Staging Browser QA` and exactly the approved report fields.

- [ ] **Step 2: Implement bounded delivery**

Implement JSON POST via `urllib.request`, three total attempts, short bounded backoff through an injected sleeper, retry for transport errors/HTTP 429/5xx, and success only when the parsed JSON object has integer `errcode` zero. Error messages must not contain the webhook or payload.

- [ ] **Step 3: Add environment entry point**

Read the webhook and safe report fields from environment variables, build the GitHub run URL from the supplied safe value, send one Markdown message, and print only `DINGTALK_NOTIFICATION_SENT` after DingTalk acceptance.

- [ ] **Step 4: Verify GREEN for the module**

Run:

```powershell
python -B -m unittest scripts.browser_qa.tests.test_dingtalk -v
```

Expected: all DingTalk sender tests pass with no network access.

### Task 3: Wire the reusable workflow and staging caller

**Files:**
- Modify: `.github/workflows/gcp-browser-qa.yml`
- Modify: `.github/workflows/gcp-deploy-staging.yml`
- Test: `scripts/browser_qa/tests/test_workflow_contract.py`

- [ ] **Step 1: Declare and pass the reusable secret**

Declare `STAGING_BROWSER_QA_DINGTALK_WEBHOOK` as required under `workflow_call.secrets`. Pass it explicitly from the `browser-qa-core` job in `gcp-deploy-staging.yml`.

- [ ] **Step 2: Export only safe manifest outputs**

Have the manifest step export `manifest_status`, `replay_status`, `exploration_status`, `exploration_actions`, `finding_count`, `cleanup_status`, and `gcs_uri`. Use closed fallback values when the manifest cannot be fetched or validated.

- [ ] **Step 3: Add the terminal notification step**

Make checkout available to all modes, then add `Send terminal Browser QA report to DingTalk` with `if: always()` immediately before the final gate. Inject the webhook only as `DINGTALK_WEBHOOK` in that step and pass all other fields as safe environment values.

- [ ] **Step 4: Verify GREEN for workflow contracts**

Run:

```powershell
python -B -m unittest scripts.browser_qa.tests.test_workflow_contract -v
```

Expected: all workflow contract tests pass.

### Task 4: Verify, publish, and run staging end to end

**Files:**
- Modify only the files listed in Tasks 1-3 and these two planning documents.

- [ ] **Step 1: Run the complete Browser QA suite**

Run:

```powershell
python -B -m unittest discover -s scripts/browser_qa/tests -v
```

Expected: zero failures and zero errors.

- [ ] **Step 2: Review the exact diff and preserve user files**

Confirm `.gitignore`, `.gitnexus/`, caches, temporary diff files, and the untracked file named `-` are not staged. Commit only the notification implementation, tests, workflow changes, and design/plan documents using a Lore-format commit message.

- [ ] **Step 3: Configure the repository secret without displaying it**

Pipe the latest payload of GCP secret `week-one-qa-dingtalk-webhook` directly to `gh secret set STAGING_BROWSER_QA_DINGTALK_WEBHOOK --repo SolveaCX/new-api`, then verify only that the secret name exists.

- [ ] **Step 4: Push to staging and capture the triggered run**

Fetch `origin/staging`, verify it is still the commit used as the implementation base, then push the tested commit to `staging`. Capture the exact staging deployment run and the reusable Browser QA run rather than redispatching blindly.

- [ ] **Step 5: Follow all terminal evidence**

Wait for staging deployment, main replay, registration verification-code flow, API-key exercise, cleanup, and notification to finish. If cleanup fails or is skipped, execute `cleanup-only` with the original Browser QA run id.

- [ ] **Step 6: Verify the private report and DingTalk delivery**

Download only `gs://<bucket>/runs/<original-run-id>/manifest.json`, validate the safe summary and cleanup status, inspect the Actions notification step for `DINGTALK_NOTIFICATION_SENT`, and report the terminal result with the GitHub run URL and private GCS URI.
