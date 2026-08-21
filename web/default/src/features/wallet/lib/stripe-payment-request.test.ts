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
import { describe, expect, test } from 'bun:test'
import { readFileSync } from 'node:fs'
import { resolveStripeCheckoutOpening } from './stripe-checkout-opening'
import { buildStripePaymentRequest } from './stripe-payment-request'

const redirectUrls = {
  success_url: 'https://app.example.com/wallet?show_history=true',
  cancel_url: 'https://app.example.com/wallet',
}

describe('buildStripePaymentRequest', () => {
  test('requests Checkout Elements when the in-console checkout is preferred', () => {
    const request = buildStripePaymentRequest({
      amount: 20,
      redirectUrls,
      preferElementsCheckout: true,
    })

    expect(request.ui_mode).toBe('elements')
  })

  test('sends USD as the default Stripe checkout currency', () => {
    const request = buildStripePaymentRequest({
      amount: 20,
      redirectUrls,
      gaIdentifiers: {
        ga_client_id: 'client-1',
        ga_session_id: 'session-1',
      },
    })

    expect(request).toEqual({
      amount: 20,
      payment_method: 'stripe',
      stripe_currency: 'USD',
      ga_client_id: 'client-1',
      ga_session_id: 'session-1',
      ...redirectUrls,
    })
  })

  test('uses the explicit Stripe currency when provided', () => {
    const request = buildStripePaymentRequest({
      amount: 200,
      stripeCurrency: 'JPY',
      redirectUrls,
    })

    expect(request.stripe_currency).toBe('JPY')
  })

  test('keeps promo card binding fields with the default currency', () => {
    const request = buildStripePaymentRequest({
      amount: 200,
      saveCard: true,
      redirectUrls,
    })

    expect(request.save_card).toBe(true)
    expect(request.stripe_currency).toBe('USD')
  })

  test('includes a recall claim only when one is provided', () => {
    const claimedRequest = buildStripePaymentRequest({
      amount: 20,
      redirectUrls,
      recallClaim: 'signed-claim-value',
    })
    const regularRequest = buildStripePaymentRequest({
      amount: 20,
      redirectUrls,
    })

    expect(claimedRequest.recall_claim).toBe('signed-claim-value')
    expect(regularRequest).not.toHaveProperty('recall_claim')
  })
})

describe('resolveStripeCheckoutOpening', () => {
  test('uses an explicit safe fallback URL for an Elements session', () => {
    expect(
      resolveStripeCheckoutOpening({
        client_secret: 'cs_test_fallback',
        publishable_key: 'pk_test_fallback',
        fallback_url: 'https://checkout.example.com/fallback',
      })
    ).toEqual({
      kind: 'elements',
      clientSecret: 'cs_test_fallback',
      publishableKey: 'pk_test_fallback',
      fallbackUrl: 'https://checkout.example.com/fallback',
    })
  })

  test('preserves a complete revision contract for an Elements session', () => {
    expect(
      resolveStripeCheckoutOpening({
        client_secret: 'cs_test_revision',
        publishable_key: 'pk_test_revision',
        fallback_url: 'https://checkout.example.com/fallback',
        checkout_context: 'signed-context',
        checkout_revision: 2,
        discount_state: {
          source: 'manual',
          display_name: 'SAVE20',
          promotion_code_masked: 'SAVE20',
          replaced_source: 'invitation',
        },
        topup_summary: {
          pay_amount: 30,
          bonus_amount: 0,
          credit_amount: 30,
          show_amounts: true,
        },
      })
    ).toEqual({
      kind: 'elements',
      clientSecret: 'cs_test_revision',
      publishableKey: 'pk_test_revision',
      fallbackUrl: 'https://checkout.example.com/fallback',
      checkoutContext: 'signed-context',
      checkoutRevision: 2,
      discountState: {
        source: 'manual',
        display_name: 'SAVE20',
        promotion_code_masked: 'SAVE20',
        replaced_source: 'invitation',
      },
      summary: {
        pay_amount: 30,
        bonus_amount: 0,
        credit_amount: 30,
        show_amounts: true,
      },
    })
  })

  test('drops incomplete revision contracts so legacy responses cannot show the control', () => {
    expect(
      resolveStripeCheckoutOpening({
        client_secret: 'cs_test_partial',
        publishable_key: 'pk_test_partial',
        checkout_context: 'signed-context',
        checkout_revision: 2,
      })
    ).toEqual({
      kind: 'elements',
      clientSecret: 'cs_test_partial',
      publishableKey: 'pk_test_partial',
    })
  })

  test('does not infer an Elements fallback from legacy hosted fields', () => {
    expect(
      resolveStripeCheckoutOpening({
        client_secret: 'cs_test_1',
        publishable_key: 'pk_test_1',
        pay_link: 'https://pay.example.com/hosted',
      })
    ).toEqual({
      kind: 'elements',
      clientSecret: 'cs_test_1',
      publishableKey: 'pk_test_1',
    })
  })

  test('falls back to checkout_url before hosted_invoice_url', () => {
    expect(
      resolveStripeCheckoutOpening({
        checkout_url: 'https://checkout.example.com/session',
        hosted_invoice_url: 'https://invoice.example.com/invoice',
      })
    ).toEqual({
      kind: 'hosted',
      url: 'https://checkout.example.com/session',
    })
  })

  test('falls back to hosted_invoice_url when checkout_url is missing', () => {
    expect(
      resolveStripeCheckoutOpening({
        hosted_invoice_url: 'https://invoice.example.com/invoice',
      })
    ).toEqual({
      kind: 'hosted',
      url: 'https://invoice.example.com/invoice',
    })
  })
})

describe('usePayment Stripe checkout adapter', () => {
  test('keeps top-up response handling as a thin adapter preserving summary data', () => {
    const source = readFileSync(
      new URL('../hooks/use-payment.ts', import.meta.url),
      'utf8'
    )

    expect(source).toContain('const openStripeCheckoutResponse = useCallback')
    expect(source).toContain('openStripeCheckout(response.data, {')
    expect(source).toContain('summary: response.data?.topup_summary ?? null')
  })
})
