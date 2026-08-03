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
import { AxiosHeaders, type InternalAxiosRequestConfig } from 'axios'
import { afterEach, describe, expect, test } from 'bun:test'
import { api } from '../src/lib/api'
import {
  batchEditApiKeys,
  getApiKeys,
  searchApiKeys,
} from '../src/features/keys/api'

const originalAdapter = api.defaults.adapter

afterEach(() => {
  api.defaults.adapter = originalAdapter
})

describe('API key requests', () => {
  test('sends group filters for list and search requests', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    api.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
      requests.push(config)
      return {
        data: { success: true, data: { items: [], total: 0 } },
        status: 200,
        statusText: 'OK',
        headers: new AxiosHeaders(),
        config,
      }
    }

    await getApiKeys({ group: 'vip', p: 1, size: 20 })
    await searchApiKeys({ group: 'vip', p: 2, size: 10 })

    expect(requests.map((request) => request.url)).toEqual([
      '/api/token/?p=1&size=20&group=vip',
      '/api/token/search?group=vip&p=2&size=10',
    ])
  })

  test('uses the generic PUT endpoint for batch model limits', async () => {
    let request: InternalAxiosRequestConfig | undefined
    api.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
      request = config
      return {
        data: { success: true, data: 2 },
        status: 200,
        statusText: 'OK',
        headers: new AxiosHeaders(),
        config,
      }
    }

    await batchEditApiKeys({
      ids: [4, 8],
      model_limits_enabled: true,
      model_limits: 'gpt-4o,gpt-5',
    })

    expect(request?.method).toBe('put')
    expect(request?.url).toBe('/api/token/batch')
    expect(JSON.parse(String(request?.data))).toEqual({
      ids: [4, 8],
      model_limits_enabled: true,
      model_limits: 'gpt-4o,gpt-5',
    })
  })
})
