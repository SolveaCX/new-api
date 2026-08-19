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
import type { StripeCheckoutSession } from '@stripe/stripe-js'
import type { StripeTopupSummary } from '../types'

export type StripeCheckoutSummaryLineKey =
  | 'subtotal'
  | 'discount'
  | 'tax'
  | 'surcharge'

export interface StripeCheckoutSummaryLine {
  key: StripeCheckoutSummaryLineKey
  amount: string
}

export interface StripeCheckoutViewModel {
  canConfirm: boolean
  currency: string
  email: string
  productDescription: string
  productName: string
  primaryAmount: string
  summaryLines: StripeCheckoutSummaryLine[]
  topupSummary: StripeTopupSummary | null
  totalAmount: string
}

export function buildStripeCheckoutViewModel(
  session: StripeCheckoutSession,
  topupSummary: StripeTopupSummary | null
): StripeCheckoutViewModel {
  const lineItem = session.lineItems[0]
  const summaryLines: StripeCheckoutSummaryLine[] = [
    { key: 'subtotal', amount: session.total.subtotal.amount },
  ]

  if (session.total.discount.minorUnitsAmount !== 0) {
    summaryLines.push({
      key: 'discount',
      amount: session.total.discount.amount,
    })
  }

  const tax =
    session.total.taxExclusive.minorUnitsAmount !== 0
      ? session.total.taxExclusive
      : session.total.taxInclusive
  if (tax.minorUnitsAmount !== 0) {
    summaryLines.push({ key: 'tax', amount: tax.amount })
  }

  if (session.total.surcharge.minorUnitsAmount !== 0) {
    summaryLines.push({
      key: 'surcharge',
      amount: session.total.surcharge.amount,
    })
  }

  const visibleTopupSummary =
    topupSummary?.show_amounts && topupSummary.bonus_amount > 0
      ? topupSummary
      : null

  return {
    canConfirm: session.canConfirm,
    currency: session.currency.toUpperCase(),
    email: session.email ?? '',
    productDescription: lineItem?.description ?? '',
    productName: lineItem?.name ?? 'Flatkey',
    primaryAmount: lineItem?.subtotal.amount ?? session.total.subtotal.amount,
    summaryLines,
    topupSummary: visibleTopupSummary,
    totalAmount: session.total.total.amount,
  }
}
