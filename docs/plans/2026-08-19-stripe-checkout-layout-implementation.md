# Stripe Checkout 双栏支付弹窗实现计划

> 执行约束：按测试先行顺序完成；每一阶段先看到相关测试失败，再做最小实现使其通过。

**目标：** 用 Stripe Checkout Elements 驱动 Flatkey 双栏支付弹窗，同时保持现有订单、Webhook 和回跳链路不变。

**架构：** 前端继续由 `usePayment` 管理打开/关闭会话，但会话类型从 Embedded Checkout 改为 Checkout Elements。后端解析 `ui_mode=elements` 并创建 Stripe Elements 模式会话；前端从 Checkout Session change 事件读取展示数据，Payment Element 只负责真实支付字段。

**技术栈：** Go、stripe-go v86、React 19、TypeScript、Bun、`@stripe/stripe-js` v9、Tailwind CSS、Vitest/Bun test。

---

## 任务 1：锁定后端 Checkout presentation 契约

**文件：**

- 修改：`service/subscription_invoice_test.go`
- 修改：`controller/topup_stripe_test.go`
- 修改：`service/subscription_invoice.go`
- 修改：`controller/topup_stripe.go`

**步骤：**

1. 新增失败测试：`ui_mode=elements` 在 publishable key 存在时解析为客户端 Elements 模式。
2. 新增失败测试：Elements 模式写入 `CheckoutSessionUIModeElements`、`return_url`，并清空 `success_url/cancel_url`。
3. 保留 legacy `embedded` 的现有测试，确保兼容。
4. 最小实现 presentation 模式解析和复用的 client-secret 判断。
5. 将充值创建、订阅创建和重放校验切到新的 client-secret 模式判断。
6. 运行定向 Go 测试。

## 任务 2：锁定前端请求与打开会话契约

**文件：**

- 修改：`web/default/src/features/wallet/lib/stripe-payment-request.test.ts`
- 修改：`web/default/src/features/wallet/lib/subscription-plan-lifecycle.test.ts`
- 修改：`web/default/src/features/wallet/components/subscription-plans-card.test.tsx`
- 修改：`web/default/src/features/wallet/lib/stripe-payment-request.ts`
- 修改：`web/default/src/features/wallet/lib/subscription-plan-lifecycle.ts`
- 修改：`web/default/src/features/wallet/types.ts`
- 修改：`web/default/src/features/subscriptions/types.ts`
- 修改：`web/default/src/features/wallet/hooks/use-payment.ts`

**步骤：**

1. 将测试期望改为 `ui_mode: "elements"` 并确认失败。
2. 将充值与订阅请求构造器改为 Elements 模式。
3. 将前端会话命名从 embedded 改为 checkout/elements，托管回退行为保持不变。
4. 运行请求构造与生命周期定向测试。

## 任务 3：用纯视图模型锁定动态摘要

**文件：**

- 新增：`web/default/src/features/wallet/lib/stripe-checkout-view-model.ts`
- 新增：`web/default/src/features/wallet/lib/stripe-checkout-view-model.test.ts`

**步骤：**

1. 用完整 Stripe Checkout Session fixture 写失败测试。
2. 断言产品名、描述、小计、折扣、税、附加费、总额、邮箱和 `canConfirm` 的字面量输出。
3. 断言零金额明细不会渲染。
4. 断言充值赠送只在服务端标记可展示时出现。
5. 实现无浮点金额运算的纯转换函数。

## 任务 4：实现 Checkout Elements 双栏弹窗

**文件：**

- 新增：`web/default/src/features/wallet/components/dialogs/stripe-checkout-dialog.tsx`
- 新增：`web/default/src/features/wallet/components/dialogs/stripe-checkout-dialog.test.tsx`
- 删除：`web/default/src/features/wallet/components/dialogs/stripe-embedded-checkout-dialog.tsx`
- 删除：`web/default/src/features/wallet/components/dialogs/stripe-embedded-checkout-dialog.test.tsx`
- 修改：`web/default/src/features/wallet/index.tsx`

**步骤：**

1. 更新 SDK mock，先验证 `initCheckoutElementsSdk`、Payment Element mount 和 fallback。
2. 实现 SDK 初始化、`loadActions`、Session change 订阅和元素清理。
3. 实现确认按钮调用 `actions.confirm({ redirect: "always" })`。
4. 用 repo-native Tailwind/CSS token 实现桌面双栏和移动单列布局。
5. 增加 Currency Selector Element 的条件挂载位置。
6. 保留关闭、错误、loading、disabled 状态与 hosted fallback。

## 任务 5：补齐 i18n

**文件：**

- 修改：`web/default/src/i18n/locales/en.json`
- 修改：`web/default/src/i18n/locales/zh.json`
- 修改：`web/default/src/i18n/locales/fr.json`
- 修改：`web/default/src/i18n/locales/ru.json`
- 修改：`web/default/src/i18n/locales/ja.json`
- 修改：`web/default/src/i18n/locales/vi.json`
- 修改：`web/default/src/i18n/locales/es.json`
- 修改：`web/default/src/i18n/locales/pt.json`

**步骤：**

1. 添加弹窗标题、摘要标签、安全提示、错误和按钮状态翻译。
2. 运行 `bun run i18n:sync`。
3. 检查 `_reports` 未把新增 key 标为未翻译。

## 任务 6：验证与视觉迭代

**文件：**

- 更新：`.omx/artifacts/visual-ralph/stripe-checkout-layout/README.md`
- 生成：`.omx/artifacts/visual-ralph/stripe-checkout-layout/actual-*.png`
- 生成：`.omx/artifacts/visual-ralph/stripe-checkout-layout/diff-*.png`
- 生成：`.omx/state/stripe-checkout-layout/ralph-progress.json`

**步骤：**

1. 运行 Go 定向测试和相关前端单测。
2. 运行 `bun run typecheck`、`bun run lint`、`bun run build:check`。
3. 启动本地 Go/前端预览并用测试会话打开支付弹窗。
4. 分别在 1440×900 与 390×844 截图。
5. 每次修改前记录 Visual Ralph verdict；低于 90 时按差异迭代。
6. 保存最终 verdict 与像素差异图。
7. 检查浏览器控制台无新增错误。

## 任务 7：审查与交付

**步骤：**

1. 检查工作树只包含本任务文件。
2. 进行代码审查，重点检查支付金额来源、重复提交、元素销毁、legacy 兼容与 i18n。
3. 给出部署建议：Router 不需要；`newapi-console` 与 staging 需要。
4. 按 Lore Commit Protocol 提交实现与验证证据。

