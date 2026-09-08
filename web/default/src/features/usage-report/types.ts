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
export interface ApiResponse<T> {
  success: boolean
  message: string
  data: T
}

export interface UsageReportDayRow {
  date: string // UTC+0 yyyy-mm-dd
  registered: number
  activated_key: number // 当日首次建 Key 的去重用户数（人）
  first_paid: number
  paid_usd: number
  // cohort 队列口径（人）：该日注册者 7 日内建 Key 人数 / 该日建Key者 14 日内首付人数
  activated_c7: number
  paid_c14: number
  calls: number
  prompt_tokens: number
  completion_tokens: number
}

export interface UsageReportModelRow {
  date: string
  model_name: string
  calls: number
  prompt_tokens: number
  completion_tokens: number
}

export interface UsageReportData {
  days: UsageReportDayRow[]
  models: UsageReportModelRow[]
}
