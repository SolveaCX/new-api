# Browser QA DingTalk Notification Design

## Goal

Every staging Browser QA replay must send one terminal DingTalk report, whether the replay passes, finds a defect, fails, or its cleanup/infrastructure path fails. The same behavior must apply to a manual dispatch and to the reusable workflow called after a staging deployment.

## Chosen approach

The notification belongs inside `.github/workflows/gcp-browser-qa.yml`, after the sanitized root manifest is summarized and before the final status gate. A small standard-library Python module owns validation, message rendering, signing, retry, and DingTalk response checking. The workflow passes only a fixed set of sanitized status fields through step-level environment variables and reads the webhook and robot signing secret only from GitHub Actions secrets.

This is preferred over notifying only in `gcp-deploy-staging.yml`, because a caller-only notification would miss manual Browser QA runs. It is also preferred over embedding a large Python program in YAML, because isolated unit tests can prove retry and redaction behavior.

## Data flow

1. Browser QA validates inputs, builds the runner image, executes the main Cloud Run Job, and always attempts cleanup.
2. The workflow fetches only `runs/<run-id>/manifest.json`, validates its schema, and exports safe outputs: final status, replay status, exploration status/actions, finding count, cleanup status, and the private GCS URI.
3. An `if: always()` notification step invokes `python3 -m scripts.browser_qa.flatkey_browser_qa.dingtalk`.
4. The module validates every status/count/URL against a closed contract, renders a Markdown report, signs each request with DingTalk's millisecond timestamp plus HMAC-SHA256 scheme, and POSTs it to DingTalk. It retries transient network/HTTP/server failures with bounded backoff and requires a JSON response whose `errcode` is exactly zero.
5. The existing final gate fails actionable QA states. A notification delivery failure also leaves the workflow failed, so a green run always means both QA and reporting completed.

## Secret flow

The repository secrets are named `STAGING_BROWSER_QA_DINGTALK_WEBHOOK` and `STAGING_BROWSER_QA_DINGTALK_SIGNING_SECRET`. The reusable workflow declares both secrets, and `gcp-deploy-staging.yml` passes them explicitly. The manual `workflow_dispatch` path reads the same repository secrets. Both values are injected only into the notification step's environment; neither is placed in command-line arguments, job outputs, summaries, manifests, committed files, or messages.

For bootstrap, the existing GCP Secret Manager values `week-one-qa-dingtalk-webhook` and `week-one-qa-dingtalk-signing-secret` may be piped directly into `gh secret set` without displaying them.

## Report contract

The message contains only:

- final status;
- replay status;
- exploration status and action count;
- finding count;
- cleanup status;
- GitHub Actions run URL;
- private GCS manifest URI.

It never contains a Gmail address or alias, password, verification code, API key, cookie, authorization header, identity seed, DingTalk webhook, signing secret, or generated signature.

## Failure behavior

- Missing or invalid manifest: report `infrastructure_failed` or `cleanup_failed` using safe fallback values.
- Replay or finding failure: notify the exact trusted manifest status, then fail the final gate.
- Cleanup failure: cleanup status has priority, notify `cleanup_failed`, then fail.
- DingTalk nonzero `errcode`, malformed response, or exhausted retries: fail the notification step; the final gate still runs because it uses `if: always()`.
- Missing webhook or signing secret: fail closed with a generic error that does not reveal secret material.

## Verification

Tests must first fail against the current workflow, then prove:

- the reusable secret declaration and staging caller pass-through exist;
- the notification is `if: always()` and precedes the final gate;
- the secret appears only in the approved step-level environment expression;
- safe manifest outputs exist for success and fallback paths;
- the Python sender renders only the fixed safe fields, generates the documented timestamp/HMAC signature, retries transient failures, rejects nonzero `errcode`, and never includes the webhook, signing secret, or signature in its message or exception text.

After local tests pass, push the exact commit to `staging`, follow the resulting deployment-triggered core replay to a terminal state, verify its private GCS root manifest, and confirm DingTalk accepted the terminal report.
