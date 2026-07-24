# new-api 发布与部署手册

> 这份文档是 `SolveaCX/new-api` 发布到 GCP 项目 `vocai-gemini-prod` 的操作指南。基础设施清单见 `INFRASTRUCTURE.md`，GCP 操作雷区见 `OPERATIONS.md`。

## 生产服务与发布目标

| 改动类型 | 生产入口 | Cloud Run service | Workflow | 说明 |
|---|---|---|---|---|
| 官网/SEO/公开页面 | `flatkey.ai`, `www.flatkey.ai` | `newapi-web` | `gcp-deploy-website.yml` | 独立 Next.js 项目 `website/` |
| 控制台、后台 API、管理 UI | `console.flatkey.ai` | `newapi-console` | `gcp-deploy.yml` | Go app，`NODE_TYPE=master`，高频发布目标 |
| 模型调用、relay、provider 适配 | `router.flatkey.ai` | `newapi-router` | `gcp-deploy.yml` | Go app，`NODE_TYPE=slave`，容量按模型调用配置 |
| default fallback | default backend | `newapi-console` | 不单独发布 | 未命中 host_rule 的请求进入 console backend |

Go app workflow 构建同一份 Go 镜像。Console 与 Router 分别使用 `production-console`、`production-router` Environment 和独立部署身份。push 到 `main` 或手动选择 `both` 时严格串行：先审批并发布 Console，等待至少 3600 秒旧管理请求 drain，Console job 成功后才会出现 Router 审批。手动选择单一目标时只进入对应 Environment。不要把 website 变更放到 Go workflow，也不要在 `web/default` 里恢复公开网站页面。

## 日常发布

### 触发方式

1. **push main**：PR 合并到 `main` 后，Go app workflow 构建并推送镜像，等待 `production-console` 审批；Console 发布及至少 3600 秒 drain 成功后，才等待 `production-router` 审批。
2. **手动 deploy**：GitHub → Actions → `GCP Deploy` → Run workflow，选择 `deploy_target`。`console`、`router` 分别进入对应 Environment；`both` 仍执行 Console → drain → Router。`image_tag` 非空会 fail closed；旧镜像只能通过带不可变 binding 证据的 Rollback workflow 使用。

`deploy_target` 行为：

| deploy_target | 行为 |
|---|---|
| `console` | 只发布 `newapi-console` |
| `router` | 只发布 `newapi-router` |
| `both` | 同一新构建镜像按 Console → 至少 3600 秒 drain → Router 串行发布 |
| `none` | 手动 run 时只构建新镜像，不发布生产 |

### Go app 发布流程

```text
push main
  -> GitHub Actions build Go image
  -> push Artifact Registry
  -> deploy console waits for production-console approval
  -> Console canary performs unauthenticated manifest/OCI checks
  -> shift Console traffic, then verify authenticated control plane only at https://console.flatkey.ai
  -> verification failure automatically restores the previous Console revision
  -> wait at least 3600 seconds for old management requests to drain
  -> deploy router waits for production-router approval
  -> Router canary and traffic shift

workflow_dispatch deploy_target=console/router/both
  -> build a new Go image (nonempty image_tag is rejected)
  -> selected deploy job waits for its target-specific approval
  -> both follows the same Console -> drain -> Router order
  -> deploy new revision with --no-traffic
  -> canary tag health check /api/status
  -> shift traffic by revision
```

风险：

- Cloud Run 流量切换不会中断已经在旧 revision 上运行的请求；旧 revision 会继续 drain。
- Console canary 健康检查失败时不会切流；切流后的固定域名控制面/readiness 校验失败时 workflow 自动回切旧 revision。
- `deploy-console` 和 `deploy-router` 是两个目标隔离的 job；push/`both` 不并行审批，Router 必须等 Console 与 drain 成功。手动 `router` 可只进入 `production-router`。
- 如果代码改动同时影响 console/router，两边都要发布并验证；不要只看一个域名。
- router 承载 LLM API/relay 真实流量，只有 review 明确要求时才 approve `deploy router`。

### Website 发布流程

`website/**` 变更走 `.github/workflows/gcp-deploy-website.yml`，部署 `newapi-web`，容器端口 4000。验证重点是 SSR、canonical、sitemap、pricing/blog 等公开页面。

## 发布后验证

### 1. URL map

```bash
gcloud compute url-maps describe newapi-urlmap \
  --project=vocai-gemini-prod --global \
  --format='yaml(hostRules,pathMatchers,defaultService)'
```

期望：

- `flatkey.ai`, `www.flatkey.ai` -> `newapi-web-backend`
- `console.flatkey.ai` -> `newapi-console-backend`
- `router.flatkey.ai` -> `newapi-router-backend`
- default -> `newapi-console-backend`

### 2. Cloud Run 当前流量

```bash
for svc in newapi-console newapi-router newapi-web; do
  echo "== $svc =="
  gcloud run services describe "$svc" \
    --project=vocai-gemini-prod --region=us-west1 \
    --format='table(status.latestReadyRevisionName,status.traffic[].revisionName,status.traffic[].percent,status.url)'
done
```

### 3. 域名健康检查

```bash
curl -i --max-time 20 https://console.flatkey.ai/api/status
curl -i --max-time 20 https://router.flatkey.ai/api/status
curl -I --max-time 20 https://flatkey.ai
```

期望：

- console/router `/api/status` 返回 200。
- website 返回 200/3xx，且响应来自 Next.js website。

### 4. LB 日志确认真实后端

```bash
gcloud logging read \
  'resource.type="http_load_balancer" AND resource.labels.backend_service_name="newapi-console-backend"' \
  --project=vocai-gemini-prod --freshness=10m --limit=100 --format=json

gcloud logging read \
  'resource.type="http_load_balancer" AND resource.labels.backend_service_name="newapi-router-backend"' \
  --project=vocai-gemini-prod --freshness=10m --limit=100 --format=json
```

看点：

- 有近期请求进入对应 backend。
- 5xx 不应上升。
- router 出现少量 400/403 可以是业务请求错误，不等于基础设施失败；重点看 5xx 和延迟。

## 回滚

### 回滚某个 Cloud Run service 的应用 revision

风险：低到中。只切流量，不改 DNS/LB/证书；正在运行的旧请求不会被强杀，但新请求会进入指定 revision。

推荐方式：GitHub → Actions → `GCP Rollback` → Run workflow，选择 `rollback_target`（`console` 或 `router`）并填写目标 revision；该 workflow 同样经过 `production` Environment 审批。

```bash
gcloud run revisions list \
  --project=vocai-gemini-prod --region=us-west1 \
  --service=<newapi-console|newapi-router|newapi-web> --limit=10

gcloud run services update-traffic <service> \
  --project=vocai-gemini-prod --region=us-west1 \
  --to-revisions=<previous-revision-name>=100
```

验证：

```bash
gcloud run services describe <service> \
  --project=vocai-gemini-prod --region=us-west1 \
  --format='value(status.traffic)'
```

### 回滚 console host 分流到 default backend

风险：低到中。当前 default backend 也是 `newapi-console-backend`，所以删除 console host_rule 不会回到 legacy 服务；但仍需 review URL map diff。不会改 Cloudflare，不会触发 GCP managed cert rotation。

步骤：

1. 改 `deploy/gcp/envs/prod/terraform.tfvars`：
   ```hcl
   console_domains = []
   ```
2. 本地 plan 并人工 review：
   ```bash
   terraform -chdir=deploy/gcp/envs/prod plan -input=false -no-color
   ```
3. 仅确认 URL map 删除 console host_rule 时再 apply。
4. 验证 `console.flatkey.ai/api/status` 和 LB 日志。

### 回滚 router host 分流到 default backend

风险：高。router 承载真实模型调用流量，当前 default backend 是 `newapi-console-backend` 而不是 legacy router 等价服务；不要把这当作常规 router 回滚。优先使用 Cloud Run revision rollback。

步骤：

1. 改 `deploy/gcp/envs/prod/terraform.tfvars`：
   ```hcl
   router_domains = []
   ```
2. 本地 plan 并人工 review。
3. 低峰 apply。
4. 重点观察 router 5xx、延迟和 provider 错误。

### 回滚 website host 分流到 default backend

风险：中。`flatkey.ai` / `www.flatkey.ai` 会回到 default backend（当前为 `newapi-console-backend`），公开官网会不可用或变成 Go app 行为；只适合 website 严重故障时短时兜底。

```hcl
website_domains = []
```

再 plan/apply，并验证 apex/www。

## 基础设施变更

任何 `terraform` / `gcloud` 前必须先读 `OPERATIONS.md`。

### 标准流程

```bash
terraform -chdir=deploy/gcp/envs/prod init
terraform -chdir=deploy/gcp/envs/prod plan -input=false -no-color
```

检查点：

- 不应出现 Cloud Run `image`、`traffic`、live env 被 Terraform 改回的 diff。
- `lb_domains` 变更会触发 GCP managed cert rotation，必须单独确认维护窗口。
- `gcp-infra.yml` 只运行 `terraform init -backend=false`、validate 和静态 contracts；它没有 GCP auth、远端 backend、plan 或 apply。production 必须使用经审查的 Owner ADC 在本地执行 refreshing plan，审阅后 apply。

首次 IAM/WIF 拆分必须按顺序 bootstrap：先冻结 deploy/rollback/promotion 并补齐下述 GitHub 外部保护；从受控运维环境取得 Owner ADC；在本地 init 并执行 refreshing plan；审阅新增 service accounts、custom roles、精确 resource IAM、六个 WIF pools/providers，以及删除旧 project-wide deployer grants/repository-wide impersonation binding 的 diff；apply saved plan；独立审计 state bucket IAM，确认 generic builder 没有 Run/Job/Scheduler/runtime-SA `actAs` 或 Terraform state access；最后才 merge/启用引用固定 identities 的 workflow，执行正向 lane 与跨 lane 拒绝验证后解除发布冻结。不能要求尚未创建的新 WIF identity 自举自身；切换窗口内旧 workflow 必须 fail closed。

### 添加或删除 LB host 分流

优先改这些变量：

```hcl
website_domains = ["flatkey.ai", "www.flatkey.ai"]
console_domains = ["console.flatkey.ai"]
router_domains  = ["router.flatkey.ai"]
```

风险：

- host_rule 变更是 URL map 原地更新，通常不停机。
- 配错 host/backend 会把新请求导到错误服务。
- 变更 `lb_domains` 不是普通 host_rule 变更，会触发证书风险。

### Cloudflare

当前策略：

- `console.flatkey.ai`：Proxied。不要改，除非决定把 console 加进 GCP cert 并接受 cert rotation 风险。
- `router.flatkey.ai`：DNS only。由 GCP managed cert 覆盖，适合模型调用长连接。
- `flatkey.ai` / `www.flatkey.ai`：Proxied。官网入口。

Cloudflare “origin IP partially exposed” warning 在当前混合模式下是预期现象，不是必须修复项。

## 密钥与凭据管理

### Secret 更新原则

- 用 `printf '%s'` 写 secret，避免尾部换行。
- Cloud Run `latest` secret 只在新 revision 启动时读取，不会热加载。
- 更新 secret 后，需要对目标 service 创建新 revision，并确认 traffic。

### 轮换数据库密码

```bash
printf '%s' '<new-password>' | gcloud secrets versions add newapi-db-app-password \
  --project=vocai-gemini-prod --data-file=-

gcloud sql users set-password newapi_app --host=% \
  --project=vocai-gemini-prod --instance=newapi-mysql --password='<new-password>'

DSN="newapi_app:<new-password>@unix(/cloudsql/vocai-gemini-prod:us-west1:newapi-mysql)/newapi?parseTime=true&charset=utf8mb4&loc=UTC"
printf '%s' "$DSN" | gcloud secrets versions add newapi-sql-dsn \
  --project=vocai-gemini-prod --data-file=-
```

然后按目标服务逐个创建新 revision 并切流量。风险较高，建议维护窗口。

## 供应商核算 Job 与兼容桥发布

本功能涉及 Terraform、`newapi-console`、`newapi-router`，以及仍在承载流量时的 legacy `newapi`。不涉及 `newapi-web`、Cloudflare、LB 域名或证书。不得因该功能修改 `lb_domains`。

Terraform 使用两阶段 fail-closed 门禁。`supplier_batch_job_enabled=false` 时 `supplier_batch_runner_image` 可为空且不创建 Job/Scheduler；这是合法的 phase-one 状态，日常 production/staging 应用发布和普通回滚不得把 Job 缺失视为错误，也不得 describe/update/execute Job 或 pause/resume Scheduler。`enabled=true` 时 lifecycle precondition 强制 `image@sha256:<digest>`，tag 会失败。Terraform 创建：

- `newapi-supplier-batch` one-shot Cloud Run Job，`/app/supplier_batch_runner`，task timeout 3600 秒、零 task retry；
- `newapi-supplier-batch-daily` Cloud Scheduler，`02:05 Asia/Shanghai`；每次 create/re-create 都以 `paused=true` 启动，Terraform ignore 后续 `paused` drift；promotion workflow 不拥有 Scheduler，恢复必须由另行授权的 operator 执行；
- 独立 runner/Scheduler/promoter service account；Scheduler 只有该 Job 的 `roles/run.invoker`；`newapi-supplier-promoter` 只有固定 Job 的 `run.jobs.get/update/run`、project-level read-only Run/log observer、固定 Artifact Registry repository reader 和精确 runner-SA `actAs`，零 `cloudscheduler.*`；
- `newapi-supplier-batch-token-current` / `-next` 空 Secret 容器，只有 runner 可读；
- Console request timeout 保持 3600 秒。Runner 自身 HTTP timeout 固定 55 分钟，服务端 batch hard stop 45 分钟。

Terraform 不创建任何 secret version，不保存 raw token 或 Console verifier hash。首次执行两次独立的本地 Owner-ADC refreshing plan/apply：第一轮保持 `supplier_batch_job_enabled=false`，创建 runner、Scheduler、promoter 三个 SA、promoter custom roles、精确 WIF/actAs/repository binding、两空 Secret 容器和 IAM；运维随后 out-of-band 写入两个 32-byte base64url token version，并只把 SHA-256 verifier hashes 部署到 Console；第二轮设置 `enabled=true`、传入验证过的 digest image，才创建 Job/Scheduler。新 Scheduler 保持 paused；promotion 完成后仍保持 paused，直到另行授权的 operator 执行下面的固定-shape 验证和 resume。因此第二次 apply 和 promotion 都不会自动触发定时核算。CI backend-disabled validation 不能证明 production state 或 live drift。

运行任何 production workflow 前还必须人工补齐并复核 GitHub 外部保护。当前观测状态是：`production`、`production-console`、`production-router` 有 reviewers 但没有 deployment branch policy，且 `prevent_self_review=false`；`staging` branch 未保护；`production-infra` Environment 不存在且没有 CI infra identity；`SUPPLIER_DEPLOY_ROOT_ACCESS_TOKEN` Environment secret 与 `SUPPLIER_DEPLOY_ROOT_USER_ID` Environment variable 缺失。本文不声称这些设置已修改。相应 branch/environment policy 或 Root 输入缺失时必须 fail closed，禁止复制 production Root 凭据到 staging。

发布顺序是强制门禁：

1. mutation gate 保持 disabled。确认 old production 是否存在供应商 mutation route；若存在或协议不兼容，先发布所有版本都尊重 gate 的 bridge release。
2. 将新 Console digest 部署到 0% tagged URL，执行 schema/readiness/preflight；失败不得切流。
3. Console 从旧 revision 直接切到新 revision 100%，等待完整 60 分钟最大管理请求 timeout。Cloud Run 不会因切流杀死旧请求。
4. 发布全部兼容 Router digest；若 legacy `newapi` 仍有流量也发布。检查各 revision 流量、实例和请求日志，证明所有不兼容 writer 已排空。
5. 排空前必须同时看到命令账本两条 legacy index 与库存 legacy index；排空后才运行一次显式 finalizer。
6. finalizer 与两次 verify 全部成功后，认证的 accounting status/readiness 必须同时返回 `admin_command_ledger_state=finalized`；缺失、`bridge`、`invalid` 或未知值都不能推广 runner。随后才允许 Root 通过 fresh secure verification + expected-version CAS 开启 gate。
7. 只通过显式 `GCP Promote Supplier Runner` workflow 推广 runner。该 workflow 使用 `github-supplier-promote` pool 的 workflow/ref/event/subject-pinned provider impersonate 固定 `newapi-supplier-promoter`；禁止复用 `GCP_DEPLOYER_SA`，也不接受 SA/资源名 input。promoter 只有固定 Job 的 `run.jobs.get/update/run`、`run.operations.get`、`run.executions.get`、`run.services.get`、`run.revisions.get`、`logging.logEntries.list`，固定 Artifact Registry repository reader，以及仅对 runner SA 的 `roles/iam.serviceAccountUser`；它没有任何 `cloudscheduler.*`。workflow 先用 association verifier 确认 Terraform Job 的 identity/command/resources/env/digest association，确认目标 Console revision 独占 100% 流量、mutation gate disabled、认证 readiness=true、ledger finalized，再验证 canonical manifest/capabilities/OCI/binding；Job 更新到精确 digest 后第二次执行 association verifier；最后手动执行一次并从 execution log 验证 43 字符 request ID 与 protocol terminal success。成功只表示 Job promotion 完成，Scheduler 仍为 paused。

promotion 成功后，由拥有 Scheduler 操作权限的独立 operator 在仓库根目录执行以下完整序列。不得省略任一 describe/read-back 或用手工目测代替 association verifier：

```bash
set -euo pipefail
project=vocai-gemini-prod
region=us-west1
scheduler=newapi-supplier-batch-daily
job=newapi-supplier-batch
scheduler_sa=newapi-supplier-scheduler@vocai-gemini-prod.iam.gserviceaccount.com

verify_scheduler() {
  bash .github/scripts/supplier-resource-association-verify.sh scheduler \
    "$1" "$project" "$region" "$scheduler" "$job" "$scheduler_sa" "$2"
}

gcloud scheduler jobs describe "$scheduler" \
  --project="$project" --location="$region" --format=json \
  > /tmp/newapi-supplier-batch-daily.paused.json
verify_scheduler /tmp/newapi-supplier-batch-daily.paused.json PAUSED

if ! gcloud scheduler jobs resume "$scheduler" \
  --project="$project" --location="$region"; then
  gcloud scheduler jobs pause "$scheduler" \
    --project="$project" --location="$region"
  gcloud scheduler jobs describe "$scheduler" \
    --project="$project" --location="$region" --format=json \
    > /tmp/newapi-supplier-batch-daily.reverted.json
  verify_scheduler /tmp/newapi-supplier-batch-daily.reverted.json PAUSED
  exit 1
fi

if ! gcloud scheduler jobs describe "$scheduler" \
    --project="$project" --location="$region" --format=json \
    > /tmp/newapi-supplier-batch-daily.enabled.json || \
   ! verify_scheduler /tmp/newapi-supplier-batch-daily.enabled.json ENABLED; then
  gcloud scheduler jobs pause "$scheduler" \
    --project="$project" --location="$region"
  gcloud scheduler jobs describe "$scheduler" \
    --project="$project" --location="$region" --format=json \
    > /tmp/newapi-supplier-batch-daily.reverted.json
  verify_scheduler /tmp/newapi-supplier-batch-daily.reverted.json PAUSED
  exit 1
fi
```

任一 association/state 检查失败都必须停止；上述序列会在 resume 或 ENABLED read-back 失败时执行 pause 并再次要求 PAUSED association 验证。若恢复 PAUSED 本身失败，视为生产事故，不得继续。promotion workflow 本身永不 pause/resume Scheduler，也不会在失败时自动恢复定时触发。

`gcp-deploy.yml`、`gcp-deploy-staging.yml` 与 `gcp-rollback.yml` 永远不拥有 batch Job。普通 production/staging 应用发布只验证应用 revision 的 manifest/OCI/binding，runner image 只由显式 `GCP Promote Supplier Runner` workflow 在全部门禁通过后更新。这样应用发布、普通回滚、以及 `supplier_batch_job_enabled=false` 的 phase-one bring-up 都不会意外创建、检查或改写 runner。

Push 或 `deploy_target=both` 的 production Router job 必须依赖 Console job 完整成功；该成功包含 Console 0% tagged preflight、直接切换 100% 和至少 3600 秒 management-request drain。显式 `deploy_target=router` 保留为独立的 Router-only 运维入口，不伪造 Console drain 证据。

### 非驻留 finalizer

发布镜像内固定包含 `/app/supplier_admin_finalize`。CLI 只接受 `verify` 或 `finalize`，必须显式提供 MySQL/PostgreSQL `SQL_DSN`，不会启动 HTTP 服务或执行全应用 AutoMigrate。推荐用 digest-pinned release image创建临时 Cloud Run Job，复用现有 `newapi-runtime` SA、Cloud SQL attachment 和 `newapi-sql-dsn` secret：

```bash
: "${SUPPLIER_ADMIN_FINALIZE_EXPECTED_DB_IDENTITY:?export the exact verify-reported identity, for production MySQL normally mysql:newapi}"
: "${SUPPLIER_ADMIN_FINALIZE_DRAIN_EVIDENCE_REF:?export a non-secret bounded evidence reference, for example change-12345-console-router-drained}"

gcloud run jobs deploy newapi-supplier-admin-finalize \
  --project=vocai-gemini-prod --region=us-west1 \
  --image='<server-image>@sha256:<digest>' \
  --service-account='newapi-runtime@vocai-gemini-prod.iam.gserviceaccount.com' \
  --set-cloudsql-instances='vocai-gemini-prod:us-west1:newapi-mysql' \
  --set-secrets='SQL_DSN=newapi-sql-dsn:latest' \
  --set-env-vars="SUPPLIER_ADMIN_FINALIZE_EXPECTED_DB_IDENTITY=${SUPPLIER_ADMIN_FINALIZE_EXPECTED_DB_IDENTITY},SUPPLIER_ADMIN_FINALIZE_DRAIN_EVIDENCE_REF=${SUPPLIER_ADMIN_FINALIZE_DRAIN_EVIDENCE_REF}" \
  --command='/app/supplier_admin_finalize' --args='finalize' \
  --max-retries=0 --task-timeout=10m

gcloud run jobs execute newapi-supplier-admin-finalize \
  --project=vocai-gemini-prod --region=us-west1 --wait
```

随后把 args 更新为 `verify` 并执行两次。保留 execution 日志；在 runner promotion 与 rollback 窗口内保留这份可审计证据。任一命令非零时 gate 保持 disabled；禁止手工只删除一条 index。

### 供应商核算回滚

- finalizer 前：保持 gate disabled，可回滚 Console/Router/legacy 到 bridge-compatible digest；停止 Scheduler Job 不影响已发布报表。
- finalizer 后：旧 uniqueness bridge 已被显式移除，任何依赖旧 index 的 writer 都禁止恢复。普通 rollback 必须从认证 accounting status 读取严格的 `admin_command_ledger_state`；`finalized` 时目标 manifest 必须声明 current `supplier_admin_schema_capabilities=[1,...]`，且 canonical payload、expanded object、OCI/binding 和 producer capability 均通过。缺失、未知或 `invalid` 一律 fail closed。
- 已 active 后若必须切到不兼容 digest，先在同一控制事务进入 degraded 并打开具名 gap epoch，再切流；不得先切流后补 gap。
- 灾难恢复先暂停 Scheduler，保留 additive schema 和已发布 fence/evidence，恢复已接受 capability，按稳定 request ID 对账，再串行恢复 Job。不要删除汇总、批次或 coverage-gap 表。

### 直接 gcloud 切流是 break-glass，不是普通回滚

文档中的 `gcloud run services update-traffic` 示例只用于另行审批的 break-glass 事件，不是 `GCP Rollback` 的替代入口。普通回滚必须走 workflow 的 authenticated ledger-state、canonical manifest、numeric capability、OCI 和 immutable binding 门禁。确需绕过时，必须先记录事故、证据、授权人和恢复计划；active 后的不兼容目标还必须先原子提交 degraded + open gap，再切流。不得用 revision 名称、时间或排序推断兼容性。

### Supplier batch monitoring

`supplier_batch_job_enabled=true` 且至少配置一个 alert email 时，Terraform 才为现有 Router Managed Prometheus `/metrics` series 创建 supplier policies。保持 false 的合法 phase-one 状态不创建这些 policy，不会因为 Job/Scheduler 或 gauge 尚未启用而产生 absence noise。Terraform 不创建第二个 observer Job、Scheduler、service account、IAM、secret 或 log metric。

Router exporter 用数据库时间和 `Asia/Shanghai` 自然日边界读取当前 `published_fence_token = 0` 集合，发出以下精确 current gauges：

- `newapi_supplier_accounting_never_published_days`：当前从未发布日数；持续 60 秒 > 1，即当前 >=2，告警。
- `newapi_supplier_accounting_oldest_never_published_age_seconds`：当前最老从未发布日的年龄；持续 60 秒严格 >86400，告警。
- `newapi_supplier_accounting_prior_day_unpublished_after_0800`：北京时间达到 08:00 后，若前一自然日仍未发布则为 1；持续 60 秒 >0，告警。08:00 是完成 SLO，不是 02:05 schedule。
- `newapi_supplier_accounting_backlog_observer_up`：snapshot 成功为 1；全部 Router series 的 max 持续 120 秒 <1，或该 metric 持续 120 秒缺失，告警。
- `newapi_supplier_accounting_backlog_observed_at_seconds`：最近成功 snapshot 的数据库观察时间，供 freshness 和 live-evidence 对照，不替代 health/absence policy。

所有 filter 都固定 `resource.type="prometheus_target"` 和 `service=${router_service_name}`。Cloud Monitoring 只对 instance/revision series 做 `REDUCE_MAX` 并按 service 分组，禁止 SUM：这些是各节点观察同一数据库事实的副本，不是可相加的分片。阈值 condition 至少覆盖两个 30 秒 scrape；health absence 使用 120 秒，超过 90 秒且覆盖多个 scrape。

现有 RunMonitoring sidecar 每 30 秒抓取每个 Router 实例，数据库 observer 读放大会随实例数增长。G006 必须在 production-equivalent 最大 Router scale 下记录 scrape 数、snapshot 查询延迟、主库 CPU/连接/锁影响，并确认容量预算后才可发布。G006 还必须在 staging 或另行批准的 live-evidence 窗口逐项触发并恢复 backlog >=2、oldest >24h、08:00 miss、observer down 和 metric absence，保存 policy open/resolve 时间线及 `observed_at_seconds` 证据；本 Terraform 变更本身不声称已完成 live fire/resolve。

## 故障排查

### 1. 请求没有进入预期 backend

```bash
gcloud compute url-maps describe newapi-urlmap \
  --project=vocai-gemini-prod --global \
  --format='yaml(hostRules,pathMatchers,defaultService)'
```

再查 LB 日志：

```bash
gcloud logging read \
  'resource.type="http_load_balancer" AND httpRequest.requestUrl:"console.flatkey.ai"' \
  --project=vocai-gemini-prod --freshness=10m --limit=50 --format=json
```

可能原因：

- URL map 尚未传播，等待 1-3 分钟再查。
- Cloudflare DNS/Proxy 指向不对。
- Host header 与期望不同。

### 2. Cloud Run revision 健康检查失败

```bash
gcloud logging read \
  'resource.type="cloud_run_revision" AND resource.labels.service_name="<service>"' \
  --project=vocai-gemini-prod --freshness=30m --limit=100 --format=json
```

健康检查失败时不要切流量。旧 revision 继续服务。

### 3. router 5xx 或流式异常上升

先按 backend 看 LB 5xx，再看 Cloud Run logs：

```bash
gcloud logging read \
  'resource.type="http_load_balancer" AND resource.labels.backend_service_name="newapi-router-backend" AND httpRequest.status>=500' \
  --project=vocai-gemini-prod --freshness=30m --limit=100 --format=json
```

如果是新 revision 引入，优先 revision rollback；如果是 LB host 分流错误，再考虑把 `router_domains=[]` 回 default backend。

### 4. console 登录/OAuth 异常

检查：

- `console.flatkey.ai/api/status` 是否 200。
- OAuth provider redirect URI 是否包含 `https://console.flatkey.ai/...`。
- 后台 `ServerAddress` / 网站 `APP_CONSOLE_ORIGIN` 是否指向 `https://console.flatkey.ai`。

## 常用命令

```bash
# URL map
gcloud compute url-maps describe newapi-urlmap --project=vocai-gemini-prod --global \
  --format='yaml(hostRules,pathMatchers,defaultService)'

# 当前 service traffic
gcloud run services describe newapi-console --project=vocai-gemini-prod --region=us-west1 \
  --format='table(status.traffic[].revisionName,status.traffic[].percent,status.traffic[].tag)'

gcloud run services describe newapi-router --project=vocai-gemini-prod --region=us-west1 \
  --format='table(status.traffic[].revisionName,status.traffic[].percent,status.traffic[].tag)'

# LB backend logs
gcloud logging read 'resource.type="http_load_balancer" AND resource.labels.backend_service_name="newapi-console-backend"' \
  --project=vocai-gemini-prod --freshness=10m --limit=50 --format=json
```
