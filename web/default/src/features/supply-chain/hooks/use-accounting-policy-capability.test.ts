/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { describe, expect, test } from 'bun:test'
import { isAccountingPolicyConfigurable } from './use-accounting-policy-capability'

const activeCapability = {
  protocol_version: 1,
  activated: true,
  active: true,
  effective_at: 1_785_000_000,
}

describe('accounting policy capability safety', () => {
  test('allows complete skip only with a healthy configurable capability', () => {
    expect(isAccountingPolicyConfigurable(activeCapability, false)).toBe(true)
    expect(isAccountingPolicyConfigurable(undefined, false)).toBe(false)
  })

  test('rejects stale active data after a background refresh failure', () => {
    expect(isAccountingPolicyConfigurable(activeCapability, true)).toBe(false)
  })
})
