# Browser QA Verification-Wait Tunnel Design

**Status:** Approved for staging-only implementation under the existing Browser QA acceptance scope.

## Problem

GitHub run `30901319843` deployed staging successfully and started the Browser QA core job, but replay ended as `replay_failed` while cleanup passed. Sanitized browser evidence shows that verification-code retrieval completed, then the final `POST /api/user/register` failed inside Chromium with `net::ERR_SSL_PROTOCOL_ERROR`. The request never reached the staging application.

The Browser QA loopback egress proxy currently uses one five-second `timeout` value for two different responsibilities:

- bounded DNS/connect/socket setup; and
- idle waiting inside an established HTTPS `CONNECT` tunnel.

Waiting for and entering an email verification code legitimately leaves the established TLS tunnel idle for more than five seconds. The proxy therefore closes the tunnel before Chromium submits registration.

## Scope and success criteria

This change is limited to the staging Browser QA runtime. It must:

1. Keep the existing short transport timeout for connection establishment and ordinary bounded I/O.
2. Allow an established allowlisted HTTPS tunnel to remain idle across the verification-code wait.
3. Preserve the existing hostname policy, private-address rejection, byte budgets, non-root runtime, replay budget, cleanup, and evidence redaction.
4. Add a regression test that fails with the former shared-timeout behavior and passes with the separated tunnel timeout.
5. Pass the complete Browser QA test suite and a fresh staging replay with cleanup.

It does not change the recorded onboarding Skill, registration UI/API, Gmail broker, Terraform, production runtime, or exploration behavior.

## Considered approaches

### 1. Separate transport and tunnel-idle timeouts (selected)

Keep the existing five-second transport timeout and give established `CONNECT` tunnels a separate 300-second idle timeout. This fixes the faulty boundary directly while retaining fast failure for DNS and connection problems. Five minutes covers the expected verification wait and remains below the job-level execution limit.

### 2. Increase the existing timeout globally

Rejected because it would also make failed DNS, dial, and ordinary proxy operations wait much longer. It preserves the category error between connection setup and established-tunnel lifetime.

### 3. Retry registration after `ERR_SSL_PROTOCOL_ERROR`

Rejected because it treats the browser symptom rather than the proxy cause and risks duplicate or ambiguous form submission. A healthy local proxy must not require application-level retries for an intentionally idle TLS tunnel.

## Design

`EgressProxy` owns two explicit timing concepts:

- `timeout`: the existing short transport timeout used when opening upstream sockets and performing bounded HTTP work;
- `tunnel_idle_timeout`: a 300-second limit used only by `_ProxyHandler._tunnel` while waiting for activity on an already established bidirectional `CONNECT` tunnel.

The forwarding loop continues to enforce independent client-to-upstream and upstream-to-client byte counts. EOF, byte-budget exhaustion, or the dedicated idle limit closes both sides as before. No host, credential, payload, or browser-policy rule changes.

Data flow after the change:

```text
Chromium -> loopback proxy -> CONNECT allowlist/DNS validation -> staging TLS tunnel
                                                        |
                                                        +-> short timeout only for setup
Established tunnel -> wait for Gmail broker/code entry -> final registration request
                                                        |
                                                        +-> 300-second idle allowance
```

## Test strategy

The regression test creates a local echo server and an `EgressProxy` with a 0.1-second transport timeout. It establishes a `CONNECT` tunnel, waits 0.25 seconds, then sends data. With the former implementation, the proxy closes the tunnel and the test receives no bytes. With the separated tunnel timeout, the echo succeeds.

Verification layers:

1. Focused regression test with explicit RED/GREEN evidence.
2. Full `test_egress_proxy` module.
3. Complete `scripts/browser_qa/tests` suite.
4. Browser evidence Node helper tests.
5. Push to `staging`, then require a new workflow run to report replay passed/checkpoint reached and cleanup passed.

## Risks and rollback

The longer idle lifetime can keep an allowlisted tunnel open longer, but existing host validation, loopback-only listener, per-direction byte caps, overall job timeout, and process cleanup still bound exposure. Rollback is the single Browser QA commit; no infrastructure or application rollback is required.
