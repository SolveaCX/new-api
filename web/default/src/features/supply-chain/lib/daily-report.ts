/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import axios from 'axios'
import type {
  SupplierDailyReportDay,
  SupplierDailyReportRerunVariables,
} from '../types'

export function isDailyRerunVerificationUnavailable(error: unknown): boolean {
  return (
    error instanceof Error &&
    error.message ===
      'No verification methods available. Enable 2FA or Passkey to continue.'
  )
}

export function shouldRetainDailyReportRerunIntent(error: unknown): boolean {
  if (!axios.isAxiosError(error)) return false
  const status = error.response?.status
  return (
    status === undefined || status >= 500 || status === 408 || status === 429
  )
}

export function shouldReleaseDailyReportRerunIntentOnDismiss(
  pending: SupplierDailyReportRerunVariables | null
): boolean {
  return pending === null
}

export function canRerunDailyReport(day: SupplierDailyReportDay): boolean {
  return (
    day.published &&
    day.persisted_log_snapshot_completeness === 'incomplete' &&
    day.published_fence_token > 0
  )
}

export function createDailyReportRerunIntent(
  day: SupplierDailyReportDay,
  reason: string,
  pending: SupplierDailyReportRerunVariables | null,
  createKey: () => string = () => globalThis.crypto.randomUUID()
): SupplierDailyReportRerunVariables | null {
  if (!canRerunDailyReport(day) || !reason.trim()) return null
  if (pending) return pending
  return {
    batchDate: day.batch_date,
    idempotencyKey: createKey(),
    data: {
      reason: reason.trim(),
      expected_published_fence_token: day.published_fence_token,
    },
  }
}
