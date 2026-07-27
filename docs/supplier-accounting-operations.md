# 供应商核算运维手册

本文档用于发布和运维[上游供应链与利润核算 V1](./supplier-supply-chain-v1.md)。权威链路以 `supplier_accounting_facts` 为源；通用日志只作审计镜像；历史估算只有显式发布后才作为报表基线，且永不影响库存。

## 1. 运行拓扑

| 服务 | 节点角色 | 职责 |
| --- | --- | --- |
| `newapi-console` | Master | 主库迁移、管理 API、报表、T+1 日结、历史估算分页和 fact 留存 |
| `newapi-router` | Slave | 已绑定同步 relay 尝试的 fact prepare/finalize；成功日志写审计镜像；不运行 ticker |
| `newapi-web` | Website | 无供应商核算职责 |

`supplier_accounting_facts` 位于 LOG_DB；其余 11 张供应商表位于主库。没有独立 `LOG_SQL_DSN` 时两者共享物理数据库。多实例正确性由数据库租约、CAS、唯一日期发布指针和 fence 保证，不依赖 Redis。

## 2. 发布前检查

### 2.1 数据库与配置

- 主库能够创建 11 张供应商表；LOG_DB 能够创建 `supplier_accounting_facts` 及唯一/日状态索引；
- 在真实 SQLite、MySQL 5.7.8+、PostgreSQL 9.6+ 实例执行迁移、fact CAS、lease/fence、重叠导入和汇总 upsert 验收；DryRun schema 检查不能替代真实数据库执行；
- Router 和 Console 指向同一个 LOG_DB 事实源；
- `SUPPLIER_ACCOUNTING_CUTOVER_AT` 在所有 Go 服务中保持一致，值为未来某个北京时间零点的 Unix 秒；
- `SUPPLIER_ACCOUNTING_FACT_RETENTION_DAYS` 首次发布建议不设置，完成账实核对后再配置；
- 不在 staging 复制生产 OAuth、支付、回调、域名或数据库凭据。

真实数据库验收使用 `TestSupplierAccountingRealDatabaseIntegration`。测试默认 skip，DSN 必须指向零表、可丢弃的隔离数据库，并显式设置 `TEST_SUPPLIER_ACCOUNTING_ALLOW_SCHEMA_MUTATION=isolated-empty-database`；MySQL/PostgreSQL DSN 分别通过 `TEST_SUPPLIER_ACCOUNTING_MYSQL_DSN`、`TEST_SUPPLIER_ACCOUNTING_POSTGRES_DSN` 提供。测试会拒绝非空 schema，并覆盖 12 表迁移、fact CAS、日报 upsert、historical lease/fence、历史发布/替代和权威日结覆盖。

### 2.2 热路径容量门禁

已绑定同步请求会在上游调用前用单条 DB-time conditional INSERT 持久化 pending fact，并在结算后同步 CAS 为 captured/void。两个持久化边界不能合并，也不能用丢失事实的内存队列或异步降级替代。生产 cutover 前必须使用生产峰值 attempt 模型完成数据库压测，且覆盖真实 retry/bound 比例与约 95% internal/root 请求分布：

- 分别记录 relay 总延迟与 fact prepare/finalize 的吞吐、p95、p99、错误率、连接池等待和数据库锁等待；
- 至少持续运行 1.5 倍生产峰值 60 分钟，并验证 2 倍峰值 10 分钟突发；数据库 CPU、连接和 I/O 峰值保留至少 30% 余量，Router p95/p99 与错误率不突破既有生产 SLO；
- 验证 LOG_DB 短暂不可用时请求按预期 fail-closed，恢复后不会产生重复终态或绕过 pending；
- 在当前共享主库拓扑下先压测；若容量或延迟不达标，保持 cutover 未配置，优先扩容共享 SQL 并重新验证；
- `LOG_SQL_DSN` 会整体切换通用 `logs`、`log_request_samples` 和 supplier facts，不是 fact 专用连接。若确需独立 LOG_DB，必须先完成现有约 7000 万历史日志的迁移/查询连续性方案，并确认 Router、Console、legacy 节点指向同一 LOG_DB；禁止把它直接指向空新库。V1 不支持只把 supplier facts 单独拆库；
- cutover 在真实数据库矩阵、故障恢复和账实核对全部通过前保持未配置。

### 2.3 样本验证

至少覆盖：

| 场景 | fact 预期 | 权威日报预期 |
| --- | --- | --- |
| 已绑定、最终成功、正用量 | `captured`，payload/hash 完整 | 进入业务或内部汇总 |
| 已绑定、明确零用量 | `void` | 不产生金额 |
| 未绑定渠道 | 不创建 fact | 不属于 V1 覆盖范围 |
| 流式响应不完整或金额证据异常 | `pending` | 对应日期不得发布 |
| 渠道重试 | 每个真实已绑定尝试独立 fact | 只统计 captured 尝试 |

同时确认成功消费日志保留 attempt ID 和财务审计镜像，Admin/普通用户日志响应看不到财务对象。

## 3. 安全发布顺序：Console 先，Router 后，最后 cutover

禁止先让 Router 在缺少 fact 表或配置不一致时进入 fail-closed 区间。推荐流程：

1. 保持 `SUPPLIER_ACCOUNTING_CUTOVER_AT` 未配置，先部署 Console；确认主库 11 张表和 LOG_DB fact 表创建完成。
2. 仍保持 cutover 未配置，部署全部 Router；此时新代码已在位但不会创建 fact。
3. 检查 Console 管理页、12 张表、LOG_DB 连通性和 Router 健康状态。
4. 选择未来一个 `Asia/Shanghai 00:00:00`，把相同 cutover Unix 秒配置到 Console、Router 和 legacy `newapi`（若仍承载流量）。
5. 在 cutover 前完成所有修订切换；cutover 后抽样确认 bound attempt 先 pending、再 captured/void。
6. 次日 02:00 后核对首个 completed batch 和权威报表。

该变更影响 `/v1` relay 尝试、财务事实和日结，生产 Router 部署为 `required`，Console 部署也为必需；`newapi-web`、Terraform 和 Cloudflare 不参与本功能代码发布。若 legacy `newapi` 仍接收 API 流量，也必须部署相同修订和 cutover。

## 4. Staging 发布与验收

staging 从远端 `staging` 分支自动部署：后端工作流为 `.github/workflows/gcp-deploy-staging.yml`，服务为 `newapi-staging`。先将目标提交合入或推送到 `staging`，不要把 staging 成功视为生产已发布。

`newapi-staging` 当前启用 Cloud Run `cpu_idle=true`。实例没有请求时 CPU 可能暂停，不能依赖空闲期间的后台 ticker 按墙钟持续执行。验收日结、历史导入或 retention 时，应在等待后台轮询期间持续发送受控的低频健康/测试请求，使实例保持可调度；若没有持续流量，后台任务未推进不能判定为业务失败。

在 staging 使用独立数据库与域名，并执行：

1. cutover 未配置时启动，确认 12 张表创建且普通 relay 不产生 fact；
2. 将 cutover 设为未来北京时间零点，重发修订并确认各节点配置一致；
3. cutover 后发送已绑定成功、零用量、失败、重试和流式请求；
4. 验证 captured/void/pending 终态和 pending 管理接口；
5. 人工解决测试 pending，确认日结冻结水位后才能 completed；
6. 验证业务/内部报表、库存公式，以及历史估算发布后进入报表但不影响库存；
7. 验证历史导入 pending/running/failed 不展示金额，completed 后可预览；显式发布后报表显示估算标记，同日权威日结覆盖估算；
8. 对同范围执行重新估算，确认新版本发布时原子替代旧版本，旧版本仍可审计。

## 5. 每日检查

北京时间 02:00 后检查前一自然日：

1. batch run 状态为 `completed`，`published_fence_token > 0`；
2. `coverage_scope` 正确，`source_max_fact_id` 与 LOG_DB 当日冻结水位相符；
3. 水位内没有 pending fact；
4. `logs_scanned`（兼容字段名，实际为 facts scanned）、snapshot 数和汇总行数量级合理；
5. 管理端最新完成日期为前一日，缺失日期没有显示为零；
6. 业务维度满足 `毛利润 = 销售额 − 采购成本`；
7. 内部维度没有销售额、毛利润或不必要的模型高维；
8. 合同余额等于库存调整累计值减去业务与内部官方原价消耗。

通用日志数量不再是权威日结完成条件。它可用于定位请求和恢复 pending，但不能替代 fact 水位核对。

## 6. Pending fact 审计

存在 pending 时，对应自然日必须保持未发布。Root 通过供应链 pending 接口按 prepared day 和 ID keyset 查询，并选择：

- `void`：有证据证明该尝试没有形成财务消费；
- `capture_from_log`：指定同一 parent request 且包含同一 attempt ID、供应商/合同/版本一致的消费日志恢复 captured payload。

每次人工解决必须记录 actor、reason 和 evidence，且走 FinanceAuth、critical rate limit 和 secure verification。不要直接改数据库状态、payload hash 或 batch fence。

若找不到足够证据，保持 pending 并调查生产者；不得为赶日结而猜测金额或强制作废。

## 7. 批次故障处理

| 状态/错误 | 处理 |
| --- | --- |
| `running` 且租约有效 | 等待当前 owner；不要启动旁路任务 |
| 租约过期 | 新 Console 可按数据库时间接管并递增 fence |
| `supplier accounting facts remain pending` | 审计并解决水位内 pending facts |
| 水位变化 | 排查 cutover 后迟到 prepare/错误日期；禁止强行发布 |
| LOG_DB 读取失败 | 检查连接、权限、索引、超时和数据库容量 |
| 主库写入失败 | 检查事务、锁、唯一约束、磁盘和连接池 |
| fence lost | 旧 owner 正常退出；确认新 owner 正在推进 |

报表只读 published fence，失败或运行中的候选行不可直接改成已发布。缺少某天时先检查更早的未完成日，因为每次 ticker 最多推进一个自然日。

## 8. 权威 catch-up 与 cutover

`SUPPLIER_ACCOUNTING_CUTOVER_AT` 必须是北京时间零点。Console 从 cutover 自然日寻找最早未完成日，每个 ticker 最多推进一天。

- cutover 前无 fact，不做权威回填；
- cutover 后只读取 durable facts，不从通用 logs 合成；
- 已完成日不提供任意 rerun；
- 水位内 pending 未解决前不能完成该日；
- 修改 cutover 会改变覆盖合同，生产启用后不得随意前移或后移。

## 9. 历史估算操作

历史估算用于少量上线前日志，不是权威 catch-up：

1. Root 打开供应链“历史估算”页；
2. 输入北京时间 `[start_date, end_date)`、字符串 QPU、排除用户 ID JSON、渠道映射 JSON 和原因；
3. 渠道映射必须明确给出 channel、supplier、contract、rate version 和采购折扣 PPM；
4. 后台 worker 首次取得该任务 lease 时冻结 LOG_DB 最大日志 ID 和候选条数；排队期间任务仍未冻结，后续接管和重试必须复用首次冻结值；
5. 每分钟 ticker 最多推进一页，可在列表查看进度；
6. completed 前金额保持隐藏；完成后查看按日 sales/cost/gross 及 unknown/unassigned 覆盖；
7. completed 后先核对预览，再显式“发布到报表”；发布不改变库存；
8. 需要修正映射、折扣或污染数据时，对 completed 任务执行“重新估算”；日期范围必须一致，新版本完成并发布后原子替代旧版本；
9. 旧结构任务若页面提示“需要重新估算”，不得直接修改数据库绕过版本门。

历史结果不得复制或回填到 `supplier_accounting_facts`，也不得用于库存扣减。报表通过 `supplier_historical_published_days` 读取当前发布版本；同日存在权威 published fence 时必须忽略历史估算。

## 10. Fact 留存

`SUPPLIER_ACCOUNTING_FACT_RETENTION_DAYS`：

- 未设置或 `0`：关闭删除；
- 正整数：只选择已经 completed 且早于保留边界的自然日；
- 每次 ticker 最多删除 5000 条 `captured`/`void`，并限制 `id <= source_max_fact_id`；
- pending facts 永不删除；未完成日期、超出已发布水位的事实也不删除；
- 日汇总和 batch 水位继续保留，用于证明历史发布边界。

首次启用前先核对至少一个完整保留周期，并确认通用日志或外部审计留存满足业务要求。启用后观察 LOG_DB 行数下降、删除批次耗时和锁等待；异常时把值恢复为 `0`，不要手工大批量删除。

## 11. 多节点与回滚

多个 Console 同时触发 ticker 属于正常场景；只有获得主库 lease/fence 的 owner 可以推进。旧 owner 恢复后必须因 fence 不匹配停止。

应用回滚不删除 12 张表、不改 fact 终态、不删除日报和历史估算：

- Router 回滚到不支持 fact 的版本会在 cutover 后造成权威覆盖缺口，必须立即停止扩大流量并恢复兼容修订；
- Console 回滚会停止日结、管理和留存，但已发布 fence 保持不变；
- 不得通过清表、降低 fence、改 batch 状态或重写 payload 来回滚；
- 若必须在 cutover 前回滚，保持 cutover 未生效或重新选择未来零点；cutover 已生效后不要通过修改时间戳掩盖缺口。

## 12. 发布验证清单

- [ ] 主库 11 表、LOG_DB 1 表已在真实 SQLite、MySQL 5.7.8+、PostgreSQL 9.6+ 实例完成迁移与状态机验证；
- [ ] 生产峰值模型 DB 压测、p95/p99、连接池等待、故障恢复和账实核对通过；若共享主库不达标，已切换并复测独立 LOG_DB；
- [ ] Console 先于 Router 完成迁移，所有流量节点在 cutover 前就绪；
- [ ] staging 使用独立配置完成 captured/void/pending 测试；
- [ ] pending 审计、secure mutation 和日结阻断生效；
- [ ] 多 Console 竞争、接管和旧 owner 恢复测试通过；
- [ ] 报表只读 published fence，业务/内部/库存口径一致；
- [ ] 历史估算完成门、显式发布、重新估算原子替代、估算标记、同日权威覆盖和库存隔离通过；
- [ ] 留存关闭态验证通过；启用时只删除符合条件的 terminal facts；
- [ ] Router deploy: required；Console/legacy 流量节点部署范围已确认；网站、Terraform、Cloudflare 无变更。
