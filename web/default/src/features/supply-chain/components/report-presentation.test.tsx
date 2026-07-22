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
import type { ReactNode } from 'react'
import { beforeAll, describe, expect, test } from 'bun:test'
import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import type {
  SupplierReportContractList,
  SupplierReportFreshness,
  SupplierReportMetrics,
  SupplierReportOverview,
} from '../types'
import { ProgressiveList } from './progressive-list'
import { ReportBreakdownTable } from './report-breakdown-table'
import { ReportChannelTable } from './report-channel-table'
import { ReportContractTable } from './report-contract-table'
import { ReportFreshnessAlert } from './report-freshness-alert'
import { ReportOverview } from './report-overview'
import { ReportTrend } from './report-trend'

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

function metrics(
  overrides: Partial<SupplierReportMetrics> = {}
): SupplierReportMetrics {
  return {
    request_count: 5,
    unattributed_request_count: 0,
    official_list: { known_count: 5, micro_usd: 10_000_000 },
    sales: { known_count: 5, micro_usd: 7_000_000 },
    procurement_cost: { known_count: 5, micro_usd: 6_500_000 },
    gross_profit: { known_count: 5, micro_usd: 500_000 },
    gross_margin_eligible_count: 5,
    gross_margin_eligible_sales_micro_usd: 7_000_000,
    gross_margin: '0.07142857',
    gross_margin_eligible_coverage: '1',
    ...overrides,
  }
}

function freshness(
  overrides: Partial<SupplierReportFreshness> = {}
): SupplierReportFreshness {
  return {
    sync_only: true,
    coverage_start_at: 1_788_192_000,
    latest_batch_date: '2026-09-15',
    batch_status: 'completed',
    fresh_through: 1_789_488_000,
    freshness_lag_seconds: 7_200,
    error_message: '',
    ...overrides,
  }
}

function overview(
  overrides: Partial<SupplierReportOverview> = {}
): SupplierReportOverview {
  return {
    range: {
      start_at: 1_788_192_000,
      end_at: 1_790_870_400,
      timezone: 'Asia/Shanghai',
      month: '2026-09',
    },
    business: metrics(),
    internal: metrics({
      sales: { known_count: 0, micro_usd: 0 },
      gross_profit: { known_count: 0, micro_usd: 0 },
      gross_margin: null,
    }),
    total_estimated_procurement_cost: {
      known_count: 10,
      micro_usd: 13_000_000,
    },
    total_inventory_micro_usd: 100_000_000,
    official_list_consumed_micro_usd: 20_000_000,
    remaining_inventory_micro_usd: 80_000_000,
    internal_dimension_available: true,
    ...overrides,
  }
}

describe('supply-chain report presentation', () => {
  test('renders unknown money as an em dash instead of a false zero', () => {
    const html = render(
      <ReportOverview
        data={overview({
          business: metrics({
            sales: { known_count: 0, micro_usd: 0 },
            gross_profit: { known_count: 0, micro_usd: 0 },
            gross_margin: null,
          }),
        })}
      />
    )

    expect(html).toContain('Business profit')
    expect(html).toContain('—')
    expect(html).toContain('$80.00')
    expect(html).not.toContain(
      'Financial totals must not be treated as complete'
    )
  })

  test('shows the latest failed batch while preserving the completed data date', () => {
    const html = render(
      <ReportFreshnessAlert
        freshness={freshness({
          latest_batch_date: '2026-09-16',
          batch_status: 'failed',
          error_message: 'database timeout',
        })}
      />
    )

    expect(html).toContain('Supplier report data update failed')
    expect(html).toContain('Synchronous final logs only')
    expect(html).toContain('Coverage starts at')
    expect(html).toContain('Usage before this time is outside report coverage')
    expect(html).toContain('Data is available through')
    expect(html).toContain('database timeout')
    expect(html).toContain('data-variant="destructive"')
  })

  test('keeps null procurement rates unknown and flags oversold contracts', () => {
    const data: SupplierReportContractList = {
      range: overview().range,
      items: [
        {
          contract_id: 1,
          supplier_id: 2,
          supplier_name: 'Upstream A',
          supplier_status: 'active',
          contract_name: 'Primary frame',
          contract_no: 'FRAME-001',
          contract_status: 'active',
          remark: '',
          current_rate_version_id: null,
          procurement_multiplier_ppm: null,
          rpm_limit: 0,
          tpm_limit: 0,
          max_concurrency: 0,
          linked_channel_count: 1,
          total_inventory_micro_usd: 10_000_000,
          official_list_consumed_micro_usd: 11_000_000,
          remaining_inventory_micro_usd: -1_000_000,
          utilization_rate: '1.1',
          oversold: true,
          business: metrics(),
          internal: metrics(),
          total_estimated_procurement_cost: {
            known_count: 10,
            micro_usd: 13_000_000,
          },
          internal_dimension_available: true,
        },
      ],
      limit: 20,
      offset: 0,
      has_more: false,
    }

    const html = render(<ReportContractTable data={data} />)

    expect(html).toContain('Primary frame')
    expect(html).toContain('Oversold')
    expect(html).toContain('−$1.00'.replace('−', '-'))
    expect(html).toContain('—')
  })

  test('uses explicit empty states for aggregate-only report sections', () => {
    const channelHtml = render(
      <ReportChannelTable
        data={{
          range: overview().range,
          items: [],
          limit: 20,
          offset: 0,
          has_more: false,
        }}
      />
    )
    const breakdownHtml = render(
      <ReportBreakdownTable
        data={{
          range: overview().range,
          items: [],
          limit: 20,
          offset: 0,
          has_more: false,
          breakdown_eligible_count: 0,
          total_business_count: 0,
          breakdown_coverage_rate: null,
          breakdown_coverage_available: false,
        }}
      />
    )
    const trendHtml = render(
      <ReportTrend
        data={{
          range: overview().range,
          points: [],
        }}
      />
    )

    expect(channelHtml).toContain('No channel data')
    expect(breakdownHtml).toContain('No pricing breakdown')
    expect(trendHtml).toContain('No trend data')
    expect(trendHtml).toContain('Asia/Shanghai')
  })

  test('renders the same channel under each historical contract', () => {
    const html = render(
      <ReportChannelTable
        data={{
          range: overview().range,
          items: [
            {
              contract_id: 10,
              channel_id: 30,
              channel_name: 'Rebound channel',
              channel_status: 1,
              business: metrics({ request_count: 2 }),
            },
            {
              contract_id: 20,
              channel_id: 30,
              channel_name: 'Rebound channel',
              channel_status: 1,
              business: metrics({ request_count: 0 }),
            },
          ],
          limit: 20,
          offset: 0,
          has_more: false,
        }}
      />
    )

    expect(html.match(/Rebound channel/g)).toHaveLength(2)
    expect(html).toContain('#10')
    expect(html).toContain('#20')
  })

  test('shows daily-batch quality without legacy processing metadata', () => {
    const html = render(
      <ReportBreakdownTable
        data={{
          range: overview().range,
          items: [
            {
              contract_id: 1,
              channel_id: 2,
              model_name: 'gpt-test',
              rate_version_id: 3,
              sales_multiplier_ppm: 700_000,
              pricing_mode: 'ratio',
              data_quality: 'authoritative',
              metrics: metrics(),
            },
          ],
          limit: 20,
          offset: 0,
          has_more: false,
          breakdown_eligible_count: 5,
          total_business_count: 5,
          breakdown_coverage_rate: '1',
          breakdown_coverage_available: true,
        }}
      />
    )

    expect(html).toContain('Authoritative')
    expect(html).toContain('Sales 70%')
    expect(html).not.toContain('Rolled up at')
    expect(html).not.toContain('Estimated backfill')
  })

  test('renders a missing sales multiplier as unknown instead of zero', () => {
    const html = render(
      <ReportBreakdownTable
        data={{
          range: overview().range,
          items: [
            {
              contract_id: 1,
              channel_id: 2,
              model_name: 'gpt-null-multiplier',
              rate_version_id: 3,
              sales_multiplier_ppm: null,
              pricing_mode: 'ratio',
              data_quality: 'authoritative',
              metrics: metrics(),
            },
          ],
          limit: 20,
          offset: 0,
          has_more: false,
          breakdown_eligible_count: 5,
          total_business_count: 5,
          breakdown_coverage_rate: '1',
          breakdown_coverage_available: true,
        }}
      />
    )

    expect(html).toContain('Sales — · ratio')
    expect(html).not.toContain('Sales 0%')
  })

  test('does not imply full history when the cutover instant is unavailable', () => {
    const html = render(
      <ReportFreshnessAlert
        freshness={freshness({ coverage_start_at: null })}
      />
    )

    expect(html).toContain('Supplier report coverage is incomplete')
    expect(html).toContain('Coverage starts at Unavailable')
    expect(html).toContain('data-variant="destructive"')
  })

  test('renders an independent load-more action for every paged report table', () => {
    const contractHtml = render(
      <ReportContractTable
        data={{
          range: overview().range,
          items: [],
          limit: 20,
          offset: 0,
          has_more: true,
        }}
        hasMore
      />
    )
    const channelHtml = render(
      <ReportChannelTable
        data={{
          range: overview().range,
          items: [],
          limit: 20,
          offset: 0,
          has_more: true,
        }}
        hasMore
      />
    )
    const breakdownHtml = render(
      <ReportBreakdownTable
        data={{
          range: overview().range,
          items: [],
          limit: 20,
          offset: 0,
          has_more: true,
          breakdown_eligible_count: 0,
          total_business_count: 0,
          breakdown_coverage_rate: null,
          breakdown_coverage_available: false,
        }}
        hasMore
      />
    )

    expect(contractHtml).toContain('Load more')
    expect(channelHtml).toContain('Load more')
    expect(breakdownHtml).toContain('Load more')
  })

  test('distinguishes progressive-list loading, error, empty, and paged states', () => {
    const loading = render(
      <ProgressiveList isLoading isError={false} isEmpty={false}>
        loaded
      </ProgressiveList>
    )
    const error = render(
      <ProgressiveList isLoading={false} isError isEmpty={false}>
        loaded
      </ProgressiveList>
    )
    const empty = render(
      <ProgressiveList isLoading={false} isError={false} isEmpty>
        loaded
      </ProgressiveList>
    )
    const paged = render(
      <ProgressiveList
        isLoading={false}
        isError={false}
        isEmpty={false}
        hasMore
      >
        loaded
      </ProgressiveList>
    )

    expect(loading).toContain('Loading')
    expect(error).toContain('Unable to load data')
    expect(empty).toContain('No data')
    expect(paged).toContain('loaded')
    expect(paged).toContain('Load more')
  })
})
