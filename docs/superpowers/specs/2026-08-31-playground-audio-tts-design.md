# Playground 通用 TTS 音频闭环设计

日期：2026-08-31  
状态：已获用户批准，待实施

## 目标

让 Playground 中已授权、且由网关 `RelayModeAudioSpeech` 支持的文本转语音模型可以完成一条可验证的闭环：选择模型、输入文本、生成音频、在消息内播放并下载。首期覆盖 Gemini TTS，以及现有 MiniMax 和火山 TTS 适配器；不改变公开 `/v1/audio/speech` 合约。

## 非目标

- 不在本期接入 ElevenLabs 原生 voice/SFX（它要求 `/v1/text-to-speech/:voice_id` 或 `/v1/sound-generation`，请求契约不同）。
- 不接入语音转文字、翻译、Realtime 或音频输入能力。
- 不把不具备 TTS 适配器的模型仅因为名字含有 `audio`/`tts` 就暴露出来。

## 方案与数据流

### 前端模型与请求

1. 将 `PlaygroundModelKind` 增加 `audio`，为已支持的 TTS 名称解析出音频 profile；TTS 模型从通用不支持列表移到可选择列表，并保持音频输入模型（例如 GPT Audio preview）仍属于 chat。
2. 音频 profile 使用最小参数集：`voice`（默认 `alloy`）和 `response_format`（默认 `mp3`）；文本来自 Playground 输入框。请求体包含 `model`、`group`、`input`、`voice`、`response_format`，必要时发送 `speed`。
3. 新增 Playground 路由 `POST /pg/audio/speech`。前端 API 以二进制 Blob 接收响应，不把音频 base64 放入持久化记录。

### 后端路由与适配器

1. `router/relay-router.go` 在 Playground 组注册 `POST /audio/speech`，控制器复用现有 `preparePlayground` 与 `RelayFormatOpenAIAudio` 生命周期。
2. 现有 MiniMax/火山适配器继续走各自的 `ConvertAudioRequest` 和响应处理。
3. Gemini 适配器新增 TTS 转换：把 `AudioRequest` 转为 Gemini `generateContent` 请求，`responseModalities=["AUDIO"]`，并设置 `speechConfig` 的 prebuilt voice；URL 仍由现有 Gemini `GetRequestURL` 生成。
4. Gemini TTS 响应处理只接受候选内容中的 audio `inlineData`，校验 base64 后解码为二进制，依据上游 MIME 设置 `Content-Type`（未知 MIME 使用 `audio/wav`）。没有音频数据时返回明确的上游响应错误，不把空响应当作成功。使用现有 usage metadata 结算，缺少 metadata 时回退到输入估算。
5. 不改变 ElevenLabs native 路由或其计费逻辑。

### 播放、下载与生命周期

- `sendMediaGeneration` 为 audio 请求使用 `responseType: 'blob'`。
- Hook 为 Blob 创建 `URL.createObjectURL`，在当前消息中渲染 `<audio controls>` 和下载入口；删除消息、停止生成、组件卸载或替换结果时撤销 object URL。
- 音频 object URL 只存在于当前会话内，不写入 Playground 持久化记录；恢复历史记录时保留“已生成音频”的文本状态，但不伪造失效 URL。
- 音频失败沿用现有 media generation 错误状态和 toast，不泄漏上游供应商内部地址或响应体。

## 错误处理与安全边界

- 前端仅将 `Blob.type` 归一化为允许的音频 MIME；非音频 Blob 视为失败。
- 后端拒绝空文本；`voice` 和 `response_format` 采用 OpenAI TTS 的默认值（`alloy`/`mp3`），具体 provider 的音色校验仍由对应适配器和上游负责。
- 所有新增 JSON 序列化/反序列化使用 `common.Marshal` / `common.Unmarshal`，遵守仓库 Rule 1。
- `/pg/audio/speech` 继续经过现有鉴权、分组、渠道选择、预扣费和结算；不引入进程内状态，因此适配多节点部署。

## 测试与验收

### 单元/集成测试

- 路由测试确认 `POST /pg/audio/speech` 已注册。
- Gemini adaptor 测试确认 TTS 请求的模型、文本、`responseModalities`、voice 和 MIME 映射；响应测试覆盖有效音频、缺失 inlineData、非法 base64、usage 回退。
- 前端模型过滤/profile 测试确认 Gemini/MiniMax/火山 TTS 可选，ElevenLabs native、音频输入模型和音频任务模型不会误分类。
- 前端 media hook/API 测试确认 Blob 转 object URL、渲染数据和 URL 撤销路径。

### 正式环境验收

至少用一个已授权 Gemini TTS 模型和一个已授权的现有 TTS 模型各发送一条短文本，确认：HTTP 200、响应 `Content-Type` 为音频、浏览器可播放/下载、Playground 不再把请求发到 `/pg/chat/completions`。若某个模型未配置对应渠道能力，选择器应隐藏它或显示明确的 unsupported 错误，而不是静默重试。

## 风险与回滚

变更同时触及路由、Gemini provider 和 Playground SPA，需部署 router 与 console；无数据库迁移。若 Gemini 上游 TTS 契约在某区域不可用，可仅关闭该模型的 TTS profile，保留通用 `/pg/audio/speech` 和其他 provider。
