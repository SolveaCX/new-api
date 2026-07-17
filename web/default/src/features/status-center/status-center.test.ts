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
import {
  buildPublishedUpdateRows,
  getStatusCenterPermissions,
  getStatusLabelKey,
  resolveStatusMutationError,
  validateStatusOverride,
} from './types'

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
