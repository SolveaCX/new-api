# NewAPI 原生状态中心设计

日期：2026-07-16

状态：交互设计与技术契约已由用户批准

目标系统：NewAPI Router、NewAPI Console、Flatkey Website

## 1. 摘要

在 NewAPI 内建设一个原生状态中心，公开展示 Router 与每个官网公开模型的当前健康度、90 天可用率、监控覆盖率、历史状态、相关事故和计划维护。系统同时支持管理员事故工作流，以及邮件、单个管理员 Discord Webhook、通用订阅 Webhook 三种通知渠道。

状态中心使用混合信号：足量真实生产流量优先，低流量或空闲模型使用自适应合成探针。真实流量、探针、状态计算、事故草稿和通知投递均在 NewAPI 内完成，不以 Uptime Kuma 或第三方 Statuspage 为主系统。现有 Uptime Kuma 集成继续保留，但不承担公开状态中心的数据源职责。

## 2. 目标与非目标

### 2.1 目标

1. 展示 Router 与官网公开模型目录中每个模型的当前状态。
2. 为每个模型提供可分享的详情页，包括 24 小时、7 天、30 天、90 天健康历史。
3. 提供 90 天可用率和独立的监控覆盖率，Unknown 不得伪装为 Operational。
4. 以真实生产请求为主、合成探针为辅，避免为高流量模型制造无谓探针成本。
5. 自动生成和更新事故草稿；管理员编辑并发布公开事故文本。
6. 支持计划维护、临时状态覆盖、审计和多管理员并发保护。
7. 支持经验证的邮件订阅、通用 Webhook 订阅和一个管理员配置的 Discord Webhook。
8. 在 SQLite、MySQL 5.7.8+、PostgreSQL 9.6+ 上保持一致行为，并在多节点生产部署中保证正确性。

### 2.2 非目标

- 不公开渠道 ID、真实上游供应商、API Key、客户流量、原始错误或请求数量。
- 不把用户密钥无效、用户额度不足、非法请求、策略拒绝或明确客户端取消计入平台可用性。
- 不声称实现网络层 exactly-once Webhook 投递；通知语义是持久化的 at-least-once，并提供事件 ID 供接收方去重。
- 不伪造新口径启用前的历史数据，也不把无数据时间补成绿色。
- 不在首版依赖新的第三方 SaaS 或新的运行时依赖。
- 不把公开状态中心变成渠道级诊断工具；渠道健康仍属于管理员内部运维视图。

## 3. 现有能力与复用边界

### 3.1 真实流量指标

`pkg/perf_metrics` 已在 Relay 完成路径中无锁采集模型请求、成功率、Latency、TTFT 和吞吐，并按时间桶持久化。最新主线还提供模型 Latency/TTFT Prometheus histogram 和最终错误分类。状态中心扩展该采样结构，增加可用性专用计数：

- `availability_eligible_count`
- `availability_success_count`
- 低基数的 `availability_outcome`

状态中心不在 Relay 热路径增加同步网络或数据库调用。新增字段继续进入现有原子桶并由 flush 任务批量持久化。

### 3.2 合成探针

`controller/model_availability_task.go` 已有模型枚举、探针目标选择、`testChannelWithOptions` 调用和错误分类能力。状态中心提取可复用的 service 级探针适配器，保留现有“官方不支持模型”任务行为，并为状态中心增加独立调度策略和独立结果表。二者不得共享含义不同的状态字段。

### 3.3 官网公开模型目录

组件同步使用 `GetWebsitePricing` 相同的可见模型口径：经过官网可用分组过滤后的 `model.GetPricing()` 结果。状态页与 `/models`、`/pricing` 的公开模型集合必须一致。后台存在但未出现在官网公开价格目录中的模型不创建公开组件。

### 3.4 现有官网健康展示

官网当前通过 `/api/perf-metrics` 展示基于真实流量的 30 天成功率和平均 TTFT。状态中心上线后，公开“健康度”统一改用状态中心 API；性能摘要仍可作为模型详情中的参考趋势，但不得替代可用性状态机。

## 4. 总体架构与数据流

```text
Router Relay 完成路径
  -> perf_metrics 原子桶（新增 eligible/success availability counters）
  -> 批量持久化

Console 状态调度器（DB lease + fencing token）
  -> 同步公开组件目录
  -> 读取最近 5 分钟真实流量
  -> 对低流量模型安排探针
  -> 每分钟计算 observed_status
  -> 应用 maintenance / manual override 得到 effective_status
  -> 写入 5 分钟状态桶与聚合桶
  -> 创建或更新私有事故草稿

管理员发布事故/维护更新
  -> 同一事务写入 append-only published update
  -> 同一事务创建 delivery outbox rows
  -> Email / Discord / Webhook workers 领取并投递

公开 API
  -> Flatkey Website 同源 API proxy
  -> /status 与 /status/models/:slug
```

Router canary 每分钟从 Console 通过配置的公开 `ROUTER_ORIGIN` 发起一次真实外部路径请求。模型探针复用渠道测试能力，Router canary 与模型探针分别判断路由层和模型层，避免把一个层面的错误重复归因到另一个层面。

## 5. 组件目录

### 5.1 组件类型

- `router`：全局唯一 Router 组件。
- `model`：官网公开价格目录中的每个模型一个组件。

每个组件有稳定内部 ID、稳定公开 slug、`component_key`、显示名称、模型名称、能力分类和生命周期状态。模型名称变化不会修改已有组件的主键；若只是显示文案变化则更新显示名，若规范模型 ID 变化则创建新组件。退出公开目录的模型标记为 retired，停止新探针，但其公开历史至少保留到最后一个 90 天窗口结束。

### 5.2 公开总体状态

总体横幅按以下优先级计算：

1. Router outage：`Major outage`。
2. 任一模型 outage，或至少 20% 活跃模型 degraded/outage：`Some systems affected`。
3. 任一模型 degraded：`Degraded performance`。
4. 仅存在 Unknown：`Monitoring incomplete`。
5. 仅存在 Maintenance：显示计划维护提示，不伪装为全绿。
6. 其余情况：`All systems operational`。

## 6. 可用性请求口径

### 6.1 计入可用性的结果

| 结果 | 是否计入分母 | 是否成功 |
| --- | --- | --- |
| 最终成功响应 | 是 | 是 |
| 上游 5xx、timeout、网络失败、坏响应 | 是 | 否 |
| 路由重试耗尽、没有可用渠道、Router 自身错误 | 是 | 否 |
| 上游渠道认证或平台运营配置错误 | 是 | 否 |
| 用户 API Key 无效 | 否 | 否 |
| 用户额度不足或用户限额 | 否 | 否 |
| 用户请求体/参数无效 | 否 | 否 |
| 策略拒绝 | 否 | 否 |
| 明确客户端取消 | 否 | 否 |

分类函数必须是表驱动、低基数和可测试的。分类发生在最终重试结果上，不对中间渠道 attempt 重复计数。

### 6.2 信号优先级

- 最近五分钟至少有 20 个 eligible 请求：真实流量为权威信号。
- 少于 20 个 eligible 请求：使用最新且仍新鲜的合成探针。
- 两类信号冲突且真实流量不足：保持 degraded，并在一分钟后重探。
- 20 分钟没有可信证据：转为 Unknown，绝不保持绿色。
- 探针本身的 Token、额度或本地配置错误属于 monitoring fault，不计为模型失败；它降低覆盖率并最终导致 Unknown。

## 7. 状态机

公开状态只有五种：

- `operational`
- `degraded`
- `outage`
- `unknown`
- `maintenance`

### 7.1 降级与故障

- 实流量足量且成功率 `>= 99.5%`：Operational。
- 实流量足量且成功率 `>= 95%` 且 `< 99.5%`：Degraded。
- 实流量足量且成功率 `< 95%`：Outage。
- 低流量时连续两次探针失败：Degraded。
- 低流量时连续三次探针失败：Outage。
- 一次足量流量桶低于 99.5% 可直接进入 Degraded；低于 95% 可直接进入 Outage。

### 7.2 恢复防抖

- 连续三次成功探针，或
- 连续两个足量流量桶且成功率均 `>= 99.9%`

满足任一条件后恢复 Operational。Unknown 不自动解决已发布事故；恢复后只创建建议的解决草稿，等待管理员发布。

### 7.3 observed 与 effective

状态引擎始终保存 `observed_status`。公开 `effective_status` 的优先级为：

1. 已发布且处于生效窗口的计划维护；
2. 未过期的手动覆盖；
3. observed 状态。

Admin 可覆盖为 maintenance、unknown、degraded、outage。强制 Operational 只允许 Root，要求二次安全验证、必填原因、最长一小时有效期，并在公开详情中显示手动状态标记。所有覆盖都必须有过期时间。

## 8. 探针调度

- Router canary：每分钟一次。
- 最近有足量真实流量的模型：跳过常规探针。
- 空闲或低流量模型：每 15 分钟一次。
- 当前 degraded/outage、信号冲突或刚恢复的模型：每分钟一次，直到稳定。
- 维护中的模型继续收集 observed 信号，但不自动发布新的公开事故。

探针按模型能力选择最小合法请求，限制输出和成本，不写普通用户请求日志，不保存响应正文。每个结果只保留状态类别、耗时、内部目标标识、时间和经过清洗的诊断类型。

## 9. 90 天历史与可用率

每个五分钟桶记录一个可用性 score：

```text
足量流量：eligible_success / eligible_count
低流量：successful_probes / probes
无可信证据：unknown
维护窗口：maintenance
```

90 天可用率是所有 known 五分钟桶 score 的算术平均。Unknown 和 Maintenance 不进入可用率分母，但分别进入覆盖率与维护时间统计。

```text
availability = score_sum / known_bucket_count
coverage = known_bucket_count / (total_bucket_count - maintenance_bucket_count)
```

为避免对四舍五入百分比二次平均，数据库保存 `score_sum_micros` 和 `known_bucket_count`。小时和日聚合只累加整数 score 与桶数量。

保留策略：

| 数据 | 保留时间 |
| --- | --- |
| 原始探针结果 | 7 天 |
| 五分钟状态桶 | 7 天 |
| 小时聚合 | 100 天 |
| 日聚合 | 100 天 |
| 已发布事故、维护、审计 | 不自动删除 |

模型详情同时展示可用率、覆盖率、日级最差状态、关联事故，以及来自现有性能指标的 Latency/TTFT 参考趋势。性能趋势是解释性数据，首版不直接改变状态机。

## 10. 持久化模型

所有时间使用 Unix 秒或 UTC 时间；状态和类型使用 `varchar`，不使用数据库 enum；JSON 数据使用 TEXT 并通过 `common.Marshal` / `common.Unmarshal` 处理。

### 10.1 核心表

- `status_components`：组件身份、目录状态、observed/effective 状态、最后证据、覆盖、手动覆盖、版本。
- `status_periods`：组件、粒度（5m/hour/day）、周期开始、score sum、known/unknown/maintenance counts、最差状态、流量/探针汇总、Latency/TTFT 汇总。
- `status_probe_results`：短期探针诊断结果，不含响应正文或密钥。
- `status_incidents`：事故或维护头、影响级别、公开状态、自动化模式、时间范围、版本。
- `status_incident_updates`：append-only 更新；草稿与发布状态分离，已发布正文不可原地修改。
- `status_incident_components`：事故与组件关联。
- `status_subscribers`：邮件或 Webhook 订阅、验证状态、管理 token hash、加密 endpoint/secret。
- `status_subscriber_components`：订阅组件过滤；空集合表示全部组件。
- `status_delivery_outbox`：不可变事件 payload、逻辑幂等键、领取 lease、尝试次数、下次重试和结果摘要。
- `status_job_leases`：调度任务 lease、holder、expires_at、fencing_token。
- `status_audit_events`：管理员、操作、对象、前后值、原因、时间。
- `status_settings`：状态中心开关、安全阈值、一个加密 Discord endpoint 及非敏感配置。

### 10.2 唯一约束

- `status_components.component_key`
- `status_components.slug`
- `status_periods(component_id, granularity, period_start)`
- 自动事故 transition idempotency key
- `status_delivery_outbox(published_update_id, destination_type, destination_id)`
- 订阅者归一化身份 hash

## 11. 多节点与幂等

数据库是正确性的最终来源。Redis 可用于短缓存或提示，不作为唯一锁。

- 调度器通过 `status_job_leases` 的 compare-and-swap 更新获取 lease，并携带 fencing token。
- 每个周期写入使用唯一键 upsert；重复调度不会创建重复状态桶。
- 事故自动化使用组件、状态转换和时间窗口生成幂等键。
- Outbox worker 使用 `status/locked_until/version` 条件更新领取任务，不依赖 MySQL 8 才支持的 `SKIP LOCKED`。
- 失去 lease 的 worker 不得用旧 fencing token 提交调度结果。
- 管理员更新携带 version；冲突返回 HTTP 409，前端显示 reload/diff。

## 12. 事故、维护与管理员工作流

### 12.1 自动事故草稿

组件从 Operational 进入 Degraded/Outage 时，自动创建或更新私有草稿并附带内部证据摘要。组件恢复时自动创建解决建议草稿。自动化永不编辑已经发布的正文，也不自动发送事故通知。

### 12.2 发布状态

公开事故更新使用 `investigating`、`identified`、`monitoring`、`resolved`。管理员每次发布都会新增一条不可变 update；修改已发布信息必须发布更正 update。

### 12.3 计划维护

管理员发布维护计划即预授权调度器在开始和结束时间执行状态切换和已配置通知。维护期间 observed 信号继续计算。维护结束后 effective 状态立即回落到当前 observed 状态；若仍失败则创建事故草稿。

### 12.4 权限

- Admin：查看证据、编辑/发布事故、管理维护、创建非绿色临时覆盖。
- Root：以上全部，加上阈值、安全设置、Discord Webhook、订阅投递控制和短时 force-green。
- Discord endpoint、阈值修改和 force-green 使用 `RootAuth`；敏感操作叠加 `SecureVerificationRequired`。

## 13. API 契约

### 13.1 公开只读 API

- `GET /api/status/summary`
- `GET /api/status/components?kind=model&query=&capability=&status=`
- `GET /api/status/components/:slug`
- `GET /api/status/components/:slug/history?range=24h|7d|30d|90d`
- `GET /api/status/incidents`
- `GET /api/status/incidents/:id`
- `GET /api/status/maintenance`

响应只包含公开聚合信息，使用 ETag 和短期 Cache-Control。响应总是携带 `generated_at`、`last_trustworthy_update_at` 和 coverage。无新鲜数据时返回 Unknown/monitoring unavailable，不返回伪造绿色。

### 13.2 公开订阅 API

- `POST /api/status/subscriptions`
- `GET /api/status/subscriptions/verify?token=`
- `GET /api/status/subscriptions/unsubscribe?token=`：仅显示确认页数据。
- `POST /api/status/subscriptions/unsubscribe`：真正取消，避免邮件扫描器触发 GET 即退订。

写接口必须有 IP 限速、通用反枚举响应和严格请求体上限。

### 13.3 管理 API

- `/api/status/admin/incidents`
- `/api/status/admin/maintenance`
- `/api/status/admin/overrides`
- `/api/status/admin/settings`
- `/api/status/admin/discord/test`
- `/api/status/admin/subscribers`
- `/api/status/admin/deliveries`
- `/api/status/admin/audit`

Controller 只负责绑定、鉴权上下文和响应；业务事务在 service，数据库读写在 model。

### 13.4 Website 同源代理

Next.js 在 `website` 内提供同源 `/api/status/*` route handlers，目标由 `APP_CONSOLE_ORIGIN` 决定。公开页面不直接硬编码 Console 或 Router 域名。

## 14. 订阅与通知安全

### 14.1 邮件

- 使用现有 SMTP。
- 新订阅先进入 pending，发送 24 小时有效的一次性验证 token。
- token 只保存 hash。
- 邮件地址归一化并以 hash 做唯一性；API 使用通用响应防止枚举。
- 退订 GET 只展示确认，POST 才改变状态。

### 14.2 通用 Webhook

- 只允许 HTTPS。
- 注册后返回 signing secret 一次，并向 endpoint 发送 challenge。
- endpoint 正确回显 challenge 后才激活。
- 每次连接前解析 DNS 并拒绝 loopback、link-local、私网、metadata 和其他非公网地址。
- 禁止重定向；限制端口、连接/总超时、响应体大小和并发。
- Header 包含 `X-NewAPI-Event-ID`、`X-NewAPI-Timestamp`、`X-NewAPI-Signature`。
- 签名为 `v1=HMAC-SHA256(timestamp + "." + raw_body)`。

### 14.3 Discord

全局只允许一个 Root 配置的 Discord Webhook。测试投递是显式操作。正式投递与普通 Webhook 共用 outbox、重试和审计，但使用 Discord payload adapter。

### 14.4 敏感数据加密

现有 `CRYPTO_SECRET` 只提供 HMAC 和密码哈希，不用于可逆加密。新增：

- `STATUS_SECRET_KEYS`：`key_id:base64-32-byte-key` 列表。
- `STATUS_SECRET_ACTIVE_KEY_ID`：当前写入 key。
- 标准库 AES-256-GCM versioned envelope。

Discord URL、订阅 Webhook URL 和 Webhook signing secret 加密保存。读取支持旧 key，写入只用 active key，以便轮换。密钥缺失时状态读取、探针和邮件仍可工作，但 Webhook/Discord 配置与投递被禁用并在管理端明确告警。

## 15. Outbox 投递语义

发布 update 与创建 outbox 记录在同一事务。每个逻辑目的地只有一个 outbox row，但 HTTP 网络语义是 at-least-once：目标已经处理请求而响应丢失时可能重投。接收方通过 event ID 去重。

- 2xx：成功。
- 永久 4xx：停止该条重试；连续永久失败会暂停目的地。
- timeout、网络错误、429、5xx：指数退避加 jitter。
- payload 入队后不可变；后续更正产生新 published update 和新 event ID。
- Webhook/Discord 失败不阻塞状态计算、公开 API 或其他订阅者。

## 16. 公开页面

官网增加 locale-aware `/status` 和 `/status/models/:slug`。

### 16.1 `/status`

1. 顶部总体状态、最后可信更新时间、监控覆盖率和订阅按钮。
2. Router 置顶，显示当前状态、90 天可用率和日条。
3. 全部模型列表，支持名称、能力、状态过滤；不按真实上游供应商分组。
4. 最近事故和计划维护。
5. 状态必须同时使用文字、图标和颜色；键盘可访问，移动端保持可读。

### 16.2 模型详情

- 当前状态与最后可信证据。
- 90 天可用率、覆盖率、事故数。
- 24h/7d/30d/90d 状态条。
- 每日最差状态、可用率和覆盖率。
- 相关事故时间线。
- Latency/TTFT 参考趋势，不显示请求数量。
- retired 模型在历史窗口内保留，并显示 retired 标签。

页面每 60 秒刷新，SSR 首屏使用短缓存。API 不可达时保留最后缓存快照并明确显示 stale/monitoring unavailable。

## 17. 管理控制台

`web/default` 增加 Status Center 功能区：

- Overview：Router、模型状态、草稿数量、投递队列。
- Incidents：自动草稿、证据、编辑、发布、解决。
- Maintenance：计划、预通知、开始/结束状态。
- Subscribers：验证状态、组件过滤、暂停/恢复。
- Delivery queue：失败原因、重试和目的地健康。
- Settings：阈值、功能开关、Discord 测试和密钥状态。
- Audit log：管理员和自动化状态变更。

用户可见文案遵循现有 8 语言 i18n 规则；后端错误使用 en/zh go-i18n。

## 18. 故障降级

- 真实流量采集不可用：探针接管；证据最终过期为 Unknown。
- 探针凭据或配置错误：记录 monitoring fault，不判定模型 Outage。
- 调度器节点死亡：lease 过期后由其他节点接管。
- DB 写入失败：不发布内存中的新状态；API 保持最后持久状态并显示 stale。
- 发布事务失败：事故 update 和 outbox 均不提交。
- Website 到 Console API 失败：显示缓存快照和 stale 时间，不显示全绿。
- Cache invalidation 失败：短 TTL 最终收敛。
- 维护结束仍有故障：立即恢复 observed 状态并生成事故草稿。

## 19. 测试策略

### 19.1 Go 单元测试

- availability outcome 分类表。
- 足量流量、低流量、信号冲突、Unknown 和恢复防抖。
- 五分钟、小时、日聚合与 90 天可用率/覆盖率。
- 探针调度间隔和 monitoring fault。
- incident state machine、append-only publish 和 override expiry。
- AES-GCM key version、HMAC 签名、验证/退订 token。
- SSRF：私网、DNS rebinding、redirect、端口和响应上限。
- outbox retry 分类、逻辑幂等和 CAS 领取。

### 19.2 数据库与并发测试

- SQLite、MySQL、PostgreSQL migration 与唯一约束。
- 两个调度器竞争同一 lease。
- 多 worker 竞争 outbox，不创建重复逻辑投递。
- fencing token 阻止过期调度器提交。
- 管理员 version conflict 返回 409。

### 19.3 HTTP 与端到端测试

- 公开 API 缓存、隐私字段和参数边界。
- Admin/Root 权限与 secure verification。
- 失败探针 -> 红色组件 -> 自动草稿 -> 管理员发布 -> 三类通知 -> 恢复草稿 -> 发布 resolved。
- 维护发布、自动开始、自动结束和结束后故障。
- Website 代理失败和 stale UI。

### 19.4 前端测试

- Router 置顶、模型搜索/过滤、Unknown/Maintenance/retired。
- 模型 90 天历史与事故关联。
- 管理端草稿、409 冲突、覆盖过期和权限。
- 键盘、文本状态、移动端和 locale 路由。
- `website` 与 `web/default` typecheck、lint、build。

### 19.5 性能守门

- Relay 热路径不新增同步 DB/网络调用。
- perf_metrics 新计数的 benchmark 吞吐回归不超过 5%。
- 公开 summary 在缓存命中时不执行逐模型 N+1 查询。

## 20. 上线与回滚

### 20.1 功能开关

- `STATUS_CENTER_ENABLED`
- `STATUS_CENTER_PUBLIC_ENABLED`
- `STATUS_CENTER_NOTIFICATIONS_ENABLED`
- `STATUS_CENTER_SHADOW_MODE`

### 20.2 顺序

1. 部署 additive migration 与关闭状态的功能代码。
2. 影子模式采集至少 7 天，校准误报、覆盖率、调度 lag 和数据库规模。
3. 开放管理员预览至少 48 小时。
4. 开放公开状态页，通知仍关闭。
5. SMTP、Discord challenge/test、通用 Webhook challenge 全部通过后开放订阅。

历史从新口径启用时间开始自然填充，启用前显示 Not monitored，不回填旧成功率。

### 20.3 部署目标

- Router deploy：required。Relay 可用性计数和真实 Router canary 路径受影响。
- `newapi-console`：required。调度器、状态 API、管理 API、通知 worker 和 migration。
- `newapi-web`：required。公开状态页和同源代理。
- `web/default`：随 Console 镜像发布管理 UI。
- Terraform/Cloudflare：默认不需要；所有跨应用 origin 使用现有环境变量。

### 20.4 回滚

关闭 public、notifications 和 scheduler 开关，保留 additive 表和只读历史。若公开页面已经上线，显示最后快照与 monitoring unavailable，而不是删除页面或继续显示绿色。

## 21. 可观测性

管理端和内部日志至少暴露：

- scheduler lease holder 与剩余时间
- component sync lag
- evaluator lag
- probe queue depth、延迟、结果分类和成本计数
- Unknown 模型数量与 coverage 分布
- rollup lag
- incident draft 数量
- outbox depth、oldest age、retry rate、suspended destinations
- encryption key 配置健康状态

## 22. 完成标准

当且仅当以下条件全部满足，功能才算完成：

- Router 和每个官网公开模型都具有诚实的五态当前状态。
- 高流量模型以真实流量为权威，低流量模型按批准间隔探针。
- 20 分钟无可信证据必为 Unknown。
- 90 天可用率与覆盖率按 approved formula 计算，Unknown 不进入可用率分母。
- 每个模型有公开历史详情和关联事故。
- 管理员可完成事故、维护、覆盖、审计和冲突处理。
- 邮件、Discord、通用 Webhook 通过验证、签名、SSRF 和 outbox 测试。
- 多节点 lease、fencing、幂等和 CAS 测试通过。
- SQLite、MySQL、PostgreSQL migration 与核心查询通过。
- Go 定向测试、race 测试、两个前端 typecheck/lint/build 和关键 E2E 通过。
- Relay 性能守门通过，公开 API 不泄漏内部或客户数据。
- 没有已知状态伪绿、重复逻辑事故、未说明失败或未记录验证缺口。
