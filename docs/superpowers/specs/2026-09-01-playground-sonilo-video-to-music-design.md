# Playground Sonilo Video-to-Music 接入设计

日期：2026-09-01
状态：已实施，待正式环境冒烟

## 目标

让 `sonilo-video-to-music` 在 Playground 的 `plg` 分组中可选择，并完成“本地视频上传 → 异步生成 → 音频播放器/下载 → 会话恢复”的闭环。首期只支持 Playground 内上传的本地视频；公开 `/v1/video-to-music` 合约和现有图片、普通视频、TTS 链路保持不变。

## 非目标

- 首期不提供 `video_url` 输入框；用户仍可通过 Playground 的本地文件选择器提交视频。
- 首期固定生成一个变体（`variants_num=1`），不提供 `segments` 编辑器。
- 不重写 Sonilo 已有的上游适配器、计费规则或公开 API。

## 方案与数据流

### 后端路由

新增 Playground 别名 `POST /pg/video-to-music` 与 `GET /pg/video-to-music/:task_id`，控制器复用现有 Playground 鉴权、分组、渠道选择、预扣费、任务持久化和 Sonilo task adaptor。路径规范化后使用 `EndpointTypeVideoToMusic`，因此只会选择支持该端点的 Sonilo 渠道。现有 `/pg/videos` 和公开 `/v1/video-to-music` 不变。完成音频继续通过现有匿名能力 URL `/v1/video-to-music/:task_id/content?variant=0` 代理，避免把 Sonilo 上游地址返回给浏览器。

### 前端模型与请求

为 `sonilo-video-to-music` 增加独立媒体 profile：输入类型为单个视频，文件选择器仅接受 `video/mp4`；模型从通用 unsupported 列表移出并归类为可用音频任务。提交前沿用现有 Playground 附件上传流程，使用刷新后的签名预览 URL 作为 multipart 的 `video_url`，并发送 `model`、`group`、`prompt`、`duration_seconds`、`output_format`、`mode=async`、`preserve_speech`、`ducking` 和 `variants_num=1`。视频时长优先由浏览器元数据读取；旧会话或元数据不可读时显示 1–3600 秒的手动参数，不使用静默固定值。

### 任务、播放与持久化

提交响应和轮询响应使用 Sonilo 专用解析器，识别 `queued/processing/succeeded/failed` 以及 `audio[]`。成功后将每个返回音频（首期实际为一个）写为 `GeneratedMedia` 的 `audio` 类型和代理 URL，复用现有 `<audio controls>`、下载和安全 URL 处理。正在轮询的消息持久化公开 task ID 及任务类型，刷新后选择正确的 Sonilo 查询端点继续轮询；完成或失败时清理任务标记。用户输入视频仍只保存 asset ID，签名 URL 不落库；代理音频 URL可直接恢复，不创建短生命周期 blob URL。

## 交互与错误处理

- 没有附件、超过一个附件、附件不是视频或混入其他类型时，在上传/扣费前拒绝并提示。
- 时长缺失、非有限数值或超出 1–3600 秒时拒绝提交。
- multipart 边界由浏览器/HTTP 客户端生成，API 层不能强行覆盖 `Content-Type`。
- 轮询沿用 3 秒间隔、200 次上限；取消、超时、上游失败均显示明确错误并结束 loading 状态。
- 只接受后端相对代理 URL或安全 HTTPS URL；禁止将 Sonilo 域名、密钥或原始错误响应写入消息。

## 测试与验收

### 自动化测试

- 前端 profile/模型过滤、视频附件约束、时长序列化、FormData 字段和 multipart 请求类型。
- Sonilo 响应解析覆盖提交中、处理中、成功（含音频列表）、失败和缺失音频；hook 覆盖轮询恢复、取消、超时和 audio 消息更新。
- 路由测试确认 Playground POST/GET 注册；端点选择测试确认 `/pg/video-to-music` 只命中 Sonilo；现有普通视频测试继续通过。

### 正式环境验收

在已发布版本中选择 `plg` 分组和 `sonilo-video-to-music`，上传一个短 mp4，确认请求成功创建任务、任务完成后出现可播放/可下载的音频控件，刷新会话后仍可播放，并检查浏览器和返回消息中没有 Sonilo 上游地址泄漏。

## 风险与回滚

变更涉及 Playground SPA、任务路由和测试，无数据库迁移。若正式环境的 Sonilo 渠道未配置或不可用，模型应由分组模型列表自然隐藏；回滚时移除新增 Playground 路由和前端 profile 即可，不影响其他媒体模型。
