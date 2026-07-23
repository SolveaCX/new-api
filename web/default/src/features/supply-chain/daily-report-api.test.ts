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
import { getDailyReports, rerunDailyReport } from './api'

const originalAdapter = api.defaults.adapter

afterEach(() => {
  api.defaults.adapter = originalAdapter
})

describe('daily supply-chain report API', () => {
  test('normalizes nullable day, warning, and fresh-gap collections', async () => {
    api.defaults.adapter = async (config: InternalAxiosRequestConfig) => ({
      data: {
        success: true,
        data: {
          range: { start_at: 1, end_at: 2, timezone: 'Asia/Shanghai' },
          persisted_log_universe:
            'successfully_persisted_consume_logs_for_final_successful_settlement',
          days: null,
        },
      },
      status: 200,
      statusText: 'OK',
      headers: new AxiosHeaders(),
      config,
    })

    await expect(getDailyReports({ month: '2026-07' })).resolves.toMatchObject({
      data: { days: [] },
    })
  })

  test('uses the report range and unwraps the rerun success envelope', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    api.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
      requests.push(config)
      return {
        data:
          config.method === 'get'
            ? {
                success: true,
                data: {
                  range: { start_at: 1, end_at: 2, timezone: 'Asia/Shanghai' },
                  persisted_log_universe:
                    'successfully_persisted_consume_logs_for_final_successful_settlement',
                  days: [],
                },
              }
            : {
                success: true,
                data: {
                  request_id: 'stable-rerun-key',
                  batch_date: '2026-07-22',
                  run_id: 42,
                  status: 'running',
                  fence_token: 8,
                  published_fence_token: 7,
                  locked_until: '2026-07-23T03:00:00+08:00',
                  error_category: 'none',
                  result: null,
                },
              },
        status: 200,
        statusText: 'OK',
        headers: new AxiosHeaders(),
        config,
      }
    }

    await getDailyReports({
      startDate: '2026-07-01',
      endDate: '2026-07-22',
    })
    const result = await rerunDailyReport({
      batchDate: '2026-07-22',
      idempotencyKey: 'stable-rerun-key',
      data: {
        reason: 'retry incomplete persisted evidence',
        expected_published_fence_token: 7,
      },
    })

    expect(requests[0]?.url).toBe('/api/supply-chain/reports/daily')
    expect(requests[0]?.params).toEqual({
      start_date: '2026-07-01',
      end_date: '2026-07-22',
    })
    expect(requests[1]?.url).toBe(
      '/api/supply-chain/reports/daily/2026-07-22/rerun'
    )
    expect(requests[1]?.headers.get('Idempotency-Key')).toBe('stable-rerun-key')
    expect(JSON.parse(String(requests[1]?.data))).toEqual({
      reason: 'retry incomplete persisted evidence',
      expected_published_fence_token: 7,
    })
    expect(result.request_id).toBe('stable-rerun-key')
  })
})
