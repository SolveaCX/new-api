# Browser QA 钉钉中文摘要设计

## 目标

将 staging Browser QA 的钉钉终态通知改为“中文为主、保留原始状态码”。非技术人员可以直接看懂测试是否完成、是否发现问题以及是否完成清理；技术人员仍能使用 `findings_detected`、`passed` 等原始值排查日志。

本次只改变通知文案与 AI finding 标题语言要求，不改变 QA 判定、告警策略、工作流拓扑、清理逻辑、Secret、Terraform、IAM、production 或 `main`。

## 选择方案

采用中文标签和中文解释，同时在括号中保留原始枚举值。例如：

```text
Staging 浏览器 QA：发现问题

测试流程已执行完成，但 AI 发现 1 个需要关注的问题；本次发布不会因此自动回滚。

- 最终状态：发现问题（findings_detected）
- 录制回放：通过（passed）
- AI 探索：通过（passed）
- 探索动作数：6
- 问题数量：1
- 账号清理：通过（passed）
- 运行记录：打开 GitHub Actions
- 证据文件：gs://.../manifest.json

发现的问题
- [高] Staging 网站的 `Sign in` 按钮跳转到 404 页面（置信度：高；页面：/）
```

不采用“完全中文且隐藏原始值”，因为状态码对排查和搜索仍有价值。不采用“保留英文模板、只增加一行中文说明”，因为主要字段仍然难以快速阅读。

## 状态映射

最终状态：

- `passed` → 全部通过
- `findings_detected` → 发现问题
- `replay_failed` → 回放失败
- `infrastructure_failed` → 测试基础设施失败
- `cleanup_failed` → 清理失败

阶段状态：

- `passed` → 通过
- `failed` → 失败
- `not_started` → 未开始
- `unknown` → 未知
- `cleanup_failed` → 清理失败

严重级别：`critical/high/medium/low` 分别显示为“严重/高/中/低”。置信度：`high/medium/low` 分别显示为“高/中/低”。所有映射都从现有闭集枚举生成，不接受任意输入。

## 终态说明

标题和第一段必须先解释业务含义：

- 全部通过：测试执行完成，未发现需要关注的问题。
- 发现问题：测试执行完成，AI 发现需要关注的问题；当前策略只告警、不自动回滚。
- 回放失败：录制流程没有走完，应检查失败步骤。
- 基础设施失败：测试环境、浏览器、任务或证据链异常，本次结果不能用于判断产品是否正常。
- 清理失败：测试结束但临时账号或资源未被确认清理，必须优先处理。

## Finding 文案

`qa-prompt.md` 要求 AI 将 finding 的 `title` 写成简体中文，同时保留 `Sign in`、API Key、URL、HTTP 状态码等必要产品或技术词。钉钉渲染层不调用额外翻译 API，也不尝试自行翻译任意模型文本，避免新增网络依赖、成本和翻译失败路径。

若上游仍返回英文标题，通知继续安全展示原文，但标题、严重级别、置信度、页面说明和整体结论仍为中文。现有 Markdown 转义和敏感信息拒绝规则保持不变。

## 安全与失败行为

- 通知仍只接收已脱敏 manifest 摘要。
- 不增加邮箱、验证码、密码、API Key、Cookie、Authorization、Webhook、签名 Secret 或私有 artifact 内容。
- 状态与严重级别翻译使用固定映射；缺少映射时失败关闭，不回显不可信输入。
- 钉钉发送、签名、重试、`errcode` 校验和最终 workflow gate 保持原行为。
- `findings_detected` 在 staging 继续只告警；回放、基础设施、清理或通知失败仍按现有策略失败。

## 测试与验收

按 TDD 先修改测试并确认当前英文实现失败，再做最小实现：

1. 成功、发现问题、回放失败、基础设施失败、清理失败五种最终状态均产生中文标题和中文说明，并保留原始状态码。
2. 回放、探索、清理、严重级别和置信度的所有允许枚举均有中文映射测试。
3. finding 保持 Markdown 转义、页路径脱敏和敏感标题拒绝行为。
4. `qa-prompt.md` 的容器契约测试要求 finding `title` 使用简体中文。
5. Browser QA 全量测试、workflow contract、actionlint 和 secret scan 通过。
6. 推送到 `staging` 后，以真实发布触发唯一 `normal` QA；无论最终是通过、发现问题还是失败，钉钉均收到中文摘要，且账号清理结果可见。

## 不在本次范围

- 不修复本次发现的 `Sign in` CTA 404 产品缺陷。
- 不把 `findings_detected` 改为阻断发布。
- 不接入额外机器翻译服务。
- 不部署 production 或合并 `main`。
