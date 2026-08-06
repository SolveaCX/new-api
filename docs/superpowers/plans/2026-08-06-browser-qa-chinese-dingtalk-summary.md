# Browser QA Chinese DingTalk Summary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every staging Browser QA DingTalk terminal report Chinese-first while retaining raw status codes, and require AI finding titles to use concise Simplified Chinese.

**Architecture:** Keep the existing sanitized `DingTalkReport` input contract unchanged. Add deterministic label and summary mappings only in the DingTalk presentation layer, and add one language requirement to the existing bounded QA prompt; no translation API, schema expansion, workflow topology change, or infrastructure change is needed.

**Tech Stack:** Python 3 standard library, `unittest`, Markdown payloads, GitHub Actions, existing Browser QA prompt/container contracts.

---

## File structure

- Modify `scripts/browser_qa/tests/test_dingtalk.py`: lock Chinese titles, explanations, field labels, status mappings, severity/confidence mappings, raw code preservation, and existing redaction behavior.
- Modify `scripts/browser_qa/flatkey_browser_qa/dingtalk.py`: render the validated report contract as Chinese-first Markdown without changing delivery, signing, retry, or validation behavior.
- Modify `scripts/browser_qa/tests/test_container_contract.py`: lock the Simplified Chinese finding-title instruction into the container prompt contract.
- Modify `scripts/browser_qa/config/qa-prompt.md`: instruct the AI to write concise Simplified Chinese finding titles while retaining necessary product and technical names.
- Use `docs/superpowers/specs/2026-08-06-browser-qa-chinese-dingtalk-summary-design.md` as the acceptance source of truth.

### Task 0: Integrate the current staging head before editing runtime code

**Files:**
- Merge only; do not edit files unless Git reports a real conflict.

- [ ] **Step 1: Fetch the exact current staging branch without tags**

```powershell
git fetch --no-tags origin refs/heads/staging:refs/remotes/origin/staging
$apiStaging = (gh api repos/SolveaCX/new-api/git/ref/heads/staging --jq .object.sha).Trim()
$trackingStaging = (git rev-parse origin/staging).Trim()
if ($apiStaging -ne $trackingStaging) { throw "origin/staging is stale" }
```

Expected: the API SHA and `origin/staging` SHA match. The staging branch had already advanced beyond this feature branch while the design was being reviewed, so this check is mandatory.

- [ ] **Step 2: Merge current staging into the published feature branch**

```powershell
git merge --no-ff origin/staging `
  -m "Carry Chinese QA alert design over current staging" `
  -m "Constraint: Preserve newer staging work before changing release notifications`nRejected: Force-push an old feature base | It could overwrite newly deployed staging changes`nConfidence: high`nScope-risk: moderate`nDirective: Recheck origin/staging immediately before the final staging push`nTested: Merge-base and staged diff inspection`nNot-tested: Runtime behavior is verified by the following TDD tasks"
```

Expected: a clean merge. If Git reports a conflict, stop the merge implementation step, inspect both sides, preserve the newer staging behavior plus the approved design document, then run the baseline tests before continuing. Never force-push staging.

- [ ] **Step 3: Establish a green baseline on the integrated branch**

```powershell
python -B -m unittest scripts.browser_qa.tests.test_dingtalk scripts.browser_qa.tests.test_container_contract -v
```

Expected: all pre-change tests PASS. If not, diagnose the integrated staging baseline before adding localization tests.

### Task 1: Chinese-first DingTalk report rendering

**Files:**
- Modify: `scripts/browser_qa/tests/test_dingtalk.py`
- Modify: `scripts/browser_qa/flatkey_browser_qa/dingtalk.py`

- [ ] **Step 1: Change the closed-contract test to require Chinese-first output**

Update `test_report_renders_only_the_closed_sanitized_contract` so its core assertions are:

```python
self.assertEqual(payload["markdown"]["title"], "Staging 浏览器 QA：全部通过")
self.assertIn("> 测试已执行完成，未发现需要关注的问题。", markdown)
for expected in [
    "最终状态：全部通过（`passed`）",
    "录制回放：通过（`passed`）",
    "AI 探索：未开始（`not_started`）",
    "探索动作数：`0`",
    "问题数量：`0`",
    "账号清理：通过（`passed`）",
    "运行记录：[打开 GitHub Actions](https://github.com/SolveaCX/new-api/actions/runs/12345)",
    "证据文件：`gs://vocai-gemini-prod-flatkey-browser-qa-reports/runs/12345/manifest.json`",
]:
    self.assertIn(expected, markdown)
for forbidden in [
    "Final status:",
    "Replay status:",
    "Exploration status:",
    "Finding count:",
    "Cleanup status:",
    "### Findings",
]:
    self.assertNotIn(forbidden, markdown)
```

- [ ] **Step 2: Add a parameterized status and summary test**

Add this test to `DingTalkTests`:

```python
def test_report_translates_every_terminal_status_and_keeps_raw_code(self):
    cases = [
        ("passed", "全部通过", "测试已执行完成，未发现需要关注的问题。"),
        ("findings_detected", "发现问题", "测试已执行完成，AI 发现 1 个需要关注的问题；当前策略只告警，不会自动回滚。"),
        ("replay_failed", "回放失败", "录制回放没有走完，请检查失败步骤。"),
        ("infrastructure_failed", "测试基础设施失败", "测试环境、浏览器、任务或证据链异常，本次结果不能用于判断产品是否正常。"),
        ("cleanup_failed", "清理失败", "测试结束，但临时账号或资源未被确认清理，请优先处理。"),
    ]
    for raw, label, summary in cases:
        with self.subTest(raw=raw):
            report = self.report(final_status=raw, finding_count=1 if raw == "findings_detected" else 0)
            self.assertEqual(report.payload()["markdown"]["title"], f"Staging 浏览器 QA：{label}")
            self.assertIn(f"最终状态：{label}（`{raw}`）", report.markdown())
            self.assertIn(f"> {summary}", report.markdown())
```

- [ ] **Step 3: Change the finding rendering test to require Chinese labels**

Use safe Chinese titles and require the original enum codes to remain visible:

```python
summaries = (
    {"severity": "high", "title": "登录入口跳转到错误页面", "confidence": "high", "page_path": "/admin"},
    {"severity": "medium", "title": "API Key 标签显示异常", "confidence": "medium", "page_path": "/keys"},
)

self.assertEqual(alert.payload()["markdown"]["title"], "Staging 浏览器 QA：发现问题")
self.assertIn("### 发现的问题", alert.markdown())
self.assertIn("[高（`high`）] 登录入口跳转到错误页面（置信度：高（`high`）；页面：/admin）", alert.markdown())
self.assertIn("[中（`medium`）] API Key 标签显示异常（置信度：中（`medium`）；页面：/keys）", alert.markdown())
self.assertEqual(passed.payload()["markdown"]["title"], "Staging 浏览器 QA：全部通过")
self.assertNotIn("### 发现的问题", passed.markdown())
self.assertEqual(failed.payload()["markdown"]["title"], "Staging 浏览器 QA：清理失败")
self.assertNotIn("### 发现的问题", failed.markdown())
```

- [ ] **Step 4: Add complete phase/severity/confidence mapping coverage**

Add a test that renders each closed enum and checks both its Chinese label and raw code:

```python
def test_report_translates_phase_severity_and_confidence_enums(self):
    for raw, label in {"passed": "通过", "failed": "失败", "not_started": "未开始", "unknown": "未知"}.items():
        with self.subTest(kind="exploration", raw=raw):
            markdown = self.report(exploration_status=raw).markdown()
            self.assertIn(f"AI 探索：{label}（`{raw}`）", markdown)

    for raw, label in {"passed": "通过", "failed": "失败", "unknown": "未知"}.items():
        with self.subTest(kind="replay", raw=raw):
            markdown = self.report(replay_status=raw).markdown()
            self.assertIn(f"录制回放：{label}（`{raw}`）", markdown)

    for raw, label in {"passed": "通过", "cleanup_failed": "清理失败", "unknown": "未知"}.items():
        with self.subTest(kind="cleanup", raw=raw):
            markdown = self.report(cleanup_status=raw).markdown()
            self.assertIn(f"账号清理：{label}（`{raw}`）", markdown)

    for severity, severity_label in {"critical": "严重", "high": "高", "medium": "中", "low": "低"}.items():
        for confidence, confidence_label in {"high": "高", "medium": "中", "low": "低"}.items():
            with self.subTest(severity=severity, confidence=confidence):
                report = self.report(
                    final_status="findings_detected",
                    finding_count=1,
                    finding_summaries=({
                        "severity": severity,
                        "title": "页面行为异常",
                        "confidence": confidence,
                        "page_path": "/settings",
                    },),
                )
                self.assertIn(
                    f"[{severity_label}（`{severity}`）] 页面行为异常（置信度：{confidence_label}（`{confidence}`）；页面：/settings）",
                    report.markdown(),
                )
```

- [ ] **Step 5: Run the DingTalk tests and verify RED**

Run:

```powershell
python -B -m unittest scripts.browser_qa.tests.test_dingtalk -v
```

Expected: FAIL only on the newly required Chinese title, summary, field labels, and enum mapping assertions; delivery, signing, validation, escaping, and redaction tests remain green.

- [ ] **Step 6: Add deterministic Chinese mappings and summaries**

Add these constants near the existing enum sets in `dingtalk.py`:

```python
FINAL_STATUS_LABELS = {
    "passed": "全部通过",
    "findings_detected": "发现问题",
    "replay_failed": "回放失败",
    "infrastructure_failed": "测试基础设施失败",
    "cleanup_failed": "清理失败",
}
PHASE_STATUS_LABELS = {
    "passed": "通过",
    "failed": "失败",
    "not_started": "未开始",
    "unknown": "未知",
    "cleanup_failed": "清理失败",
}
SEVERITY_LABELS = {"critical": "严重", "high": "高", "medium": "中", "low": "低"}
CONFIDENCE_LABELS = {"high": "高", "medium": "中", "low": "低"}
```

Replace `markdown`, `payload`, and `_terminal` with this presentation-only behavior:

```python
def markdown(self):
    lines = [
        f"### Staging 浏览器 QA：{self._final_label()}",
        f"> {self._summary()}",
        "",
        f"- 最终状态：{self._final_label()}（`{self.final_status}`）",
        f"- 录制回放：{PHASE_STATUS_LABELS[self.replay_status]}（`{self.replay_status}`）",
        f"- AI 探索：{PHASE_STATUS_LABELS[self.exploration_status]}（`{self.exploration_status}`）",
        f"- 探索动作数：`{self.exploration_actions}`",
        f"- 问题数量：`{self.finding_count}`",
        f"- 账号清理：{PHASE_STATUS_LABELS[self.cleanup_status]}（`{self.cleanup_status}`）",
        f"- 运行记录：[打开 GitHub Actions]({self.github_run_url})",
        f"- 证据文件：`{self.gcs_uri}`",
    ]
    if self.final_status == "findings_detected":
        lines.extend(["", "### 发现的问题"])
        for item in self.finding_summaries:
            title = _markdown_escape(item["title"])
            page_path = _markdown_escape(item["page_path"])
            severity = f"{SEVERITY_LABELS[item['severity']]}（`{item['severity']}`）"
            confidence = f"{CONFIDENCE_LABELS[item['confidence']]}（`{item['confidence']}`）"
            lines.append(f"- [{severity}] {title}（置信度：{confidence}；页面：{page_path}）")
    return "\n".join(lines)

def payload(self):
    return {
        "msgtype": "markdown",
        "markdown": {
            "title": f"Staging 浏览器 QA：{self._final_label()}",
            "text": self.markdown(),
        },
    }

def _final_label(self):
    return FINAL_STATUS_LABELS[self.final_status]

def _summary(self):
    if self.final_status == "passed":
        return "测试已执行完成，未发现需要关注的问题。"
    if self.final_status == "findings_detected":
        return f"测试已执行完成，AI 发现 {self.finding_count} 个需要关注的问题；当前策略只告警，不会自动回滚。"
    if self.final_status == "replay_failed":
        return "录制回放没有走完，请检查失败步骤。"
    if self.final_status == "infrastructure_failed":
        return "测试环境、浏览器、任务或证据链异常，本次结果不能用于判断产品是否正常。"
    return "测试结束，但临时账号或资源未被确认清理，请优先处理。"
```

Do not change `send_report`, signing, retry, response parsing, input validation, or sensitive-title rules.

- [ ] **Step 7: Run the DingTalk tests and verify GREEN**

Run:

```powershell
python -B -m unittest scripts.browser_qa.tests.test_dingtalk -v
```

Expected: all DingTalk tests PASS with no warnings or secret values in output.

- [ ] **Step 8: Commit the renderer slice with a Lore message**

```powershell
git add scripts/browser_qa/flatkey_browser_qa/dingtalk.py scripts/browser_qa/tests/test_dingtalk.py
git commit -m "Make Browser QA alerts readable to Chinese operators" -m "Constraint: Preserve raw enum values and the existing sanitized payload contract`nRejected: Hide raw status codes | They remain useful for incident search`nConfidence: high`nScope-risk: narrow`nDirective: Keep DingTalk delivery, signing, retry, and redaction behavior unchanged`nTested: python -B -m unittest scripts.browser_qa.tests.test_dingtalk -v`nNot-tested: Live staging delivery is verified in the final task"
```

### Task 2: Simplified Chinese finding-title prompt contract

**Files:**
- Modify: `scripts/browser_qa/tests/test_container_contract.py`
- Modify: `scripts/browser_qa/config/qa-prompt.md`

- [ ] **Step 1: Write the failing prompt-contract test**

In `test_container_includes_bounded_ai_policy_and_scenario_contract`, add:

```python
self.assertIn(
    "Write each finding `title` in concise Simplified Chinese. Keep required product names, UI labels, URLs, and HTTP status codes unchanged.",
    prompt,
)
```

- [ ] **Step 2: Run the contract test and verify RED**

Run:

```powershell
python -B -m unittest scripts.browser_qa.tests.test_container_contract.ContainerContractTests.test_container_includes_bounded_ai_policy_and_scenario_contract -v
```

Expected: FAIL because the current prompt does not contain the Simplified Chinese title instruction.

- [ ] **Step 3: Add the minimal prompt instruction**

Append this paragraph to `scripts/browser_qa/config/qa-prompt.md` after the existing cookie-free docs paragraph:

```markdown
Write each finding `title` in concise Simplified Chinese. Keep required product names, UI labels, URLs, and HTTP status codes unchanged. Do not translate or expose credentials, verification data, cookies, authorization values, query strings, or fragments.
```

- [ ] **Step 4: Run the contract test and verify GREEN**

Run:

```powershell
python -B -m unittest scripts.browser_qa.tests.test_container_contract.ContainerContractTests.test_container_includes_bounded_ai_policy_and_scenario_contract -v
```

Expected: PASS.

- [ ] **Step 5: Commit the prompt slice with a Lore message**

```powershell
git add scripts/browser_qa/config/qa-prompt.md scripts/browser_qa/tests/test_container_contract.py
git commit -m "Make AI findings readable in Chinese release alerts" -m "Constraint: Keep product names and technical identifiers exact while localizing the human summary`nRejected: Add a translation API | It would add secrets, cost, and a new failure path`nConfidence: high`nScope-risk: narrow`nDirective: Preserve the existing finding schema and sensitive-data boundaries`nTested: targeted container prompt contract test`nNot-tested: Live model compliance is verified during the staging run"
```

### Task 3: Full verification and staging acceptance

**Files:**
- Verify: `scripts/browser_qa/flatkey_browser_qa/dingtalk.py`
- Verify: `scripts/browser_qa/config/qa-prompt.md`
- Verify: `.github/workflows/gcp-browser-qa.yml`
- Verify: `.github/workflows/gcp-deploy-staging.yml`

- [ ] **Step 1: Run the full Browser QA test suite**

```powershell
python -B -m unittest discover -s scripts/browser_qa/tests -v
```

Expected: all tests PASS; only the existing platform-conditional skips remain.

- [ ] **Step 2: Lint both affected workflows**

Use the locally verified actionlint 1.7.12 binary and ignore only its unsupported `concurrency.queue` schema message:

```powershell
& "$env:TEMP\actionlint-1.7.12-browserqa\actionlint.exe" -ignore 'queue' `
  '.github\workflows\gcp-browser-qa.yml' `
  '.github\workflows\gcp-deploy-staging.yml'
```

Expected: exit code 0 and no other diagnostics. GitHub itself has already accepted `concurrency.queue` in live runs.

- [ ] **Step 3: Verify diff scope and secrets**

```powershell
git diff --check
git diff --name-only origin/staging...HEAD
git diff origin/staging...HEAD | Select-String -Pattern 'sk-[A-Za-z0-9_-]{12,}|access_token=|Authorization:\s*Bearer|[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}'
```

Expected: diff check passes; changed files are limited to the approved design/plan, DingTalk renderer/tests, and prompt/contract test; the secret scan prints no match.

- [ ] **Step 4: Render local success, alert, and failure samples**

Run a Python snippet that constructs `DingTalkReport` for `passed`, `findings_detected`, and `cleanup_failed`, then assert:

```python
assert "Staging 浏览器 QA：全部通过" in passed.markdown()
assert "测试已执行完成，AI 发现 1 个需要关注的问题" in alert.markdown()
assert "Staging 浏览器 QA：清理失败" in failed.markdown()
assert "findings_detected" in alert.markdown()
assert "### Findings" not in alert.markdown()
```

Expected: `CHINESE_DINGTALK_SAMPLES_OK`, with no webhook, email, credential, verification code, Cookie, or Authorization value printed.

- [ ] **Step 5: Re-fetch staging, integrate any new commit, and rerun verification if needed**

```powershell
git fetch --no-tags origin refs/heads/staging:refs/remotes/origin/staging
git merge-base --is-ancestor origin/staging HEAD
```

Expected: exit code 0. If staging advanced again, merge it into the feature branch with a Lore-formatted merge commit, then repeat Steps 1-4 before publishing.

- [ ] **Step 6: Push the feature branch and verify the remote SHA**

```powershell
git push origin HEAD:refs/heads/ops/browser-qa-dingtalk-notification-20260805
git ls-remote --heads origin ops/browser-qa-dingtalk-notification-20260805
```

Expected: remote feature SHA equals local `HEAD`; do not force-push.

- [ ] **Step 7: Fast-forward staging to the verified feature commit**

```powershell
git push origin HEAD:refs/heads/staging
```

Expected: a fast-forward update that triggers `GCP Deploy Staging (backend)`; do not force-push.

- [ ] **Step 8: Follow the deployment-triggered run to completion**

Find the run whose `headSha` equals the pushed commit, then verify:

- `build`: success;
- `deploy`: success;
- exactly one `browser-qa-normal / browser-qa`: success for `passed` or alert-only `findings_detected`;
- `Execute cleanup browser QA job`: success;
- `Send terminal Browser QA report to DingTalk`: success;
- no rollback job appears;
- DingTalk message title and body are Chinese-first and retain raw codes.

- [ ] **Step 9: Record final evidence without committing private artifacts**

Report the GitHub run URL, final status, replay status, exploration status/action count, finding count, cleanup status, DingTalk delivery result, commit SHA, and test count. Do not download or commit private manifest contents, screenshots, webhook values, credentials, Gmail data, verification codes, cookies, Authorization values, or API keys.
