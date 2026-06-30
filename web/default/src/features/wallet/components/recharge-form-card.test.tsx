import { beforeAll, describe, expect, test } from 'bun:test'
import i18n from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { initReactI18next } from 'react-i18next'
import type { TopupInfo } from '../types'
import { RechargeFormCard } from './recharge-form-card'

const topupInfoWithStripe: TopupInfo = {
  enable_online_topup: false,
  enable_stripe_topup: true,
  pay_methods: [{ name: 'Stripe Card', type: 'stripe', min_topup: 1 }],
  min_topup: 1,
  stripe_min_topup: 1,
  amount_options: [],
  discount: {},
  bonus: {},
  enable_redemption: false,
}

describe('RechargeFormCard', () => {
  beforeAll(async () => {
    await i18n.use(initReactI18next).init({
      lng: 'en',
      fallbackLng: 'en',
      resources: {
        en: {
          translation: {
            'Top up for {{amount}}': 'Top up for {{amount}}',
          },
        },
      },
      interpolation: {
        escapeValue: false,
      },
    })
  })

  test('renders website-style USD Stripe top-up plans without the legacy add-funds flow', () => {
    const html = renderToStaticMarkup(
      <RechargeFormCard
        topupInfo={{
          ...topupInfoWithStripe,
          amount_options: [10, 20, 200],
        }}
        presetAmounts={[{ value: 10 }, { value: 20 }, { value: 200 }]}
        selectedPreset={null}
        onSelectPreset={() => undefined}
        onStripeTopUp={() => undefined}
        paymentLoadingAmount={null}
        redemptionCode=''
        onRedemptionCodeChange={() => undefined}
        onRedeem={() => undefined}
        redeeming={false}
      />
    )

    expect(html).toContain('Top-up Packages')
    expect(html).toContain('$10 USD')
    expect(html).toContain('$20 USD')
    expect(html).toContain('$200 USD')
    expect(html).toContain('Top up for $10')
    expect(html).toContain('Top up for $20')
    expect(html).toContain('Top up for $200')
    expect(html).not.toContain('Top Up')
    expect(html).toContain('Lowest entry to get started')
    expect(html).toContain('Most Popular')
    expect(html).toContain('40% OFF')
    expect(html).toContain('3X more usage than the official plan')
    expect(html).toContain('50% OFF')
    expect(html).toContain('40X more usage than the official plan')
    expect(html).not.toContain(
      'Best first top-up for trying real API workloads with a clear discount.'
    )
    expect(html).not.toContain(
      'Best value for production testing, team workflows, and sustained model traffic.'
    )
    expect(html).not.toContain('Add Funds')
    expect(html).not.toContain('Choose an amount and payment method')
    expect(html).not.toContain('Need company invoice')
    expect(html).not.toContain('Order History')
    expect(html).not.toContain('Waffo Pix')
  })
})
