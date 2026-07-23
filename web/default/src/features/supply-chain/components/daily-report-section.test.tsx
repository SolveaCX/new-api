/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import type { ReactNode } from 'react'
import { beforeAll, describe, expect, test } from 'bun:test'
import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import {
  canRerunDailyReport,
  createDailyReportRerunIntent,
  isDailyRerunVerificationUnavailable,
  shouldReleaseDailyReportRerunIntentOnDismiss,
  shouldRetainDailyReportRerunIntent,
} from '../lib/daily-report'
import type {
  SupplierDailyReportDay,
  SupplierDailyReportProjection,
} from '../types'
import { DailyReportTable } from './daily-report-section'

const testI18n = createInstance()

beforeAll(async () => {
  await testI18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })
})

function render(element: ReactNode): string {
  return renderToStaticMarkup(
    <I18nextProvider i18n={testI18n}>{element}</I18nextProvider>
  )
}

function day(
  overrides: Partial<SupplierDailyReportDay> = {}
): SupplierDailyReportDay {
  return {
    batch_date: '2026-07-22',
    published: true,
    published_fence_token: 7,
    published_at: 1_769_004_000,
    persisted_log_snapshot_completeness: 'complete',
    finance_attention_required: false,
    logs_scanned: 12,
    producer_markers_present: 11,
    captured_snapshot_count: 9,
    disposition_counts: {
      captured: 9,
      unsupported_path: 1,
      not_financially_committed: 0,
      zero_usage: 0,
      unbound: 0,
      producer_error: 1,
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
    ...overrides,
  }
}

function projection(
  days: SupplierDailyReportDay[]
): SupplierDailyReportProjection {
  return {
    range: {
      start_at: 1_769_000_000,
      end_at: 1_769_086_400,
      timezone: 'Asia/Shanghai',
      month: '2026-07',
    },
    persisted_log_universe:
      'successfully_persisted_consume_logs_for_final_successful_settlement',
    days,
  }
}

describe('daily report presentation', () => {
  test('shows the V1 persisted-log boundary and fresh gap warning explicitly', () => {
    const html = render(
      <DailyReportTable
        data={projection([
          day({
            finance_attention_required: true,
            known_coverage_gaps: [
              {
                id: 5,
                start_at: 1_769_000_000,
                end_at: null,
                reason_category: 'log_write_failure',
                reason_text: 'writer outage confirmed by incident 42',
                expected_capability_version: 2,
                affected_capability_version: 1,
                affected_oci_digest: null,
                affected_build_commit: null,
                activation_state_version_before: 1,
                activation_state_version_after: 2,
                open_command_id: 'gap-open-5',
                close_command_id: null,
                opened_by: 1,
                closed_by: null,
                finance_disposition: 'pending',
                evidence_refs: [],
                record_version: 1,
                created_at: 1_769_000_000,
                updated_at: 1_769_000_000,
              },
            ],
          }),
        ])}
      />
    )

    expect(html).toContain('V1 persisted-log universe')
    expect(html).toContain(
      'Absent or unpersisted logs are outside historical completeness.'
    )
    expect(html).toContain('writer outage confirmed by incident 42')
    expect(html).toContain('Fresh coverage gaps')
    expect(html).toContain('Finance attention required')
  })

  test('offers rerun only for a published incomplete row with a positive fence', () => {
    const rows = [
      day({ batch_date: '2026-07-20' }),
      day({
        batch_date: '2026-07-21',
        persisted_log_snapshot_completeness: 'incomplete',
        finance_attention_required: true,
      }),
      day({
        batch_date: '2026-07-22',
        published: false,
        published_fence_token: 0,
        published_at: null,
        persisted_log_snapshot_completeness: 'not_scanned',
        finance_attention_required: true,
      }),
    ]
    const html = render(
      <DailyReportTable data={projection(rows)} onRerun={() => undefined} />
    )

    expect(rows.map(canRerunDailyReport)).toEqual([false, true, false])
    expect(html.match(/Rerun incomplete day/g)).toHaveLength(1)
    expect(html).toContain('aria-label="Published daily supply-chain evidence"')
  })

  test('retains one exact CAS intent across verification and transport retry', () => {
    let keyCalls = 0
    const incomplete = day({
      persisted_log_snapshot_completeness: 'incomplete',
    })
    const first = createDailyReportRerunIntent(
      incomplete,
      '  finance requested rerun  ',
      null,
      () => {
        keyCalls += 1
        return 'stable-rerun-key'
      }
    )
    const retried = createDailyReportRerunIntent(
      day({
        batch_date: '2026-07-23',
        published_fence_token: 99,
        persisted_log_snapshot_completeness: 'incomplete',
      }),
      'different second intent',
      first,
      () => {
        keyCalls += 1
        return 'replacement-key'
      }
    )

    expect(retried).toBe(first)
    expect(retried).toEqual({
      batchDate: '2026-07-22',
      idempotencyKey: 'stable-rerun-key',
      data: {
        reason: 'finance requested rerun',
        expected_published_fence_token: 7,
      },
    })
    expect(keyCalls).toBe(1)
  })

  test('releases an unavailable-verification intent but retains ambiguous outcomes', () => {
    expect(
      isDailyRerunVerificationUnavailable(
        new Error(
          'No verification methods available. Enable 2FA or Passkey to continue.'
        )
      )
    ).toBe(true)
    expect(
      shouldRetainDailyReportRerunIntent({
        isAxiosError: true,
        response: undefined,
      })
    ).toBe(true)
    expect(
      shouldRetainDailyReportRerunIntent({
        isAxiosError: true,
        response: { status: 503 },
      })
    ).toBe(true)
    expect(
      shouldRetainDailyReportRerunIntent({
        isAxiosError: true,
        response: { status: 409 },
      })
    ).toBe(false)
  })

  test('keeps the same request anchor across ambiguous close, reopen, and retry', () => {
    const original = createDailyReportRerunIntent(
      day({ persisted_log_snapshot_completeness: 'incomplete' }),
      'finance evidence repair',
      null,
      () => 'lost-response-key'
    )
    if (!original) throw new Error('Expected an eligible daily rerun intent')

    for (const error of [
      { isAxiosError: true, response: undefined },
      { isAxiosError: true, response: { status: 503 } },
    ]) {
      expect(shouldRetainDailyReportRerunIntent(error)).toBe(true)
      // Cancel and Escape share closeRerun; both may hide but must not release.
      expect(shouldReleaseDailyReportRerunIntentOnDismiss(original)).toBe(false)
      expect(shouldReleaseDailyReportRerunIntentOnDismiss(original)).toBe(false)
      const retried = createDailyReportRerunIntent(
        day({
          batch_date: '2026-07-23',
          published_fence_token: 99,
          persisted_log_snapshot_completeness: 'incomplete',
        }),
        'replacement payload must not win',
        original,
        () => 'replacement-key'
      )
      expect(retried).toBe(original)
      expect(retried).toEqual({
        batchDate: '2026-07-22',
        idempotencyKey: 'lost-response-key',
        data: {
          reason: 'finance evidence repair',
          expected_published_fence_token: 7,
        },
      })
    }

    expect(shouldReleaseDailyReportRerunIntentOnDismiss(null)).toBe(true)
  })

  test('renders accessible loading, empty, and error states', () => {
    const loading = render(<DailyReportTable isLoading />)
    const empty = render(<DailyReportTable data={projection([])} />)
    const error = render(<DailyReportTable isError />)

    expect(loading).toContain('aria-label="Loading daily reports"')
    expect(empty).toContain('No eligible daily reports')
    expect(error).toContain('role="alert"')
    expect(error).toContain('Unable to load published daily reports')
  })
})
