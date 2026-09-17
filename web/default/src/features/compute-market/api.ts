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
import type {
  BidFormValues,
  ComputeBid,
  ComputeRFQ,
  MyRFQItem,
  OpenRFQItem,
  RFQDetail,
  RFQDraft,
  RFQFormValues,
  SupplierProfileResponse,
} from './types'

/**
 * Compute Market API. Both sides of the market only ever receive aliases
 * (#B-xxxx / #S-xxxx); the backend never returns user ids or contacts.
 */

interface ApiResponse<T> {
  success: boolean
  message: string
  data: T
}

const BASE = '/api/compute/market'

async function unwrap<T>(p: Promise<{ data: ApiResponse<T> }>): Promise<T> {
  const res = await p
  if (!res.data.success) throw new Error(res.data.message || 'Request failed')
  return res.data.data
}

export function parseRFQText(text: string) {
  return unwrap<RFQDraft>(api.post(`${BASE}/rfqs/parse`, { text }))
}

export function createRFQ(values: RFQFormValues) {
  return unwrap<{ rfq: ComputeRFQ }>(api.post(`${BASE}/rfqs`, values))
}

export function listMyRFQs() {
  return unwrap<{ items: MyRFQItem[] }>(api.get(`${BASE}/rfqs`))
}

export function listOpenRFQs() {
  return unwrap<{ items: OpenRFQItem[] }>(api.get(`${BASE}/rfqs/open`))
}

export function getRFQ(id: number) {
  return unwrap<RFQDetail>(api.get(`${BASE}/rfqs/${id}`))
}

export function placeBid(rfqId: number, values: BidFormValues) {
  return unwrap<{ bid: ComputeBid; matching_deadline: number }>(
    api.post(`${BASE}/rfqs/${rfqId}/bids`, values)
  )
}

export function withdrawBid(bidId: number) {
  return unwrap<{ id: number }>(api.post(`${BASE}/bids/${bidId}/withdraw`))
}

export function acceptBid(rfqId: number, bidId: number) {
  return unwrap<{ rfq: ComputeRFQ; accepted_bid: ComputeBid }>(
    api.post(`${BASE}/rfqs/${rfqId}/accept`, { bid_id: bidId })
  )
}

export function extendRFQ(rfqId: number) {
  return unwrap<{ rfq: ComputeRFQ }>(api.post(`${BASE}/rfqs/${rfqId}/extend`))
}

export function raiseCeiling(rfqId: number, priceCeiling: number) {
  return unwrap<{ rfq: ComputeRFQ }>(
    api.post(`${BASE}/rfqs/${rfqId}/raise`, { price_ceiling: priceCeiling })
  )
}

export function cancelRFQ(rfqId: number) {
  return unwrap<{ rfq: ComputeRFQ }>(api.post(`${BASE}/rfqs/${rfqId}/cancel`))
}

export function getSupplierProfile() {
  return unwrap<SupplierProfileResponse>(api.get(`${BASE}/supplier`))
}

export function upsertSupplierProfile(values: {
  company: string
  contact: string
  regions: string[]
  inventory: string
}) {
  return unwrap<SupplierProfileResponse>(api.post(`${BASE}/supplier`, values))
}
