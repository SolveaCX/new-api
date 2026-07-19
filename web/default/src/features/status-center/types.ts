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
export type StatusValue =
  | 'operational'
  | 'degraded'
  | 'outage'
  | 'unknown'
  | 'maintenance'

export type StatusCenterRole = 'user' | 'admin' | 'root'

export interface ApiEnvelope<T> {
  success: boolean
  message?: string
  data: T
}

export interface StatusComponent {
  id: number
  slug: string
  kind: string
  display_name: string
  capability?: string
  lifecycle: string
  status: StatusValue
  last_trustworthy_update_at: number
  coverage: number
  version?: number
  override_status?: StatusValue
  override_reason?: string
  override_expires_at?: number
}

export interface StatusAdminComponent extends Omit<StatusComponent, 'version'> {
  version: number
}

export interface StatusSummary {
  generated_at: number
  last_trustworthy_update_at: number
  coverage: number
  status: StatusValue | 'monitoring_incomplete'
  message?: string
  components: StatusComponent[]
}

export interface StatusIncident {
  id: number
  public_id: string
  kind: 'incident' | 'maintenance'
  title: string
  impact: string
  status: string
  visibility: string
  automation_mode: string
  scheduled_start_at?: number
  scheduled_end_at?: number
  started_at?: number
  resolved_at?: number
  version: number
  created_by?: number
  created_at: number
  updated_at: number
}

export interface StatusIncidentUpdate {
  id: number
  incident_id: number
  event_id: string
  state: string
  body: string
  published: boolean
  published_at: number
  actor_id?: number
  created_at: number
}

export interface StatusIncidentRecord {
  incident: StatusIncident
  updates: StatusIncidentUpdate[]
  component_ids: number[]
}

export interface StatusSubscriber {
  id: number
  kind: string
  display_address?: string
  status: string
  failure_count: number
  suspended_at?: number
  created_at: number
  updated_at: number
}

export interface StatusDelivery {
  id: number
  published_update_id: number
  destination_type: string
  destination_id: number
  event_id: string
  status: string
  locked_until?: number
  attempts: number
  next_attempt_at: number
  last_error?: string
  version: number
  created_at: number
  updated_at: number
}

export interface StatusSetting {
  key: string
  value?: string
  sensitive: boolean
  configured: boolean
  version: number
  updated_by: number
  updated_at: number
}

export interface StatusAuditEvent {
  id: number
  actor_id: number
  actor_type: string
  action: string
  object_type: string
  object_id: string
  reason?: string
  created_at: number
}

export interface StatusOverrideInput {
  component_id: number
  expected_version: number
  status: StatusValue
  reason: string
  expires_at: number
}

export interface StatusDeliveryRetryInput {
  expected_version: number
  reason: string
}

export interface StatusIncidentPublishInput {
  expected_version: number
  state: 'investigating' | 'identified' | 'monitoring' | 'resolved'
  body: string
  event_id: string
  reason: string
  destinations: Array<{ type: string; id: number }>
}

export interface IncidentPublicationFormState {
  sourceIncidentId: number
  sourceDraftId: number
  state: StatusIncidentPublishInput['state']
  body: string
  dirty: boolean
}

export interface StatusMaintenanceInput {
  title: string
  body: string
  impact: string
  idempotency_key: string
  component_ids: number[]
  scheduled_start_at: number
  scheduled_end_at: number
  reason: string
}

export interface StatusSettingInput {
  value: string
  expected_version: number
}

export interface StatusDiscordInput {
  endpoint: string
  expected_version: number
}

export interface OverrideValidationInput {
  status: StatusValue
  reason: string
  expiresAt: number
  now: number
  role: StatusCenterRole
  secureVerified: boolean
}

export type StatusCenterValidationKey =
  | 'statusCenter.validation.reasonRequired'
  | 'statusCenter.validation.futureExpiryRequired'
  | 'statusCenter.validation.rootRequired'
  | 'statusCenter.validation.secureVerificationRequired'
  | 'statusCenter.validation.forceGreenOneHour'

export interface PublishedUpdateRow {
  id: number
  state: string
  body: string
  publishedAt: number
  canEdit: false
  correctionMode: 'append'
}

export interface StatusMutationErrorResolution {
  conflict: boolean
  messageKey:
    | 'statusCenter.conflict.reload'
    | 'statusCenter.error.requestFailed'
}

export interface StatusCenterPermissions {
  canView: boolean
  canPublishIncidents: boolean
  canManageMaintenance: boolean
  canCreateNonGreenOverride: boolean
  canViewRootControls: boolean
  canUseSensitiveRootControls: boolean
  requiresSecureVerification: boolean
}

export type StatusTranslationKey =
  | 'statusCenter.componentKind.router'
  | 'statusCenter.componentKind.model'
  | 'statusCenter.componentKind.custom'
  | 'statusCenter.audit.action.deliveryRetry'
  | 'statusCenter.audit.action.incidentDraftAuto'
  | 'statusCenter.audit.action.incidentPublish'
  | 'statusCenter.audit.action.maintenanceCreate'
  | 'statusCenter.audit.action.maintenanceStart'
  | 'statusCenter.audit.action.maintenanceEnd'
  | 'statusCenter.audit.action.maintenanceComponentStart'
  | 'statusCenter.audit.action.maintenanceComponentEnd'
  | 'statusCenter.audit.action.overrideSet'
  | 'statusCenter.audit.action.overrideExpire'
  | 'statusCenter.audit.action.custom'
  | 'statusCenter.audit.objectType.delivery'
  | 'statusCenter.audit.objectType.incident'
  | 'statusCenter.audit.objectType.maintenance'
  | 'statusCenter.audit.objectType.component'
  | 'statusCenter.audit.objectType.custom'
  | 'statusCenter.settings.label.discordWebhookEndpoint'
  | 'statusCenter.settings.label.discordDeliveryState'
  | 'statusCenter.settings.label.evidenceMaxAgeSeconds'
  | 'statusCenter.settings.label.custom'

export interface StatusTranslationLabel {
  key: StatusTranslationKey
  values?: { identifier: string }
}

const componentKindLabelKeys = {
  router: 'statusCenter.componentKind.router',
  model: 'statusCenter.componentKind.model',
} as const satisfies Record<string, StatusTranslationKey>

const auditActionLabelKeys = {
  'status.delivery.retry': 'statusCenter.audit.action.deliveryRetry',
  'status.incident.draft.auto': 'statusCenter.audit.action.incidentDraftAuto',
  'status.incident.publish': 'statusCenter.audit.action.incidentPublish',
  'status.maintenance.create': 'statusCenter.audit.action.maintenanceCreate',
  'status.maintenance.start': 'statusCenter.audit.action.maintenanceStart',
  'status.maintenance.end': 'statusCenter.audit.action.maintenanceEnd',
  'status.maintenance.component.start':
    'statusCenter.audit.action.maintenanceComponentStart',
  'status.maintenance.component.end':
    'statusCenter.audit.action.maintenanceComponentEnd',
  'status.override.set': 'statusCenter.audit.action.overrideSet',
  'status.override.expire': 'statusCenter.audit.action.overrideExpire',
} as const satisfies Record<string, StatusTranslationKey>

const auditObjectTypeLabelKeys = {
  delivery: 'statusCenter.audit.objectType.delivery',
  incident: 'statusCenter.audit.objectType.incident',
  maintenance: 'statusCenter.audit.objectType.maintenance',
  component: 'statusCenter.audit.objectType.component',
} as const satisfies Record<string, StatusTranslationKey>

const settingLabelKeys = {
  'status.discord.webhook_endpoint':
    'statusCenter.settings.label.discordWebhookEndpoint',
  'status.discord.delivery_state':
    'statusCenter.settings.label.discordDeliveryState',
  'status.evidence_max_age_seconds':
    'statusCenter.settings.label.evidenceMaxAgeSeconds',
} as const satisfies Record<string, StatusTranslationKey>

function getKnownTranslationLabel(
  identifier: string,
  labels: Record<string, StatusTranslationKey>,
  fallbackKey: StatusTranslationKey,
  preserveIdentifier: boolean
): StatusTranslationLabel {
  const key = labels[identifier]
  if (key) return { key }
  if (preserveIdentifier) {
    return { key: fallbackKey, values: { identifier } }
  }
  return { key: fallbackKey }
}

export function getStatusComponentKindLabel(
  kind: string
): StatusTranslationLabel {
  return getKnownTranslationLabel(
    kind,
    componentKindLabelKeys,
    'statusCenter.componentKind.custom',
    true
  )
}

export function getStatusAuditActionLabel(
  action: string
): StatusTranslationLabel {
  return getKnownTranslationLabel(
    action,
    auditActionLabelKeys,
    'statusCenter.audit.action.custom',
    true
  )
}

export function getStatusAuditObjectTypeLabel(
  objectType: string
): StatusTranslationLabel {
  return getKnownTranslationLabel(
    objectType,
    auditObjectTypeLabelKeys,
    'statusCenter.audit.objectType.custom',
    true
  )
}

export function getStatusSettingLabel(key: string): StatusTranslationLabel {
  return getKnownTranslationLabel(
    key,
    settingLabelKeys,
    'statusCenter.settings.label.custom',
    false
  )
}

const statusLabelKeys: Record<StatusValue, string> = {
  operational: 'statusCenter.status.operational',
  degraded: 'statusCenter.status.degraded',
  outage: 'statusCenter.status.outage',
  unknown: 'statusCenter.status.unknown',
  maintenance: 'statusCenter.status.maintenance',
}

export function getStatusLabelKey(status: string): string {
  if (status in statusLabelKeys) {
    return statusLabelKeys[status as StatusValue]
  }
  return statusLabelKeys.unknown
}

export function validateStatusOverride(
  input: OverrideValidationInput
): StatusCenterValidationKey[] {
  const errors: StatusCenterValidationKey[] = []
  if (!input.reason.trim()) {
    errors.push('statusCenter.validation.reasonRequired')
  }
  if (input.expiresAt <= input.now) {
    errors.push('statusCenter.validation.futureExpiryRequired')
  }
  if (input.status !== 'operational') {
    return errors
  }
  if (input.role !== 'root') {
    errors.push('statusCenter.validation.rootRequired')
    return errors
  }
  if (!input.secureVerified) {
    errors.push('statusCenter.validation.secureVerificationRequired')
  }
  if (input.expiresAt - input.now > 60 * 60) {
    errors.push('statusCenter.validation.forceGreenOneHour')
  }
  return errors
}

export function getRequiredStatusComponentVersion(
  component: Pick<StatusComponent, 'version'> | undefined
): number | null {
  return typeof component?.version === 'number' ? component.version : null
}

export function buildStatusDeliveryRetryInput(
  delivery: Pick<StatusDelivery, 'version'>,
  reason: string
): StatusDeliveryRetryInput | null {
  const trimmedReason = reason.trim()
  if (!trimmedReason) return null
  return {
    expected_version: delivery.version,
    reason: trimmedReason,
  }
}

export function buildPublishedUpdateRows(
  updates: StatusIncidentUpdate[]
): readonly Readonly<PublishedUpdateRow>[] {
  const rows = updates
    .filter((update) => update.published)
    .map((update) =>
      Object.freeze({
        id: update.id,
        state: update.state,
        body: update.body,
        publishedAt: update.published_at,
        canEdit: false as const,
        correctionMode: 'append' as const,
      })
    )
  return Object.freeze(rows)
}

export function getLatestStatusAutomationDraft(
  record: StatusIncidentRecord
): StatusIncidentUpdate | null {
  if (
    record.incident.visibility !== 'private' ||
    record.incident.automation_mode !== 'automatic'
  ) {
    return null
  }

  return record.updates.reduce<StatusIncidentUpdate | null>(
    (latest, update) => {
      if (update.published) return latest
      if (!latest) return update
      if (update.created_at !== latest.created_at) {
        return update.created_at > latest.created_at ? update : latest
      }
      return update.id > latest.id ? update : latest
    },
    null
  )
}

const automationEvidencePrefixes = [
  'Automated evidence: ',
  'Recovery observed; review this resolution suggestion: ',
] as const

export function getStatusAutomationDraftEvidence(
  draft: StatusIncidentUpdate | null | undefined
): string {
  const body = draft?.body.trim() ?? ''
  const prefix = automationEvidencePrefixes.find((value) =>
    body.startsWith(value)
  )
  return prefix ? body.slice(prefix.length).trim() : body
}

const incidentPublicationStates = [
  'investigating',
  'identified',
  'monitoring',
  'resolved',
] as const satisfies readonly StatusIncidentPublishInput['state'][]

function getIncidentPublicationState(
  state: string | undefined
): StatusIncidentPublishInput['state'] {
  return incidentPublicationStates.includes(
    state as StatusIncidentPublishInput['state']
  )
    ? (state as StatusIncidentPublishInput['state'])
    : 'investigating'
}

export function syncIncidentPublicationForm(
  current: IncidentPublicationFormState,
  record: StatusIncidentRecord | undefined
): IncidentPublicationFormState {
  const draft = record ? getLatestStatusAutomationDraft(record) : null
  const sourceIncidentId = record?.incident.id ?? 0
  const sourceDraftId = draft?.id ?? 0

  if (
    current.sourceIncidentId === sourceIncidentId &&
    current.sourceDraftId === sourceDraftId &&
    current.dirty
  ) {
    return current
  }

  const next: IncidentPublicationFormState = {
    sourceIncidentId,
    sourceDraftId,
    state: getIncidentPublicationState(draft?.state),
    body: draft?.body ?? '',
    dirty: false,
  }

  if (
    current.sourceIncidentId === next.sourceIncidentId &&
    current.sourceDraftId === next.sourceDraftId &&
    current.state === next.state &&
    current.body === next.body &&
    current.dirty === next.dirty
  ) {
    return current
  }
  return next
}

function isHttpConflict(error: unknown): boolean {
  if (!error || typeof error !== 'object' || !('response' in error)) {
    return false
  }
  const response = error.response
  return (
    !!response &&
    typeof response === 'object' &&
    'status' in response &&
    response.status === 409
  )
}

export async function resolveStatusMutationError(
  error: unknown,
  reload: () => void | Promise<void>
): Promise<StatusMutationErrorResolution> {
  if (isHttpConflict(error)) {
    await reload()
    return {
      conflict: true,
      messageKey: 'statusCenter.conflict.reload',
    }
  }
  return {
    conflict: false,
    messageKey: 'statusCenter.error.requestFailed',
  }
}

export function getStatusCenterPermissions(
  role: StatusCenterRole,
  secureVerified: boolean
): StatusCenterPermissions {
  const canView = role === 'admin' || role === 'root'
  const canViewRootControls = role === 'root'
  const canUseSensitiveRootControls = canViewRootControls && secureVerified
  return {
    canView,
    canPublishIncidents: canView,
    canManageMaintenance: canView,
    canCreateNonGreenOverride: canView,
    canViewRootControls,
    canUseSensitiveRootControls,
    requiresSecureVerification: canViewRootControls && !secureVerified,
  }
}
