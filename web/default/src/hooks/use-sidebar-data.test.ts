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
import { type TFunction } from 'i18next'
import { filterSidebarGroups } from './use-sidebar-config'
import { buildSidebarData } from './use-sidebar-data'

const t = ((key: string) => key) as TFunction

describe('buildSidebarData', () => {
  test('places available models between dashboard and API keys', () => {
    const generalGroup = buildSidebarData(t).navGroups.find(
      (group) => group.id === 'general'
    )
    const urls = generalGroup?.items.flatMap((item) =>
      'url' in item && item.url ? [item.url] : []
    )

    expect(urls).toEqual([
      '/dashboard/overview',
      '/dashboard/models',
      '/available-models',
      '/keys',
      '/usage-logs/common',
      '/usage-logs/task',
    ])
  })

  test('highlights the invitation entry with the configured reward', () => {
    const personalGroup = buildSidebarData(t, 6.5).navGroups.find(
      (group) => group.id === 'personal'
    )
    const inviteItem = personalGroup?.items.find(
      (item) => 'url' in item && item.url === '/invite'
    )

    expect(inviteItem).toMatchObject({
      title: 'Invite',
      badge: '+6.5$',
      badgeVariant: 'promotion',
    })
  })

  test('shows Recall Campaigns by default', () => {
    const admin = buildSidebarData(t).navGroups.find(
      (group) => group.id === 'admin'
    )
    expect(
      admin?.items.some(
        (item) => 'url' in item && item.url === '/recall-campaigns'
      )
    ).toBe(true)
  })

  test('hides Recall Campaigns when the admin config disables it', () => {
    const groups = filterSidebarGroups(
      buildSidebarData(t).navGroups,
      JSON.stringify({ admin: { enabled: true, recall_campaigns: false } }),
      null
    )
    const admin = groups.find((group) => group.id === 'admin')
    expect(
      admin?.items.some(
        (item) => 'url' in item && item.url === '/recall-campaigns'
      )
    ).toBe(false)
  })

  test('allows the user config to narrow Recall Campaigns visibility', () => {
    const groups = filterSidebarGroups(
      buildSidebarData(t).navGroups,
      null,
      JSON.stringify({ admin: { enabled: true, recall_campaigns: false } })
    )
    const admin = groups.find((group) => group.id === 'admin')
    expect(
      admin?.items.some(
        (item) => 'url' in item && item.url === '/recall-campaigns'
      )
    ).toBe(false)
  })

  test('keeps the reward badge language independent', () => {
    const translateToChinese = ((key: string) =>
      key === 'Invite' ? '邀请' : key) as TFunction
    const personalGroup = buildSidebarData(
      translateToChinese,
      10
    ).navGroups.find((group) => group.id === 'personal')
    const inviteItem = personalGroup?.items.find(
      (item) => 'url' in item && item.url === '/invite'
    )

    expect(inviteItem).toMatchObject({
      title: '邀请',
      badge: '+10$',
    })
  })

  test('keeps the reward amount format locale independent', () => {
    const personalGroup = buildSidebarData(t, 1234.5).navGroups.find(
      (group) => group.id === 'personal'
    )
    const inviteItem = personalGroup?.items.find(
      (item) => 'url' in item && item.url === '/invite'
    )

    expect(inviteItem?.badge).toBe('+1234.5$')
  })

  test('hides the reward badge when the configured reward is zero', () => {
    const personalGroup = buildSidebarData(t, 0).navGroups.find(
      (group) => group.id === 'personal'
    )
    const inviteItem = personalGroup?.items.find(
      (item) => 'url' in item && item.url === '/invite'
    )

    expect(inviteItem?.badge).toBeUndefined()
  })

  test.each([undefined, Number.NaN, Number.POSITIVE_INFINITY, -1])(
    'hides the reward badge for invalid value %s',
    (reward) => {
      const personalGroup = buildSidebarData(t, reward).navGroups.find(
        (group) => group.id === 'personal'
      )
      const inviteItem = personalGroup?.items.find(
        (item) => 'url' in item && item.url === '/invite'
      )

      expect(inviteItem?.badge).toBeUndefined()
    }
  )
})
