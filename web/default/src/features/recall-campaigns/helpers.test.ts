import { describe, expect, test } from 'bun:test'
import {
  getRecallPageCount,
  getRecallRecipientRetry,
  normalizeRecallCouponSource,
  normalizeRecallDiscountType,
  removeRecallEmailStage,
} from './helpers'
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
      minimum_amount: 100,
      minimum_amount_currency: 'usd',
      coupon_redeem_by: 0,
    },
  } as RecallCampaignDraft
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
  messageStates: RecallRecipient['messages'][number]['state'][]
): RecallRecipient {
  return {
    state,
    messages: messageStates.map((messageState, index) => ({
      id: index + 1,
      state: messageState,
    })),
  } as RecallRecipient
}

describe('recall campaign editor normalization', () => {
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
      minimum_amount: 100,
      minimum_amount_currency: 'USD',
    })
  })

  test('clears percent-only fields and establishes one fixed currency', () => {
    const draft = makeDraft()
    draft.discount_config = {
      ...draft.discount_config,
      type: 'percent',
      percent_off: 15,
      amount_off: 0,
      currency: '',
      minimum_amount: 100,
      minimum_amount_currency: 'eur',
    }

    const normalized = normalizeRecallDiscountType(draft, 'fixed')

    expect(normalized.discount_config).toMatchObject({
      type: 'fixed',
      percent_off: 0,
      amount_off: 1,
      currency: 'EUR',
      minimum_amount_currency: 'EUR',
    })
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
  })
})
