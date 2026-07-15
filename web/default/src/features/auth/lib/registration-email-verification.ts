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

export function getRegistrationEmailToken(hash: string): string | null {
  const fragment = hash.startsWith('#') ? hash.slice(1) : hash
  const token = new URLSearchParams(fragment).get('token')?.trim()
  return token || null
}

declare global {
  interface Window {
    __consumeRegistrationEmailVerificationToken?: () => string | null
  }
}

let bootstrappedRegistrationEmailToken: string | null | undefined

export function consumeRegistrationEmailVerificationToken(): string | null {
  if (bootstrappedRegistrationEmailToken !== undefined) {
    return bootstrappedRegistrationEmailToken
  }
  if (typeof window === 'undefined') return null

  bootstrappedRegistrationEmailToken =
    window.__consumeRegistrationEmailVerificationToken?.() ?? null
  return bootstrappedRegistrationEmailToken
}

export type EmailVerificationScreenState =
  | 'verifying'
  | 'verified'
  | 'unavailable'

type ResolveRegistrationEmailVerificationDependencies = {
  exchangeToken: (token: string) => Promise<unknown>
}

export async function resolveRegistrationEmailVerification(
  token: string | null,
  dependencies: ResolveRegistrationEmailVerificationDependencies
): Promise<EmailVerificationScreenState> {
  if (!token) return 'unavailable'

  try {
    const response = await dependencies.exchangeToken(token)
    return isRegistrationEmailVerified(response) ? 'verified' : 'unavailable'
  } catch {
    return 'unavailable'
  }
}

export function startRegistrationEmailVerificationEffect(
  token: string | null,
  dependencies: ResolveRegistrationEmailVerificationDependencies,
  onResolved: (state: EmailVerificationScreenState) => void
): () => void {
  let active = true
  void resolveRegistrationEmailVerification(token, dependencies).then(
    (nextState) => {
      if (active) onResolved(nextState)
    }
  )
  return () => {
    active = false
  }
}

export function isRegistrationEmailVerified(response: unknown): boolean {
  if (!response || typeof response !== 'object') return false

  const envelope = response as Record<string, unknown>
  if (envelope.success !== true) return false
  if (!envelope.data || typeof envelope.data !== 'object') return false

  const data = envelope.data as Record<string, unknown>
  return data.verified === true
}
