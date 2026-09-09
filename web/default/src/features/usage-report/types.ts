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
  registered: number // 该日注册（enabled+verified，人）
  activated_key: number // 当日首次建 Key 的去重用户数（事件口径，备用）
  first_paid: number // 当日首次付费的去重用户数（事件口径，备用）
  paid_usd: number
  // 当天口径（主口径，C 端快进快出；⊆ registered）：
  activated_day: number // 该日注册的人中当天首次建 Key 的人数
  paid_day: number // 该日注册的人中当天首次付费的人数
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
  filling?: boolean // 历史窗口仍在后台回填中，前端应轮询直到消失
}
