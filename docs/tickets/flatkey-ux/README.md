# 需求单 · flatkey.ai 前端 2 个 UX 问题

**指派**:shilong
**仓库**:`SolveaCX/new-api` → `web/default`(线上 flatkey.ai 前端)
**提单**:Hunter · 2026-06-18
**优先级**:P1(都在"拿 key"主转化路径上,直接影响绑卡/激活)

---

## 需求 1 · 弹窗关闭按钮(X)盖住标题文字

**现象**:首充优惠弹窗(「Parabéns — você desbloqueou uma oferta exclusiva para novos usuários」/ Top up & get up to 50% OFF)右上角的关闭按钮 **X 压在标题最后一个词上**,葡语长标题被遮住,观感很 low。
**截图**:`img/01-modal-X盖字.png`

**定位**:`web/default/src/features/onboarding/index.tsx`
- 弹窗是 Dialog,`showCloseButton`(第 ~123 行),X 是右上角绝对定位的默认关闭键。
- 标题文字没给右侧留出 X 的空间,长文案(尤其葡/西/日等长语言)就会被压。

**期望**:标题右侧留出关闭键的空间,X 不再压字。
**建议改法**(任选其一):
1. 给标题容器加右内边距 `pr-10`(给 X 让位);或
2. 把 X 关闭键往外/往上挪,不与标题行重叠;
3. 顺带检查所有语言(EN/PT/ES/JP/DE)下标题不换行压字。

**验收**:葡语等长标题下,X 与标题完全不重叠,各语言都正常。

---

## 需求 2 · 首页 Get key → Google 登录后,没自动跳到 API keys 页

**现象**:首页点 **「Get a key」** → 走 Google 登录 → 登录成功后**落在「Visão geral / 总览」页**,**没有自动跳到「Chaves de API / API keys」页**。用户的意图是"拿 key",却还要自己再点一下侧栏才能到 keys 页,多一步、流失点。
**截图流程**:
- `img/02-首页-Getkey按钮.png`(点 Get a key)
- `img/03-Google登录.png`(选账号登录)
- `img/04-登录后落在总览页错误.png`(❌ 实际落点:Visão geral 总览)
- `img/05-期望落在APIkeys页.png`(✅ 期望落点:Chaves de API)

**定位**:
- 「Get a key」按钮目标:`web/default/src/features/home/components/sections/hero.tsx`(第 ~49/130 行)→ `buildAttributionHref('/sign-up')`,即去 `/sign-up`。
- 登录(含 Google OAuth)成功后的跳转目标当前是 `/dashboard`(总览),没有区分"从 Get key 进来"的意图。

**期望**:**从「Get a key」入口发起 → 登录成功后直达 API keys 页**(Chaves de API),让"拿 key"一步到位。
**建议改法**:
1. Get key 入口带一个意图参数(如 `/sign-up?next=/console/api-keys` 或 `intent=getkey`);
2. 登录 / OAuth 回调成功后,读取该 `next` / 意图,有则跳到 API keys 页,无则保持默认(总览);
3. 注意 Google OAuth 回调要把这个 `next` 透传过去(别在跳转 Google 时丢掉)。

**验收**:首页点 Get a key → Google 登录 → **自动落在 API keys 页**;其它入口登录仍落总览,不受影响。

---

## 备注
- 两处都在 `web/default`,可一个 PR 一起改。
- 改完按 RELEASE flow 部署到 flatkey.ai 后,**外部 curl/真机回归**两条路径(长语言弹窗 + Get key 登录跳转)。
