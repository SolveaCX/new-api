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
import { useCallback, useEffect, useMemo, type ReactNode } from 'react'
import {
  SecureVerificationDialog,
  useSecureVerification,
} from '@/features/auth/secure-verification'
import {
  SecureMutationContext,
  type RunSecureMutation,
} from '../lib/secure-mutation-context'
import { createSecureMutationCoordinator } from '../lib/secure-mutation-coordinator'

export function SupplyChainSecureVerificationProvider(props: {
  children: ReactNode
}) {
  const {
    open,
    methods,
    state,
    startVerification,
    executeVerification,
    cancel: cancelVerification,
    reset: resetVerification,
    setCode,
    switchMethod,
  } = useSecureVerification()
  const coordinator = useMemo(
    () => createSecureMutationCoordinator(startVerification),
    [startVerification]
  )

  useEffect(
    () => () => {
      coordinator.cancel()
    },
    [coordinator]
  )

  const cancel = useCallback(() => {
    coordinator.cancel()
    cancelVerification()
  }, [cancelVerification, coordinator])

  const runSecureMutation = useCallback<RunSecureMutation>(
    (mutation) => coordinator.run(mutation),
    [coordinator]
  )

  return (
    <SecureMutationContext.Provider value={runSecureMutation}>
      {props.children}
      <SecureVerificationDialog
        open={open}
        onOpenChange={(nextOpen) => {
          if (!nextOpen) cancel()
        }}
        methods={methods}
        state={state}
        onVerify={async (method, code) => {
          try {
            await executeVerification(method, code)
          } catch (error) {
            coordinator.cancel(error)
            resetVerification()
          }
        }}
        onCancel={cancel}
        onCodeChange={setCode}
        onMethodChange={switchMethod}
      />
    </SecureMutationContext.Provider>
  )
}
