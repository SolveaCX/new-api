# Browser QA Email Alias Length Design

## Problem and evidence

Staging run `30906966375` reached `POST /api/user/register` and received HTTP 200 with a business failure. No subsequent `/api/user/self` request occurred. A read-only query against `newapi_staging` found zero candidate user rows for that run, including soft-deleted rows, so the request failed before user insertion.

The Browser QA identity currently builds the Gmail tag as `flatkey-qa-<run-id>-<10-char-suffix>`. With the configured mailbox shape and the current 11-digit GitHub run id, the resulting alias is 57 characters. The backend `model.User.Email` validation contract is `max=50`, so registration rejects the request before email-code verification or database insertion.

## Considered approaches

1. **Shorten the Browser QA tag (selected).** Use `qa-<run-id>-<8-char-suffix>`. This preserves deterministic run correlation and a seed-derived suffix while reducing the current alias to 47 characters. The change stays inside QA identity generation, broker validation, tests, and QA documentation.
2. **Increase the backend email limit.** This would require a product/schema decision and cross-database migration for production merely to accommodate a staging test identity. The scope and deployment risk are disproportionate.
3. **Dynamically truncate the tag from the Gmail base address.** This would make the identity API depend on mailbox configuration and introduce variable formats across supervisor, broker, and cleanup. It is unnecessary for the configured mailbox and harder to audit.

## Design

`derive_identity` will emit `qa-<run-id>-<8 lowercase-alphanumeric characters>`. The run id remains verbatim, so broker requests and cleanup remain attributable to one GitHub Actions run. The suffix remains HMAC-derived from the identity seed.

The broker service, broker MCP client, and Gmail reader will accept only that exact format. Strict validation remains fail-closed: wrong run ids, uppercase suffixes, malformed tags, or arbitrary aliases are rejected.

No production API, database schema, Terraform resource, secret, Gmail OAuth scope, cleanup authorization, or report-redaction boundary changes.

## Data flow

1. The supervisor derives one deterministic identity from the secret seed and run id.
2. It constructs the Gmail alias from the configured base address plus the shortened tag.
3. The staging console requests a verification email and submits the returned code with the same alias.
4. The broker validates the shortened tag, finds only mail addressed to that exact alias after the run start, and returns the code through the existing protected channel.
5. Registration now passes the backend 50-character email validation and proceeds to the existing replay checkpoint.
6. The independent cleanup job derives the same shortened identity and removes only that run's disposable resources.

## Testing and acceptance

- A regression test must fail on the old format by proving that the configured mailbox shape plus an 11-digit run id exceeds the 50-character backend contract.
- Identity, broker, broker MCP, Gmail parsing, cleanup, supervisor, workflow-contract, and full Browser QA tests must pass after the format change.
- A fresh staging deployment must complete with replay passed, replay checkpoint reached, cleanup passed, and the GitHub workflow successful.
- Database verification remains read-only and reports only counts or non-sensitive status fields.

## Operational compatibility

Existing failed-run artifacts and cleanup inputs remain tied to the image that created them. The new deployment consistently uses the shortened format for both main and cleanup jobs. No main/production deployment is authorized or required for this staging-only tooling correction.
