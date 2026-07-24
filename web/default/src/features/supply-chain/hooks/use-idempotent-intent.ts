/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useRef, useState } from 'react'

export type IntentRunResult = 'success' | 'failed' | 'blocked'

interface RunIntentOptions {
  execute: (idempotencyKey: string) => Promise<unknown>
}

function createIntentKey(): string {
  return globalThis.crypto.randomUUID()
}

export function getOrCreateIntentKey(
  current: string | null,
  createKey: () => string = createIntentKey
): string {
  return current ?? createKey()
}

export function useIdempotentIntent() {
  const keyRef = useRef<string | null>(null)
  const runningRef = useRef(false)
  const [isSubmitting, setIsSubmitting] = useState(false)

  function clearIntent(): void {
    keyRef.current = null
  }

  async function run(options: RunIntentOptions): Promise<IntentRunResult> {
    if (runningRef.current) return 'blocked'
    const key = getOrCreateIntentKey(keyRef.current)
    keyRef.current = key
    runningRef.current = true
    setIsSubmitting(true)
    try {
      await options.execute(key)
      clearIntent()
      return 'success'
    } catch {
      // Keep the same key so an explicit retry remains idempotent.
      return 'failed'
    } finally {
      runningRef.current = false
      setIsSubmitting(false)
    }
  }

  return {
    run,
    clearIntent,
    isSubmitting,
  }
}
