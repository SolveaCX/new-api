import { createFormControl } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { describe, expect, test } from 'bun:test'
import {
  formatRecallMinorAmount,
  getRecallPageCount,
  getRecallRecipientRetry,
  normalizeRecallGroupsForMode,
  normalizeRecallCouponSource,
  normalizeRecallDiscountType,
  parseRecallMajorAmount,
  prepareRecallCampaignSubmitDraft,
  removeRecallEmailStage,
  setRecallCampaignGroupMode,
} from './helpers'
import { recallCampaignDraftSchema } from './schemas'
import type {
  RecallCampaignDraft,
  RecallEmailStage,
  RecallRecipient,
} from './types'

function makeDraft(): RecallCampaignDraft {
  return {
    coupon_source: 'existing',
    existing_coupon_id: 'coupon_old',
    discount_config: {
      type: 'fixed',
      percent_off: 0,
      amount_off: 500,
      currency: 'usd',
      currency_options: { inr: 45_000, brl: 2_500, jpy: 750 },
      minimum_amount: 100,
      minimum_amount_currency: 'usd',
      coupon_redeem_by: 0,
    },
  } as RecallCampaignDraft
}

function makeValidDraft(): RecallCampaignDraft {
  return {
    name: 'Win back inactive customers',
    audience_template: 'first_purchase',
    audience_config: {
      registration_age_days: 14,
      min_request_count: 1,
      max_quota: 10,
      min_paid_amount: 0,
      last_api_call_age_days: 0,
      last_payment_age_days: 0,
      subscription_expired_days: 0,
      min_subscription_amount: 0,
      min_subscription_count: 0,
      payment_providers: [],
      groups: ['paid'],
      group_mode: '',
      require_verified_email: true,
    },
    execution_mode: 'manual',
    schedule: {
      scheduled_at: 0,
      timezone: '',
      frequency: '',
      weekday: 0,
      hour: 0,
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
    product_scope: {
      topup_price_ids: ['price_topup_20'],
      subscription_price_ids: [],
    },
    promotion_valid_seconds: 604_800,
    enrollment_limit: 1_000,
    worker_concurrency: 5,
    email_sequence: [
      {
        stage_no: 1,
        delay_seconds: 0,
        template_version: 1,
        templates: {
          en: { subject: 'We miss you', body_text: 'Come back soon.' },
        },
      },
    ],
  }
}

function makeStage(stageNo: number, delaySeconds: number): RecallEmailStage {
  return {
    stage_no: stageNo,
    delay_seconds: delaySeconds,
    template_version: 1,
    templates: { en: { subject: `Stage ${stageNo}`, body_text: 'Body' } },
  }
}

function makeRecipient(
  state: RecallRecipient['state'],
  messageStates: RecallRecipient['messages'][number]['state'][],
  leaseExpiresAt: number[] = []
): RecallRecipient {
  return {
    state,
    messages: messageStates.map((messageState, index) => ({
      id: index + 1,
      state: messageState,
      lease_expires_at: leaseExpiresAt[index] ?? 0,
    })),
  } as RecallRecipient
}

describe('recall campaign editor normalization', () => {
  test('canonicalizes no-filter groups at the editor submission boundary without dropping translations', () => {
    const draft = makeDraft()
    draft.audience_config = {
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
      groups: ['stale-group'],
      group_mode: '',
      require_verified_email: true,
    }
    draft.email_sequence = [
      {
        stage_no: 1,
        delay_seconds: 0,
        template_version: 1,
        templates: {
          en: { subject: 'English subject', body_text: 'English body' },
          fr: { subject: 'Sujet français', body_text: 'Corps français' },
        },
      },
    ]

    const normalized = prepareRecallCampaignSubmitDraft(draft)

    expect(normalized).not.toBe(draft)
    expect(draft.audience_config.groups).toEqual(['stale-group'])
    expect(normalized.audience_config.groups).toEqual([])
    expect(normalized.email_sequence[0].templates).toEqual(
      draft.email_sequence[0].templates
    )
    expect(normalized.email_sequence[0].templates.fr).toEqual({
      subject: 'Sujet français',
      body_text: 'Corps français',
    })
  })

  test('clears groups when no group filter is selected', () => {
    expect(normalizeRecallGroupsForMode(['paid', 'trial'], '')).toEqual([])
  })

  test.each(['allow', 'block'] as const)(
    'preserves normalized groups in %s mode',
    (mode) => {
      const groups = ['paid', 'trial']

      expect(normalizeRecallGroupsForMode(groups, mode)).toEqual(groups)
    }
  )

  test.each([
    ['', []],
    ['allow', ['paid']],
  ] as const)(
    'revalidates the audience after switching group mode to %s',
    async (mode, expectedGroups) => {
      const form = createFormControl<RecallCampaignDraft>({
        resolver: zodResolver(recallCampaignDraftSchema),
        defaultValues: makeValidDraft(),
      })
      const subscription = form.subscribe({
        formState: { values: true },
        callback: () => undefined,
      })
      form.register('audience_config.group_mode')
      form.register('audience_config.groups')
      await form.trigger('audience_config')
      expect(
        form.getFieldState('audience_config.group_mode').error?.message
      ).toBe('Group mode is required')

      await setRecallCampaignGroupMode(form, mode)

      expect(form.getValues('audience_config.groups')).toEqual(expectedGroups)
      expect(
        form.getFieldState('audience_config.group_mode').error
      ).toBeUndefined()
      subscription()
    }
  )

  test('clears the hidden existing coupon ID when switching to automatic', () => {
    const normalized = normalizeRecallCouponSource(makeDraft(), 'automatic')

    expect(normalized.coupon_source).toBe('automatic')
    expect(normalized.existing_coupon_id).toBe('')
  })

  test('clears fixed-only fields when switching to percent', () => {
    const normalized = normalizeRecallDiscountType(makeDraft(), 'percent')

    expect(normalized.discount_config).toMatchObject({
      type: 'percent',
      percent_off: 20,
      amount_off: 0,
      currency: '',
      currency_options: {},
      minimum_amount: 100,
      minimum_amount_currency: 'USD',
    })
  })

  test('establishes the four automatic fixed discount defaults', () => {
    const draft = makeDraft()
    draft.coupon_source = 'automatic'
    draft.discount_config = {
      ...draft.discount_config,
      type: 'percent',
      percent_off: 15,
      amount_off: 0,
      currency: '',
      currency_options: {},
      minimum_amount: 100,
      minimum_amount_currency: 'eur',
    }

    const normalized = normalizeRecallDiscountType(draft, 'fixed')

    expect(normalized.discount_config).toMatchObject({
      type: 'fixed',
      percent_off: 0,
      amount_off: 500,
      currency: 'USD',
      currency_options: { inr: 45_000, brl: 2_500, jpy: 750 },
      minimum_amount: 0,
      minimum_amount_currency: '',
    })
  })

  test('converts human-readable fixed amounts to Stripe minor units', () => {
    expect(parseRecallMajorAmount('USD', '5.00')).toBe(500)
    expect(parseRecallMajorAmount('INR', '450.00')).toBe(45_000)
    expect(parseRecallMajorAmount('BRL', '25.00')).toBe(2_500)
    expect(parseRecallMajorAmount('JPY', '750')).toBe(750)
    expect(parseRecallMajorAmount('USD', '5.001')).toBeNull()
    expect(parseRecallMajorAmount('JPY', '750.5')).toBeNull()
    expect(parseRecallMajorAmount('BRL', '0')).toBeNull()
  })

  test('formats Stripe minor units as human-readable fixed amounts', () => {
    expect(formatRecallMinorAmount('USD', 500)).toBe('5.00')
    expect(formatRecallMinorAmount('INR', 45_000)).toBe('450.00')
    expect(formatRecallMinorAmount('BRL', 2_500)).toBe('25.00')
    expect(formatRecallMinorAmount('JPY', 750)).toBe('750')
  })

  test('renumbers stages after removing a middle stage', () => {
    const stages = [
      makeStage(1, 0),
      makeStage(2, 86_400),
      makeStage(3, 172_800),
    ]

    expect(removeRecallEmailStage(stages, 1)).toEqual([
      makeStage(1, 0),
      { ...makeStage(3, 172_800), stage_no: 2 },
    ])
  })
})

describe('recall campaign detail guards', () => {
  test('exposes a second detail page beyond the first 100 rows', () => {
    expect(getRecallPageCount(101, 100)).toBe(2)
  })

  test('matches backend retry eligibility and uncertain acknowledgment', () => {
    expect(getRecallRecipientRetry(makeRecipient('failed', []))).toEqual({
      allowed: true,
      acknowledgeUncertain: false,
    })
    expect(
      getRecallRecipientRetry(makeRecipient('contacting', ['failed']))
    ).toEqual({ allowed: true, acknowledgeUncertain: false })
    expect(
      getRecallRecipientRetry(makeRecipient('contacting', ['uncertain']))
    ).toEqual({ allowed: true, acknowledgeUncertain: true })
    expect(
      getRecallRecipientRetry(
        makeRecipient('contacting', ['uncertain', 'failed'])
      )
    ).toEqual({ allowed: true, acknowledgeUncertain: false })
    expect(
      getRecallRecipientRetry(makeRecipient('contacting', ['accepted']))
    ).toEqual({ allowed: false, acknowledgeUncertain: false })
    expect(
      getRecallRecipientRetry(
        makeRecipient('contacting', ['sending'], [998]),
        999
      )
    ).toEqual({ allowed: true, acknowledgeUncertain: true })
    expect(
      getRecallRecipientRetry(
        makeRecipient('contacting', ['sending'], [999]),
        999
      )
    ).toEqual({ allowed: false, acknowledgeUncertain: false })
    expect(
      getRecallRecipientRetry(
        makeRecipient('contacting', ['sending'], [1_000]),
        999
      )
    ).toEqual({ allowed: false, acknowledgeUncertain: false })
    expect(
      getRecallRecipientRetry(
        makeRecipient('contacting', ['sending'], [0]),
        999
      )
    ).toEqual({ allowed: false, acknowledgeUncertain: false })
  })
})
