# TokenSpace 独立渠道类型设计

## 背景

目前 106 渠道记录使用 `ChannelTypeDoubaoVideo`，但真人认证已经通过显式
`tokenspace_material` Provider 走 TokenSpace Action API。这样会把两个上游的
配置、可用性和路由语义混在同一个渠道类型里；同时历史渠道记录不能被迁移或
破坏。

本次把 TokenSpace 注册为独立的、可持久化的渠道类型，并保留旧
`DoubaoVideo` + 显式 Provider 配置的兼容路径。

## 目标与非目标

目标：

- 新增 `ChannelTypeTokenSpace`（值 114），并将保留哨兵顺延到 115。
- 新类型使用 TokenSpace 默认网关，能够被渠道选择器、任务路由和真人认证
  Provider 正确识别。
- 两套管理端可以创建、编辑和展示该类型；异步视频类型不参与同步模型拉取。
- 让已有 106 记录继续工作，不修改数据库中的渠道类型或设置。
- 用后端和前端回归测试锁定新旧行为。

非目标：

- 不新增 APIType；该类型是 Seedance 兼容的异步视频渠道。
- 不新增一套重复的任务协议适配器；复用现有 Doubao/Seedance 任务适配器。
- 不把 TokenSpace 加入真人认证的自动随机候选；真人认证仍需显式绑定/锁定。
- 不自动为新类型开启普通素材库物化；普通素材仍通过现有显式素材 Provider
  配置启用。

## 方案选择

### 方案 A（采用）：独立类型注册 + 复用任务适配器 + 类型化 Provider 选择

在 channel constants、endpoint 映射、task adaptor 分发和 Provider 解析处增加
TokenSpace 类型分支。新类型没有显式素材配置时，真人 Provider 从该渠道的
`BaseURL` 和 API key 解析；旧 DoubaoVideo 记录继续只在显式
`tokenspace_material` 配置下走 TokenSpace。普通 Seedance 请求复用
`taskdoubao.TaskAdaptor`，因为请求/轮询协议相同。

优点是行为边界清晰、无需数据迁移、兼容已有记录，且不会复制协议代码。

### 方案 B（不采用）：新增独立 task adaptor 包

为 TokenSpace 复制一套 Doubao adaptor，仅修改名称和默认 URL。当前两套上游
使用同一生成任务协议，复制会造成后续修复分叉，且不能解决 Provider/资产绑定
边界问题。

### 方案 C（不采用）：继续使用 DoubaoVideo，仅增加渠道 ID 特判

这会保留当前耦合，新的渠道仍无法按类型管理和路由；也会把业务语义绑定到
易变的数据库 ID，不符合此前“可独立出来 type”的要求。

## 设计细节

### 类型与默认值

`ChannelTypeTokenSpace = 114` 插入现有自定义类型和 `ChannelTypeDummy` 之间；
`ChannelBaseURLs[114]` 为 `https://api.tokenspace.net.cn`，名称映射为
`TokenSpace`。旧数值保持不变，只有哨兵值顺延。

### 请求路由

- `GetEndpointTypesByChannelType` 将新类型归入 `OpenAIVideo`。
- `GetTaskAdaptor` 将新类型分发到现有 Doubao/Seedance adaptor。
- 不映射到 `common.ChannelType2APIType`，因此不会被当作同步 OpenAI API 渠道。
- 渠道测试和不测试探针列表将新类型按异步视频处理。

### 真人认证 Provider

Provider 选择顺序如下：

1. 新 TokenSpace 类型：使用显式素材配置中的 TokenSpace gateway（若存在），
   否则使用渠道 `BaseURL`；要求恰好一个启用的渠道 key。
2. 旧 DoubaoVideo：保持现有显式 `tokenspace_material` 配置路径。
3. 其他类型：保持 BytePlus 原生 Provider 逻辑。

新类型不进入自动真人认证候选。现有 TokenSpace Action、multipart 临时对象
存储和回调状态机继续复用同一 Provider 接口。

### 资产绑定与锁定

TokenSpace Provider 可消费真人认证所需的 image/video/audio 资产；选择器和
任务 relay 的 pinned-channel 校验接受可用的 TokenSpace 类型。BytePlus 原生
素材能力判断不扩大到 TokenSpace，避免误把普通素材请求路由到不具备配置的渠道。

### 管理端

- default 主题：增加类型标签、默认 URL、Doubao/Seedance 图标和类型配置；不
  加入可同步拉取模型的类型集合。
- classic 主题：增加类型下拉项和图标渲染分支。
- 新增的品牌标签在所有现有 locale 中保持 key 完整。

## 风险与缓解

- **旧数据回归**：保留旧 Provider 分支并增加测试，绝不按渠道 ID 特判。
- **哨兵值变化**：所有范围循环使用 `ChannelTypeDummy`，增加边界测试确认新值
  可遍历且旧值未变。
- **协议误判**：新类型明确标记为异步视频，不添加同步 APIType 映射。
- **误选普通素材**：仅真人 Provider 能力扩展；普通素材仍需显式配置。

## 验证策略

- Go：类型常量/名称/默认 URL、endpoint、task adaptor、Provider 解析、资产
  消费与 pinned-channel 校验的单元测试；运行受影响包测试和 `go test ./...`。
- 前端：default constants/config/icon 测试，classic 构建与 i18n lint。
- 最终检查：`git diff --check`、前端测试/构建；不执行部署、重启或数据库操作。
