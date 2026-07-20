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
import type { TFunction } from 'i18next'
import type { ModelAvailabilityStatus } from '@/lib/model-availability'
import type { ModelAccessModel, UserModelAccess } from '../types'
import {
  getAccountModels,
  getScopeModels,
  resolveCreateScope,
} from './model-access'

export type ModelEndpointFilter = string

export function getModelEndpointLabel(
  endpoint: ModelEndpointFilter,
  t: TFunction
): string {
  if (endpoint === 'all') return t('All')
  if (endpoint.startsWith('openai')) return t('OpenAI')
  if (endpoint === 'anthropic') return t('Anthropic')
  if (endpoint === 'gemini') return t('Gemini')
  if (endpoint === 'image-generation') return t('Image Generation')
  return endpoint
}

export function getCreateKeySearch(scopeId?: string | null): {
  open: 'create'
  group?: string
} {
  return scopeId ? { open: 'create', group: scopeId } : { open: 'create' }
}

export function isFixedModelAccessView(access: UserModelAccess): boolean {
  return access.scope_mode === 'fixed_account' || access.groups.length === 0
}

export function resolveModelAccessScope(
  access: UserModelAccess,
  currentScope?: string | null
): string | null {
  if (isFixedModelAccessView(access)) return null
  if (
    currentScope &&
    access.groups.some((scope) => scope.id === currentScope)
  ) {
    return currentScope
  }
  return resolveCreateScope(access) ?? access.groups[0]?.id ?? null
}

export function getModelAccessScopeModels(
  access: UserModelAccess,
  scopeId?: string | null
): ModelAccessModel[] {
  if (isFixedModelAccessView(access)) return getAccountModels(access)
  return getScopeModels(access, scopeId)
}

function matchesEndpoint(
  model: ModelAccessModel,
  endpoint: ModelEndpointFilter
): boolean {
  if (endpoint === 'all') return true
  if (endpoint === 'openai') {
    return model.supported_endpoint_types.some((type) =>
      type.startsWith('openai')
    )
  }
  return model.supported_endpoint_types.includes(endpoint)
}

function endpointFilterValue(endpoint: string): string {
  return endpoint.startsWith('openai') ? 'openai' : endpoint
}

export function getModelEndpointFilters(
  models: readonly ModelAccessModel[]
): ModelEndpointFilter[] {
  const endpoints = new Set<string>()
  for (const model of models) {
    for (const endpoint of model.supported_endpoint_types) {
      endpoints.add(endpointFilterValue(endpoint))
    }
  }
  return ['all', ...Array.from(endpoints).sort()]
}

export function normalizeModelAvailabilityStatus(
  status: ModelAccessModel['availability_status']
): ModelAvailabilityStatus {
  return status === 'unknown' ? 'unknown_failure' : status
}

export function filterModelAccessModels(
  models: readonly ModelAccessModel[],
  query: string,
  endpoint: ModelEndpointFilter
): ModelAccessModel[] {
  const normalizedQuery = query.trim().toLocaleLowerCase()
  return models.filter((model) => {
    if (!matchesEndpoint(model, endpoint)) return false
    if (!normalizedQuery) return true
    return [model.id, model.vendor?.name ?? ''].some((value) =>
      value.toLocaleLowerCase().includes(normalizedQuery)
    )
  })
}
