# Browser QA Verification-Wait Tunnel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent the staging Browser QA HTTPS tunnel from closing while the replay waits for and enters an email verification code.

**Architecture:** Keep the existing short transport timeout for upstream setup and add a separate five-minute idle timeout for established `CONNECT` tunnels. Preserve all existing allowlist and byte-budget enforcement, then prove the boundary with a local echo-server regression test and a fresh staging replay.

**Tech Stack:** Python 3 standard library, `unittest`, socketserver, GitHub Actions, Cloud Run Browser QA runtime.

---

## File map

- `scripts/browser_qa/tests/test_egress_proxy.py` — reproduces a `CONNECT` tunnel surviving a verification-code-sized idle interval.
- `scripts/browser_qa/flatkey_browser_qa/egress_proxy.py` — separates transport setup timeout from established-tunnel idle timeout.
- `docs/superpowers/specs/2026-08-04-browser-qa-verification-wait-tunnel-design.md` — records root cause, selected design, risks, and acceptance criteria.

### Task 1: Lock the regression with a failing tunnel-lifetime test

**Files:**

- Modify: `scripts/browser_qa/tests/test_egress_proxy.py`
- Test: `scripts/browser_qa/tests/test_egress_proxy.py`

- [ ] **Step 1: Add the regression test**

Add this test to `EgressPolicyTests`:

```python
def test_connect_tunnel_outlives_transport_timeout_while_waiting_for_signup_code(self):
    class EchoHandler(socketserver.BaseRequestHandler):
        def handle(self):
            while True:
                data = self.request.recv(1024)
                if not data:
                    return
                self.request.sendall(data)

    upstream = socketserver.TCPServer(("127.0.0.1", 0), EchoHandler)
    thread = threading.Thread(target=upstream.serve_forever, daemon=True)
    thread.start()
    port = upstream.server_address[1]
    proxy = egress_proxy.EgressProxy(
        policy=egress_proxy.EgressPolicy({"staging-console.flatkey.ai"}),
        resolver=lambda host, resolved_port, *_: [
            (socket.AF_INET, socket.SOCK_STREAM, 6, "", ("93.184.216.34", resolved_port))
        ],
        connector=lambda _resolved, timeout: socket.create_connection(("127.0.0.1", port), timeout=timeout),
        timeout=0.1,
    )
    proxy.start()
    try:
        with socket.create_connection((proxy.host, proxy.port), timeout=2) as sock:
            sock.settimeout(2)
            sock.sendall(
                b"CONNECT staging-console.flatkey.ai:443 HTTP/1.1\r\n"
                b"Host: staging-console.flatkey.ai\r\n\r\n"
            )
            response, _data = _read_connect_response(sock)
            self.assertIn(b"200", response)

            threading.Event().wait(0.25)
            sock.sendall(b"registration")

            try:
                echoed = sock.recv(len(b"registration"))
            except OSError:
                echoed = b""
            self.assertEqual(echoed, b"registration")
    finally:
        proxy.stop()
        upstream.shutdown()
        upstream.server_close()
```

- [ ] **Step 2: Run the focused test against the former shared-timeout behavior**

Run:

```powershell
python -B -m unittest scripts.browser_qa.tests.test_egress_proxy.EgressPolicyTests.test_connect_tunnel_outlives_transport_timeout_while_waiting_for_signup_code -v
```

Expected RED: `AssertionError: b'' != b'registration'` because the tunnel closes after the 0.1-second transport timeout.

### Task 2: Separate tunnel-idle timeout from transport timeout

**Files:**

- Modify: `scripts/browser_qa/flatkey_browser_qa/egress_proxy.py`
- Test: `scripts/browser_qa/tests/test_egress_proxy.py`

- [ ] **Step 1: Add the dedicated timeout and use it only in the tunnel loop**

Add the module constant:

```python
DEFAULT_TUNNEL_IDLE_TIMEOUT_SECONDS = 300
```

Initialize the separate property without changing the existing `timeout` argument:

```python
self.tunnel_idle_timeout = DEFAULT_TUNNEL_IDLE_TIMEOUT_SECONDS
self.timeout = timeout
```

Use the new property in `_ProxyHandler._tunnel`:

```python
readable, _, _ = select.select(sockets, [], [], self.proxy.tunnel_idle_timeout)
```

- [ ] **Step 2: Run the focused GREEN test**

Run the same focused command from Task 1.

Expected GREEN: one test passes and the echo after the idle interval equals `b"registration"`.

- [ ] **Step 3: Run the proxy module tests**

```powershell
python -B -m unittest scripts.browser_qa.tests.test_egress_proxy -v
```

Expected: 13 tests pass.

### Task 3: Verify, review, submit, and run staging acceptance

**Files:**

- Modify only files required by review findings.

- [ ] **Step 1: Run the full Browser QA verification suite**

```powershell
python -B -m unittest discover -s scripts/browser_qa/tests -p 'test_*.py'
node --test scripts/browser_qa/flatkey_browser_qa/browser_evidence_helper.test.cjs
git diff --check
```

Expected: Python reports 355 tests passing with four intentional skips; Node reports 22 passing; diff check reports no errors.

- [ ] **Step 2: Review the exact change against the design**

Review only the committed design plus the egress proxy and regression-test diff. Require confirmation that:

- the five-second transport timeout still governs connection setup;
- only established `CONNECT` idle waiting uses 300 seconds;
- allowlist and byte limits are unchanged;
- the regression test fails under the former behavior and passes under the new behavior;
- no production, Terraform, application registration, or credential surface changed.

Fix every Critical or Important finding, then rerun Step 1.

- [ ] **Step 3: Commit the implementation using the repository Lore protocol**

```powershell
git add -- scripts/browser_qa/flatkey_browser_qa/egress_proxy.py scripts/browser_qa/tests/test_egress_proxy.py
git add -f -- docs/superpowers/plans/2026-08-04-browser-qa-verification-wait-tunnel.md
git commit -m "Keep Browser QA TLS alive while verification mail arrives" `
  -m "Constraint: Preserve short dial failures, the egress allowlist, byte budgets, and staging-only scope." `
  -m "Rejected: Increase the shared timeout | It would slow unrelated network failures and preserve the faulty coupling." `
  -m "Confidence: high" `
  -m "Scope-risk: narrow" `
  -m "Directive: Treat connection setup and established tunnel idleness as separate timeout domains." `
  -m "Tested: Focused RED/GREEN regression, Browser QA Python suite, Node evidence helper, and diff check." `
  -m "Not-tested: Live staging replay is verified after the push."
```

- [ ] **Step 4: Push the reviewed commit to the staging deployment branch**

```powershell
git push origin HEAD:staging
```

Expected: the push advances only `origin/staging` to the reviewed commits and triggers the staging deployment plus Browser QA core workflow.

- [ ] **Step 5: Monitor the new GitHub Actions run to its terminal state**

Use `gh run list` to identify the run for the new staging SHA, then `gh run watch --exit-status` and inspect the job summary.

Acceptance requires:

- staging build and deploy pass;
- Browser QA replay status is `passed`;
- replay checkpoint is reached before any optional exploration;
- cleanup status is `passed`;
- workflow conclusion is `success`;
- no production or Terraform workflow is triggered by this change.
