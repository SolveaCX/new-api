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
import { describe, expect, test } from 'bun:test'
import {
  buildAuthorizationDetailsParams,
  buildAuthorizePayload,
  buildOAuthAuthorizeRedirectTarget,
} from './lib'

describe('MCP OAuth authorize helpers', () => {
  test('preserves pathname, duplicate query values, and hash in the sign-in redirect target', () => {
    const target = buildOAuthAuthorizeRedirectTarget({
      pathname: '/oauth/authorize',
      search:
        '?client_id=flatkey-tools&scope=read&scope=write&state=s-1&resource=https%3A%2F%2Fmcp.flatkey.ai&code_challenge=abc&code_challenge_method=S256',
      hash: 'review',
    })

    expect(target).toBe(
      '/oauth/authorize?client_id=flatkey-tools&scope=read&scope=write&state=s-1&resource=https%3A%2F%2Fmcp.flatkey.ai&code_challenge=abc&code_challenge_method=S256#review'
    )
  })

  test('keeps already-prefixed hash fragments compatible', () => {
    const target = buildOAuthAuthorizeRedirectTarget({
      pathname: '/oauth/authorize',
      search: '?client_id=flatkey-tools',
      hash: '#review',
    })

    expect(target).toBe('/oauth/authorize?client_id=flatkey-tools#review')
  })

  test('forwards the exact authorize query string to authorization details', () => {
    const params = buildAuthorizationDetailsParams(
      '?client_id=flatkey-tools&scope=read&scope=write&resource=https%3A%2F%2Fmcp.flatkey.ai&state=s-1'
    )

    expect(params.getAll('scope')).toEqual(['read', 'write'])
    expect(params.toString()).toBe(
      'client_id=flatkey-tools&scope=read&scope=write&resource=https%3A%2F%2Fmcp.flatkey.ai&state=s-1'
    )
  })

  test('constructs the authorize POST body only from OAuth authorize query fields', () => {
    const payload = buildAuthorizePayload(
      '?client_id=flatkey-tools&resource=https%3A%2F%2Fmcp.flatkey.ai&redirect_uri=https%3A%2F%2Fclient.example%2Fcb&scope=read+write&response_type=code&code_challenge=abc&code_challenge_method=S256&state=s-1&ignored=secret',
      'approve'
    )

    expect(payload).toEqual({
      action: 'approve',
      client_id: 'flatkey-tools',
      resource: 'https://mcp.flatkey.ai',
      redirect_uri: 'https://client.example/cb',
      scope: 'read write',
      response_type: 'code',
      code_challenge: 'abc',
      code_challenge_method: 'S256',
      state: 's-1',
    })
    expect(payload).not.toHaveProperty('ignored')
  })
})
