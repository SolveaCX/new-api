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
  test('sends expected versions and one caller-owned key for all six CAS mutations', async () => {
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
    const idempotencyKey = 'stable-versioned-mutation-key'

    await updateSupplier(3, {
      idempotencyKey,
      data: { name: 'Supplier', expected_version: 4 },
    })
    await inactivateSupplier(3, {
      idempotencyKey,
      data: { expected_version: 4 },
    })
    await updateContract(7, {
      idempotencyKey,
      data: { remark: 'renewed', expected_version: 8 },
    })
    await inactivateContract(7, {
      idempotencyKey,
      data: { expected_version: 8 },
    })
    await bindChannel(11, {
      idempotencyKey,
      data: { contract_id: 7, expected_contract_id: 0 },
    })
    await unbindChannel(11, { idempotencyKey, expectedContractId: 7 })

    expect(requests).toHaveLength(6)
    for (const request of requests) {
      expect(request.headers.get('Idempotency-Key')).toBe(idempotencyKey)
    }
    expect(JSON.parse(String(requests[0]?.data))).toEqual({
      name: 'Supplier',
      expected_version: 4,
    })
    expect(JSON.parse(String(requests[1]?.data))).toEqual({
      expected_version: 4,
    })
    expect(JSON.parse(String(requests[2]?.data))).toEqual({
      remark: 'renewed',
      expected_version: 8,
    })
    expect(JSON.parse(String(requests[3]?.data))).toEqual({
      expected_version: 8,
    })
    expect(JSON.parse(String(requests[4]?.data))).toEqual({
      contract_id: 7,
      expected_contract_id: 0,
    })
    expect(requests[5]?.params).toEqual({ expected_contract_id: 7 })
  })
})
