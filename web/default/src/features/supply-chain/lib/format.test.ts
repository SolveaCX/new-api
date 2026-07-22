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
import { describe, expect, it } from 'bun:test'
import {
  formatMicroUsd,
  formatNullableRatioPercent,
  formatPpmDiscount,
  formatPpmPercent,
  knownMoneyValue,
} from './format'

describe('formatMicroUsd', () => {
  it('formats integer micro-USD without floating-point arithmetic', () => {
    expect(formatMicroUsd(200_000_000_000, 'unknown')).toBe('$200,000.00')
    expect(formatMicroUsd(1_234_567, 'unknown')).toBe('$1.234567')
    expect(formatMicroUsd(-650_000, 'unknown')).toBe('-$0.65')
  })

  it('uses the caller-provided unknown label for unsafe values', () => {
    expect(formatMicroUsd(null, 'unknown')).toBe('unknown')
    expect(formatMicroUsd(Number.MAX_SAFE_INTEGER + 1, 'unknown')).toBe(
      'unknown'
    )
  })
})

describe('ratio formatting', () => {
  it('formats nullable backend ratio strings without recomputing money', () => {
    expect(formatNullableRatioPercent('0.25', 'unknown')).toBe('25%')
    expect(formatNullableRatioPercent('-0.125', 'unknown')).toBe('-12.5%')
    expect(formatNullableRatioPercent(null, 'unknown')).toBe('unknown')
  })

  it('maps 650000 PPM to both 65% and 6.5 discount units', () => {
    expect(formatPpmPercent(650_000, 'unknown')).toBe('65%')
    expect(formatPpmDiscount(650_000, '折', 'unknown')).toBe('6.5折')
  })
})

describe('knownMoneyValue', () => {
  it('keeps known zero and hides money with no known requests', () => {
    expect(
      knownMoneyValue({ known_count: 1, micro_usd: 0 }, { request_count: 1 })
    ).toBe(0)
    expect(
      knownMoneyValue({ known_count: 0, micro_usd: 0 }, { request_count: 1 })
    ).toBeNull()
  })
})
