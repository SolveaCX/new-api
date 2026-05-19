# Follow-up TODOs

Outstanding work tracked from past planning / review cycles. Each item links back to the PR or plan that surfaced it.

---

## Redis pubsub config sync — follow-ups

Source: PR #16 (merged 2026-05-15), plan at `docs/superpowers/plans/2026-05-15-redis-pubsub-config-sync.md`.

The feature shipped with full unit-test coverage + production verification. These items were intentionally deferred from the PR scope.

### Functional gaps

- [ ] **`controller/console_migrate.go:100` direct option delete bypasses publish.** `model.DB.Where("key IN ?", oldKeys).Delete(&model.Option{})` deletes rows without going through `model.UpdateOption`, so peers don't get a pubsub notification. One-shot admin migration → 60s polling fallback covers, but worth wiring properly.
  - Fix: add `common.PublishConfigChanged(ctx, common.ConfigScopeOptions)` after the delete, or refactor to go through `UpdateOption`.

- [ ] **`UpdateChannelStatus` multi-key partial-state edge case bypasses both DB save and publish.** Pre-existing bug not introduced by PR #16: when `IsMultiKey` is true and the aggregate `channel.Status` is unchanged but `MultiKeyStatusList` changed, the `if channel.Status == status { return false }` short-circuit (around `model/channel.go:734`) skips both `SaveWithoutKey` and `publishChannelsChanged`. Peers stay out of sync until the 60s tick.
  - Fix: in the multi-key branch (`model/channel.go:738-747`), detect whether `handlerMultiKeyUpdate` mutated `MultiKeyStatusList` and force a save+publish even when the aggregate status didn't move.

### Robustness / latency

- [ ] **Wrap `PublishConfigChanged` calls in a 500ms timeout context.** Both `model/option.go:227` and `model/channel.go:27` use `context.Background()`. If Redis is reachable-but-slow, the admin save endpoint stalls until the go-redis default timeout. Low blast radius (admin-only), but worth tightening.
  - Fix: `ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond); defer cancel(); common.PublishConfigChanged(ctx, ...)`
  - Apply to both call sites consistently.

- [ ] **Pre-existing `DB.FirstOrCreate` / `DB.Save` error swallowing in `model.UpdateOption`** (`model/option.go:215, 217`). Save errors are silently ignored — a successful-looking admin save can fail while peers correctly stay on the old value. Made more visible by pubsub (peers now reload faster and notice divergence sooner).
  - Fix: check `.Error` and return it from `UpdateOption`.

### Test hygiene

- [ ] **Tighten UUID parse assertion in `common/replica_id_test.go::TestReplicaID_LooksLikeUUID`.** Currently checks `len(id) == 36`; would accept any 36-char string. Use `uuid.Parse(id)` and assert `parsed.Version() == 4`.

- [ ] **Replace `time.Sleep(100 * time.Millisecond)` in pubsub tests with `sub.Receive` synchronization.** `common/redis_pubsub_test.go:89, 120`. The sleep races against go-redis SUBSCRIBE registration; flaky on slow CI. Use the `sub.Receive(ctx)` ack pattern already demonstrated at line 35.

- [ ] **Add concurrent-safety test for `GetReplicaID`.** `sync.Once` is correct today, but no test exercises concurrent first-call. A regression that swaps `sync.Once` for a plain bool flag would not be caught.
  - Fix: add `TestReplicaID_ConcurrentSafe` firing N goroutines and asserting all return the same value.

### Cosmetics

- [ ] **Switch `encoding/json` in `common/redis_pubsub.go` to package-local `Marshal` / `Unmarshal` wrappers.** The file is in `common/` where the wrappers themselves live, so direct usage is technically permitted — but using the wrappers would keep CLAUDE.md Rule 1 audits clean by grep without false positives.

- [ ] **Doc comment on `SubscribeConfigChanged` about handler latency.** Handler runs synchronously inside the receive loop. go-redis subscriber buffer is 100 messages with a 60s send timeout — a slow handler could drop messages on burst.
  - Fix: one-line godoc: `// Handler is invoked inline on the receive goroutine; it must complete quickly to avoid dropping messages during bursts.`

- [ ] **Clean-shutdown pattern for self-filter test.** `common/redis_pubsub_test.go::TestSubscribeConfigChanged_FiltersSelfMessages` races `defer cancel()` against `cleanup()`, producing a benign `redis: discarding bad PubSub connection` log on teardown. Cosmetic; refactor to explicit `<-done` ordering before cleanup.

---

## batchUpdate deadlock fix + graceful shutdown — follow-ups

Source: 2026-05-19 PR（修 batchUpdate goroutine 死锁 + 加优雅上下线，分支 `worktree-fix-batch-update-deadlock`）。

线上 2026-05-18 09:04 UTC batchUpdate goroutine 卡在某条 MySQL UPDATE 上 30+ 小时不返回，~$277 漏扣。这次 PR 在 driver 层加 read/write timeout + flusher panic recover + 优雅上下线，让 bug 不再发生且重启时不丢内存数据。但有几条限制 / 边界条件留作后续：

### 长流式响应在重启时被 SIGKILL 中断

- [ ] **长流式响应（Claude 大输出 30-60s）在重启时会被 Cloud Run 第 10 秒强杀，客户端收到 connection reset。**
  Cloud Run 默认 termination grace period 是 10s 且 v2 API 暂不支持配置（Google 平台硬限制）。我们用了 8s HTTP shutdown + 1s wait flusher + ms 级 sync flush，留 ~1s 给 defer model.CloseDB()。短请求完全无感，但长流式没办法在 8s 内自然结束。
  - **方案 A（推荐，工程成本最低）**：让所有调用方客户端实现 connection-reset retry。Anthropic / OpenAI 官方 SDK 自带；自定义业务 client 需要补（典型 5-10 行）。
  - **方案 B**：streaming-aware shutdown — 应用启动时建一个 `streamRootCtx`，所有 stream handler 监听它的 `Done()`，shutdown 时先 `cancelStreams()` + 短 sleep 让 stream 收尾再 `srv.Shutdown`。需要改所有 stream handler（每条写循环加 select-case），1 天工作量。Reviewer 之前提过 `srv.RegisterOnShutdown` 不适用此场景（per-conn 钩子），用 ctx propagation 是对的路径。
  - **方案 C**：换 GKE 自管，可配 `terminationGracePeriodSeconds: 300` + `preStop` hook。月费 + 运维复杂度上升，不推荐。

### Stream 中断协作通道（方案 B 的载体）

- [ ] **`request ctx` 没有 propagation 到 upstream call**。当前 stream handler 拿 gin.Context 但不一定把 `r.Context().Done()` 透传到上游 HTTP client。即使有了 streamRootCtx，stream handler 不监听也没用。
  - Fix：在 `relay/relay_adaptor.go` / `service/` 各 stream loop 里加 `select { case <-ctx.Done(): return; default: }`，或者让所有 stream copy 用 `ctx`-aware `io.Copy`-类。
  - 这是方案 B 的前置条件。

### 测试 hygiene 微调

- [ ] **`gin.DefaultErrorWriter` 在 `model/dsn_defaults_test.go:92` 和 `model/batch_update_test.go:152` 的 capture 不 `t.Parallel()` 安全。** testify 默认串行所以现在不会出问题，但有人未来加 `t.Parallel()` 会引入 race。
  - Fix：加 `// not Parallel-safe — captures global gin.DefaultErrorWriter` 注释；或者用 `LogWriterMu.Lock()` 包裹 swap。

- [ ] **`lifecycle.Graceful` 当 server shutdown 失败时无可观测信号。**`pkg/lifecycle/lifecycle.go:57-59` 只 SysError 打日志，调用方拿不到 bool/error 区分 "clean shutdown" vs "forceful shutdown"。
  - Fix：让 Graceful 返回 `error`，main.go 据此打不同日志（便于事后排查）。

### 数据补偿（如需要）

- [ ] **2026-05-18 09:04 UTC ~ 2026-05-19 08:37 UTC 漏扣约 $277（17572 条 consume log）**。需要从 logs 表反推回填 tokens / users / channels 三张表。看板数据本身基于 logs 表无影响（用户已确认）。是否补取决于"账面 vs 实际"容忍度。
  - 已准备好 dry-run + UPDATE SQL（按 token_id / user_id / channel_id 三维度聚合），见 PR 讨论历史。
  - 补偿前必须先重启服务（已完成），否则内存里堆积的扣费会继续干扰窗口边界。

---

## How to use this file

- Pick an item, file a PR, mark it `- [x]` in the same PR.
- If something stays here for >3 months, either schedule it or delete it (dead TODOs are noise).
- New items should follow the same shape: title in bold + the source PR/plan + the concrete fix.
