# Flatkey Staging Browser QA 设计

**日期：** 2026-07-31
**状态：** 已批准，实施中
**实施计划：** [docs/superpowers/plans/2026-07-31-flatkey-staging-browser-qa.md](../plans/2026-07-31-flatkey-staging-browser-qa.md)
**范围：** 将 Mac Record & Replay 生成的 onboarding Skill 运行在 Google Cloud，对 Flatkey staging 做一次可重复的语义回放和有边界的 AI 发散测试

## 1. 目标

建立一条与现有发布链路隔离的 staging 浏览器 QA：由 GitHub Actions 手动触发 Cloud Run Job，在无界面的 Chromium 中运行 `codex exec`、Playwright MCP 和仓库内 Skill，完成新用户注册、邮箱验证、免费额度结果检查、API Key 检查以及文档入口检查；核心流程完成后，Codex 在固定时间和操作预算内继续探索可能的 UI、Console、网络与恢复路径问题。

第一阶段的完成标准不是接入生产发布，而是证明下面这条闭环能够稳定运行：

1. 手动触发一次测试。
2. 语义化 replay 能适应按钮文案、位置和页面跳转的小幅变化，而不是机械复现坐标。
3. AI 能基于现场观察提出并验证额外 bug 假设。
4. 测试报告、截图及经过脱敏的 Console/网络证据进入私有 GCS。
5. 无论测试成功、失败、Codex 中断或主 Job 超时，都执行进程内清理和独立 cleanup execution，删除测试账号拥有的全部 API Key，并删除测试账号。
6. GitHub Actions 明确显示成功或告警，但不阻断任何生产发布。

## 2. 已选方案与备选方案

### 选定：GitHub Actions → Cloud Run Job

GitHub Actions 只负责构建、更新三个 QA runtime 的镜像、启动 Jobs 并等待最终状态。浏览器、Codex、Skill、预算控制、证据采集和进程内清理均在主 Cloud Run Job 中完成；broker 只提供受限验证码能力，cleanup Job 只负责幂等回收和验证。

选择它的原因是运行环境可复现、按需计费、与个人电脑无关，并且以后可在不改测试内核的情况下接到告警或发布前检查。

### 未选：GitHub-hosted runner 直接跑浏览器

优点是首版文件较少；缺点是 Gmail、OpenAI 和测试账号相关能力会直接落入 CI runner，执行环境与浏览器缓存也更难长期固定。它适合作为短期 smoke test，不适合作为本方案的凭据和证据边界。

### 未选：Windows 或 Mac 本机定时运行

优点是能复用已登录的桌面浏览器；缺点是机器必须在线，录制端与执行端耦合，难以作为团队可审计的 staging 基础设施。Mac 仍可继续录制 Skill，但不是 replay 的运行前提。

## 3. 范围和非目标

### 本阶段包含

- 目标站点：
  - `https://staging-website.flatkey.ai`
  - `https://staging-console.flatkey.ai`
- 只读验证文档链接时，可在一个不携带 staging Cookie 的独立浏览器上下文打开 `https://docs.flatkey.ai`。
- 邮箱/密码注册、六字符验证码、首次登录、免费测试额度结果、API Key 结果和 Quickstart/OpenAI SDK 文档入口。
- 核心 replay 之后的 bounded exploration。
- 私有 GCS 报告与 GitHub Actions 告警状态。
- API Key 和测试账号清理。

### 本阶段不包含

- 不访问或修改 `flatkey.ai`、`console.flatkey.ai`、`router.flatkey.ai` 的生产数据。
- 不接入 `main` 分支部署、生产 Environment 审批或生产流量门禁。
- 不自动批准发布；当前只有告警结果。
- 不测试充值、订阅、邀请奖励套利、真实模型调用或任何会产生不可逆费用的操作。
- 不把个人 Gmail OAuth 文件、refresh token、OpenAI API Key、一次性密码、验证码或完整 Flatkey API Key 写入仓库、镜像、日志或报告。
- 不在本阶段增加数据库级硬清理或管理员凭据。账号清理采用产品现有的自助删除语义，并验证账号不能再次登录。

## 4. 总体架构

Task 6 browser ownership update: the QA supervisor owns the single Chromium process for the run. It starts Chromium after preflight through the deny-by-default proxy, exposes only a loopback `http://127.0.0.1:<port>` CDP endpoint from the private runtime profile, and then starts Codex. Playwright MCP attaches to that endpoint; it does not launch its own browser.

```text
GitHub workflow_dispatch
  ├─ WIF 登录 Google Cloud
  ├─ 构建并推送固定版本 QA 镜像
  ├─ 仅更新 QA broker Service 和两个 QA Jobs 的镜像
  ├─ gcloud run jobs execute browser-qa --wait
  └─ always：gcloud run jobs execute browser-qa-cleanup --wait
       │
       ├─ QA supervisor
       │    ├─ 生成一次性 username / Gmail plus alias / password
       │    ├─ 启动 supervisor-owned Chromium；Playwright MCP 仅通过 CDP attach
       │    ├─ 调用非交互 codex exec
       │    ├─ 约束阶段、时间、操作与 origin
       │    ├─ 收集并脱敏证据
       │    └─ finally：删除全部 Key 和测试账号
       │
       ├─ Codex
       │    ├─ 加载 .agents/skills/flatkey-new-user-onboarding
       │    ├─ 阶段 A：语义 replay
       │    └─ 阶段 B：bounded exploration
       │
       ├─ 私有 verification-code broker
       │    └─ Gmail readonly → 只返回本次 alias 的验证码
       │
       └─ 私有 GCS report bucket
            └─ manifest / JSON / screenshots / sanitized logs
```

浏览器 QA 是一次性、凭据敏感、会产生工件的操作型 Job，不复用现有 `newapi-staging` 或 `newapi-web-staging` 服务，也不加入它们的 deploy workflow。QA 资源元数据仍由 `deploy/gcp/envs/prod` 这一现有 Terraform root/state 管理，但报告 artifacts 使用新的专用 bucket，绝不写入 Terraform state bucket。

## 5. 仓库落点

计划中的主要落点如下；具体任务拆分由后续 implementation plan 定义：

- `.github/workflows/gcp-browser-qa.yml`：仅 `workflow_dispatch` 的薄工作流。
- `.agents/skills/flatkey-new-user-onboarding/`：录制所得 Skill 及其 agent metadata。
- `scripts/browser_qa/`：supervisor、Codex prompt、Playwright 配置、报告 schema、只会调用私有 broker 的 MCP client、清理和脱敏逻辑；client 不包含 Gmail SDK，也不能接收 OAuth material。
- `scripts/browser_qa/Dockerfile`：固定 Codex、Node、Playwright 和 Chromium 版本；同一镜像用不同 entrypoint 运行 broker 或 QA Job，只复制 QA 所需文件，不复制应用仓库或本机配置。
- `deploy/gcp/envs/prod/browser_qa.tf`：staging-only QA Job、独立 cleanup Job、私有 broker、专用身份、Secret/IAM 元数据和报告 bucket。
- `deploy/gcp/envs/prod/variables.tf` 与 `terraform.tfvars`：独立 `enable_browser_qa` 开关和非敏感配置。
- `deploy/gcp/docs/OPERATIONS.md`：首次授权、Secret 轮换、手动执行、报告定位和清理失败处理。

Terraform 只管理 Secret 容器，不管理 Secret 版本内容，避免敏感值进入 state。两个 QA Jobs 和 broker Service 镜像由 CI 拥有，Terraform 对这三个镜像字段采用与现有 deploy 模式一致的生命周期忽略，避免下次 plan 回滚运行版本。

## 6. 身份和密钥边界

### Codex 认证

无人值守的 `codex exec` 使用专门的 OpenAI API Key，而不是桌面 Codex/ChatGPT 的持久登录文件。API Key 作为 Secret Manager secret 在运行时注入，并只用于本次非交互执行。它与用户的 ChatGPT 订阅是两套计费与认证路径。

另有一个随机的 QA identity seed 存入 Secret Manager。supervisor 和 cleanup Job 使用 `HMAC-SHA256(seed, github-run-id)` 确定性派生本次 username nonce、Gmail plus alias nonce 和符合产品规则的伪随机密码。Codex 只能看到当前一次性身份，看不到 seed。这样主 Job 被强制终止后，cleanup Job 或人工 cleanup-only dispatch 仍可从 run id 重算同一身份，不需要把密码持久化。

### Gmail 认证

个人 Gmail 当前仅作为 staging 首次闭环的邮箱。OAuth scope 保持 `gmail.readonly`。OAuth client 信息和 refresh token 只挂载到独立、私有的 verification-code broker，绝不挂载到 QA Job 或 Codex 容器。

Cloud Run Job sidecar 虽然可行，但同实例容器共享网络，不能作为 Gmail 个人邮箱的强隔离边界。因此 broker 使用单独的私有 Cloud Run Service：

- 不允许匿名调用。
- 只有 QA runtime service account 可以 invoke。
- broker runtime service account 只可读取 Gmail OAuth secret 和写日志。
- broker API 只接受符合本次 QA 前缀、有效时间窗和高熵 nonce 的 plus alias。
- Gmail 查询同时约束精确收件 alias、Flatkey 发件/主题特征和本次开始时间。
- broker 解析候选邮件正文，但对调用方只返回匹配模板的六字符验证码，不返回主题、正文、发件人列表或其他邮件内容。
- broker 和 client 禁止记录验证码及 OAuth 响应。

Codex 只看到一个无参数的 `get_current_verification_code` MCP tool；alias 由 supervisor 固定，tool 配置与实现位于非 root 运行用户不可修改的只读路径，模型不能传入或改写任意邮箱进行查询。broker URL 和调用凭据不进入 prompt，浏览器也不具备 broker 的 OIDC 身份。

Google OAuth 应用目前处于 `Testing`，外部应用的 refresh token 可能约 7 天失效。首次闭环可使用现有 token；跑通后必须将 OAuth 应用发布为 `In production`，重新授权一次并轮换 Secret Manager 中的 refresh token。即使发布后，撤销授权、长期未使用、密码变化或 Google 策略仍可能使 token 失效，因此 `invalid_grant` 被归类为明确的基础设施告警，而不是无限重试。

### GCP 身份

- QA runtime SA：只能读取 Codex API secret、QA identity seed、调用 broker、以 object creator 身份写入自己的报告 bucket 和写日志；不能覆盖或删除已上传工件。
- Broker runtime SA：只能读取 Gmail OAuth secret 和写日志。
- Cleanup runtime SA：只能读取 QA identity seed、读写专用报告 bucket 中本次 run 的 cleanup/manifest 结果和写日志；不能读取 Codex 或 Gmail secret。它的 object admin 权限只作用于专用 QA bucket，不作用于其他 bucket。
- 专用 QA WIF deployer SA：通过 repo/ref 条件绑定到目标仓库的 `staging` ref，只在 QA broker Service、两个 QA Jobs、QA runtime identities 和目标 Artifact Registry repository 上获得所需权限；它拿不到运行时 Secret 值，也不复用现有应用 deployer SA。
- 报告 bucket 使用 uniform access、public access prevention 和 14 天生命周期删除。

## 7. 一次性测试身份

每次执行从 QA identity seed 和 GitHub run id 确定性派生独立身份：

- username：`qa` + 短 run id + 随机 nonce，保持在产品长度限制内。
- email：个人 Gmail 的 `+flatkey-qa-<run-id>-<nonce>` alias。
- password：20 位伪随机字符串，满足注册校验，只在本次 supervisor/Codex/cleanup 进程内存中流转，不落盘或进入报告。
- API Key name：`cloud-qa-<run-id>`。

这些值只授权当前 staging run。现有 Skill 中“让用户接管密码输入、创建 Key 前再次确认、提交条款前再次确认”等面向真实用户的保护，继续保留在通用 Skill；云端 QA policy 则明确声明：用户已预授权在两个 staging origin 上使用一次性身份完成注册、接受 staging 条款和创建临时 Key。该授权不延伸到生产、付费行为或其他账号。

## 8. 执行状态机

### 8.1 Preflight

supervisor 首先检查 `/api/status`，要求：

- 当前 origin 是 staging。
- registration 和 password registration 已启用。
- `email_verification=true`。
- 首版要求 `turnstile_check=false`；若开启则 fail closed，并提示需要人工挑战方案，不尝试绕过 CAPTCHA。
- staging 必须允许 Gmail `+` alias。该设置当前不在 `/api/status` 暴露，因此首次部署前由操作者核对 `EmailAliasRestrictionEnabled=false`；运行中若发送验证码返回 alias restriction 错误，则归类为明确的 staging 配置失败。
- broker、GCS 和 Codex 认证可用。

主 Cloud Run Job 固定 `task_count=1`、`parallelism=1`、`max_retries=0`，Cloud Run 硬超时 20 分钟。supervisor 的内部 deadline 为 14 分钟，之后强制停止 Codex；预留最多 3 分钟做进程内清理、2 分钟上传报告、1 分钟余量，并处理 SIGTERM 立即转入清理。cleanup Job 硬超时 5 分钟、无自动重试，由 workflow 或 cleanup-only 显式重跑。GitHub workflow 使用单一 concurrency group，避免两个账号流程竞争验证码或注册限流。

### 8.2 阶段 A：语义 replay

Codex 显式调用 `$flatkey-new-user-onboarding`，但以目标结果而非坐标或固定文案执行：

1. 从 staging website 的 CTA 到 staging console 注册页。
2. 填写一次性身份并发送验证码。
3. 通过受限 MCP tool 获取本次 alias 的验证码并完成注册；broker client 每 5 秒轮询一次，最多等待 120 秒，超时后不自动重发第二封邮件。
4. 验证自动登录和新用户落地页。
5. 验证免费测试额度结果。当前产品可能在注册或首次 Key 创建时自动发放，Skill 不强制寻找一个已经不存在的独立“领取”按钮。
6. 验证 API Key 页面。后端当前会在注册时幂等创建默认 Key；测试仍创建或确认一个带本次 run id 的 Key，并记录其内部 id，但不复制、输出或截图完整 secret。
7. 验证 Quickstart/OpenAI SDK 文档入口；若打开 `docs.flatkey.ai`，使用无 Cookie 的独立上下文并保持只读。

任何核心结果失败都将 replay 标记为失败。Codex 可以在剩余 replay 预算内做语义恢复，例如重新定位同义按钮、刷新一次或返回上一步，但不自动创建第二个账号。

### 8.3 阶段 B：有边界的 AI 发散探索

只有核心账号流程已到达可清理状态时才开始发散探索。预算为最多 5 分钟或 30 个浏览器 tool actions，以先到者为准。模型从观察到的页面、Console 和网络信号形成小型假设队列，优先覆盖：

- 页面文案或路由变化导致的 onboarding 断链。
- 表单验证、返回/刷新、重复点击和加载态恢复。
- 中文/英文语义映射和明显的布局、遮挡、空状态问题。
- 未处理的 JavaScript exception、严重 Console error。
- staging 同源 API 的 4xx/5xx、失败资源和异常重试。
- API Key 列表、文档入口及新用户落地页之间的导航一致性。

探索禁止付费、真实 API 调用、邀请行为、管理员页面、修改全局设置、访问其他用户数据以及任何生产写操作。核心 replay 失败时，只允许做与该失败直接相关的诊断和证据采集，不进入广泛探索。

## 9. 浏览器和网络控制

- Chromium is supervisor-owned and launched with `--remote-debugging-port=0`, a runtime-local user data directory, the supervisor proxy, loopback proxy bypass removal, QUIC disabled, and non-proxied WebRTC disabled. Playwright MCP is fixed to `/usr/local/bin/playwright-mcp` and is invoked only with CDP attach arguments (`--cdp-endpoint`, timeout, and per-run output directory). MCP launch/browser/proxy/headless flags are not trusted for the already-running browser.
- Codex does not receive `browser_take_screenshot`. It receives `qa_capture_screenshot`, a restricted evidence MCP tool that accepts only a logical screenshot name. A supervisor-owned Node helper connects to the same CDP browser, masks inputs, textareas, contenteditable fields, verification-code fields, and known in-memory sensitive values, receives the Playwright screenshot as a Buffer, and writes the masked PNG privately under `screenshots/` with exclusive create semantics. The model cannot set paths, selectors, filenames, or disable masking.
- The same helper keeps bounded console and network event buffers in memory. Raw browser events and Playwright MCP `playwright-output` are temporary only; before artifact upload, supervisor writes redacted `browser/console.jsonl` and projected `browser/network.jsonl` and then removes `playwright-output` in `finally`.
- 使用固定版本 Playwright/Chromium；Chromium 由 supervisor 启动，MCP 不携带 launch/headless flags，运行时不执行 `npx ...@latest`。
- `codex exec` 使用非交互、ephemeral、JSON 事件流和最终 output schema；忽略本机用户配置，Skill、模型名和 QA policy 均来自镜像中的版本化配置，workflow 不接受任意模型输入。模型变更必须通过 PR，并记录在报告中。
- Codex shell 保持 workspace-write 且禁用 shell 子进程网络；启动自检必须证明 shell 对任意外部 URL 和 Cloud metadata 均无法直连。Codex 主进程只保留调用 OpenAI 所需连接，页面操作通过 Playwright MCP 完成。
- 顶层导航只允许两个 staging origin 和只读 docs origin。
- Chromium 强制使用 supervisor 管理的 outbound proxy，关闭 QUIC、非代理 WebRTC 和 service worker 注册；代理和 Playwright route 双层阻止生产 Flatkey origin、localhost、link-local metadata、RFC1918 和非 HTTP(S) 导航。策略覆盖 document/subresource、redirect、websocket、popup、download 和新 browser context，浏览器进程不得直连绕过 proxy。
- staging 页面所需的第三方静态资源 host 来自版本化 allowlist；未知 host 默认阻止并记录为观察项。Authorization、Cookie、Set-Cookie、请求体、query secret 和 API Key 字段在落盘前统一脱敏。
- 页面内容被视为不可信数据，不能覆盖 QA policy、origin 限制、预算或清理规则。
- broker MCP client 运行在 Codex sandbox 外，只能调用一个由只读配置固定的私有 broker endpoint 和当前 alias；报告上传在 Codex 退出后由 supervisor 完成。

## 10. 确定性清理

清理使用两层机制，均不依赖 Codex 最后一条消息，也不要求管理员权限：主 Job supervisor 在 `finally`/SIGTERM handler 中立即清理；GitHub Actions 在主 Job 之后用 `if: always()` 启动独立 cleanup Job，并以同一 GitHub run id 重算身份，再做一次幂等清理和验证。只有 cleanup Job 返回成功，工作流才可能为绿。

1. 用内存中的一次性 username/password 对 staging console 重新登录。
2. 用该用户 session 分页枚举 `/api/token/` 下的全部 Key，包括注册时自动创建的默认 Key；持续读取页数据和 total，直到没有未处理 id，不能只检查第一页。
3. 删除每个 Key，并再次列举确认结果为空。删除返回 404 时按“已不存在”处理。
4. 调用产品现有的 `DELETE /api/user/self`。
5. 清理 Cookie 后再次尝试登录，确认账号不再可用。

网络 5xx 或连接中断可做最多三次有界重试。若 Codex 声称账号已创建但两层清理都无法证明 Key 与账号已清理，最终状态强制为 `cleanup_failed`，即使功能测试本身通过也让 GitHub Actions 变红。报告只保留可定位的 run id 和确定性 username，不保存密码、验证码或完整邮箱。

若主 Job 被 Cloud Run 硬终止，GitHub 仍执行 cleanup Job；若整个 GitHub workflow 被人工取消或平台故障，则运维 runbook 要求用原 GitHub run id 手动触发 `cleanup-only`，它会重算身份并执行同一流程。任何系统都不能对平台级强制终止提供绝对的同步 `finally` 保证，因此验收依据是独立 cleanup execution 和可重复的 cleanup-only 恢复路径，而不是仅依赖进程语义。

当前自助删除是产品定义的账号删除语义，底层可能保留软删除审计行；数据库级 hard purge 和管理员 janitor 不属于首版。若 staging 长期高频运行导致软删除数据积累，再单独设计前缀受限的 janitor。

## 11. 报告和告警

Artifact upload is allowlisted by runtime-root relative POSIX paths only: root `result.json`, `codex-events.jsonl`, `codex-stderr.txt`, and `manifest.json`; one-level `screenshots/*.png`; and exact `browser/console.jsonl` plus `browser/network.jsonl`. Object names preserve those nested paths, content types are exact, symlinks and duplicate logical/real paths are rejected, and `manifest.json` is uploaded last. The temporary `playwright-output` directory is never uploaded and is deleted after Codex/MCP shutdown on success, nonzero exit, timeout, signal, and exception paths.

每次 workflow 使用根前缀 `runs/<github-run-id>/`。主 Job 写入 `main/<execution-id>/`，cleanup Job 写入 `cleanup/<execution-id>/cleanup.json`，并在根前缀创建或更新最终 `manifest.json`；cleanup-only 重跑会保留新的 execution 记录并刷新最终 manifest：

- `manifest.json`：版本、目标、阶段状态、预算消耗、清理结果和 artifact 索引。
- `codex-events.jsonl`：经过脱敏的 Codex 事件流。
- `result.json`：符合固定 JSON Schema 的 replay、exploration 和 finding 结果。
- `screenshots/`：关键节点和失败现场；捕获前对个人 Gmail base address、完整 plus alias、密码、验证码、Cookie 和完整 Key 所在 DOM 区域应用遮罩，原始未遮罩截图不落盘。
- `browser/console.jsonl`：脱敏后的 Console 事件。
- `browser/network.jsonl`：URL、method、status、timing 和脱敏错误，不保存敏感 header/body。

finding 至少包含 severity（`critical|high|medium|low|info`）、标题、目标 URL、复现步骤、期望/实际、证据路径和置信度。日志 redactor 将个人 Gmail base address 和完整 plus alias 替换为 run-scoped 占位符；AI 的叙述不能决定最终清理状态，manifest 中的 cleanup 由 supervisor/cleanup Job 覆盖。`info` 只作为观察记录，至少一个已证实的 `low` 及以上 finding 才进入 `findings_detected`。

首版告警载体是 GitHub Actions 的红/绿状态和 Job Summary，摘要只显示 replay、exploration、finding 数量、cleanup 状态及私有 GCS URI。它不接生产审批，也不使用 Gmail 发送告警。后续可将同一 manifest 接到邮件、Slack、钉钉或发布前只告警检查。

## 12. 失败处理

最终状态按优先级归类：

1. `cleanup_failed`：最高优先级，需要人工检查 staging 残留。
2. `infrastructure_failed`：Codex/Gmail OAuth/broker/浏览器/GCS/Cloud Run 配置失败。
3. `replay_failed`：核心 onboarding 结果未达成。
4. `findings_detected`：核心通过，但发散阶段发现问题；首版告警但不阻断发布。
5. `passed`：核心通过、报告上传且清理已验证。

不论前面发生哪类失败，都先尝试截图、导出脱敏日志和执行清理，再决定 exit code。主 Job 退出后，GitHub 保存其状态并始终执行 cleanup Job，再综合两个结果决定最终状态。两个 Job 都必须使用 `gcloud run jobs execute --wait`，避免把“成功启动”误判为“测试通过”。

## 13. 基础设施安全边界

- QA 资源使用独立命名、QA/broker/cleanup runtime SAs、专用 WIF deployer SA、report bucket 和开关。
- 不修改现有 Cloud Run services、LB、证书、域名、流量或生产 Environment。
- Terraform plan 必须证明只新增/修改 browser QA 资源；若出现 `newapi-web`、`newapi-console`、`newapi-router`、URL map、certificate 或 traffic diff，禁止 apply。
- 仓库现有 `gcp-infra.yml` 因 IAM 缺口不能作为可信的 apply 路径。首次资源创建使用有刷新能力的本地 Owner ADC 生成并审阅 plan，再按运维手册执行；不得用 `-refresh=false` 结果代替 live drift 检查。
- Secret 值由操作者通过安全输入写入 Secret Manager，禁止通过 Terraform variable、GitHub log、命令历史中的明文参数或提交文件写入。
- WIF impersonation 条件和资源级 IAM 必须有测试，证明非 `staging` ref 或非目标仓库不能冒充 QA deployer，QA deployer 也不能更新现有应用 Cloud Run services。

## 14. 测试策略

### 单元和契约测试

- Gmail parser：精确 alias、时间窗、发件/主题模板、MIME 解析、六字符提取和无关邮件拒绝。
- Broker：IAM 拒绝、无效 alias、pending、timeout、`invalid_grant` 和日志脱敏。
- Origin policy：允许 staging/read-only docs，拒绝生产、metadata 和内网地址。
- Budget controller：时间或 action 任一到达即停止发散。
- Report redactor：个人 Gmail base address、完整 plus alias、Cookie、Authorization、验证码、密码和完整 API Key 不得进入 artifacts。
- Cleanup state machine：账号不存在、部分 Key 已删、多页 Key、DELETE 响应丢失、5xx 重试、主 Job 超时、独立 cleanup 重入和清理失败优先级。
- Codex output schema：缺字段或非法 severity 必须使结果校验失败。

### 容器和工作流测试

- 容器内 `codex --version`、Playwright/Chromium 和 MCP CDP attach smoke test。
- 镜像以非 root 用户运行，且不包含本机 OAuth 文件、`auth.json` 或 Secret 值。
- workflow lint；确认只有 `workflow_dispatch`，主/cleanup 执行命令都包含 `--wait`，cleanup 使用 `if: always()`，并发为 1。
- egress smoke：shell 直连、生产 origin、metadata、popup、redirect、service worker、websocket 和绕过 proxy 的连接均被拒绝；staging 与无 Cookie docs 仍可用。

### Terraform 验证

- `terraform fmt -check`、`terraform validate`。
- 有刷新 plan 只出现两个 QA Jobs、broker、IAM、Secret metadata 和报告 bucket。
- 自动审计 plan 中不存在现有服务、LB、证书、DNS 或 traffic 资源地址。

### Staging 验收

1. 关闭探索预算跑一次核心 replay。
2. 开启 5 分钟/30 action 探索跑一次完整流程。
3. 验证 GCS manifest、截图、Console/网络日志和 GitHub Summary 一致。
4. 验证报告中不存在 secret，并确认账号及全部 Key 已删除。
5. 人为使 broker token 无效，确认得到明确 infrastructure 告警且仍执行清理。
6. 强制终止主 Job，确认独立 cleanup Job 仍删除账号和跨多页的全部 Key；再用同一 run id 运行 cleanup-only，确认幂等。

干净的 `origin/staging` 在 Windows 上当前全量 `go test ./...` 已存在与本功能无关的失败，包括缺少 `web/classic/dist`、SQLite 文件锁和若干既有 package tests。实施时记录该基线，不以“修复全仓既有失败”为本功能范围；新增的定向测试、容器 smoke、Terraform 验证和 staging E2E 必须全部通过。

## 15. 验收标准

- GitHub Actions 可以手动启动主 QA execution，并无条件启动 cleanup execution，且等待两者的真实完成状态。
- 运行使用仓库 `.agents/skills/flatkey-new-user-onboarding`，报告包含 Skill 内容 hash、Codex 版本、模型配置和 Playwright/Chromium 版本。
- 核心 onboarding 每一步都有结果断言和证据，不依赖录制坐标。
- 探索严格遵守 5 分钟/30 action 预算，并输出可复现 findings。
- Codex 无法获得 Gmail OAuth secret，也无法要求 broker 查询任意邮箱。
- 生产 Flatkey origin 没有请求；docs 仅在无 Cookie 上下文只读访问。
- GCS artifacts 私有、脱敏并在 14 天后自动删除。
- staging 已确认允许 Gmail plus alias；若配置禁用则在账号创建前给出明确配置失败。
- 正常、核心失败、Codex 中断、主 Job 硬超时和报告失败路径都由独立 cleanup execution 兜底；只有账号及全部分页 Key 清理验证成功，run 才能离开 `cleanup_failed`。
- 第一次 `Testing` token 跑通只算 bootstrap；OAuth 应用发布为 `In production`、重新授权并轮换 Secret 后，必须再完成一次全流程，才能声明“可重复运行”。
- 当前方案不改变或阻断任何生产发布路径。

## 16. 官方运行契约依据

- [OpenAI Codex non-interactive mode](https://learn.chatgpt.com/docs/non-interactive-mode) 与 [authentication](https://learn.chatgpt.com/docs/auth)：`codex exec`、ephemeral、JSON/schema output 和 CI API-key auth。
- [OpenAI Codex skills](https://learn.chatgpt.com/docs/build-skills)：当前仓库级 Skill 发现路径为 `.agents/skills`。
- [Playwright MCP](https://playwright.dev/docs/getting-started-mcp)、[Docker](https://playwright.dev/docs/docker) 与 [CI](https://playwright.dev/docs/ci)：Linux 容器固定浏览器依赖，MCP 仅 attach 到 supervisor-owned Chromium。
- [Google Cloud Run Jobs execute](https://docs.cloud.google.com/run/docs/execute/jobs) 与 [container contract](https://docs.cloud.google.com/run/docs/container-contract)：`execute --wait`、任务成功语义和容器 exit code。
- [Google OAuth 2.0](https://developers.google.com/identity/protocols/oauth2)：External/Testing refresh token 的七天限制和常规撤销/过期条件。

实现时以当日官方文档和固定工具版本再次校验这些契约；若 `.agents/skills` 与已固定 Codex CLI 版本不一致，优先升级或固定到支持当前官方路径的 CLI，而不是同时维护两份 Skill。
