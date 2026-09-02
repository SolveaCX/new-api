# TokenSpace 独立渠道类型实施计划

> 执行约束：在 `E:\workspace\new-api-worktrees\tokenspace-real-person-channel-type` 分支执行；不修改主工作树，不迁移数据库，不部署。每个行为变更先补失败测试，再写实现。

## 1. 后端类型注册与路由（RED → GREEN）

**范围**：`constant/channel.go`、`common/endpoint_type.go`、`relay/relay_adaptor.go`、相关测试。

1. 在 `common/endpoint_type_test.go` 增加 TokenSpace 视频 endpoint 用例。
2. 在 `relay/relay_adaptor_test.go` 增加 TokenSpace task adaptor 用例（断言非空且为 Doubao/Seedance adaptor）。
3. 增加一个常量边界测试，确认旧类型值不变、TokenSpace 为 114、Dummy 为 115、名称和默认 URL 已注册。
4. 先运行：
   `go test ./common ./relay ./constant`（应因缺少新类型分支失败）。
5. 在 `constant/channel.go` 插入新类型、默认 URL、名称映射并顺延 Dummy。
6. 在 endpoint switch 和 task adaptor factory 注册新类型；保持 `ChannelType2APIType` 不增加映射。
7. 运行同一组测试并用 `gofmt` 检查改动。

## 2. TokenSpace 真人 Provider 与资产边界（RED → GREEN）

**范围**：`service/real_person_provider.go`、`service/asset_reference.go`、
`middleware/distributor.go`、`controller/relay.go`、相关测试。

1. 在 `service/real_person_provider_test.go` 增加：新类型从默认/自定义 BaseURL 解析、恰好一个启用 key、禁用/多 key 拒绝、旧 Doubao 显式配置仍可用、新类型不进入自动候选的用例。
2. 在资产引用测试中增加 TokenSpace 对 image/video/audio 的消费能力和 pinned-channel 校验用例。
3. 运行：
   `go test ./service ./middleware ./controller`（新测试应先失败）。
4. 实现独立类型 Provider 解析：新类型使用渠道 BaseURL（显式 TokenSpace 配置存在时优先其 gateway），复用现有 Action Provider；保留旧 Doubao 显式配置和 BytePlus 原生分支。
5. 扩展资产类型能力与 pinned-channel 校验，不能扩大 `bytePlusAssetChannelIsUsable` 的普通素材语义；自动候选仍排除 TokenSpace。
6. 运行受影响包测试、`gofmt`，确认旧测试全部通过。

## 3. 控制器探针与渠道测试分类（RED → GREEN）

**范围**：`controller/model_availability_task.go`、`controller/channel-test.go`、相关测试。

1. 为两处异步视频分类增加断言/表驱动用例。
2. 运行对应 controller 测试确认 RED。
3. 将 TokenSpace 加入不拉取同步模型、不支持通用渠道测试的集合，运行对应测试。

## 4. default 管理端类型配置（RED → GREEN）

**范围**：`web/default/src/features/channels/constants.ts`、
`lib/channel-type-config.ts`、`lib/channel-utils.ts`、测试及 locale 文件。

1. 在 `constants.test.ts` 增加 114 的 label、顺序、默认 URL、图标和不可模型拉取断言。
2. 在 default 主题目录执行 `bun test` 的定向测试（应先失败）。
3. 注册 `114: TokenSpace`、默认 `https://api.tokenspace.net.cn`、Doubao 图标和 Seedance 异步提示；不加入 `MODEL_FETCHABLE_TYPES`。
4. 在 8 个 locale 中补齐品牌 key（值保持 `TokenSpace`），运行 i18n lint 和定向测试。

## 5. classic 管理端同步（RED → GREEN）

**范围**：`web/classic/src/constants/channel.constants.js`、
`web/classic/src/helpers/render.jsx` 及必要的 locale。

1. 增加类型选项/图标分支；沿用现有 i18next 约定，不复制 default 组件。
2. 执行 `bun run build` 和 `bun run i18n:lint`。

## 6. 集成验证与交付检查

1. 在后端工作树执行：
   `go test ./common ./constant ./relay ./service ./middleware ./controller`；
   随后 `go test ./...`（若环境依赖导致失败，记录具体包和原因）。
2. 在两个前端目录分别执行定向测试、i18n lint、build（按依赖可用性）。
3. 执行 `git diff --check`、`git status --short`，检查没有生成物和无关文件。
4. 汇总兼容性、未验证项和需要部署路由服务的说明；不执行部署/重启。

## 文件责任与回滚边界

- 常量/路由改动只新增 114 分支；旧数值 1–113 不改。
- Provider 改动保留旧显式配置路径，失败时返回原有错误语义。
- 前端改动只增加类型注册和展示，不改变既有类型表单默认值。
- 任一步骤可按 commit 回滚，不依赖数据库迁移。
