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
import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider } from 'react-i18next'
import { StripeCheckoutLayout } from './stripe-checkout-layout'

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Stripe secure checkout': 'Stripe secure checkout',
        'Confirm Payment': 'Confirm Payment',
        Email: 'Email',
        'Payment method': 'Payment method',
        Subtotal: 'Subtotal',
        Discount: 'Discount',
        Tax: 'Tax',
        Surcharge: 'Surcharge',
        'Total due': 'Total due',
        Continue: 'Continue',
        'Encrypted payment powered by Stripe':
          'Encrypted payment powered by Stripe',
        'Powered by Stripe': 'Powered by Stripe',
        Terms: 'Terms',
        Privacy: 'Privacy',
        'Close payment': 'Close payment',
      },
    },
  },
})

describe('StripeCheckoutLayout', () => {
  test('renders the approved two-pane data contract and disables an unconfirmable payment', () => {
    const html = renderToStaticMarkup(
      <I18nextProvider i18n={i18n}>
        <StripeCheckoutLayout
          title='Confirm Payment'
          description='Payment is processed securely by Stripe.'
          viewModel={{
            canConfirm: false,
            currency: 'BRL',
            email: 'buyer@example.com',
            productDescription: '1 month subscription',
            productName: 'Flatkey Go',
            primaryAmount: 'R$48.65',
            summaryLines: [
              { key: 'subtotal', amount: 'R$49.90' },
              { key: 'discount', amount: 'R$5.00' },
              { key: 'tax', amount: 'R$2.00' },
              { key: 'surcharge', amount: 'R$1.75' },
            ],
            topupSummary: null,
            totalAmount: 'R$48.65',
          }}
          onPaymentContainer={() => undefined}
          onCurrencyContainer={() => undefined}
          showCurrencySelector
          mounting={false}
          submitting={false}
          error={null}
          onConfirm={() => undefined}
        />
      </I18nextProvider>
    )

    expect(html).toContain('data-slot="stripe-checkout-form-pane"')
    expect(html).toContain('data-slot="stripe-checkout-summary-pane"')
    expect(html).toContain('value="buyer@example.com"')
    expect(html).toContain('readOnly=""')
    expect(html).toContain('Flatkey Go')
    expect(html).toContain('R$48.65')
    expect(html).toContain('Surcharge')
    expect(html).toContain('bg-[#0576d7]')
    expect(html).toMatch(/<button[^>]*disabled=""[^>]*>.*Continue.*<\/button>/)
  })
})
