# 上游供应链与利润核算 V1

本文档是上游供应链 V1 的设计与实现口径。日常调度和故障处理见[供应商日结运维手册](./supplier-accounting-operations.md)。

## 1. 目标与边界

V1 用于管理“供应商—合同—渠道”的采购关系，并回答以下业务问题：

- 某渠道按官方模型原价消耗了多少库存；
- 按请求发生时的采购折扣，应计采购成本是多少；
- 对外最终结算的销售额是多少；
- 毛利润和毛利率是多少；
- 内部账号消耗了多少官方原价库存，但不把内部消费当作收入或利润。

所有模型类型统一以系统在请求发生时使用的官方模型原价配置为基准，包括 Claude、GPT、Gemini、DeepSeek 等。每份合同同一时刻只使用一个统一采购折扣；折扣变更通过新版本生效，历史请求继续保留请求发生时的版本和金额。

V1 采用“请求成功结算时冻结事实，T+1 汇总并展示”的轻量方案：

```mermaid
flowchart LR
    A["供应商、合同、采购折扣、库存、排除账号"] --> B["渠道选择时冻结配置版本"]
    B --> C["最终成功结算"]
    C --> D["写入 logs.other.supplier_accounting_v1"]
    D --> E["北京时间 T+1 日汇总"]
    E --> F["供应链报表与库存展示"]
```

该方案不引入供应链 Redis Stream、publisher/consumer、常驻 Worker 或实时汇总。请求热路径读取进程内不可变原子快照，不增加请求级数据库或 Redis I/O。

## 2. 核算口径

### 2.1 金额公式

所有持久化金额均为整数 micro-USD（`1 USD = 1,000,000 micro-USD`）。计算过程使用 decimal，最终只执行一次 `ROUND_HALF_UP`，避免浮点误差。

```text
official_list =
  使用请求时冻结的官方定价输入，按实际定价模式和最终用量证据
  计算出的分组倍率前官方金额

procurement_cost_micro_usd =
  ROUND_HALF_UP(official_list_micro_usd × procurement_multiplier_ppm / 1,000,000)

sales =
  final_sales_quota / 请求时 quota_per_unit × 1,000,000

gross_profit = sales - procurement_cost

gross_margin = eligible_gross_profit / eligible_sales
```

其中：

- `official_list` 是未乘用户分组倍率的官方原价口径，也是库存扣减口径；
- `official_list` 不能在所有模式下简化为“单价 × token”：ratio、固定价、阶梯表达式，以及工具调用、音频等附加项，分别沿用现有定价模式的计算规则；
- `procurement_multiplier_ppm` 是采购折扣，6.5 折记为 `650000`；
- 采购成本必须严格按 `ROUND_HALF_UP(official_micro × procurement_ppm / 1,000,000)` 计算；中间乘法和最终结果都要检查溢出，持久化值与公式不一致视为篡改，两者都不得生成财务快照；
- `sales` 直接使用现有计费链路最终结算的 Quota 换算，不用“官方价 × 分组倍率”反推，因此兼容固定价、阶梯价和其他现有定价模式；
- `sales_multiplier_ppm = ROUND_HALF_UP(最终成功尝试的 GroupRatio × 1,000,000)`；倍率小数位不超过 6 位时等价于精确换算，超过 6 位时按 PPM 确定性量化；该值仅作为请求时销售倍率维度保留，不参与销售额的二次计算；
- 指针金额用于区分“未知”和“已知为 0”，报表同时展示金额及 `known_count`，不会把未知强行当作 0。

示例：官方原价为 100 美元，采购折扣为 6.5 折，最终对外结算为 7 折时：

| 指标 | 金额 |
| --- | ---: |
| 官方原价库存消耗 | $100 |
| 采购成本 | $65 |
| 销售额 | $70 |
| 毛利润 | $5 |

### 2.2 统计范围

只统计与现有消费日志口径一致、最终已成功完成财务结算的同步请求。生成 `captured` 财务快照必须同时满足：

- 最终选择的渠道已绑定供应商合同；
- `FinanciallyCommitted=true` 且财务确认时间大于 0；
- 存在正向最终用量证据：文本路径可由 `totalTokens > 0`、`FinalSalesQuota > 0` 或明确的工具/图片调用证明；音频/WSS 路径只接受 `totalTokens > 0` 或 `FinalSalesQuota > 0`。fixed/price 定价模式本身与财务已提交状态都不构成正用量证据。

每条已覆盖的消费日志都会写入一个有界 disposition marker，判定顺序固定为 `unsupported_path → not_financially_committed → zero_usage → unbound → captured → producer_error`。只有 `captured` 携带财务快照并进入金额汇总；异步任务等 V1 尚未支持的路径写 `unsupported_path`，不会伪装成已核算数据。

## 3. 请求时事实快照

### 3.1 冻结时机

渠道选定时，系统从进程内供应商缓存复制以下标量配置：供应商、合同、渠道绑定版本、采购折扣版本和采购倍率。发生渠道重试时，以最终实际成功渠道对应的冻结配置为准。模型原价输入和销售计费输入沿用该请求实际使用的现有定价配置。

显式账号排除决定在请求阶段冻结，后续配置更新不会改变在途请求的业务/内部归类。最终成功结算后，系统计算金额并把不可变快照写入同一条消费日志的 `other.supplier_accounting_v1`。

普通用户查询消费日志时会剔除该管理快照；供应链批处理直接从 `LOG_DB` 读取它。

### 3.2 快照字段

当前持久化协议的外层控制字段保持可读：`v`（envelope schema）、`c`（producer capability）、`a`（activation state version）、`d`（disposition）以及仅在 `captured` 时出现的 `s`。`s` 使用 capability-V1 固定宽度二进制布局和 canonical Raw URL Base64，避免 7000 万行级日志库产生冗长 JSON。包含 112-bit 阶梯表达式指纹的最坏 business/internal payload 分别精确为 330/320 字节，硬上限仍为 384/320 字节；disposition-only 上限为 160 字节。

倍率统一量化为 `ROUND_HALF_UP(GroupRatio × 1,000,000)`。阶梯定价指纹取请求时冻结的精确 UTF-8 `ExprString` 的 SHA-256 前 112 bits（前 8 字节加后续 6 字节，均为大端序），不做空格归一化；已有非空 `ExprHash` 必须与完整 canonical SHA-256 一致。该短指纹只用于批处理关联和核验，不是认证、唯一键，也不能替代表达式历史。

定价模式本身不能证明存在正用量。文本路径只接受正 token、正 `FinalSalesQuota` 或明确工具/图片调用；音频/WSS 路径只接受正 token 或正 `FinalSalesQuota`。不满足这些条件的 fixed/price 请求必须写成 `zero_usage`，不能生成官方金额和采购成本后再形成销售额为 0 的虚假负利润。

`types.SupplierAccountingLogSnapshotV1` 是解码后的逻辑财务快照。下表短 key 仅用于读取切换前已经写入的 legacy direct-snapshot，当前 envelope 不再把这些字段逐项展开为 JSON：

| Key | 含义 |
| --- | --- |
| `bv` | 渠道绑定版本 ID |
| `s` | 供应商 ID |
| `c` | 合同 ID |
| `rv` | 采购折扣版本 ID |
| `pm` | 采购倍率 PPM |
| `sm` | 请求时销售倍率 PPM，可空 |
| `ol` | 官方原价金额 micro-USD，可空 |
| `sa` | 最终销售额 micro-USD，可空 |
| `pc` | 采购成本 micro-USD，可空 |
| `gp` | 毛利润 micro-USD，可空，可为负数 |
| `ss` | `business` 或 `internal` |
| `ed` | `included` 或 `excluded` |
| `er` | 命中的排除规则 ID，可空 |
| `q` | 请求时 `quota_per_unit`，可空 |
| `p` | 请求时定价模式，可空 |
| `fc` | 财务确认时间戳，仅用于审计 |
| `qr` | 数据质量原因，可空 |

日报归属日以消费日志的 `logs.created_at` 所在北京时间自然日为准，`fc` 不参与归属日计算。

## 4. 账号排除

排除规则按显式 `user_id` 配置，不根据 root、admin 等角色动态猜测。规则采用追加版本，可用 `exclude` 将账号归为内部流量，也可用后续 `include` 恢复业务统计。

| 范围 | 官方原价 | 采购成本 | 库存消耗 | 销售额 | 毛利润 | 模型等高维信息 |
| --- | --- | --- | --- | --- | --- | --- |
| `business` | 统计 | 统计 | 统计 | 统计 | 统计 | 保留 |
| `internal` | 统计 | 统计 | 统计 | 不统计 | 不统计 | 不保留 |

这样可以在请求占比很高的公司自用账号上尽早冻结排除决定，同时仍然如实反映其对上游合同库存的消耗。

## 5. 数据模型

V1 新增恰好十张供应链表：

| 表 | 职责 |
| --- | --- |
| `upstream_suppliers` | 独立供应商实体，可关联多份合同和多个渠道 |
| `supplier_contracts` | 供应商合同及展示用 RPM、TPM、最大并发等信息 |
| `supplier_contract_rate_versions` | 追加式采购折扣版本；历史版本不可改写 |
| `supplier_channel_binding_versions` | 追加式渠道—合同绑定历史 |
| `supplier_inventory_adjustments` | 追加式库存台账 |
| `supplier_statistics_exclusion_rules` | 显式用户统计范围版本 |
| `supplier_admin_commands` | 创建类管理操作的幂等命令记录 |
| `supplier_usage_daily_summaries` | 唯一的供应链日汇总事实表 |
| `supplier_usage_daily_batch_runs` | 每个自然日的租约、游标、fence 和发布状态 |
| `supplier_accounting_coverage_gaps` | 低频控制面覆盖缺口台账；保存可重叠、可跨日的具名 gap epoch |

现有 `channels` 表新增 nullable `supplier_contract_id` 及索引，作为路由和管理查询的当前绑定投影；不可变历史保存在绑定版本表。

现有 `logs` 表不新增物理列或索引。供应链事实复用已有 `other` JSON 文本字段；日报扫描复用 `idx_type_created_at_quota (type, created_at, quota)`，再按 `(created_at, id)` 保证稳定分页。这样避免对现有约 7000 万行日志执行高风险表结构改造。

`LOG_SQL_DSN` 仍然是系统支持的可选日志分库配置，但本方案不要求新增或专用的 `LOG_SQL_DSN`。未配置时，现有初始化逻辑令 `model.LOG_DB = model.DB`；配置后，批处理显式从日志库读、向主库写。

## 6. 配置缓存与多节点一致性

供应商运行时配置由主库加载成完整、不可变的进程内索引，再通过原子指针整体替换。路由请求只读取该快照。

配置变更通知复用现有 channels 配置变更 Redis Pub/Sub；Redis 未启用或通知失败时，现有 60 秒周期刷新负责最终收敛。因此，准确口径是“不新增供应链 Redis 队列或专用 Redis”，而不是“系统完全不使用 Redis”。

采购折扣、绑定和排除规则均保留版本或不可变记录。请求已经冻结的版本不受后续缓存刷新影响，因此采购折扣从 6.5 折调整后，历史利润仍按当时的 6.5 折保留。

## 7. T+1 日汇总

### 7.1 调度规则

- 时区固定为 `Asia/Shanghai`；
- 02:00 前处于关账缓冲期，调用不做工作；
- 每次 catch-up 最多处理一个缺失自然日，只处理 D-1 及以前；
- Terraform 管理的 Cloud Scheduler 在 `02:05 Asia/Shanghai` 只启动一次 one-shot Cloud Run Job，不直接调用 Console；
- 每次成功 Job 最多推进一个最早 `published_fence_token = 0` 的自然日；N 个积压日需要 N 次串行成功执行，可由运维手动串行加速；
- catch-up/status 是 scheduler-only 接口，使用稳定 trusted job identity、current/next 双 token slot 和稳定 `Idempotency-Key`/`request_id`，不接受 Root token；
- 仓库内不启动常驻 Worker。Runner 在响应不确定时按同一 request ID 查询状态，不创建新 key。

Scheduler 接口只处理最早从未发布的日期，不接受指定日期。已发布但不完整的日期只允许 Root 从日报界面发起受 mutation gate、fresh secure verification、expected published fence 和稳定幂等键保护的 rerun；完整、未扫描、未发布或已有活动任务的日期拒绝 rerun。

每页最多扫描 5000 条消费日志，使用 `(created_at, id)` keyset 分页，避免 offset 在大表上的退化。扫描范围为北京时间自然日对应的 `created_at >= day_start AND created_at < day_end`，并只读取 `id`、`created_at`、`channel_id`、`model_name`、`other`。

响应字段：

- `processed_days`：本次完成的天数，当前最大为 1；
- `remaining_work`：是否仍存在可处理日期；
- `next_batch_date`：下一待处理自然日，若无则为空。

### 7.2 多节点安全

生产 Console 可运行多个实例，Cloud Scheduler/Job 也可能重复启动或遇到丢响应，因此批处理按多节点语义实现：

- 使用数据库时间判断租约，不依赖单机时钟；
- 每个自然日只有一条 batch run；
- 租约默认 30 分钟，并使用单调递增 fence token；
- 每页在事务中写本 fence generation 的汇总，并以 owner、fence、旧游标做 CAS 推进；
- 完成后才把本 generation 发布为 `published_fence_token`；
- 报表只读取已发布 generation；
- 新 owner 接管后，旧 owner 不能覆盖新结果；旧 generation 会被清理。

任务失败或新 owner 接管时，会递增 fence，并从该自然日开头重新计算；旧的已发布 generation 在新 generation 完成前继续供报表读取。持久化游标用于页级 CAS 和阻止旧 owner 继续推进，当前不承诺从失败页面断点续跑。

### 7.3 精确积压监控

现有 Router Managed Prometheus `/metrics` 从数据库当前事实生成五个低基数 gauge：

- `newapi_supplier_accounting_never_published_days`：当前 `published_fence_token = 0` 的可处理自然日数量，>1 表示当前至少积压两天；
- `newapi_supplier_accounting_oldest_never_published_age_seconds`：最老从未发布日的当前年龄，严格 >86400 表示超过 24 小时；
- `newapi_supplier_accounting_prior_day_unpublished_after_0800`：北京时间 08:00 后若前一日仍未发布则为 1；
- `newapi_supplier_accounting_backlog_observer_up`：数据库 snapshot 成功为 1，失败为 0；
- `newapi_supplier_accounting_backlog_observed_at_seconds`：最近成功 snapshot 的数据库观察时间，用于 freshness 证据。

Terraform policy 只过滤 `prometheus_target` 上的 Router service，并用 max 汇总 instance/revision 副本，绝不求和，因为所有实例观察的是同一组数据库事实。普通阈值持续 60 秒，至少覆盖两个 30 秒 scrape；observer max <1 或 metric 缺失持续 120 秒触发 health 告警。`supplier_batch_job_enabled=false` 时不创建这些 policy，phase one 不产生 absence noise，也不新增 observer Job、Scheduler、SA、secret、log metric、表或索引。

每个 Router 实例每 30 秒被独立抓取，数据库 snapshot 读放大会随实例数增长。G006 必须在 production-equivalent 最大实例规模下验证查询延迟、主库 CPU/连接/锁和 series 基数，并保存 backlog >=2、oldest >24h、08:00 miss、observer down/absence 的 live fire/resolve 与 `observed_at_seconds` 时间线，才能形成生产发布证据。

## 8. 历史数据边界

应用启动时在 `options` 表中一次性创建稳定切点：

```text
supplier_accounting_v1.coverage_start_at
```

首次启动可通过 Unix 秒环境变量 `SUPPLIER_ACCOUNTING_CUTOVER_AT` 指定；未指定时使用数据库当前时间。写入采用“已存在则不覆盖”的语义，因此后续修改环境变量不会改写既有切点。

- 切点前日志不扫描、不使用今天的定价猜算历史利润；
- 切点后没有 `supplier_accounting_v1` 快照的消费日志会被扫描但跳过；
- 当前公开接口只支持追赶缺失或未完成日期；已 completed 日期没有公开的指定日期强制重跑 API。内部 generation/fence 模型可以安全替换结果，但需另行提供受控运维入口后才能操作；
- 报表必须展示 coverage start 和 freshness，避免把部分覆盖误认为完整历史。

上线时参与首次初始化的所有 Go 服务必须配置同一个未来 cutover，且时间点要晚于全部 Router 新版本切流完成。环境值不一致时，最先成功写入 `options` 的值永久胜出；若 coverage 已开始而旧 Router 仍未写快照，会形成无法回算的静默缺口。

## 9. 库存

库存按官方原价口径管理，是展示和防止业务人员误判供应能力的辅助信息。台账支持：

- `initial`：初始库存；
- `replenishment`：追加库存，例如 `+200,000 USD`；
- `correction`：差错修正；
- `reversal`：冲销既有调整。

```text
total_inventory = Σ inventory_delta_micro_usd
consumed = Σ known official_list_micro_usd（business + internal）
remaining = total_inventory - consumed
```

未知官方原价的快照不会扣减库存。余额允许为负并显示 oversold；V1 不因此禁用渠道、拦截请求或自动防止超卖。库存也没有月框、年框或自动周期重置，合同中的 RPM、TPM、最大并发在 V1 仅存储展示，不执行上游限流。

## 10. 管理 API、报表与控制台

### 10.1 管理 API

`/api/supply-chain` 下提供：

- 供应商、合同、采购折扣版本和库存调整；
- 显式账号排除规则；
- 渠道绑定及绑定历史投影；
- overview、trend、contracts、channels、breakdown、freshness 报表；
- T+1 catch-up 入口。

创建供应商、合同、采购折扣版本、库存调整和排除规则时要求 `Idempotency-Key`，服务端保存命令或唯一台账记录以支持安全重放。渠道绑定变更使用 `expected_contract_id` CAS，避免两个管理员互相覆盖；更新和停用操作遵守各自的状态约束。

所有用户可访问的供应链读取、报表、命令结果和管理入口都由 `FinanceAuth` 限定 Root。全部 mutation（包含不完整日报 rerun）还必须经过版本化 mutation gate 和 fresh secure verification；只有 gate transition 可绕过 gate 本身，但仍要求 Root、fresh verification、expected-version CAS 和 actor-local idempotency。catch-up/status 使用独立 scheduler-only token，不能调用报表 rerun或任何 Root/Admin API。

### 10.2 报表

报表按 `Asia/Shanghai` 自然月或日期范围查询，单次范围最多 366 天，包含：

- `business` 与 `internal` 分栏；
- 官方原价、销售额、采购成本、毛利润、毛利率；
- 每个金额的 known count、未归因数量及数据质量；
- 库存总额、消耗、剩余、利用率和 oversold；
- 合同、渠道、模型、采购折扣版本、销售倍率、定价模式等维度；
- coverage start、最新批次和 freshness。

控制台入口为 `/supply-chain`，包含供应商、合同、采购折扣、库存、账号排除、渠道绑定和报表展示，并覆盖默认控制台全部 8 种语言。

## 11. 性能与可靠性

性能设计重点是让每日数百万请求的 95% 内部流量仍保持低开销：

- 请求路径只做进程内快照读取、整数/decimal 计算和现有日志 JSON 扩展；
- 不进行供应链请求级 DB/Redis 写入；
- `logs` 不加列、不加新索引；
- T+1 使用已有消费日志索引、5000 行 keyset 分页和页面级事务；
- 日报把高成本聚合结果写入唯一汇总表，管理查询不实时扫描原始日志。

已通过百万行普通场景和高维基数场景的本地性能门禁。本地 SQLite 证据约为普通场景 12–15 秒/百万行、高维基数场景约 94 秒；这些数据用于回归比较，不作为生产 MySQL/PostgreSQL SLA。

可靠性限制必须明确：

- `LogConsumeEnabled` 必须为 `true`；关闭消费日志后不会产生供应链事实；
- 现有消费日志写失败只记录错误，不会回滚已经完成的客户结算，批处理无法发现“整条消费日志未落库”的请求；
- 已持久化行中的 malformed/missing/incompatible 供应链快照属于行数据缺陷：日结发布仍可计算的金额、固定计数和有界 warning，将 `persisted_log_snapshot_completeness` 标为 `incomplete`，并推进 backlog；
- 日志页读取/扫描错误、fence 丢失、租约错误、主库事务失败或其他执行错误不发布任何新 generation，也不推进 backlog；已有 published generation 保持可见，后续在新 fence 下重试；
- 官方原价必须来自请求时实际计费链路的权威定价证据；本地 token 估算、启发式缓存或其他非权威回退不能用于财务快照。权威官方金额缺失或无法校验时必须 fail closed 为仅 disposition 的 `producer_error`，不持久 `s`、`quality_reason` 或任何部分财务事实。

## 12. 发布方案

发布前最低要求：

1. 在 staging 使用真实 MySQL/PostgreSQL 验证十张表、`channels.supplier_contract_id`、candidate/published 双缓冲和不变的 `logs` 物理 schema/index；
2. 验证 Router 与 Console 使用同一主库，日志库配置与当前环境一致，并给所有 Go 服务预设同一个未来 cutover；
3. mutation gate 保持 disabled。先部署兼容桥 Console 到 0% tagged URL，完成迁移/readiness/preflight，再直接切至 100%，等待完整 60 分钟管理请求硬超时；
4. 部署全部兼容 Router；若 legacy `newapi` 仍承载流量也同步部署。证明所有不兼容旧 writer 已排空；
5. 混合版本期间同时保留命令账本两条 legacy uniqueness bridge 与 `ux_supplier_inventory_contract_idempotency`。新 actor-local/scheduler/inventory 索引必须先有效；
6. 排空后为非驻留 CLI 显式提供实际物理数据库 identity 与有界、非秘密的 drain-evidence reference；identity 不匹配、options/command/inventory prerequisite 缺失、gate malformed/enabled 都必须在删桥前失败；
7. 调用 `/app/supplier_admin_finalize finalize`，finalizer 在删桥前后都验证 mutation gate disabled；再以 `verify` 两次证明三条旧桥都消失、三条新索引定义正确、digest 无异常；
8. 认证状态必须精确返回 `admin_command_ledger_state=finalized`。`bridge` 仅表示三条 legacy bridge 与替代索引同时完整，缺失、部分或未知 prerequisite 一律为 `invalid` 并阻断 promotion；
9. 只有 finalizer/readiness/preflight 都成功后，Root 才可通过 fresh secure verification 与 expected-version CAS 开启 mutation gate；
10. 使用 digest-pinned runner image 创建 Terraform Job/Scheduler/IAM/secret containers，验证 32-byte base64url 双 slot 轮换、45/55/>=60/60 分钟 deadline 顺序、多节点 fence/丢响应恢复和 08:00 SLO；
11. canary 核对消费日志快照、日报、合同库存和报表账实一致性。

部署目标：

- `newapi-router`：必须部署，因为请求选择、结算和日志快照写入发生在 Router；
- `newapi-console`：必须部署，因为迁移、管理 API、日结入口和控制台页面在 Console；
- legacy `newapi`：若仍承载 API 或控制台流量，必须同步部署；
- Terraform：必须应用 Cloud Run Job、Scheduler、独立 service accounts、最小 IAM 和 current/next Secret 容器；Terraform 不持有 raw token、具体 secret version 或 verifier hash；
- `newapi-web`、Cloudflare：不涉及。

回滚必须区分 finalizer 前后：finalizer 前可在 gate disabled 状态回到兼容桥版本；finalizer 后不得恢复依赖旧唯一索引的 writer，只能回滚到已证明兼容 finalized schema 的 digest。active 后的不兼容紧急切流必须先原子进入 degraded 并打开具名 coverage gap。完整操作顺序见[供应商核算运维手册](./supplier-accounting-operations.md)。

当前实现尚未因此文档自动部署 staging 或 production。

## 13. 明确非目标

V1 不做以下事项：

- 供应链库存不足时自动停用渠道或拦截超卖；
- 月框、年框的自动周期建账或重置；
- 实时利润汇总；
- 切点前历史利润估算；
- 异步任务类请求的供应链快照和日报；
- 修改现有 7000 万行 `logs` 表结构；
- 供应链 Redis Stream、独立 Redis、publisher/consumer 或常驻 Worker；
- Cloud Scheduler 直接长连接调用 Console；它只能启动 Terraform 管理的 one-shot Cloud Run Job；
- Cloudflare 或 `newapi-web` 部署。
