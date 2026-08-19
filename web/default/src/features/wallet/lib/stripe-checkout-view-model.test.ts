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
import type { StripeCheckoutSession } from '@stripe/stripe-js'
import { describe, expect, test } from 'bun:test'
import { buildStripeCheckoutViewModel } from './stripe-checkout-view-model'

const checkoutSession: StripeCheckoutSession = {
  billingAddress: null,
  businessName: null,
  canConfirm: true,
  currency: 'brl',
  currencyOptions: null,
  discountAmounts: null,
  email: 'buyer@example.com',
  id: 'cs_test_elements',
  lastPaymentError: null,
  lineItems: [
    {
      id: 'li_flatkey_go',
      name: 'Flatkey Go',
      description: '1 month subscription',
      quantity: 1,
      unitLabel: null,
      discount: { minorUnitsAmount: 500, amount: 'R$5.00' },
      subtotal: { minorUnitsAmount: 4990, amount: 'R$49.90' },
      total: { minorUnitsAmount: 4865, amount: 'R$48.65' },
      taxExclusive: { minorUnitsAmount: 200, amount: 'R$2.00' },
      taxInclusive: { minorUnitsAmount: 0, amount: 'R$0.00' },
      unitAmount: { minorUnitsAmount: 4990, amount: 'R$49.90' },
      unitAmountDecimal: null,
      discountAmounts: null,
      taxAmounts: null,
      recurring: {
        interval: 'month',
        intervalCount: 1,
        usageType: 'licensed',
      },
      adjustableQuantity: null,
      images: [],
    },
  ],
  livemode: false,
  minorUnitsAmountDivisor: 100,
  phoneNumber: null,
  recurring: null,
  savedPaymentMethods: null,
  shipping: null,
  shippingAddress: null,
  shippingOptions: [],
  status: { type: 'open' },
  surcharge: { status: 'complete' },
  tax: { status: 'ready' },
  taxAmounts: null,
  taxIdInfo: null,
  total: {
    appliedBalance: { minorUnitsAmount: 0, amount: 'R$0.00' },
    balanceAppliedToNextInvoice: false,
    discount: { minorUnitsAmount: 500, amount: 'R$5.00' },
    shippingRate: { minorUnitsAmount: 0, amount: 'R$0.00' },
    subtotal: { minorUnitsAmount: 4990, amount: 'R$49.90' },
    surcharge: { minorUnitsAmount: 175, amount: 'R$1.75' },
    taxExclusive: { minorUnitsAmount: 200, amount: 'R$2.00' },
    taxInclusive: { minorUnitsAmount: 0, amount: 'R$0.00' },
    total: { minorUnitsAmount: 4865, amount: 'R$48.65' },
  },
}

describe('buildStripeCheckoutViewModel', () => {
  test('projects Stripe-formatted session values without recomputing payment totals', () => {
    expect(buildStripeCheckoutViewModel(checkoutSession, null)).toEqual({
      canConfirm: true,
      currency: 'BRL',
      email: 'buyer@example.com',
      productDescription: '1 month subscription',
      productName: 'Flatkey Go',
      primaryAmount: 'R$49.90',
      summaryLines: [
        { key: 'subtotal', amount: 'R$49.90' },
        { key: 'discount', amount: 'R$5.00' },
        { key: 'tax', amount: 'R$2.00' },
        { key: 'surcharge', amount: 'R$1.75' },
      ],
      topupSummary: null,
      totalAmount: 'R$48.65',
    })
  })

  test('hides zero-value optional lines and untrusted top-up bonus amounts', () => {
    const zeroOptionalSession: StripeCheckoutSession = {
      ...checkoutSession,
      total: {
        ...checkoutSession.total,
        discount: { minorUnitsAmount: 0, amount: 'R$0.00' },
        surcharge: { minorUnitsAmount: 0, amount: 'R$0.00' },
        taxExclusive: { minorUnitsAmount: 0, amount: 'R$0.00' },
      },
    }

    const viewModel = buildStripeCheckoutViewModel(zeroOptionalSession, {
      pay_amount: 20,
      bonus_amount: 7,
      credit_amount: 27,
      show_amounts: false,
    })

    expect(viewModel.summaryLines).toEqual([
      { key: 'subtotal', amount: 'R$49.90' },
    ])
    expect(viewModel.topupSummary).toBeNull()
  })

  test('keeps a server-authorized top-up bonus for the dialog summary', () => {
    const summary = {
      pay_amount: 20,
      bonus_amount: 7,
      credit_amount: 27,
      show_amounts: true,
    }

    expect(
      buildStripeCheckoutViewModel(checkoutSession, summary).topupSummary
    ).toEqual(summary)
  })
})
