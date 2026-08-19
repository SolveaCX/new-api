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
import { describe, expect, mock, test } from 'bun:test'
import { recoverStripeCheckoutMountFailure } from '../../lib/stripe-checkout-recovery'

describe('recoverStripeCheckoutMountFailure', () => {
  test('uses an explicitly supplied safe hosted fallback before closing', () => {
    const navigate = mock(() => undefined)
    const notifyFailure = mock(() => undefined)
    const close = mock(() => undefined)

    expect(
      recoverStripeCheckoutMountFailure({
        fallbackUrl: 'https://checkout.stripe.test/fallback',
        navigate,
        notifyFailure,
        close,
      })
    ).toBe('hosted')
    expect(navigate).toHaveBeenCalledWith(
      'https://checkout.stripe.test/fallback'
    )
    expect(notifyFailure).not.toHaveBeenCalled()
    expect(close).not.toHaveBeenCalled()
  })

  test('fails closed when no safe hosted fallback was supplied', () => {
    const navigate = mock(() => undefined)
    const notifyFailure = mock(() => undefined)
    const close = mock(() => undefined)

    expect(
      recoverStripeCheckoutMountFailure({
        navigate,
        notifyFailure,
        close,
      })
    ).toBe('closed')
    expect(navigate).not.toHaveBeenCalled()
    expect(notifyFailure).toHaveBeenCalledTimes(1)
    expect(close).toHaveBeenCalledTimes(1)
  })
})
