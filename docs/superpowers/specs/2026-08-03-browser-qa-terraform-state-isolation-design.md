# Browser QA Terraform State 隔离设计

**日期：** 2026-08-03

**状态：** 待书面审阅

**目标环境：** Flatkey staging

**不包含：** main、production 发布接入、AI 发散测试扩展

## 1. 结论

Browser QA 基础设施从现有 `deploy/gcp/envs/prod/` Terraform root 中拆出，改由新的 `deploy/gcp/envs/browser-qa-staging/` root 和独立 GCS state 管理。新 root 只拥有 Browser QA 专属资源，不拥有 Cloud SQL、现有 Cloud Run 应用服务、负载均衡、DNS、证书、生产 Secret、监控或 staging 应用服务。

本次工作的唯一业务目标是：先安全创建 Browser QA 所需云资源，然后把已经完成的 staging 自动接线推送到 `staging`；当 staging 后端完成部署和健康检查后，自动执行一次录制 Skill 的 `core` replay。QA 失败只让 GitHub Actions 变红并产生告警，不回滚已经完成的 staging 部署。

修改仓库里的 `.tf` 文件本身不会改变云端；只有 `terraform apply` 才会改变 GCP。本次不 apply 旧的 production root，只允许人工操作员 apply 新 root 中已经通过 create-only 白名单的 saved plan。虽然 QA 资源仍创建在项目 `vocai-gemini-prod` 内，但它们使用独立命名、独立身份和独立 state，不复用或修改现有应用资源。

本设计替代 [2026-07-31 Flatkey Staging Browser QA 设计](./2026-07-31-flatkey-staging-browser-qa-design.md) 中“Browser QA 与 production 共用 `deploy/gcp/envs/prod` root/state”的基础设施归属。原设计的浏览器运行时、身份、Secret、报告、清理、record/replay 和安全边界保持不变。

## 2. 为什么必须隔离

当前 Browser QA 资源写在 `deploy/gcp/envs/prod/browser_qa.tf`，并与生产 Cloud SQL、Cloud Run、Secret、监控和 staging 服务共用 backend prefix `envs/prod`。一次真实 refreshing plan 已同时出现：

- Browser QA 专属资源创建；
- Cloud SQL 因磁盘配置而替换；
- 多个非 QA Secret version 替换；
- production console/router、staging 服务和监控更新。

因此，当前完整 plan 不能 apply。`-target`、`-refresh=false` 或手工 `gcloud` 创建都不能解决 state ownership 问题，只会隐藏 drift 或制造 Terraform 之外的资源。

独立 state 是唯一能让 Browser QA 在 production root 存在 drift 时仍可单独创建和审查的长期边界。

## 3. 已选方案与未选方案

### 3.1 已选：独立 root + 独立 state

- 新 root：`deploy/gcp/envs/browser-qa-staging/`
- 继续使用现有 state bucket：`vocai-gemini-prod-newapi-tfstate`
- 新 backend prefix：`envs/browser-qa-staging`
- 新 root 只声明 Browser QA 专属资源和 `browser_qa_*` outputs
- production root 不再声明、输出或开关 Browser QA 资源
- 项目公共 API 仍由 production root 管理；新 root 只做启用状态 preflight，不重复创建 `google_project_service`

共享 state bucket 不等于共享 state。不同 prefix 会生成互不相见的 state object；Browser QA plan 无法读取或规划 `envs/prod` 中的 Cloud SQL、应用 Cloud Run、Secret version 或监控资源。

### 3.2 未选：继续使用 production root

它不需要重构，但每次 Browser QA 变更都受 production drift 阻塞，已经不能满足“只验证 staging、不影响其他资源”的目标。

### 3.3 未选：`terraform -target` 或 `-refresh=false`

这两种方式会缩小 Terraform 的观察面，却不改变资源仍由同一 state 所有的事实。保存的计划也不能证明被隐藏的资源安全，因此继续禁止。

### 3.4 未选：手工 `gcloud` 创建

手工创建会让云端资源与 Terraform state 分离，后续需要 import，并可能产生重复资源或 IAM 漂移，因此不作为 bootstrap 路径。

## 4. 资源所有权

| 所有者 | 资源 |
|---|---|
| `browser-qa-staging` state | Browser QA Artifact Registry repository 及其成员 IAM |
| `browser-qa-staging` state | `runtime`、`broker`、`cleanup`、`deployer` 四个 QA service account 及专属 IAM |
| `browser-qa-staging` state | Codex API key、identity seed、Gmail OAuth 三个 Secret 容器及访问 IAM |
| `browser-qa-staging` state | 私有 broker Cloud Run service、main/cleanup Cloud Run jobs 及专属 IAM |
| `browser-qa-staging` state | 私有 QA report bucket、生命周期规则及专属 IAM |
| `browser-qa-staging` state | Browser QA GitHub WIF pool、provider 和 staging-ref impersonation binding |
| `prod` state | GCP project API enablement、production/staging 应用服务、Cloud SQL、网络、LB、DNS、证书、生产 Secret、监控 |
| GitHub Actions | 三个 QA runtime 的容器 image 字段；Terraform 继续 `ignore_changes` |
| 人工操作员 | 三个 Secret 的版本内容、五个 GitHub repository variables、Gmail OAuth 生命周期 |

新 root 禁止声明下列 Terraform 类型或对象：

- `google_sql_*`、`google_compute_*`、`google_dns_*`、`cloudflare_*`；
- 非 Browser QA 的 Cloud Run service/job；
- `google_secret_manager_secret_version`；
- `google_project_service`；
- 任何 `newapi-console`、`newapi-router`、`newapi-web`、`newapi-staging` 或 traffic 资源；
- 具有全量覆盖语义的 IAM policy/binding。QA IAM 继续使用增量的 `*_iam_member`。

## 5. 新 Terraform root 结构

```text
deploy/gcp/envs/browser-qa-staging/
├── backend.tf
├── versions.tf
├── variables.tf
├── terraform.tfvars
├── browser_qa.tf
├── outputs.tf
└── .terraform.lock.hcl
```

- `backend.tf` 只配置现有 state bucket 和 `envs/browser-qa-staging` prefix。
- `versions.tf` 固定与 production root 兼容的 Terraform 和 Google provider 版本。
- `variables.tf` 只接受非敏感的 `project_id` 和 `region`；不接受 API key、OAuth JSON、refresh token、Gmail 地址或其他 Secret 值。
- `terraform.tfvars` 只保存 `vocai-gemini-prod` 和 `us-west1` 等非敏感环境标识。
- `browser_qa.tf` 包含现有 QA 专属资源。专用 root 本身就是启用边界，不再保留容易误触发整组销毁的 `enable_browser_qa` 布尔开关。
- `outputs.tf` 保持现有 `browser_qa_*` 名称和语义，避免重写 GitHub workflow 契约。
- `.terraform.lock.hcl` 提交到仓库，保证 operator 和 CI 解析同一 provider 版本。

production root 中同步移除：

- `browser_qa.tf`；
- `enable_browser_qa` variable 和 `terraform.tfvars` 值；
- 八个 `browser_qa_*` outputs。

`deploy/gcp/docs/OPERATIONS.md` 中 Browser QA 的 root 表格、`terraform init/plan/apply/output/state`、broker 验证和 GCS 报告查询命令必须全部改为新 root。文档总览同时明确存在两套 root：`prod` 继续管理共享/生产基础设施，`browser-qa-staging` 只管理 staging Browser QA。不得留下会引导操作员从 `deploy/gcp/envs/prod/` 执行 Browser QA 命令的示例。

云端审计和原 production plan 均证明 Browser QA 资源尚未创建，旧 state 也将通过 `terraform state list` 再次确认不存在 `browser_qa` 地址。因此本次不需要 `state mv`、import 或 destroy。若实施时发现任一旧 state 地址或同名 live 资源，bootstrap 必须停止，并另行设计 state migration/import，不能继续创建。

## 6. 公共 API 边界

Browser QA 会使用 Artifact Registry、Cloud Run、Secret Manager、IAM/IAM Credentials、STS、Logging 和 Cloud Storage 等已启用 API。API 是 project 级共享前置，只允许一个 Terraform state 拥有；所有 `google_project_service` 继续留在 production root。

新 runbook 在 `terraform plan` 前执行只读 preflight：列出 Browser QA 需要的 API，并与 `gcloud services list --enabled` 比对。若缺少 API，输出确切名称并停止。操作员随后通过 production 基础设施流程补齐 API；Browser QA root 不自动启用或关闭 API，也不以跨 state data 引用 production outputs。

## 7. Fail-closed plan 审核

独立 state 是主要隔离，saved plan 白名单是第二道保护。所有 apply 都必须来自同一 shell 中刚生成并审查过的 refreshing saved plan；继续禁止无保存的 `terraform apply`、`-target` 和 `-refresh=false`。

首次 bootstrap 的自动检查必须同时满足：

1. 当前目录是 `deploy/gcp/envs/browser-qa-staging/`。
2. backend bucket 与 prefix 精确等于设计值。
3. active account、project 和 region 精确匹配 runbook。
4. 新 state 的 `terraform state list` 为空。
5. 同名 Browser QA live 资源均不存在。
6. plan 中所有 resource address 都在版本化的 QA 精确白名单内。
7. 每个非 no-op resource action 都严格为 `create`；任何 `update`、`delete`、`replace` 立即失败。
8. output change 只允许 `browser_qa_*`。
9. plan 不包含 Secret version 或敏感值。
10. 人工阅读 human-readable plan 后，显式输入固定确认短语，才 apply 同一个 saved plan。

后续维护仍要求所有 resource address 位于 QA 白名单。`delete` 或 `replace` 永远需要单独审查和明确的变更设计，不能由常规 runbook 自动放行。

即使有人将非 QA 资源误加到新 root，只要它不在白名单中，计划检查也会在 apply 前失败。

白名单和动作规则落成仓库内可测试的 `scripts/browser_qa/terraform_plan_guard.py`，由 runbook 调用；测试位于 `scripts/browser_qa/tests/`。它按完整 Terraform resource address 精确匹配，不使用 `"browser_qa" in address` 这类子串判断。QA 专属的 project-level `logging.logWriter` member 必须逐地址列入，其余 project IAM 一律拒绝。

## 8. Outputs 与 GitHub Variables 契约

以下映射保持不变：

| GitHub repository variable | 独立 root output |
|---|---|
| `GCP_BROWSER_QA_AR_REPO_URL` | `browser_qa_artifact_registry_url` |
| `GCP_BROWSER_QA_WIF_PROVIDER` | `browser_qa_wif_provider` |
| `GCP_BROWSER_QA_DEPLOYER_SA` | `browser_qa_deployer_sa_email` |
| `GCP_BROWSER_QA_GCS_BUCKET` | `browser_qa_report_bucket` |

`browser_qa_broker_uri`、`browser_qa_broker_service_name`、`browser_qa_main_job_name` 和 `browser_qa_cleanup_job_name` 也继续存在，供 runbook 审计和负向 IAM 验证使用。

第五个变量 `GCP_BROWSER_QA_GMAIL_BASE` 不是 Terraform output，由人工操作员安全写入 GitHub repository variable。它只包含 Gmail base address，不含 plus tag。任何 API key、OAuth JSON、refresh token、验证码或完整 Flatkey key 都不得写入 GitHub Variables、Terraform state、命令参数、日志或聊天。

## 9. 操作流程

### 9.1 代码阶段

1. 在当前隔离 worktree 中实现新 root、production root 解耦、runbook 和合约测试。
2. 只运行本地只读验证：格式、validate、合约测试和 plan 守卫 fixture 测试。
3. 不 apply production plan，不写 Secret，不写 GitHub Variables，不 dispatch workflow。

### 9.2 人工 bootstrap

1. 操作员确认 GCP account、project、region、ADC 和独立 backend prefix。
2. 只读检查 production state 中没有 Browser QA 地址，并再次检查同名 live 资源不存在。
3. 运行 API preflight。
4. 在新 root 生成 refreshing saved plan，通过 create-only 白名单检查并人工阅读。
5. 人工 apply 同一个 saved plan。
6. 通过安全 stdin 流程写入三个 Secret version；Secret 容器由 Terraform 管理，内容不由 Terraform 管理。
7. 从独立 root outputs 写入四个 output-backed GitHub Variables，再单独写入 `GCP_BROWSER_QA_GMAIL_BASE`。
8. 对 IAM、broker 私有性、report bucket 和 Secret 可访问性运行只读/负向验证。

### 9.3 真实 staging 验收

1. 重新确认远端 `staging` 仍位于预期父提交，避免覆盖他人的新提交。
2. 将独立 Terraform root 实现和已提交的自动接线快进到 `staging`。
3. GitHub Actions 部署 staging 后端并完成健康检查。
4. 同一个 workflow run 自动调用 Browser QA reusable workflow，`mode: core`。
5. 监控 build、deploy、`browser-qa-core`、main execution 和 cleanup execution。
6. 验证 GCS manifest、脱敏 artifacts、账号/API Key 清理和 GitHub Summary。

`core` 失败时 staging 仍保持已部署状态；Actions 变红用于告警和排查，不执行 rollback。cleanup 失败时，用原始 staging workflow run id 手动执行 `cleanup-only`。

## 10. 测试策略

### 10.1 静态与 Terraform 合约

- `terraform fmt -check` 和 `terraform validate` 覆盖新 root。
- backend 测试精确断言 `envs/browser-qa-staging`，并断言不等于 `envs/prod`。
- production root 测试断言不再包含 Browser QA resources、variable、tfvar 或 outputs。
- 新 root 测试断言只存在 QA 专属资源类型、名称和增量 IAM member。
- operations 测试断言所有 Browser QA Terraform 命令均指向新 root，且旧 prod root 不再作为 Browser QA 操作目录。
- 测试断言不含 `google_project_service`、`google_secret_manager_secret_version` 和所有禁止资源。
- outputs 与四个 GitHub Variables 的映射保持现有名称。
- Secret 内容、Gmail base address 和 credentials 不出现在 Terraform 文件或 state 输入面。

### 10.2 Plan 守卫测试

使用脱敏 JSON fixture 验证：

- 纯 QA create-only plan 通过；
- 非 QA resource address 失败；
- QA resource 的 update、delete 或 replace 在 bootstrap 模式失败；
- 非 `browser_qa_*` output 失败；
- 空 plan 不会进入 apply；
- backend、project 或 region 不匹配时失败。

### 10.3 Workflow 回归

- 现有 Browser QA 单元、容器、Terraform、operations 和 workflow contract tests 全部通过。
- staging deploy 仍只在 deploy 和健康检查成功后调用 `mode: core`。
- QA 失败不会触发任何 rollback、traffic 恢复或 production workflow。
- 手动 `core`、`normal` 和 `cleanup-only` 入口保持可用。

### 10.4 Live 验收

- 独立 plan 只包含 QA create，且 production plan 不被 apply。
- staging push 产生一次自动 `core` execution。
- replay 完成录制的 onboarding 任务。
- cleanup execution 成功，测试账号和所有 API Key 均不可再使用。
- 报告只进入私有 QA bucket，且不含 Secret。

## 11. 故障与恢复

- **plan 出现非 QA 或非 create 变更：** 立即停止，不 apply，不使用 `-target` 绕过。
- **发现旧 state/live 同名资源：** 停止 bootstrap，改走单独的 import/migration 设计。
- **API 未启用：** 由 production 基础设施 owner 补齐；新 root 不抢占 API ownership。
- **apply 中断：** 不切回 production root；在独立 root 重新 refresh/plan，审查实际已创建的 QA 资源后恢复。
- **Secret 或 GitHub Variables 不完整：** 不推送 `staging`，修复后重新做只读 readiness audit。
- **staging 部署成功但 QA 失败：** 保留 staging，读取私有报告并修复；不自动回滚。
- **cleanup 失败：** 使用原始 run id 执行 `cleanup-only`，并把 run 标记为需要人工处理。
- **需要撤销自动触发：** 回退 staging workflow 接线即可；QA 云资源保留，不影响 staging 应用服务。

state bucket 已启用 object versioning，可用于独立 state 的灾难恢复，但不能替代 saved plan 审查。

## 12. 验收标准

- Browser QA state prefix 与 production state prefix 完全分离。
- 新 root 中没有任何 production/staging 应用基础设施或共享 API ownership。
- 首次 plan 只包含精确白名单内的 Browser QA `create`；零 `update`、`delete`、`replace`。
- 旧的危险 production plan 从未 apply；Cloud SQL、现有 Cloud Run、Secret version、监控、LB、DNS 和 traffic 均未被本工作修改。
- Secret 内容不进入 Terraform state、仓库、GitHub logs 或聊天。
- 五个 GitHub Variables 和三个 Secret version 就绪后，推送 `staging` 自动执行一次 `core` replay。
- replay 成功或给出可定位失败；无论结果如何，cleanup 都执行。
- QA 失败只告警，不回滚 staging，不接入 main 或 production 发布。
- 在首次真实 staging `core` run 和 cleanup 验证完成前，不声称整条链路已经跑通。
