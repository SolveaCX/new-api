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
  createHistoricalImport,
  getHistoricalImport,
  listHistoricalImportSummaries,
  listHistoricalImports,
} from './api'

const originalAdapter = api.defaults.adapter

afterEach(() => {
  api.defaults.adapter = originalAdapter
})

describe('supplier historical estimate API', () => {
  test('uses root import endpoints and preserves caller idempotency', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    api.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
      requests.push(config)
      const isImportList =
        config.method === 'get' &&
        config.url === '/api/supply-chain/historical-imports'
      return {
        data: {
          success: true,
          data: isImportList
            ? { page: 1, page_size: 20, total: 0, items: [] }
            : config.url?.endsWith('/summaries')
              ? { items: [], limit: 200, has_more: false, next_cursor: null }
              : {},
        },
        status: 200,
        statusText: 'OK',
        headers: new AxiosHeaders(),
        config,
      }
    }
    await createHistoricalImport({
      idempotencyKey: 'historical-command',
      data: {
        start_date: '2026-01-01',
        end_date: '2026-02-01',
        quota_per_unit: '500000',
        excluded_user_ids: [1],
        channel_mappings: [],
        reason: 'legacy estimate',
      },
    })
    await listHistoricalImports({ p: 1, page_size: 20 })
    await getHistoricalImport(9)
    await listHistoricalImportSummaries(9)

    expect(requests.map((request) => request.url)).toEqual([
      '/api/supply-chain/historical-imports',
      '/api/supply-chain/historical-imports',
      '/api/supply-chain/historical-imports/9',
      '/api/supply-chain/historical-imports/9/summaries',
    ])
    expect(requests[0]?.headers.get('Idempotency-Key')).toBe(
      'historical-command'
    )
    expect(JSON.parse(String(requests[0]?.data)).quota_per_unit).toBe('500000')
  })
})
