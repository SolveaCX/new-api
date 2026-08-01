# GCP Operations Guide

Read this **before** touching anything under `deploy/gcp/`. Captures the parts of operating the GCP infrastructure that aren't obvious from the Terraform code, especially landmines that have already bitten this project.

This is the AI-facing operations playbook. Architecture inventory is in `INFRASTRUCTURE.md`; deploy/rollback procedures are in `DEPLOYMENT.md`.

---

## Project / environment

- **GCP project**: `vocai-gemini-prod` (project number `528088078482`)
- **Region**: `us-west1` (Oregon)
- **Terraform working directory**: `deploy/gcp/envs/prod/`
- **Terraform state**: GCS backend, bucket `vocai-gemini-prod-newapi-tfstate`, prefix `envs/prod` (versioning enabled — recoverable)
- **Sole approver / human-in-loop**: `slZhong` (manual deploy gate in GitHub Actions)

---

## Auth

Two separate token systems — they expire independently and you need both:

| Purpose | Command | Used by |
|---|---|---|
| Application Default Credentials (ADC) | `gcloud auth application-default login` | Terraform, REST API via `Authorization: Bearer $(gcloud auth application-default print-access-token)` |
| User CLI session | `gcloud auth login` | `gcloud compute ...`, `gcloud run ...`, `gcloud sql ...` |

`gcloud auth application-default print-access-token` succeeding is **not** enough to prove `gcloud compute X` will work — those use the user-CLI token. If you see "Reauthentication failed. cannot prompt during non-interactive execution", the user-CLI token expired; ask the user to run `gcloud auth login`.

**Network gotcha**: the user's local network sometimes can't reach specific `*.googleapis.com` endpoints (notably `cloudresourcemanager.googleapis.com`) — symptoms are `EOF` on `terraform plan/apply` or `Recv failure: Connection reset by peer` on curl. Test with:

```bash
curl -sS --connect-timeout 5 -o /dev/null -w "%{http_code}\n" \
  https://cloudresourcemanager.googleapis.com/v1/projects
```

`000` (timeout / RST) → network blocked, ask user to switch network/VPN before retrying. `401` → network works (auth header missing is fine for the probe).

---

## Resource ownership: who writes what

This is the single most important thing to know before running `terraform apply`. Several Cloud Run fields are written by **CI/CD at deploy time**, not Terraform. Terraform must `ignore_changes` on them or every plan will fight CI/CD and try to revert.

**Ignored on `google_cloud_run_v2_service`** (see `modules/cloud-run/main.tf`):

| Field | Owner | Why |
|---|---|---|
| `template[0].containers[0].image` | CI/CD | New image per deploy |
| `template[0].revision` | CI/CD | Pinned revision name per deploy |
| `client`, `client_version` | gcloud | Set by `gcloud run` writes |
| `scaling` (top-level block) | gcloud | Populated with zero values by `gcloud run services update` — harmless drift |
| `traffic` | CI/CD | Canary blue/green with revision-pinned tags; the LATEST block in TF is only for first bring-up |
| `template[0].containers[0].env` | gcp-deploy.yml + gcloud | PADDLE_*, GA_*, BATCH_UPDATER_RESET etc. exist only on the live service; the TF env blocks are bring-up defaults. Added to ignore_changes 2026-06-12 after a plain plan tried to strip the live payment config |
| `template[0].vpc_access` | gcloud | egress flipped to `ALL_TRAFFIC` out-of-band (fixed-IP egress); TF code still says `PRIVATE_RANGES_ONLY` |

If a plan ever shows a diff on these fields, **do not apply**. Either the ignore list got broken, or CI/CD's state was lost — investigate, don't bulldoze.

**Env vars are out-of-band owned (in `ignore_changes` since 2026-06-12).** The env blocks in the TF module only seed a brand-new service; the live service's env is written by `gcp-deploy.yml` (PADDLE_*) and ad-hoc `gcloud run services update` (GA_*, secret refs, ...). Terraform neither strips nor updates env anymore. To change env on the live service (CI/CD's pinned revision names cause HTTP 409 on TF-driven updates anyway):

```bash
# Update env vars directly (gcloud auto-creates a new revision name)
gcloud run services update newapi \
  --region=us-west1 \
  --update-env-vars=KEY1=value1,KEY2=value2

# Then sync TF state without making changes
terraform apply -refresh-only
```

---

## Usage reconciliation token (`BLOCKRUN_USAGE_SUMMARY_TOKEN`) — already set up, keep it on

The BlockRun usage reconciliation endpoints — `GET /usage/summary` and `GET /usage/transactions` (code: `controller/usage_reconciliation.go`, `router/usage_reconciliation.go`, auth in `middleware/usage_recon_auth.go`; design spec `docs/superpowers/specs/2026-06-08-blockrun-usage-reconciliation-design.md`) — authenticate with a single static Bearer token read from env `BLOCKRUN_USAGE_SUMMARY_TOKEN`. Same value goes to the external reconciliation consumer.

**State (as of 2026-06-08):**

- Secret Manager secret `newapi-blockrun-usage-summary-token` exists (Terraform-owned: `google_secret_manager_secret.blockrun_usage_summary_token` in `envs/prod/main.tf`), value set (version 1), runtime SA `newapi-runtime@vocai-gemini-prod.iam.gserviceaccount.com` granted `roles/secretmanager.secretAccessor`.
- The env was **pre-injected on the live service via gcloud** (`gcloud run services update newapi --update-secrets=BLOCKRUN_USAGE_SUMMARY_TOKEN=newapi-blockrun-usage-summary-token:latest --no-traffic`), creating revision `newapi-00051-v4v` at **0% traffic** (serving revision and the `canary` tag were left untouched). So `spec.template` already carries the secret env, and every later CI image deploy inherits it — `gcp-deploy.yml` uses `--update-env-vars` (a delta) + `--image`, which preserves existing env/secrets rather than replacing them.
- `enable_usage_recon_token = true` in `envs/prod/terraform.tfvars` gates the `dynamic "env"` block in `modules/cloud-run/main.tf`.

**Don't break it:**

- **Keep `enable_usage_recon_token = true`.** It keeps desired-state honest and seeds fresh bring-ups. (Env is in `ignore_changes` since 2026-06-12, so `terraform apply` no longer strips live env either way — but don't rely on that to paper over a wrong flag.)
- The env was set out-of-band via gcloud, so TF state can lag reality. The committed flag keeps desired-state aligned, so a refreshing `terraform plan` shows no env diff; run `terraform apply -refresh-only` to sync state exactly.
- When writing the secret value, use `printf '%s'`, not `echo` (no trailing newline in the token).

**Rotate the token** (single shared secret — the reconciliation consumer must change in lockstep):

```bash
printf '%s' '<new-token>' | gcloud secrets versions add newapi-blockrun-usage-summary-token \
  --project=vocai-gemini-prod --data-file=-
gcloud run services update newapi --region=us-west1 --project=vocai-gemini-prod \
  --update-secrets=BLOCKRUN_USAGE_SUMMARY_TOKEN=newapi-blockrun-usage-summary-token:latest
# then shift traffic to the new revision — see the revision-pinned traffic section above
```

First-time setup runbook: `DEPLOYMENT.md` → "用量对账 token（`BLOCKRUN_USAGE_SUMMARY_TOKEN`）".

---

## `gcp-infra.yml` apply currently does not work (IAM gap)

**Symptom**: `workflow_dispatch` on `gcp-infra.yml` fails at the very first `terraform apply` step with errors like:

```
Error 403: Permission denied to list services for consumer container [projects/528088078482]
reason: AUTH_PERMISSION_DENIED on serviceusage.googleapis.com
  with module.apis.google_project_service.this["serviceusage.googleapis.com"]
```

**Cause**: the CI service account `newapi-ci-deployer@vocai-gemini-prod.iam.gserviceaccount.com` only has the three minimum roles needed for **app deploy** (`run.developer`, `artifactregistry.writer`, `iam.serviceAccountUser`). `terraform apply` does a full state refresh that reads every module's GCP state — needing read perms across serviceusage, IAM, secretmanager, compute, cloudsql, redis, monitoring, etc. Until those are granted, **infra apply via CI will never succeed**.

**Update 2026-07-09: pull-request plans intentionally use `terraform plan -refresh=false`.** This avoids the deployer SA's refresh-time `AUTH_PERMISSION_DENIED` on serviceusage and other prod resources, but it only checks the desired Terraform diff against the current state file. **Treat CI plan comments as non-authoritative for live drift** — always run a refreshing `terraform plan` locally with Owner ADC before applying.

**Workaround (works today, no Terraform drift)**: when the Terraform code on `main` is already merged with the desired state, just apply via `gcloud` using a user account with Owner / `roles/run.admin`. Terraform's `desired` and reality will reconverge — no drift, no refresh-only needed.

Worked example (2026-05-25 scaling tune, PR #22):

```bash
gcloud run services update newapi \
  --region=us-west1 \
  --project=vocai-gemini-prod \
  --min-instances=4 \
  --concurrency=50

# Then redirect traffic to the new revision — see next section.
```

**Long-term fix** (separate PR): grant the deployer SA the full set of read + write roles in `modules/service-accounts/main.tf`. Minimum starter list:

```
roles/serviceusage.serviceUsageAdmin
roles/iam.securityReviewer            # read IAM policies across resources
roles/secretmanager.viewer
roles/compute.viewer                  # network/LB
roles/cloudsql.viewer
roles/redis.viewer
roles/monitoring.viewer
roles/iam.workloadIdentityPoolViewer
roles/artifactregistry.reader
# plus admin roles per module for the write side: secretmanager.admin, cloudsql.admin, redis.admin, compute.networkAdmin, compute.loadBalancerAdmin, monitoring.admin
```

This is a meaningful blast radius (broad cross-resource admin) — review carefully and consider splitting into a separate `infra-deployer` SA instead of upgrading the existing `ci-deployer`.

---

## Cloud Run traffic is revision-pinned — gcloud-only scaling tweaks don't auto-receive traffic

When CI/CD deploys a new image, the workflow pins traffic to that specific revision name (the LATEST block in Terraform is only for first bring-up). After such a deploy, `spec.traffic[*].latestRevision == false` and traffic = 100% on the explicit revision name.

**Consequence**: if you then run `gcloud run services update newapi --min-instances=X --concurrency=Y` to tweak scaling, gcloud creates a **brand-new revision** (with auto-generated name like `newapi-00021-zxs`) carrying the new scaling. But **traffic stays on the previously-pinned revision**, which keeps the old scaling values. You'll see `spec.template.containerConcurrency = 50` in `services describe` (that's the next revision's template), but in reality 100% of traffic is still served by the old revision at conc=80.

To make the new scaling take effect immediately:

```bash
gcloud run services update-traffic newapi \
  --region=us-west1 \
  --project=vocai-gemini-prod \
  --to-revisions=newapi-00021-zxs=100
```

Verify with:

```bash
gcloud run services describe newapi --region=us-west1 --project=vocai-gemini-prod \
  --format='value(status.traffic)'
# Want: status.traffic[0].revisionName=newapi-00021-zxs, percent=100
```

After traffic flips, the next CI/CD app deploy still works normally — it creates yet another revision (with commit-hash suffix), inherits the *current* `spec.template` (so new scaling carries over), and re-pins traffic to itself.

To roll back to the prior revision quickly:

```bash
gcloud run services update-traffic newapi --region=us-west1 --project=vocai-gemini-prod \
  --to-revisions=<previous-revision-name>=100
```

The same rule applies to the split production services:

| Service | Role | Normal traffic owner |
|---|---|---|
| `newapi-console` | console/admin/API, `NODE_TYPE=master` | `console.flatkey.ai` |
| `newapi-router` | model relay/API, `NODE_TYPE=slave` | `router.flatkey.ai` |
| `newapi-web` | public website | `flatkey.ai`, `www.flatkey.ai` |
| `newapi-console` | fallback for unmatched hosts | URL map default backend |

When rolling back or shifting traffic, target the specific service that serves the failing host. The legacy `newapi` service is decommissioned (`enable_legacy_runtime=false`) and is not a deployment or rollback target. Restoring it requires a separately approved infrastructure recovery plan; do not treat it as an ordinary revision rollback.

---

## Production runtime split

Current production routing is host-based at the GCP LB:

| Host | Backend service | Cloud Run service | Runtime role |
|---|---|---|---|
| `flatkey.ai`, `www.flatkey.ai` | `newapi-web-backend` | `newapi-web` | Next.js website |
| `console.flatkey.ai` | `newapi-console-backend` | `newapi-console` | Go app, `NODE_TYPE=master`, `APP_ROLE=console` |
| `router.flatkey.ai` | `newapi-router-backend` | `newapi-router` | Go app, `NODE_TYPE=slave`, `APP_ROLE=router` |
| default | `newapi-console-backend` | `newapi-console` | fallback for unmatched hosts |

Verify the live URL map before and after any host-split change:

```bash
gcloud compute url-maps describe newapi-urlmap \
  --project=vocai-gemini-prod --global \
  --format='yaml(hostRules,pathMatchers,defaultService)'
```

Rollback levers:

- Bad console revision: `gcloud run services update-traffic newapi-console ... --to-revisions=<old>=100`
- Bad router revision: `gcloud run services update-traffic newapi-router ... --to-revisions=<old>=100`
- Bad website revision: `gcloud run services update-traffic newapi-web ... --to-revisions=<old>=100`
- Bad console host split: set `console_domains = []`, plan, review URL map diff, apply
- Bad router host split: set `router_domains = []`, plan, review URL map diff, apply
- Bad website host split: set `website_domains = []`, plan, review URL map diff, apply

Host-rule rollback sends new requests to the URL map default backend (`newapi-console-backend`). It does not stop in-flight requests on the previous Cloud Run revision, but it can change application behavior if the fallback service has different image/env. Check logs before choosing host-rule rollback over revision rollback.

---

## HTTPS LB cert rotation has a downtime window

The managed SSL cert is recreated whenever `lb_domains` changes (via `random_id.cert_suffix` keepers). With `create_before_destroy`, Terraform creates the new cert and points the HTTPS proxy at it **before** destroying the old one. That sounds safe but isn't:

- The new cert is in `PROVISIONING` immediately after creation
- The HTTPS proxy now references only the new cert (old one detached)
- Until Google verifies all listed domains' DNS and signs the cert (10–30 min), the LB has no usable cert
- **All HTTPS traffic to all domains in `lb_domains` fails during that window** (TLS handshake errors like `SSL_ERROR_SYSCALL`)

Always warn the user before applying a `lb_domains` change. Schedule it during low-traffic windows.

**Check cert status without gcloud CLI** (works with just ADC):

```bash
TOKEN=$(gcloud auth application-default print-access-token)
curl -sS -H "Authorization: Bearer $TOKEN" \
  "https://compute.googleapis.com/compute/v1/projects/vocai-gemini-prod/global/sslCertificates?filter=name+eq+.*newapi-cert.*" \
  | python3 -c "import sys,json; d=json.load(sys.stdin); [print(c['name'],'|',c['managed']['status'],'|',c['managed'].get('domainStatus',{})) for c in d.get('items',[])]"
```

`domainStatus` showing all `ACTIVE` but `status: PROVISIONING` means cert is about to flip — typically a few more minutes. Outright `FAILED_NOT_VISIBLE` means DNS isn't pointing at the LB IP yet — fix DNS, then bump the `cert_rotation` Terraform variable to force-recreate the cert.

---

## Cloudflare DNS mode

Current production uses a deliberate mixed Cloudflare mode:

| Host | Mode | Why |
|---|---|---|
| `flatkey.ai`, `www.flatkey.ai` | Proxied | Public website, covered by Cloudflare Universal SSL |
| `console.flatkey.ai` | Proxied | Console is intentionally kept out of GCP `lb_domains` to avoid managed-cert rotation |
| `router.flatkey.ai` | DNS only | Covered by GCP managed cert; avoids proxy behavior on long-lived model calls |
| `new-api.app.flatkey.ai`, `new-api.api.flatkey.ai` | DNS only | Depth-3 names are not covered by Cloudflare Universal SSL |
| `one.flatkey.ai` | DNS only | Compatibility entry; URL map default routes to `newapi-console` |

Cloudflare's "origin IP partially exposed" warning is expected because the same GCP LB IP has both Proxied and DNS-only records.

Do not flip DNS modes casually:

- Switching depth-3 names to Proxied fails unless Cloudflare Total TLS / ACM is enabled.
- Switching `console.flatkey.ai` to DNS-only would require adding it to GCP `lb_domains`, which triggers managed cert rotation and a possible HTTPS downtime window.
- Switching `router.flatkey.ai` to Proxied is technically possible at depth 2, but should be tested for streaming/model-call behavior first.

To use Proxied for depth-3 names would require Total TLS ($10/mo) — previously declined per cost.

---

## Flatkey staging browser QA first-run and recovery runbook

This runbook is for the isolated staging browser QA surface managed by `deploy/gcp/envs/prod/browser_qa.tf` and `.github/workflows/gcp-browser-qa.yml`. It is separate from the production deploy path.

Operator rules:

- Treat every command in this section as an example until the operator has reviewed the current branch, account, project, and target resource names.
- Do not run these commands from an agent session. The human operator runs them from an authenticated terminal.
- Keep `set -x` disabled. Do not paste secret values into argv, Terraform variables, Terraform state, GitHub output, GitHub summaries, committed files, or shell history.
- Use stdin for secret values. Use `printf`, `read -s`, and pipes; do not use `echo` for secrets.
- Keep temporary files under `mktemp -d`, remove them on exit, and never commit Terraform plans, JSON plan exports, OAuth material, or report downloads.
- Do not manually run `gcloud run jobs execute ... --update-env-vars=FLATKEY_QA_GMAIL_BASE=...`; the workflow owns runtime injection from the repository variable.

### Resource names to verify before touching anything

These names are the implementation contract as of this Terraform root:

| Purpose | Name |
|---|---|
| Terraform root | `deploy/gcp/envs/prod/` |
| Project | `vocai-gemini-prod` |
| Region | `us-west1` |
| Browser QA toggle | `enable_browser_qa = true` |
| Artifact Registry repository | `flatkey-staging-browser-qa` |
| Broker Cloud Run service | `flatkey-staging-browser-qa-broker` |
| Main Cloud Run job | `flatkey-staging-browser-qa` |
| Cleanup Cloud Run job | `flatkey-staging-browser-qa-cleanup` |
| Codex API key secret | `flatkey-browser-qa-codex-api-key` |
| Identity seed secret | `flatkey-browser-qa-identity-seed` |
| Gmail OAuth secret | `flatkey-browser-qa-gmail-oauth` |
| Report bucket | `vocai-gemini-prod-flatkey-browser-qa-reports` |
| Runtime service accounts | `flatkey-browser-qa-runtime`, `flatkey-browser-qa-broker`, `flatkey-browser-qa-cleanup`, `flatkey-browser-qa-deployer` |
| GitHub workflow | `.github/workflows/gcp-browser-qa.yml` |
| Workflow modes | `core`, `normal`, `cleanup-only` |
| Non-committed GitHub variable | `GCP_BROWSER_QA_GMAIL_BASE` |

### 1. Authenticated refreshing Terraform plan review

Do not trust PR plan comments for this apply. They use `-refresh=false` and cannot prove live drift is absent. Use Owner-capable ADC locally and save the exact plan that will be applied.

```bash
# Example review commands. Non-mutating until the final terraform apply.
set -euo pipefail
set +x

cd deploy/gcp/envs/prod
gcloud auth application-default login
terraform init

review_dir="$(mktemp -d)"
trap 'rm -rf "$review_dir"' EXIT
plan_path="$review_dir/browser-qa.tfplan"
plan_json="$review_dir/browser-qa.tfplan.json"

terraform plan -out="$plan_path"
terraform show -json "$plan_path" > "$plan_json"
terraform show -no-color "$plan_path" > "$review_dir/browser-qa.tfplan.txt"

python3 - "$plan_json" <<'PY'
import json
import sys

plan = json.load(open(sys.argv[1], encoding="utf-8"))
changes = []
for change in plan.get("resource_changes", []):
    actions = change.get("change", {}).get("actions", [])
    if actions and actions != ["no-op"]:
        changes.append((change.get("address", ""), actions))

if not changes:
    raise SystemExit("ABORT: saved plan has no resource changes; do not apply")

bad = [(address, actions) for address, actions in changes if "browser_qa" not in address]
if bad:
    print("ABORT: non-browser-QA resource changes found:")
    for address, actions in bad:
        print(f"  {address}: {actions}")
    raise SystemExit(1)

forbidden = (
    "module.cloud_run",
    "module.cloud_lb",
    "google_compute_",
    "google_dns_",
    "cloudflare_",
    "newapi-web",
    "newapi-console",
    "newapi-router",
    "newapi-urlmap",
    "ssl_certificate",
    "certificate",
    "traffic",
)
hits = [(address, actions) for address, actions in changes if any(term in address for term in forbidden)]
if hits:
    print("ABORT: existing service/LB/cert/DNS/traffic diff found:")
    for address, actions in hits:
        print(f"  {address}: {actions}")
    raise SystemExit(1)

outputs = plan.get("output_changes", {})
bad_outputs = [name for name, change in outputs.items() if change.get("actions") != ["no-op"] and not name.startswith("browser_qa_")]
if bad_outputs:
    print("ABORT: unrelated output changes found:", ", ".join(sorted(bad_outputs)))
    raise SystemExit(1)

print("OK: saved plan is limited to dedicated browser-QA addresses.")
for address, actions in changes:
    print(f"  {address}: {actions}")
PY

printf '\nReview the human-readable plan before applying:\n  %s\n' "$review_dir/browser-qa.tfplan.txt"
printf 'If approved, apply before this shell exits so the exact saved plan still exists.\n'
IFS= read -r -p "Type APPLY_BROWSER_QA_SAVED_PLAN to apply this exact saved plan: " APPLY_CONFIRM
if [ "$APPLY_CONFIRM" = "APPLY_BROWSER_QA_SAVED_PLAN" ]; then
  terraform apply "$plan_path"
else
  printf 'Skipped apply; temp plan will be removed by the EXIT trap.\n'
fi
```

Abort immediately if the saved plan contains any existing Cloud Run service, load balancer, URL map, certificate, DNS, traffic, or unrelated IAM/resource diff. A plan is applyable only when every resource change address is dedicated browser-QA Terraform, such as `google_cloud_run_v2_job.browser_qa_main[0]` or `google_secret_manager_secret.browser_qa_gmail_oauth[0]`.

Apply only from the same review shell before the `EXIT` trap removes `plan_path`. Never replace this with an unsaved `terraform apply`, `-refresh=false`, or `-target`.

### 2. Add Secret Manager versions without leaking values

Terraform creates only the Secret Manager containers. Secret versions are operator-owned.

Generate a random 32-byte identity seed as base64, validate that it decodes to exactly 32 bytes, and only then launch `gcloud` with the payload on stdin:

```bash
# Mutating; operator review required. No secret value appears in argv.
set -euo pipefail
set +x

python3 - <<'PY'
import base64
import os
import subprocess

payload = base64.b64encode(os.urandom(32))
decoded = base64.b64decode(payload, validate=True)
if len(decoded) != 32:
    raise SystemExit("identity seed must decode to exactly 32 bytes")

subprocess.run(
    [
        "gcloud",
        "secrets",
        "versions",
        "add",
        "flatkey-browser-qa-identity-seed",
        "--project=vocai-gemini-prod",
        "--data-file=-",
    ],
    input=payload,
    check=True,
)
PY
```

Add the dedicated Codex API key through non-echoing stdin:

```bash
# Mutating; operator review required. The key is read silently and never placed in argv.
set -euo pipefail
set +x

python3 - <<'PY'
import getpass
import subprocess

secret = getpass.getpass("Dedicated Codex API key: ")
if not secret or not secret.strip():
    raise SystemExit("Codex API key must be non-empty")
payload = secret.encode("utf-8")

subprocess.run(
    [
        "gcloud",
        "secrets",
        "versions",
        "add",
        "flatkey-browser-qa-codex-api-key",
        "--project=vocai-gemini-prod",
        "--data-file=-",
    ],
    input=payload,
    check=True,
)
PY
```

Transform the existing local post-consent Gmail OAuth credential JSON in memory and pass only the canonical broker payload to Secret Manager. Do not copy the source file, do not print the transformed JSON, and do not commit either file.

```bash
# Mutating; operator review required. Set GMAIL_OAUTH_SOURCE to the local credential file path.
set -euo pipefail
set +x

IFS= read -rsp "Local Gmail OAuth credential JSON path: " GMAIL_OAUTH_SOURCE
printf '\n'
export GMAIL_OAUTH_SOURCE

python3 - <<'PY'
import json
import os
import subprocess

GMAIL_READONLY = "https://www.googleapis.com/auth/gmail.readonly"
GOOGLE_TOKEN_URI = "https://oauth2.googleapis.com/token"

source = os.environ["GMAIL_OAUTH_SOURCE"]
if not source.strip():
    raise SystemExit("GMAIL_OAUTH_SOURCE must be non-empty")
with open(source, encoding="utf-8") as handle:
    raw = json.load(handle)
if not isinstance(raw, dict):
    raise SystemExit("OAuth credential JSON must be an object")

client = raw.get("installed") or raw.get("web") or raw
if not isinstance(client, dict):
    raise SystemExit("OAuth client fields must be an object")

missing = []
if not isinstance(raw.get("refresh_token"), str) or not raw["refresh_token"].strip():
    missing.append("refresh_token")
for name in ("client_id", "client_secret"):
    if not isinstance(client.get(name), str) or not client[name].strip():
        missing.append(name)
if missing:
    raise SystemExit("OAuth credential JSON missing required post-consent field(s): " + ", ".join(sorted(missing)))

token_uri = client.get("token_uri") or raw.get("token_uri")
if token_uri != GOOGLE_TOKEN_URI:
    raise SystemExit("OAuth token_uri must be exactly https://oauth2.googleapis.com/token")

if "scopes" in raw:
    scopes = raw["scopes"]
elif "scope" in raw:
    scopes = raw["scope"]
else:
    raise SystemExit("OAuth credential JSON must explicitly contain scope or scopes")
if isinstance(scopes, str):
    scopes = scopes.split()
elif isinstance(scopes, list) and all(isinstance(item, str) for item in scopes):
    scopes = list(scopes)
else:
    raise SystemExit("OAuth scopes must be a string or list of strings")
if scopes != [GMAIL_READONLY]:
    raise SystemExit("OAuth scopes must be exactly the singleton gmail.readonly scope")

payload = {
    "refresh_token": raw["refresh_token"],
    "token_uri": token_uri,
    "client_id": client["client_id"],
    "client_secret": client["client_secret"],
    "scopes": scopes,
}
payload_bytes = json.dumps(payload, separators=(",", ":")).encode("utf-8")

subprocess.run(
    [
        "gcloud",
        "secrets",
        "versions",
        "add",
        "flatkey-browser-qa-gmail-oauth",
        "--project=vocai-gemini-prod",
        "--data-file=-",
    ],
    input=payload_bytes,
    check=True,
)
PY

unset GMAIL_OAUTH_SOURCE
```

The originally downloaded Google OAuth client-secret JSON is insufficient by itself: it normally contains `installed` or `web` client metadata, but no `refresh_token` and no granted `scopes`. The source for the transform must be a post-consent credential JSON containing `refresh_token` plus explicit `scope` or `scopes`. If it does not, use the Google OAuth app's existing client to perform a one-time local authorization outside the repo with `access_type=offline`, `prompt=consent`, and exactly `https://www.googleapis.com/auth/gmail.readonly`; save the resulting credential JSON only in an operator-owned `0600` local path, then rerun the transform above. Do not use a broad Gmail scope, do not use a production mailbox other than the approved base mailbox, and do not copy the credential JSON into the repository.

Reviewer-only self-check for the rewritten snippets: each snippet constructs and validates `payload` or `payload_bytes` before `subprocess.run(...)`; every `raise SystemExit(...)` path occurs before the `gcloud` process is launched, so parse/validation failure cannot create an empty or malformed Secret Manager version. Static syntax validation can be done locally without invoking `gcloud` by extracting the Python heredocs from this file and running `ast.parse` on each snippet.

### 3. Set the GitHub repository variable

`GCP_BROWSER_QA_GMAIL_BASE` is a repository variable, not a committed file and not a Terraform variable. It must be the base Gmail address only, with no plus tag, comma, CR, or LF. Use the GitHub repository UI if possible so the value does not enter CLI argv or shell history.

If policy requires CLI, do not put the value in the command line. Use a locked temporary file and remove it immediately after `gh` reads it:

```bash
# Mutating; operator review required. Prefer the GitHub UI if your gh version cannot read variables from stdin.
set -euo pipefail
set +x

tmp_var="$(mktemp)"
chmod 600 "$tmp_var"
trap 'rm -f "$tmp_var"' EXIT
IFS= read -rsp "Base Gmail address without plus tag: " GCP_BROWSER_QA_GMAIL_BASE
printf '\n'
printf '%s' "$GCP_BROWSER_QA_GMAIL_BASE" > "$tmp_var"
unset GCP_BROWSER_QA_GMAIL_BASE

gh variable set GCP_BROWSER_QA_GMAIL_BASE \
  --repo SolveaCX/new-api \
  < "$tmp_var"
```

Abort if the installed `gh variable set` cannot read from stdin; use the GitHub UI instead. Do not fall back to `--body "$GCP_BROWSER_QA_GMAIL_BASE"`.

### 4. OAuth publication, bootstrap, and repeatability rule

A token issued while the OAuth app Publishing status is `Testing` proves bootstrap only. External Testing refresh tokens can expire quickly, so one successful run with a Testing token is not repeatability evidence.

Repeatability is accepted only after all three steps are complete:

1. Publish the OAuth app as `In production`.
2. Reauthorize exactly `gmail.readonly`.
3. Rotate `flatkey-browser-qa-gmail-oauth` with the new refresh token and complete a second successful full `normal` run.

### 5. Verify broker IAM denial

The broker must deny unauthenticated calls and calls from an explicitly reviewed identity that is not `flatkey-browser-qa-runtime`. Use the Terraform output for the broker URI; do not hardcode the generated `run.app` URL.

```bash
# Read-only verification example.
set -euo pipefail
set +x

cd deploy/gcp/envs/prod
BROKER_URI="$(terraform output -raw browser_qa_broker_uri)"
NEGATIVE_PROBE_SA="flatkey-browser-qa-cleanup@vocai-gemini-prod.iam.gserviceaccount.com"

case "$NEGATIVE_PROBE_SA" in
  flatkey-browser-qa-runtime@vocai-gemini-prod.iam.gserviceaccount.com|\
  flatkey-browser-qa-broker@vocai-gemini-prod.iam.gserviceaccount.com|\
  flatkey-browser-qa-deployer@vocai-gemini-prod.iam.gserviceaccount.com)
    echo "negative probe must not be runtime, broker, or deployer" >&2
    exit 1
    ;;
esac

echo "Before using this negative-control SA, review org/project/service IAM and confirm it has no roles/run.invoker on ${BROKER_URI}."
echo "The operator must have impersonation authority, such as roles/iam.serviceAccountTokenCreator, on ${NEGATIVE_PROBE_SA}."

service_invoker_binding="$(gcloud run services get-iam-policy flatkey-staging-browser-qa-broker \
  --project=vocai-gemini-prod \
  --region=us-west1 \
  --flatten='bindings[].members' \
  --filter="bindings.role=roles/run.invoker AND bindings.members=serviceAccount:${NEGATIVE_PROBE_SA}" \
  --format='value(bindings.role)')"
test -z "$service_invoker_binding"

project_invoker_binding="$(gcloud projects get-iam-policy vocai-gemini-prod \
  --flatten='bindings[].members' \
  --filter="bindings.role=roles/run.invoker AND bindings.members=serviceAccount:${NEGATIVE_PROBE_SA}" \
  --format='value(bindings.role)')"
test -z "$project_invoker_binding"

ORG_ID="<gcp-org-id>"
if [ "$ORG_ID" = "<gcp-org-id>" ]; then
  echo "Set ORG_ID and check organization IAM for roles/run.invoker before continuing." >&2
  exit 1
fi
org_invoker_binding="$(gcloud organizations get-iam-policy "$ORG_ID" \
  --flatten='bindings[].members' \
  --filter="bindings.role=roles/run.invoker AND bindings.members=serviceAccount:${NEGATIVE_PROBE_SA}" \
  --format='value(bindings.role)')"
test -z "$org_invoker_binding"

status="$(curl -sS -o /dev/null -w '%{http_code}' \
  -X POST \
  -H 'Content-Type: application/json' \
  --data '{"run_id":"0","email_tag":"flatkey-qa-0-0000000000","start_time":0}' \
  "${BROKER_URI}/v1/current-code")"
test "$status" = "401" -o "$status" = "403"

token_file="$(mktemp)"
header_file="$(mktemp)"
chmod 600 "$token_file" "$header_file"
trap 'rm -f "$token_file" "$header_file"' EXIT
gcloud auth print-identity-token \
  --audiences="$BROKER_URI" \
  --impersonate-service-account="$NEGATIVE_PROBE_SA" \
  > "$token_file"
{ printf 'Authorization: Bearer '; cat "$token_file"; printf '\n'; } > "$header_file"
status="$(curl -sS -o /dev/null -w '%{http_code}' \
  -X POST \
  -H 'Content-Type: application/json' \
  -H @"$header_file" \
  --data '{"run_id":"0","email_tag":"flatkey-qa-0-0000000000","start_time":0}' \
  "${BROKER_URI}/v1/current-code")"
test "$status" = "401" -o "$status" = "403"
```

Abort if the negative-control identity is the operator Owner, runtime, broker, or deployer identity; if IAM review shows it has `roles/run.invoker` at org/project/service scope; or if the operator lacks impersonation authority for that service account. Abort if either request reaches application-level validation, returns `200`, or returns a broker JSON error such as `invalid_fields`. That means IAM is not enforcing the private broker boundary.

### 6. Dispatch core or normal and capture the exact run id

Run `core` first. It exercises the onboarding replay and stops before the five-minute exploration phase. Run `normal` only after `core` finishes and cleanup succeeds; `normal` performs core replay plus the bounded exploration phase, capped by the implementation at five minutes or thirty browser actions.

Use this helper for both modes. It captures the UTC dispatch timestamp and exact `staging` head SHA before dispatch, then polls `workflow_dispatch` runs by `databaseId`, `createdAt`, `headSha`, `status`, and `url`. It prints `ORIGINAL_GITHUB_RUN_ID` only when exactly one post-dispatch run matches the captured SHA.

```bash
# Mutating; operator review required. Starts a GitHub Actions run on the staging ref.
set -euo pipefail
set +x

dispatch_browser_qa() {
  mode="$1"
  case "$mode" in
    core|normal) ;;
    *) echo "mode must be core or normal" >&2; return 2 ;;
  esac

  DISPATCHED_AT_UTC="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  STAGING_HEAD_SHA="$(gh api repos/SolveaCX/new-api/git/ref/heads/staging --jq '.object.sha')"

  gh workflow run gcp-browser-qa.yml \
    --repo SolveaCX/new-api \
    --ref staging \
    -f mode="$mode"

  python3 - "$DISPATCHED_AT_UTC" "$STAGING_HEAD_SHA" <<'PY'
import datetime as dt
import json
import subprocess
import sys
import time

created_after = dt.datetime.fromisoformat(sys.argv[1].replace("Z", "+00:00"))
head_sha = sys.argv[2]
deadline = time.time() + 120

while True:
    raw = subprocess.check_output(
        [
            "gh",
            "run",
            "list",
            "--repo",
            "SolveaCX/new-api",
            "--workflow",
            "gcp-browser-qa.yml",
            "--branch",
            "staging",
            "--event",
            "workflow_dispatch",
            "--limit",
            "20",
            "--json",
            "databaseId,createdAt,headSha,status,url",
        ],
        text=True,
    )
    runs = json.loads(raw)
    matches = []
    for run in runs:
        created = dt.datetime.fromisoformat(run["createdAt"].replace("Z", "+00:00"))
        if created >= created_after and run["headSha"] == head_sha:
            matches.append(run)
    if len(matches) == 1:
        run = matches[0]
        print(f"ORIGINAL_GITHUB_RUN_ID={run['databaseId']}")
        print(f"RUN_STATUS={run['status']}")
        print(f"RUN_URL={run['url']}")
        break
    if len(matches) > 1:
        print("ABORT: multiple workflow_dispatch runs match the dispatch timestamp and staging SHA", file=sys.stderr)
        for run in matches:
            print(f"  {run['databaseId']} {run['createdAt']} {run['headSha']} {run['status']} {run['url']}", file=sys.stderr)
        raise SystemExit(1)
    if time.time() > deadline:
        raise SystemExit("ABORT: no matching workflow_dispatch run found; use GitHub UI for manual disambiguation")
    time.sleep(5)
PY
}

# First run:
dispatch_browser_qa core

# After core passes and cleanup succeeds, run:
# dispatch_browser_qa normal
```

Record `ORIGINAL_GITHUB_RUN_ID` from `databaseId` for the specific `core` or `normal` run. Do not use a workflow attempt number. If the matcher finds zero or multiple runs, abort and disambiguate manually in GitHub Actions before any cleanup-only dispatch.

The GitHub summary must show only status, replay status, exploration status/actions, finding count, cleanup status, and the private GCS URI. Abort and redact the run if a secret, full Gmail address, full plus alias, verification code, password, Cookie, Authorization header, or full API key appears in the summary.

### 7. Private GCS report lookup

Use the GitHub Summary `gcs_uri`, or derive the manifest path from the original GitHub run id:

```bash
# Read-only report lookup example.
set -euo pipefail
set +x

GITHUB_RUN_ID="<original-github-run-id>"
GCP_BROWSER_QA_GCS_BUCKET="$(terraform -chdir=deploy/gcp/envs/prod output -raw browser_qa_report_bucket)"
report_dir="$(mktemp -d)"
trap 'rm -rf "$report_dir"' EXIT

gcloud storage cp \
  "gs://${GCP_BROWSER_QA_GCS_BUCKET}/runs/${GITHUB_RUN_ID}/manifest.json" \
  "$report_dir/manifest.json" \
  --quiet

python3 -m json.tool "$report_dir/manifest.json"
```

Report objects are private and expire by bucket lifecycle after 14 days. Do not upload report downloads to issues, PR comments, tickets, or chat unless they have been manually redacted again.

### 8. Cleanup-only with the original GitHub run id

Use `cleanup-only` when the main run was cancelled, the platform interrupted the workflow before cleanup completed, or cleanup needs to be retried. Always use the original GitHub run id for the run that created the staging identity.

```bash
# Mutating; operator review required. Recomputes the same QA identity from the original run id.
ORIGINAL_GITHUB_RUN_ID="<original-github-run-id>"

gh workflow run gcp-browser-qa.yml \
  --repo SolveaCX/new-api \
  --ref staging \
  -f mode=cleanup-only \
  -f original_run_id="$ORIGINAL_GITHUB_RUN_ID"
```

Abort if `original_run_id` is unknown. Do not substitute the cleanup workflow's new run id; that would derive a different identity and leave the original account unproven.

### 9. `invalid_grant` recovery

`gmail_invalid_grant` from the broker is an infrastructure failure, not a retryable app failure.

Recovery:

1. Stop rerunning `normal`; repeated runs will consume cleanup capacity and produce the same failure.
2. Publish the OAuth app as `In production` if it is still `Testing`.
3. Reauthorize the OAuth client for exactly `https://www.googleapis.com/auth/gmail.readonly`.
4. Rotate `flatkey-browser-qa-gmail-oauth` using the in-memory transform above.
5. Run `core`.
6. If `core` passes and cleanup succeeds, run `normal`.

Abort if the new OAuth grant requires a broader Gmail scope, if the base Gmail profile is not the expected base mailbox, or if `gmail_invalid_grant` repeats after publication and secret rotation.

### 10. Gmail plus-alias restriction failure

Browser QA requires staging to accept Gmail plus aliases generated as `+flatkey-qa-<run-id>-<nonce>`. If the workflow fails before account creation with an alias restriction error, classify it as staging configuration failure.

Recovery:

1. Verify staging has `EmailAliasRestrictionEnabled=false`.
2. Do not switch to a non-plus personal mailbox, disposable domain, or committed static test address.
3. Run `cleanup-only` with the original run id if the report or logs show that account creation may have started.
4. Rerun `core` after the staging setting is fixed.

Abort if staging policy intentionally forbids plus aliases; the current QA design depends on deterministic plus-alias derivation and must be redesigned before use.

### Recovery and abort rules

| Condition | Required action |
|---|---|
| Terraform plan contains any non-`browser_qa` resource change | Abort; do not apply. |
| Plan touches existing Cloud Run services, LB, URL map, cert, DNS, traffic, or unrelated IAM | Abort; investigate drift or lifecycle ownership. |
| Secret command would put a secret in argv, history, Terraform state, GitHub output, or a committed file | Abort; use stdin/UI/in-memory transform instead. |
| Broker unauthenticated or bad-identity call returns `200` or broker JSON validation | Abort; IAM boundary failed. |
| `core` fails and cleanup succeeds | Triage replay failure from private report; do not run `normal` yet. |
| `normal` returns `findings_detected` and cleanup succeeds | Treat as actionable QA finding; production deploy path remains unchanged. |
| Any run returns `cleanup_failed` | Run `cleanup-only` with the original GitHub run id; manually inspect staging if it repeats. |
| Manifest is missing and cleanup failed or skipped | Run `cleanup-only` with the original GitHub run id before further testing. |
| Manifest is missing but cleanup succeeded | Classify as `infrastructure_failed`; inspect GCS/report upload path. |
| OAuth app is `Testing` and the run passed | Count it as bootstrap only; publish In production, rotate, and rerun before claiming repeatability. |
| `gmail_invalid_grant` | Publish/reauthorize/rotate; do not blind-retry. |
| Gmail plus-alias restriction | Fix staging alias policy or stop; do not change identity strategy ad hoc. |
| Unknown original run id for cleanup | Stop and recover the run id from GitHub Actions before attempting cleanup. |

---

## CI/CD constraints

- Workflow: `.github/workflows/deploy.yml` (GitHub Actions), uses Workload Identity Federation — no static keys.
- Push to `main` triggers `build` automatically.
- `deploy` job is gated by a `production` Environment with `slZhong` as the sole required reviewer.
- Don't bypass the approval gate. Don't merge to main without an approved PR (the auto-mode classifier will block direct merges).

---

## Common destructive actions — confirm first

- Any change to `lb_domains` (causes HTTPS downtime window — see above)
- `terraform destroy` on any module (obviously)
- Bumping `cert_rotation` while a cert is currently ACTIVE (causes new downtime window unnecessarily)
- Removing or changing `router_domains`, `console_domains`, or `website_domains` (changes live host routing)
- Changing Cloudflare proxy mode for `console.flatkey.ai` or `router.flatkey.ai`
- Editing `ServerAddress` admin setting (breaks OAuth callbacks, video proxy URLs, email reset links until rolled to all instances)
- Setting `enable_usage_recon_token = false` or removing the `BLOCKRUN_USAGE_SUMMARY_TOKEN` secret env (breaks the `/usage` reconciliation endpoints → 503; see the section above)

---

## Whitelabel channels (kuaizi etc.)

Some video channels run through the whitelabel pipeline — customer-facing responses must hide upstream provider identity. The registry is in `relay/channel/task/taskcommon/helpers.go::whitelabelChannels`. When adding a new whitelabel channel:

1. Add the channel type constant to that map
2. Add a case in `controller/video_proxy.go::VideoProxy` to resolve the real upstream URL from `task.Data` (see kuaizi case for the pattern)
3. Optionally provide an `ExtractUpstream...VideoURL` helper inside the channel adaptor package

The `ScrubBrandedText` helper has a keyword list — extend `brandKeywords` if the new provider's name leaks through error strings.

---

## When in doubt

1. **Plan before apply, always.** Save with `terraform plan -out=newapi.plan` and inspect.
2. **Targeted applies don't help here** — `cloud_lb` references `cloud_run` so `-target=module.cloud_lb` will still pull cloud_run changes. Fix the lifecycle config instead.
3. **State is recoverable** — GCS versioning is on. If state corrupts, list versions: `gsutil ls -a gs://vocai-gemini-prod-newapi-tfstate/envs/prod/default.tfstate`.
