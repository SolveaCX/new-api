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
import { describe, expect, it } from 'vitest'
import {
  calculateModel,
  getCashInputs,
  getDiscountForModel,
} from './cost-calculation'
import type {
  CostCalculationAssumptions,
  CostCalculationDiscounts,
} from './types'

const assumptions: CostCalculationAssumptions = {
  listPrice: 10,
  bonus: 35,
  coupon: 5,
  seedanceSellFactor: 0.9,
  otherSellFactor: 0.8,
  inputTokens: 1_000_000,
  outputTokens: 1_000_000,
  nonTokenUsage: 1,
}

const discounts: CostCalculationDiscounts = {
  openai: 0.6,
  anthropic: 0.7,
  moonshot: 0.7,
  national: 0.8,
  bytedance: 0.95,
  other: null,
}

describe('cost calculation', () => {
  it('uses discounted cash against the original quota', () => {
    expect(getCashInputs(assumptions)).toEqual({
      actualCash: 5,
      totalQuota: 45,
      cashFactor: 5 / 45,
    })
  })

  it('calculates the GPT example from the workbook', () => {
    const result = calculateModel(
      {
        name: 'gpt-5.4-mini',
        vendor: 'OpenAI',
        billingUnit: 'USD/1M tokens',
        officialInput: 0.75,
        officialOutput: 4.5,
        officialNonToken: 0,
        costDiscount: 0.6,
        note: '',
      },
      assumptions,
      discounts
    )

    expect(result.officialFee).toBeCloseTo(5.25)
    expect(result.flatkeyPrice).toBeCloseTo(4.2)
    expect(result.cashIncome).toBeCloseTo(4.2 * (5 / 45))
    expect(result.upstreamCost).toBeCloseTo(3.15)
    expect(result.profit).toBeCloseTo(-2.6833333333)
    expect(result.status).toBe('loss')
  })

  it('does not fabricate a cost when the discount is missing', () => {
    const result = calculateModel(
      {
        name: 'grok-imagine-video-1.5',
        vendor: 'xAI',
        billingUnit: 'USD/秒',
        officialInput: 0,
        officialOutput: 0,
        officialNonToken: 0.11,
        costDiscount: null,
        note: '',
      },
      assumptions,
      discounts
    )

    expect(result.upstreamCost).toBeNull()
    expect(result.profit).toBeNull()
    expect(result.status).toBe('missing-discount')
  })

  it('uses a model override before the vendor discount', () => {
    expect(
      getDiscountForModel(
        {
          name: 'gpt-5.4-mini',
          vendor: 'OpenAI',
          billingUnit: 'USD/1M tokens',
          officialInput: 1,
          officialOutput: 1,
          officialNonToken: 0,
          costDiscount: 0.9,
          note: '',
          customFields: {},
        },
        discounts
      )
    ).toBe(0.9)
  })

  it('keeps dynamic fields independent for each model', () => {
    const config = {
      fields: ['Region'],
      models: [
        {
          name: 'model-a',
          vendor: 'Other',
          billingUnit: 'USD/request',
          officialInput: 0,
          officialOutput: 0,
          officialNonToken: 1,
          costDiscount: null,
          note: '',
          customFields: { Region: 'US' },
        },
      ],
    }

    expect(config.models[0].customFields[config.fields[0]]).toBe('US')
  })
})
