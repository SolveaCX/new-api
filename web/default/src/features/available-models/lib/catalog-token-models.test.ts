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
import { expect, test } from 'bun:test'
import type { UserModelAccess } from '../types'
import { getCatalogTokenModels } from './catalog-token-models'

const access: UserModelAccess = {
  scope_mode: 'fixed_account',
  identity_scope: null,
  identity_model_ids: [],
  identity_model_ratios: {},
  identity_default_ratio: null,
  create_default_scope: null,
  groups: [],
  account_model_ids: ['healthy', 'blocked', 'failed', 'restricted'],
  account_model_ratios: {},
  account_default_ratio: 1,
  models: ['healthy', 'blocked', 'failed', 'restricted'].map((id) => ({
    id,
    allowlist_match_key: id,
    vendor: null,
    supported_endpoint_types: ['openai'],
    availability_status: id === 'failed' ? 'temporary_failure' : 'available',
  })),
}

test('quick start applies catalog health, key allowlist and blacklist together', () => {
  const models = getCatalogTokenModels(access, {
    group: 'stale',
    model_limits_enabled: true,
    model_limits: 'healthy,blocked,failed',
    model_blacklist_enabled: true,
    model_blacklist: 'blocked',
  })
  expect(models.map((model) => model.id)).toEqual(['healthy'])
})

test('before key selection quick start still excludes failed catalog models', () => {
  expect(getCatalogTokenModels(access, null).map((model) => model.id)).toEqual([
    'healthy',
    'blocked',
    'restricted',
  ])
})
