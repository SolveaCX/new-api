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
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { getSelf } from '@/lib/api'
import { AuthorizePage } from '@/features/mcp-oauth/authorize-page'
import { buildOAuthAuthorizeRedirectTarget } from '@/features/mcp-oauth/lib'

export const Route = createFileRoute('/oauth/authorize')({
  beforeLoad: async ({ location }) => {
    const target = buildOAuthAuthorizeRedirectTarget({
      pathname: location.pathname,
      search: location.searchStr,
      hash: location.hash,
    })
    const { auth } = useAuthStore.getState()

    if (!auth.user) {
      throw redirect({
        to: '/sign-in',
        search: { redirect: target },
      })
    }

    const res = await getSelf().catch(() => null)
    if (res?.success && res.data) {
      auth.setUser(res.data)
      return
    }

    auth.reset()
    throw redirect({
      to: '/sign-in',
      search: { redirect: target },
    })
  },
  component: AuthorizePage,
})
