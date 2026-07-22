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
export const SHANGHAI_TIME_ZONE = 'Asia/Shanghai'

const NATURAL_MONTH_PATTERN = /^(\d{4})-(0[1-9]|1[0-2])$/

export function isNaturalMonth(value: string): boolean {
  return NATURAL_MONTH_PATTERN.test(value)
}

export function getShanghaiNaturalMonth(date = new Date()): string {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: SHANGHAI_TIME_ZONE,
    year: 'numeric',
    month: '2-digit',
  }).formatToParts(date)
  const year = parts.find((part) => part.type === 'year')?.value
  const month = parts.find((part) => part.type === 'month')?.value
  if (!year || !month) {
    throw new Error('Unable to resolve Asia/Shanghai natural month')
  }
  return `${year}-${month}`
}

export function shiftNaturalMonth(
  month: string,
  offset: number
): string | null {
  const match = NATURAL_MONTH_PATTERN.exec(month)
  if (!match || !Number.isInteger(offset)) return null
  const year = Number(match[1])
  const monthIndex = Number(match[2]) - 1
  const shifted = year * 12 + monthIndex + offset
  const shiftedYear = Math.floor(shifted / 12)
  const shiftedMonth = ((shifted % 12) + 12) % 12
  if (shiftedYear < 0 || shiftedYear > 9999) return null
  return `${shiftedYear.toString().padStart(4, '0')}-${(shiftedMonth + 1)
    .toString()
    .padStart(2, '0')}`
}

export function formatTime(timestamp: number | null | undefined): string {
  if (!timestamp) return '—'
  return new Date(timestamp * 1000).toLocaleString()
}
