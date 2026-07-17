import type {
  RecallCampaignDraft,
  RecallCouponSource,
  RecallDiscountType,
  RecallEmailStage,
  RecallRecipient,
} from './types'

export function normalizeRecallCouponSource(
  draft: RecallCampaignDraft,
  couponSource: RecallCouponSource
): RecallCampaignDraft {
  return {
    ...draft,
    coupon_source: couponSource,
    existing_coupon_id:
      couponSource === 'automatic' ? '' : draft.existing_coupon_id,
  }
}

export function normalizeRecallDiscountType(
  draft: RecallCampaignDraft,
  discountType: RecallDiscountType
): RecallCampaignDraft {
  const discount = draft.discount_config
  if (discountType === 'percent') {
    return {
      ...draft,
      discount_config: {
        ...discount,
        type: 'percent',
        percent_off:
          discount.percent_off > 0 && discount.percent_off <= 100
            ? discount.percent_off
            : 20,
        amount_off: 0,
        currency: '',
        minimum_amount_currency:
          discount.minimum_amount > 0
            ? discount.minimum_amount_currency.trim().toUpperCase()
            : '',
      },
    }
  }

  const currency =
    discount.currency.trim().toUpperCase() ||
    discount.minimum_amount_currency.trim().toUpperCase() ||
    'USD'
  return {
    ...draft,
    discount_config: {
      ...discount,
      type: 'fixed',
      percent_off: 0,
      amount_off: discount.amount_off > 0 ? discount.amount_off : 1,
      currency,
      minimum_amount_currency: discount.minimum_amount > 0 ? currency : '',
    },
  }
}

export function removeRecallEmailStage(
  stages: RecallEmailStage[],
  removeIndex: number
): RecallEmailStage[] {
  let previousDelay = -1
  return stages
    .filter((_stage, index) => index !== removeIndex)
    .map((stage, index) => {
      const delaySeconds =
        index === 0 ? 0 : Math.max(stage.delay_seconds, previousDelay + 1)
      previousDelay = delaySeconds
      return { ...stage, stage_no: index + 1, delay_seconds: delaySeconds }
    })
}

export function getRecallPageCount(total: number, pageSize: number): number {
  if (pageSize < 1) return 1
  return Math.max(1, Math.ceil(total / pageSize))
}

export function getRecallRecipientRetry(recipient: RecallRecipient): {
  allowed: boolean
  acknowledgeUncertain: boolean
} {
  if (
    recipient.state === 'failed' ||
    recipient.messages.some((message) => message.state === 'failed')
  ) {
    return { allowed: true, acknowledgeUncertain: false }
  }
  if (recipient.messages.some((message) => message.state === 'uncertain')) {
    return { allowed: true, acknowledgeUncertain: true }
  }
  return { allowed: false, acknowledgeUncertain: false }
}
