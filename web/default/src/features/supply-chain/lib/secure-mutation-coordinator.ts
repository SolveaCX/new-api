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
import { isVerificationRequiredError } from '@/lib/secure-verification'

type RetryMutation = () => Promise<unknown>
type StartVerification = (retryMutation: RetryMutation) => Promise<boolean>

export interface SecureMutationCoordinator {
  run<TResult>(mutation: () => Promise<TResult>): Promise<TResult>
  cancel(error?: unknown): void
}

export function createSecureMutationCoordinator(
  startVerification: StartVerification
): SecureMutationCoordinator {
  let rejectPending: ((error: unknown) => void) | null = null

  function cancel(
    error: unknown = new Error('Secure verification was cancelled')
  ): void {
    rejectPending?.(error)
  }

  async function run<TResult>(
    mutation: () => Promise<TResult>
  ): Promise<TResult> {
    try {
      return await mutation()
    } catch (error) {
      if (!isVerificationRequiredError(error)) throw error
      if (rejectPending) {
        throw new Error('Secure verification is already in progress', {
          cause: error,
        })
      }

      return await new Promise<TResult>((resolve, reject) => {
        let settled = false

        const settleReject = (settlementError: unknown): void => {
          if (settled) return
          settled = true
          rejectPending = null
          reject(settlementError)
        }
        rejectPending = settleReject

        const retryMutation = async (): Promise<TResult> => {
          try {
            const result = await mutation()
            if (!settled) {
              settled = true
              rejectPending = null
              resolve(result)
            }
            return result
          } catch (retryError) {
            settleReject(retryError)
            throw retryError
          }
        }

        void startVerification(retryMutation)
          .then((started) => {
            if (!started) {
              settleReject(
                new Error('Secure verification could not be started')
              )
            }
          })
          .catch(settleReject)
      })
    }
  }

  return { run, cancel }
}
