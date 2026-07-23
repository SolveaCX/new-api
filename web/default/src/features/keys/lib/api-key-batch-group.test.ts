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
import type { UserModelAccess } from '@/features/available-models'
import {
  buildBatchGroupPayload,
  canBatchUpdateApiKeyGroup,
  coordinateBatchGroupUpdate,
  getBatchGroupOptions,
  type BatchGroupUpdateSuccessEffects,
} from './api-key-batch-group'

function buildModelAccess(
  overrides: Partial<UserModelAccess> = {}
): UserModelAccess {
  return {
    scope_mode: 'selectable_group',
    identity_scope: null,
    identity_model_ids: [],
    identity_model_ratios: {},
    identity_default_ratio: null,
    create_default_scope: 'default',
    groups: [
      {
        id: 'default',
        label: 'Default',
        description: 'Default group',
        ratio: 1,
        model_ids: [],
        model_ratios: {},
      },
    ],
    account_model_ids: [],
    account_model_ratios: {},
    account_default_ratio: null,
    models: [],
    ...overrides,
  }
}

function createSuccessEffects() {
  const calls = {
    resetSelection: 0,
    refresh: 0,
    clearGroup: 0,
    closeDialog: 0,
  }
  const successEffects: BatchGroupUpdateSuccessEffects = {
    resetSelection: () => {
      calls.resetSelection += 1
    },
    refresh: () => {
      calls.refresh += 1
    },
    clearGroup: () => {
      calls.clearGroup += 1
    },
    closeDialog: () => {
      calls.closeDialog += 1
    },
  }

  return { calls, successEffects }
}

describe('batch API key group updates', () => {
  test('builds the exact backend payload and preserves explicit IDs', () => {
    expect(buildBatchGroupPayload([3, 9], 'premium')).toEqual({
      ids: [3, 9],
      group: 'premium',
    })
  })

  test('rejects empty, oversized, and group-less payloads', () => {
    expect(() => buildBatchGroupPayload([], 'default')).toThrow()
    expect(() =>
      buildBatchGroupPayload(
        Array.from({ length: 101 }, (_, index) => index + 1),
        'default'
      )
    ).toThrow()
    expect(() => buildBatchGroupPayload([1], '')).toThrow()
  })

  test('only enables the action for usable selectable groups', () => {
    const selectable = buildModelAccess()
    expect(canBatchUpdateApiKeyGroup(true, true, selectable)).toBe(true)
    expect(
      canBatchUpdateApiKeyGroup(
        true,
        true,
        buildModelAccess({ scope_mode: 'fixed_account' })
      )
    ).toBe(false)
    expect(
      canBatchUpdateApiKeyGroup(true, true, buildModelAccess({ groups: [] }))
    ).toBe(false)
  })

  test('hides the bulk group action when PLG gating disallows groups', () => {
    expect(canBatchUpdateApiKeyGroup(true, false, buildModelAccess())).toBe(
      false
    )
  })

  test('hides the bulk group action until the backend capability is enabled', () => {
    expect(canBatchUpdateApiKeyGroup(false, true, buildModelAccess())).toBe(
      false
    )
  })

  test('maps model access groups to the shared combobox option shape', () => {
    expect(getBatchGroupOptions(buildModelAccess())).toEqual([
      {
        value: 'default',
        label: 'Default',
        desc: 'Default group',
        ratio: 1,
      },
    ])
  })

  test('runs all success effects once after the request succeeds', async () => {
    const requestedPayloads: Array<{ ids: number[]; group: string }> = []
    const effects = createSuccessEffects()

    const result = await coordinateBatchGroupUpdate({
      request: async (ids, group) => {
        requestedPayloads.push({ ids: [...ids], group })
        return { success: true, data: 2 }
      },
      ids: [3, 9],
      group: 'premium',
      successEffects: effects.successEffects,
    })

    expect(requestedPayloads).toEqual([{ ids: [3, 9], group: 'premium' }])
    expect(result).toEqual({ success: true, count: 2 })
    expect(effects.calls).toEqual({
      resetSelection: 1,
      refresh: 1,
      clearGroup: 1,
      closeDialog: 1,
    })
  })

  test('declared failure retains effects and retries the same payload', async () => {
    const requestedPayloads: Array<{ ids: number[]; group: string }> = []
    const effects = createSuccessEffects()
    const request = async (ids: number[], group: string) => {
      requestedPayloads.push({ ids: [...ids], group })
      return { success: false, message: 'Group update rejected' }
    }
    const params = {
      request,
      ids: [3, 9],
      group: 'premium',
      successEffects: effects.successEffects,
    }

    const firstResult = await coordinateBatchGroupUpdate(params)
    const retryResult = await coordinateBatchGroupUpdate(params)

    expect(requestedPayloads).toEqual([
      { ids: [3, 9], group: 'premium' },
      { ids: [3, 9], group: 'premium' },
    ])
    expect(firstResult).toEqual({
      success: false,
      message: 'Group update rejected',
    })
    expect(retryResult).toEqual(firstResult)
    expect(effects.calls).toEqual({
      resetSelection: 0,
      refresh: 0,
      clearGroup: 0,
      closeDialog: 0,
    })
  })

  test('thrown failure retains effects and permits a successful retry', async () => {
    const requestedPayloads: Array<{ ids: number[]; group: string }> = []
    const effects = createSuccessEffects()
    let attempt = 0
    const params = {
      request: async (ids: number[], group: string) => {
        requestedPayloads.push({ ids: [...ids], group })
        attempt += 1
        if (attempt === 1) throw new Error('Network unavailable')
        return { success: true, data: 2 }
      },
      ids: [3, 9],
      group: 'premium',
      successEffects: effects.successEffects,
    }

    await expect(coordinateBatchGroupUpdate(params)).rejects.toThrow(
      'Network unavailable'
    )
    expect(effects.calls).toEqual({
      resetSelection: 0,
      refresh: 0,
      clearGroup: 0,
      closeDialog: 0,
    })

    const retryResult = await coordinateBatchGroupUpdate(params)

    expect(requestedPayloads).toEqual([
      { ids: [3, 9], group: 'premium' },
      { ids: [3, 9], group: 'premium' },
    ])
    expect(retryResult).toEqual({ success: true, count: 2 })
    expect(effects.calls).toEqual({
      resetSelection: 1,
      refresh: 1,
      clearGroup: 1,
      closeDialog: 1,
    })
  })
})
