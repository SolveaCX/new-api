/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { describe, expect, test } from 'bun:test'
import { getOrCreateIntentKey } from './use-idempotent-intent'

describe('supported append idempotent intent lifecycle', () => {
  test('retains the same key for an explicit supported-operation retry', () => {
    const key = getOrCreateIntentKey(null, () => 'stable-408-key')
    const original = {
      idempotencyKey: key,
      values: {
        expected_version: 7,
        reason: 'append-only inventory adjustment',
      },
    }

    const retriedKey = getOrCreateIntentKey(key, () => 'replacement-key')
    const retried = { ...original, idempotencyKey: retriedKey }

    expect(retriedKey).toBe('stable-408-key')
    expect(retried).toEqual(original)
  })

  test('creates a new key only for a new logical operation', () => {
    expect(getOrCreateIntentKey(null, () => 'new-key')).toBe('new-key')
  })

  test('is not used by mutations without backend idempotency support', async () => {
    const featureRoot = new URL('../components/', import.meta.url)
    const supplier = await Bun.file(
      new URL('supplier-management.tsx', featureRoot)
    ).text()
    const contract = await Bun.file(
      new URL('contract-management.tsx', featureRoot)
    ).text()
    const binding = await Bun.file(
      new URL('channel-binding-management.tsx', featureRoot)
    ).text()
    const adminHooks = await Bun.file(
      new URL('./use-supply-chain-admin.ts', import.meta.url)
    ).text()

    expect(supplier).not.toContain('useIdempotentIntent')
    expect(binding).not.toContain('useIdempotentIntent')
    expect(contract.match(/useIdempotentIntent\(\)/g)).toHaveLength(1)
    expect(contract).toContain('createInventoryAdjustment')
    expect(adminHooks).toContain('retry: false')
    expect(adminHooks).toContain('onSuccess: async () =>')
    expect(adminHooks).not.toContain('onSettled: async () =>')
  })
})
