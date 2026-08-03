# Browser QA 两阶段 Secret Bootstrap 设计

**日期：** 2026-08-03
**状态：** 已批准，实施中
**目标环境：** Flatkey staging
**前置设计：** [Browser QA Terraform State 隔离设计](./2026-08-03-browser-qa-terraform-state-isolation-design.md)

## 结论

Browser QA 继续只由 `deploy/gcp/envs/browser-qa-staging/` 和 GCS backend prefix `envs/browser-qa-staging` 管理。首次创建改为同一 root、同一 state 内的两阶段 bootstrap：

1. Phase A 只创建 26 个基础设施资源和 4 个基础 outputs，不创建 Cloud Run workloads。
2. 人工操作员通过 stdin 为三个 Secret 容器写入首个 enabled version；Terraform 不管理 Secret version 或值。
3. Phase B 才创建 3 个 Cloud Run workloads、6 个 resource-level Run IAM members 和 4 个 workload outputs。
4. 稳态配置保持 `create_workloads = true`，避免日常 plan 把 workloads 规划为销毁。

旧的 35-resource 单阶段 saved plan 不得 apply。新 phase-aware guard 会让它在 Phase A 和 Phase B 都失败。

## 根因

原计划在同一次 apply 中先创建空 Secret containers，再创建引用 `secret_key_ref.version = "latest"` 的 broker service、main job 和 cleanup job。Secret version 是人工管理的，因此首次 plan 生成时三个 Secret 都没有 `latest`。Cloud Run 会在创建或启动阶段解析 Secret，Terraform provider 会等待 Cloud Run operation；结果可能是 apply 失败并留下部分资源。

State 隔离本身没有问题。问题是“Secret 容器存在”和“workload 可以安全启动”被错误地视为同一个 bootstrap 里程碑。

## 资源边界

### Phase A：26 个基础设施资源

- Artifact Registry repository 及 deployer writer IAM：2
- runtime、broker、cleanup、deployer service accounts：4
- project log-writer IAM members：3
- Secret containers：3
- Secret accessor IAM members：4
- service-account user/WIF IAM members：4
- private report bucket 及 bucket IAM members：4
- WIF pool/provider：2

Phase A 的 4 个 outputs：

- `browser_qa_artifact_registry_url`
- `browser_qa_wif_provider`
- `browser_qa_deployer_sa_email`
- `browser_qa_report_bucket`

### Phase B：9 个 workload 资源

- `google_cloud_run_v2_service.browser_qa_broker[0]`
- `google_cloud_run_v2_job.browser_qa_main[0]`
- `google_cloud_run_v2_job.browser_qa_cleanup[0]`
- broker 的 2 个 resource-level IAM members，地址带 `[0]`
- main job 的 2 个 resource-level IAM members，地址带 `[0]`
- cleanup job 的 2 个 resource-level IAM members，地址带 `[0]`

Phase B 的 4 个 outputs：

- `browser_qa_broker_uri`
- `browser_qa_broker_service_name`
- `browser_qa_main_job_name`
- `browser_qa_cleanup_job_name`

Workload resources 使用 `count = var.create_workloads ? 1 : 0`，因此 Terraform plan JSON 中的稳态地址必须带 `[0]`。Workload outputs 使用 `one(resource[*].field)`；Phase A 中空 tuple 返回 `null`，这些 outputs 不进入 state，Phase B 创建资源后再出现。

## 配置与稳态

`variables.tf` 新增：

```hcl
variable "create_workloads" {
  description = "Create the Secret-dependent Browser QA Cloud Run service, jobs, and their resource IAM"
  type        = bool
  default     = true
}
```

`terraform.tfvars` 显式保存 `create_workloads = true`。Phase A 仅通过命令行临时覆盖 `-var='create_workloads=false'`。Phase B 和后续普通 plan 使用 committed steady-state `true`。

在 Phase B 完成后再次使用 `create_workloads=false` 会产生 destroy 计划；phase-aware guard 必须拒绝所有 delete、replace 和 update，因此 runbook 不允许把 Phase A 当作日常维护模式。

## Saved-plan guard

`terraform_plan_guard.py` 必须要求 `--phase infra` 或 `--phase workloads`：

- `infra`：精确接受 26 个 Phase A creates 和 4 个 Phase A output creates。
- `workloads`：允许 Phase A 资源/output 为 `no-op`，但有意义的 change 必须精确为 9 个带 `[0]` 的 workload creates 和 4 个 workload output creates。
- 任意未知 resource/output，即使是 `no-op`，也失败。
- 任意 update、delete、replace、deposed object 或重复 JSON key 都失败。
- 旧的 35-create plan 同时包含两个 phase 的资源，因此两个 mode 都失败。

每个 phase 只 apply 同一 shell 中刚生成、刚导出 JSON、刚通过 guard、刚人工阅读的 saved plan。继续禁止 unsaved apply、`-target` 和 `-refresh=false`。

## Phase B readiness

Phase B plan 前必须同时证明：

1. active project 为 `vocai-gemini-prod`，region 为 `us-west1`；
2. 独立 state 的 resource address 集合精确等于 26 个 Phase A 地址；
3. broker、main job、cleanup job 仍不存在；
4. 三个 Secret 的 `latest` version 均存在且 state 为 `ENABLED`，但不读取 payload；
5. refreshing Phase B plan 通过精确 9-create/4-output guard。

## 中断与恢复

- Phase A 或 Phase B apply 返回非零时，原 saved plan 立即作废，不自动重试，不切换到 `-target`，也不手工删除或 import。
- 操作员只收集 resource addresses、Cloud Run Ready 状态和非敏感诊断；不读取 Secret payload 或 Terraform state value。
- 因部分 apply 导致后续 plan 只剩 create 子集时，严格 bootstrap guard 会失败。这是预期的 fail-closed 行为；必须先写一份针对实际残留资源的一次性恢复方案，再生成新的 saved plan。
- Phase B 失败不回滚 Phase A，也不影响现有 staging 应用服务；staging 自动 replay 在 workloads、Secret versions 和 GitHub Variables 全部 ready 前不得启用。

## 验证

- TDD 先证明旧 guard 接受单阶段 plan 的行为会失败。
- Terraform contract tests 锁定 `create_workloads` 默认/稳态、9 个 gated blocks、26 个 ungated blocks、带 `[0]` 地址和 nullable workload outputs。
- Operations contract tests 锁定 `Phase A -> output-backed variables -> Secret versions -> Phase B -> IAM/dispatch` 顺序以及两个 confirmation token。
- 本地执行完整 Browser QA unittest、Terraform fmt/init/validate、泄漏扫描和变更范围扫描。
- 最后只生成真实 Phase A saved plan并审查；本任务不执行 `terraform apply`、Secret 写入、GitHub Variable 写入、workflow dispatch、staging push 或 production/main 接线。
