# 需求单：注册用户召回邮件序列（Lifecycle Re-engagement Email Sequence）

> 交付对象：Shilong → Claude Code 直接落地
> 仓库：`SolveaCX/new-api`（Go 后端）
> 本单只写需求，不写代码。所有"参考位置"是给 cc 定位用的，最终实现以代码现状为准。

---

## 1. 背景与目标

广告投放的注册用户激活率几乎为 0（141 个广告归因用户里仅 1 个有 API 调用记录，外部付费 = 0）。需要一套**自动化生命周期邮件**，在注册后的关键节点召回用户，**以"充值送 credit"为勾子**，把"注册→首次调用→首次充值"的漏斗拉起来。

**目标指标**：
- 主：被触达用户的 7 日激活率（`request_count > 0` 占比）相对未触达组提升。
- 次：首次充值率提升；邮件点击率可观测。

---

## 2. 范围

**In scope（本单）**
- 后端定时调度器 + 4 封邮件 + 发送去重表 + 抑制规则 + 多语言模板 + admin 开关 + 退订。

**Out of scope（后续阶段，本单不做）**
- 打开率/点击率像素级追踪（本期只做"已发送计数" + 链接 UTM）。
- 短信 / 站内信。
- A/B 文案实验框架。

---

## 3. 受众与资格

**可进入序列的用户**（全部满足）：
1. 外部用户：排除内部账号（`group` 非 `default`/`plg`，或邮箱域名属于内部白名单 `lockin.com / voc.ai / shulex / solvea / flatkey.ai / quantumnous`，或用户名含 `test`）。
2. `email` 非空且格式合法。
3. `status` = 启用。
4. 未退订（见 §7）。

> 参考：用户字段在 `model/user.go`（`Email / CreatedAt / RequestCount / UsedQuota / Group / Status / AdsAttribution / Setting`，语言在 `Setting` JSON 的 `language`）。

---

## 4. 邮件序列（4 封，时机相对 `created_at`）

| # | 触发时机 | 发送的**前置条件** | 邮件意图 | 勾子 | CTA |
|---|---|---|---|---|---|
| **E1 当天** | 注册后 ≤ 1 小时 | 无额外条件（所有合资格新用户） | 欢迎 + 30 秒跑通第一次调用 | 首充 50% 送 credit（同款模型半价） | 「立即创建 API Key」→ `/sign-up?redirect=/keys`；「查看 Quickstart」→ `/quickstart` |
| **E2 当周** | 注册后第 3 天 | `request_count == 0`（还没调用过） | 提醒：你还没发起第一次调用 | 首充送 credit 仍有效 | 「一键试调」→ `/quickstart` |
| **E3 两周** | 注册后第 14 天 | `used_quota == 0` 或未充值（无成功充值记录） | 加码召回 | **加码**充值 bonus（比首充更高一档，文案由配置驱动） | 「去充值领 bonus」→ 充值页 `/console/topup`（以代码实际路由为准） |
| **E4 当月** | 注册后第 30 天 | 仍未充值 | 最后一次 / 临门一脚 | 限时最高档 bonus | 「最后机会，领 bonus」→ 充值页 |

**规则**：
- 每封都按用户语言本地化（§9）。
- 每封正文都要清楚写出**当前生效的充值 bonus 数值**（不要写死，从充值 bonus 配置读取，见 §6/§8）。
- 同一用户每个 step 只发一次（§5 去重）。
- 时机用"自然天偏移"，具体延迟天数做成可配置（§8），上面是默认值（0 / 3 / 14 / 30）。

---

## 5. 发送与去重机制

- **调度器**：新增一个后台定时任务（参考 `model/main.go` 启动日志里已有的同类任务，如 *subscription quota reset task* / *codex credential auto-refresh task* 的注册方式），建议**每小时**跑一次。
- 每次运行：扫描合资格用户，对每个用户计算"当前应发的 step"（基于 `created_at` 偏移 + 该 step 未发过 + §7 抑制全部通过）。命中则发送并落记录。
- **幂等**：调度器重启/重复运行**绝不重复发送**同一 (user, step)。
- **节流**：单用户每天最多收到 1 封本序列邮件；单次任务运行有批量上限（可配），避免 SMTP 被打爆。
- **补发策略**：若用户在 step 时间窗内不合资格（如已激活），则**跳过**该 step，不顺延、不补发。

---

## 6. 数据模型

新增表（跨库兼容 SQLite / MySQL / PostgreSQL，遵守仓库 Rule 2，用 GORM，主键交给 GORM；保留字列名用 `commonXxxCol` 套路）：

**`user_email_sequence`**（或同义命名）
| 字段 | 说明 |
|---|---|
| `user_id` | 用户 id（索引） |
| `step` | 1–4 |
| `status` | sent / skipped / failed |
| `sent_at` | 时间戳 |
| 唯一约束 | `(user_id, step)` 唯一 —— 去重的硬保证 |

**退订状态**：新增用户级标记（可放 `User.Setting` JSON 的 `email_opt_out: true`，或独立列 `email_opt_out bool`，cc 选更干净的一种；若加列走 AutoMigrate + 跨库默认 false）。

复用的现有字段：`created_at / request_count / used_quota / email / setting.language / status / group / ads_attribution`。
"是否充值过"：以现有充值/订单模型为准（参考 `TopUp` / 充值成功的判定逻辑），不要自己造。

---

## 7. 抑制 / 排除规则（任一命中即不发）

1. 已退订（`email_opt_out`）。
2. 内部账号（§3 的排除条件）。
3. 该 step 已发过（去重表）。
4. **已达成该 step 目标**：E2 若 `request_count > 0` 跳过；E3/E4 若已成功充值跳过。
5. 邮箱为空/非法 / 用户被禁用。
6. 全局开关 `EmailSequenceEnabled = false`。
7. SMTP 未配置 → 整个序列静默禁用（不报错刷屏）。

---

## 8. 复用现有基础设施（重要，别重造）

- **发邮件**：复用现有邮件发送能力（验证码 / 重置密码邮件走的那套 SMTP）。参考 controller 里发"重置密码"邮件的实现（用到 `system_setting.ServerAddress` 拼链接）。所有邮件里的链接 origin 一律用 `ServerAddress`，**不要写死 router.flatkey.ai**。
- **充值 bonus 数值**：从现有充值赠送配置读取（参考 `TopUpBonusClaim` / `StripeBonusClaim` / 充值 `amount_discount` / 首充 50% 横幅 `card-bind-banner` 背后的配置）。邮件文案里的 bonus 数字必须来自配置，配置改了邮件自动跟着变。
- **后台任务注册**：沿用仓库已有的定时任务注册模式（与 subscription/codex 那几个任务一致），不要新造调度框架。

---

## 9. 多语言（i18n）

- 邮件**主题 + 正文**按用户语言渲染，语言取 `User.Setting.language`，缺省回退 `en`。
- 必须覆盖投放市场语言：**en / zh / pt / es / ja / de**（fr / ru / vi 有则补全，没有先回退 en）。
- 注意：后端 go-i18n 当前只对 en/zh-CN/zh-TW/pt 做了完整翻译（见根 `CLAUDE.md` 后端 i18n 段）。邮件模板属于**用户可见、非控制台**文案，建议**邮件模板自带一套独立的多语言文案表**（每封 × 每语言一份主题+正文），不要硬塞进控制台前端 i18n。
- 禁止把英文原文当其他语言的值（历史踩过坑）。

---

## 10. 链接、UTM 与退订

- 所有 CTA 链接带 UTM：`utm_source=lifecycle_email&utm_medium=email&utm_campaign=recall&utm_content=e1|e2|e3|e4`，落地走 `/sign-up?redirect=/keys`（未登录会引导登录后直达 keys）或充值页。
- **每封邮件页脚必须有退订链接**（带签名 token，点击即置 `email_opt_out=true`，无需登录）。这是合规硬要求。
- 退订后该用户永久退出本序列。

---

## 11. 管理配置（admin）

- 全局开关：`EmailSequenceEnabled`（默认 false，配齐后由管理员开启）。
- 每个 step 可单独开关 + 延迟天数可配（默认 0/3/14/30）。
- 单次运行批量上限、单用户每日上限 可配。
- 配置走仓库现有 option/setting 体系（与其他系统设置一致），支持热更新优先。

---

## 12. 验收标准（cc 自检 + Shilong 验收）

- [ ] 新建合资格用户，1 小时内收到 E1（本地/测试 SMTP 验证）。
- [ ] 同一用户同一 step 永不重复发送（重复跑调度器验证）。
- [ ] E2 仅对 `request_count==0` 用户发；用户一旦有调用记录即被跳过。
- [ ] E3/E4 仅对未充值用户发。
- [ ] 内部账号 / 已退订 / 禁用用户 / 空邮箱 全部不发。
- [ ] 邮件链接 origin = `ServerAddress`，UTM 正确，退订链接可用且生效。
- [ ] bonus 数值来自充值配置，改配置邮件文案跟着变。
- [ ] 至少 en/zh/pt/es/ja/de 六语言邮件渲染正确，无英文串混入。
- [ ] 全局开关关闭时完全静默。
- [ ] 迁移在 SQLite / MySQL / PostgreSQL 三库均通过（Rule 2）。
- [ ] 后端 `go build ./...` 通过；新增表走 AutoMigrate；保留字列名处理正确（参考此前 backfill 因 `key` 是 MySQL 保留字踩坑）。

---

## 13. 边界与风险

- **去重幂等**是第一优先级——宁可漏发不可重复轰炸。
- 发送时段：本期不做时区择时（全天可发），节流靠"单用户每日 1 封"兜底。
- 送达率：SPF/DKIM 由现有 SMTP/域名配置保证，本单不涉及，但需在 README/PR 里提示运维确认。
- 存量用户：本序列**只对开关开启后新注册的用户**自然生效；对历史 211 个已流失用户的**一次性批量召回**是另一个独立动作（不在本单，可后续单独做一次群发）。

---

## 14. 落地顺序建议（给 cc）

1. 数据模型（去重表 + 退订标记）+ 迁移。
2. 邮件模板与多语言文案表（4 封 × N 语言）。
3. 调度器 + 资格/抑制/去重判定 + 发送。
4. 退订链接端点 + UTM。
5. admin 配置项 + 全局开关。
6. 自测覆盖 §12 验收清单。
