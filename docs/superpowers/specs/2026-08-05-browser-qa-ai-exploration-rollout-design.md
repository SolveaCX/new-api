# Browser QA AI 发散测试接入设计

**日期：** 2026-08-05
**状态：** 已确认，待实施
**依赖设计：**

- [Flatkey Staging Browser QA 设计](./2026-07-31-flatkey-staging-browser-qa-design.md)
- [Browser QA 钉钉通知设计](./2026-08-05-browser-qa-dingtalk-notification-design.md)

## 1. 目标

在现有 staging 发布后 Browser QA 中启用受控的 AI 发散测试。每次运行仍先完成录制 Skill 的语义回放；只有 replay 到达可清理 checkpoint 后，AI 才能在同一个测试账号和同一组临时资源上探索相邻异常路径。

第一阶段的 AI finding 只发送钉钉告警，不阻断 staging 发布。replay、基础设施、报告校验或 cleanup 失败仍然让 GitHub Actions 失败。生产发布、生产域名和生产数据不在本设计范围内。

## 2. 当前状态

现有 Browser QA 已经具备以下基础能力：

- `core`、`normal` 和 `cleanup-only` 三种运行模式；
- replay checkpoint 与 `qa_start_exploration` 阶段门；
- 5 分钟和 30 次 Playwright action 的发散预算；
- staging 可写域名、只读文档域名和 deny-by-default 网络策略；
- run-scoped 测试身份、验证码 broker、脱敏证据、GCS manifest；
- 主进程 cleanup 与独立 cleanup Job；
- 成功和失败终态的钉钉通知。

staging 发布后当前固定调用 `core`，所以录制回放完成后会停止，`exploration.status` 为 `not_started`。本设计不新建第二套 QA 基础设施，而是分阶段把现有发布后调用切换到 `normal`。

`core` 与 `normal` 是严格的包含关系，不是两套彼此独立的测试：

```text
core   = replay -> checkpoint -> cleanup -> report
normal = replay -> checkpoint -> bounded exploration -> cleanup -> report
```

因此每一次 `normal` 都必须先完整执行 `core` 所代表的录制 Skill 语义回放。`normal` 只是在 checkpoint 之后增加发散阶段，不能跳过、缩短或替代 replay。

## 3. 选定方案

采用渐进切换方案：

1. 补齐发散策略、降噪、门禁、钉钉摘要和自动化测试。
2. 手动连续执行三次完整 `normal`；每次都包含 replay、checkpoint、发散、cleanup 和报告。
3. 三次均满足验收标准后，把 staging 发布后的 Browser QA 从 `core` 切换为 `normal`。
4. staging 使用 `fail_on_findings=false`，AI finding 只告警。

三次手动运行是自动接入前的稳定性验收，不代表 `normal` 是另一套独立测试。自动接入后，每次 staging 发布只启动一个 `normal` Job，由这个 Job 先完成 replay，再进入发散。

不采用 `core` 与 `normal` 双 Job 并行方案，因为它会重复注册、重复清理并增加运行时间和数据污染风险。也不采用仅手动或定时运行方案，因为它不能覆盖每次 staging 发布。

## 4. 数据流

```text
staging 部署和健康检查成功
  -> Browser QA normal
  -> 录制 Skill 语义 replay
  -> qa_replay_checkpoint
  -> 同账号、同资源的 AI bounded exploration
  -> 主进程 cleanup
  -> 独立 cleanup Job 复核
  -> 生成脱敏 root manifest
  -> 发送钉钉 PASSED / ALERT / FAILED
  -> 按门禁策略设置 GitHub Actions 结论
```

AI finding 不得跳过 cleanup，也不得提前结束报告链路。钉钉通知继续位于最终 GitHub 状态门禁之前。

## 5. 发散范围

### 5.1 允许探索

发散仅覆盖录制 onboarding 路径附近的页面和状态：

- 注册、验证码和首次进入控制台后的状态展示；
- onboarding 落地页；
- API Key 列表、现有 Key 状态和创建弹窗的非提交校验；
- Quickstart/OpenAI SDK 文档入口；
- 表单空值、长度、非法字符和错误提示；
- 返回、刷新、重新进入页面和重复非破坏性点击；
- 弹窗开关、按钮禁用状态、加载状态和空状态；
- 中文/英文切换后的路由、文案和明显布局问题；
- staging 同源 API 的异常 4xx/5xx、未处理 JavaScript exception 和核心资源失败。

### 5.2 禁止探索

- 不注册第二个账号；
- 不在 replay 已有资源之外创建额外 API Key；
- 不进入或修改付费、充值、订阅、邀请、管理员、组织、路由、模型、价格、安全或全局设置；
- 不调用真实模型；
- 不访问其他用户数据；
- 不访问生产 Flatkey 网站、控制台或 Router；
- 不绕过 CAPTCHA、Cloudflare 或 Turnstile；
- 不使用 shell、任意 web/search、route/mock、坐标点击或不受控代码执行。

发散只使用当前 run 的一次性账号。运行时应记录 replay checkpoint 时的临时 Key 集合；exploration 不得增加该集合。cleanup 仍枚举并删除账号拥有的全部 Key，避免仅依赖模型行为保证无残留。

## 6. AI 探索策略

AI 不执行随机点击。它从当前页面、Console、网络和 replay 结果形成一个有上限的假设队列，优先顺序如下：

1. 核心 replay 附近的状态恢复和导航问题；
2. 表单校验、重复操作和加载状态问题；
3. 同源 API 失败与未处理前端异常；
4. 多语言、空状态和明显 UI 一致性问题；
5. 仍有预算时才检查低风险的相邻路径。

每个假设先观察，再执行最小可复现操作。已证伪的假设不进入 finding。预算达到 5 分钟或 30 次 Playwright action 中的任一上限后，立即停止浏览器操作并输出报告。

## 7. Finding 质量和降噪

每条正式 finding 必须包含：

- `severity`；
- `title`；
- 不含 query/fragment/凭据的目标页面；
- `expected` 与 `actual`；
- `confidence`；
- 至少一个有效的截图、Console 或网络证据路径。

没有证据的问题不能成为正式 finding。同一页面、同一问题类型和同一证据指纹只保留一条。

已知第三方子资源噪声由运行时确定性分类，不交给模型自行决定。GTM、Mixpanel、客服脚本等非 allowlist host 被 Browser QA egress policy 主动拒绝时，默认记录为环境观察并降为 `info`，不作为产品 bug。只有当第三方失败能够复现并直接导致核心 staging 页面不可用时，才允许升级为产品 finding。

降噪发生在 manifest 和钉钉摘要生成前，但原始脱敏 Console/网络证据仍保存在私有 GCS 中用于审计。

## 8. 状态与门禁

最终 QA 状态优先级保持：

```text
cleanup_failed
> infrastructure_failed
> replay_failed
> findings_detected
> passed
```

工作流增加布尔输入 `fail_on_findings`：

- staging 自动发布和首轮手动 `normal` 验收使用 `false`；
- `findings_detected` 保留在 manifest 中，但 GitHub Actions 不因 finding 失败；
- `cleanup_failed`、`infrastructure_failed` 和 `replay_failed` 始终失败；
- 后续稳定后可以显式使用 `true` 开启 finding 门禁，不需要改变报告格式。

第一阶段不按 severity 阻断发布。所有有效 finding 都进入报告并发送钉钉，积累样本后再单独评审是否对 `critical/high/medium` 开启门禁。

## 9. 钉钉报告

钉钉终态分为：

- `PASSED`：replay、exploration、cleanup 均完成且没有有效 finding；
- `ALERT`：存在有效 finding，但 replay 和 cleanup 成功；
- `FAILED`：replay、基础设施、结果校验、cleanup 或钉钉发送本身失败。

报告继续包含 replay、exploration、action 数量、finding 数量、cleanup、GitHub Actions URL 和私有 GCS URI，并新增最多三条脱敏 finding 摘要。每条摘要只包含 severity、title、confidence 和去掉 query/fragment 的 staging 页面路径。

钉钉消息、GitHub Summary 和 root manifest 不得包含 Gmail base/alias、密码、验证码、Cookie、Authorization、完整 API Key、identity seed、webhook、signing secret 或签名。通知发送失败按现有规则进行有限重试；最终仍失败时，工作流失败，避免出现“测试结束但无人收到报告”的绿色状态。

## 10. Cleanup

cleanup 继续采用双保险：

1. supervisor 在 Codex 退出后的 `finally` 中删除测试账号拥有的全部 Key，再删除账号并验证无法登录；
2. 独立 cleanup Job 使用原 GitHub run id 重算同一身份，重复执行删除和登录拒绝验证。

AI narration 不能决定 cleanup 结果。若 finding 与 cleanup failure 同时出现，最终状态必须为 `cleanup_failed`。若整个 GitHub workflow 被平台强制取消，继续使用现有 `cleanup-only` 和原始 run id 恢复。

## 11. 实施影响面

实现计划预计只涉及以下 Browser QA 范围：

- `.github/workflows/gcp-browser-qa.yml`：增加 `fail_on_findings`、ALERT 门禁语义和安全输出；
- `.github/workflows/gcp-deploy-staging.yml`：三次手动验收后把调用模式从 `core` 切换为 `normal`，并传入 `fail_on_findings=false`；
- `.agents/skills/flatkey-new-user-onboarding/` 与 `scripts/browser_qa/config/qa-prompt.md`：明确假设队列、探索范围和禁止额外账号/Key；
- `scripts/browser_qa/flatkey_browser_qa/`：确定性第三方噪声分类、finding 去重、checkpoint 资源基线和报告状态；
- DingTalk sender：增加 `ALERT` 和最多三条安全摘要；
- `scripts/browser_qa/tests/`：预算、阶段门、安全边界、降噪、门禁、cleanup、钉钉和工作流契约测试。

本设计不需要新增 Terraform 资源、GCP IAM、Secret、Cloud Run Service/Job 或生产部署目标。

## 12. 验证和上线

### 12.1 自动化验证

实施完成后必须证明：

- `core` 在 checkpoint 后拒绝 exploration；
- `normal` 在 checkpoint 前拒绝 exploration；
- exploration 不超过 5 分钟或 30 actions；
- exploration 不增加 replay checkpoint 时的临时账号或 Key 集合；
- 已知 egress-denied 第三方资源只产生环境观察；
- 正式 finding 必须有完整证据并完成去重；
- `fail_on_findings=false` 只告警，其他失败状态仍变红；
- 钉钉正确渲染 `PASSED`、`ALERT`、`FAILED` 和前三条安全摘要；
- cleanup 删除全部 Key 和账号并验证登录拒绝；
- 所有报告载体通过敏感信息扫描。

### 12.2 三次手动 `normal` 验收

连续三次完整 `normal` 运行均必须满足：

- replay 为 `passed` 且 checkpoint reached；
- exploration 已启动且 actions 在 `1..30`；
- exploration 未增加 checkpoint 时的临时账号/Key 集合；
- cleanup 删除全部临时 Key 和账号，且删除后无法登录；
- 钉钉收到且分类正确；
- GitHub Summary、钉钉和 GCS manifest 无敏感信息；
- 已知第三方 egress 噪声未升级为产品 bug；
- 如果存在真实 finding，GitHub 保持非阻断，钉钉发送 `ALERT`。

### 12.3 自动接入 staging

三次连续验收通过后，才将 staging 发布后的单个 Browser QA 调用从 `core-only` 切换为完整 `normal`。切换后每次发布只执行一个 `normal` Job；该 Job 必须先完成 replay，再执行 exploration。切换后的首个发布触发运行必须再次核对 replay、exploration、cleanup、钉钉和 root manifest，再将自动接入视为完成。

## 13. 回退

如果发散出现不稳定、误报过多或执行时间不可接受，只把 staging 调用模式从 `normal` 改回 `core`。录制 replay、cleanup、GCS 和钉钉链路继续保留，不删除 GCP 资源，也不影响生产。

## 14. 非目标

- 本阶段不接入 `main` 或生产发布审批；
- 不让 AI 自动修改产品代码或自动回滚 staging；
- 不让 finding 自动创建外部工单；
- 不增加发散预算；
- 不开放更多 writable origin；
- 不在本阶段决定 severity 门禁阈值；
- 不实现第二账号、多账号并行或额外 API Key 压力测试。
