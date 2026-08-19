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
import type { StripeTopupSummary } from '../types'

export type StripeCheckoutData = {
  client_secret?: string
  publishable_key?: string
  fallback_url?: string
  pay_link?: string
  checkout_url?: string
  hosted_invoice_url?: string
  topup_summary?: StripeTopupSummary
}

export type StripeCheckoutOpening =
  | {
      kind: 'elements'
      clientSecret: string
      publishableKey: string
      fallbackUrl?: string
    }
  | {
      kind: 'hosted'
      url: string
    }

export function normalizeCheckoutUrl(
  url: string | undefined
): string | undefined {
  if (!url) return undefined

  const normalizedUrl = url.trim()
  const isAbsoluteHttpUrl = /^https?:\/\//i.test(normalizedUrl)
  const isRootRelativeUrl =
    normalizedUrl.startsWith('/') && !normalizedUrl.startsWith('//')
  if (!isAbsoluteHttpUrl && !isRootRelativeUrl) return undefined
  if (isRootRelativeUrl && typeof window === 'undefined') return undefined

  try {
    const parsedUrl = new URL(
      normalizedUrl,
      typeof window === 'undefined' ? undefined : window.location.origin
    )
    if (parsedUrl.protocol === 'http:' || parsedUrl.protocol === 'https:') {
      return parsedUrl.href
    }
  } catch (_error) {
    return undefined
  }

  return undefined
}

export function resolveStripeCheckoutOpening(
  data: StripeCheckoutData | null | undefined
): StripeCheckoutOpening | null {
  if (data?.client_secret && data.publishable_key) {
    const fallbackUrl = normalizeCheckoutUrl(data.fallback_url)
    return {
      kind: 'elements',
      clientSecret: data.client_secret,
      publishableKey: data.publishable_key,
      ...(fallbackUrl ? { fallbackUrl } : {}),
    }
  }

  const fallbackUrl =
    normalizeCheckoutUrl(data?.pay_link) ??
    normalizeCheckoutUrl(data?.checkout_url) ??
    normalizeCheckoutUrl(data?.hosted_invoice_url)
  if (fallbackUrl) return { kind: 'hosted', url: fallbackUrl }
  return null
}
