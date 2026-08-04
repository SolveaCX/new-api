# Browser QA Email Alias Length Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep the run-scoped Gmail alias within the backend's 50-character email limit and prove the complete staging Browser QA replay succeeds.

**Architecture:** Replace the QA-only tag format with `qa-<run-id>-<8-char-HMAC-suffix>`. Apply the same strict contract in identity generation, broker validation, broker MCP configuration, Gmail lookup, fixtures, and operator documentation; do not change production email/schema limits.

**Tech Stack:** Python 3 `unittest`, GitHub Actions, Cloud Run Browser QA jobs, Gmail API broker, read-only Cloud SQL verification.

---

### Task 1: Lock the backend email-length contract with RED tests

**Files:**
- Modify: `scripts/browser_qa/tests/test_identity.py`
- Modify: `scripts/browser_qa/tests/test_broker.py`
- Modify: `scripts/browser_qa/tests/test_mcp.py`
- Modify: `scripts/browser_qa/tests/test_gmail.py`
- Modify: `scripts/browser_qa/tests/fixtures/gmail_flatkey_message.json`

- [ ] **Step 1: Add the failing alias-length regression test**

```python
def test_email_alias_fits_backend_registration_limit(self):
    identity = derive_identity(
        b"seed-with-32-bytes-minimum-value",
        "30906966375",
    )
    local, domain = "qaowner123456@gmail.com".split("@", 1)
    alias = f"{local}+{identity.email_tag}@{domain}"

    self.assertLessEqual(len(alias), 50)
```

- [ ] **Step 2: Change contract fixtures to the desired format before production code**

Use `qa-123456789-abc123de` for fixed test tags, require `r"^qa-123456789-[a-z0-9]{8}$"` in the identity test, and retain explicit invalid cases for uppercase suffixes, mismatched run ids, and extra request fields.

- [ ] **Step 3: Run the focused tests and verify RED**

Run:

```powershell
python -B -m unittest `
  scripts.browser_qa.tests.test_identity `
  scripts.browser_qa.tests.test_broker `
  scripts.browser_qa.tests.test_mcp `
  scripts.browser_qa.tests.test_gmail -v
```

Expected: failures show the old `flatkey-qa-...` tag and a 57-character alias, proving the tests detect the live staging incompatibility.

### Task 2: Implement the minimal shared tag contract

**Files:**
- Modify: `scripts/browser_qa/flatkey_browser_qa/identity.py`
- Modify: `scripts/browser_qa/flatkey_browser_qa/broker.py`
- Modify: `scripts/browser_qa/flatkey_browser_qa/broker_mcp.py`
- Modify: `scripts/browser_qa/flatkey_browser_qa/gmail.py`

- [ ] **Step 1: Shorten deterministic identity generation**

```python
email_suffix = _encode_alphabet(email_digest, _LOWER_ALNUM, 8)

return DerivedIdentity(
    run_id=run_id,
    username=f"qa{username_digits:08d}{username_suffix}",
    email_tag=f"qa-{run_id}-{email_suffix}",
    password=_derive_password(seed, run_id),
    key_name=f"cloud-qa-{run_id}",
)
```

- [ ] **Step 2: Make the broker and Gmail reader fail closed on the same format**

```python
_TAG_RE = re.compile(r"^qa-(?P<run_id>[0-9]+)-[a-z0-9]{8}$")
```

```python
if not isinstance(email_tag, str) or not email_tag.startswith(f"qa-{run_id}-"):
    raise ToolExecutionError("broker mcp configuration invalid")
```

```python
def _valid_email_tag(value):
    return isinstance(value, str) and re.fullmatch(
        r"qa-[0-9]+-[a-z0-9]{8}", value
    ) is not None
```

- [ ] **Step 3: Run the focused tests and verify GREEN**

Run the Task 1 command again.

Expected: all identity, broker, broker MCP, and Gmail tests pass.

### Task 3: Verify all Browser QA behavior and update operator-facing contracts

**Files:**
- Modify: `deploy/gcp/docs/OPERATIONS.md`
- Modify: `docs/browser-qa/browser-qa-technical-implementation.html`
- Modify: `docs/browser-qa/ai-browser-testing-migration-guide.html`
- Test: `scripts/browser_qa/tests/test_operations_contract.py`
- Test: `scripts/browser_qa/tests/test_workflow_contract.py`

- [ ] **Step 1: Update the documented tag examples**

Replace operator request examples with `qa-0-00000000`, describe the Gmail alias as `+qa-<run-id>-<nonce>`, and document that the shortened format exists to satisfy the application's 50-character email limit.

- [ ] **Step 2: Run the complete Browser QA suite**

```powershell
python -B -m unittest discover -s scripts/browser_qa/tests -v
```

Expected: all Python Browser QA tests pass with no failures.

- [ ] **Step 3: Run the browser evidence Node tests**

```powershell
node --test scripts/browser_qa/flatkey_browser_qa/browser_evidence_helper.test.cjs
```

Expected: all Node tests pass.

- [ ] **Step 4: Run diff hygiene and impact checks**

```powershell
git diff --check
gitnexus detect-changes --scope unstaged
```

If GitNexus remains unavailable because its Windows analyzer cannot index the Browser QA files, record that tool failure and verify direct callers with `rg` plus the complete test suite.

### Task 4: Review, deploy staging, and prove the live workflow

**Files:**
- Review: all Task 1-3 files
- Deploy trigger: remote branch `staging`

- [ ] **Step 1: Perform a focused code review**

Check exact-format validation, run-id binding, cleanup determinism, redaction, Gmail time filtering, and that no production runtime/schema path changed.

- [ ] **Step 2: Commit only intended files using the Lore protocol**

The commit must state the staging-only constraint, rejection of a backend schema expansion, targeted/full test evidence, and the remaining live replay check.

- [ ] **Step 3: Push the reviewed commit to `staging`**

```powershell
git push origin HEAD:staging
```

Expected: a new `GCP Deploy Staging (backend)` run starts for the pushed SHA.

- [ ] **Step 4: Monitor the new run to its terminal state**

Acceptance evidence:

- build passed;
- deploy passed;
- Browser QA replay status passed;
- replay checkpoint reached;
- exploration remains not started in core mode;
- cleanup passed;
- overall GitHub workflow succeeded.

- [ ] **Step 5: Perform final read-only staging verification**

Report only counts/status. Confirm no disposable candidate user remains after cleanup and do not expose the email alias, verification code, token, or private report.
