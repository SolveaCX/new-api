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
import { createContext, useContext } from 'react'

export type RunSecureMutation = <TResult>(
  mutation: () => Promise<TResult>
) => Promise<TResult>

export const SecureMutationContext = createContext<RunSecureMutation | null>(
  null
)

export function useSupplyChainSecureMutation(): RunSecureMutation {
  const runSecureMutation = useContext(SecureMutationContext)
  if (!runSecureMutation) {
    throw new Error(
      'useSupplyChainSecureMutation must be used within SupplyChainSecureVerificationProvider'
    )
  }
  return runSecureMutation
}
