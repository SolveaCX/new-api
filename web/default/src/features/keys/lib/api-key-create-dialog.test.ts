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
import type { UserModelAccess } from '@/features/available-models'
import {
  getCreateDialogDeepLinkAction,
  shouldApplyResolvedCreateGroup,
} from './api-key-create-dialog'

function buildAccess(): UserModelAccess {
  return {
    scope_mode: 'selectable_group',
    identity_scope: 'identity',
    identity_model_ids: [],
    create_default_scope: 'default',
    groups: [
      { id: 'default', label: 'Default', ratio: 1, model_ids: [] },
      { id: 'premium', label: 'Premium', ratio: 2, model_ids: [] },
    ],
    account_model_ids: [],
    models: [],
  }
}

describe('create dialog deep-link model access contract', () => {
  test('opens on an access error so the dialog can show Retry', () => {
    expect(
      getCreateDialogDeepLinkAction(true, undefined, false, 'premium')
    ).toEqual({ shouldOpen: false })
    expect(
      getCreateDialogDeepLinkAction(true, undefined, true, 'premium')
    ).toEqual({ shouldOpen: true })
  })

  test('resolves the requested group after Retry succeeds', () => {
    expect(
      getCreateDialogDeepLinkAction(true, buildAccess(), false, 'premium')
    ).toEqual({ shouldOpen: true, resolvedGroup: 'premium' })
    expect(
      getCreateDialogDeepLinkAction(true, buildAccess(), false, 'removed')
    ).toEqual({ shouldOpen: true, resolvedGroup: 'default' })
  })

  test('applies the resolved group only when the user has not selected one', () => {
    const base = {
      access: buildAccess(),
      groupDirty: false,
      initialized: true,
      isUpdate: false,
      open: true,
    }

    expect(shouldApplyResolvedCreateGroup(base)).toBeTrue()
    expect(
      shouldApplyResolvedCreateGroup({ ...base, groupDirty: true })
    ).toBeFalse()
    expect(shouldApplyResolvedCreateGroup({ ...base, open: false })).toBeFalse()
  })
})
