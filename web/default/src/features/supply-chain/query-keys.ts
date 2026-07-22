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
import type {
  SupplierChannelBindingListParams,
  SupplierContractChildListParams,
  SupplierContractListParams,
  SupplierExclusionListParams,
  SupplierListParams,
  SupplierReportPageQuery,
  SupplierReportQuery,
} from './types'

export const supplyChainQueryKeys = {
  all: ['supply-chain'] as const,
  suppliers: {
    all: () => [...supplyChainQueryKeys.all, 'suppliers'] as const,
    list: (params: SupplierListParams) =>
      [...supplyChainQueryKeys.all, 'suppliers', 'list', params] as const,
  },
  contracts: {
    all: () => [...supplyChainQueryKeys.all, 'contracts'] as const,
    list: (params: SupplierContractListParams) =>
      [...supplyChainQueryKeys.all, 'contracts', 'list', params] as const,
    rates: (params: SupplierContractChildListParams) =>
      [...supplyChainQueryKeys.all, 'contracts', 'rates', params] as const,
    inventoryAdjustments: (params: SupplierContractChildListParams) =>
      [
        ...supplyChainQueryKeys.all,
        'contracts',
        'inventory-adjustments',
        params,
      ] as const,
  },
  exclusions: {
    all: () => [...supplyChainQueryKeys.all, 'exclusions'] as const,
    history: (params: SupplierExclusionListParams) =>
      [...supplyChainQueryKeys.all, 'exclusions', 'history', params] as const,
    effective: (params: SupplierExclusionListParams) =>
      [...supplyChainQueryKeys.all, 'exclusions', 'effective', params] as const,
  },
  channelBindings: {
    all: () => [...supplyChainQueryKeys.all, 'channel-bindings'] as const,
    list: (params: SupplierChannelBindingListParams) =>
      [
        ...supplyChainQueryKeys.all,
        'channel-bindings',
        'list',
        params,
      ] as const,
  },
  reports: {
    all: () => [...supplyChainQueryKeys.all, 'reports'] as const,
    overview: (query: SupplierReportQuery) =>
      [...supplyChainQueryKeys.all, 'reports', 'overview', query] as const,
    trend: (query: SupplierReportQuery) =>
      [...supplyChainQueryKeys.all, 'reports', 'trend', query] as const,
    contracts: (query: SupplierReportPageQuery) =>
      [...supplyChainQueryKeys.all, 'reports', 'contracts', query] as const,
    channels: (query: SupplierReportPageQuery) =>
      [...supplyChainQueryKeys.all, 'reports', 'channels', query] as const,
    breakdown: (query: SupplierReportPageQuery) =>
      [...supplyChainQueryKeys.all, 'reports', 'breakdown', query] as const,
    freshness: () =>
      [...supplyChainQueryKeys.all, 'reports', 'freshness'] as const,
  },
}
