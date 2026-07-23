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
import type { UserModelAccess } from '@/features/available-models'
import type { ApiKeyGroupOption } from '../components/api-key-group-combobox'
import type { ApiResponse } from '../types'

export const MAX_BATCH_GROUP_API_KEYS = 100

export type BatchGroupPayload = {
  ids: number[]
  group: string
}

export type BatchGroupUpdateSuccessEffects = {
  resetSelection: () => void
  refresh: () => void
  clearGroup: () => void
  closeDialog: () => void
}

type BatchGroupUpdateRequest = (
  ids: number[],
  group: string
) => Promise<ApiResponse<number>>

type BatchGroupUpdateResult =
  | { success: true; count: number }
  | { success: false; message?: string }

type CoordinateBatchGroupUpdateParams = {
  request: BatchGroupUpdateRequest
  ids: number[]
  group: string
  successEffects: BatchGroupUpdateSuccessEffects
}

export function buildBatchGroupPayload(
  ids: number[],
  group: string
): BatchGroupPayload {
  if (ids.length === 0 || ids.length > MAX_BATCH_GROUP_API_KEYS) {
    throw new RangeError(
      `Batch group updates require 1-${MAX_BATCH_GROUP_API_KEYS} API keys`
    )
  }
  if (!group) {
    throw new Error('A group is required for batch group updates')
  }

  return { ids: [...ids], group }
}

export function getBatchGroupOptions(
  modelAccess: UserModelAccess | undefined
): ApiKeyGroupOption[] {
  return (
    modelAccess?.groups.map((group) => ({
      value: group.id,
      label: group.label,
      desc: group.description,
      ratio: group.ratio ?? undefined,
    })) ?? []
  )
}

export function canBatchUpdateApiKeyGroup(
  featureEnabled: boolean,
  canUseGroups: boolean,
  modelAccess: UserModelAccess | undefined
): boolean {
  return (
    featureEnabled &&
    canUseGroups &&
    modelAccess?.scope_mode === 'selectable_group' &&
    modelAccess.groups.length > 0
  )
}

export async function coordinateBatchGroupUpdate(
  params: CoordinateBatchGroupUpdateParams
): Promise<BatchGroupUpdateResult> {
  const payload = buildBatchGroupPayload(params.ids, params.group)
  const response = await params.request(payload.ids, payload.group)

  if (!response.success) {
    return { success: false, message: response.message }
  }

  params.successEffects.resetSelection()
  params.successEffects.refresh()
  params.successEffects.clearGroup()
  params.successEffects.closeDialog()

  return { success: true, count: response.data ?? payload.ids.length }
}
