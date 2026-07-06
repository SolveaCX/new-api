import { beforeAll, describe, expect, test } from 'bun:test'
import i18n from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { initReactI18next } from 'react-i18next'
import { StripeCancelFallbackNotice } from './index'

describe('StripeCancelFallbackNotice', () => {
  beforeAll(async () => {
    await i18n.use(initReactI18next).init({
      lng: 'en',
      fallbackLng: 'en',
      resources: {
        en: {
          translation: {},
        },
      },
      interpolation: {
        escapeValue: false,
      },
    })
  })

  test('renders founder fallback support copy after a canceled Stripe checkout', () => {
    const html = renderToStaticMarkup(<StripeCancelFallbackNotice />)

    expect(html).toContain('Payment didn&#x27;t work?')
    expect(html).toContain(
      'Email founder@flatkey.ai and we&#x27;ll sort you out.'
    )
  })
})
