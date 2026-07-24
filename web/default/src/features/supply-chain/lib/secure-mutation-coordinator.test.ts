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
import { QueryClient } from '@tanstack/react-query'
import { describe, expect, test } from 'bun:test'
import { createSupplyChainAdminMutationOptions } from '../hooks/use-supply-chain-admin'
import { createSecureMutationCoordinator } from './secure-mutation-coordinator'

function verificationRequiredError(): object {
  return {
    response: {
      status: 403,
      data: { code: 'VERIFICATION_REQUIRED' },
    },
  }
}

function createSignal(): { promise: Promise<void>; resolve: () => void } {
  let resolve = () => undefined
  const promise = new Promise<void>((signalResolve) => {
    resolve = signalResolve
  })
  return { promise, resolve }
}

describe('supply-chain secure mutation coordinator', () => {
  test('keeps the original promise pending and retries the exact mutation closure', async () => {
    const variables = {
      values: { reason: 'append inventory' },
      key: 'stable-idempotency-key',
    }
    const receivedVariables: (typeof variables)[] = []
    const verificationStarted = createSignal()
    let retryMutation: (() => Promise<unknown>) | undefined
    let attempts = 0
    let verificationStartCount = 0
    const coordinator = createSecureMutationCoordinator(async (retry) => {
      verificationStartCount += 1
      retryMutation = retry
      verificationStarted.resolve()
      return true
    })

    const resultPromise = coordinator.run(async () => {
      receivedVariables.push(variables)
      attempts += 1
      if (attempts === 1) throw verificationRequiredError()
      return { success: true }
    })
    let settled = false
    void resultPromise.then(
      () => {
        settled = true
      },
      () => {
        settled = true
      }
    )

    await verificationStarted.promise
    expect(retryMutation).toBeDefined()
    expect(verificationStartCount).toBe(1)
    expect(settled).toBe(false)

    await retryMutation?.()
    await expect(resultPromise).resolves.toEqual({ success: true })
    expect(receivedVariables).toEqual([variables, variables])
    expect(receivedVariables[1]).toBe(receivedVariables[0])
  })

  test('invalidates every configured query exactly once after the verified retry succeeds', async () => {
    const verificationStarted = createSignal()
    let retryMutation: (() => Promise<unknown>) | undefined
    let verificationStartCount = 0
    const coordinator = createSecureMutationCoordinator(async (retry) => {
      verificationStartCount += 1
      retryMutation = retry
      verificationStarted.resolve()
      return true
    })
    const invalidatedKeys: (readonly unknown[])[] = []
    const queryClient = new QueryClient()
    queryClient.invalidateQueries = async (filters) => {
      invalidatedKeys.push(filters?.queryKey ?? [])
    }
    const supplierKey = ['supply-chain', 'suppliers'] as const
    const contractKey = ['supply-chain', 'contracts'] as const
    let attempts = 0
    const mutationOptions = createSupplyChainAdminMutationOptions(
      {
        mutationFn: async (variables: { key: string }) => {
          attempts += 1
          if (attempts === 1) throw verificationRequiredError()
          return variables.key
        },
        invalidate: [supplierKey, contractKey],
      },
      queryClient,
      coordinator.run
    )

    const resultPromise = mutationOptions.mutationFn({ key: 'stable-key' })
    void resultPromise.catch(() => undefined)
    await verificationStarted.promise

    expect(verificationStartCount).toBe(1)
    await retryMutation?.()
    await resultPromise
    await mutationOptions.onSuccess()

    expect(invalidatedKeys).toEqual([supplierKey, contractKey])
  })

  test('rejects the pending mutation when verification is cancelled', async () => {
    const cancellation = new Error('cancelled by user')
    const verificationStarted = createSignal()
    const coordinator = createSecureMutationCoordinator(async () => {
      verificationStarted.resolve()
      return true
    })
    const resultPromise = coordinator.run(async () => {
      throw verificationRequiredError()
    })
    void resultPromise.catch(() => undefined)

    await verificationStarted.promise
    coordinator.cancel(cancellation)

    let rejectedWith: unknown
    try {
      await resultPromise
    } catch (error) {
      rejectedWith = error
    }
    expect(rejectedWith).toBe(cancellation)
  })

  test('rejects the pending mutation when the exact retry fails', async () => {
    const retryFailure = new Error('retry failed')
    const verificationStarted = createSignal()
    let retryMutation: (() => Promise<unknown>) | undefined
    let attempts = 0
    const coordinator = createSecureMutationCoordinator(async (retry) => {
      retryMutation = retry
      verificationStarted.resolve()
      return true
    })
    const resultPromise = coordinator.run(async () => {
      attempts += 1
      if (attempts === 1) throw verificationRequiredError()
      throw retryFailure
    })
    void resultPromise.catch(() => undefined)

    await verificationStarted.promise
    let retryRejectedWith: unknown
    try {
      await retryMutation?.()
    } catch (error) {
      retryRejectedWith = error
    }
    expect(retryRejectedWith).toBe(retryFailure)

    let resultRejectedWith: unknown
    try {
      await resultPromise
    } catch (error) {
      resultRejectedWith = error
    }
    expect(resultRejectedWith).toBe(retryFailure)
  })
})
