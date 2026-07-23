# Supplier accounting daily batch operations

The canonical architecture, accounting rules, storage model, and rollout boundaries are documented in [Upstream supply chain and profit accounting V1](./supplier-supply-chain-v1.md).

Production release requires an external scheduler or operator job. This repository does not create that scheduler. Store its credentials in the platform secret manager and rotate them under the normal root-credential policy.

- After 02:00 Asia/Shanghai, send `POST /api/supply-chain/daily-batches/catch-up` to the console application with credentials accepted by `RootAuth`. Use either `Authorization: <root management access token>` or an equivalent valid RootAuth session, and in both cases send `New-Api-User: <root user id>`. The endpoint is protected by the critical rate limiter.
- Only `HTTP 200` with response JSON `success: true` is a successful continuation result. Its `data` contains `processed_days`, `remaining_work`, and `next_batch_date`, and each call processes at most one accounting day.
- Repeat the request while `data.remaining_work` is `true`, respecting rate limits. Stop when it is `false`; an empty `data.next_batch_date` means no accounting day is currently eligible.
- A request before the 02:00 close grace performs no work and returns `data.processed_days: 0`, `data.remaining_work: false`, and an empty `data.next_batch_date`.
- Treat `HTTP 200` with `success: false` as an authentication or business failure. Alert and retry according to policy; do not advance any scheduler checkpoint.
- HTTP 409 with `data.status: "busy"` means another lease owner is processing the day. Back off with jitter and retry; do not start a competing local worker.
- HTTP 429 means the critical rate limiter rejected the call. Honor the configured retry/backoff policy before calling again.
- HTTP 5xx means the batch did not complete successfully. Record the failure, alert after the scheduler's retry threshold, and retry with bounded exponential backoff. Never advance the scheduler checkpoint on a 5xx response.
- Retries are safe because lease ownership, fence generations, and page-level cursor CAS are stored in the database. After a failed run, a new fence recomputes the accounting day from its beginning; the scheduler must not assume cross-failure page resume.

This operational prerequisite requires external scheduler configuration only. Do not run a Terraform apply or deploy additional application code merely to create the schedule.
