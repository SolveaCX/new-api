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

function createIntentKey(): string {
  return globalThis.crypto.randomUUID()
}

function responseStatus(error: unknown): number | undefined {
  return axios.isAxiosError(error) ? error.response?.status : undefined
}

function hasUnknownOutcome(error: unknown): boolean {
  if (!axios.isAxiosError(error)) return false
  return error.response === undefined || error.response.status >= 500
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
    const key = keyRef.current ?? createIntentKey()
    keyRef.current = key
    reconcileRef.current = () => options.reconcile(key)
    runningRef.current = true
    setIsSubmitting(true)
    try {
      await options.execute(key)
      clearIntent()
      return 'success'
    } catch (error) {
      const status = responseStatus(error)
      if (status === 409) {
        try {
          await options.reconcile(key)
        } catch {
          // The conflict is known even if refreshing the latest state fails.
        }
        clearIntent()
        return 'conflict'
      }
      if (status === 429) {
        return 'rate_limited'
      }
      if (hasUnknownOutcome(error)) {
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

  async function reconcilePending(): Promise<void> {
    const reconcile = reconcileRef.current
    if (!reconcile) return
    runningRef.current = true
    setIsSubmitting(true)
    try {
      if (await reconcile()) clearIntent()
      else setIsPendingConfirmation(false)
    } catch {
      setIsPendingConfirmation(true)
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
