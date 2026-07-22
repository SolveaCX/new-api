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
import { describe, expect, it } from 'bun:test'
import { supplyChainQueryKeys } from './query-keys'

describe('supplyChainQueryKeys', () => {
  it('provides invalidation prefixes for admin resources', () => {
    expect(supplyChainQueryKeys.suppliers.all()).toEqual([
      'supply-chain',
      'suppliers',
    ])
    expect(supplyChainQueryKeys.contracts.all()).toEqual([
      'supply-chain',
      'contracts',
    ])
    expect(supplyChainQueryKeys.channelBindings.all()).toEqual([
      'supply-chain',
      'channel-bindings',
    ])
  })

  it('separates report resources and includes complete query input', () => {
    const query = { month: '2026-07' as const, limit: 50, offset: 0 }
    expect(supplyChainQueryKeys.reports.contracts(query)).toEqual([
      'supply-chain',
      'reports',
      'contracts',
      query,
    ])
    expect(supplyChainQueryKeys.reports.freshness()).toEqual([
      'supply-chain',
      'reports',
      'freshness',
    ])
  })
})
