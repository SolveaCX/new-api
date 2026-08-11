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
import type {
  OAuthAuthorizationDetails,
  OAuthAuthorizeAction,
  OAuthAuthorizePayload,
} from './types'

type OAuthAuthorizeLocation = {
  pathname: string
  search?: string
  hash?: string
}

const AUTHORIZE_PAYLOAD_FIELDS = [
  'client_id',
  'resource',
  'redirect_uri',
  'scope',
  'response_type',
  'code_challenge',
  'code_challenge_method',
  'state',
] as const

export function buildOAuthAuthorizeRedirectTarget(
  location: OAuthAuthorizeLocation
): string {
  return `${location.pathname}${location.search ?? ''}${location.hash ?? ''}`
}

export function buildAuthorizationDetailsParams(search: string): URLSearchParams {
  const normalizedSearch = search.startsWith('?') ? search.slice(1) : search
  return new URLSearchParams(normalizedSearch)
}

export function buildAuthorizePayload(
  search: string,
  action: OAuthAuthorizeAction
): OAuthAuthorizePayload {
  const params = buildAuthorizationDetailsParams(search)
  const payload = AUTHORIZE_PAYLOAD_FIELDS.reduce(
    (acc, field) => ({
      ...acc,
      [field]: params.get(field) ?? '',
    }),
    { action } as OAuthAuthorizePayload
  )

  return payload
}

export function normalizeScopes(
  scopes: OAuthAuthorizationDetails['scopes']
): string[] {
  if (Array.isArray(scopes)) return scopes.filter(Boolean)
  return scopes.split(/\s+/).filter(Boolean)
}

export function formatConnectedAppTimestamp(
  value: number,
  formatTimestamp: (timestamp: number) => string,
  neverLabel: string
): string {
  if (!value || value === -1) return neverLabel
  return formatTimestamp(value)
}
