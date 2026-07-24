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
  MicroUsd,
  NullableRatio,
  SupplierReportMetrics,
  SupplierReportMoney,
} from '../types'

type IntegerDisplayValue = number | bigint | null | undefined
type MicroUsdDisplayValue = IntegerDisplayValue | string

function toExactInteger(value: MicroUsdDisplayValue): bigint | null {
  if (typeof value === 'bigint') return value
  if (typeof value === 'string') {
    const trimmed = value.trim()
    return /^-?\d+$/.test(trimmed) ? BigInt(trimmed) : null
  }
  if (typeof value !== 'number') return null
  if (!Number.isSafeInteger(value)) return null
  return BigInt(value)
}

function groupInteger(value: string): string {
  return value.replace(/\B(?=(\d{3})+(?!\d))/g, ',')
}

function renderScaledInteger(value: bigint, scale: number): string {
  const negative = value < 0n
  const absolute = negative ? -value : value
  const digits = absolute.toString().padStart(scale + 1, '0')
  const whole = scale === 0 ? digits : digits.slice(0, -scale)
  const rawFraction = scale === 0 ? '' : digits.slice(-scale)
  const fraction = rawFraction.replace(/0+$/, '')
  const sign = negative ? '-' : ''
  return fraction ? `${sign}${whole}.${fraction}` : `${sign}${whole}`
}

function renderDecimalShift(value: string, shift: number): string | null {
  const match = /^([+-]?)(\d+)(?:\.(\d+))?$/.exec(value.trim())
  if (!match) return null
  const sign = match[1] === '-' ? '-' : ''
  const whole = match[2]
  const fraction = match[3] ?? ''
  const coefficient = BigInt(`${whole}${fraction}`)
  const decimalPlaces = fraction.length - shift
  let rendered: string
  if (decimalPlaces <= 0) {
    rendered = (coefficient * 10n ** BigInt(-decimalPlaces)).toString()
  } else {
    rendered = renderScaledInteger(coefficient, decimalPlaces)
  }
  if (sign === '-' && rendered !== '0') return `-${rendered}`
  return rendered
}

export function formatMicroUsd(
  value: MicroUsd | bigint | null | undefined,
  unknownLabel: string,
  currencySymbol = '$'
): string {
  const integer = toExactInteger(value)
  if (integer === null) return unknownLabel
  const negative = integer < 0n
  const absolute = negative ? -integer : integer
  const whole = absolute / 1_000_000n
  const microFraction = (absolute % 1_000_000n).toString().padStart(6, '0')
  const trimmedFraction = microFraction.replace(/0+$/, '')
  const fraction = trimmedFraction.padEnd(2, '0')
  const sign = negative ? '-' : ''
  return `${sign}${currencySymbol}${groupInteger(whole.toString())}.${fraction}`
}

export function formatNullableRatioPercent(
  value: NullableRatio | undefined,
  unknownLabel: string
): string {
  if (value === null || value === undefined || value.trim() === '') {
    return unknownLabel
  }
  const percent = renderDecimalShift(value, 2)
  return percent === null ? unknownLabel : `${percent}%`
}

export function formatPpmPercent(
  value: IntegerDisplayValue,
  unknownLabel: string
): string {
  const integer = toExactInteger(value)
  return integer === null ? unknownLabel : `${renderScaledInteger(integer, 4)}%`
}

export function formatPpmDiscount(
  value: IntegerDisplayValue,
  unitLabel: string,
  unknownLabel: string
): string {
  const integer = toExactInteger(value)
  return integer === null
    ? unknownLabel
    : `${renderScaledInteger(integer, 5)}${unitLabel}`
}

export function knownMoneyValue(
  money: SupplierReportMoney,
  metrics: Pick<SupplierReportMetrics, 'request_count'>
): MicroUsd | null {
  if (metrics.request_count > 0 && money.known_count === 0) return null
  return money.micro_usd
}
