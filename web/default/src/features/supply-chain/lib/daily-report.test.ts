/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { describe, expect, test } from 'bun:test'
import { isVerificationRequiredError } from '@/lib/secure-verification'
import { createSupplyChainSecurityLifecycle } from '../hooks/use-supply-chain-admin'
import type { SupplierDailyReportDay } from '../types'
import { createDailyReportRerunIntent } from './daily-report'

function verificationRequiredError(): unknown {
  return {
    response: {
      status: 403,
      data: { code: 'VERIFICATION_REQUIRED' },
    },
  }
}

function incompleteDay(): SupplierDailyReportDay {
  return {
    batch_date: '2026-07-22',
    published: true,
    published_fence_token: 7,
    published_at: 1,
    persisted_log_snapshot_completeness: 'incomplete',
    finance_attention_required: true,
    logs_scanned: 1,
    producer_markers_present: 0,
    captured_snapshot_count: 0,
    disposition_counts: {
      captured: 0,
      unsupported_path: 0,
      not_financially_committed: 0,
      zero_usage: 0,
      unbound: 0,
      producer_error: 0,
    },
    failure_counts: {
      unknown_producer_capability: 0,
      incompatible_producer_capability: 0,
      absent_marker_after_cutover: 1,
      invalid_captured_snapshot: 0,
      unknown_official_amount: 0,
    },
    warnings: [],
    known_coverage_gaps: [],
  }
}

describe('daily rerun secure lifecycle', () => {
  test('keeps the exact reason, fence, and key across invalid verification and one verified send', async () => {
    const lifecycle = createSupplyChainSecurityLifecycle()
    let retry: (() => Promise<unknown>) | null = null
    const bridge = {
      withVerification: async (apiCall: () => Promise<unknown>) => {
        try {
          return await apiCall()
        } catch (error) {
          if (!isVerificationRequiredError(error)) throw error
          retry = apiCall
          return null
        }
      },
      reset: () => undefined,
    }
    const intent = createDailyReportRerunIntent(
      incompleteDay(),
      'finance evidence repair',
      null,
      () => 'daily-rerun-stable-key'
    )
    if (!intent) throw new Error('Expected an eligible daily rerun intent')

    const observed = [] as (typeof intent)[]
    let attempts = 0
    const result = lifecycle.execute(async () => {
      attempts += 1
      observed.push(intent)
      if (attempts === 1) throw verificationRequiredError()
      return 'rerun-started'
    }, bridge)

    for (let wait = 0; wait < 10 && !retry; wait += 1) await Promise.resolve()
    await Promise.resolve()
    lifecycle.handleVerificationError(new Error('Invalid authenticator code'))
    expect(lifecycle.isPending()).toBe(true)
    await expect(
      lifecycle.execute(async () => 'replacement-intent', bridge)
    ).rejects.toThrow('already pending')
    if (!retry) throw new Error('Expected the verified rerun callback')
    expect(await retry()).toBe('rerun-started')
    expect(await result).toBe('rerun-started')
    expect(observed).toEqual([intent, intent])
    expect(observed[1]).toMatchObject({
      idempotencyKey: 'daily-rerun-stable-key',
      data: {
        reason: 'finance evidence repair',
        expected_published_fence_token: 7,
      },
    })
    expect(attempts).toBe(2)
  })

  test('releases an intent cancelled during verification before the mutation is sent', async () => {
    const lifecycle = createSupplyChainSecurityLifecycle()
    let retry: (() => Promise<unknown>) | null = null
    const bridge = {
      withVerification: async (apiCall: () => Promise<unknown>) => {
        retry = apiCall
        return null
      },
      reset: () => undefined,
    }

    const pending = lifecycle.execute(async () => 'must-not-send', bridge)
    for (let wait = 0; wait < 10 && !retry; wait += 1) await Promise.resolve()
    lifecycle.cancel()

    await expect(pending).rejects.toMatchObject({
      name: 'SupplyChainVerificationCancelledError',
    })
    expect(lifecycle.isPending()).toBe(false)
    expect(
      await lifecycle.execute(async () => 'new-intent', {
        withVerification: (apiCall) => apiCall(),
        reset: () => undefined,
      })
    ).toBe('new-intent')
  })
})
