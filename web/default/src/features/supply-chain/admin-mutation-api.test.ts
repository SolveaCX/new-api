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
  getAccountingPolicyCapability,
  getAccountingRuntimeSettings,
  inactivateContract,
  inactivateSupplier,
  unbindChannel,
  updateAccountingPolicyCapability,
  updateAccountingRuntimeSettings,
  updateContract,
  updateSupplier,
} from './api'

const originalAdapter = api.defaults.adapter

afterEach(() => {
  api.defaults.adapter = originalAdapter
})

describe('supply-chain versioned mutation API', () => {
  test('reads and updates the explicit accounting policy capability', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    api.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
      requests.push(config)
      return {
        data: {
          success: true,
          data: {
            protocol_version: 1,
            activated: config.method === 'put',
            active: false,
            effective_at: 1_785_000_000,
          },
        },
        status: 200,
        statusText: 'OK',
        headers: new AxiosHeaders(),
        config,
      }
    }

    const current = await getAccountingPolicyCapability()
    const updated = await updateAccountingPolicyCapability({ activated: true })

    expect(current.data.activated).toBe(false)
    expect(updated.data.activated).toBe(true)
    expect(requests.map((request) => request.method)).toEqual(['get', 'put'])
    expect(requests[0]?.url).toEndWith('/channel-binding-policy-v1')
    expect(requests[1]?.url).toEndWith('/channel-binding-policy-v1')
    expect(JSON.parse(String(requests[1]?.data))).toEqual({ activated: true })
    expect(requests[1]?.skipErrorHandler).toBe(true)
  })

  test('reads and updates shared supplier accounting runtime settings', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    api.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
      requests.push(config)
      return {
        data: {
          success: true,
          data: {
            protocol_version: 1,
            revision: config.method === 'put' ? 2 : 1,
            cutover_at: 1_785_254_400,
            retention_days: config.method === 'put' ? 30 : 0,
            source: 'database',
            cutover_locked: false,
          },
        },
        status: 200,
        statusText: 'OK',
        headers: new AxiosHeaders(),
        config,
      }
    }

    await getAccountingRuntimeSettings()
    const updated = await updateAccountingRuntimeSettings({
      expected_revision: 1,
      cutover_at: 1_785_254_400,
      retention_days: 30,
    })

    expect(updated.data.revision).toBe(2)
    expect(requests.map((request) => request.method)).toEqual(['get', 'put'])
    expect(requests[0]?.url).toEndWith('/runtime-settings-v1')
    expect(requests[1]?.url).toEndWith('/runtime-settings-v1')
    expect(JSON.parse(String(requests[1]?.data))).toEqual({
      expected_revision: 1,
      cutover_at: 1_785_254_400,
      retention_days: 30,
    })
    expect(requests[1]?.skipErrorHandler).toBe(true)
  })

  test('does not claim idempotency for unsupported admin mutations', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    api.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
      requests.push(config)
      const isBindingPolicyRequest = config.url?.includes(
        '/channel-bindings/11/policy-v1'
      )
      return {
        data: {
          success: true,
          data: isBindingPolicyRequest
            ? { skip_internal_accounting: config.method !== 'delete' }
            : {},
        },
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
      skip_internal_accounting: true,
      expected_skip_internal_accounting: false,
    })
    await unbindChannel(11, {
      expectedContractId: 7,
      expectedSkipInternalAccounting: true,
    })

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
      skip_internal_accounting: true,
      expected_skip_internal_accounting: false,
    })
    expect(requests[8]?.params).toEqual({
      expected_contract_id: 7,
      expected_skip_internal_accounting: true,
    })
    expect(requests[7]?.url).toEndWith('/channel-bindings/11/policy-v1')
    expect(requests[8]?.url).toEndWith('/channel-bindings/11/policy-v1')
    expect(requests[8]?.skipErrorHandler).toBe(true)
  })

  test('rejects a mutation response from a console without policy v1 support', async () => {
    api.defaults.adapter = async (config: InternalAxiosRequestConfig) => ({
      data: { success: true, data: {} },
      status: 200,
      statusText: 'OK',
      headers: new AxiosHeaders(),
      config,
    })

    await expect(
      bindChannel(11, {
        contract_id: 7,
        expected_contract_id: 0,
        skip_internal_accounting: true,
        expected_skip_internal_accounting: false,
      })
    ).rejects.toThrow('supplier accounting policy v1')
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
