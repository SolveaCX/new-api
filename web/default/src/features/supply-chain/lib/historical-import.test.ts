/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { describe, expect, test } from 'bun:test'
import {
  historicalImportProgress,
  rollupHistoricalSeries,
} from './historical-import'

describe('historical estimate presentation', () => {
  test('reports bounded progress and preserves unknown coverage', () => {
    expect(historicalImportProgress(5, 10)).toBe(50)
    expect(historicalImportProgress(0, 0)).toBe(0)
    expect(historicalImportProgress(12, 10)).toBe(100)
    expect(
      rollupHistoricalSeries([
        {
          date: '2026-01-01',
          source_request_count: 2,
          unassigned_request_count: 1,
          official_list_known_count: 1,
          official_list_unknown_count: 1,
          official_list_micro_usd: '1000000',
          sales_known_count: 2,
          sales_unknown_count: 0,
          sales_micro_usd: '1400000',
          procurement_cost_known_count: 1,
          procurement_cost_unknown_count: 1,
          procurement_cost_micro_usd: '650000',
          gross_profit_known_count: 1,
          gross_profit_unknown_count: 1,
          gross_profit_micro_usd: '50000',
        },
      ])
    ).toEqual([
      {
        date: '2026-01-01',
        sourceCount: 2,
        unknownCount: 1,
        unassignedCount: 1,
        salesUnknownCount: 0,
        costUnknownCount: 1,
        grossUnknownCount: 1,
        salesMicroUsd: 1400000n,
        costMicroUsd: 650000n,
        grossMicroUsd: 50000n,
      },
    ])
  })
})
