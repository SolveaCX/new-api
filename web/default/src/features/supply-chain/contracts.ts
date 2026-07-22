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
import type { SupplierStatus } from './types'

export const SUPPLY_CHAIN_TABS = [
  'report',
  'suppliers',
  'contracts',
  'exclusions',
  'channel-bindings',
] as const

export type SupplyChainTab = (typeof SUPPLY_CHAIN_TABS)[number]

export interface SupplyChainSearchState {
  tab: SupplyChainTab
  month: string
  page: number
  pageSize: number
  filter: string
  status?: SupplierStatus
  supplierId?: number
  contractId?: number
  channelId?: number
  userId?: number
}

export interface SupplyChainManagementProps {
  search: SupplyChainSearchState
  onSearchChange: (patch: Partial<SupplyChainSearchState>) => void
}
