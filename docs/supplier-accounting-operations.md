# Supplier accounting daily batch operations

Production release requires an external scheduler or operator job. This repository does not create that scheduler.

- After 02:00 Asia/Shanghai, send `POST /api/supply-chain/daily-batches/catch-up` to the console application with credentials accepted by `RootAuth`. The endpoint is also protected by the critical rate limiter.
- Each successful request processes at most one accounting day and returns `processed_days`, `remaining_work`, and `next_batch_date`.
- Repeat the request while `remaining_work` is `true`, respecting rate limits. Stop when it is `false`; an empty `next_batch_date` means no accounting day is currently eligible.
- A request before the 02:00 close grace performs no work and returns `processed_days: 0`, `remaining_work: false`, and an empty `next_batch_date`.

This operational prerequisite requires external scheduler configuration only. Do not run a Terraform apply or deploy additional application code merely to create the schedule.
