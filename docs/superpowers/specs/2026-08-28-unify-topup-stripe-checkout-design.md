# 充值 Stripe Checkout 与订阅页面统一设计

## 状态

- 状态：已获用户批准（A 方案）
- 日期：2026-08-28
- 范围：只统一充值的 Checkout 页面与交互，不合并充值和订阅订单流程

## 目标

修复充值点击套餐后无法拉起 Stripe Checkout 的问题，并确保充值成功创建
`ui_mode=elements` 会话后，使用与订阅套餐完全相同的 Flatkey Checkout Elements
双栏页面、确认动作、错误处理和托管页面回退策略。

## 不在范围内

- 不修改订阅支付接口、订阅订单状态机、Recurring 参数或订阅 Webhook。
- 不把充值订单改造成订阅订单，也不共用余额入账逻辑。
- 不改变充值金额、币种选择、优惠码/Recall、开票和 `topup_summary` 语义。
- 不部署生产环境或重启服务。

## 现状与边界

充值和订阅前端已经共用
`web/default/src/features/wallet/components/dialogs/stripe-checkout-dialog.tsx`。
充值通过 `usePayment.openStripeCheckout` 打开它，订阅流程直接使用同一组件；差异
仅限于各自创建订单和向页面传入的摘要数据。因此本次改动不创建新的支付页面，
只修正充值会话创建失败的参数，并用测试锁定共享入口。

## 设计

### 1. 后端 Checkout 参数

保留充值 Stripe 创建器现有的 `ui_mode` 分支。对于 Elements/客户端渲染会话，
不设置 Stripe 不支持的 `custom_text`；对于托管 Checkout，继续保留提交提示文本。
这样只影响充值创建器的参数合法性，不触碰订阅创建器。

### 2. 前端页面入口

充值请求继续显式发送 `ui_mode: "elements"`，收到 `client_secret` 和
`publishable_key` 后走共享 `StripeCheckoutDialog`。共享页面负责 Payment Element、
币种选择、摘要、优惠码、确认和安全回退；充值的 `topup_summary` 只作为摘要输入。

订阅现有调用路径和参数保持原样。任何后续页面样式变化应继续只修改共享组件，
从而让充值和订阅同时得到相同的页面行为。

### 3. 生命周期与关闭

保留现有交易号、Checkout revision、折扣状态、关闭未支付订单和回跳恢复逻辑。
不为同一充值订单创建第二个支付会话；Elements 挂载失败时仅使用服务端明确提供
且已校验的托管回退地址，否则关闭并提示失败。

## 错误处理

- Stripe Elements 会话创建失败：沿用现有充值错误提示，不影响订阅错误处理。
- Elements 参数不被 Stripe 接受：由后端测试阻止 `custom_text` 回归。
- 前端挂载失败：沿用共享对话框的安全回退/关闭策略。
- 用户关闭未支付 Checkout：沿用现有关闭接口和订单状态处理。

## 验证方案

1. Go：验证充值 Elements 参数不包含 `custom_text`，托管模式仍包含它；运行充值
   Stripe 定向测试。
2. 前端：验证充值请求发送 `ui_mode=elements`，响应通过共享
   `StripeCheckoutDialog` 打开并保留 `topup_summary`、折扣和回退字段。
3. 订阅回归：运行订阅 Stripe 请求和共享 Checkout 对话框相关测试，确认订阅请求
   仍发送 `ui_mode=elements` 且参数/页面契约未变化。
4. 工作树与 PR：只提交本功能文件，记录未执行生产部署；合并后仅需部署
   `newapi-console`（Router 无需变更）。

## 验收标准

- 充值套餐点击可以创建并打开与订阅相同的 Checkout Elements 页面。
- 充值金额、优惠码、Recall、赠送额度、订单号和余额入账行为不变。
- 订阅 Stripe 支付、订阅 Webhook 和订阅页面测试全部通过。
- Elements 模式不再向 Stripe 发送 `custom_text`，托管模式行为保持不变。
