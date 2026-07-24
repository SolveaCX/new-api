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
export type SupplierStatus = 'active' | 'inactive'
export type SupplierInventoryAdjustmentType =
  | 'initial'
  | 'replenishment'
  | 'correction'
  | 'reversal'
export type SupplierStatisticsAction = 'exclude' | 'include'
export type SupplierDataQuality = 'authoritative' | 'unattributed'
export type NullableRatio = string | null
export type MicroUsd = string | number

export interface SupplyChainApiResponse<T> {
  success: boolean
  message?: string
  data: T
}

export interface SupplyChainAdminPage<T> {
  page: number
  page_size: number
  total: number
  items: T[]
}

interface SupplyChainAdminPageParams {
  p: number
  page_size: number
}

export interface SupplierListParams extends SupplyChainAdminPageParams {
  status?: SupplierStatus
  keyword?: string
}

export interface SupplierContractListParams extends SupplyChainAdminPageParams {
  supplier_id?: number
  status?: SupplierStatus
  keyword?: string
}

export interface SupplierContractChildListParams extends SupplyChainAdminPageParams {
  contract_id: number
}

export interface SupplierExclusionListParams extends SupplyChainAdminPageParams {
  user_id?: number
  action?: SupplierStatisticsAction
  keyword?: string
  current_only?: boolean
}

export interface SupplierChannelBindingListParams extends SupplyChainAdminPageParams {
  contract_id?: number
  keyword?: string
  bound_state?: 'bound' | 'unbound'
  channel_status?: number
}

export interface UpstreamSupplier {
  id: number
  name: string
  status: SupplierStatus
  remark: string
  contract_count: number
  active_contract_count: number
  linked_channel_count: number
  inventory_total_micro_usd: MicroUsd
  row_version: number
  created_at: number
  updated_at: number
}

export interface SupplierCreateRequest {
  name: string
  remark: string
}

export interface SupplierUpdateRequest {
  name?: string
  remark?: string
  expected_version: number
}

export interface SupplierInactivateRequest {
  expected_version: number
}

export interface SupplierContract {
  id: number
  supplier_id: number
  name: string
  contract_no: string
  remark: string
  status: SupplierStatus
  supplier_name: string
  current_rate_version_id: number | null
  current_procurement_multiplier_ppm: number | null
  current_rate_effective_at: number | null
  inventory_total_micro_usd: MicroUsd
  linked_channel_count: number
  rpm_limit: number
  tpm_limit: number
  max_concurrency: number
  row_version: number
  created_at: number
  updated_at: number
}

export interface SupplierContractCreateRequest {
  supplier_id: number
  name: string
  contract_no: string
  remark: string
  rpm_limit: number
  tpm_limit: number
  max_concurrency: number
}

export interface SupplierContractUpdateRequest {
  name?: string
  contract_no?: string
  remark?: string
  rpm_limit?: number
  tpm_limit?: number
  max_concurrency?: number
  expected_version: number
}

export interface SupplierContractInactivateRequest {
  expected_version: number
}

export interface SupplierContractRateVersion {
  id: number
  contract_id: number
  procurement_multiplier_ppm: number
  effective_at: number
  created_by: number
  reason: string
  created_at: number
}

export interface SupplierRateVersionCreateRequest {
  procurement_multiplier_ppm: number
  reason: string
}

export interface SupplierInventoryAdjustment {
  id: number
  contract_id: number
  delta_micro_usd: MicroUsd
  type: SupplierInventoryAdjustmentType
  reason: string
  idempotency_key: string
  created_by: number
  created_at: number
}

export interface SupplierInventoryAdjustmentCreateRequest {
  delta_micro_usd: number
  type: SupplierInventoryAdjustmentType
  reason: string
}

export interface SupplierStatisticsExclusionRule {
  id: number
  user_id: number
  action: SupplierStatisticsAction
  effective_at: number
  reason: string
  idempotency_key: string
  created_by: number
  created_at: number
}

export interface SupplierEffectiveExclusion {
  rule_id: number
  user_id: number
  username: string
  display_name: string
  role: number | null
  status: number | null
  identity_present: boolean
  action: SupplierStatisticsAction
  excluded: boolean
  effective_at: number
  reason: string
  created_by: number
  created_at: number
}

export interface SupplierExclusionRuleCreateRequest {
  user_id: number
  action: SupplierStatisticsAction
  reason: string
}

export interface SupplierChannelBinding {
  channel_id: number
  channel_name: string
  channel_status: number
  supplier_contract_id: number | null
  contract_name: string | null
  contract_no: string | null
  supplier_id: number | null
  supplier_name: string | null
  current_rate_version_id: number | null
  current_procurement_multiplier_ppm: number | null
}

export interface SupplierChannelBindingRequest {
  contract_id: number
  expected_contract_id: number
}

export interface SupplierChannelUnbindResult {
  channel_id: number
  supplier_contract_id: null
}

export interface SupplierChannelUnbindVariables {
  expectedContractId: number
}

export interface IdempotentMutationVariables<T> {
  data: T
  idempotencyKey: string
}

interface SupplierReportMonthRange {
  month: string
  startDate?: never
  endDate?: never
}

interface SupplierReportDateRange {
  month?: never
  startDate: string
  endDate: string
}

type SupplierReportRangeParams =
  | SupplierReportMonthRange
  | SupplierReportDateRange

interface SupplierReportFilters {
  supplierIds?: readonly number[]
  contractIds?: readonly number[]
  channelIds?: readonly number[]
}

export type SupplierReportQuery = SupplierReportRangeParams &
  SupplierReportFilters

export type SupplierReportPageQuery = SupplierReportQuery & {
  limit?: number
  offset?: number
}

interface SupplierReportRange {
  start_at: number
  end_at: number
  timezone: 'Asia/Shanghai'
  month?: string
}

export interface SupplierReportMoney {
  known_count: number
  micro_usd: MicroUsd
}

export interface SupplierReportMetrics {
  request_count: number
  unattributed_request_count: number
  official_list: SupplierReportMoney
  sales: SupplierReportMoney
  procurement_cost: SupplierReportMoney
  gross_profit: SupplierReportMoney
  gross_margin_eligible_count: number
  gross_margin_eligible_sales_micro_usd: MicroUsd
  gross_margin: NullableRatio
  gross_margin_eligible_coverage: NullableRatio
}

export interface SupplierReportOverview {
  range: SupplierReportRange
  business: SupplierReportMetrics
  internal: SupplierReportMetrics
  total_estimated_procurement_cost: SupplierReportMoney | null
  total_inventory_micro_usd: MicroUsd
  official_list_consumed_micro_usd: MicroUsd
  remaining_inventory_micro_usd: MicroUsd
  internal_dimension_available: boolean
}

interface SupplierReportTrendPoint {
  bucket_start: number
  date: string
  business: SupplierReportMetrics
  internal: SupplierReportMetrics
  internal_dimension_available: boolean
}

export interface SupplierReportTrend {
  range: SupplierReportRange
  points: SupplierReportTrendPoint[]
  day_statuses: SupplierReportDayState[]
  latest_completed_date: string | null
  has_incomplete_days: boolean
  incomplete_day_count: number
}

type SupplierReportDayStatus = 'completed' | 'running' | 'failed' | 'missing'

interface SupplierReportDayState {
  date: string
  status: SupplierReportDayStatus
}

interface SupplierReportContractRow {
  contract_id: number
  supplier_id: number
  supplier_name: string
  supplier_status: SupplierStatus
  contract_name: string
  contract_no: string
  contract_status: SupplierStatus
  remark: string
  current_rate_version_id: number | null
  procurement_multiplier_ppm: number | null
  rpm_limit: number
  tpm_limit: number
  max_concurrency: number
  linked_channel_count: number
  total_inventory_micro_usd: MicroUsd
  official_list_consumed_micro_usd: MicroUsd
  remaining_inventory_micro_usd: MicroUsd
  utilization_rate: NullableRatio
  oversold: boolean
  business: SupplierReportMetrics
  internal: SupplierReportMetrics
  total_estimated_procurement_cost: SupplierReportMoney | null
  internal_dimension_available: boolean
}

export interface SupplierReportContractList {
  range: SupplierReportRange
  items: SupplierReportContractRow[]
  limit: number
  offset: number
  has_more: boolean
}

interface SupplierReportChannelRow {
  channel_id: number
  channel_name: string
  channel_status: number
  contract_id: number
  business: SupplierReportMetrics
}

export interface SupplierReportChannelList {
  range: SupplierReportRange
  items: SupplierReportChannelRow[]
  limit: number
  offset: number
  has_more: boolean
}

interface SupplierReportBreakdownItem {
  contract_id: number
  channel_id: number
  model_name: string
  rate_version_id: number
  sales_multiplier_ppm: number | null
  pricing_mode: string
  data_quality: SupplierDataQuality
  metrics: SupplierReportMetrics
}

export interface SupplierReportBreakdownList {
  range: SupplierReportRange
  items: SupplierReportBreakdownItem[]
  limit: number
  offset: number
  has_more: boolean
  breakdown_eligible_count: number
  total_business_count: number
  breakdown_coverage_rate: NullableRatio
  breakdown_coverage_available: boolean
}
