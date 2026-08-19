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
import {
  loadStripe,
  type Stripe,
  type StripeCheckoutConfirmResult,
  type StripeCheckoutSession,
} from '@stripe/stripe-js'

const checkoutAppearance = {
  theme: 'stripe' as const,
  variables: {
    colorPrimary: '#0576d7',
    colorBackground: '#ffffff',
    colorText: '#20242a',
    colorDanger: '#dc2626',
    colorTextSecondary: '#646a73',
    borderRadius: '12px',
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
    spacingUnit: '4px',
  },
  rules: {
    '.Input': {
      border: '1px solid #cfd5dc',
      boxShadow: '0 2px 7px rgba(25, 31, 40, 0.10)',
      padding: '16px',
    },
    '.Input:focus': {
      borderColor: '#0576d7',
      boxShadow: '0 0 0 4px rgba(5, 118, 215, 0.12)',
    },
    '.Tab': {
      border: '1px solid #dfe3e8',
      boxShadow: 'none',
    },
    '.Tab--selected': {
      borderColor: '#0576d7',
      boxShadow: '0 0 0 1px #0576d7',
    },
    '.Label': {
      color: '#5c6066',
      fontWeight: '600',
    },
  },
}

interface MountStripeCheckoutElementsOptions {
  clientSecret: string
  publishableKey: string
  paymentContainer: HTMLElement
  currencyContainer: HTMLElement | null
  onSessionChange: (session: StripeCheckoutSession) => void
  loadStripe?: (publishableKey: string) => Promise<Stripe | null>
}

export interface MountedStripeCheckoutElements {
  confirm: () => Promise<StripeCheckoutConfirmResult>
  destroy: () => void
}

export async function mountStripeCheckoutElements(
  options: MountStripeCheckoutElementsOptions
): Promise<MountedStripeCheckoutElements> {
  const stripe = await (options.loadStripe ?? loadStripe)(
    options.publishableKey
  )
  if (!stripe) {
    throw new Error('stripe.js failed to load')
  }

  const checkout = stripe.initCheckoutElementsSdk({
    clientSecret: options.clientSecret,
    elementsOptions: {
      appearance: checkoutAppearance,
      loader: 'auto',
    },
  })
  const loadActionsResult = await checkout.loadActions()
  if (loadActionsResult.type === 'error') {
    throw new Error(loadActionsResult.error.message)
  }

  const paymentElement = checkout.createPaymentElement({
    layout: 'accordion',
  })
  const initialSession = loadActionsResult.actions.getSession()
  const currencyElement =
    options.currencyContainer &&
    (initialSession.currencyOptions?.length ?? 0) > 1
      ? checkout.createCurrencySelectorElement()
      : null

  checkout.on('change', options.onSessionChange)
  paymentElement.mount(options.paymentContainer)
  currencyElement?.mount(options.currencyContainer as HTMLElement)
  options.onSessionChange(initialSession)

  return {
    confirm: () =>
      loadActionsResult.actions.confirm({
        redirect: 'always',
      }),
    destroy: () => {
      currencyElement?.destroy()
      paymentElement.destroy()
    },
  }
}
