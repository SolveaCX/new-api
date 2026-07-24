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
import { beforeAll, describe, expect, test } from 'bun:test'
import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import type {
  SupplierReportContractList,
  SupplierReportMetrics,
  SupplierReportOverview,
} from '../types'
import { ReportContractTable } from './report-contract-table'
import { ReportOverview } from './report-overview'

const testI18n = createInstance()

beforeAll(async () => {
  await testI18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })
})

const emptyMetrics: SupplierReportMetrics = {
  request_count: 0,
  unattributed_request_count: 0,
  official_list: { known_count: 0, micro_usd: '0' },
  sales: { known_count: 0, micro_usd: '0' },
  procurement_cost: { known_count: 0, micro_usd: '0' },
  gross_profit: { known_count: 0, micro_usd: '0' },
  gross_margin_eligible_count: 0,
  gross_margin_eligible_sales_micro_usd: '0',
  gross_margin: null,
  gross_margin_eligible_coverage: null,
}

function render(element: React.ReactNode): string {
  return renderToStaticMarkup(
    <I18nextProvider i18n={testI18n}>{element}</I18nextProvider>
  )
}

describe('supply-chain unavailable financial dimensions', () => {
  test('renders overview internal values as unknown when the total is unavailable', () => {
    const data: SupplierReportOverview = {
      range: { start_at: 1, end_at: 2, timezone: 'Asia/Shanghai' },
      business: emptyMetrics,
      internal: emptyMetrics,
      total_estimated_procurement_cost: null,
      total_inventory_micro_usd: '0',
      official_list_consumed_micro_usd: '0',
      remaining_inventory_micro_usd: '0',
      internal_dimension_available: false,
    }

    const html = render(<ReportOverview data={data} />)

    expect(html).toMatch(/Internal requests[\s\S]{0,180}>—<\/div>/)
    expect(html).toMatch(/Estimated procurement cost[\s\S]{0,180}>—<\/div>/)
  })

  test('renders a contract internal cost as unknown when its total is unavailable', () => {
    const data = {
      range: { start_at: 1, end_at: 2, timezone: 'Asia/Shanghai' },
      items: [
        {
          contract_id: 7,
          supplier_id: 3,
          supplier_name: 'Supplier',
          supplier_status: 'active',
          contract_name: 'Contract',
          contract_no: 'C-7',
          contract_status: 'active',
          remark: '',
          current_rate_version_id: 2,
          procurement_multiplier_ppm: 800_000,
          rpm_limit: 0,
          tpm_limit: 0,
          max_concurrency: 0,
          linked_channel_count: 1,
          total_inventory_micro_usd: '0',
          official_list_consumed_micro_usd: '0',
          remaining_inventory_micro_usd: '0',
          utilization_rate: null,
          oversold: false,
          business: emptyMetrics,
          internal: emptyMetrics,
          total_estimated_procurement_cost: null,
          internal_dimension_available: false,
        },
      ],
      limit: 20,
      offset: 0,
      has_more: false,
    } satisfies SupplierReportContractList

    const html = render(<ReportContractTable data={data} />)

    expect(html).toMatch(/Internal cost[\s\S]*<td[^>]*>—<\/td>/)
  })
})
