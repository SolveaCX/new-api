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
import { type NavBadgeVariant } from '../types'

export function getNavBadgeClassName(variant: NavBadgeVariant) {
  if (variant === 'promotion') {
    return 'bg-destructive text-destructive-foreground dark:bg-destructive dark:text-background group-data-[collapsible=icon]:hidden min-w-0 max-w-28 flex-1 truncate px-1 py-0 text-[10px] font-semibold tracking-tight'
  }

  return 'shrink-0 px-1 py-0 text-xs'
}

export function getNavItemTitleClassName(variant?: NavBadgeVariant) {
  return variant === 'promotion'
    ? 'min-w-0 shrink-0 truncate'
    : 'min-w-0 flex-1 truncate'
}
