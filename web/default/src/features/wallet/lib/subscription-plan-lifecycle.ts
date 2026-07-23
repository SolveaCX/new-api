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
  ChangePlanPaymentMode,
  PlanRecord,
  SelfSubscriptionData,
  SelfSubscriptionDataResponse,
  SubscriptionContract,
  SubscriptionCurrentPeriod,
  SubscriptionQuota,
} from '@/features/subscriptions/types'
import type { TopupInfo } from '../types'

export type PlanAction =
  | 'current'
  | 'upgrade_now'
  | 'downgrade_next_period'
  | 'unavailable'

type PlanRelation = 'current' | 'upgrade' | 'downgrade' | 'unavailable'

type LegacyCapabilityAliases = {
  migration_required?: boolean
  migration_blocked_reason?: string
}

type WalletSubscriptionCapabilities = {
  can_change_plan: boolean
  can_use_stripe_recurring: boolean
  can_use_balance_one_period: boolean
  can_cancel: boolean
  can_resume: boolean
  requires_support: boolean
  has_pending_intent: boolean
  is_grace: boolean
  is_cancel_at_period_end: boolean
  has_migration_conflict: boolean
} & LegacyCapabilityAliases

type WalletSubscriptionMigration = {
  requires_admin_review: boolean
  classification: string
  reason: string
}

export type LifecyclePlanRecord = PlanRecord & {
  relation?: PlanRelation | string
  tier_rank?: number | null
}

export type WalletSubscriptionContract = SubscriptionContract & {
  id: number
  current_period_start?: number
  current_period_end?: number
  grace_period_end?: number
}

export type WalletSelfSubscriptionData = Omit<
  SelfSubscriptionData,
  'capabilities' | 'contract' | 'current_period' | 'migration' | 'quota'
> & {
  contract?: WalletSubscriptionContract | null
  current_period?: SubscriptionCurrentPeriod
  quota?: SubscriptionQuota
  capabilities: WalletSubscriptionCapabilities
  migration: WalletSubscriptionMigration
}

const DEFAULT_CAPABILITIES: WalletSelfSubscriptionData['capabilities'] = {
  can_change_plan: false,
  can_use_stripe_recurring: false,
  can_use_balance_one_period: false,
  can_cancel: false,
  can_resume: false,
  requires_support: true,
  has_pending_intent: false,
  is_grace: false,
  is_cancel_at_period_end: false,
  has_migration_conflict: true,
}

const DEFAULT_CURRENT_PERIOD: SubscriptionCurrentPeriod = {
  start: 0,
  end: 0,
  grace_period_end: 0,
}

const DEFAULT_QUOTA: SubscriptionQuota = {
  amount_total: 0,
  amount_used: 0,
  amount_remaining: 0,
  unlimited: true,
}

const DEFAULT_MIGRATION: WalletSelfSubscriptionData['migration'] = {
  requires_admin_review: true,
  classification: 'unknown',
  reason: '',
}

function normalizeMigration(
  data: SelfSubscriptionDataResponse | undefined
): WalletSelfSubscriptionData['migration'] {
  const migration = data?.migration
  const requiresReview =
    typeof migration?.requires_admin_review === 'boolean'
      ? migration.requires_admin_review
      : true

  return {
    requires_admin_review: requiresReview,
    classification:
      migration?.classification ?? DEFAULT_MIGRATION.classification,
    reason: migration?.reason ?? DEFAULT_MIGRATION.reason,
  }
}

function normalizeContract(
  contract: SelfSubscriptionDataResponse['contract'],
  currentPeriod: SubscriptionCurrentPeriod | undefined
): WalletSubscriptionContract | null | undefined {
  if (!contract) return contract
  const canonicalContract = contract as SubscriptionContract & {
    id?: number
    current_period_start?: number
    current_period_end?: number
    grace_period_end?: number
  }
  return {
    ...canonicalContract,
    id: canonicalContract.id ?? canonicalContract.contract_id ?? 0,
    current_period_start:
      canonicalContract.current_period_start ?? currentPeriod?.start ?? 0,
    current_period_end:
      canonicalContract.current_period_end ?? currentPeriod?.end ?? 0,
    grace_period_end:
      canonicalContract.grace_period_end ??
      currentPeriod?.grace_period_end ??
      0,
  }
}

export function normalizeSelfSubscriptionData(
  data: SelfSubscriptionDataResponse | undefined
): WalletSelfSubscriptionData {
  const migration = normalizeMigration(data)
  const capabilities = data?.capabilities
  const hasMigrationConflict =
    typeof capabilities?.has_migration_conflict === 'boolean'
      ? capabilities.has_migration_conflict
      : DEFAULT_CAPABILITIES.has_migration_conflict

  return {
    billing_preference: data?.billing_preference || 'subscription_first',
    contract: normalizeContract(data?.contract, data?.current_period) ?? null,
    current_entitlement: data?.current_entitlement ?? null,
    current_period: data?.current_period ?? DEFAULT_CURRENT_PERIOD,
    quota: data?.quota ?? DEFAULT_QUOTA,
    pending_change: data?.pending_change ?? null,
    capabilities: {
      ...DEFAULT_CAPABILITIES,
      can_change_plan: capabilities?.can_change_plan === true,
      can_use_stripe_recurring: capabilities?.can_use_stripe_recurring === true,
      can_use_balance_one_period:
        capabilities?.can_use_balance_one_period === true,
      can_cancel: capabilities?.can_cancel === true,
      can_resume: capabilities?.can_resume === true,
      requires_support:
        capabilities?.requires_support ?? DEFAULT_CAPABILITIES.requires_support,
      has_pending_intent: capabilities?.has_pending_intent === true,
      is_grace: capabilities?.is_grace === true,
      is_cancel_at_period_end: capabilities?.is_cancel_at_period_end === true,
      has_migration_conflict: hasMigrationConflict,
    },
    migration,
    subscriptions: data?.subscriptions || [],
    all_subscriptions: data?.all_subscriptions || [],
    recurring_subscriptions: data?.recurring_subscriptions || [],
  }
}

export function getDisplayedPlanAction(
  planRecord: LifecyclePlanRecord,
  currentPlanId: number,
  allowedPaymentModes: ChangePlanPaymentMode[],
  lifecycle:
    | WalletSelfSubscriptionData['capabilities']
    | Pick<WalletSelfSubscriptionData, 'capabilities' | 'migration'>
): PlanAction {
  if (planRecord.relation === 'current' || planRecord.plan.id === currentPlanId)
    return 'current'
  const hasLifecycle = 'capabilities' in lifecycle
  const capabilities = hasLifecycle ? lifecycle.capabilities : lifecycle
  const migrationRequiresAdminReview = hasLifecycle
    ? lifecycle.migration.requires_admin_review
    : true
  if (
    allowedPaymentModes.length === 0 ||
    capabilities.can_change_plan !== true ||
    migrationRequiresAdminReview ||
    capabilities.has_migration_conflict
  ) {
    return 'unavailable'
  }
  if (planRecord.relation === 'upgrade') return 'upgrade_now'
  if (planRecord.relation === 'downgrade') return 'downgrade_next_period'
  return 'unavailable'
}

export function getAllowedPaymentModes(
  plan: PlanRecord['plan'],
  topupInfo: TopupInfo | null,
  capabilities: WalletSelfSubscriptionData['capabilities']
): ChangePlanPaymentMode[] {
  const configuredModes = plan.payment_modes ?? []

  const modes: ChangePlanPaymentMode[] = []
  if (
    configuredModes.includes('stripe_recurring') &&
    !!topupInfo?.enable_stripe_topup &&
    capabilities.can_use_stripe_recurring
  ) {
    modes.push('stripe_recurring')
  }
  if (
    configuredModes.includes('balance_one_period') &&
    capabilities.can_use_balance_one_period
  ) {
    modes.push('balance_one_period')
  }
  return modes
}
