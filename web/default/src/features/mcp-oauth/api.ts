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
import { api } from '@/lib/api'
import { buildAuthorizationDetailsParams } from './lib'
import type {
  ApiEnvelope,
  ConnectedApp,
  OAuthAuthorizationDetails,
  OAuthAuthorizePayload,
  OAuthAuthorizeResponse,
} from './types'

export const mcpOAuthQueryKeys = {
  authorizationDetails: (search: string) => [
    'mcp-oauth',
    'authorization-details',
    search,
  ],
  connectedApps: ['mcp-oauth', 'connected-apps'] as const,
}

export async function getOAuthAuthorizationDetails(
  search: string
): Promise<ApiEnvelope<OAuthAuthorizationDetails>> {
  const res = await api.get('/api/oauth/authorization-details', {
    params: buildAuthorizationDetailsParams(search),
    skipBusinessError: true,
  })
  return res.data
}

export async function submitOAuthAuthorization(
  payload: OAuthAuthorizePayload
): Promise<ApiEnvelope<OAuthAuthorizeResponse>> {
  const res = await api.post('/api/oauth/authorize', payload, {
    skipBusinessError: true,
  })
  return res.data
}

export async function getConnectedApps(): Promise<ApiEnvelope<ConnectedApp[]>> {
  const res = await api.get('/api/user/connected-apps')
  return res.data
}

export async function revokeConnectedApp(
  grantPublicId: string
): Promise<ApiEnvelope<null>> {
  const res = await api.post(
    `/api/user/connected-apps/${encodeURIComponent(grantPublicId)}/revoke`
  )
  return res.data
}
