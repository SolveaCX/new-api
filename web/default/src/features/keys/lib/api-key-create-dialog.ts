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
import {
  resolveCreateScope,
  type UserModelAccess,
} from '@/features/available-models'

export type CreateDialogDeepLinkAction = {
  resolvedGroup?: string | null
  shouldOpen: boolean
}

export function getCreateDialogDeepLinkAction(
  requested: boolean,
  access: UserModelAccess | undefined,
  isError: boolean,
  requestedGroup?: string
): CreateDialogDeepLinkAction {
  if (!requested) return { shouldOpen: false }
  if (access) {
    return {
      shouldOpen: true,
      resolvedGroup: resolveCreateScope(access, requestedGroup),
    }
  }
  return { shouldOpen: isError }
}

export function shouldApplyResolvedCreateGroup(options: {
  access: UserModelAccess | undefined
  groupDirty: boolean
  initialized: boolean
  isUpdate: boolean
  open: boolean
}): boolean {
  return (
    options.open &&
    !options.isUpdate &&
    options.initialized &&
    options.access !== undefined &&
    !options.groupDirty
  )
}
