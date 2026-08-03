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
import {
  buildBatchEditApiKeysPayload,
  coordinateBatchEditApiKeys,
  isBatchQuotaInputValid,
} from '../src/features/keys/lib/api-key-batch-group'

describe('batch API key edits', () => {
  test('builds and normalizes model allowlist payloads', () => {
    expect(
      buildBatchEditApiKeysPayload([3, 9], {
        model_limits_enabled: true,
        model_limits: 'gpt-4o,gpt-5',
      })
    ).toEqual({
      ids: [3, 9],
      model_limits_enabled: true,
      model_limits: 'gpt-4o,gpt-5',
    })
    expect(
      buildBatchEditApiKeysPayload([3, 9], {
        model_limits_enabled: false,
        model_limits: 'ignored',
      })
    ).toEqual({
      ids: [3, 9],
      model_limits_enabled: false,
      model_limits: '',
    })
  })

  test('rejects incomplete model allowlist edits', () => {
    expect(() =>
      buildBatchEditApiKeysPayload([1], { model_limits_enabled: true })
    ).toThrow()
    expect(() =>
      buildBatchEditApiKeysPayload([1], { model_limits: 'gpt-5' })
    ).toThrow()
  })

  test('validates quota input for token and currency display modes', () => {
    expect(isBatchQuotaInputValid('12', true)).toBe(true)
    expect(isBatchQuotaInputValid('1.5', true)).toBe(false)
    expect(isBatchQuotaInputValid('1.5', false)).toBe(true)
    expect(isBatchQuotaInputValid('', false)).toBe(false)
  })

  test('runs success effects only after a successful request', async () => {
    const calls: string[] = []
    const result = await coordinateBatchEditApiKeys({
      request: async () => ({ success: true, data: 2 }),
      payload: {
        ids: [3, 9],
        model_limits_enabled: true,
        model_limits: 'gpt-5',
      },
      successEffects: {
        resetSelection: () => calls.push('selection'),
        refresh: () => calls.push('refresh'),
        resetForm: () => calls.push('form'),
        closeDialog: () => calls.push('dialog'),
      },
    })

    expect(result).toEqual({ success: true, count: 2 })
    expect(calls).toEqual(['selection', 'refresh', 'form', 'dialog'])
  })
})
