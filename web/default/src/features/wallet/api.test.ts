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

  test('returns a rejected invalid promotion-code envelope', async () => {
    const updateStripeCheckoutDiscount = (walletApi as {
      updateStripeCheckoutDiscount?: (request: unknown) => Promise<unknown>
    }).updateStripeCheckoutDiscount
    expect(updateStripeCheckoutDiscount).toBeFunction()
    if (!updateStripeCheckoutDiscount) return

    const response = {
      success: false,
      message: 'promotion_code_invalid',
    }
    spyOn(api, 'post').mockRejectedValue({
      response: { status: 400, data: response },
    } as never)

    await expect(
      updateStripeCheckoutDiscount({
        checkout_context: 'signed-context',
        expected_revision: 1,
        request_id: 'request-1',
        action: 'apply',
        promotion_code: 'BADCODE',
      })
    ).resolves.toEqual(response)
  })

  test('returns a rejected checkout conflict envelope with latest revision data', async () => {
    const updateStripeCheckoutDiscount = (walletApi as {
      updateStripeCheckoutDiscount?: (request: unknown) => Promise<unknown>
    }).updateStripeCheckoutDiscount
    expect(updateStripeCheckoutDiscount).toBeFunction()
    if (!updateStripeCheckoutDiscount) return

    const response = {
      success: false,
      message: 'checkout_revision_conflict',
      data: {
        client_secret: 'cs_latest',
        publishable_key: 'pk_latest',
        fallback_url: 'https://checkout.example.test/latest',
        checkout_context: 'ctx-latest',
        checkout_revision: 3,
        discount_state: { source: 'invitation', display_name: 'Invite' },
        topup_summary: null,
      },
    }
    spyOn(api, 'post').mockRejectedValue({
      response: { status: 409, data: response },
    } as never)

    await expect(
      updateStripeCheckoutDiscount({
        checkout_context: 'signed-context',
        expected_revision: 2,
        request_id: 'request-2',
        action: 'restore',
      })
    ).resolves.toEqual(response)
  })

  test('rejects a server-error response even when it has a message field', async () => {
    const updateStripeCheckoutDiscount = (walletApi as {
      updateStripeCheckoutDiscount?: (request: unknown) => Promise<unknown>
    }).updateStripeCheckoutDiscount
    expect(updateStripeCheckoutDiscount).toBeFunction()
    if (!updateStripeCheckoutDiscount) return

    const error = {
      response: {
        status: 500,
        data: {
          message: 'server_error',
        },
      },
    }
    spyOn(api, 'post').mockRejectedValue(error as never)

    await expect(
      updateStripeCheckoutDiscount({
        checkout_context: 'signed-context',
        expected_revision: 2,
        request_id: 'request-3',
        action: 'restore',
      })
    ).rejects.toBe(error)
  })

  test('rejects a malformed bad-request response without a failure envelope', async () => {
    const updateStripeCheckoutDiscount = (walletApi as {
      updateStripeCheckoutDiscount?: (request: unknown) => Promise<unknown>
    }).updateStripeCheckoutDiscount
    expect(updateStripeCheckoutDiscount).toBeFunction()
    if (!updateStripeCheckoutDiscount) return

    const error = {
      response: {
        status: 400,
        data: {
          message: 'promotion_code_invalid',
        },
      },
    }
    spyOn(api, 'post').mockRejectedValue(error as never)

    await expect(
      updateStripeCheckoutDiscount({
        checkout_context: 'signed-context',
        expected_revision: 2,
        request_id: 'request-4',
        action: 'apply',
        promotion_code: 'BADCODE',
      })
    ).rejects.toBe(error)
  })
})
