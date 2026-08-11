# Flatkey MCP OAuth Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付可通过本地 `FLATKEY_API_KEY` 或 OAuth 使用的 Flatkey Codex 插件/原生 MCP，并在 Flatkey Console 管理每个连接的专用 Key。

**Architecture:** 独立 `flatkey-mcp` 仓库负责插件与 MCP transport；`new-api` 作为 OAuth authorization server 和 data-tools resource API。OAuth access token 是短期 Ed25519 JWT，真正的 Flatkey 专用 Key 永远留在 `new-api` 服务端，并由 grant 一对一管理。

**Tech Stack:** Go 1.22、Gin、GORM、Ed25519/JWT、React 19、TanStack Router、i18next、Bun、TypeScript、Node MCP SDK、Vitest。

---

### Task 1: 持久化 OAuth client、grant、code、refresh 和专用 Token

**Files:**
- Modify: `model/token.go`
- Modify: `model/token_cache.go`
- Modify: `model/main.go`
- Create: `model/mcp_oauth.go`
- Test: `model/mcp_oauth_test.go`
- Test: `model/token_cache_test.go`
- Test: `controller/token_test.go`

- [x] **Step 1: 先写失败测试**

覆盖专用 Token 不可见、grant/token 一对一、code 单次消费、refresh rotation/replay、revoke 级联、Redis cache 完整字段和并发锁顺序。

- [x] **Step 2: 确认 RED**

Run: `go test ./model ./controller -run 'TestMcpOAuth|TestTokenCacheSchema|TestMcpOAuthDedicatedTokenHidden' -count=1`

- [x] **Step 3: 实现模型与事务**

实现 `McpOAuthClient`、`McpOAuthGrant`、`McpOAuthAuthorizationCode`、`McpOAuthRefreshToken`、`TokenSourceMcpOAuth`、`Token.OAuthGrantId`，并以数据库行锁和唯一约束保证多节点安全。

- [x] **Step 4: 确认 GREEN 并审查**

Run: `go test ./model ./controller -run 'TestMcpOAuth|TestTokenCacheSchema|TestMcpOAuthDedicatedTokenHidden' -count=1`

- [x] **Step 5: Lore commit**

已由提交 `74ae7aa7`、`f0d2725e`、`2f4e4077`、`4582ecdd` 完成。

### Task 2: 实现签名、PKCE 和 client metadata 协议服务

**Files:**
- Modify: `.env.example`
- Create: `service/mcp_oauth_signing.go`
- Test: `service/mcp_oauth_signing_test.go`
- Create: `service/mcp_oauth_protocol.go`
- Test: `service/mcp_oauth_protocol_test.go`

- [x] **Step 1: 写失败测试**

测试 PKCS#8 加载、EdDSA/kid/JWKS、固定 claims、PKCE S256、redirect/resource/scope、CIMD/DCR 和 SSRF/DNS rebinding 防护。

- [x] **Step 2: 确认 RED**

Run: `go test ./service -run 'TestMcpOAuth' -count=1 -timeout 60s`

- [x] **Step 3: 实现最小协议服务**

公开入口应保持以下边界：

```go
func LoadMcpOAuthSignerFromEnv() (*McpOAuthSigner, error)
func (s *McpOAuthSigner) SignAccessToken(claims McpOAuthAccessTokenClaims) (string, error)
func (s *McpOAuthSigner) VerifyAccessToken(raw string) (*McpOAuthAccessTokenClaims, error)
func (s *McpOAuthSigner) JWKS() McpOAuthJWKS
func VerifyMcpOAuthPKCES256(verifier, challenge string) bool
func ValidateMcpOAuthAuthorizationRequest(input McpOAuthAuthorizationRequest, client McpOAuthClientMetadata) error
func ResolveMcpOAuthClientMetadata(ctx context.Context, clientID string) (*McpOAuthClientMetadata, error)
```

- [x] **Step 4: 确认 GREEN 并双重审查**

Run: `go test ./service -run 'TestMcpOAuth' -count=1 -timeout 60s`

- [x] **Step 5: Lore commit**

已由提交 `13c354e5` 完成，待规格/质量审查通过后锁定。

### Task 3: 实现 consent、code exchange、refresh 和 revoke 生命周期

**Files:**
- Modify: `model/mcp_oauth.go`
- Test: `model/mcp_oauth_test.go`
- Create: `service/mcp_oauth_lifecycle.go`
- Test: `service/mcp_oauth_lifecycle_test.go`

- [ ] **Step 1: 写 grant+token+code 原子创建失败测试**

断言任一写入失败后不存在 grant、专用 Token 或 code；相同授权请求重试不会生成两个 active grant。

- [ ] **Step 2: 运行并确认因生命周期 API 缺失而失败**

Run: `go test ./model ./service -run 'TestMcpOAuth(Approve|Exchange|Refresh|Revoke|Connected)' -count=1 -timeout 60s`

- [ ] **Step 3: 实现模型原子 helper 与 service API**

```go
type McpOAuthApprovalInput struct {
    UserID, AccountID int
    ClientID, ClientName, RedirectURI, Scope, Resource string
    CodeChallenge, CodeChallengeMethod string
}

type McpOAuthTokenResponse struct {
    AccessToken, TokenType, Scope, RefreshToken string
    ExpiresIn int64
}

func ApproveMcpOAuthAuthorization(ctx context.Context, in McpOAuthApprovalInput) (redirectURL string, err error)
func ExchangeMcpOAuthAuthorizationCode(ctx context.Context, code, redirectURI, verifier, clientID, resource string) (*McpOAuthTokenResponse, error)
func RefreshMcpOAuthAccessToken(ctx context.Context, refreshToken, clientID, scope, resource string) (*McpOAuthTokenResponse, error)
func RevokeMcpOAuthCredential(ctx context.Context, token, hint, clientID string) error
func ListMcpOAuthConnectedApps(ctx context.Context, userID int) ([]McpOAuthConnectedApp, error)
func RevokeMcpOAuthConnectedApp(ctx context.Context, userID int, grantPublicID string) error
func ResolveMcpOAuthDataToolIdentity(ctx context.Context, claims *McpOAuthAccessTokenClaims) (*McpOAuthDataToolIdentity, error)
```

生成专用 Token 时默认 `RemainQuota=500000`、`UnlimitedQuota=true`，允许环境覆盖；返回值、日志和 OAuth response 不得包含其 Key。

- [ ] **Step 4: 确认 GREEN、并发与失败关闭**

Run: `go test ./model ./service -run 'TestMcpOAuth(Approve|Exchange|Refresh|Revoke|Connected|DataTool)' -count=1 -timeout 60s`

- [ ] **Step 5: Lore commit 并做规格/质量双审**

### Task 4: 暴露标准 OAuth 与登录态管理 HTTP API

**Files:**
- Create: `controller/mcp_oauth.go`
- Test: `controller/mcp_oauth_test.go`
- Create: `router/mcp_oauth.go`
- Test: `router/mcp_oauth_test.go`
- Modify: `router/main.go`
- Modify: `router/api-router.go`

- [ ] **Step 1: 写失败路由/响应测试**

覆盖 `/.well-known/oauth-authorization-server`、`/oauth/jwks`、`/oauth/register`、`/oauth/token`、`/oauth/revoke`，以及登录态 `/api/oauth/authorization-details`、`/api/oauth/authorize`、`/api/user/connected-apps`、`/api/user/connected-apps/:grant_id/revoke`。

- [ ] **Step 2: 确认 RED**

Run: `go test ./controller ./router -run 'TestMcpOAuth' -count=1 -timeout 60s`

- [ ] **Step 3: 实现薄 controller 和路由**

标准 OAuth form/JSON 错误结构统一为：

```go
type mcpOAuthErrorResponse struct {
    Error            string `json:"error"`
    ErrorDescription string `json:"error_description,omitempty"`
}
```

根路径标准端点不挂 `UserAuth`，但使用全局 API 限流与请求体限制；consent/Connected Apps API 挂 `middleware.UserAuth()`。`/oauth/authorize` 留给 SPA 页面，不与 API controller 抢路由。

- [ ] **Step 4: 确认 GREEN 与未知 token revoke=200**

Run: `go test ./controller ./router -run 'TestMcpOAuth' -count=1 -timeout 60s`

- [ ] **Step 5: Lore commit 并做规格/质量双审**

### Task 5: 将 OAuth 限定接入 data-tools 三条路由

**Files:**
- Modify: `constant/context_key.go`
- Create: `middleware/mcp_oauth.go`
- Test: `middleware/mcp_oauth_test.go`
- Modify: `router/api-router.go`
- Modify: `controller/data_tools.go`
- Test: `controller/data_tools_test.go`

- [ ] **Step 1: 写失败认证与 scope 测试**

覆盖 API Key 回归、用户 session 回归、有效 OAuth、错误 issuer/audience/resource/kid/alg、revoked grant、三个 scope、`oauth_grant_id` context，以及 OAuth JWT 对非 data-tools 路由不可用。

- [ ] **Step 2: 确认 RED**

Run: `go test ./middleware ./controller ./router -run 'TestMcpOAuthDataTool|TestDataToolAuth' -count=1 -timeout 60s`

- [ ] **Step 3: 实现 data-tools 专用 middleware**

```go
const ContextKeyOAuthGrantId ContextKey = "oauth_grant_id"

func DataToolTokenOrUserAuth(requiredScope string) gin.HandlerFunc
func DataToolTokenAuth(requiredScope string) gin.HandlerFunc
```

middleware 先保留现有 session/API Key 路径，只在 bearer 符合 JWT 形态时验证 OAuth；验证成功后由 service 取回 grant 绑定的专用 Token，并填充现有 `id`、`token_id`、`token_key`、`token_unlimited_quota` 与 `ContextKeyOAuthGrantId`。不得修改全局 `TokenAuth()` 来接受 OAuth。

- [ ] **Step 4: 按路由绑定 scopes 并确认 GREEN**

```go
dataToolRoute.GET("", middleware.DataToolTokenOrUserAuth(service.McpOAuthScopeToolsSearch), controller.ListDataTools)
dataToolRoute.GET("/inspect", middleware.DataToolTokenOrUserAuth(service.McpOAuthScopeToolsRead), controller.InspectDataTool)
dataToolRoute.POST("/run", middleware.DataToolTokenAuth(service.McpOAuthScopeToolsExecute), controller.RunDataTool)
```

Run: `go test ./middleware ./controller ./router -run 'TestMcpOAuthDataTool|TestDataToolAuth' -count=1 -timeout 60s`

- [ ] **Step 5: Lore commit 并做规格/质量双审**

### Task 6: 实现 OAuth consent 与 Connected Apps UI

**Files:**
- Create: `web/default/src/features/mcp-oauth/api.ts`
- Create: `web/default/src/features/mcp-oauth/types.ts`
- Create: `web/default/src/features/mcp-oauth/authorize-page.tsx`
- Create: `web/default/src/features/mcp-oauth/connected-apps-page.tsx`
- Create: `web/default/src/routes/oauth/authorize.tsx`
- Create: `web/default/src/routes/_authenticated/connected-apps/index.tsx`
- Modify: `web/default/src/hooks/use-sidebar-data.ts`
- Modify: `web/default/src/i18n/locales/{en,zh,es,fr,ja,pt,ru,vi}.json`

- [ ] **Step 1: 写失败的纯函数/组件测试或类型契约**

至少锁定 OAuth query 保留、scope label、redirect error 处理和 revoke 后列表刷新；无法直接运行组件测试时，将 URL/响应解析抽成纯函数并先测。

- [ ] **Step 2: 确认 RED**

Run: `bun test src/features/mcp-oauth`

- [ ] **Step 3: 实现页面与 API**

授权页未登录时把完整当前 URL 放入既有登录 redirect 参数；登录后获取 authorization details，显示客户端、资源、scopes，批准/拒绝后只跳服务端返回的已验证 URL。Connected Apps 加入主侧栏 personal 分组，显示名称、scope、创建/最后使用/状态，撤销有确认对话框。

- [ ] **Step 4: 写八语言真实翻译并验证**

Run: `bun run i18n:sync`

检查 `_reports/{lang}.untranslated.json` 不包含本任务新增 key；品牌名和协议 literal 可保持不翻译。

- [ ] **Step 5: typecheck/build/smoke**

Run: `bun run typecheck`

Run: `bun run build`

- [ ] **Step 6: Lore commit 并做规格/质量双审**

### Task 7: 文档、最终验证与 GitHub 交付

**Files:**
- Modify: `E:/workspace/flatkey-mcp/README.md`
- Modify: `README.md` 或部署文档（仅在 NewAPI 环境变量需要说明时）
- Verify: `E:/workspace/flatkey-mcp/.codex-plugin/plugin.json`
- Verify: `E:/workspace/flatkey-mcp/.mcp.json`

- [ ] **Step 1: 核对中英文 README**

中文和英文都必须分别说明：插件安装、`FLATKEY_API_KEY` 快速路径、无 Key OAuth、原生 MCP 配置、权限、撤销、开发与部署。

- [ ] **Step 2: MCP/插件全量验证**

Run from `E:/workspace/flatkey-mcp`: `npm test && npm run typecheck && npm run build && npm audit`

Run plugin validator and secret scan；确认 `.tmp/` 未跟踪。

- [ ] **Step 3: NewAPI 全量验证**

Run: `go test ./...`

Run from `web/default`: `bun run i18n:sync && bun run typecheck && bun run build`

Run from `web/classic`: `bun run build`

Run: `git diff --check`

Run: `gitnexus detect-changes --scope compare --base-ref main`；索引不可用时记录具体工具缺口。

- [ ] **Step 4: 最终安全/规格/质量审查**

确认没有 secret、普通 Token API 不暴露专用 Key、OAuth 不能访问非 data-tools API、并发 refresh/revoke fail closed、所有用户文案有八语言值。

- [ ] **Step 5: 推送**

独立仓库：`git push origin main`

NewAPI：`git push -u origin feat/flatkey-mcp-oauth`

交付时列出 GitHub URL、提交 SHA、验证证据，以及 DNS/TLS/真实 Codex OAuth E2E 是否仍为生产发布门禁。
