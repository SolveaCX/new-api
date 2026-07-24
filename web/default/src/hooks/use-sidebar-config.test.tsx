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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { describe, expect, test } from 'bun:test'
import { renderToStaticMarkup } from 'react-dom/server'
import type { NavGroup } from '@/components/layout/types'
import type { SystemStatus } from '@/features/auth/types'
import {
  filterSidebarNavGroupsForConfig,
  useSidebarConfig,
} from './use-sidebar-config'

const navGroups: NavGroup[] = [
  {
    id: 'admin',
    title: 'Admin',
    items: [
      { title: 'Ops Daily Report', url: '/ops-report' },
      { title: 'Supply Chain', url: '/supply-chain' },
    ],
  },
]

function SidebarProbe() {
  const groups = useSidebarConfig(navGroups)
  const urls = groups.flatMap((group) =>
    group.items.flatMap((item) =>
      'url' in item && item.url ? [String(item.url)] : []
    )
  )
  return <span>{urls.join(',')}</span>
}

function renderSidebar(adminConfig: string | undefined): string {
  const queryClient = new QueryClient()
  queryClient.setQueryData(['status'], {
    SidebarModulesAdmin: adminConfig,
  } as unknown as SystemStatus)
  return renderToStaticMarkup(
    <QueryClientProvider client={queryClient}>
      <SidebarProbe />
    </QueryClientProvider>
  )
}

describe('useSidebarConfig supply-chain module', () => {
  test('is visible by default for legacy admin configuration', () => {
    expect(renderSidebar(undefined)).toContain('/supply-chain')
  })

  test('can be disabled by the authoritative admin configuration', () => {
    expect(
      renderSidebar(
        JSON.stringify({ admin: { enabled: true, supply_chain: false } })
      )
    ).not.toContain('/supply-chain')
  })

  test('can be narrowed by an explicit user preference', () => {
    const filtered = filterSidebarNavGroupsForConfig(
      navGroups,
      undefined,
      JSON.stringify({ admin: { enabled: true, supply_chain: false } }),
      true
    )
    const urls = filtered.flatMap((group) =>
      group.items.flatMap((item) =>
        'url' in item && item.url ? [String(item.url)] : []
      )
    )

    expect(urls).not.toContain('/supply-chain')
  })
})
