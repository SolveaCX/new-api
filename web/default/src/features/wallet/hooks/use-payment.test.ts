import { describe, expect, test } from 'bun:test'
import { getStripeRedirectUrls } from './use-payment'

describe('getStripeRedirectUrls', () => {
  test('marks Stripe cancel returns so the wallet can show fallback help', () => {
    const originalWindow = globalThis.window
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: {
        location: {
          origin: 'https://console.flatkey.ai',
        },
      },
    })

    try {
      const urls = getStripeRedirectUrls()

      expect(urls.success_url).toBe(
        'https://console.flatkey.ai/wallet?show_history=true'
      )
      expect(urls.cancel_url).toBe(
        'https://console.flatkey.ai/wallet?payment_cancelled=stripe'
      )
    } finally {
      if (originalWindow === undefined) {
        delete (globalThis as { window?: Window }).window
      } else {
        Object.defineProperty(globalThis, 'window', {
          configurable: true,
          value: originalWindow,
        })
      }
    }
  })
})
