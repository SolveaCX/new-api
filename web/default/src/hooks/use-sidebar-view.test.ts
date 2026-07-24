/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { describe, expect, test } from 'bun:test'
import { ROLE } from '@/lib/roles'
import type { NavGroup } from '@/components/layout/types'
import { buildSidebarData } from './use-sidebar-data'
import { filterRootNavGroupsByRole } from './use-sidebar-view'

const t = ((key: string) => key) as import('i18next').TFunction
const supplyChainItem = buildSidebarData(t)
  .navGroups.find((group) => group.id === 'admin')
  ?.items.find((item) => 'url' in item && item.url === '/supply-chain')

if (!supplyChainItem) throw new Error('Supply-chain sidebar item is missing')

const groups: NavGroup[] = [
  {
    id: 'general',
    title: 'General',
    items: [
      { title: 'Home', url: '/' },
      {
        title: 'Guest minimum',
        url: '/guest-minimum',
        minimumRole: ROLE.GUEST,
      },
    ],
  },
  {
    id: 'admin',
    title: 'Admin',
    items: [{ title: 'Users', url: '/users' }, supplyChainItem],
  },
]

function urls(role: number): string[] {
  return filterRootNavGroupsByRole(groups, role).flatMap((group) =>
    group.items.flatMap((item) =>
      'url' in item && item.url ? [String(item.url)] : []
    )
  )
}

describe('root sidebar role filtering', () => {
  test('shows supply chain only to Root without hiding other Admin entries', () => {
    expect(urls(ROLE.SUPER_ADMIN)).toContain('/supply-chain')
    expect(urls(ROLE.ADMIN)).not.toContain('/supply-chain')
    expect(urls(ROLE.ADMIN)).toContain('/users')
    expect(urls(ROLE.USER)).not.toContain('/users')
    expect(urls(ROLE.USER)).toContain('/')
    expect(urls(ROLE.GUEST)).toContain('/guest-minimum')
  })
})
