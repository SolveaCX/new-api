# 需求单：新用户「落地即 Playground」激活 onboarding（方案3 · aha-first）

> 落 `SolveaCX/new-api`。目标:新 plg 用户首次会话内跑通第一次调用 → **使用率 request_count>0 → 50%**。
> 给 Claude Code 直接落地。配套高保真原型:`onboarding-hifi.html`。

---

## 0. 核心结论（已实测验证,降低工程量）
1. **playground 免 key**：后端已有 `/pg/chat/completions`(`controller.Playground`,`router/relay-router.go:67`),用 `UserAuth()`(session 鉴权)+ `Distribute()`,**不需要用户先建 API key**。零配置首条消息=现成能力。
2. **plg 用户自然走 plg 组**：playground 走 `UserAuth`,用 `user.Group`(新用户=plg),Distribute 自然解析到 plg(已挂渠道 4/34/35)。无需额外改 group 逻辑。
3. **playground 调用计入激活**：经共享计费 `model.UpdateUserUsedQuotaAndRequestCount`(`model/user.go:1027`)递增 `request_count`。**即:用户在 playground 发一条成功消息 = 激活指标 +1**。

→ 所以本方案**主要是前端**,后端基本零改。

---

## 1. 流程
```
注册成功 → 重定向到 /playground?first=1（而非 dashboard）
  → playground 首跑态:预选【廉价模型】+ 欢迎提示/示例 prompt
  → 用户发首条消息 → 走 /pg(session 鉴权,免 key)→ 真实 AI 回话 ✓（request_count+1=激活）
  → 首条成功响应后 → 滑出「爱了?把它用到你的代码 → 领取 API Key」卡片
  → 点击 → /keys（自动发 key + reveal 弹窗,复用已部署的 ApiKeyRevealDialog）或 /quickstart
```

## 2. ⚠️ 关键细节:首条用【廉价模型】(决定首条会不会失败)
- 新用户免费额度 `QuotaForNewUser = $0.10`(本期不调)。**claude-opus 单条可能 > $0.10 → 首条失败**,直接毁掉 aha。
- **首跑默认模型必须用便宜的**:`gemini-2.0-flash` 或 `claude-haiku-4-5`(plg 组可达,$0.10 够发几十条)。用户之后可自己切别的。
- 兜底:若首条仍因额度失败,前端要给友好提示 + 直接引导"充值送 credit",不能白屏报错。

## 3. 前端改动（web/default,主体）
1. **注册后重定向到 /playground**：复用已有 redirect 机制(`handleLoginSuccess` 的 `redirectTo`)。给新注册(signup 成功且 `is_new_user`)设 `redirectTo=/playground?first=1`;老用户登录不变。
2. **playground 首跑态(`first=1`)**:
   - 预选廉价默认模型(gemini-2.0-flash / claude-haiku-4-5)。
   - 空状态欢迎语 + 1-2 个示例 prompt 气泡(点了自动填入并发送),如「Hello!」「用 Python 写个快排」。
   - 隐藏分组等高级控件(PLG 用户本就隐藏,沿用 `useIsEnterprise` 门控)。
3. **首条成功后的「领 key」卡片**:监听首条 assistant 响应成功 → 滑出卡片(品牌渐变)「⚡ 爱了?把它用到你自己的代码」→ 按钮「领取我的 API Key」跳 `/keys`(触发自动发 key + reveal)或 `/quickstart`。该卡片每用户只弹一次。
4. **埋点**:`flatkey_playground_first_message`(发首条)、`flatkey_playground_first_success`(首条成功=激活)、`flatkey_onboarding_get_key_click`。

## 4. 后端改动（最小）
- 基本无需改 relay。**确认项**:
  - a. 新用户 `is_new_user` 标记已在登录响应里(`setupLogin` 的 `data["is_new_user"]`,已存在)——前端据此决定重定向。
  - b. playground relay 对 plg 用户走 plg 组且计 request_count —— 上面已验证,跑通一次真实新用户确认即可。
  - c.（可选)给"首跑廉价模型"做服务端兜底:若前端传的首条模型超出免费额度可承受,后端可返回明确的 `insufficient_quota` 让前端引导充值。

## 5. 防滥用 / 成本边界
- playground 免 key 但**仍走该用户的额度 + plg 组**(廉价模型 + $0.10 上限),薅羊毛价值≈0;不需要 ephemeral key,不引入新攻击面。
- 首跑默认廉价模型进一步压成本。

## 6. 指标 / 验收
- [ ] 新注册 → 自动落 /playground(老用户登录仍落 dashboard)。
- [ ] 首跑预选廉价模型;发「Hello!」能拿到真实回话(无需建 key)。
- [ ] 该次调用使该用户 `request_count` 从 0→1(查 admin API 确认=激活计入)。
- [ ] 首条成功后弹「领 key」卡片 → 跳 /keys 能拿到 key(复用 reveal 弹窗)。
- [ ] $0.10 额度下首条不失败;若失败有友好充值引导。
- [ ] 企业/老用户行为不变。

## 7. 落地顺序（给 cc）
1. 注册后重定向 /playground?first=1（+ is_new_user 判定）。
2. playground 首跑态:预选廉价模型 + 欢迎/示例 prompt。
3. 首条成功 → 「领 key」卡片 → /keys（自动发 key + reveal）。
4. 埋点 + $0.10 兜底提示。
5. 真实新用户端到端验收(确认 request_count 计入)。

> 预期:这是把"广告用户落地后没人引导动手"直接解决——进来就和 AI 对上话(aha),再要 key。是除召回邮件外最能拉「使用率」的产品杠杆。
