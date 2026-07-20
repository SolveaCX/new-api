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
import type { UserModelAccess } from '../types'
import {
  getAccountModels,
  getEffectiveTokenModels,
  getScopeModels,
  resolveCreateScope,
} from './model-access'

function buildSelectableAccess(
  overrides: Partial<UserModelAccess> = {}
): UserModelAccess {
  return {
    scope_mode: 'selectable_group',
    identity_scope: 'identity-only',
    identity_model_ids: ['identity-model'],
    create_default_scope: 'auto',
    groups: [
      {
        id: 'auto',
        label: 'Auto',
        ratio: null,
        model_ids: ['gpt-gizmo-*', 'scope-model'],
      },
      {
        id: 'standard',
        label: 'Standard',
        ratio: 1,
        model_ids: ['scope-model'],
      },
    ],
    account_model_ids: [],
    models: [
      {
        id: 'gpt-gizmo-*',
        allowlist_match_key: 'gpt-gizmo',
        vendor: null,
        supported_endpoint_types: ['openai'],
        availability_status: 'available',
      },
      {
        id: 'identity-model',
        allowlist_match_key: 'identity-model',
        vendor: null,
        supported_endpoint_types: [],
        availability_status: 'unknown',
      },
      {
        id: 'scope-model',
        allowlist_match_key: 'scope-model',
        vendor: null,
        supported_endpoint_types: ['anthropic'],
        availability_status: 'temporary_failure',
      },
    ],
    ...overrides,
  }
}

describe('model access scopes', () => {
  test('resolves only selectable create scopes and falls back to the API default', () => {
    const access = buildSelectableAccess()

    expect(resolveCreateScope(access, 'standard')).toBe('standard')
    expect(resolveCreateScope(access, 'identity-only')).toBe('auto')
    expect(resolveCreateScope(access, 'not-authorized')).toBe('auto')
  })

  test('returns no create scope when no selectable scope exists', () => {
    const access = buildSelectableAccess({
      groups: [],
      create_default_scope: null,
    })

    expect(resolveCreateScope(access, 'identity-only')).toBeNull()
    expect(getAccountModels(access).map((model) => model.id)).toEqual([
      'identity-model',
    ])
  })

  test('fixed accounts ignore requested groups and use account models', () => {
    const access = buildSelectableAccess({
      scope_mode: 'fixed_account',
      identity_scope: null,
      identity_model_ids: [],
      create_default_scope: null,
      groups: [],
      account_model_ids: ['scope-model'],
    })

    expect(resolveCreateScope(access, 'plg')).toBeNull()
    expect(getScopeModels(access, 'plg').map((model) => model.id)).toEqual([
      'scope-model',
    ])
    expect(
      getEffectiveTokenModels(access, {
        group: 'not-a-real-group',
        model_limits_enabled: false,
        model_limits: '',
      }).map((model) => model.id)
    ).toEqual(['scope-model'])
  })

  test('returns the exact ordinary and auto scope collections', () => {
    const access = buildSelectableAccess()

    expect(getScopeModels(access, 'standard').map((model) => model.id)).toEqual(
      ['scope-model']
    )
    expect(getScopeModels(access, 'auto').map((model) => model.id)).toEqual([
      'gpt-gizmo-*',
      'scope-model',
    ])
  })
})

describe('effective token models', () => {
  test('uses identity_model_ids for an existing token with an empty group', () => {
    const models = getEffectiveTokenModels(buildSelectableAccess(), {
      group: '',
      model_limits_enabled: false,
      model_limits: '',
    })

    expect(models.map((model) => model.id)).toEqual(['identity-model'])
  })

  test('does not substitute create_default_scope for an invalid token group', () => {
    const models = getEffectiveTokenModels(buildSelectableAccess(), {
      group: 'not-authorized',
      model_limits_enabled: false,
      model_limits: '',
    })

    expect(models).toEqual([])
  })

  test('uses server-provided allowlist_match_key without normalizing in TypeScript', () => {
    const base = buildSelectableAccess()
    const access = buildSelectableAccess({
      groups: base.groups.map((group) =>
        group.id === 'auto'
          ? {
              ...group,
              model_ids: ['gpt-gizmo-*', 'gpt-gizmo-thinking', 'scope-model'],
            }
          : group
      ),
      models: [
        ...base.models,
        {
          id: 'gpt-gizmo-thinking',
          allowlist_match_key: 'gpt-gizmo',
          vendor: null,
          supported_endpoint_types: ['openai'],
          availability_status: 'available',
        },
      ],
    })
    const models = getEffectiveTokenModels(access, {
      group: 'auto',
      model_limits_enabled: true,
      model_limits: 'gpt-gizmo',
    })

    expect(models.map((model) => model.id)).toEqual([
      'gpt-gizmo-*',
      'gpt-gizmo-thinking',
    ])
    expect(
      getEffectiveTokenModels(access, {
        group: 'auto',
        model_limits_enabled: true,
        model_limits: ' gpt-gizmo ',
      })
    ).toEqual([])
    expect(
      getEffectiveTokenModels(access, {
        group: 'auto',
        model_limits_enabled: true,
        model_limits: 'GPT-GIZMO',
      })
    ).toEqual([])
  })

  test('keeps an explicitly disabled non-empty allowlist unrestricted', () => {
    const models = getEffectiveTokenModels(buildSelectableAccess(), {
      group: 'auto',
      model_limits_enabled: false,
      model_limits: ['scope-model'],
    })

    expect(models.map((model) => model.id)).toEqual([
      'gpt-gizmo-*',
      'scope-model',
    ])
  })

  test('treats an enabled empty allowlist as zero models', () => {
    const models = getEffectiveTokenModels(buildSelectableAccess(), {
      group: 'standard',
      model_limits_enabled: true,
      model_limits: [],
    })

    expect(models).toEqual([])
  })
})
