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
import { createElement } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeAll, describe, expect, test } from 'bun:test'
import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import {
  PrivateIncidentDraftReview,
  PublishedIncidentHistory,
} from './components/incidents'
import { DeliveryRetryAction } from './components/operations'
import { SettingsPanel } from './components/settings'
import { statusCenterQueryKeys } from './api'
import {
  buildStatusDeliveryRetryInput,
  buildPublishedUpdateRows,
  getStatusAuditActionLabel,
  getStatusAuditObjectTypeLabel,
  getStatusComponentKindLabel,
  getLatestStatusAutomationDraft,
  getRequiredStatusComponentVersion,
  getStatusAutomationDraftEvidence,
  getStatusCenterPermissions,
  getStatusLabelKey,
  getStatusSettingLabel,
  resolveStatusMutationError,
  syncIncidentPublicationForm,
  type IncidentPublicationFormState,
  type StatusIncidentRecord,
  type StatusSetting,
  type StatusTranslationLabel,
  validateStatusOverride,
} from './types'

const testI18n = createInstance()

beforeAll(async () => {
  await testI18n.use(initReactI18next).init({
    lng: 'en',
    resources: {
      en: {
        translation: {
          'statusCenter.incidents.state.investigating': 'Investigating',
          'statusCenter.incidents.correctionsAppendOnly':
            'Published updates are immutable. Add a correction instead.',
          'statusCenter.incidents.appendCorrection': 'Append correction',
          'statusCenter.deliveries.retry': 'Retry',
          'statusCenter.componentKind.router': 'Router',
          'statusCenter.componentKind.model': 'Model',
          'statusCenter.componentKind.custom':
            'Other component ({{identifier}})',
          'statusCenter.audit.action.deliveryRetry': 'Delivery retried',
          'statusCenter.audit.action.incidentDraftAuto':
            'Automatic incident draft created',
          'statusCenter.audit.action.incidentPublish': 'Incident published',
          'statusCenter.audit.action.maintenanceCreate':
            'Maintenance created',
          'statusCenter.audit.action.maintenanceStart': 'Maintenance started',
          'statusCenter.audit.action.maintenanceEnd': 'Maintenance ended',
          'statusCenter.audit.action.maintenanceComponentStart':
            'Component maintenance started',
          'statusCenter.audit.action.maintenanceComponentEnd':
            'Component maintenance ended',
          'statusCenter.audit.action.overrideSet': 'Override set',
          'statusCenter.audit.action.overrideExpire': 'Override expired',
          'statusCenter.audit.action.custom': 'Other action ({{identifier}})',
          'statusCenter.audit.objectType.delivery': 'Delivery',
          'statusCenter.audit.objectType.incident': 'Incident',
          'statusCenter.audit.objectType.maintenance': 'Maintenance',
          'statusCenter.audit.objectType.component': 'Component',
          'statusCenter.audit.objectType.custom':
            'Other object ({{identifier}})',
          'statusCenter.settings.label.discordWebhookEndpoint':
            'Discord webhook endpoint',
          'statusCenter.settings.label.discordDeliveryState':
            'Discord delivery state',
          'statusCenter.settings.label.evidenceMaxAgeSeconds':
            'Maximum evidence age',
          'statusCenter.settings.label.custom': 'Custom status setting',
          'statusCenter.settings.readOnly': 'Read only',
          'statusCenter.incidents.privateDraft.title':
            'Private automation draft',
          'statusCenter.incidents.privateDraft.description':
            'Review the draft and evidence before publishing.',
          'statusCenter.incidents.privateDraft.body': 'Draft body',
          'statusCenter.incidents.privateDraft.evidence': 'Internal evidence',
        },
      },
    },
    interpolation: { escapeValue: false },
  })
})

function renderWithI18n(component: ReturnType<typeof createElement>): string {
  return renderToStaticMarkup(
    createElement(I18nextProvider, { i18n: testI18n }, component)
  )
}

function renderStatusLabel(label: StatusTranslationLabel): string {
  return testI18n.t(label.key, label.values)
}

function renderSettingsPanel(settings: StatusSetting[]): string {
  const queryClient = new QueryClient()
  queryClient.setQueryData(statusCenterQueryKeys.settings(), settings)
  return renderWithI18n(
    createElement(
      QueryClientProvider,
      { client: queryClient },
      createElement(SettingsPanel, {
        active: true,
        isRoot: true,
        runSensitiveAction: async (action) => {
          await action()
        },
      })
    )
  )
}

describe('status center identifier labels', () => {
  test.each([
    ['router', 'statusCenter.componentKind.router', 'Router'],
    ['model', 'statusCenter.componentKind.model', 'Model'],
  ] as const)('translates the %s component kind', (kind, key, rendered) => {
    const label = getStatusComponentKindLabel(kind)
    expect(label.key).toBe(key)
    expect(renderStatusLabel(label)).toBe(rendered)
  })

  test.each([
    ['status.delivery.retry', 'Delivery retried'],
    ['status.incident.draft.auto', 'Automatic incident draft created'],
    ['status.incident.publish', 'Incident published'],
    ['status.maintenance.create', 'Maintenance created'],
    ['status.maintenance.start', 'Maintenance started'],
    ['status.maintenance.end', 'Maintenance ended'],
    ['status.maintenance.component.start', 'Component maintenance started'],
    ['status.maintenance.component.end', 'Component maintenance ended'],
    ['status.override.set', 'Override set'],
    ['status.override.expire', 'Override expired'],
  ] as const)('translates the %s audit action', (action, rendered) => {
    expect(renderStatusLabel(getStatusAuditActionLabel(action))).toBe(rendered)
  })

  test.each([
    ['delivery', 'Delivery'],
    ['incident', 'Incident'],
    ['maintenance', 'Maintenance'],
    ['component', 'Component'],
  ] as const)('translates the %s audit object type', (objectType, rendered) => {
    expect(renderStatusLabel(getStatusAuditObjectTypeLabel(objectType))).toBe(
      rendered
    )
  })

  test('preserves unknown audit identifiers in translated diagnostic wrappers', () => {
    expect(
      renderStatusLabel(getStatusAuditActionLabel('status.future.action'))
    ).toBe('Other action (status.future.action)')
    expect(
      renderStatusLabel(getStatusAuditObjectTypeLabel('future-object'))
    ).toBe('Other object (future-object)')
  })

  test.each([
    ['status.discord.webhook_endpoint', 'Discord webhook endpoint'],
    ['status.discord.delivery_state', 'Discord delivery state'],
    ['status.evidence_max_age_seconds', 'Maximum evidence age'],
    ['status.custom.setting', 'Custom status setting'],
  ] as const)('translates the %s setting label', (key, rendered) => {
    expect(renderStatusLabel(getStatusSettingLabel(key))).toBe(rendered)
  })
})

describe('status settings controls', () => {
  test('renders internal Discord delivery state without generic mutation controls', () => {
    const savedEndpoint = 'https://discord.example/secret'
    const html = renderSettingsPanel([
      {
        key: 'status.discord.delivery_state',
        value: 'enabled',
        sensitive: false,
        configured: true,
        version: 2,
        updated_by: 1,
        updated_at: 1_800_000_000,
      },
      {
        key: 'status.evidence_max_age_seconds',
        value: '300',
        sensitive: false,
        configured: true,
        version: 4,
        updated_by: 1,
        updated_at: 1_800_000_000,
      },
      {
        key: 'status.discord.webhook_endpoint',
        value: savedEndpoint,
        sensitive: true,
        configured: true,
        version: 6,
        updated_by: 1,
        updated_at: 1_800_000_000,
      },
    ])

    const deliveryStateStart = html.indexOf('status.discord.delivery_state')
    const supportedSettingStart = html.indexOf(
      'status.evidence_max_age_seconds'
    )
    const deliveryStateRow = html.slice(
      deliveryStateStart,
      supportedSettingStart
    )

    expect(deliveryStateStart).toBeGreaterThan(-1)
    expect(supportedSettingStart).toBeGreaterThan(deliveryStateStart)
    expect(deliveryStateRow).toContain('enabled')
    expect(deliveryStateRow).toContain('Read only')
    expect(deliveryStateRow).not.toContain('<input')
    expect(deliveryStateRow).not.toContain('<button')
    expect(html).toContain(
      'id="setting-status.evidence_max_age_seconds"'
    )
    expect(html).toContain('id="status-discord-endpoint"')
    expect(html).not.toContain(savedEndpoint)
  })
})

describe('status center status labels', () => {
  test.each([
    ['operational', 'statusCenter.status.operational'],
    ['degraded', 'statusCenter.status.degraded'],
    ['outage', 'statusCenter.status.outage'],
    ['unknown', 'statusCenter.status.unknown'],
    ['maintenance', 'statusCenter.status.maintenance'],
  ] as const)('maps %s to translated copy', (status, labelKey) => {
    expect(getStatusLabelKey(status)).toBe(labelKey)
  })

  test('treats an unrecognized server state as unknown', () => {
    expect(getStatusLabelKey('stale-server-value')).toBe(
      'statusCenter.status.unknown'
    )
  })
})

describe('status center override validation', () => {
  const now = 1_800_000_000

  test('requires a reason and a future expiry for every override', () => {
    expect(
      validateStatusOverride({
        status: 'degraded',
        reason: '   ',
        expiresAt: now,
        now,
        role: 'admin',
        secureVerified: false,
      })
    ).toEqual([
      'statusCenter.validation.reasonRequired',
      'statusCenter.validation.futureExpiryRequired',
    ])
  })

  test('restricts operational overrides to securely verified Root users', () => {
    expect(
      validateStatusOverride({
        status: 'operational',
        reason: 'Recovery confirmed',
        expiresAt: now + 600,
        now,
        role: 'admin',
        secureVerified: false,
      })
    ).toContain('statusCenter.validation.rootRequired')

    expect(
      validateStatusOverride({
        status: 'operational',
        reason: 'Recovery confirmed',
        expiresAt: now + 600,
        now,
        role: 'root',
        secureVerified: false,
      })
    ).toContain('statusCenter.validation.secureVerificationRequired')
  })

  test('limits force-green to one hour', () => {
    expect(
      validateStatusOverride({
        status: 'operational',
        reason: 'Recovery confirmed',
        expiresAt: now + 3_601,
        now,
        role: 'root',
        secureVerified: true,
      })
    ).toContain('statusCenter.validation.forceGreenOneHour')

    expect(
      validateStatusOverride({
        status: 'operational',
        reason: 'Recovery confirmed',
        expiresAt: now + 3_600,
        now,
        role: 'root',
        secureVerified: true,
      })
    ).toEqual([])
  })

  test('requires the server-provided admin component version without a fallback', () => {
    expect(getRequiredStatusComponentVersion({ version: 7 })).toBe(7)
    expect(getRequiredStatusComponentVersion({})).toBeNull()
  })
})

describe('published incident updates', () => {
  test('renders published updates as immutable history and excludes drafts', () => {
    const rows = buildPublishedUpdateRows([
      {
        id: 1,
        incident_id: 10,
        event_id: 'published-1',
        state: 'investigating',
        body: 'Investigating elevated errors.',
        published: true,
        published_at: 1_800_000_000,
        actor_id: 2,
        created_at: 1_800_000_000,
      },
      {
        id: 2,
        incident_id: 10,
        event_id: 'draft-1',
        state: 'identified',
        body: 'Draft correction.',
        published: false,
        published_at: 0,
        actor_id: 2,
        created_at: 1_800_000_100,
      },
    ])

    expect(rows).toHaveLength(1)
    expect(rows[0]).toMatchObject({
      id: 1,
      body: 'Investigating elevated errors.',
      canEdit: false,
      correctionMode: 'append',
    })
    expect(Object.isFrozen(rows[0])).toBe(true)
    expect(Object.isFrozen(rows)).toBe(true)
  })

  test('renders immutable published history with an append-only correction path', () => {
    const html = renderWithI18n(
      createElement(PublishedIncidentHistory, {
        updates: [
          {
            id: 1,
            incident_id: 10,
            event_id: 'published-1',
            state: 'investigating',
            body: 'Investigating elevated errors.',
            published: true,
            published_at: 1_800_000_000,
            actor_id: 2,
            created_at: 1_800_000_000,
          },
          {
            id: 2,
            incident_id: 10,
            event_id: 'draft-1',
            state: 'identified',
            body: 'Draft correction.',
            published: false,
            published_at: 0,
            actor_id: 2,
            created_at: 1_800_000_100,
          },
        ],
        correctionTargetId: 'incident-new-update',
      })
    )

    expect(html).toContain('Investigating elevated errors.')
    expect(html).not.toContain('Draft correction.')
    expect(html).toContain('Investigating')
    expect(html).toContain('href="#incident-new-update"')
    expect(html).toContain('Append correction')
    expect(html).not.toContain('>Edit<')
  })
})

describe('private automation incident drafts', () => {
  const record: StatusIncidentRecord = {
    incident: {
      id: 10,
      public_id: 'inc-10',
      kind: 'incident',
      title: 'Automated component status review',
      impact: 'outage',
      status: 'draft',
      visibility: 'private',
      automation_mode: 'automatic',
      version: 3,
      created_at: 1_800_000_000,
      updated_at: 1_800_000_200,
    },
    component_ids: [4],
    updates: [
      {
        id: 1,
        incident_id: 10,
        event_id: 'published-1',
        state: 'investigating',
        body: 'Earlier public update.',
        published: true,
        published_at: 1_800_000_050,
        created_at: 1_800_000_050,
      },
      {
        id: 2,
        incident_id: 10,
        event_id: 'draft-older',
        state: 'investigating',
        body: 'Automated evidence: earlier probe failure',
        published: false,
        published_at: 0,
        created_at: 1_800_000_100,
      },
      {
        id: 3,
        incident_id: 10,
        event_id: 'draft-latest',
        state: 'identified',
        body: 'Automated evidence: probe and traffic signals confirm the outage',
        published: false,
        published_at: 0,
        created_at: 1_800_000_200,
      },
    ],
  }

  test('extracts the latest private automation draft and its internal evidence', () => {
    const draft = getLatestStatusAutomationDraft(record)

    expect(draft?.id).toBe(3)
    expect(draft?.published).toBe(false)
    expect(getStatusAutomationDraftEvidence(draft)).toBe(
      'probe and traffic signals confirm the outage'
    )
    expect(buildPublishedUpdateRows(record.updates)).toHaveLength(1)
  })

  test('prefills from the selected draft without overwriting active edits on refresh', () => {
    const initial: IncidentPublicationFormState = {
      sourceIncidentId: 0,
      sourceDraftId: 0,
      state: 'investigating',
      body: '',
      dirty: false,
    }
    const prefilled = syncIncidentPublicationForm(initial, record)
    expect(prefilled).toMatchObject({
      sourceIncidentId: 10,
      sourceDraftId: 3,
      state: 'identified',
      body: 'Automated evidence: probe and traffic signals confirm the outage',
      dirty: false,
    })

    const edited = { ...prefilled, body: 'Operator-reviewed copy', dirty: true }
    const refreshedRecord = {
      ...record,
      updates: record.updates.map((update) =>
        update.id === 3
          ? { ...update, body: 'Automated evidence: refreshed evidence' }
          : update
      ),
    }
    expect(syncIncidentPublicationForm(edited, refreshedRecord)).toEqual(
      edited
    )

    const nextRecord = {
      ...record,
      incident: { ...record.incident, id: 11, public_id: 'inc-11' },
      updates: [
        {
          ...record.updates[2],
          id: 4,
          incident_id: 11,
          event_id: 'draft-next',
          state: 'resolved',
          body: 'Recovery observed; review this resolution suggestion: probes recovered',
        },
      ],
    }
    expect(syncIncidentPublicationForm(edited, nextRecord)).toMatchObject({
      sourceIncidentId: 11,
      sourceDraftId: 4,
      state: 'resolved',
      body: 'Recovery observed; review this resolution suggestion: probes recovered',
      dirty: false,
    })
  })

  test('renders private draft body and evidence for administrator review', () => {
    const html = renderWithI18n(
      createElement(PrivateIncidentDraftReview, {
        draft: getLatestStatusAutomationDraft(record),
      })
    )

    expect(html).toContain('Private automation draft')
    expect(html).toContain(
      'Automated evidence: probe and traffic signals confirm the outage'
    )
    expect(html).toContain('Internal evidence')
    expect(html).toContain('probe and traffic signals confirm the outage')
  })
})

describe('delivery retry controls', () => {
  test('requires and trims the retry reason in the optimistic payload', () => {
    const delivery = {
      id: 41,
      version: 4,
    }

    expect(buildStatusDeliveryRetryInput(delivery, '   ')).toBeNull()
    expect(buildStatusDeliveryRetryInput(delivery, '  transient outage  ')).toEqual(
      {
        expected_version: 4,
        reason: 'transient outage',
      }
    )
  })

  test('renders retry only for Root users viewing a dead delivery', () => {
    const deadDelivery = {
      id: 41,
      published_update_id: 2,
      destination_type: 'webhook',
      destination_id: 5,
      event_id: 'delivery-41',
      status: 'dead',
      locked_until: 0,
      attempts: 3,
      next_attempt_at: 0,
      last_error: 'failed',
      version: 4,
      created_at: 1,
      updated_at: 2,
    }
    const rootHTML = renderWithI18n(
      createElement(DeliveryRetryAction, {
        delivery: deadDelivery,
        isRoot: true,
        pending: false,
        onRetry: () => undefined,
      })
    )
    const adminHTML = renderWithI18n(
      createElement(DeliveryRetryAction, {
        delivery: deadDelivery,
        isRoot: false,
        pending: false,
        onRetry: () => undefined,
      })
    )
    const deliveredHTML = renderWithI18n(
      createElement(DeliveryRetryAction, {
        delivery: { ...deadDelivery, status: 'delivered' },
        isRoot: true,
        pending: false,
        onRetry: () => undefined,
      })
    )

    expect(rootHTML).toContain('Retry')
    expect(adminHTML).not.toContain('Retry')
    expect(deliveredHTML).not.toContain('Retry')
  })
})

describe('optimistic-version conflicts', () => {
  test('reloads affected state and returns translated conflict messaging on 409', async () => {
    let reloads = 0

    const result = await resolveStatusMutationError(
      { response: { status: 409 } },
      async () => {
        reloads += 1
      }
    )

    expect(reloads).toBe(1)
    expect(result).toEqual({
      conflict: true,
      messageKey: 'statusCenter.conflict.reload',
    })
  })

  test('does not reload for non-conflict failures', async () => {
    let reloads = 0

    const result = await resolveStatusMutationError(
      new Error('offline'),
      () => {
        reloads += 1
      }
    )

    expect(reloads).toBe(0)
    expect(result).toEqual({
      conflict: false,
      messageKey: 'statusCenter.error.requestFailed',
    })
  })
})

describe('permission-based controls', () => {
  test('allows Admin operations without exposing Root-only controls', () => {
    expect(getStatusCenterPermissions('admin', false)).toEqual({
      canView: true,
      canPublishIncidents: true,
      canManageMaintenance: true,
      canCreateNonGreenOverride: true,
      canViewRootControls: false,
      canUseSensitiveRootControls: false,
      requiresSecureVerification: false,
    })
  })

  test('keeps sensitive Root controls disabled until secure verification', () => {
    expect(getStatusCenterPermissions('root', false)).toMatchObject({
      canViewRootControls: true,
      canUseSensitiveRootControls: false,
      requiresSecureVerification: true,
    })
    expect(getStatusCenterPermissions('root', true)).toMatchObject({
      canViewRootControls: true,
      canUseSensitiveRootControls: true,
      requiresSecureVerification: false,
    })
  })

  test('forbids non-admin users', () => {
    expect(getStatusCenterPermissions('user', false).canView).toBe(false)
  })
})
