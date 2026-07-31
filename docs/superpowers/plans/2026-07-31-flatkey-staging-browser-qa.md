# Flatkey Staging Browser QA Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a manually triggered, staging-only Cloud Run browser QA system that semantically replays the recorded Flatkey onboarding Skill, performs bounded AI exploration, uploads redacted evidence, and deterministically removes every test API key and account.

**Architecture:** A private verification-code broker is the only component with Gmail OAuth access. A main Cloud Run Job derives a one-time identity, runs `codex exec` against Playwright MCP through a host-allowlisting proxy, enforces replay/exploration budgets, uploads redacted evidence, and attempts in-process cleanup; an independent cleanup Job always recomputes the same identity from the GitHub run id and verifies deletion. A standalone `workflow_dispatch` workflow builds and updates only QA resources, runs both Jobs with `--wait`, and never joins the production release gate.

**Tech Stack:** Python 3 standard library and `unittest`, OpenAI Codex CLI `0.146.0`, Playwright MCP `0.0.78`, Chromium installed at image build time, Docker, GitHub Actions, Google Cloud Run v2, Secret Manager, GCS, Artifact Registry, Workload Identity Federation, Terraform Google provider `~> 6.13`.

---

## File map

- `.agents/skills/flatkey-new-user-onboarding/SKILL.md` — repository copy of the recorded semantic onboarding workflow; keeps human confirmation rules.
- `.agents/skills/flatkey-new-user-onboarding/agents/openai.yaml` — Skill UI metadata.
- `.agents/skills/flatkey-new-user-onboarding/references/staging-cloud-qa-policy.md` — explicit unattended authorization, staging origins, exploration and cleanup boundaries.
- `.agents/skills/flatkey-new-user-onboarding/tests/staging-cloud-qa-scenarios.md` — RED/GREEN skill application evidence.
- `scripts/browser_qa/flatkey_browser_qa/identity.py` — HMAC-derived username, Gmail alias, password and key name.
- `scripts/browser_qa/flatkey_browser_qa/config.py` — strict environment parsing and immutable runtime settings.
- `scripts/browser_qa/flatkey_browser_qa/budget.py` — monotonic replay/exploration deadline and action budget.
- `scripts/browser_qa/flatkey_browser_qa/origin_policy.py` — deny-by-default hostname/IP/origin decisions.
- `scripts/browser_qa/flatkey_browser_qa/redaction.py` — recursive text/JSON secret and personal-email redaction.
- `scripts/browser_qa/flatkey_browser_qa/report.py` — result/manifest validation and failure-priority aggregation.
- `scripts/browser_qa/flatkey_browser_qa/api.py` — cookie-session staging API client with bounded retry.
- `scripts/browser_qa/flatkey_browser_qa/cleanup.py` — paginated token deletion, self-deletion and login-failure verification.
- `scripts/browser_qa/flatkey_browser_qa/gmail.py` — OAuth refresh, Gmail REST queries and MIME verification-code parsing.
- `scripts/browser_qa/flatkey_browser_qa/broker.py` — private Cloud Run HTTP broker.
- `scripts/browser_qa/flatkey_browser_qa/gcp.py` — metadata identity/access tokens and write-only GCS upload helpers.
- `scripts/browser_qa/flatkey_browser_qa/mcp.py` — minimal stdio JSON-RPC/MCP helpers.
- `scripts/browser_qa/flatkey_browser_qa/broker_mcp.py` — parameterless `get_current_verification_code` tool.
- `scripts/browser_qa/flatkey_browser_qa/control_mcp.py` — replay checkpoint and exploration-start control tools.
- `scripts/browser_qa/flatkey_browser_qa/mcp_budget_wrapper.py` — transparent Playwright MCP proxy enforcing exploration actions/time.
- `scripts/browser_qa/flatkey_browser_qa/egress_proxy.py` — Chromium HTTP/CONNECT allowlist proxy.
- `scripts/browser_qa/flatkey_browser_qa/supervisor.py` — signal-aware Codex lifecycle, evidence collection, upload and in-process cleanup.
- `scripts/browser_qa/flatkey_browser_qa/cleanup_job.py` — independent idempotent cleanup entrypoint.
- `scripts/browser_qa/config/allowed_hosts.json` — versioned staging/docs/static host allowlist.
- `scripts/browser_qa/config/result.schema.json` — strict Codex final-output schema.
- `scripts/browser_qa/config/qa-prompt.md` — versioned replay/exploration instructions.
- `scripts/browser_qa/Dockerfile`, `scripts/browser_qa/entrypoint.sh` — pinned non-root runtime image and entrypoints.
- `scripts/browser_qa/tests/` — focused Python, workflow, container and Terraform contract tests.
- `.dockerignore` — allow the selected repository Skill into the QA build context.
- `.github/workflows/gcp-browser-qa.yml` — manual build/deploy/run/always-cleanup workflow.
- `deploy/gcp/envs/prod/browser_qa.tf` — opt-in QA services, Jobs, identities, IAM, WIF, secrets and report bucket.
- `deploy/gcp/envs/prod/variables.tf`, `terraform.tfvars`, `outputs.tf` — QA switch, non-secret constants and resource outputs.
- `deploy/gcp/docs/OPERATIONS.md` — bootstrap, rotation, execution, evidence and cleanup recovery runbook.

### Task 1: Install and pressure-test the recorded Skill

**Files:**
- Create: `.agents/skills/flatkey-new-user-onboarding/SKILL.md`
- Create: `.agents/skills/flatkey-new-user-onboarding/agents/openai.yaml`
- Create: `.agents/skills/flatkey-new-user-onboarding/references/staging-cloud-qa-policy.md`
- Create: `.agents/skills/flatkey-new-user-onboarding/tests/staging-cloud-qa-scenarios.md`

- [ ] **Step 1: Record the failing application scenario**

Write the already-observed RED result into `staging-cloud-qa-scenarios.md`: the unmodified Skill stops at password takeover, CAPTCHA, final API-key creation confirmation and destructive cleanup; it defaults toward production URLs and has no exploration budget or report schema.

- [ ] **Step 2: Copy the recorded Skill without weakening its human workflow**

Copy `C:/Users/11247/Documents/onbroadskill/SKILL.md` and `agents/openai.yaml`, then change the frontmatter description to trigger-only wording:

```yaml
---
name: flatkey-new-user-onboarding
description: Use when onboarding a Flatkey user through browser signup, email verification, starting credit, API key setup, or first-call documentation.
---
```

Add one mode-selection rule: the unattended staging policy applies only when the invoking prompt explicitly names the versioned policy file and supplies a run id; otherwise every existing human confirmation remains mandatory.

- [ ] **Step 3: Add the minimal unattended staging policy**

The policy must state all of these invariants verbatim in substance:

```markdown
- Authorization covers only a deterministic disposable identity on staging, acceptance of staging terms, and creation/deletion of temporary keys and that identity.
- Writable origins are exactly https://staging-website.flatkey.ai and https://staging-console.flatkey.ai.
- https://docs.flatkey.ai is read-only in a separate cookie-free context.
- Production Flatkey origins, payment, subscription, invitation, admin, global settings and real model calls are forbidden.
- CAPTCHA or Turnstile causes a closed failure; never bypass it.
- Call qa_replay_checkpoint only after the account is in a cleanup-capable state; call qa_start_exploration before exploratory actions.
- Exploration stops at five minutes or thirty Playwright tool actions, whichever happens first.
- Never expose the Gmail base address, plus alias, password, code, Cookie, Authorization value or complete API key.
- Cleanup is owned by the supervisor and independent cleanup Job, not by the model's final statement.
```

- [ ] **Step 4: Run GREEN application tests with fresh agents**

Run the same scenario with the Skill and policy loaded, plus a variation where replay fails before exploration. Expected: the agent proceeds without human confirmation only for the disposable staging identity, refuses production/payment/CAPTCHA, enters exploration only after the checkpoint, and delegates deterministic cleanup to the runtime.

- [ ] **Step 5: Commit the Skill**

```powershell
git add .agents/skills/flatkey-new-user-onboarding
git commit -m "Make the recorded onboarding reusable in isolated staging QA" -m "Constraint: Human confirmation rules remain the default outside the explicit disposable staging policy." -m "Rejected: Replacing the recorded Skill with a cloud-only script | It would lose semantic replay and broaden unattended authority." -m "Confidence: high" -m "Scope-risk: narrow" -m "Directive: Never extend unattended authorization to production, payments, or non-disposable accounts." -m "Tested: RED and GREEN agent application scenarios recorded in the Skill test artifact." -m "Not-tested: Live staging browser execution awaits the runtime image and GCP resources."
```

### Task 2: Build deterministic identity, policy, budget and redaction foundations

**Files:**
- Create: `scripts/browser_qa/flatkey_browser_qa/__init__.py`
- Create: `scripts/browser_qa/flatkey_browser_qa/config.py`
- Create: `scripts/browser_qa/flatkey_browser_qa/identity.py`
- Create: `scripts/browser_qa/flatkey_browser_qa/budget.py`
- Create: `scripts/browser_qa/flatkey_browser_qa/origin_policy.py`
- Create: `scripts/browser_qa/flatkey_browser_qa/redaction.py`
- Create: `scripts/browser_qa/tests/test_identity.py`
- Create: `scripts/browser_qa/tests/test_budget.py`
- Create: `scripts/browser_qa/tests/test_origin_policy.py`
- Create: `scripts/browser_qa/tests/test_redaction.py`

- [ ] **Step 1: Write failing identity tests**

```python
def test_identity_is_stable_without_exposing_seed(self):
    first = derive_identity(b"seed-with-32-bytes-minimum-value", "123456789")
    second = derive_identity(b"seed-with-32-bytes-minimum-value", "123456789")
    self.assertEqual(first, second)
    self.assertRegex(first.username, r"^qa[0-9]{8}[a-z0-9]{8}$")
    self.assertRegex(first.email_tag, r"^flatkey-qa-123456789-[a-z0-9]{10}$")
    self.assertEqual(len(first.password), 30)
    self.assertNotIn("seed", repr(first))
```

Run: `python -m unittest scripts.browser_qa.tests.test_identity -v`
Expected: `ImportError` for the missing module.

- [ ] **Step 2: Implement HMAC-separated derivation**

Use `HMAC-SHA256(seed, b"flatkey-browser-qa/v1/<field>/" + run_id)` independently for username nonce, email tag and password bytes. Validate a decimal run id and at least 32 seed bytes. The password alphabet must contain upper, lower, digit and punctuation, with deterministic required-character insertion and no value in `repr`.

- [ ] **Step 3: Write failing budget and origin tests**

```python
def test_exploration_stops_on_first_limit(self):
    clock = FakeClock()
    budget = ExplorationBudget(300, 30, clock)
    budget.start()
    for _ in range(30):
        budget.consume_action()
    with self.assertRaises(BudgetExceeded):
        budget.consume_action()

def test_policy_denies_production_metadata_and_private_ips(self):
    policy = OriginPolicy.from_hosts(["staging-console.flatkey.ai", "docs.flatkey.ai"])
    for url in ["https://console.flatkey.ai", "http://169.254.169.254", "http://127.0.0.1"]:
        self.assertFalse(policy.decide(url).allowed)
```

Run the two tests and verify missing implementations fail.

- [ ] **Step 4: Implement strict configuration, origin decisions and budgets**

Reject unknown target origins, non-HTTPS destinations except the loopback proxy listener, URL credentials, non-HTTP(S) schemes, literal private/link-local/loopback IPs and DNS resolutions into those ranges. Track time with `time.monotonic`; a missing replay checkpoint never silently starts exploration.

- [ ] **Step 5: Write failing redaction tests**

```python
def test_redactor_removes_every_sensitive_representation(self):
    redactor = Redactor(email="owner+flatkey-qa-1-x@gmail.com", password="Aa1!secret", code="123456")
    raw = {"Authorization": "Bearer sk-live-secret", "message": "owner@gmail.com 123456 Aa1!secret", "cookie": "sid=abc"}
    text = json.dumps(redactor.clean(raw), sort_keys=True)
    for secret in ["owner", "123456", "Aa1!secret", "sk-live-secret", "sid=abc"]:
        self.assertNotIn(secret, text)
```

Run and verify failure, then implement recursive key-aware and pattern-aware redaction. Complete API-key matches become `[REDACTED_API_KEY]`; base/plus email become run-scoped placeholders; credential headers and cookie-like keys are replaced wholesale.

- [ ] **Step 6: Run the focused foundation suite**

Run: `python -m unittest scripts.browser_qa.tests.test_identity scripts.browser_qa.tests.test_budget scripts.browser_qa.tests.test_origin_policy scripts.browser_qa.tests.test_redaction -v`
Expected: all tests pass with no warnings.

- [ ] **Step 7: Commit the foundations**

```powershell
git add scripts/browser_qa/flatkey_browser_qa scripts/browser_qa/tests
git commit -m "Keep staging QA identities and evidence reproducible without retaining secrets" -m "Constraint: A hard-terminated main Job must be recoverable from only the identity seed and GitHub run id." -m "Rejected: Persisting generated credentials in GCS | Reports are a weaker boundary than Secret Manager and cleanup can recompute them." -m "Confidence: high" -m "Scope-risk: narrow" -m "Directive: Add every new evidence field to redaction tests before emitting it." -m "Tested: Focused identity, budget, origin-policy and recursive-redaction unit tests." -m "Not-tested: Browser and Cloud Run integration are covered by later tasks."
```

### Task 3: Implement idempotent, paginated cleanup

**Files:**
- Create: `scripts/browser_qa/flatkey_browser_qa/api.py`
- Create: `scripts/browser_qa/flatkey_browser_qa/cleanup.py`
- Create: `scripts/browser_qa/flatkey_browser_qa/cleanup_job.py`
- Create: `scripts/browser_qa/tests/test_cleanup.py`

- [ ] **Step 1: Write failing multi-page cleanup tests**

Create a local `ThreadingHTTPServer` fake staging API with login, paginated `GET /api/token/`, `DELETE /api/token/{id}` and `DELETE /api/user/self`. Seed 205 token ids, return one deletion as 404, and one transient 503 before success.

```python
result = CleanupRunner(client, page_size=100, max_attempts=3).run(identity)
self.assertEqual(result.deleted_token_count, 205)
self.assertTrue(result.account_deleted)
self.assertTrue(result.login_rejected_after_delete)
self.assertGreaterEqual(fake.list_calls, 4)
```

Run: `python -m unittest scripts.browser_qa.tests.test_cleanup -v`
Expected: missing cleanup module failure.

- [ ] **Step 2: Implement the staging API client**

Use `urllib.request` plus `http.cookiejar.CookieJar`. Only the configured staging-console origin is accepted. Retry connection errors and 5xx at most three attempts with bounded exponential delays; never retry 4xx except treating token-delete 404 as already absent. Do not log request bodies, cookies or response secrets.

- [ ] **Step 3: Implement the cleanup state machine**

Login with deterministic username/password; repeatedly list pages until no unseen ids remain; delete every id; list again and require an empty set; delete self; clear cookies; require the same login to be rejected. A login rejection before any session is treated as already-clean only when the API response is an authentication failure, never on 5xx or malformed responses.

- [ ] **Step 4: Prove idempotence and failure priority**

Add tests for account already absent, malformed pagination, repeated ids, a page whose `total` exceeds one page, lost DELETE response, persistent 5xx, account deletion failure and a second cleanup execution. Verify any unproven key/account state yields `cleanup_failed`.

- [ ] **Step 5: Run and commit**

Run: `python -m unittest scripts.browser_qa.tests.test_cleanup -v`
Expected: all cleanup cases pass.

```powershell
git add scripts/browser_qa/flatkey_browser_qa/api.py scripts/browser_qa/flatkey_browser_qa/cleanup.py scripts/browser_qa/flatkey_browser_qa/cleanup_job.py scripts/browser_qa/tests/test_cleanup.py
git commit -m "Make disposable staging accounts recoverable after interrupted browser runs" -m "Constraint: Cleanup has only ordinary user APIs and may be re-entered after a lost response." -m "Rejected: Deleting only the run-named key | Registration creates a default key and pagination can hide additional keys." -m "Confidence: high" -m "Scope-risk: moderate" -m "Directive: Cleanup is successful only after an empty token listing and rejected post-delete login are observed." -m "Tested: Local HTTP contract tests cover 205 keys, pagination, retries, 404, re-entry and failure priority." -m "Not-tested: Live staging response compatibility is verified in bootstrap."
```

### Task 4: Build the Gmail parser and private verification broker

**Files:**
- Create: `scripts/browser_qa/flatkey_browser_qa/gmail.py`
- Create: `scripts/browser_qa/flatkey_browser_qa/gcp.py`
- Create: `scripts/browser_qa/flatkey_browser_qa/broker.py`
- Create: `scripts/browser_qa/tests/fixtures/gmail_flatkey_message.json`
- Create: `scripts/browser_qa/tests/test_gmail.py`
- Create: `scripts/browser_qa/tests/test_broker.py`

- [ ] **Step 1: Write failing Gmail MIME/parser tests**

Use a synthetic Gmail payload containing multipart plain text and HTML, a matching exact plus alias, a known Flatkey sender/subject marker, an internal date after the run start and one six-digit code. Add rejection cases for wrong alias, old time, wrong sender, wrong subject, two codes and unrelated inbox mail.

Run: `python -m unittest scripts.browser_qa.tests.test_gmail -v`
Expected: missing parser failure.

- [ ] **Step 2: Implement OAuth and Gmail REST access**

Read the JSON secret only from `GMAIL_OAUTH_JSON`; require `refresh_token`, `token_uri`, `client_id`, `client_secret` and `gmail.readonly`. Refresh with form-encoded HTTPS, call `users/me/profile` to discover the base address, query only `to:<exact alias> after:<epoch>`, fetch bounded candidates, parse MIME locally and return only one code. Classify `invalid_grant` without unbounded retry and never log OAuth responses or message bodies.

- [ ] **Step 3: Write failing broker contract tests**

Test `POST /v1/current-code` with valid run id, plus tag and start time; reject arbitrary base addresses, stale/future windows, malformed tags, GET, oversized bodies and unexpected fields. Expected response is only `{"status":"ready","code":"123456"}` or `{"status":"pending"}`; error responses contain stable non-secret codes.

- [ ] **Step 4: Implement the private broker**

Use `ThreadingHTTPServer`, bind `0.0.0.0:$PORT`, apply request-size and deadline limits, validate the plus tag against the Gmail profile base address, and rely on Cloud Run IAM for caller authentication. Structured logs may include run id, status and latency but never alias, Gmail base, code, token or request body.

- [ ] **Step 5: Run and commit**

Run: `python -m unittest scripts.browser_qa.tests.test_gmail scripts.browser_qa.tests.test_broker -v`
Expected: all parser and broker cases pass.

```powershell
git add scripts/browser_qa/flatkey_browser_qa/gmail.py scripts/browser_qa/flatkey_browser_qa/gcp.py scripts/browser_qa/flatkey_browser_qa/broker.py scripts/browser_qa/tests
git commit -m "Keep personal Gmail access behind a one-purpose verification boundary" -m "Constraint: The browser agent must never receive OAuth material or arbitrary mailbox-query capability." -m "Rejected: A Cloud Run sidecar broker | Same-instance containers share a network boundary and are not strong credential isolation." -m "Confidence: high" -m "Scope-risk: moderate" -m "Directive: Broker responses must never grow beyond status and the matched six-digit code." -m "Tested: Synthetic MIME, OAuth error classification, alias/time/sender/subject filters and HTTP contract tests." -m "Not-tested: Live Gmail delivery is verified only after the private service is deployed."
```

### Task 5: Add restricted MCP tools and enforce Playwright exploration budgets

**Files:**
- Create: `scripts/browser_qa/flatkey_browser_qa/mcp.py`
- Create: `scripts/browser_qa/flatkey_browser_qa/broker_mcp.py`
- Create: `scripts/browser_qa/flatkey_browser_qa/control_mcp.py`
- Create: `scripts/browser_qa/flatkey_browser_qa/mcp_budget_wrapper.py`
- Create: `scripts/browser_qa/tests/test_mcp.py`
- Create: `scripts/browser_qa/tests/test_mcp_budget_wrapper.py`

- [ ] **Step 1: Write failing MCP protocol tests**

Feed newline-delimited JSON-RPC initialize, `tools/list` and `tools/call` messages through in-memory streams. Assert the broker exposes exactly one tool named `get_current_verification_code` with an empty-object input schema and rejects any arguments. Assert control exposes only `qa_replay_checkpoint` and `qa_start_exploration`.

- [ ] **Step 2: Implement the broker and control MCP servers**

The broker MCP owns run id, plus tag, start time and broker URL from its process environment; it obtains a Cloud Run identity token from metadata for the broker audience and polls every five seconds for no more than 120 seconds. The model supplies no mailbox data. Control writes atomic JSON state under the supervisor-owned runtime directory.

- [ ] **Step 3: Write failing transparent-wrapper budget tests**

Use a fake child MCP server. Before the exploration marker, forward tool calls unchanged. After the marker, forward exactly 30 `tools/call` messages and return JSON-RPC budget errors for further calls. Advance a fake monotonic clock beyond 300 seconds and assert the same closed behavior. Initialization/listing messages do not consume actions.

- [ ] **Step 4: Implement the Playwright MCP wrapper**

Spawn the fixed child command without a shell, forward stdin/stdout/stderr safely, parse client requests only to count `tools/call`, poll the atomic control-state file, and terminate the child on parent signal or EOF. Protocol parse failures stop closed and surface a non-secret error.

- [ ] **Step 5: Run and commit**

Run: `python -m unittest scripts.browser_qa.tests.test_mcp scripts.browser_qa.tests.test_mcp_budget_wrapper -v`
Expected: all MCP contract and budget tests pass.

```powershell
git add scripts/browser_qa/flatkey_browser_qa/mcp.py scripts/browser_qa/flatkey_browser_qa/broker_mcp.py scripts/browser_qa/flatkey_browser_qa/control_mcp.py scripts/browser_qa/flatkey_browser_qa/mcp_budget_wrapper.py scripts/browser_qa/tests
git commit -m "Constrain the agent to the current verification code and a measurable exploration budget" -m "Constraint: Codex may invoke MCP tools but must not choose a mailbox or bypass the thirty-action limit." -m "Rejected: Prompt-only action counting | Model compliance is not an enforcement boundary." -m "Confidence: high" -m "Scope-risk: moderate" -m "Directive: Any new MCP argument or tool requires a new threat-model and contract test." -m "Tested: JSON-RPC protocol, zero-argument broker tool, phase transitions, time and action cutoffs." -m "Not-tested: Compatibility with the pinned live Playwright MCP binary is covered by the container smoke test."
```

### Task 6: Enforce browser egress and supervise Codex/report lifecycle

**Files:**
- Create: `scripts/browser_qa/flatkey_browser_qa/egress_proxy.py`
- Create: `scripts/browser_qa/flatkey_browser_qa/report.py`
- Create: `scripts/browser_qa/flatkey_browser_qa/supervisor.py`
- Create: `scripts/browser_qa/config/allowed_hosts.json`
- Create: `scripts/browser_qa/config/result.schema.json`
- Create: `scripts/browser_qa/config/qa-prompt.md`
- Create: `scripts/browser_qa/tests/test_egress_proxy.py`
- Create: `scripts/browser_qa/tests/test_report.py`
- Create: `scripts/browser_qa/tests/test_supervisor.py`

- [ ] **Step 1: Write failing proxy policy tests**

Start local destination servers and the proxy. Verify allowed HTTP/CONNECT hosts are forwarded, while production Flatkey, redirects to production, localhost, RFC1918, link-local metadata, literal-IP tricks, non-HTTP schemes and unknown hosts are rejected. Verify CONNECT uses the requested host policy before opening a socket.

- [ ] **Step 2: Implement the deny-by-default proxy**

Use a threaded HTTP proxy with bounded headers, connection/read timeouts and no request-body logging. Resolve hostnames before connection and reject any private/link-local/loopback result. Keep a versioned exact/suffix host allowlist for the two staging origins, read-only docs and audited static dependencies. Chromium receives `--proxy-server`, an empty `--proxy-bypass`, disabled QUIC, non-proxied WebRTC and service workers; Playwright's own origin flags are defense in depth only.

- [ ] **Step 3: Write failing report/state aggregation tests**

Validate the JSON schema fields for replay, exploration, budgets and findings. Reject missing fields or invalid severities. Assert status priority `cleanup_failed > infrastructure_failed > replay_failed > findings_detected > passed`, with `info` excluded from findings alerts. Assert manifest cleanup fields can only be overwritten by runtime cleanup results.

- [ ] **Step 4: Write failing supervisor lifecycle tests**

Use fake subprocess, clock, uploader and cleanup runner. Cover normal completion, Codex nonzero, invalid result, 14-minute internal deadline, SIGTERM, upload failure and cleanup failure. In every case cleanup is attempted before final exit; the process never prints the prompt-expanded password, alias, code or API key.

- [ ] **Step 5: Implement the supervisor**

Preflight `/api/status` for email verification and disabled Turnstile; classify alias-restriction responses explicitly. Derive the identity in memory, start the proxy, write an owner-only temporary Codex config, and run:

```text
codex exec --ignore-user-config --strict-config --ephemeral --json --output-schema /opt/flatkey-browser-qa/config/result.schema.json --output-last-message <temp-result> --sandbox workspace-write --model gpt-5.4 --cd <empty-workspace> -
```

Configure only the wrapped Playwright server, broker MCP and control MCP. Set `sandbox_workspace_write.network_access=false`; the startup contract test must fail if a Codex shell can reach an external URL or metadata. Redact JSONL events as they arrive, stop Codex at the internal deadline, collect only masked screenshots/output, run cleanup in `finally`, upload through GCS JSON API after Codex exits, and return the aggregate status exit code.

- [ ] **Step 6: Run and commit**

Run: `python -m unittest scripts.browser_qa.tests.test_egress_proxy scripts.browser_qa.tests.test_report scripts.browser_qa.tests.test_supervisor -v`
Expected: all policy, report and lifecycle cases pass.

```powershell
git add scripts/browser_qa/flatkey_browser_qa/egress_proxy.py scripts/browser_qa/flatkey_browser_qa/report.py scripts/browser_qa/flatkey_browser_qa/supervisor.py scripts/browser_qa/config scripts/browser_qa/tests
git commit -m "Bound autonomous browser exploration while preserving failure evidence and cleanup time" -m "Constraint: Cloud Run hard timeout is twenty minutes, so Codex ends at fourteen minutes and cleanup/reporting retain reserved time." -m "Rejected: Trusting Playwright origin flags alone | Its own help states they are not a security boundary and do not cover redirects." -m "Confidence: medium" -m "Scope-risk: broad" -m "Directive: Unknown egress hosts stay blocked until reviewed and committed to the allowlist." -m "Tested: Proxy bypass cases, status priority, schema rejection, timeout, signal, upload and cleanup lifecycle tests." -m "Not-tested: Third-party staging asset hosts may require an allowlist update after the first controlled run."
```

### Task 7: Build and smoke-test the non-root runtime image

**Files:**
- Create: `scripts/browser_qa/Dockerfile`
- Create: `scripts/browser_qa/entrypoint.sh`
- Create: `scripts/browser_qa/tests/test_container_contract.py`
- Modify: `.dockerignore`

- [ ] **Step 1: Write failing Dockerfile contract tests**

Assert exact pins for `@openai/codex@0.146.0`, `@playwright/mcp@0.0.78` and its matching Playwright dependency; assert `USER` is non-root; assert no `COPY .`, Gmail local path, `.codex/auth.json`, client secret or secret value; assert only QA scripts, config and selected Skill enter the image.

- [ ] **Step 2: Add build-context exceptions and the pinned image**

Append `.dockerignore` exceptions only for the onboarding Skill beneath the existing `*.md` rule. Build from a pinned Node 22 Debian slim tag, install Python and the exact npm packages, install matching Chromium during build, copy files to `/opt/flatkey-browser-qa`, create an unprivileged user and select `broker`, `main` or `cleanup` through the entrypoint.

- [ ] **Step 3: Build and run container smoke tests**

Run:

```powershell
docker build -f scripts/browser_qa/Dockerfile -t flatkey-browser-qa:test .
docker run --rm --entrypoint sh flatkey-browser-qa:test -c 'test "$(id -u)" != 0 && codex --version && playwright-mcp --version && python3 -m unittest discover -s /opt/flatkey-browser-qa/tests -v'
```

Expected: nonzero UID, Codex `0.146.0`, Playwright MCP `0.0.78`, focused tests pass. Run a headless Playwright MCP initialize/tools-list smoke through the budget wrapper and require a clean exit.

- [ ] **Step 4: Commit the image**

```powershell
git add .dockerignore scripts/browser_qa/Dockerfile scripts/browser_qa/entrypoint.sh scripts/browser_qa/tests/test_container_contract.py
git commit -m "Make the browser QA runtime reproducible without desktop state or embedded credentials" -m "Constraint: Cloud execution cannot depend on the user's Mac recording machine or ChatGPT login." -m "Rejected: Installing npm latest at Job startup | Runtime drift would make replay evidence non-reproducible." -m "Confidence: high" -m "Scope-risk: moderate" -m "Directive: Upgrade Codex, MCP and Chromium only together with container smoke evidence." -m "Tested: Dockerfile contract, image build, non-root UID, version checks and headless MCP smoke." -m "Not-tested: Cloud Run kernel sandbox behavior is checked after deployment."
```

### Task 8: Add the manual GitHub Actions execution and unconditional cleanup

**Files:**
- Create: `.github/workflows/gcp-browser-qa.yml`
- Create: `scripts/browser_qa/tests/test_workflow_contract.py`

- [ ] **Step 1: Write failing workflow contract tests**

Parse YAML as text-safe data and assert: only `workflow_dispatch`; permissions are `contents: read` and `id-token: write`; one concurrency group with `cancel-in-progress: false`; no production environment; WIF variables name the QA provider/deployer; image tag includes commit SHA; main and cleanup commands both use `gcloud run jobs execute --wait`; cleanup has `if: always()`; `cleanup-only` accepts an explicit original run id; no secret value appears in an argument or output.

- [ ] **Step 2: Implement build/deploy/run jobs**

Normal modes authenticate with the dedicated QA WIF identity, push the pinned image to the dedicated QA repository, update only `flatkey-staging-browser-qa-broker`, `flatkey-staging-browser-qa` and `flatkey-staging-browser-qa-cleanup`, then execute main with `--wait`. Always execute cleanup with the same effective run id. `cleanup-only` skips image mutation and main execution. Preserve main status, fetch only the sanitized manifest, write a summary with counts/status/GCS URI, and fail the standalone workflow for cleanup/infrastructure/replay/findings states without connecting it to a release gate.

- [ ] **Step 3: Lint and commit**

Run: `python -m unittest scripts.browser_qa.tests.test_workflow_contract -v`
Run: `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.7 .github/workflows/gcp-browser-qa.yml`
Expected: both pass.

```powershell
git add .github/workflows/gcp-browser-qa.yml scripts/browser_qa/tests/test_workflow_contract.py
git commit -m "Make every manual staging browser run end with an independently observed cleanup" -m "Constraint: A main Job timeout or nonzero exit must not suppress the cleanup execution." -m "Rejected: Embedding QA in staging deploy workflows | It would couple exploratory alerts to service publication." -m "Confidence: high" -m "Scope-risk: moderate" -m "Directive: Keep this workflow manual and outside production environments until a separate release-gate design is approved." -m "Tested: Workflow contract and actionlint checks cover trigger, WIF, wait semantics, concurrency and always-cleanup." -m "Not-tested: GitHub-hosted WIF execution requires provisioned cloud resources."
```

### Task 9: Provision isolated QA resources with least privilege

**Files:**
- Create: `deploy/gcp/envs/prod/browser_qa.tf`
- Modify: `deploy/gcp/envs/prod/variables.tf`
- Modify: `deploy/gcp/envs/prod/terraform.tfvars`
- Modify: `deploy/gcp/envs/prod/outputs.tf`
- Create: `scripts/browser_qa/tests/test_terraform_contract.py`

- [ ] **Step 1: Write failing Terraform contract tests**

Assert the file defines: dedicated Artifact Registry repository; QA, broker, cleanup and deployer service accounts; three secret containers with no secret versions; private broker v2 Service; main and cleanup v2 Jobs with task count/parallelism 1, retries 0, timeouts 1200s/300s; private uniform-access GCS bucket with public-access prevention and 14-day lifecycle; branch/repository-bound WIF; per-resource Run IAM; secret access only for the intended identities; report creator/admin/viewer split; lifecycle image ignores; no LB, DNS, certificate, traffic or existing service address.

- [ ] **Step 2: Implement the opt-in resources**

Add `enable_browser_qa` default `false` and set it `true` in the environment tfvars for the approved test environment. Use count/for_each gates consistently. Terraform owns secret metadata only for `flatkey-browser-qa-codex-api-key`, `flatkey-browser-qa-identity-seed` and `flatkey-browser-qa-gmail-oauth`; no value enters variables or state. Use a harmless public placeholder image for first creation and ignore subsequent CI-owned image changes.

Bind the WIF provider to `assertion.repository == 'SolveaCX/new-api' && assertion.ref == 'refs/heads/staging'`. Grant the deployer only repository writer, resource-level Run developer/invoker, service-account user on the three runtime identities and report-object viewer. Grant main only Codex/seed access, broker invoke and object create; broker only Gmail secret access; cleanup only seed access and report-bucket object admin.

- [ ] **Step 3: Format, initialize and validate**

Run from `deploy/gcp/envs/prod`:

```powershell
terraform fmt -check -recursive
terraform init -backend=false
terraform validate
```

Expected: formatting and validation succeed without touching remote state.

- [ ] **Step 4: Run the static safety contract and commit**

Run: `python -m unittest scripts.browser_qa.tests.test_terraform_contract -v`
Expected: all least-privilege/resource-isolation assertions pass.

```powershell
git add deploy/gcp/envs/prod scripts/browser_qa/tests/test_terraform_contract.py
git commit -m "Isolate staging browser QA identities, artifacts and deploy authority from application services" -m "Constraint: QA metadata shares the existing prod Terraform root/state, but reports and runtime authority must remain dedicated." -m "Rejected: Reusing the application deployer | Its project-level roles exceed the QA update surface and do not prove branch/resource isolation." -m "Confidence: medium" -m "Scope-risk: broad" -m "Directive: Abort any apply whose refreshing plan mentions existing Cloud Run services, load balancers, certificates, DNS or traffic." -m "Tested: Terraform format, backend-free validate and static IAM/resource boundary tests." -m "Not-tested: Refreshing live plan and apply require current Owner ADC."
```

### Task 10: Document bootstrap, secret rotation and recovery

**Files:**
- Modify: `deploy/gcp/docs/OPERATIONS.md`
- Modify: `docs/superpowers/specs/2026-07-31-flatkey-staging-browser-qa-design.md`

- [ ] **Step 1: Add the operational runbook**

Document exact non-logging commands for: refreshing plan review; applying only after the plan contains QA addresses; adding identity seed and Codex API key via stdin; transforming the local Gmail credential JSON in memory so the existing file is never committed; publishing OAuth `In production`, reauthorizing and rotating the secret; verifying broker IAM denial; first core replay; full exploration run; GCS report lookup; `cleanup-only` with original run id; `invalid_grant`; alias-restriction failure; and abort conditions for any existing service/LB/cert/traffic diff.

The runbook must explicitly say the current Testing token proves bootstrap only, and repeatability is accepted only after an In-production token rotation and second full run.

- [ ] **Step 2: Mark the design approved and implementation-linked**

Set the design status to `已批准，实施中` and link this plan. Do not rewrite architectural decisions.

- [ ] **Step 3: Commit documentation**

```powershell
git add deploy/gcp/docs/OPERATIONS.md docs/superpowers/specs/2026-07-31-flatkey-staging-browser-qa-design.md
git commit -m "Leave a recoverable path for Gmail expiry, interrupted runs and infrastructure drift" -m "Constraint: Personal OAuth and API secret values must never appear in Terraform, GitHub logs, command arguments or committed files." -m "Rejected: Treating a Testing refresh token as durable | External Testing tokens can expire after roughly seven days." -m "Confidence: high" -m "Scope-risk: narrow" -m "Directive: Repeat the full run after OAuth publication and every credential rotation." -m "Tested: Commands are cross-checked against resource names and workflow inputs." -m "Not-tested: Secret-writing and cleanup-only commands are exercised during bootstrap."
```

### Task 11: Verify, review, plan safely, and bootstrap staging

**Files:**
- Modify only files identified by review or failed focused verification.

- [ ] **Step 1: Run the complete focused suite**

```powershell
python -m unittest discover -s scripts/browser_qa/tests -v
docker build -f scripts/browser_qa/Dockerfile -t flatkey-browser-qa:test .
terraform -chdir=deploy/gcp/envs/prod fmt -check -recursive
terraform -chdir=deploy/gcp/envs/prod init -backend=false
terraform -chdir=deploy/gcp/envs/prod validate
```

Expected: every new test, image build and backend-free Terraform validation passes. Record the known unrelated `go test ./...` baseline failures rather than changing them.

- [ ] **Step 2: Run change-scope and two-stage reviews**

Run GitNexus `detect_changes()` against `origin/staging`. Dispatch a spec-compliance reviewer against the approved design and this plan, fix every gap, then dispatch a code-quality/security reviewer and fix every Critical/Important issue. Re-run focused verification after fixes.

- [ ] **Step 3: Generate a refreshing live Terraform plan**

With current Owner ADC, run:

```powershell
terraform -chdir=deploy/gcp/envs/prod init
terraform -chdir=deploy/gcp/envs/prod plan -out=browser-qa.plan
terraform -chdir=deploy/gcp/envs/prod show -no-color browser-qa.plan | Set-Content -Encoding utf8 browser-qa-plan.txt
```

Audit every address. Required result: only dedicated browser-QA resources and the three new outputs/variables. If any existing service, URL map, certificate, DNS, LB, traffic or unrelated IAM resource changes, do not apply; fix drift/lifecycle first. Delete local plan artifacts after the audit without committing them.

- [ ] **Step 4: Apply resource metadata and add secret values through stdin**

Apply only the audited saved plan. Add a random 32-byte identity seed, the dedicated `CODEX_API_KEY`, and the Gmail OAuth JSON through non-echoing stdin pipelines from the existing local credential file. Never print secret values. Verify Secret Manager versions exist and that the QA Job identities—not the deployer—have only their intended access.

- [ ] **Step 5: Execute bootstrap acceptance**

From the `staging` ref, manually run core replay with exploration disabled, then the full five-minute/thirty-action mode. Verify private GCS manifest/evidence, redaction scan, report/summary agreement, all paginated keys absent and account login rejected. Invalidate broker auth once and hard-stop the main Job once; verify `always()` cleanup and `cleanup-only` recover the same identity.

- [ ] **Step 6: Rotate out of OAuth Testing mode and prove repeatability**

Publish the OAuth app `In production`, reauthorize `gmail.readonly`, rotate the Gmail secret, and run one more full flow. Only that second successful full run qualifies the system as repeatable.

- [ ] **Step 7: Final Lore commit after review fixes**

```powershell
git add -A
git commit -m "Prove the staging browser QA loop is bounded, observable and self-cleaning" -m "Constraint: Success requires both functional evidence and independently verified cleanup; it does not authorize production release gating." -m "Rejected: Declaring success from unit tests alone | Gmail, Cloud Run IAM, browser egress and staging cleanup are integration boundaries." -m "Confidence: high" -m "Scope-risk: broad" -m "Directive: Keep reports private, retain the fourteen-day lifecycle, and rerun cleanup-only for any interrupted workflow." -m "Tested: Focused unit/contracts, container smoke, Terraform validate/refreshing-plan audit, core replay, full exploration, forced failure, hard-stop cleanup and post-OAuth-rotation repeat run." -m "Not-tested: Production origins and release paths are intentionally out of scope."
```

## Completion evidence

- Every new Python unit/contract test passes from the repository root.
- The pinned image builds, runs as non-root and initializes headless Playwright MCP.
- Workflow lint proves manual-only execution, `--wait`, single concurrency and unconditional cleanup.
- Terraform validate passes and a refreshing plan contains no existing application/LB/certificate/DNS/traffic diff.
- A core staging run and a bounded exploration run upload only redacted private artifacts.
- Forced broker failure and hard termination still lead to independently verified account/key cleanup.
- OAuth is published In production, the token is rotated, and a second full run proves repeatability.
- No production deploy, traffic, approval gate or origin is mutated.
