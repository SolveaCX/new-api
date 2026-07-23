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
export type SupplierReportBatchStatus = 'running' | 'completed' | 'failed' | ''
export type SupplierPersistedLogCompleteness =
  | 'complete'
  | 'incomplete'
  | 'not_scanned'
export type NullableRatio = string | null
export type MicroUsd = number

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

export interface SupplyChainAdminPageParams {
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
  delta_micro_usd: MicroUsd
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

export interface SupplyChainStatusResult {
  id: number
  status: SupplierStatus
}

export interface SupplyChainCommandResult {
  scope: string
  idempotency_key: string
  resource_type: string
  resource_id: number
  created_at: number
}

export interface SupplierChannelUnbindResult {
  channel_id: number
  supplier_contract_id: null
}

export interface SupplierChannelUnbindVariables {
  expectedContractId: number
  idempotencyKey: string
}

export interface IdempotentMutationVariables<T> {
  data: T
  idempotencyKey: string
}

export interface SupplierReportMonthRange {
  month: string
  startDate?: never
  endDate?: never
}

export interface SupplierReportDateRange {
  month?: never
  startDate: string
  endDate: string
}

export type SupplierReportRangeParams =
  | SupplierReportMonthRange
  | SupplierReportDateRange

export interface SupplierReportFilters {
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

export interface SupplierReportRange {
  start_at: number
  end_at: number
  timezone: 'Asia/Shanghai'
  month?: string
}

export interface SupplierPublishedDispositionCounts {
  captured: number
  unsupported_path: number
  not_financially_committed: number
  zero_usage: number
  unbound: number
  producer_error: number
}

export interface SupplierPublishedFailureCounts {
  unknown_producer_capability: number
  incompatible_producer_capability: number
  absent_marker_after_cutover: number
  invalid_captured_snapshot: number
  unknown_official_amount: number
}

export interface SupplierPublishedWarning {
  code: string
  count: number
  message_key: string
}

export interface SupplierAccountingCoverageGap {
  id: number
  start_at: number
  end_at: number | null
  reason_category: string
  reason_text: string
  expected_capability_version: number
  affected_capability_version: number | null
  affected_oci_digest: string | null
  affected_build_commit: string | null
  activation_state_version_before: number
  activation_state_version_after: number
  open_command_id: string
  close_command_id: string | null
  opened_by: number
  closed_by: number | null
  finance_disposition: string
  evidence_refs: string[]
  record_version: number
  created_at: number
  updated_at: number
}

export interface SupplierDailyReportDay {
  batch_date: string
  published: boolean
  published_fence_token: number
  published_at: number | null
  persisted_log_snapshot_completeness: SupplierPersistedLogCompleteness
  finance_attention_required: boolean
  logs_scanned: number
  producer_markers_present: number
  captured_snapshot_count: number
  disposition_counts: SupplierPublishedDispositionCounts
  failure_counts: SupplierPublishedFailureCounts
  warnings: SupplierPublishedWarning[]
  known_coverage_gaps: SupplierAccountingCoverageGap[]
}

export interface SupplierDailyReportProjection {
  range: SupplierReportRange
  persisted_log_universe: 'successfully_persisted_consume_logs_for_final_successful_settlement'
  days: SupplierDailyReportDay[]
}

export interface SupplierDailyReportRerunRequest {
  reason: string
  expected_published_fence_token: number
}

export interface SupplierDailyReportRerunVariables {
  batchDate: string
  data: SupplierDailyReportRerunRequest
  idempotencyKey: string
}

export interface SupplierDailyReportRerunResult {
  request_id: string
  batch_date: string | null
  run_id: number | null
  status: 'running' | 'completed' | 'failed'
  fence_token: number
  published_fence_token: number
  locked_until: string | null
  error_category: string
  result: {
    processed_days: number
    remaining_work: boolean
    next_batch_date: string | null
  } | null
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

export interface SupplierReportFreshness {
  sync_only: boolean
  coverage_start_at: number | null
  latest_batch_date: string
  batch_status: SupplierReportBatchStatus
  fresh_through: number | null
  freshness_lag_seconds: number | null
  error_message: string
}

export interface SupplierReportOverview {
  range: SupplierReportRange
  business: SupplierReportMetrics
  internal: SupplierReportMetrics
  total_estimated_procurement_cost: SupplierReportMoney
  total_inventory_micro_usd: MicroUsd
  official_list_consumed_micro_usd: MicroUsd
  remaining_inventory_micro_usd: MicroUsd
  internal_dimension_available: boolean
}

export interface SupplierReportTrendPoint {
  bucket_start: number
  date: string
  business: SupplierReportMetrics
  internal: SupplierReportMetrics
  internal_dimension_available: boolean
}

export interface SupplierReportTrend {
  range: SupplierReportRange
  points: SupplierReportTrendPoint[]
}

export interface SupplierReportContractRow {
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
  total_estimated_procurement_cost: SupplierReportMoney
  internal_dimension_available: boolean
}

export interface SupplierReportContractList {
  range: SupplierReportRange
  items: SupplierReportContractRow[]
  limit: number
  offset: number
  has_more: boolean
}

export interface SupplierReportRateVersion {
  id: number
  procurement_multiplier_ppm: number
  effective_at: number
  created_by: number
  reason: string
  created_at: number
}

export interface SupplierReportInventoryAdjustment {
  id: number
  delta_micro_usd: MicroUsd
  type: SupplierInventoryAdjustmentType
  reason: string
  idempotency_key: string
  created_by: number
  created_at: number
}

export interface SupplierReportChannelRow {
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

export interface SupplierReportBreakdownItem {
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

export interface SupplierReportContractDetail {
  range: SupplierReportRange
  summary: SupplierReportContractRow
  rate_versions: SupplierReportRateVersion[]
  inventory_adjustments: SupplierReportInventoryAdjustment[]
  channels: SupplierReportChannelList
  internal_trend: SupplierReportTrendPoint[]
  breakdown: SupplierReportBreakdownList
}
