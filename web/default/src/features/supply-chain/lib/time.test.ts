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
  formatTime,
  getShanghaiNaturalMonth,
  isNaturalMonth,
  shiftNaturalMonth,
} from './time'

describe('Asia/Shanghai natural month', () => {
  it('uses Shanghai rather than the host timezone at a UTC boundary', () => {
    expect(getShanghaiNaturalMonth(new Date('2026-01-31T16:00:00Z'))).toBe(
      '2026-02'
    )
  })

  it('validates and shifts calendar months across years', () => {
    expect(isNaturalMonth('2026-01')).toBe(true)
    expect(isNaturalMonth('2026-13')).toBe(false)
    expect(shiftNaturalMonth('2026-01', -1)).toBe('2025-12')
    expect(shiftNaturalMonth('2026-12', 1)).toBe('2027-01')
    expect(shiftNaturalMonth('invalid', 1)).toBeNull()
  })
})

describe('Unix timestamp formatting', () => {
  it('formats seconds and keeps unavailable timestamps explicit', () => {
    expect(formatTime(null)).toBe('—')
    expect(formatTime(1_789_488_000)).toBe(
      new Date(1_789_488_000 * 1000).toLocaleString()
    )
  })
})
