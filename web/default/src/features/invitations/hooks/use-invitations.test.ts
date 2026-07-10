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
import {
  keepPreviousData,
  MutationObserver,
  QueryClient,
} from '@tanstack/react-query'
import '@/i18n/config'
import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  mock,
  spyOn,
} from 'bun:test'
import { toast } from 'sonner'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'
import { api } from '@/lib/api'
import type { AffiliateTransferResponse } from '../types'
import {
  createInvitationQueryOptions,
  createInvitationTransferMutationOptions,
} from './use-invitations'

const originalAdapter = api.defaults.adapter

const refreshedUser: AuthUser = {
  id: 42,
  username: 'invite-owner',
  role: 1,
  quota: 250,
}

function createInvitationMutation(): {
  mutateAsync: (quota: number) => Promise<AffiliateTransferResponse>
  queryClient: QueryClient
} {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  const observer = new MutationObserver(
    queryClient,
    createInvitationTransferMutationOptions(queryClient)
  )

  return {
    mutateAsync: (quota: number) => observer.mutate(quota),
    queryClient,
  }
}

function useApiAdapter(
  transferResponse: AffiliateTransferResponse | Error
): void {
  api.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
    if (config.url === '/api/user/aff_transfer') {
      if (transferResponse instanceof Error) {
        throw new AxiosError(
          transferResponse.message,
          AxiosError.ERR_NETWORK,
          config
        )
      }
      return {
        data: transferResponse,
        status: 200,
        statusText: 'OK',
        headers: new AxiosHeaders(),
        config,
      }
    }

    if (config.url === '/api/user/self') {
      return {
        data: { success: true, data: refreshedUser },
        status: 200,
        statusText: 'OK',
        headers: new AxiosHeaders(),
        config,
      }
    }

    throw new Error(`Unexpected request: ${config.url}`)
  }
}

beforeEach(() => {
  useAuthStore.getState().auth.setUser(null)
})

afterEach(() => {
  mock.restore()
  api.defaults.adapter = originalAdapter
  useAuthStore.getState().auth.setUser(null)
})

describe('useInvitations transfer mutation', () => {
  it('refreshes user data, invalidates invitation queries, and resolves on explicit success', async () => {
    useApiAdapter({ success: true })
    const successToast = spyOn(toast, 'success')
    const errorToast = spyOn(toast, 'error')
    const rendered = createInvitationMutation()
    const invalidate = spyOn(rendered.queryClient, 'invalidateQueries')

    const response = await rendered.mutateAsync(100)

    expect(response).toEqual({ success: true })
    expect(useAuthStore.getState().auth.user).toEqual(refreshedUser)
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ['invitations'] })
    expect(successToast).toHaveBeenCalledWith('Transfer successful')
    expect(errorToast).not.toHaveBeenCalled()
  })

  it('rejects explicit business failure without success side effects', async () => {
    useApiAdapter({ success: false, message: 'Quota unavailable' })
    const successToast = spyOn(toast, 'success')
    const errorToast = spyOn(toast, 'error')
    const rendered = createInvitationMutation()
    const invalidate = spyOn(rendered.queryClient, 'invalidateQueries')

    await expect(rendered.mutateAsync(100)).rejects.toThrow('Quota unavailable')
    expect(useAuthStore.getState().auth.user).toBeNull()
    expect(invalidate).not.toHaveBeenCalled()
    expect(successToast).not.toHaveBeenCalled()
    expect(errorToast).toHaveBeenCalledTimes(1)
    expect(errorToast).toHaveBeenCalledWith('Quota unavailable')
  })

  it('rejects a response without explicit success and skips success side effects', async () => {
    useApiAdapter({ message: 'Malformed transfer response' })
    const successToast = spyOn(toast, 'success')
    const errorToast = spyOn(toast, 'error')
    const rendered = createInvitationMutation()
    const invalidate = spyOn(rendered.queryClient, 'invalidateQueries')

    await expect(rendered.mutateAsync(100)).rejects.toThrow(
      'Malformed transfer response'
    )
    expect(useAuthStore.getState().auth.user).toBeNull()
    expect(invalidate).not.toHaveBeenCalled()
    expect(successToast).not.toHaveBeenCalled()
    expect(errorToast).toHaveBeenCalledTimes(1)
    expect(errorToast).toHaveBeenCalledWith('Malformed transfer response')
  })

  it('shows one translated fallback toast for a transport error', async () => {
    useApiAdapter(new Error('socket closed'))
    const successToast = spyOn(toast, 'success')
    const errorToast = spyOn(toast, 'error')
    const rendered = createInvitationMutation()
    const invalidate = spyOn(rendered.queryClient, 'invalidateQueries')

    await expect(rendered.mutateAsync(100)).rejects.toThrow('socket closed')
    expect(useAuthStore.getState().auth.user).toBeNull()
    expect(invalidate).not.toHaveBeenCalled()
    expect(successToast).not.toHaveBeenCalled()
    expect(errorToast).toHaveBeenCalledTimes(1)
    expect(errorToast).toHaveBeenCalledWith('Transfer failed')
  })
})

describe('invitation query options', () => {
  it('retains the previous page while the next page loads', () => {
    const options = createInvitationQueryOptions(2)

    expect(options.queryKey).toEqual(['invitations', 2])
    expect(options.placeholderData).toBe(keepPreviousData)
  })
})
