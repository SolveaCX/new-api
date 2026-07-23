/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { beforeAll, describe, expect, test } from 'bun:test'
import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import type { PlanRecord } from '@/features/subscriptions/types'
import {
  buildFlexibleQuoteRequest,
  getMatchingPaymentQuote,
  mergeFlexibleQuoteProjection,
  normalizeSelfSubscriptionData,
} from '../lib/subscription-plan-lifecycle'
import type { TopupInfo } from '../types'
import { PlanPurchaseDialog } from './plan-purchase-dialog'
import { SubscriptionPlansCard } from './subscription-plans-card'

const testI18n = createInstance()

beforeAll(async () => {
  await testI18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })
})

const topupInfo = {
  enable_online_topup: true,
  enable_stripe_topup: true,
  pay_methods: [],
  min_topup: 1,
  stripe_min_topup: 1,
  amount_options: [],
  discount: {},
  bonus: {},
} satisfies TopupInfo

function plan(id: number, title: string, price: number): PlanRecord {
  return {
    plan: {
      id,
      title,
      price_amount: price,
      currency: 'USD',
      duration_unit: 'month',
      duration_value: 1,
      quota_reset_period: 'monthly',
      enabled: true,
      sort_order: id,
      allow_balance_pay: true,
      max_purchase_per_user: 0,
      total_amount: price * 1000,
      window_5h_amount: price * 100,
      window_week_amount: price * 250,
      media_credits_monthly: price,
      payment_modes: ['stripe_recurring', 'balance_one_period'],
    },
  }
}

const plans = [plan(1, 'Go', 10), plan(2, 'Pro', 20), plan(3, 'Max', 40)]

function renderWalletCard(selfData = normalizeSelfSubscriptionData(undefined)) {
  return renderToStaticMarkup(
    <I18nextProvider i18n={testI18n}>
      <SubscriptionPlansCard
        topupInfo={topupInfo}
        initialPlans={plans}
        initialSelfData={selfData}
        initialLoading={false}
        userQuota={12345}
      />
    </I18nextProvider>
  )
}

describe('SubscriptionPlansCard flexible wallet plan UI', () => {
  test('hides the current plan module when there is no active plan and shows Go Pro Max first', () => {
    const html = renderWalletCard()

    expect(html).not.toContain('Current subscription')
    expect(html).not.toContain('No active plan')
    expect(html).not.toContain('Choose a plan now')
    expect(html.indexOf('Go')).toBeLessThan(html.indexOf('Pro'))
    expect(html.indexOf('Pro')).toBeLessThan(html.indexOf('Max'))
    expect(html).toContain('Buy now')
  })

  test('renders a read-only current card with correct badges and exactly four usage meters', () => {
    const html = renderWalletCard(
      normalizeSelfSubscriptionData({
        contract: {
          contract_id: 9,
          id: 9,
          status: 'active',
          payment_mode: 'prepaid',
          current_plan_id: 2,
          current_entitlement_id: 20,
          current_provider_binding_id: 0,
          latest_change_intent_id: 0,
          pending_plan_id: 0,
          pending_effective_at: 0,
          current_period_start: 1717200000,
          current_period_end: 1719792000,
          grace_period_end: 0,
          change_version: 1,
        },
        renewal_source: 'wallet_auto',
        renewal_status: 'enabled',
        current_entitlement: {
          entitlement_id: 20,
          plan_id: 2,
          provider_binding_id: 0,
          status: 'active',
          payment_mode: 'balance_one_period',
          start_time: 1717200000,
          end_time: 1719792000,
          access_end_time: 1719792000,
        },
        quota: {
          amount_total: 20000,
          amount_used: 7000,
          amount_remaining: 13000,
          unlimited: false,
        },
        window_5h: { used: 200, total: 2000, remaining: 1800, reset_at: 1 },
        window_7d: { used: 1000, total: 5000, remaining: 4000, reset_at: 1 },
        media_credits: { used: 3, total: 20, remaining: 17, reset_at: 1 },
      })
    )

    expect(html).toContain('Current plan')
    expect(html).toContain('Pro')
    expect(html).toContain('Active')
    expect(html).toContain('Auto-renew enabled')
    expect(html.match(/data-wallet-usage-meter=/g)?.length).toBe(4)
    expect(html.match(/data-wallet-secondary-meter=/g)?.length).toBe(3)
    expect(html).not.toContain('Cancel auto-renewal')
    expect(html).not.toContain('Resume auto-renewal')
    expect(html).not.toContain('Manage')
    expect(html).not.toContain('Renewal time')
  })

  test('does not show the Flatkey wallet auto-renew badge for provider recurring contracts', () => {
    const html = renderWalletCard(
      normalizeSelfSubscriptionData({
        contract: {
          contract_id: 10,
          id: 10,
          status: 'active',
          payment_mode: 'stripe_recurring',
          current_plan_id: 2,
          current_entitlement_id: 20,
          current_provider_binding_id: 88,
          latest_change_intent_id: 0,
          pending_plan_id: 0,
          pending_effective_at: 0,
          current_period_start: 1717200000,
          current_period_end: 1719792000,
          grace_period_end: 0,
          change_version: 1,
        },
        renewal_source: 'provider_recurring',
        renewal_status: 'enabled',
      })
    )

    expect(html).toContain('Active')
    expect(html).not.toContain('Auto-renew enabled')
  })

  test('renders zero media credits as not included instead of unlimited', () => {
    const html = renderWalletCard(
      normalizeSelfSubscriptionData({
        contract: {
          contract_id: 11,
          id: 11,
          status: 'active',
          payment_mode: 'prepaid',
          current_plan_id: 2,
          current_entitlement_id: 20,
          current_provider_binding_id: 0,
          latest_change_intent_id: 0,
          pending_plan_id: 0,
          pending_effective_at: 0,
          current_period_start: 1717200000,
          current_period_end: 1719792000,
          grace_period_end: 0,
          change_version: 1,
        },
        media_credits: {
          used: 0,
          total: 0,
          remaining: 0,
          reset_at: 0,
          unlimited: false,
        },
      })
    )

    expect(html).toContain('Not included')
    expect(html).toContain('0 remaining')
  })

  test('uses repurchase for the same plan and switch for every other active plan without next-period copy', () => {
    const html = renderWalletCard(
      normalizeSelfSubscriptionData({
        contract: {
          contract_id: 10,
          id: 10,
          status: 'active',
          payment_mode: 'stripe_recurring',
          current_plan_id: 2,
          current_entitlement_id: 20,
          current_provider_binding_id: 88,
          latest_change_intent_id: 0,
          pending_plan_id: 0,
          pending_effective_at: 0,
          current_period_start: 1717200000,
          current_period_end: 1719792000,
          grace_period_end: 0,
          change_version: 1,
        },
      })
    )

    expect(html).toContain('Repurchase now')
    expect(html.match(/Switch now/g)?.length).toBe(2)
    expect(html).not.toContain('Downgrade next period')
    expect(html).not.toContain('next period')
  })
})

describe('PlanPurchaseDialog payment choices', () => {
  test('defaults to Stripe recurring and hides the month selector', () => {
    const html = renderToStaticMarkup(
      <I18nextProvider i18n={testI18n}>
        <PlanPurchaseDialog
          open
          plan={plans[1]}
          currentPlanId={0}
          paymentAvailability={{}}
          selectedPaymentChoice='stripe_recurring'
          months={1}
          onOpenChange={() => undefined}
          onConfirm={() => undefined}
        />
      </I18nextProvider>
    )

    expect(html).toContain('Stripe automatic subscription')
    expect(html.indexOf('Stripe automatic subscription')).toBeLessThan(
      html.indexOf('Alipay')
    )
    expect(html.indexOf('Alipay')).toBeLessThan(html.indexOf('Pix'))
    expect(html.indexOf('Pix')).toBeLessThan(html.indexOf('UPI'))
    expect(html.indexOf('UPI')).toBeLessThan(html.indexOf('Flatkey balance'))
    expect(html).not.toContain('1 month')
    expect(html).not.toContain('12 months')
  })

  test('reveals a 1-12 month selector for one-time payment choices', () => {
    for (const selectedPaymentChoice of [
      'alipay',
      'pix',
      'upi',
      'balance',
    ] as const) {
      const html = renderToStaticMarkup(
        <I18nextProvider i18n={testI18n}>
          <PlanPurchaseDialog
            open
            plan={plans[1]}
            currentPlanId={2}
            paymentAvailability={{}}
            selectedPaymentChoice={selectedPaymentChoice}
            months={6}
            onOpenChange={() => undefined}
            onConfirm={() => undefined}
          />
        </I18nextProvider>
      )

      expect(html).toContain('1 month')
      expect(html).toContain('12 months')
      expect(html).toContain('No prorating or credit is applied.')
      expect(html).not.toContain('future months')
    }
  })

  test('uses backend quote snapshots for Pix BRL and UPI INR display amounts', () => {
    const pixHtml = renderToStaticMarkup(
      <I18nextProvider i18n={testI18n}>
        <PlanPurchaseDialog
          open
          plan={plans[1]}
          currentPlanId={0}
          paymentAvailability={{}}
          paymentQuotes={{
            pix: {
              currency: 'BRL',
              months: 3,
              unit_price: 1234.56,
              total: 3703.68,
              quote_id: 'quote-pix-3',
            },
            upi: {
              currency: 'INR',
              months: 3,
              unit_price: 1800,
              total: 5400,
              quote_id: 'quote-upi-3',
            },
          }}
          selectedPaymentChoice='pix'
          months={3}
          onOpenChange={() => undefined}
          onConfirm={() => undefined}
        />
      </I18nextProvider>
    )
    const upiHtml = renderToStaticMarkup(
      <I18nextProvider i18n={testI18n}>
        <PlanPurchaseDialog
          open
          plan={plans[1]}
          currentPlanId={0}
          paymentAvailability={{}}
          paymentQuotes={{
            pix: {
              currency: 'BRL',
              months: 3,
              unit_price: 1234.56,
              total: 3703.68,
              quote_id: 'quote-pix-3',
            },
            upi: {
              currency: 'INR',
              months: 3,
              unit_price: 1800,
              total: 5400,
              quote_id: 'quote-upi-3',
            },
          }}
          selectedPaymentChoice='upi'
          months={3}
          onOpenChange={() => undefined}
          onConfirm={() => undefined}
        />
      </I18nextProvider>
    )

    expect(pixHtml).toContain('Unit price')
    expect(pixHtml).toContain('R$')
    expect(pixHtml).toContain('1.234,56')
    expect(pixHtml).toContain('3.703,68')
    expect(upiHtml).toContain('Unit price')
    expect(upiHtml).toContain('₹1,800.00')
    expect(upiHtml).toContain('₹5,400.00')
    expect(pixHtml).not.toContain('$60')
    expect(upiHtml).not.toContain('$60')
  })

  test('keeps Pix selectable when a quote is missing and disables only Continue', () => {
    const html = renderToStaticMarkup(
      <I18nextProvider i18n={testI18n}>
        <PlanPurchaseDialog
          open
          plan={plans[1]}
          currentPlanId={0}
          paymentAvailability={{}}
          paymentQuotes={{}}
          selectedPaymentChoice='pix'
          months={3}
          onOpenChange={() => undefined}
          onConfirm={() => undefined}
        />
      </I18nextProvider>
    )

    expect(html).toContain('Local currency quote is unavailable.')
    expect(html).not.toContain('$60')
    expect(html).toContain('checked="" value="pix"')
    expect(html).toContain('disabled=""')
    expect(html).not.toContain('checked="" disabled="" value="pix"')
  })

  test('shows local quote loading while keeping the selected choice active', () => {
    const html = renderToStaticMarkup(
      <I18nextProvider i18n={testI18n}>
        <PlanPurchaseDialog
          open
          plan={plans[1]}
          currentPlanId={0}
          paymentAvailability={{}}
          paymentQuotes={{}}
          selectedPaymentChoice='upi'
          months={2}
          isQuoteLoading
          onOpenChange={() => undefined}
          onConfirm={() => undefined}
        />
      </I18nextProvider>
    )

    expect(html).toContain('Loading local currency quote')
    expect(html).not.toContain('$40')
    expect(html).toContain('checked="" value="upi"')
  })
})

describe('flexible local-currency quote interaction helpers', () => {
  test('selecting Pix and UPI creates quote requests for the selected months', () => {
    expect(
      buildFlexibleQuoteRequest({
        planId: 2,
        paymentChoice: 'pix',
        months: 3,
        requestId: 'request-1',
      })
    ).toEqual({
      plan_id: 2,
      payment_choice: 'pix',
      months: 3,
      request_id: 'request-1',
    })

    expect(
      buildFlexibleQuoteRequest({
        planId: 2,
        paymentChoice: 'upi',
        months: 12,
        requestId: 'request-1',
      }).months
    ).toBe(12)
  })

  test('uses only the quote matching the selected local-currency months', () => {
    const quotes = {
      pix: {
        currency: 'BRL',
        months: 1,
        unit_price: 100,
        total: 100,
        quote_id: 'quote-pix-1',
      },
    }

    expect(getMatchingPaymentQuote('pix', quotes, 3)).toBeUndefined()
    expect(getMatchingPaymentQuote('pix', quotes, 1)?.quote_id).toBe(
      'quote-pix-1'
    )
  })

  test('month changes accept only the latest matching quote response', () => {
    const current = {
      status: 'applied' as const,
      payment_quotes: {
        pix: {
          currency: 'BRL',
          months: 1,
          unit_price: 100,
          total: 100,
          quote_id: 'quote-pix-1',
        },
      },
    }

    const latest = {
      sequence: 2,
      paymentChoice: 'pix' as const,
      months: 3,
      requestId: 'request-1',
    }

    const stale = mergeFlexibleQuoteProjection(
      current,
      {
        payment_quotes: {
          pix: {
            currency: 'BRL',
            months: 2,
            unit_price: 200,
            total: 400,
            quote_id: 'quote-pix-2',
          },
        },
      },
      { ...latest, sequence: 1 },
      latest
    )
    const accepted = mergeFlexibleQuoteProjection(
      current,
      {
        payment_quotes: {
          pix: {
            currency: 'BRL',
            months: 3,
            unit_price: 1234.56,
            total: 3703.68,
            quote_id: 'quote-pix-3',
          },
        },
      },
      latest,
      latest
    )

    expect(stale).toBe(current)
    expect(accepted?.payment_quotes?.pix?.quote_id).toBe('quote-pix-3')
  })
})
