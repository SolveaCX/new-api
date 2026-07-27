import { describe, expect, test } from 'bun:test'
import type { SupplierChannelBinding } from '../types'
import {
  buildHistoricalMappings,
  parseHistoricalMappings,
} from './historical-mapping'

function binding(
  overrides: Partial<SupplierChannelBinding>
): SupplierChannelBinding {
  return {
    channel_id: 11,
    channel_name: 'Codex',
    channel_status: 1,
    supplier_contract_id: 3,
    contract_name: 'Flatkey contract',
    contract_no: 'FK-1',
    supplier_id: 2,
    supplier_name: 'Flatkey',
    current_rate_version_id: 5,
    current_procurement_multiplier_ppm: 600_000,
    skip_internal_accounting: false,
    ...overrides,
  }
}

describe('historical channel mapping presentation', () => {
  test('builds sorted mappings only from complete bound channel snapshots', () => {
    expect(
      buildHistoricalMappings([
        binding({ channel_id: 12, current_rate_version_id: null }),
        binding({ channel_id: 9 }),
        binding({ channel_id: 4, current_procurement_multiplier_ppm: 650_000 }),
      ])
    ).toEqual([
      {
        channel_id: 4,
        supplier_id: 2,
        contract_id: 3,
        rate_version_id: 5,
        procurement_multiplier_ppm: 650_000,
      },
      {
        channel_id: 9,
        supplier_id: 2,
        contract_id: 3,
        rate_version_id: 5,
        procurement_multiplier_ppm: 600_000,
      },
    ])
  })

  test('returns no presentation rows for invalid JSON', () => {
    expect(parseHistoricalMappings('{')).toEqual([])
    expect(parseHistoricalMappings('{}')).toEqual([])
    expect(parseHistoricalMappings('[{"channel_id": 1}]')).toEqual([])
  })
})
