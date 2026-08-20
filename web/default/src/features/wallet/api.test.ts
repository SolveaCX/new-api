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
import { afterEach, describe, expect, mock, spyOn, test } from 'bun:test'
import { api } from '@/lib/api'
import * as walletApi from './api'

type RefundableTermsApi = {
  getRefundableSubscriptionTerms?: () => Promise<unknown>
  refundSubscriptionTerm?: (termSegmentId: number) => Promise<unknown>
}

afterEach(() => {
  mock.restore()
})

describe('refundable subscription term API', () => {
  test('loads the current user refundable term segments', async () => {
    const getRefundableSubscriptionTerms = (walletApi as RefundableTermsApi)
      .getRefundableSubscriptionTerms
    expect(getRefundableSubscriptionTerms).toBeFunction()
    if (!getRefundableSubscriptionTerms) return

    const response = {
      success: true,
      data: {
        items: [],
        total_refund_money: 0,
        total_refund_quota: 0,
      },
    }
    const get = spyOn(api, 'get').mockResolvedValue({ data: response } as never)

    await expect(getRefundableSubscriptionTerms()).resolves.toEqual(response)
    expect(get).toHaveBeenCalledWith(
      '/api/subscription/self/refundable-terms',
      expect.any(Object)
    )
  })

  test('refunds one term segment through its canonical route', async () => {
    const refundSubscriptionTerm = (walletApi as RefundableTermsApi)
      .refundSubscriptionTerm
    expect(refundSubscriptionTerm).toBeFunction()
    if (!refundSubscriptionTerm) return

    const response = {
      success: true,
      data: {
        term_segment_id: 42,
        refunded_money: 3.25,
        refunded_quota: 1_625_000,
        status: 'refunded',
      },
    }
    const post = spyOn(api, 'post').mockResolvedValue({
      data: response,
    } as never)

    await expect(refundSubscriptionTerm(42)).resolves.toEqual(response)
    expect(post).toHaveBeenCalledWith(
      '/api/subscription/self/refundable-terms/42/refund',
      {},
      expect.any(Object)
    )
  })
})

describe('stripe checkout discount API', () => {
  test('posts discount mutations through the locked checkout route', async () => {
    const updateStripeCheckoutDiscount = (walletApi as {
      updateStripeCheckoutDiscount?: (request: unknown) => Promise<unknown>
    }).updateStripeCheckoutDiscount
    expect(updateStripeCheckoutDiscount).toBeFunction()
    if (!updateStripeCheckoutDiscount) return

    const response = {
      success: true,
      data: {
        client_secret: 'cs_next',
        publishable_key: 'pk_next',
        checkout_context: 'signed-context',
        checkout_revision: 2,
      },
    }
    const post = spyOn(api, 'post').mockResolvedValue({
      data: response,
    } as never)

    await expect(
      updateStripeCheckoutDiscount({
        checkout_context: 'signed-context',
        expected_revision: 1,
        request_id: 'request-1',
        action: 'apply',
        promotion_code: 'SAVE20',
      })
    ).resolves.toEqual(response)

    expect(post).toHaveBeenCalledWith(
      '/api/user/stripe/checkout/discount',
      {
        checkout_context: 'signed-context',
        expected_revision: 1,
        request_id: 'request-1',
        action: 'apply',
        promotion_code: 'SAVE20',
      },
      expect.objectContaining({
        skipBusinessError: true,
        skipErrorHandler: true,
      })
    )
  })
})
