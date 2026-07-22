# Token Batch Group Rollout

`TOKEN_BATCH_GROUP_ENABLED` controls the authenticated batch token-group endpoint. It defaults to `false` so every Go process sharing Redis can adopt the cache protocol before users can invoke the endpoint.

Roll out in this order:

1. Deploy every Redis-sharing Go service and revision with `TOKEN_BATCH_GROUP_ENABLED=false`. This includes router, console, and any legacy Go service connected to the same Redis database.
2. Drain old revisions and their in-flight goroutines.
3. Wait for each drained process's pending in-memory token quota batches to flush to the database.
4. Only after the quota batches have flushed, purge existing `token:*` hashes or wait one full token-cache TTL. Do not purge while an old process can still flush a pending quota batch.
5. Enable `TOKEN_BATCH_GROUP_ENABLED=true` on every relevant Go service, including the console service that exposes the endpoint. The console reads the same runtime capability from `/api/status`, so the embedded batch action remains hidden while the flag is off and appears after the status cache refreshes.

The quota batch store remains process-local. A safe rollout therefore depends on draining every old revision and allowing its pending batch to flush before cache cleanup or endpoint enablement.
