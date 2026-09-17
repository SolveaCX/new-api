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
import { describe, expect, test } from 'bun:test'
import {
  annualValue,
  filterMarketplace,
  marketplaceStats,
  paginate,
  applyDraft,
  canAcceptBid,
  emptyRFQForm,
  formatRemaining,
  formatUsd,
  hardRequirementsFor,
  suggestedBidPrice,
} from './market'
import type { ComputeBid, ComputeRFQ } from './types'

const rfq: ComputeRFQ = {
  id: 2041,
  code: 'RFQ-2041',
  buyer_alias: '#B-1170',
  mine: true,
  gpu_model: 'B300',
  gpus_per_node: 8,
  nodes: 20,
  term_months: 12,
  start_date: '2026-11-01',
  price_ceiling: 5,
  regions: 'JP',
  delivery: 'bare_metal',
  interconnect: 'IB 400G',
  storage_tb: 2000,
  compliance: 'SOC 2,ISO 27001',
  payment_terms: '',
  notes: '',
  status: 'matching',
  matching_deadline: 0,
  extensions: 0,
  accepted_bid_id: 0,
  created_time: 0,
  updated_time: 0,
}

function bid(
  id: number,
  price: number,
  eligible = true,
  status: ComputeBid['status'] = 'live'
): ComputeBid {
  return {
    id,
    rfq_id: rfq.id,
    supplier_alias: `#S-${id}`,
    supplier_level: 1,
    mine: false,
    rank: 0,
    price_per_gpu_hour: price,
    deliver_date: '',
    sla_pct: 99.5,
    hard_reqs: '{}',
    deviations: '',
    eligible,
    status,
    created_time: 0,
    updated_time: 0,
  }
}

describe('compute market helpers', () => {
  test('annual value uses total GPUs x 8760h', () => {
    expect(annualValue(rfq, 4)).toBe(160 * 4 * 8760)
    expect(formatUsd(annualValue(rfq, 4))).toBe('$5.61M')
    expect(formatUsd(981_120)).toBe('$981K')
  })

  test('applyDraft fills only what the draft knows and keeps user notes', () => {
    const form = { ...emptyRFQForm(), notes: 'keep me' }
    const next = applyDraft(form, {
      gpu_model: 'B300',
      nodes: 20,
      regions: ['JP'],
      notes: 'pasted',
      source: 'heuristic',
      filled: ['gpu_model', 'nodes', 'regions'],
      missing: ['term_months'],
    })
    expect(next.gpu_model).toBe('B300')
    expect(next.nodes).toBe(20)
    expect(next.regions).toEqual(['JP'])
    expect(next.term_months).toBe(1)
    expect(next.notes).toBe('keep me')
  })

  test('buyer can accept any eligible live bid while matching, only top 3 once choosing', () => {
    const bids = [
      bid(1, 3.6, false),
      bid(2, 3.85),
      bid(3, 4.2),
      bid(4, 4.45),
      bid(5, 4.7),
    ]
    const top = bids.filter((b) => b.eligible).slice(0, 3)
    expect(canAcceptBid(rfq, bids[0], top)).toBe(false)
    expect(canAcceptBid(rfq, bids[4], top)).toBe(true)
    const choosing = { ...rfq, status: 'choosing' as const }
    expect(canAcceptBid(choosing, bids[4], top)).toBe(false)
    expect(canAcceptBid(choosing, bids[2], top)).toBe(true)
    expect(canAcceptBid(rfq, bid(9, 4, true, 'rejected'), top)).toBe(false)
  })

  test('suggested bid undercuts the lowest by 0.05 and never exceeds the ceiling', () => {
    expect(suggestedBidPrice(rfq, 3.85)).toBe(3.8)
    expect(suggestedBidPrice(rfq, 0)).toBe(4.95)
    expect(suggestedBidPrice(rfq, 4.2, 3.9)).toBe(3.85)
  })

  test('countdown formatting', () => {
    expect(formatRemaining(1000 + 6 * 3600 + 12 * 60, 1000)).toBe('6h 12m')
    expect(formatRemaining(1000, 2000)).toBe('0m')
    expect(formatRemaining(1000 + 300, 1000)).toBe('5m')
  })

  test('hard requirements derive from the RFQ', () => {
    const reqs = hardRequirementsFor(rfq)
    expect(reqs).toContain('region:JP')
    expect(reqs).toContain('compliance:SOC 2')
    expect(reqs).toContain('delivery:bare_metal')
  })

  test('marketplace stats, filters and pagination', () => {
    const now = 1_000_000
    const items = [
      {
        rfq: {
          ...rfq,
          id: 1,
          status: 'matching' as const,
          matching_deadline: now + 600,
        },
        bid_count: 2,
        lowest_price: 3.8,
        my_bid: null,
      },
      {
        rfq: {
          ...rfq,
          id: 2,
          status: 'matching' as const,
          matching_deadline: now + 7200,
          gpu_model: 'H100',
        },
        bid_count: 0,
        lowest_price: 0,
        my_bid: bid(9, 3),
      },
      {
        rfq: { ...rfq, id: 3, status: 'matched' as const },
        bid_count: 4,
        lowest_price: 4.1,
        my_bid: null,
      },
    ]
    const stats = marketplaceStats(items, now)
    expect(stats.matching).toBe(2)
    expect(stats.endingSoon).toBe(1)
    expect(stats.bids).toBe(2)
    expect(stats.matchedRecently).toBe(1)
    expect(stats.openValue).toBe(annualValue(rfq, 3.8) + annualValue(rfq, 5))
    expect(
      filterMarketplace(items, {
        gpu: 'H100',
        status: 'all',
        query: '',
        now,
      }).map((i) => i.rfq.id)
    ).toEqual([2])
    expect(
      filterMarketplace(items, {
        gpu: 'all',
        status: 'ending',
        query: '',
        now,
      }).map((i) => i.rfq.id)
    ).toEqual([1])
    expect(
      filterMarketplace(items, {
        gpu: 'all',
        status: 'mine',
        query: '',
        now,
      }).map((i) => i.rfq.id)
    ).toEqual([1, 2, 3])
    expect(
      filterMarketplace(
        items.map((i) => ({ ...i, rfq: { ...i.rfq, mine: false } })),
        { gpu: 'all', status: 'mine', query: '', now }
      ).map((i) => i.rfq.id)
    ).toEqual([2])
    expect(
      filterMarketplace(items, { gpu: 'all', status: 'all', query: 'jp', now })
        .length
    ).toBe(3)
    const pg = paginate([1, 2, 3, 4, 5], 9, 2)
    expect(pg).toEqual({ pages: 3, current: 3, slice: [5] })
  })
})
