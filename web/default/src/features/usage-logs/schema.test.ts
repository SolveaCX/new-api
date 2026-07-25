/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import { describe, expect, test } from 'bun:test'
import { usageLogSchema } from './data/schema'

const baseLog = {
  id: 1,
  user_id: 2,
  created_at: 1_784_801_200,
  type: 2,
  content: 'consume',
}

describe('usage log supplier accounting projection', () => {
  test('preserves readable Root projection and exact micro-USD strings', () => {
    const log = usageLogSchema.parse({
      ...baseLog,
      supplier_accounting: {
        binding_version_id: 11,
        supplier_id: 12,
        contract_id: 13,
        rate_version_id: 14,
        procurement_multiplier_ppm: 650_000,
        sales_multiplier_ppm: 700_000,
        official_list_micro_usd: '9007199254740993',
        sales_micro_usd: '70000000',
        procurement_cost_micro_usd: '65000000',
        gross_profit_micro_usd: '5000000',
        statistics_scope: 'business',
        exclusion_decision: 'included',
        financially_committed_at: 1_784_801_200,
        pricing_evidence: {
          mode: 'ratio',
          model_ratio_ppm: 2_500_000,
          group_multiplier_ppm: 700_000,
          dimensions: ['tool'],
        },
      },
    })

    expect(log.supplier_accounting?.official_list_micro_usd).toBe(
      '9007199254740993'
    )
    expect(log.supplier_accounting?.pricing_evidence?.mode).toBe('ratio')
  })

  test('accepts Admin and user responses without the Root-only projection', () => {
    expect(usageLogSchema.parse(baseLog).supplier_accounting).toBeUndefined()
  })
})
