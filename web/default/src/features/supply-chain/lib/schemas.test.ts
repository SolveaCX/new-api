/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { describe, expect, test } from 'bun:test'
import {
  channelBindingFormSchema,
  contractFormSchema,
  inventoryAdjustmentFormSchema,
  rateVersionFormSchema,
  usdInputToMicroUsd,
} from './schemas'

describe('supply-chain management schemas', () => {
  test('converts business-facing USD to exact integer micro-USD', () => {
    expect(usdInputToMicroUsd('200000')).toBe(200_000_000_000)
    expect(usdInputToMicroUsd('-1.000001')).toBe(-1_000_001)
    expect(usdInputToMicroUsd('0')).toBeNull()
    expect(usdInputToMicroUsd('1.0000001')).toBeNull()
    expect(usdInputToMicroUsd('9007199254.740993')).toBeNull()
  })

  test('rejects invalid append-only and binding values before submission', () => {
    expect(
      inventoryAdjustmentFormSchema.safeParse({
        delta_usd: '0',
        type: 'replenishment',
        reason: '',
      }).success
    ).toBeFalse()
    expect(
      rateVersionFormSchema.safeParse({
        procurement_multiplier_ppm: 1_000_001,
        reason: '',
      }).success
    ).toBeFalse()
    expect(
      channelBindingFormSchema.safeParse({ contract_id: 0 }).success
    ).toBeFalse()
  })

  test('returns field-level errors for every invalid contract capacity limit', () => {
    const result = contractFormSchema.safeParse({
      supplier_id: 1,
      name: 'Primary',
      contract_no: 'C-1',
      remark: '',
      rpm_limit: -1,
      tpm_limit: 1.5,
      max_concurrency: Number.NaN,
    })

    expect(result.success).toBeFalse()
    if (result.success) return
    expect(result.error.issues.map((issue) => issue.path[0]).sort()).toEqual([
      'max_concurrency',
      'rpm_limit',
      'tpm_limit',
    ])
  })
})
