/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { describe, expect, test } from 'bun:test'
import axios from 'axios'
import {
  classifyIntentError,
  getOrCreateIntentKey,
  reconcileIntentResult,
} from './use-idempotent-intent'

function httpError(status: number): axios.AxiosError {
  return new axios.AxiosError(
    `HTTP ${status}`,
    undefined,
    undefined,
    undefined,
    { status } as never
  )
}

describe('shared idempotent intent lifecycle', () => {
  test('retains the exact key and payload for HTTP 408 reconciliation and retry', () => {
    const key = getOrCreateIntentKey(null, () => 'stable-408-key')
    const original = {
      idempotencyKey: key,
      values: {
        expected_version: 7,
        reason: 'exact append-only command',
      },
    }

    expect(classifyIntentError(httpError(408))).toBe('unknown')
    const retriedKey = getOrCreateIntentKey(key, () => 'replacement-key')
    const reconciled = { ...original, idempotencyKey: retriedKey }

    expect(retriedKey).toBe('stable-408-key')
    expect(reconciled).toEqual(original)
  })

  test('preserves the existing 429 policy and releases known terminal outcomes', () => {
    expect(classifyIntentError(httpError(429))).toBe('rate_limited')
    expect(classifyIntentError(httpError(409))).toBe('conflict')
    expect(classifyIntentError(httpError(400))).toBe('failed')

    const cancelled = new Error('cancelled')
    cancelled.name = 'SupplyChainVerificationCancelledError'
    expect(classifyIntentError(cancelled)).toBe('verification_cancelled')
  })

  test('reports committed reconciliation without issuing a second mutation', async () => {
    let reconcileCalls = 0
    let mutationCalls = 0
    const sendMutation = () => {
      mutationCalls += 1
    }
    sendMutation()
    const result = await reconcileIntentResult(async () => {
      reconcileCalls += 1
      return true
    })

    expect(result).toBe('reconciled')
    expect(reconcileCalls).toBe(1)
    expect(mutationCalls).toBe(1)

    expect(await reconcileIntentResult(async () => false)).toBe('failed')
    expect(
      await reconcileIntentResult(async () => {
        throw new Error('still unknown')
      })
    ).toBe('pending_confirmation')
  })
})
