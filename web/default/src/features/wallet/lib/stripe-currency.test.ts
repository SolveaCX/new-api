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
  getStripeCheckoutCurrencyForLanguage,
  normalizeStripeCheckoutCurrency,
} from './stripe-currency'

describe('normalizeStripeCheckoutCurrency', () => {
  test('normalizes explicit checkout currency search params', () => {
    expect(normalizeStripeCheckoutCurrency('usd')).toBe('USD')
    expect(normalizeStripeCheckoutCurrency(' JPY ')).toBe('JPY')
    expect(normalizeStripeCheckoutCurrency('brl')).toBe('BRL')
    expect(normalizeStripeCheckoutCurrency('eur')).toBeUndefined()
    expect(normalizeStripeCheckoutCurrency(undefined)).toBeUndefined()
  })
})

describe('getStripeCheckoutCurrencyForLanguage', () => {
  test('maps Japanese and Portuguese UI languages to local currencies', () => {
    expect(getStripeCheckoutCurrencyForLanguage('ja')).toBe('JPY')
    expect(getStripeCheckoutCurrencyForLanguage('ja-JP')).toBe('JPY')
    expect(getStripeCheckoutCurrencyForLanguage('pt')).toBe('BRL')
    expect(getStripeCheckoutCurrencyForLanguage('pt-BR')).toBe('BRL')
  })

  test('leaves other languages on the default checkout currency', () => {
    expect(getStripeCheckoutCurrencyForLanguage('en')).toBeUndefined()
    expect(getStripeCheckoutCurrencyForLanguage('zh-CN')).toBeUndefined()
    expect(getStripeCheckoutCurrencyForLanguage('')).toBeUndefined()
    expect(getStripeCheckoutCurrencyForLanguage(undefined)).toBeUndefined()
  })

  test('does not treat languages merely prefixed with ja/pt as local', () => {
    expect(getStripeCheckoutCurrencyForLanguage('jab')).toBeUndefined()
    expect(getStripeCheckoutCurrencyForLanguage('pta')).toBeUndefined()
  })
})
