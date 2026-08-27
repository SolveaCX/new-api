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
import { useCallback, useEffect, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { fetchTokenKey } from '@/features/keys/api'
import { ERROR_MESSAGES } from '@/features/keys/constants'

const TOKEN_KEY_STALE_TIME = 60 * 1000
const TOKEN_KEY_GC_TIME = 5 * 60 * 1000

export interface ResolvedApiKeysResult {
  resolvedKeys: Record<number, string>
  loadingKeys: Record<number, boolean>
  resolveKey: (id: number) => Promise<string | null>
}

/** Fetch one unmasked key and add the public prefix used by the console. */
async function fetchResolvedKey(id: number): Promise<string> {
  const result = await fetchTokenKey(id)
  if (!result.success || !result.data?.key) {
    throw new Error(result.message || 'Failed to resolve the API key')
  }
  return `sk-${result.data.key}`
}

/**
 * The key list endpoint only ever returns masked keys, so the real value is
 * resolved to back copy actions and ready-to-run samples. The selected key is
 * resolved when the integration dialog opens; other keys are resolved only
 * when their own copy button is pressed. This keeps unrelated secrets out of
 * the page until the user explicitly asks for one. Resolved values accumulate
 * so keys the user already copied stay available after switching selection.
 */
export function useResolvedApiKeys(
  selectedKeyId: number | null,
  enabled: boolean
): ResolvedApiKeysResult {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const userId = useAuthStore((state) => state.auth.user?.id)
  const [resolvedState, setResolvedState] = useState<{
    userId: number | undefined
    keys: Record<number, string>
  }>({ userId, keys: {} })
  const [loadingState, setLoadingState] = useState<{
    userId: number | undefined
    keys: Record<number, boolean>
  }>({ userId, keys: {} })
  const pendingRequests = useRef<Record<string, Promise<string | null>>>({})

  // A different account must never see key material resolved for the
  // previous one, even for a single frame. Resetting during render (instead
  // of in an effect) keeps the stale map out of the committed tree entirely:
  // React re-renders with the empty map before anything reaches the DOM.
  if (resolvedState.userId !== userId) {
    setResolvedState({ userId, keys: {} })
  }
  if (loadingState.userId !== userId) {
    setLoadingState({ userId, keys: {} })
  }

  const keyId =
    typeof selectedKeyId === 'number' && Number.isFinite(selectedKeyId)
      ? selectedKeyId
      : null

  const resolveKey = useCallback(
    async (requestedId: number): Promise<string | null> => {
      if (!enabled || userId === undefined) return null
      if (!Number.isFinite(requestedId) || requestedId <= 0) return null

      const cached =
        resolvedState.userId === userId
          ? resolvedState.keys[requestedId]
          : undefined
      if (cached) return cached

      // Include the account id in the pending key. A late response from the
      // previous account must not suppress a request for the same token id
      // after an account switch.
      const pendingKey = `${userId}:${requestedId}`
      const pending = pendingRequests.current[pendingKey]
      if (pending) return pending

      setLoadingState((prev) => {
        if (prev.userId !== userId) return prev
        return {
          ...prev,
          keys: { ...prev.keys, [requestedId]: true },
        }
      })

      const request = queryClient
        .fetchQuery({
          queryKey: ['dashboard', 'overview', 'token-key', userId, requestedId],
          queryFn: () => fetchResolvedKey(requestedId),
          staleTime: TOKEN_KEY_STALE_TIME,
          gcTime: TOKEN_KEY_GC_TIME,
        })
        .then((resolvedValue) => {
          // The auth store is the source of truth here rather than the
          // closure: it lets an in-flight response become a harmless no-op
          // when the user signs out or switches accounts.
          if (useAuthStore.getState().auth.user?.id !== userId) return null

          setResolvedState((prev) => {
            if (prev.userId !== userId) return prev
            if (prev.keys[requestedId] === resolvedValue) return prev
            return {
              ...prev,
              keys: { ...prev.keys, [requestedId]: resolvedValue },
            }
          })
          return resolvedValue
        })
        .catch(() => {
          toast.error(t(ERROR_MESSAGES.UNEXPECTED))
          return null
        })
      const trackedRequest = request.finally(() => {
        if (pendingRequests.current[pendingKey] === request) {
          delete pendingRequests.current[pendingKey]
        }
        setLoadingState((prev) => {
          if (prev.userId !== userId) return prev
          const next = { ...prev.keys }
          delete next[requestedId]
          return { ...prev, keys: next }
        })
      })

      pendingRequests.current[pendingKey] = request
      return trackedRequest
    },
    [enabled, queryClient, resolvedState, t, userId]
  )

  const resolvedKeys = resolvedState.userId === userId ? resolvedState.keys : {}
  const loadingKeys =
    loadingState.userId === userId ? { ...loadingState.keys } : {}

  // Resolve the selected key as soon as the dialog opens, preserving the
  // ready-to-run snippet behavior that existed before per-row copy controls.
  useEffect(() => {
    if (!enabled || keyId === null) return
    void resolveKey(keyId)
  }, [enabled, keyId, resolveKey])

  return { resolvedKeys, loadingKeys, resolveKey }
}
