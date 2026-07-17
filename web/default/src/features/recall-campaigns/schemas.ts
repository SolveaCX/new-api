import { z } from 'zod'
import type { RecallCampaignDraft } from './types'

const nonNegativeInteger = z.number().int().min(0)
const nonNegativeNumber = z.number().min(0)
const currencySchema = z.string().regex(/^[A-Z]{3}$/)

function isIanaTimezone(value: string): boolean {
  if (value === '' || value === 'Local') return false
  try {
    new Intl.DateTimeFormat('en-US', { timeZone: value }).format()
    return true
  } catch {
    return false
  }
}

const audienceSchema = z
  .object({
    registration_age_days: nonNegativeInteger,
    min_request_count: nonNegativeInteger,
    max_quota: nonNegativeInteger,
    min_paid_amount: nonNegativeNumber,
    last_api_call_age_days: nonNegativeInteger,
    last_payment_age_days: nonNegativeInteger,
    subscription_expired_days: nonNegativeInteger,
    min_subscription_amount: nonNegativeNumber,
    min_subscription_count: nonNegativeInteger,
    payment_providers: z.array(z.string().trim().min(1)),
    groups: z.array(z.string().trim().min(1)),
    group_mode: z.enum(['', 'allow', 'block']),
    require_verified_email: z.boolean(),
  })
  .strict()
  .superRefine((audience, context) => {
    if (audience.groups.length === 0 && audience.group_mode !== '') {
      context.addIssue({
        code: 'custom',
        path: ['group_mode'],
        message: 'Groups are required',
      })
    }
    if (audience.groups.length > 0 && audience.group_mode === '') {
      context.addIssue({
        code: 'custom',
        path: ['group_mode'],
        message: 'Group mode is required',
      })
    }
  })

const scheduleSchema = z
  .object({
    scheduled_at: z.number().int(),
    timezone: z.string(),
    frequency: z.string(),
    weekday: z.number().int(),
    hour: z.number().int(),
    minute: z.number().int(),
  })
  .strict()

const discountSchema = z
  .object({
    type: z.enum(['percent', 'fixed']),
    percent_off: z.number().min(0),
    amount_off: nonNegativeInteger,
    currency: z.string(),
    minimum_amount: nonNegativeInteger,
    minimum_amount_currency: z.string(),
    coupon_redeem_by: nonNegativeInteger,
  })
  .strict()
  .superRefine((discount, context) => {
    if (discount.type === 'percent') {
      if (discount.percent_off <= 0 || discount.percent_off > 100) {
        context.addIssue({
          code: 'custom',
          path: ['percent_off'],
          message: 'Percent is invalid',
        })
      }
      if (discount.amount_off !== 0 || discount.currency !== '') {
        context.addIssue({
          code: 'custom',
          path: ['amount_off'],
          message: 'Amount must be zero',
        })
      }
    }
    if (discount.type === 'fixed') {
      if (discount.percent_off !== 0) {
        context.addIssue({
          code: 'custom',
          path: ['percent_off'],
          message: 'Percent must be zero',
        })
      }
      if (discount.amount_off <= 0) {
        context.addIssue({
          code: 'custom',
          path: ['amount_off'],
          message: 'Amount is required',
        })
      }
      if (!currencySchema.safeParse(discount.currency).success) {
        context.addIssue({
          code: 'custom',
          path: ['currency'],
          message: 'Currency is invalid',
        })
      }
    }
    if (discount.minimum_amount > 0) {
      if (!currencySchema.safeParse(discount.minimum_amount_currency).success) {
        context.addIssue({
          code: 'custom',
          path: ['minimum_amount_currency'],
          message: 'Minimum amount currency is invalid',
        })
      }
      if (
        discount.type === 'fixed' &&
        discount.minimum_amount_currency !== discount.currency
      ) {
        context.addIssue({
          code: 'custom',
          path: ['minimum_amount_currency'],
          message: 'Only one currency is supported',
        })
      }
    } else if (discount.minimum_amount_currency !== '') {
      context.addIssue({
        code: 'custom',
        path: ['minimum_amount_currency'],
        message: 'Minimum amount currency is not needed',
      })
    }
    if (
      discount.coupon_redeem_by > 0 &&
      discount.coupon_redeem_by <= Date.now() / 1000
    ) {
      context.addIssue({
        code: 'custom',
        path: ['coupon_redeem_by'],
        message: 'Coupon redeem-by must be in the future',
      })
    }
  })

const productScopeSchema = z
  .object({
    topup_price_ids: z.array(z.string().trim().min(1)),
    subscription_price_ids: z.array(z.string().trim().min(1)),
  })
  .strict()
  .refine(
    (scope) =>
      scope.topup_price_ids.length + scope.subscription_price_ids.length > 0,
    { message: 'At least one Stripe Price is required' }
  )

const emailTemplateSchema = z
  .object({
    subject: z.string().trim().min(1),
    body_text: z.string().trim().min(1),
  })
  .strict()

const emailStageSchema = z
  .object({
    stage_no: z.number().int().min(1).max(3),
    delay_seconds: nonNegativeInteger,
    template_version: z.number().int().min(1),
    templates: z.record(z.string().trim().min(1), emailTemplateSchema),
  })
  .strict()
  .refine((stage) => Boolean(stage.templates.en), {
    path: ['templates', 'en'],
    message: 'English template is required',
  })

const emailSequenceSchema = z
  .array(emailStageSchema)
  .min(1)
  .max(3)
  .superRefine((sequence, context) => {
    let previousDelay = -1
    sequence.forEach((stage, index) => {
      if (stage.stage_no !== index + 1) {
        context.addIssue({
          code: 'custom',
          path: [index, 'stage_no'],
          message: 'Email stages must be ordered from one',
        })
      }
      if (
        (index === 0 && stage.delay_seconds !== 0) ||
        stage.delay_seconds <= previousDelay
      ) {
        context.addIssue({
          code: 'custom',
          path: [index, 'delay_seconds'],
          message: 'Email delays must start at zero and increase',
        })
      }
      previousDelay = stage.delay_seconds
    })
  })

const activatedUpdateFieldsSchema = z
  .object({
    name: z.string().trim().min(1).max(128),
    email_sequence: emailSequenceSchema,
  })
  .passthrough()

export const recallCampaignActivatedUpdateSchema =
  activatedUpdateFieldsSchema as unknown as z.ZodType<
    RecallCampaignDraft,
    RecallCampaignDraft
  >

export const recallCampaignDraftSchema = z
  .object({
    name: z.string().trim().min(1).max(128),
    audience_template: z.enum([
      'first_purchase',
      'lapsed_payer',
      'expired_subscription',
    ]),
    audience_config: audienceSchema,
    execution_mode: z.enum(['manual', 'scheduled_once', 'recurring']),
    schedule: scheduleSchema,
    coupon_source: z.enum(['automatic', 'existing']),
    existing_coupon_id: z.string(),
    discount_config: discountSchema,
    product_scope: productScopeSchema,
    promotion_valid_seconds: z.number().int().positive(),
    enrollment_limit: z.number().int().min(1).max(100_000),
    worker_concurrency: z.number().int().min(1).max(20),
    email_sequence: emailSequenceSchema,
  })
  .strict()
  .superRefine((draft, context) => {
    if (
      draft.coupon_source === 'automatic' &&
      draft.existing_coupon_id.trim() !== ''
    ) {
      context.addIssue({
        code: 'custom',
        path: ['existing_coupon_id'],
        message: 'Automatic coupons cannot use an existing coupon ID',
      })
    }
    if (
      draft.coupon_source === 'existing' &&
      draft.existing_coupon_id.trim() === ''
    ) {
      context.addIssue({
        code: 'custom',
        path: ['existing_coupon_id'],
        message: 'Existing coupon ID is required',
      })
    }
    if (
      draft.execution_mode === 'scheduled_once' &&
      draft.schedule.scheduled_at <= Date.now() / 1000
    ) {
      context.addIssue({
        code: 'custom',
        path: ['schedule', 'scheduled_at'],
        message: 'Scheduled time must be in the future',
      })
    }
    if (draft.execution_mode === 'recurring') {
      if (!isIanaTimezone(draft.schedule.timezone)) {
        context.addIssue({
          code: 'custom',
          path: ['schedule', 'timezone'],
          message: 'IANA timezone is required',
        })
      }
      if (
        draft.schedule.frequency !== 'daily' &&
        draft.schedule.frequency !== 'weekly'
      ) {
        context.addIssue({
          code: 'custom',
          path: ['schedule', 'frequency'],
          message: 'Frequency is invalid',
        })
      }
      if (draft.schedule.hour < 0 || draft.schedule.hour > 23) {
        context.addIssue({
          code: 'custom',
          path: ['schedule', 'hour'],
          message: 'Hour is invalid',
        })
      }
      if (draft.schedule.minute < 0 || draft.schedule.minute > 59) {
        context.addIssue({
          code: 'custom',
          path: ['schedule', 'minute'],
          message: 'Minute is invalid',
        })
      }
      if (
        draft.schedule.frequency === 'weekly' &&
        (draft.schedule.weekday < 0 || draft.schedule.weekday > 6)
      ) {
        context.addIssue({
          code: 'custom',
          path: ['schedule', 'weekday'],
          message: 'Weekday is invalid',
        })
      }
    }
  })
