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
import type {
  Stripe,
  StripeCheckoutElementsSdk,
  StripeCheckoutLoadActionsSuccess,
  StripeCheckoutSession,
  StripeCurrencySelectorElement,
  StripePaymentElement,
} from '@stripe/stripe-js'
import { describe, expect, mock, test } from 'bun:test'
import { mountStripeCheckoutElements } from './stripe-checkout-elements'

function createCheckoutSession(): StripeCheckoutSession {
  return {
    billingAddress: null,
    businessName: null,
    canConfirm: true,
    currency: 'brl',
    currencyOptions: [
      { currency: 'brl', minorUnitsAmount: 4990, amount: 'R$49.90' },
      { currency: 'usd', minorUnitsAmount: 999, amount: '$9.99' },
    ],
    discountAmounts: null,
    email: 'buyer@example.com',
    id: 'cs_test_elements',
    lastPaymentError: null,
    lineItems: [],
    livemode: false,
    minorUnitsAmountDivisor: 100,
    phoneNumber: null,
    recurring: null,
    savedPaymentMethods: null,
    shipping: null,
    shippingAddress: null,
    shippingOptions: [],
    status: { type: 'open' },
    surcharge: null,
    tax: { status: 'ready' },
    taxAmounts: null,
    taxIdInfo: null,
    total: {
      appliedBalance: { minorUnitsAmount: 0, amount: 'R$0.00' },
      balanceAppliedToNextInvoice: false,
      discount: { minorUnitsAmount: 0, amount: 'R$0.00' },
      shippingRate: { minorUnitsAmount: 0, amount: 'R$0.00' },
      subtotal: { minorUnitsAmount: 4990, amount: 'R$49.90' },
      surcharge: { minorUnitsAmount: 0, amount: 'R$0.00' },
      taxExclusive: { minorUnitsAmount: 0, amount: 'R$0.00' },
      taxInclusive: { minorUnitsAmount: 0, amount: 'R$0.00' },
      total: { minorUnitsAmount: 4990, amount: 'R$49.90' },
    },
  }
}

describe('mountStripeCheckoutElements', () => {
  test('mounts Stripe-owned fields, publishes session changes, confirms, and cleans up', async () => {
    const session = createCheckoutSession()
    const paymentContainer = {} as HTMLElement
    const currencyContainer = {} as HTMLElement
    const mountPayment = mock(() => undefined)
    const destroyPayment = mock(() => undefined)
    const mountCurrency = mock(() => undefined)
    const destroyCurrency = mock(() => undefined)
    const confirm = mock(async () => ({ type: 'success', session }) as const)
    const applyPromotionCode = mock(
      async () => ({ type: 'success', session }) as const
    )
    const removePromotionCode = mock(
      async () => ({ type: 'success', session }) as const
    )
    let changeHandler: ((next: StripeCheckoutSession) => void) | undefined

    const paymentElement = {
      mount: mountPayment,
      destroy: destroyPayment,
    } as unknown as StripePaymentElement
    const currencyElement = {
      mount: mountCurrency,
      destroy: destroyCurrency,
    } as unknown as StripeCurrencySelectorElement
    const actions = {
      getSession: () => session,
      confirm,
      applyPromotionCode,
      removePromotionCode,
    } as unknown as StripeCheckoutLoadActionsSuccess
    const checkout = {
      on: (
        _event: 'change',
        handler: (next: StripeCheckoutSession) => void
      ) => {
        changeHandler = handler
      },
      loadActions: async () => ({ type: 'success', actions }) as const,
      createPaymentElement: () => paymentElement,
      createCurrencySelectorElement: () => currencyElement,
    } as unknown as StripeCheckoutElementsSdk
    const initCheckoutElementsSdk = mock(() => checkout)
    const stripe = {
      initCheckoutElementsSdk,
    } as unknown as Stripe
    const onSessionChange = mock(() => undefined)

    const mounted = await mountStripeCheckoutElements({
      clientSecret: 'cs_test_secret',
      publishableKey: 'pk_test_flatkey',
      paymentContainer,
      currencyContainer,
      onSessionChange,
      loadStripe: async () => stripe,
    })

    expect(initCheckoutElementsSdk).toHaveBeenCalledWith({
      clientSecret: 'cs_test_secret',
      elementsOptions: {
        appearance: expect.any(Object),
        loader: 'auto',
      },
    })
    expect(mountPayment).toHaveBeenCalledWith(paymentContainer)
    expect(mountCurrency).toHaveBeenCalledWith(currencyContainer)
    expect(onSessionChange).toHaveBeenCalledWith(session)

    const changedSession = { ...session, canConfirm: false }
    changeHandler?.(changedSession)
    expect(onSessionChange).toHaveBeenLastCalledWith(changedSession)

    await mounted.confirm()
    expect(confirm).toHaveBeenCalledWith({ redirect: 'always' })

    await mounted.applyPromotionCode('SAVE20')
    expect(applyPromotionCode).toHaveBeenCalledWith('SAVE20')
    await mounted.removePromotionCode()
    expect(removePromotionCode).toHaveBeenCalledTimes(1)

    mounted.destroy()
    expect(destroyPayment).toHaveBeenCalledTimes(1)
    expect(destroyCurrency).toHaveBeenCalledTimes(1)
  })

  test('fails closed when Stripe cannot load', async () => {
    expect(
      mountStripeCheckoutElements({
        clientSecret: 'cs_test_secret',
        publishableKey: 'pk_test_flatkey',
        paymentContainer: {} as HTMLElement,
        currencyContainer: null,
        onSessionChange: () => undefined,
        loadStripe: async () => null,
      })
    ).rejects.toThrow('stripe.js failed to load')
  })
})
