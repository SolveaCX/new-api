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
export type OAuthAuthorizeAction = 'approve' | 'deny'

export type OAuthAuthorizePayload = {
  action: OAuthAuthorizeAction
  client_id: string
  resource: string
  redirect_uri: string
  scope: string
  response_type: string
  code_challenge: string
  code_challenge_method: string
  state: string
}

export type OAuthAuthorizationDetails = {
  client_name: string
  resource: string
  scopes: string[] | string
}

export type OAuthAuthorizeResponse = {
  redirect_url: string
}

export type ConnectedApp = {
  grant_public_id: string
  client_id: string
  display_name: string
  scopes: string
  created_time: number
  last_used_at: number
  status: string
}

export type ApiEnvelope<T> = {
  success: boolean
  message?: string
  data?: T
}
