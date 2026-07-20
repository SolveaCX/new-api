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
import type {
  ModelAccessModel,
  TokenModelAccessConfig,
  UserModelAccess,
} from '../types'

function getModelsById(
  access: UserModelAccess,
  modelIds: readonly string[]
): ModelAccessModel[] {
  const modelsById = new Map(access.models.map((model) => [model.id, model]))
  return modelIds.flatMap((id) => {
    const model = modelsById.get(id)
    return model ? [model] : []
  })
}

export function resolveCreateScope(
  access: UserModelAccess,
  requestedScope?: string | null
): string | null {
  if (access.scope_mode === 'fixed_account') return null

  const selectableScopes = new Set(access.groups.map((group) => group.id))
  if (requestedScope && selectableScopes.has(requestedScope)) {
    return requestedScope
  }
  if (
    access.create_default_scope &&
    selectableScopes.has(access.create_default_scope)
  ) {
    return access.create_default_scope
  }
  return null
}

export function getScopeModels(
  access: UserModelAccess,
  scopeId: string | null | undefined
): ModelAccessModel[] {
  if (access.scope_mode === 'fixed_account') {
    return getAccountModels(access)
  }
  if (!scopeId) {
    return getModelsById(access, access.identity_model_ids)
  }
  const scope = access.groups.find((group) => group.id === scopeId)
  return scope ? getModelsById(access, scope.model_ids) : []
}

export function getAccountModels(access: UserModelAccess): ModelAccessModel[] {
  const modelIds =
    access.scope_mode === 'fixed_account'
      ? access.account_model_ids
      : access.identity_model_ids
  return getModelsById(access, modelIds)
}

function parseAllowlist(
  modelLimits: TokenModelAccessConfig['model_limits']
): Set<string> {
  const entries = Array.isArray(modelLimits)
    ? modelLimits
    : (modelLimits ?? '').split(',')
  return new Set(entries.filter(Boolean))
}

export function getEffectiveTokenModels(
  access: UserModelAccess,
  token: TokenModelAccessConfig
): ModelAccessModel[] {
  const scopeModels =
    access.scope_mode === 'fixed_account'
      ? getAccountModels(access)
      : getScopeModels(access, token.group || null)

  if (!token.model_limits_enabled) return scopeModels

  const allowlist = parseAllowlist(token.model_limits)
  return scopeModels.filter((model) => allowlist.has(model.allowlist_match_key))
}
