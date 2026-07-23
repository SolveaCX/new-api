/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useRef, useState } from 'react'
import axios from 'axios'

export type IntentRunResult =
  | 'success'
  | 'reconciled'
  | 'conflict'
  | 'rate_limited'
  | 'pending_confirmation'
  | 'failed'
  | 'blocked'

interface RunIntentOptions {
  execute: (idempotencyKey: string) => Promise<unknown>
  reconcile: (idempotencyKey: string) => Promise<boolean>
}

export type IntentErrorDisposition =
  | 'verification_cancelled'
  | 'conflict'
  | 'rate_limited'
  | 'unknown'
  | 'failed'

function createIntentKey(): string {
  return globalThis.crypto.randomUUID()
}

export function getOrCreateIntentKey(
  current: string | null,
  createKey: () => string = createIntentKey
): string {
  return current ?? createKey()
}

export function classifyIntentError(error: unknown): IntentErrorDisposition {
  if (
    error instanceof Error &&
    error.name === 'SupplyChainVerificationCancelledError'
  ) {
    return 'verification_cancelled'
  }
  if (!axios.isAxiosError(error)) return 'failed'
  const status = error.response?.status
  if (status === 409) return 'conflict'
  if (status === 429) return 'rate_limited'
  if (status === undefined || status === 408 || status >= 500) return 'unknown'
  return 'failed'
}

export async function reconcileIntentResult(
  reconcile: () => Promise<boolean>
): Promise<IntentRunResult> {
  try {
    return (await reconcile()) ? 'reconciled' : 'failed'
  } catch {
    return 'pending_confirmation'
  }
}

export function useIdempotentIntent() {
  const keyRef = useRef<string | null>(null)
  const reconcileRef = useRef<(() => Promise<boolean>) | null>(null)
  const runningRef = useRef(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [isPendingConfirmation, setIsPendingConfirmation] = useState(false)

  function clearIntent(): void {
    keyRef.current = null
    reconcileRef.current = null
    setIsPendingConfirmation(false)
  }

  async function run(options: RunIntentOptions): Promise<IntentRunResult> {
    if (runningRef.current || isPendingConfirmation) return 'blocked'
    const key = getOrCreateIntentKey(keyRef.current)
    keyRef.current = key
    reconcileRef.current = () => options.reconcile(key)
    runningRef.current = true
    setIsSubmitting(true)
    try {
      await options.execute(key)
      clearIntent()
      return 'success'
    } catch (error) {
      const disposition = classifyIntentError(error)
      if (disposition === 'verification_cancelled') {
        clearIntent()
        return 'blocked'
      }
      if (disposition === 'conflict') {
        try {
          await options.reconcile(key)
        } catch {
          // The conflict is known even if refreshing the latest state fails.
        }
        clearIntent()
        return 'conflict'
      }
      if (disposition === 'rate_limited') {
        return 'rate_limited'
      }
      if (disposition === 'unknown') {
        setIsPendingConfirmation(true)
        try {
          if (await options.reconcile(key)) {
            clearIntent()
            return 'reconciled'
          }
        } catch {
          return 'pending_confirmation'
        }
        return 'pending_confirmation'
      }
      clearIntent()
      return 'failed'
    } finally {
      runningRef.current = false
      setIsSubmitting(false)
    }
  }

  async function reconcilePending(): Promise<IntentRunResult> {
    const reconcile = reconcileRef.current
    if (!reconcile || runningRef.current) return 'blocked'
    runningRef.current = true
    setIsSubmitting(true)
    try {
      const result = await reconcileIntentResult(reconcile)
      if (result === 'reconciled') clearIntent()
      else if (result === 'failed') setIsPendingConfirmation(false)
      else setIsPendingConfirmation(true)
      return result
    } finally {
      runningRef.current = false
      setIsSubmitting(false)
    }
  }

  return {
    run,
    clearIntent,
    reconcilePending,
    isSubmitting,
    isPendingConfirmation,
  }
}
