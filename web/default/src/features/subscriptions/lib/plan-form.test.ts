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
import type { TFunction } from 'i18next'
import { parseQuotaFromDollars, quotaUnitsToDollars } from '@/lib/format'
import type { SubscriptionPlan } from '../types'
import {
  formValuesToPlanPayload,
  getPlanFormSchema,
  planToFormValues,
  PLAN_FORM_DEFAULTS,
} from './plan-form'

const t = ((key: string) => key) as TFunction

describe('subscription plan local price form fields', () => {
  test('serializes short-window limits and preserves the monthly quota', () => {
    const payload = formValuesToPlanPayload({
      ...PLAN_FORM_DEFAULTS,
      title: 'Monthly media plan',
      total_amount: 12.5,
      media_credits_monthly: 300,
      window_5h_amount: 1.25,
      window_week_amount: 7.5,
    } as typeof PLAN_FORM_DEFAULTS & {
      media_credits_monthly: number
      window_5h_amount: number
      window_week_amount: number
    })

    expect(payload.plan.window_5h_amount).toBe(parseQuotaFromDollars(1.25))
    expect(payload.plan.window_week_amount).toBe(parseQuotaFromDollars(7.5))
    expect(payload.plan).not.toHaveProperty('media_credits_monthly')
    expect(payload.plan.total_amount).toBe(6250000)
  })

  test('round-trips window values through quota-unit payloads', () => {
    const values = {
      ...PLAN_FORM_DEFAULTS,
      title: 'Go',
      price_amount: 10,
      total_amount: 45,
      window_5h_amount: 8,
      window_week_amount: 12,
    }
    const payload = formValuesToPlanPayload(values).plan
    expect(payload.window_5h_amount).toBe(parseQuotaFromDollars(8))
    expect(payload.window_week_amount).toBe(parseQuotaFromDollars(12))

    const restored = planToFormValues({
      id: 2,
      title: 'Go',
      price_amount: 10,
      currency: 'USD',
      duration_unit: 'month',
      duration_value: 1,
      quota_reset_period: 'monthly',
      enabled: true,
      sort_order: 0,
      max_purchase_per_user: 0,
      total_amount: Number(payload.total_amount),
      window_5h_amount: Number(payload.window_5h_amount),
      window_week_amount: Number(payload.window_week_amount),
    } as SubscriptionPlan)
    expect(restored.window_5h_amount).toBe(
      quotaUnitsToDollars(Number(payload.window_5h_amount))
    )
    expect(restored.window_week_amount).toBe(
      quotaUnitsToDollars(Number(payload.window_week_amount))
    )
  })

  test('rejects negative window limits', () => {
    const result = getPlanFormSchema(t).safeParse({
      ...PLAN_FORM_DEFAULTS,
      title: 'Invalid',
      window_5h_amount: -1,
      window_week_amount: 0,
    })
    expect(result.success).toBe(false)
  })

  test('serializes blank Pix and UPI local prices as null', () => {
    const payload = formValuesToPlanPayload({
      ...PLAN_FORM_DEFAULTS,
      title: 'Local price plan',
      pix_price_brl: '',
      upi_price_inr: '',
    })

    expect(payload.plan.pix_price_brl).toBeNull()
    expect(payload.plan.upi_price_inr).toBeNull()
  })

  test('serializes entered Pix and UPI local prices as numbers', () => {
    const payload = formValuesToPlanPayload({
      ...PLAN_FORM_DEFAULTS,
      title: 'Local price plan',
      pix_price_brl: '49.90',
      upi_price_inr: '799.50',
    })

    expect(payload.plan.pix_price_brl).toBe(49.9)
    expect(payload.plan.upi_price_inr).toBe(799.5)
  })

  test('hydrates null local prices back to blank inputs', () => {
    const formValues = planToFormValues({
      id: 1,
      title: 'Local price plan',
      subtitle: '',
      price_amount: 9.99,
      currency: 'USD',
      duration_unit: 'month',
      duration_value: 1,
      enabled: true,
      sort_order: 0,
      max_purchase_per_user: 0,
      total_amount: 0,
      pix_price_brl: null,
      upi_price_inr: 799.5,
    } as SubscriptionPlan)

    expect(formValues.pix_price_brl).toBe('')
    expect(formValues.upi_price_inr).toBe('799.5')
  })

  test('requires entered local prices to be positive and bounded', () => {
    const schema = getPlanFormSchema(t)

    expect(
      schema.safeParse({
        ...PLAN_FORM_DEFAULTS,
        title: 'Local price plan',
        pix_price_brl: '0',
      }).success
    ).toBe(false)
    expect(
      schema.safeParse({
        ...PLAN_FORM_DEFAULTS,
        title: 'Local price plan',
        upi_price_inr: '10000',
      }).success
    ).toBe(false)
    expect(
      schema.safeParse({
        ...PLAN_FORM_DEFAULTS,
        title: 'Local price plan',
        pix_price_brl: '',
        upi_price_inr: '',
      }).success
    ).toBe(true)
  })
})
