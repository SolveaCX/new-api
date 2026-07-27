/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import type {
  SupplierChannelBinding,
  SupplierChannelBindingRequest,
} from '../types'

export type ChannelBatchBindingResult = {
  succeeded: SupplierChannelBinding[]
  failed: Array<{ binding: SupplierChannelBinding; error: unknown }>
}

export async function coordinateChannelBindings(options: {
  bindings: readonly SupplierChannelBinding[]
  contractId: number
  skipInternalAccounting: boolean
  request: (
    channelId: number,
    payload: SupplierChannelBindingRequest
  ) => Promise<unknown>
}): Promise<ChannelBatchBindingResult> {
  const result: ChannelBatchBindingResult = { succeeded: [], failed: [] }

  for (const binding of options.bindings) {
    try {
      await options.request(binding.channel_id, {
        contract_id: options.contractId,
        expected_contract_id: binding.supplier_contract_id ?? 0,
        skip_internal_accounting: options.skipInternalAccounting,
        expected_skip_internal_accounting:
          binding.skip_internal_accounting ?? false,
      })
      result.succeeded.push(binding)
    } catch (error) {
      result.failed.push({ binding, error })
    }
  }

  return result
}
