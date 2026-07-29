/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { describe, expect, test } from 'bun:test'
import { ROLE } from '@/lib/roles'
import { canAccessSupplyChain } from './index'

describe('supply-chain route access', () => {
  test('allows Root and denies Admin and User', () => {
    expect(canAccessSupplyChain(ROLE.SUPER_ADMIN)).toBe(true)
    expect(canAccessSupplyChain(ROLE.ADMIN)).toBe(false)
    expect(canAccessSupplyChain(ROLE.USER)).toBe(false)
    expect(canAccessSupplyChain(undefined)).toBe(false)
  })
})
