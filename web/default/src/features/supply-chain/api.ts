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
import axios from 'axios'
import { api } from '@/lib/api'
import type {
  IdempotentMutationVariables,
  SupplierChannelBinding,
  SupplierChannelBindingListParams,
  SupplierChannelBindingRequest,
  SupplierChannelUnbindResult,
  SupplierContract,
  SupplierContractChildListParams,
  SupplierContractCreateRequest,
  SupplierContractListParams,
  SupplierContractRateVersion,
  SupplierContractUpdateRequest,
  SupplierCreateRequest,
  SupplierExclusionListParams,
  SupplierEffectiveExclusion,
  SupplierExclusionRuleCreateRequest,
  SupplierInventoryAdjustment,
  SupplierInventoryAdjustmentCreateRequest,
  SupplierListParams,
  SupplierRateVersionCreateRequest,
  SupplierReportBreakdownList,
  SupplierReportChannelList,
  SupplierReportContractDetail,
  SupplierReportContractList,
  SupplierReportFreshness,
  SupplierReportOverview,
  SupplierReportPageQuery,
  SupplierReportQuery,
  SupplierReportTrend,
  SupplierStatisticsExclusionRule,
  SupplierUpdateRequest,
  SupplyChainAdminPage,
  SupplyChainApiResponse,
  SupplyChainCommandResult,
  SupplyChainStatusResult,
  UpstreamSupplier,
} from './types'

const SUPPLY_CHAIN_API = '/api/supply-chain'

type SupplierReportFreshnessWire = Omit<
  SupplierReportFreshness,
  'sync_only' | 'coverage_start_at'
> & {
  sync_only?: unknown
  coverage_start_at?: unknown
}

export async function isSupplyChainCommandCommitted(
  scope: string,
  idempotencyKey: string
): Promise<boolean> {
  try {
    const response = await api.get<
      SupplyChainApiResponse<SupplyChainCommandResult>
    >(`${SUPPLY_CHAIN_API}/commands/result`, {
      params: { scope },
      headers: idempotencyHeaders(idempotencyKey),
    })
    const result = response.data.data
    if (
      result.scope !== scope ||
      result.idempotency_key !== idempotencyKey ||
      !Number.isSafeInteger(result.resource_id) ||
      result.resource_id <= 0
    ) {
      throw new TypeError('Invalid supply-chain command result')
    }
    return true
  } catch (error) {
    if (axios.isAxiosError(error) && error.response?.status === 404) {
      return false
    }
    throw error
  }
}

function idempotencyHeaders(idempotencyKey: string): Record<string, string> {
  return { 'Idempotency-Key': idempotencyKey }
}

function normalizeAdminPageResponse<T>(
  response: SupplyChainApiResponse<SupplyChainAdminPage<T>>
): SupplyChainApiResponse<SupplyChainAdminPage<T>> {
  const items: unknown = response.data.items
  if (Array.isArray(items)) return response
  if (items !== null) {
    throw new TypeError(
      'Invalid supply-chain admin page: items must be an array or null'
    )
  }
  return {
    ...response,
    data: { ...response.data, items: [] },
  }
}

function appendCsv(
  target: Record<string, string | number>,
  key: string,
  values: readonly (number | string)[] | undefined
): void {
  if (values && values.length > 0) {
    target[key] = values.join(',')
  }
}

export function buildSupplierReportQueryParams(
  query: SupplierReportQuery | SupplierReportPageQuery
): Record<string, string | number> {
  const params: Record<string, string | number> = {}
  if ('month' in query && query.month) {
    params.month = query.month
  } else if (
    'startDate' in query &&
    'endDate' in query &&
    query.startDate !== undefined &&
    query.endDate !== undefined
  ) {
    params.start_date = query.startDate
    params.end_date = query.endDate
  }
  appendCsv(params, 'supplier_ids', query.supplierIds)
  appendCsv(params, 'contract_ids', query.contractIds)
  appendCsv(params, 'channel_ids', query.channelIds)
  if ('limit' in query && query.limit !== undefined) {
    params.limit = query.limit
  }
  if ('offset' in query && query.offset !== undefined) {
    params.offset = query.offset
  }
  return params
}

export async function listSuppliers(
  params: SupplierListParams
): Promise<SupplyChainApiResponse<SupplyChainAdminPage<UpstreamSupplier>>> {
  const response = await api.get(`${SUPPLY_CHAIN_API}/suppliers`, { params })
  return normalizeAdminPageResponse(response.data)
}

export async function createSupplier(
  variables: IdempotentMutationVariables<SupplierCreateRequest>
): Promise<SupplyChainApiResponse<UpstreamSupplier>> {
  const response = await api.post(
    `${SUPPLY_CHAIN_API}/suppliers`,
    variables.data,
    { headers: idempotencyHeaders(variables.idempotencyKey) }
  )
  return response.data
}

export async function updateSupplier(
  supplierId: number,
  data: SupplierUpdateRequest
): Promise<SupplyChainApiResponse<UpstreamSupplier>> {
  const response = await api.patch(
    `${SUPPLY_CHAIN_API}/suppliers/${supplierId}`,
    data
  )
  return response.data
}

export async function inactivateSupplier(
  supplierId: number
): Promise<SupplyChainApiResponse<SupplyChainStatusResult>> {
  const response = await api.post(
    `${SUPPLY_CHAIN_API}/suppliers/${supplierId}/inactivate`
  )
  return response.data
}

export async function listContracts(
  params: SupplierContractListParams
): Promise<SupplyChainApiResponse<SupplyChainAdminPage<SupplierContract>>> {
  const response = await api.get(`${SUPPLY_CHAIN_API}/contracts`, { params })
  return normalizeAdminPageResponse(response.data)
}

export async function createContract(
  variables: IdempotentMutationVariables<SupplierContractCreateRequest>
): Promise<SupplyChainApiResponse<SupplierContract>> {
  const response = await api.post(
    `${SUPPLY_CHAIN_API}/contracts`,
    variables.data,
    { headers: idempotencyHeaders(variables.idempotencyKey) }
  )
  return response.data
}

export async function updateContract(
  contractId: number,
  data: SupplierContractUpdateRequest
): Promise<SupplyChainApiResponse<SupplierContract>> {
  const response = await api.patch(
    `${SUPPLY_CHAIN_API}/contracts/${contractId}`,
    data
  )
  return response.data
}

export async function inactivateContract(
  contractId: number
): Promise<SupplyChainApiResponse<SupplyChainStatusResult>> {
  const response = await api.post(
    `${SUPPLY_CHAIN_API}/contracts/${contractId}/inactivate`
  )
  return response.data
}

export async function listRateVersions(
  params: SupplierContractChildListParams
): Promise<
  SupplyChainApiResponse<SupplyChainAdminPage<SupplierContractRateVersion>>
> {
  const response = await api.get(
    `${SUPPLY_CHAIN_API}/contracts/${params.contract_id}/rates`,
    { params: { p: params.p, page_size: params.page_size } }
  )
  return normalizeAdminPageResponse(response.data)
}

export async function createRateVersion(
  contractId: number,
  variables: IdempotentMutationVariables<SupplierRateVersionCreateRequest>
): Promise<SupplyChainApiResponse<SupplierContractRateVersion>> {
  const response = await api.post(
    `${SUPPLY_CHAIN_API}/contracts/${contractId}/rates`,
    variables.data,
    { headers: idempotencyHeaders(variables.idempotencyKey) }
  )
  return response.data
}

export async function listInventoryAdjustments(
  params: SupplierContractChildListParams
): Promise<
  SupplyChainApiResponse<SupplyChainAdminPage<SupplierInventoryAdjustment>>
> {
  const response = await api.get(
    `${SUPPLY_CHAIN_API}/contracts/${params.contract_id}/inventory-adjustments`,
    { params: { p: params.p, page_size: params.page_size } }
  )
  return normalizeAdminPageResponse(response.data)
}

export async function createInventoryAdjustment(
  contractId: number,
  variables: IdempotentMutationVariables<SupplierInventoryAdjustmentCreateRequest>
): Promise<SupplyChainApiResponse<SupplierInventoryAdjustment>> {
  const response = await api.post(
    `${SUPPLY_CHAIN_API}/contracts/${contractId}/inventory-adjustments`,
    variables.data,
    { headers: idempotencyHeaders(variables.idempotencyKey) }
  )
  return response.data
}

export async function listExclusionRules(
  params: SupplierExclusionListParams
): Promise<
  SupplyChainApiResponse<SupplyChainAdminPage<SupplierStatisticsExclusionRule>>
> {
  const response = await api.get(`${SUPPLY_CHAIN_API}/exclusions`, { params })
  return normalizeAdminPageResponse(response.data)
}

export async function listEffectiveExclusions(
  params: Omit<SupplierExclusionListParams, 'current_only'>
): Promise<
  SupplyChainApiResponse<SupplyChainAdminPage<SupplierEffectiveExclusion>>
> {
  const response = await api.get(`${SUPPLY_CHAIN_API}/exclusions`, {
    params: { ...params, current_only: true },
  })
  return normalizeAdminPageResponse(response.data)
}

export async function createExclusionRule(
  variables: IdempotentMutationVariables<SupplierExclusionRuleCreateRequest>
): Promise<SupplyChainApiResponse<SupplierStatisticsExclusionRule>> {
  const response = await api.post(
    `${SUPPLY_CHAIN_API}/exclusions`,
    variables.data,
    { headers: idempotencyHeaders(variables.idempotencyKey) }
  )
  return response.data
}

export async function listChannelBindings(
  params: SupplierChannelBindingListParams
): Promise<
  SupplyChainApiResponse<SupplyChainAdminPage<SupplierChannelBinding>>
> {
  const response = await api.get(`${SUPPLY_CHAIN_API}/channel-bindings`, {
    params,
  })
  return normalizeAdminPageResponse(response.data)
}

export async function bindChannel(
  channelId: number,
  data: SupplierChannelBindingRequest
): Promise<SupplyChainApiResponse<SupplierChannelBinding>> {
  const response = await api.put(
    `${SUPPLY_CHAIN_API}/channel-bindings/${channelId}`,
    data
  )
  return response.data
}

export async function unbindChannel(
  channelId: number,
  expectedContractId: number
): Promise<SupplyChainApiResponse<SupplierChannelUnbindResult>> {
  const response = await api.delete(
    `${SUPPLY_CHAIN_API}/channel-bindings/${channelId}`,
    { params: { expected_contract_id: expectedContractId } }
  )
  return response.data
}

export async function getReportOverview(
  query: SupplierReportQuery
): Promise<SupplyChainApiResponse<SupplierReportOverview>> {
  const response = await api.get(`${SUPPLY_CHAIN_API}/reports/overview`, {
    params: buildSupplierReportQueryParams(query),
  })
  return response.data
}

export async function getReportTrend(
  query: SupplierReportQuery
): Promise<SupplyChainApiResponse<SupplierReportTrend>> {
  const response = await api.get(`${SUPPLY_CHAIN_API}/reports/trend`, {
    params: buildSupplierReportQueryParams(query),
  })
  return response.data
}

export async function listReportContracts(
  query: SupplierReportPageQuery
): Promise<SupplyChainApiResponse<SupplierReportContractList>> {
  const response = await api.get(`${SUPPLY_CHAIN_API}/reports/contracts`, {
    params: buildSupplierReportQueryParams(query),
  })
  return response.data
}

export async function getReportContract(
  contractId: number,
  query: SupplierReportPageQuery
): Promise<SupplyChainApiResponse<SupplierReportContractDetail>> {
  const response = await api.get(
    `${SUPPLY_CHAIN_API}/reports/contracts/${contractId}`,
    { params: buildSupplierReportQueryParams(query) }
  )
  return response.data
}

export async function listReportChannels(
  query: SupplierReportPageQuery
): Promise<SupplyChainApiResponse<SupplierReportChannelList>> {
  const response = await api.get(`${SUPPLY_CHAIN_API}/reports/channels`, {
    params: buildSupplierReportQueryParams(query),
  })
  return response.data
}

export async function listReportBreakdown(
  query: SupplierReportPageQuery
): Promise<SupplyChainApiResponse<SupplierReportBreakdownList>> {
  const response = await api.get(`${SUPPLY_CHAIN_API}/reports/breakdown`, {
    params: buildSupplierReportQueryParams(query),
  })
  return response.data
}

export async function getReportFreshness(): Promise<
  SupplyChainApiResponse<SupplierReportFreshness>
> {
  const response = await api.get<
    SupplyChainApiResponse<SupplierReportFreshnessWire>
  >(`${SUPPLY_CHAIN_API}/reports/freshness`)
  const freshness = response.data.data
  const coverageStart = freshness.coverage_start_at
  return {
    ...response.data,
    data: {
      ...freshness,
      sync_only: freshness.sync_only === true,
      coverage_start_at:
        typeof coverageStart === 'number' &&
        Number.isSafeInteger(coverageStart) &&
        coverageStart > 0
          ? coverageStart
          : null,
    },
  }
}
