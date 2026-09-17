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
export type RFQStatus =
  | 'matching'
  | 'choosing'
  | 'matched'
  | 'contracted'
  | 'live'
  | 'completed'
  | 'cancelled'

export type BidStatus = 'live' | 'accepted' | 'rejected' | 'withdrawn'

export interface ComputeRFQ {
  id: number
  code: string
  buyer_alias: string
  mine: boolean
  gpu_model: string
  gpus_per_node: number
  nodes: number
  term_months: number
  start_date: string
  price_ceiling: number
  regions: string
  delivery: string
  interconnect: string
  storage_tb: number
  compliance: string
  payment_terms: string
  notes: string
  status: RFQStatus
  matching_deadline: number
  extensions: number
  accepted_bid_id: number
  created_time: number
  updated_time: number
}

export interface ComputeBid {
  id: number
  rfq_id: number
  supplier_alias: string
  supplier_level: number
  mine: boolean
  rank: number
  price_per_gpu_hour: number
  deliver_date: string
  sla_pct: number
  hard_reqs: string
  deviations: string
  eligible: boolean
  status: BidStatus
  created_time: number
  updated_time: number
}

export interface ComputeSupplier {
  id: number
  user_id: number
  company: string
  contact: string
  regions: string
  inventory: string
  level: number
  rating: number
  completed_contracts: number
  created_time: number
  updated_time: number
}

export interface RFQDraft {
  gpu_model?: string
  gpus_per_node?: number
  nodes?: number
  term_months?: number
  start_date?: string
  price_ceiling?: number
  target_price_min?: number
  target_price_max?: number
  regions?: string[]
  delivery?: string
  interconnect?: string
  storage_tb?: number
  compliance?: string[]
  notes?: string
  source: 'ai' | 'heuristic'
  filled: string[]
  missing: string[]
}

export interface RFQFormValues {
  gpu_model: string
  gpus_per_node: number
  nodes: number
  term_months: number
  start_date: string
  price_ceiling: number
  target_price_min: number
  target_price_max: number
  regions: string[]
  delivery: string
  interconnect: string
  storage_tb: number
  compliance: string[]
  payment_terms: string
  notes: string
}

export interface MyRFQItem {
  rfq: ComputeRFQ
  bid_count: number
  eligible_count: number
  lowest_price: number
  accepted_bid: ComputeBid | null
}

export interface OpenRFQItem {
  rfq: ComputeRFQ
  bid_count: number
  lowest_price: number
  my_bid: ComputeBid | null
  accepted_bid: ComputeBid | null
}

export interface RFQDetail {
  rfq: ComputeRFQ
  bids: ComputeBid[]
  top: ComputeBid[]
  lowest_price: number
  is_owner: boolean
  accepted_bid: ComputeBid | null
  target_price_min?: number
  target_price_max?: number
  now: number
}

export interface SupplierProfileResponse {
  supplier: ComputeSupplier | null
  alias: string
  can_bid: boolean
  bids?: ComputeBid[]
}

export interface BidFormValues {
  price_per_gpu_hour: number
  deliver_date: string
  sla_pct: number
  hard_reqs: Record<string, boolean>
  deviations: string
}
