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
import type { ComputeBid, ComputeRFQ, RFQDraft, RFQFormValues } from './types'

/** Pure helpers shared by the market screens (unit-tested). */

export const GPU_MODELS = [
  'B300',
  'B200',
  'H200',
  'H100',
  'A100',
  'L40S',
  'RTX 5090',
  'RTX 4090',
  'M3 Ultra',
] as const

export const REGIONS = [
  'JP',
  'SG',
  'KR',
  'US-WEST',
  'US-EAST',
  'EU',
  'HK',
  'TW',
  'IN',
  'AU',
  'ME',
] as const

export const DELIVERY_OPTIONS = ['bare_metal', 'vm', 'slurm', 'k8s'] as const

export const COMPLIANCE_OPTIONS = [
  'SOC 2',
  'ISO 27001',
  'Tier 3+',
  'SLA 99.5%',
  'HIPAA',
  'GDPR',
  'Data residency',
] as const

export const HOURS_PER_MONTH = 730
export const HOURS_PER_YEAR = 8760

export function emptyRFQForm(): RFQFormValues {
  return {
    gpu_model: 'H100',
    gpus_per_node: 8,
    nodes: 1,
    term_months: 1,
    start_date: '',
    price_ceiling: 0,
    target_price_min: 0,
    target_price_max: 0,
    regions: [],
    delivery: 'bare_metal',
    interconnect: '',
    storage_tb: 0,
    compliance: [],
    payment_terms: 'Monthly prepaid · 1 month deposit · USD',
    notes: '',
  }
}

/** Merge an AI/heuristic draft into the form, keeping user-entered values for fields the draft did not fill. */
export function applyDraft(
  form: RFQFormValues,
  draft: RFQDraft
): RFQFormValues {
  const next = { ...form }
  if (draft.gpu_model) next.gpu_model = draft.gpu_model
  if (draft.gpus_per_node) next.gpus_per_node = draft.gpus_per_node
  if (draft.nodes) next.nodes = draft.nodes
  if (draft.term_months) next.term_months = draft.term_months
  if (draft.start_date) next.start_date = draft.start_date
  if (draft.price_ceiling) next.price_ceiling = draft.price_ceiling
  if (draft.target_price_min) next.target_price_min = draft.target_price_min
  if (draft.target_price_max) next.target_price_max = draft.target_price_max
  if (draft.regions?.length) next.regions = draft.regions
  if (draft.delivery) next.delivery = draft.delivery
  if (draft.interconnect) next.interconnect = draft.interconnect
  if (draft.storage_tb) next.storage_tb = draft.storage_tb
  if (draft.compliance?.length) next.compliance = draft.compliance
  if (draft.notes && !form.notes) next.notes = draft.notes
  return next
}

export function totalGpus(rfq: Pick<ComputeRFQ, 'gpus_per_node' | 'nodes'>) {
  return rfq.gpus_per_node * rfq.nodes
}

/** Annual contract value in USD at a given $/GPU-hour. */
export function annualValue(
  rfq: Pick<ComputeRFQ, 'gpus_per_node' | 'nodes'>,
  price: number
) {
  return totalGpus(rfq) * price * HOURS_PER_YEAR
}

export function monthlyValue(
  rfq: Pick<ComputeRFQ, 'gpus_per_node' | 'nodes'>,
  price: number
) {
  return totalGpus(rfq) * price * HOURS_PER_MONTH
}

export function formatUsd(value: number, digits = 0) {
  if (!Number.isFinite(value)) return '—'
  if (value >= 1_000_000) return `$${(value / 1_000_000).toFixed(2)}M`
  if (value >= 1_000)
    return `$${(value / 1_000).toFixed(digits === 0 ? 0 : digits)}K`
  return `$${value.toFixed(digits)}`
}

export function formatPrice(price: number) {
  return price > 0 ? `$${price.toFixed(2)}` : '—'
}

/** "6h 12m" style countdown; "0m" once expired. */
export function formatRemaining(deadline: number, now: number) {
  const secs = Math.max(0, deadline - now)
  const h = Math.floor(secs / 3600)
  const m = Math.floor((secs % 3600) / 60)
  if (h === 0 && m === 0) return '0m'
  if (h === 0) return `${m}m`
  return `${h}h ${String(m).padStart(2, '0')}m`
}

export function isOpenStatus(status: ComputeRFQ['status']) {
  return status === 'matching' || status === 'choosing'
}

/** Whether a buyer may accept this bid right now (matching: any eligible live bid; choosing: top 3 only). */
export function canAcceptBid(
  rfq: ComputeRFQ,
  bid: ComputeBid,
  top: ComputeBid[]
) {
  if (!bid.eligible || bid.status !== 'live') return false
  if (rfq.status === 'matching') return true
  if (rfq.status === 'choosing') return top.some((b) => b.id === bid.id)
  return false
}

/** Suggested next bid: $0.05 under the current lowest eligible price, or under the ceiling. */
export function suggestedBidPrice(
  rfq: ComputeRFQ,
  lowest: number,
  myCurrent?: number
) {
  const base =
    myCurrent && myCurrent > 0
      ? Math.min(myCurrent, lowest || myCurrent)
      : lowest || rfq.price_ceiling
  const price = Math.max(0.05, Math.round((base - 0.05) * 100) / 100)
  return Math.min(price, Math.round((rfq.price_ceiling - 0.05) * 100) / 100)
}

export function parseHardReqs(raw: string): Record<string, boolean> {
  try {
    const v = JSON.parse(raw || '{}')
    return v && typeof v === 'object' ? (v as Record<string, boolean>) : {}
  } catch {
    return {}
  }
}

export function splitList(value: string) {
  return value
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
}

/** Hard requirements a supplier must confirm for a given RFQ, derived from its fields. */
export function hardRequirementsFor(rfq: ComputeRFQ): string[] {
  const reqs = [
    `region:${rfq.regions}`,
    `gpu:${rfq.gpu_model} x${rfq.gpus_per_node} x${rfq.nodes}`,
    `start:${rfq.start_date}`,
  ]
  for (const c of splitList(rfq.compliance)) reqs.push(`compliance:${c}`)
  if (rfq.delivery) reqs.push(`delivery:${rfq.delivery}`)
  return reqs
}
