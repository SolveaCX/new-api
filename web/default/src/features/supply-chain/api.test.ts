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
import {
  AxiosError,
  AxiosHeaders,
  type InternalAxiosRequestConfig,
} from 'axios'
import { afterEach, describe, expect, it } from 'bun:test'
import { api } from '@/lib/api'
import {
  buildSupplierReportQueryParams,
  createContract,
  createExclusionRule,
  createInventoryAdjustment,
  createRateVersion,
  createSupplier,
  getReportFreshness,
  isSupplyChainCommandCommitted,
  bindChannel,
  listReportContracts,
  listChannelBindings,
  listContracts,
  listEffectiveExclusions,
  listExclusionRules,
  listInventoryAdjustments,
  listRateVersions,
  listSuppliers,
  unbindChannel,
} from './api'

const originalAdapter = api.defaults.adapter

function captureRequests(): InternalAxiosRequestConfig[] {
  const requests: InternalAxiosRequestConfig[] = []
  api.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
    requests.push(config)
    return {
      data: {
        success: true,
        data: { page: 1, page_size: 0, total: 0, items: [] },
      },
      status: 200,
      statusText: 'OK',
      headers: new AxiosHeaders(),
      config,
    }
  }
  return requests
}

afterEach(() => {
  api.defaults.adapter = originalAdapter
})

describe('buildSupplierReportQueryParams', () => {
  it('serializes a natural month, entity filters, and report pagination', () => {
    expect(
      buildSupplierReportQueryParams({
        month: '2026-07',
        supplierIds: [3, 5],
        contractIds: [8],
        channelIds: [13, 21],
        limit: 50,
        offset: 100,
      })
    ).toEqual({
      month: '2026-07',
      supplier_ids: '3,5',
      contract_ids: '8',
      channel_ids: '13,21',
      limit: 50,
      offset: 100,
    })
  })

  it('serializes an inclusive date query with backend field names', () => {
    expect(
      buildSupplierReportQueryParams({
        startDate: '2026-07-01',
        endDate: '2026-07-31',
      })
    ).toEqual({ start_date: '2026-07-01', end_date: '2026-07-31' })
  })
})

describe('supply-chain pagination contracts', () => {
  it('keeps admin p/page_size separate from report limit/offset', async () => {
    const requests = captureRequests()
    await listSuppliers({ p: 2, page_size: 25, status: 'active' })
    await listReportContracts({ month: '2026-07', limit: 25, offset: 25 })

    expect(requests[0]?.params).toEqual({
      p: 2,
      page_size: 25,
      status: 'active',
    })
    expect(requests[1]?.params).toEqual({
      month: '2026-07',
      limit: 25,
      offset: 25,
    })
  })

  it('normalizes nullable empty admin pages from the Go API', async () => {
    api.defaults.adapter = async (config: InternalAxiosRequestConfig) => ({
      data: {
        success: true,
        data: { page: 1, page_size: 20, total: 0, items: null },
      },
      status: 200,
      statusText: 'OK',
      headers: new AxiosHeaders(),
      config,
    })

    const pages = await Promise.all([
      listSuppliers({ p: 1, page_size: 20 }),
      listContracts({ p: 1, page_size: 20 }),
      listRateVersions({ contract_id: 1, p: 1, page_size: 20 }),
      listInventoryAdjustments({ contract_id: 1, p: 1, page_size: 20 }),
      listExclusionRules({ p: 1, page_size: 20 }),
      listEffectiveExclusions({ p: 1, page_size: 20 }),
      listChannelBindings({ p: 1, page_size: 20 }),
    ])

    for (const page of pages) {
      expect(page.data.items).toEqual([])
    }
  })

  it('rejects missing or malformed admin page items instead of hiding API drift', async () => {
    for (const items of [undefined, {}, 'not-an-array']) {
      api.defaults.adapter = async (config: InternalAxiosRequestConfig) => ({
        data: {
          success: true,
          data: { page: 1, page_size: 20, total: 0, items },
        },
        status: 200,
        statusText: 'OK',
        headers: new AxiosHeaders(),
        config,
      })

      await expect(listSuppliers({ p: 1, page_size: 20 })).rejects.toThrow(
        'items must be an array or null'
      )
    }
  })
})

describe('supply-chain report freshness contract', () => {
  it('preserves the synchronous-only cutover metadata', async () => {
    api.defaults.adapter = async (config: InternalAxiosRequestConfig) => ({
      data: {
        success: true,
        data: {
          sync_only: true,
          coverage_start_at: 1_788_192_000,
          latest_batch_date: '2026-09-15',
          batch_status: 'completed',
          fresh_through: 1_789_488_000,
          freshness_lag_seconds: 7_200,
          error_message: '',
        },
      },
      status: 200,
      statusText: 'OK',
      headers: new AxiosHeaders(),
      config,
    })

    await expect(getReportFreshness()).resolves.toMatchObject({
      data: { sync_only: true, coverage_start_at: 1_788_192_000 },
    })
  })

  it('marks missing or malformed legacy cutover metadata as unavailable', async () => {
    api.defaults.adapter = async (config: InternalAxiosRequestConfig) => ({
      data: {
        success: true,
        data: {
          latest_batch_date: '2026-09-15',
          batch_status: 'completed',
          fresh_through: 1_789_488_000,
          freshness_lag_seconds: 7_200,
          error_message: '',
          sync_only: 'unexpected-mode',
          coverage_start_at: 'not-a-timestamp',
        },
      },
      status: 200,
      statusText: 'OK',
      headers: new AxiosHeaders(),
      config,
    })

    await expect(getReportFreshness()).resolves.toMatchObject({
      data: { sync_only: false, coverage_start_at: null },
    })
  })

  it('marks an omitted legacy mode as unavailable', async () => {
    api.defaults.adapter = async (config: InternalAxiosRequestConfig) => ({
      data: {
        success: true,
        data: {
          latest_batch_date: '2026-09-15',
          batch_status: 'completed',
          fresh_through: 1_789_488_000,
          freshness_lag_seconds: 7_200,
          error_message: '',
          coverage_start_at: 1_788_192_000,
        },
      },
      status: 200,
      statusText: 'OK',
      headers: new AxiosHeaders(),
      config,
    })

    await expect(getReportFreshness()).resolves.toMatchObject({
      data: { sync_only: false, coverage_start_at: 1_788_192_000 },
    })
  })
})

describe('exact idempotent command reconciliation', () => {
  it('uses the original key and exact scope to confirm a committed command', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    api.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
      requests.push(config)
      return {
        data: {
          success: true,
          data: {
            scope: 'supplier_rate.create',
            idempotency_key: 'stable-command-key',
            resource_type: 'supplier_rate_version',
            resource_id: 19,
            created_at: 1_789_488_000,
          },
        },
        status: 200,
        statusText: 'OK',
        headers: new AxiosHeaders(),
        config,
      }
    }

    await expect(
      isSupplyChainCommandCommitted(
        'supplier_rate.create',
        'stable-command-key'
      )
    ).resolves.toBe(true)
    expect(requests[0]?.url).toBe('/api/supply-chain/commands/result')
    expect(requests[0]?.params).toEqual({ scope: 'supplier_rate.create' })
    expect(requests[0]?.headers.get('Idempotency-Key')).toBe(
      'stable-command-key'
    )
  })

  it('does not accept matching current state when the exact command lookup is 404', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    api.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
      requests.push(config)
      if (config.url?.endsWith('/commands/result')) {
        throw new AxiosError('not found', '404', config, undefined, {
          data: { success: false },
          status: 404,
          statusText: 'Not Found',
          headers: new AxiosHeaders(),
          config,
        })
      }
      return {
        data: {
          success: true,
          data: {
            page: 1,
            page_size: 20,
            total: 1,
            items: [{ id: 1, current_procurement_multiplier_ppm: 650_000 }],
          },
        },
        status: 200,
        statusText: 'OK',
        headers: new AxiosHeaders(),
        config,
      }
    }

    await expect(
      isSupplyChainCommandCommitted(
        'supplier_rate.create',
        'missing-command-key'
      )
    ).resolves.toBe(false)
    expect(requests).toHaveLength(1)
    expect(requests[0]?.url).toBe('/api/supply-chain/commands/result')
  })
})

describe('idempotent mutation variables', () => {
  it('forwards the caller-owned stable key on every retryable command', async () => {
    const requests = captureRequests()
    const idempotencyKey = 'stable-key-from-mutation-variables'

    await createSupplier({
      idempotencyKey,
      data: { name: 'supplier', remark: '' },
    })
    await createContract({
      idempotencyKey,
      data: {
        supplier_id: 1,
        name: 'contract',
        contract_no: 'C-1',
        remark: '',
        rpm_limit: 0,
        tpm_limit: 0,
        max_concurrency: 0,
      },
    })
    await createRateVersion(1, {
      idempotencyKey,
      data: { procurement_multiplier_ppm: 650_000, reason: 'renewal' },
    })
    await createInventoryAdjustment(1, {
      idempotencyKey,
      data: {
        delta_micro_usd: 200_000_000_000,
        type: 'replenishment',
        reason: 'top up',
      },
    })
    await createExclusionRule({
      idempotencyKey,
      data: { user_id: 1, action: 'exclude', reason: 'internal' },
    })

    expect(requests).toHaveLength(5)
    for (const request of requests) {
      expect(request.headers.get('Idempotency-Key')).toBe(idempotencyKey)
    }
  })

  it('reuses the same key when the same mutation variables are retried', async () => {
    const requests = captureRequests()
    const variables = {
      idempotencyKey: 'one-logical-command',
      data: { name: 'supplier', remark: '' },
    }

    await createSupplier(variables)
    await createSupplier(variables)

    expect(requests[0]?.headers.get('Idempotency-Key')).toBe(
      'one-logical-command'
    )
    expect(requests[1]?.headers.get('Idempotency-Key')).toBe(
      'one-logical-command'
    )
  })
})

describe('channel binding compare-and-swap contract', () => {
  it('sends the observed contract for bind and unbind operations', async () => {
    const requests = captureRequests()

    await bindChannel(7, { contract_id: 12, expected_contract_id: 0 })
    await unbindChannel(7, 12)

    expect(requests[0]?.data).toBe(
      JSON.stringify({ contract_id: 12, expected_contract_id: 0 })
    )
    expect(requests[1]?.params).toEqual({ expected_contract_id: 12 })
  })
})
