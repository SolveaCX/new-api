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
import { getNavBadgeClassName, getNavItemTitleClassName } from './nav-badge'

describe('getNavBadgeClassName', () => {
  test('keeps red promotion badges compact and readable in dark mode', () => {
    const className = getNavBadgeClassName('promotion')
    const classTokens = className.split(' ')

    expect(className).toContain('bg-destructive')
    expect(className).toContain('dark:bg-destructive')
    expect(className).toContain('dark:text-background')
    expect(className).toContain('max-w-28')
    expect(className).toContain('truncate')
    expect(className).toContain('group-data-[collapsible=icon]:hidden')
    expect(classTokens).toContain('flex-1')
    expect(classTokens).not.toContain('shrink')
  })

  test('keeps the navigation title ahead of the promotion badge', () => {
    const classTokens = getNavItemTitleClassName('promotion').split(' ')

    expect(classTokens).toContain('shrink-0')
    expect(classTokens).not.toContain('flex-1')
  })
})
