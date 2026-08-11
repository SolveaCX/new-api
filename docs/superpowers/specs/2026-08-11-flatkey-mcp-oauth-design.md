# Flatkey Codex 插件与 MCP OAuth 设计

## 目标

让用户用同一套 Flatkey Tools 能力走两条接入路径：

1. 在 Codex 中安装 Flatkey 插件后直接调用远程 MCP；
2. 在任意支持 MCP 的客户端中手工配置同一个远程 MCP。

客户端存在 `FLATKEY_API_KEY` 时直接使用该 Key，不进入浏览器授权。客户端没有 Key 时，由 MCP 返回 OAuth challenge，Codex/客户端跳转到 Flatkey Console 完成登录与授权。README 同时提供中文和英文说明。

## 仓库边界

- `think-back/flatkey-mcp`：独立、可分发的 Codex 插件与远程 MCP server，包含插件清单、marketplace 元数据、MCP 工具实现、部署样例和中英双语 README。
- `SolveaCX/new-api`：Flatkey Console 的 OAuth authorization server、专用 Key 生命周期、data-tools 双认证和 Connected Apps UI。
- MCP server 不持有或返回用户的 Flatkey 专用 Key。API Key 模式由客户端直接发送 Key；OAuth 模式只向客户端签发短期 JWT access token 和可轮换 refresh token。

## 固定协议

- Issuer：`https://console.flatkey.ai`
- Resource/Audience：`https://mcp.flatkey.ai`
- Authorization endpoint：`https://console.flatkey.ai/oauth/authorize`
- Token endpoint：`https://console.flatkey.ai/oauth/token`
- Revocation endpoint：`https://console.flatkey.ai/oauth/revoke`
- JWKS endpoint：`https://console.flatkey.ai/oauth/jwks`
- Scopes：`tools:search`、`tools:read`、`tools:execute`
- Authorization code：5 分钟、单次使用、PKCE S256
- Access token：Ed25519 JWT，15 分钟
- Refresh family：30 天，每次使用后 rotation；旧 refresh token 重放时撤销整个 family
- 私钥环境变量：`FLATKEY_MCP_OAUTH_ED25519_PRIVATE_KEY`，内容为 PKCS#8 PEM

协议按 MCP Authorization 2026-07-28 实现：客户端元数据优先使用 CIMD，兼容 DCR；authorization 与 token 请求都要求精确 `resource`；公共客户端只允许 `token_endpoint_auth_method=none`；撤销未知 token 仍按 RFC 7009 返回成功。

## 身份与 Key 生命周期

### API Key 快速路径

Codex 插件的 MCP server 声明 `bearer_token_env_var: FLATKEY_API_KEY`。本机存在该环境变量时，客户端直接把 Key 作为 bearer token 发送到 MCP。MCP 再以相同凭据调用 Flatkey data-tools API，不创建 OAuth grant，也不要求用户登录。

### OAuth 路径

用户同意授权时，在一个数据库事务中创建：

1. 一个只属于本次连接的 `McpOAuthGrant`；
2. 一个隐藏的 `TokenSourceMcpOAuth` 专用 Flatkey Token，并用 `OAuthGrantId` 一对一绑定；
3. 一个只保存哈希、5 分钟有效、单次消费的 authorization code。

专用 Token 不得出现在普通 Token list/get/search/batch/update/delete 接口，也不得被普通 `ValidateUserToken` 接受。它只能由服务端在 data-tools OAuth 路径内解析 grant 后使用。grant 创建失败必须回滚全部对象；撤销 grant 必须在一个事务内禁用专用 Token、未消费 code 和整个 refresh family。

专用 Token 的默认额度使用明确的 MCP 平台策略，而不是复制用户任意现有 Key。默认值为 `RemainQuota=500000`、`UnlimitedQuota=true`，并允许通过环境配置覆盖；该选择需要在部署说明和提交记录中保留。

## 服务边界

- `model/`：OAuth client/grant/code/refresh 的持久化、唯一约束、事务、行锁和跨数据库兼容；不返回 HTTP 语义。
- `service/`：Ed25519/JWT、PKCE、CIMD/DCR 校验、consent approval、code exchange、refresh rotation、revoke 和专用 Token 调用边界；不向 controller 返回 `*gorm.DB`。
- `controller/`：只做输入绑定、当前用户读取、调用 service 和标准 OAuth/JSON 响应映射。
- `router/`：标准 OAuth 根路径公开端点和 `/api` 下的登录态 Connected Apps/consent 端点。
- `middleware/`：data-tools 的 API Key/OAuth bearer 判别、JWT 验证、scope enforcement 和 context 注入。

所有 JSON 编解码使用 `common.Marshal`、`common.Unmarshal` 或 `common.DecodeJson`。所有数据库逻辑同时支持 SQLite、MySQL 5.7.8+ 和 PostgreSQL 9.6+；并发正确性依赖数据库事务/行锁/唯一约束，不依赖进程内锁。

## HTTP 与 UI

公开端点：authorization-server metadata、protected-resource metadata、JWKS、DCR register、authorization redirect、token、revoke。登录态 API：读取 consent 信息、批准/拒绝、列出当前用户 Connected Apps、撤销一个连接。

`/oauth/authorize` 在未登录时保留完整 OAuth 参数并跳到登录；登录后显示独立 consent 页面，明确客户端名、Flatkey Tools 资源和所请求 scopes。批准后创建连接并回跳精确注册的 redirect URI；拒绝后返回标准 `access_denied`。

Profile 左栏新增 Connected Apps。列表只显示客户端名、scopes、创建时间、最后使用时间和状态；撤销需要确认，成功后立即刷新。UI 与 toast/错误文案全部走 `t()`，在 `en/zh/es/fr/ja/pt/ru/vi` 八个 locale 中提供真实翻译。

## Data-tools 双认证

- `search` 要求 `tools:search`
- `inspect` 要求 `tools:read`
- `run` 要求 `tools:execute`

API Key bearer 保留现有行为。JWT bearer 必须校验算法、签名、issuer、audience、resource、过期时间、grant 状态与 scope，并在 context 中写入 `oauth_grant_id`。context key 只在 `constant/context_key.go` 定义。data-tools service 根据 grant 在服务端取得专用 Token；不得把 secret 放进 response、Node MCP、Codex、UI 或日志。每次成功调用更新 grant 的 `last_used_at`，写入必须适合多节点并发。

## 安全与错误处理

- redirect URI 必须与注册值精确匹配；禁止通配符与前缀匹配。
- CIMD URL 获取仅允许 HTTPS，限制响应大小/超时/跳转，并在每次解析与跳转时拒绝回环、私网、链路本地、保留地址和 DNS rebinding。
- client、scope、resource、PKCE、code、refresh 失败使用标准 OAuth 错误，避免泄露账号、Key 或内部数据库状态。
- access token `kid` 必须能在 JWKS 找到；私钥缺失或非法时服务 fail closed。
- refresh rotation 与 revoke 使用一致的 grant→refresh 锁顺序；重放或并发失败后不得留下可用 refresh token。
- 日志仅记录公开 grant/client ID、错误类别和 trace ID，不记录 authorization code、refresh token、JWT、专用 API Key 或完整 redirect query。

## 验证与交付

- Go：模型并发/缓存/隐藏边界、签名/JWKS、PKCE、客户端注册、生命周期、controller/router、middleware/scope 测试；随后 `go test ./...`。
- 前端：typecheck、i18n sync/status、`web/default` 与 `web/classic` 构建；对 consent 和 Connected Apps 走一次浏览器 smoke。
- 插件/MCP：Vitest、typecheck、build、插件 validator、secret scan、`npm audit`。
- 代码审查：每个实现切片先规格审查，再质量审查；最终做整分支审查和 GitNexus impact 检查。
- GitHub：独立 MCP 仓库推送 `main`；NewAPI 推送 `feat/flatkey-mcp-oauth`。除非实际完成 DNS/TLS、部署和真实 Codex OAuth E2E，否则不得宣称生产已上线。
