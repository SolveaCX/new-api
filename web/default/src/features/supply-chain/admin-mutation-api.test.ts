/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { AxiosHeaders, type InternalAxiosRequestConfig } from 'axios'
import { afterEach, describe, expect, test } from 'bun:test'
import { api } from '@/lib/api'
import {
  bindChannel,
  createContract,
  createExclusionRule,
  createInventoryAdjustment,
  createRateVersion,
  createSupplier,
  inactivateContract,
  inactivateSupplier,
  unbindChannel,
  updateContract,
  updateSupplier,
} from './api'

const originalAdapter = api.defaults.adapter

afterEach(() => {
  api.defaults.adapter = originalAdapter
})

describe('supply-chain versioned mutation API', () => {
  test('does not claim idempotency for unsupported admin mutations', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    api.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
      requests.push(config)
      return {
        data: { success: true, data: {} },
        status: 200,
        statusText: 'OK',
        headers: new AxiosHeaders(),
        config,
      }
    }
    await createSupplier({ name: 'Supplier', remark: '' })
    await updateSupplier(3, {
      name: 'Supplier',
      expected_version: 4,
    })
    await inactivateSupplier(3, { expected_version: 4 })
    await createContract({
      supplier_id: 3,
      name: 'Contract',
      contract_no: 'C-1',
      remark: '',
      rpm_limit: 0,
      tpm_limit: 0,
      max_concurrency: 0,
    })
    await updateContract(7, {
      remark: 'renewed',
      expected_version: 8,
    })
    await inactivateContract(7, { expected_version: 8 })
    await createRateVersion(7, {
      procurement_multiplier_ppm: 800_000,
      reason: 'renewed',
    })
    await bindChannel(11, {
      contract_id: 7,
      expected_contract_id: 0,
    })
    await unbindChannel(11, { expectedContractId: 7 })

    expect(requests).toHaveLength(9)
    for (const request of requests) {
      expect(request.headers.get('Idempotency-Key')).toBeUndefined()
    }
    expect(JSON.parse(String(requests[1]?.data))).toEqual({
      name: 'Supplier',
      expected_version: 4,
    })
    expect(JSON.parse(String(requests[2]?.data))).toEqual({
      expected_version: 4,
    })
    expect(JSON.parse(String(requests[4]?.data))).toEqual({
      remark: 'renewed',
      expected_version: 8,
    })
    expect(JSON.parse(String(requests[5]?.data))).toEqual({
      expected_version: 8,
    })
    expect(JSON.parse(String(requests[7]?.data))).toEqual({
      contract_id: 7,
      expected_contract_id: 0,
    })
    expect(requests[8]?.params).toEqual({ expected_contract_id: 7 })
  })

  test('sends stable caller-owned keys only to supported append endpoints', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    api.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
      requests.push(config)
      return {
        data: {
          success: true,
          data: config.url?.endsWith('/inventory-adjustments')
            ? { delta_micro_usd: '9007199254740993' }
            : {},
        },
        status: 200,
        statusText: 'OK',
        headers: new AxiosHeaders(),
        config,
      }
    }
    const idempotencyKey = 'stable-supported-mutation-key'

    const inventory = await createInventoryAdjustment(7, {
      idempotencyKey,
      data: {
        delta_micro_usd: 1_000_000,
        type: 'replenishment',
        reason: 'funding',
      },
    })
    await createExclusionRule({
      idempotencyKey,
      data: { user_id: 9, action: 'exclude', reason: 'internal account' },
    })

    expect(requests).toHaveLength(2)
    for (const request of requests) {
      expect(request.headers.get('Idempotency-Key')).toBe(idempotencyKey)
    }
    expect(inventory.data.delta_micro_usd).toBe('9007199254740993')
    expect(JSON.parse(String(requests[0]?.data)).delta_micro_usd).toBe(
      1_000_000
    )
  })
})
