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
import type { InternalAxiosRequestConfig } from 'axios'
import { afterEach, describe, expect, spyOn, test } from 'bun:test'
import { toast } from 'sonner'
import {
  protectRecallClaimRedirectForAuth,
  resolvePendingPostLoginRedirect,
} from '@/features/auth/lib/storage'
import { api, getUserModels } from './api'

const originalWindow = globalThis.window

function installWindowStorage() {
  const localValues = new Map<string, string>()
  const sessionValues = new Map<string, string>()
  Object.defineProperty(globalThis, 'window', {
    configurable: true,
    value: {
      localStorage: {
        getItem: (key: string) => localValues.get(key) ?? null,
        removeItem: (key: string) => localValues.delete(key),
        setItem: (key: string, value: string) => localValues.set(key, value),
      },
      sessionStorage: {
        getItem: (key: string) => sessionValues.get(key) ?? null,
        removeItem: (key: string) => sessionValues.delete(key),
        setItem: (key: string, value: string) => sessionValues.set(key, value),
      },
    },
  })
}

afterEach(() => {
  Object.defineProperty(globalThis, 'window', {
    configurable: true,
    value: originalWindow,
  })
})

describe('API 401 auth reset lifecycle', () => {
  test('uses one stable toast id for concurrent session expiry errors', async () => {
    installWindowStorage()
    const toastSpy = spyOn(toast, 'error').mockImplementation(
      () => 'session-expired'
    )
    const rejectAsUnauthorized = async (config: InternalAxiosRequestConfig) => {
      throw {
        config,
        response: { status: 401 },
      }
    }

    await Promise.all([
      api
        .get('/test/session-expired/one', {
          disableDuplicate: true,
          adapter: rejectAsUnauthorized,
        })
        .catch(() => undefined),
      api
        .get('/test/session-expired/two', {
          disableDuplicate: true,
          adapter: rejectAsUnauthorized,
        })
        .catch(() => undefined),
    ])

    expect(toastSpy).toHaveBeenCalledTimes(2)
    expect(
      toastSpy.mock.calls.every((call) => call[1]?.id === 'session-expired')
    ).toBe(true)
    toastSpy.mockRestore()
  })

  test('preserves a pending recall redirect for the OAuth logout preflight only', async () => {
    installWindowStorage()
    const protectedRedirect = protectRecallClaimRedirectForAuth(
      '/console/topup?recall_claim=signed-secret'
    )

    await api
      .get('/test/oauth-logout-preflight', {
        disableDuplicate: true,
        skipErrorHandler: true,
        preservePendingPostLoginRedirectOn401: true,
        adapter: async (config) => {
          throw {
            config,
            response: { status: 401 },
          }
        },
      })
      .catch(() => undefined)

    expect(
      resolvePendingPostLoginRedirect(
        protectedRedirect?.sanitizedTarget,
        protectedRedirect?.nonce
      )
    ).toBe('/console/topup?recall_claim=signed-secret')
  })
})

describe('user model visibility', () => {
  test('requests the pricing-visible model set for authenticated pickers', async () => {
    const getSpy = spyOn(api, 'get').mockResolvedValueOnce({
      data: { success: true, data: ['gpt-4o'] },
    } as never)

    await expect(getUserModels('plg')).resolves.toEqual({
      success: true,
      data: ['gpt-4o'],
    })
    expect(getSpy).toHaveBeenCalledWith('/api/user/models', {
      params: { group: 'plg', exclude_hidden: true },
    })
    getSpy.mockRestore()
  })
})
