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
import { filterRootNavGroupsByRole } from './use-sidebar-view'

const groups: NavGroup[] = [
  { id: 'general', title: 'General', items: [{ title: 'Home', url: '/' }] },
  {
    id: 'admin',
    title: 'Admin',
    items: [
      { title: 'Users', url: '/users' },
      { title: 'Supply Chain', url: '/supply-chain' },
    ],
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
  })
})
