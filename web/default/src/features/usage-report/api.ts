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
import { api } from '@/lib/api'
import type { ApiResponse, UsageReportData } from './types'

export type UsageReportGroup = 'plg' | 'all'

export const usageReportQueryKeys = {
  all: ['usage-report'] as const,
  report: (days: number, group: UsageReportGroup) =>
    [...usageReportQueryKeys.all, days, group] as const,
}

export interface UsageReportFillResult {
  filled: string[]
  remaining: number
  last_error?: string
}

/** Ask the server to compute a small batch of missing days inside a request
 * (Cloud Run throttles CPU for idle background work, so the page drives it). */
export async function fillUsageReport(
  days: number,
  batch = 2
): Promise<ApiResponse<UsageReportFillResult>> {
  const res = await api.get('/api/data/usage_report_fill', { params: { days, batch } })
  return res.data
}

export async function getUsageReport(
  days: number,
  group: UsageReportGroup
): Promise<ApiResponse<UsageReportData>> {
  const res = await api.get('/api/data/usage_report', { params: { days, group } })
  return res.data
}

export async function downloadUsageReportCSV(
  days: number,
  dim: 'daily' | 'models',
  group: UsageReportGroup
): Promise<void> {
  const res = await api.get('/api/data/usage_report', {
    params: { days, format: 'csv', dim, group },
    responseType: 'blob',
  })
  const url = URL.createObjectURL(res.data as Blob)
  const a = document.createElement('a')
  a.href = url
  a.download =
    dim === 'models' ? `usage_report_models_${days}d.csv` : `usage_report_daily_${days}d.csv`
  document.body.appendChild(a)
  a.click()
  setTimeout(() => {
    URL.revokeObjectURL(url)
    a.remove()
  }, 300)
}
