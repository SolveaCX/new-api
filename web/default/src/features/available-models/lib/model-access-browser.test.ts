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
  filterModelAccessModels,
  getCreateKeySearch,
  getModelEndpointFilters,
  getModelAccessScopeModels,
  isFixedModelAccessView,
  normalizeModelAvailabilityStatus,
  resolveModelAccessScope,
} from './model-access-browser'

function buildAccess(
  overrides: Partial<UserModelAccess> = {}
): UserModelAccess {
  return {
    scope_mode: 'selectable_group',
    identity_scope: 'identity',
    identity_model_ids: ['identity-model'],
    create_default_scope: 'auto',
    groups: [
      {
        id: 'auto',
        label: 'Auto',
        ratio: null,
        model_ids: ['gpt-main', 'image-main'],
      },
      {
        id: 'standard',
        label: 'Standard',
        ratio: 1,
        model_ids: ['gpt-main'],
      },
    ],
    account_model_ids: [],
    models: [
      {
        id: 'gpt-main',
        allowlist_match_key: 'gpt-main',
        vendor: { id: 1, name: 'OpenAI' },
        supported_endpoint_types: ['openai', 'anthropic'],
        availability_status: 'temporary_failure',
      },
      {
        id: 'identity-model',
        allowlist_match_key: 'identity-model',
        vendor: null,
        supported_endpoint_types: ['gemini'],
        availability_status: 'unknown',
      },
      {
        id: 'image-main',
        allowlist_match_key: 'image-main',
        vendor: { id: 2, name: 'Example Labs' },
        supported_endpoint_types: ['image-generation'],
        availability_status: 'available',
      },
    ],
    ...overrides,
  }
}

describe('available models browser scope selection', () => {
  test('builds exact selectable and fixed-account create-key deep links', () => {
    expect(getCreateKeySearch('standard')).toEqual({
      open: 'create',
      group: 'standard',
    })
    expect(getCreateKeySearch(null)).toEqual({ open: 'create' })
  })

  test('prefers a valid current scope, then the create default', () => {
    const access = buildAccess()

    expect(resolveModelAccessScope(access, 'standard')).toBe('standard')
    expect(resolveModelAccessScope(access, 'removed')).toBe('auto')
  })

  test('uses identity models without exposing a selector when groups are empty', () => {
    const access = buildAccess({ groups: [], create_default_scope: null })

    expect(isFixedModelAccessView(access)).toBe(true)
    expect(resolveModelAccessScope(access, 'identity')).toBeNull()
    expect(getModelAccessScopeModels(access).map((model) => model.id)).toEqual([
      'identity-model',
    ])
  })

  test('uses account models for fixed accounts regardless of group state', () => {
    const access = buildAccess({
      scope_mode: 'fixed_account',
      identity_scope: null,
      identity_model_ids: [],
      create_default_scope: null,
      groups: [],
      account_model_ids: ['gpt-main'],
    })

    expect(
      getModelAccessScopeModels(access, 'plg').map((model) => model.id)
    ).toEqual(['gpt-main'])
  })
})

describe('available models browser filters', () => {
  const models = buildAccess().models

  test('matches model IDs and public vendor names case-insensitively', () => {
    expect(
      filterModelAccessModels(models, 'GPT-', 'all').map((model) => model.id)
    ).toEqual(['gpt-main'])
    expect(
      filterModelAccessModels(models, 'example labs', 'all').map(
        (model) => model.id
      )
    ).toEqual(['image-main'])
  })

  test('groups OpenAI-compatible endpoint variants under OpenAI', () => {
    const withResponses = [
      ...models,
      {
        ...models[0],
        id: 'responses-main',
        supported_endpoint_types: ['openai-response'],
      },
    ]

    expect(
      filterModelAccessModels(withResponses, '', 'openai').map(
        (model) => model.id
      )
    ).toEqual(['gpt-main', 'responses-main'])
    expect(getModelEndpointFilters(withResponses)).toEqual([
      'all',
      'anthropic',
      'gemini',
      'image-generation',
      'openai',
    ])
  })

  test('keeps unknown endpoint filters instead of dropping new endpoint types', () => {
    const withUnknown = [
      ...models,
      {
        ...models[0],
        id: 'rerank-main',
        supported_endpoint_types: ['jina-rerank'],
      },
    ]

    expect(getModelEndpointFilters(withUnknown)).toContain('jina-rerank')
    expect(
      filterModelAccessModels(withUnknown, '', 'jina-rerank').map(
        (model) => model.id
      )
    ).toEqual(['rerank-main'])
  })

  test('keeps an all-only filter for models without endpoint metadata', () => {
    expect(
      getModelEndpointFilters([{ ...models[0], supported_endpoint_types: [] }])
    ).toEqual(['all'])
  })

  test('keeps temporary failures in the catalog while filtering endpoints', () => {
    const visible = filterModelAccessModels(models, '', 'anthropic')

    expect(visible).toHaveLength(1)
    expect(visible[0]).toMatchObject({
      id: 'gpt-main',
      availability_status: 'temporary_failure',
    })
  })

  test('maps unknown availability to the neutral unknown-failure config', () => {
    expect(normalizeModelAvailabilityStatus('unknown')).toBe('unknown_failure')
    expect(normalizeModelAvailabilityStatus('temporary_failure')).toBe(
      'temporary_failure'
    )
  })
})
