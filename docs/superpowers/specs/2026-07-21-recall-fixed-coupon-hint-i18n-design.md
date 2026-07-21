# Recall Fixed-Coupon Hint Internationalization Design

## Problem

The recall campaign editor already renders the fixed-coupon currency warning through `t()`, but the English source string is absent from the locale catalogs. i18next therefore falls back to English for every locale:

> Stripe does not convert fixed Coupon amounts automatically. Configure each checkout currency explicitly.

Operators using a non-English interface must receive the same explanation in their selected language.

## Scope

- Keep the existing English sentence as the flat i18n key, matching the console's current convention.
- Add a natural translation to all eight locale catalogs: English, Simplified Chinese, French, Russian, Japanese, Vietnamese, Spanish, and Portuguese.
- Keep the current component, layout, Stripe behavior, supported currencies, and form behavior unchanged.
- Do not modify legacy coupons or any backend API.

## Translations

| Locale | Text |
| --- | --- |
| `en` | Stripe does not convert fixed Coupon amounts automatically. Configure each checkout currency explicitly. |
| `zh` | Stripe 不会自动换算固定金额优惠券。请为每种结账货币分别配置减免金额。 |
| `fr` | Stripe ne convertit pas automatiquement les montants fixes des coupons. Configurez explicitement chaque devise de paiement. |
| `ru` | Stripe не конвертирует фиксированные суммы купонов автоматически. Настройте каждую валюту оплаты отдельно. |
| `ja` | Stripe はクーポンの固定割引額を自動換算しません。決済に使用する各通貨の割引額を個別に設定してください。 |
| `vi` | Stripe không tự động quy đổi số tiền giảm giá cố định của phiếu giảm giá. Hãy cấu hình riêng từng loại tiền tệ thanh toán. |
| `es` | Stripe no convierte automáticamente los importes fijos de los cupones. Configura explícitamente cada moneda de pago. |
| `pt` | A Stripe não converte automaticamente os valores fixos dos cupões. Configure explicitamente cada moeda de pagamento. |

## Validation

Add a focused Bun test beside the recall campaign feature. The test will import all eight locale catalogs and assert that:

1. Every catalog contains the exact English source key.
2. Every value is non-empty.
3. Each non-English locale differs from the English fallback.

Then run the focused test, the i18n synchronization check, TypeScript type checking, and the production frontend build. This change is console-only and does not require a router deployment.
