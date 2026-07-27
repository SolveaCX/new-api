/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { describe, expect, test } from 'bun:test'
import type {
  SupplierChannelBinding,
  SupplierChannelBindingRequest,
} from '../types'
import { coordinateChannelBindings } from './channel-batch-binding'

function binding(
  channelId: number,
  contractId: number | null,
  skipInternalAccounting: boolean
): SupplierChannelBinding {
  return {
    channel_id: channelId,
    channel_name: `Channel ${channelId}`,
    channel_status: 1,
    supplier_contract_id: contractId,
    contract_name: contractId === null ? null : `Contract ${contractId}`,
    contract_no: contractId === null ? null : `C-${contractId}`,
    supplier_id: contractId === null ? null : contractId,
    supplier_name: contractId === null ? null : `Supplier ${contractId}`,
    current_rate_version_id: contractId === null ? null : contractId,
    current_procurement_multiplier_ppm: contractId === null ? null : 500_000,
    skip_internal_accounting: skipInternalAccounting,
  }
}

describe('coordinateChannelBindings', () => {
  test('uses each channel snapshot and reports partial failures for retry', async () => {
    const requests: Array<{
      channelId: number
      payload: SupplierChannelBindingRequest
    }> = []
    const conflict = new Error('binding changed')

    const result = await coordinateChannelBindings({
      bindings: [
        binding(11, null, false),
        binding(12, 7, true),
        binding(13, 8, false),
      ],
      contractId: 21,
      skipInternalAccounting: true,
      request: async (channelId, payload) => {
        requests.push({ channelId, payload })
        if (channelId === 12) throw conflict
      },
    })

    expect(requests).toEqual([
      {
        channelId: 11,
        payload: {
          contract_id: 21,
          expected_contract_id: 0,
          skip_internal_accounting: true,
          expected_skip_internal_accounting: false,
        },
      },
      {
        channelId: 12,
        payload: {
          contract_id: 21,
          expected_contract_id: 7,
          skip_internal_accounting: true,
          expected_skip_internal_accounting: true,
        },
      },
      {
        channelId: 13,
        payload: {
          contract_id: 21,
          expected_contract_id: 8,
          skip_internal_accounting: true,
          expected_skip_internal_accounting: false,
        },
      },
    ])
    expect(result.succeeded.map((item) => item.channel_id)).toEqual([11, 13])
    expect(result.failed).toEqual([
      { binding: expect.objectContaining({ channel_id: 12 }), error: conflict },
    ])
  })
})
