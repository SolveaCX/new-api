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
import type { UserModelAccess, TokenModelAccessConfig } from '../types'
import { getEffectiveTokenModels } from './model-access'
import { getModelAccessScopeModels } from './model-access-browser'

type CatalogToken = TokenModelAccessConfig & {
  model_blacklist_enabled?: boolean
  model_blacklist?: string | null
}

export function getCatalogTokenModels(
  access: UserModelAccess,
  token: CatalogToken | null
) {
  const visibleModels = getModelAccessScopeModels(access, token?.group)
  if (!token) return visibleModels

  const permitted = new Set(
    getEffectiveTokenModels(access, token).map((model) => model.id)
  )
  const blocked = new Set(
    token.model_blacklist_enabled
      ? (token.model_blacklist ?? '').split(',')
      : []
  )
  return visibleModels.filter(
    (model) =>
      permitted.has(model.id) && !blocked.has(model.allowlist_match_key)
  )
}
