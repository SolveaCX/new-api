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
import { api } from '@/lib/api'
import type {
  ApiEnvelope,
  StatusAuditEvent,
  StatusComponent,
  StatusDelivery,
  StatusDiscordInput,
  StatusIncidentPublishInput,
  StatusIncidentRecord,
  StatusMaintenanceInput,
  StatusOverrideInput,
  StatusSetting,
  StatusSettingInput,
  StatusSubscriber,
  StatusSummary,
} from './types'

const requestConfig = {
  skipErrorHandler: true,
  disableDuplicate: true,
} as const

export const statusCenterQueryKeys = {
  all: ['status-center'] as const,
  summary: () => ['status-center', 'summary'] as const,
  incidents: () => ['status-center', 'incidents'] as const,
  maintenance: () => ['status-center', 'maintenance'] as const,
  subscribers: () => ['status-center', 'subscribers'] as const,
  deliveries: () => ['status-center', 'deliveries'] as const,
  settings: () => ['status-center', 'settings'] as const,
  audit: () => ['status-center', 'audit'] as const,
}

async function getData<T>(url: string): Promise<T> {
  const response = await api.get<ApiEnvelope<T>>(url, requestConfig)
  return response.data.data
}

export function getStatusSummary(): Promise<StatusSummary> {
  return getData('/api/status/summary')
}

export function getStatusIncidents(): Promise<StatusIncidentRecord[]> {
  return getData('/api/status/admin/incidents')
}

export function getStatusMaintenance(): Promise<StatusIncidentRecord[]> {
  return getData('/api/status/admin/maintenance')
}

export function getStatusSubscribers(): Promise<StatusSubscriber[]> {
  return getData('/api/status/admin/subscribers')
}

export function getStatusDeliveries(): Promise<StatusDelivery[]> {
  return getData('/api/status/admin/deliveries')
}

export function getStatusSettings(): Promise<StatusSetting[]> {
  return getData('/api/status/admin/settings')
}

export function getStatusAudit(): Promise<StatusAuditEvent[]> {
  return getData('/api/status/admin/audit')
}

export async function publishStatusIncident(
  incidentId: number,
  input: StatusIncidentPublishInput
): Promise<StatusIncidentRecord> {
  const response = await api.post<ApiEnvelope<StatusIncidentRecord>>(
    `/api/status/admin/incidents/${incidentId}/publish`,
    input,
    requestConfig
  )
  return response.data.data
}

export async function createStatusMaintenance(
  input: StatusMaintenanceInput
): Promise<StatusIncidentRecord['incident']> {
  const response = await api.post<
    ApiEnvelope<StatusIncidentRecord['incident']>
  >('/api/status/admin/maintenance', input, requestConfig)
  return response.data.data
}

export async function reconcileStatusMaintenance(
  incidentId: number,
  expectedVersion: number
): Promise<StatusIncidentRecord['incident']> {
  const response = await api.post<
    ApiEnvelope<StatusIncidentRecord['incident']>
  >(
    `/api/status/admin/maintenance/${incidentId}/reconcile`,
    { expected_version: expectedVersion },
    requestConfig
  )
  return response.data.data
}

export async function createStatusOverride(
  input: StatusOverrideInput
): Promise<StatusComponent> {
  const path =
    input.status === 'operational'
      ? '/api/status/admin/overrides/force-green'
      : '/api/status/admin/overrides'
  const response = await api.post<ApiEnvelope<StatusComponent>>(
    path,
    input,
    requestConfig
  )
  return response.data.data
}

export async function updateStatusSetting(
  key: string,
  input: StatusSettingInput
): Promise<StatusSetting> {
  const response = await api.put<ApiEnvelope<StatusSetting>>(
    `/api/status/admin/settings/${encodeURIComponent(key)}`,
    input,
    requestConfig
  )
  return response.data.data
}

export async function configureStatusDiscord(
  input: StatusDiscordInput
): Promise<StatusSetting> {
  const response = await api.put<ApiEnvelope<StatusSetting>>(
    '/api/status/admin/settings/discord',
    input,
    requestConfig
  )
  return response.data.data
}

export async function testStatusDiscord(): Promise<unknown> {
  const response = await api.post<ApiEnvelope<unknown>>(
    '/api/status/admin/discord/test',
    {},
    requestConfig
  )
  return response.data.data
}
