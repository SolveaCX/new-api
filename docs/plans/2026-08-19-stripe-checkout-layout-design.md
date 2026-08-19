# Stripe Checkout 双栏支付弹窗设计

## 状态

- 状态：已批准
- 日期：2026-08-19
- 视觉基准：`.omx/artifacts/visual-ralph/stripe-checkout-layout/reference-1440x900.png`
- 移动端基准：`.omx/artifacts/visual-ralph/stripe-checkout-layout/reference-390x844.png`
- 批准依据：用户在当前任务中明确要求“照着这个干”

## 目标

把现有 Stripe Embedded Checkout 整表嵌入弹窗替换成 Flatkey 自有的双栏支付弹窗，并由 Stripe Checkout Elements 在左栏动态渲染真实支付方式字段。

所有现有支付业务语义保持不变：订单创建、套餐与充值金额、赠送额度、优惠、开票、Webhook、支付成功回跳、订单恢复和余额刷新仍由现有链路负责。

## 不在范围内

- 不改 Stripe 价格、币种、支付方式或风控配置。
- 不在前端自行计算应付金额、税费、折扣或汇率。
- 不改变订单状态机、Webhook 幂等或余额入账逻辑。
- 不新增外部依赖或新的设计系统。

## 已确认的产品决策

- 所有 Stripe 支付方式和币种使用同一套弹窗布局。
- 用户邮箱由 Checkout Session 提供并以锁定状态展示。
- 左栏支付字段由 Stripe Payment Element 动态决定，不能写死 Pix、卡或本地支付字段。
- 右栏订单摘要来自 Stripe Checkout Session 的格式化金额与行项目数据。
- 确认按钮由 Flatkey 渲染，但确认动作调用 Stripe Checkout Elements 的 `confirm`。
- 成功与需要跳转的支付继续使用现有 `return_url`，不改变回跳后的订单恢复逻辑。
- Stripe SDK 或 Elements 初始化失败时，仅使用响应中明确提供并经过 URL 校验的托管链接回退；Stripe Elements Session 本身不生成托管 URL，因此无安全回退时关闭弹窗并展示错误，不创建第二个可支付 Session。

## 桌面布局

- 视口基准：1440 × 900。
- 弹窗主体为最大宽度约 1120px、圆角 24px、带柔和阴影的白色卡片。
- 两栏比例约 58:42。
- 左栏：Stripe 安全提示、确认付款标题、锁定邮箱、支付方式标题、Payment Element 容器。
- 右栏：浅灰渐变背景、产品名、主金额、周期/描述、动态明细、确认按钮、安全提示和法律链接。
- 关闭按钮位于卡片右上角，使用紫色描边与外圈光晕，键盘可访问。

## 移动布局

- 视口基准：390 × 844。
- 900px 以下改为单列；520px 以下为全宽、无外层圆角的沉浸式布局。
- 左栏在上，右栏在下；关闭按钮固定在内容右上角。
- Payment Element、摘要和按钮保持 100% 宽度，避免横向滚动。

## 视觉令牌

- 页面/遮罩底色：`#f4f6f8` 一类的冷灰。
- 主表面：`#ffffff`；摘要表面：`#f7f8fa`。
- 文字：`#20242a`；次要文字：`#646a73`。
- 分隔线：`#dfe3e8`。
- 主按钮：Flatkey 蓝，沿用仓库 `primary` token。
- 安全状态：青绿色；关闭按钮强调：紫色。
- 卡片圆角：24px；字段/摘要圆角：12–14px；主按钮圆角：8px。
- 阴影保持轻量，复用 Tailwind/CSS 变量，不建立新的全局 token 层。

## 数据与交互契约

### Checkout Session

后端在请求 `ui_mode: "elements"` 且配置 Stripe publishable key 时创建 `ui_mode=elements` 的 Checkout Session，返回：

- `client_secret`
- `publishable_key`
- 现有 `topup_summary`（充值时）

缺少 publishable key 时保持托管 Checkout 回退。

### 左栏

- SDK：`stripe.initCheckoutElementsSdk({ clientSecret, elementsOptions })`。
- 元素：`createPaymentElement()`；如 Checkout Session 暴露多币种选择，则渲染 Currency Selector Element。
- 邮箱：展示 `session.email`，不可编辑。
- Payment Element 的就绪、错误和 Stripe Session change 事件驱动加载与按钮状态。

### 右栏

- 产品名与描述：Stripe `lineItems[0]`。
- 小计：`session.total.subtotal.amount`。
- 折扣、税、附加费：仅在对应 minor-unit 金额非零时显示。
- 主金额：首个行项目的小计（无行项目时使用 `session.total.subtotal.amount`）；应付总额：`session.total.total.amount`。这样手续费或税费仍只在明细和应付总额中体现。
- 充值赠送信息继续使用服务端 `topup_summary`，且只在其 `show_amounts` 为真时展示。
- 前端不做浮点金额运算。

### 确认

- `session.canConfirm` 为真且未提交时按钮可用。
- 点击后调用 `actions.confirm({ redirect: "always" })`，继续由现有 `return_url` 处理完成态。
- 同一次提交期间禁用按钮并显示处理中状态，避免重复确认。
- Stripe 返回可展示错误时在弹窗内展示，同时保留全局错误提示。

## 可访问性

- 弹窗使用现有 Dialog 焦点陷阱和 Esc 关闭语义。
- 关闭和确认按钮必须有可读名称与清晰焦点样式。
- 邮箱标签和锁定状态对屏幕阅读器可理解。
- 加载与错误信息使用 `role=status` / `role=alert`。
- 所有新增用户文案进入 8 个语言文件。

## 验收标准

- 充值、一次性订阅和 Stripe recurring 都请求 `ui_mode: "elements"`。
- 后端生成 `CheckoutSessionUIModeElements`，并保留 legacy `embedded` 请求兼容。
- 桌面与移动布局分别匹配批准的两张基准图，Visual Ralph 最终评分不低于 90。
- 支付字段、币种、摘要和确认状态随 Stripe Session 更新。
- 原有托管 Checkout、恢复订单、Webhook 和回跳查询参数行为不回归。
- Go 定向测试、前端单测、typecheck、lint/build 与本地预览验证通过。
