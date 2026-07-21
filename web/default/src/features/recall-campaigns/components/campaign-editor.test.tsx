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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeAll, describe, expect, test } from 'bun:test'
import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import type { RecallAudienceTemplate, RecallCampaignDraft } from '../types'
import { CampaignEditor } from './campaign-editor'

const commonHelp =
  'Audience templates define the base audience. The rules shown below narrow it further, and built-in eligibility filters also apply. Preview the audience before activation.'
const firstPurchaseHelp =
  'Targets registered users in the PLG group who have never paid, for campaigns that encourage a first purchase.'
const automaticTranslationHelp =
  "Email content is translated automatically when saved, sent in each user's language, and falls back to English when unavailable."
const testI18n = createInstance()

function makeDraft(template: RecallAudienceTemplate): RecallCampaignDraft {
  return {
    name: 'Test campaign',
    audience_template: template,
    audience_config: {
      registration_age_days: 30,
      min_request_count: 1,
      max_quota: 0,
      min_paid_amount: 0,
      last_api_call_age_days: 30,
      last_payment_age_days: 30,
      subscription_expired_days: 30,
      min_subscription_amount: 0,
      min_subscription_count: 1,
      payment_providers: [],
      groups: [],
      group_mode: '',
      require_verified_email: true,
    },
    execution_mode: 'manual',
    schedule: {
      scheduled_at: 0,
      timezone: 'UTC',
      frequency: 'daily',
      weekday: 1,
      hour: 9,
      minute: 0,
    },
    coupon_source: 'automatic',
    existing_coupon_id: '',
    discount_config: {
      type: 'percent',
      percent_off: 20,
      amount_off: 0,
      currency: '',
      currency_options: {},
      minimum_amount: 0,
      minimum_amount_currency: '',
      coupon_redeem_by: 0,
    },
    product_scope: { topup_price_ids: [], subscription_price_ids: [] },
    promotion_valid_seconds: 604800,
    enrollment_limit: 1000,
    worker_concurrency: 5,
    email_sequence: [
      {
        stage_no: 1,
        delay_seconds: 0,
        template_version: 1,
        templates: {
          en: { subject: 'English subject', body_text: 'English body' },
          fr: { subject: 'Sujet français', body_text: 'Corps français' },
        },
      },
    ],
  }
}

function renderEditor(
  template: RecallAudienceTemplate,
  draft = makeDraft(template)
): string {
  return renderToStaticMarkup(
    <QueryClientProvider client={new QueryClient()}>
      <I18nextProvider i18n={testI18n}>
        <CampaignEditor initialDraft={draft} />
      </I18nextProvider>
    </QueryClientProvider>
  )
}

const audienceThresholdFields = [
  'registration_age_days',
  'min_request_count',
  'max_quota',
  'min_paid_amount',
  'last_api_call_age_days',
  'last_payment_age_days',
  'subscription_expired_days',
  'min_subscription_amount',
  'min_subscription_count',
] as const

function expectAudienceThresholds(
  html: string,
  shownFields: (typeof audienceThresholdFields)[number][]
) {
  for (const field of audienceThresholdFields) {
    const inputName = `name="audience_config.${field}"`
    if (shownFields.includes(field)) {
      expect(html).toContain(inputName)
    } else {
      expect(html).not.toContain(inputName)
    }
  }
}

beforeAll(async () => {
  await testI18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    resources: {
      en: {
        translation: {
          [commonHelp]: commonHelp,
          [firstPurchaseHelp]: firstPurchaseHelp,
          [automaticTranslationHelp]: automaticTranslationHelp,
        },
      },
    },
    interpolation: { escapeValue: false },
  })
})

describe('CampaignEditor audience rules', () => {
  test('keeps the no-filter group input visible, disabled, and labelled', () => {
    const html = renderEditor('first_purchase')

    expect(html).toContain('for="recall-groups"')
    expect(html).toMatch(
      /<input(?=[^>]*id="recall-groups")(?=[^>]*disabled="")[^>]*>/
    )
  })

  test('clears stale groups when loading a no-filter draft', () => {
    const draft = makeDraft('first_purchase')
    draft.audience_config.groups = ['stale-group']
    const html = renderEditor('first_purchase', draft)
    const groupInput = html.match(/<input[^>]*id="recall-groups"[^>]*>/)?.[0]

    expect(groupInput).toBeTruthy()
    expect(groupInput).toContain('disabled=""')
    expect(groupInput).toContain('value=""')
    expect(groupInput).not.toContain('stale-group')
  })

  test('uses configured product selectors instead of manual Stripe Price ID inputs', () => {
    const html = renderEditor('first_purchase')

    expect(html).toContain('Top-up products')
    expect(html).toContain('Subscription products')
    expect(html).not.toContain('Top-up Stripe Price IDs')
    expect(html).not.toContain('Subscription Stripe Price IDs')
  })

  test('explains the selected audience and associates the help with the selector', () => {
    const html = renderEditor('first_purchase')

    expect(html).toContain(commonHelp)
    expect(html).toContain(firstPurchaseHelp)
    expect(html).toContain('aria-describedby="recall-audience-help"')
    expect(html).toContain('id="recall-audience-help"')
    expect(html).toContain('aria-live="polite"')
  })

  test('shows every rule applied to first-purchase audiences', () => {
    const html = renderEditor('first_purchase')

    expectAudienceThresholds(html, [
      'registration_age_days',
      'min_request_count',
      'max_quota',
      'last_api_call_age_days',
    ])
    expect(html).not.toContain('Payment providers (comma separated)')
  })

  test('shows every rule applied to lapsed-payer audiences', () => {
    const html = renderEditor('lapsed_payer')

    expectAudienceThresholds(html, [
      'max_quota',
      'min_paid_amount',
      'last_api_call_age_days',
      'last_payment_age_days',
    ])
    expect(html).toContain('Payment providers (comma separated)')
  })

  test('shows every rule applied to expired-subscription audiences', () => {
    const html = renderEditor('expired_subscription')

    expectAudienceThresholds(html, [
      'last_api_call_age_days',
      'subscription_expired_days',
      'min_subscription_amount',
      'min_subscription_count',
    ])
    expect(html).toContain('Payment providers (comma separated)')
  })
})

describe('CampaignEditor email sequence', () => {
  test('renders only English template fields', () => {
    const draft = makeDraft('first_purchase')
    const html = renderEditor('first_purchase', draft)

    expect(html).not.toContain('Template language')
    expect(html).toContain('name="email_sequence.0.templates.en.subject"')
    expect(html).toContain('name="email_sequence.0.templates.en.body_text"')
    expect(html).not.toContain('templates.fr')
  })

  test('explains automatic localization without UTF-16 native limits', () => {
    const html = renderEditor('first_purchase')
    const subjectInput = html.match(
      /<input[^>]*name="email_sequence\.0\.templates\.en\.subject"[^>]*>/
    )?.[0]
    const bodyInput = html.match(
      /<textarea[^>]*name="email_sequence\.0\.templates\.en\.body_text"[^>]*>/
    )?.[0]

    expect(html.replaceAll('&#x27;', "'")).toContain(automaticTranslationHelp)
    expect(subjectInput).toBeTruthy()
    expect(subjectInput?.toLowerCase()).not.toContain('maxlength')
    expect(bodyInput).toBeTruthy()
    expect(bodyInput?.toLowerCase()).not.toContain('maxlength')
  })
})
