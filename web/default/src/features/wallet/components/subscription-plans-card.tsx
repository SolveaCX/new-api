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
import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  AlertTriangle,
  Check,
  Crown,
  PauseCircle,
  PlayCircle,
  RefreshCw,
  Sparkles,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  StatusBadge,
  dotColorMap,
  textColorMap,
} from '@/components/status-badge'
import {
  cancelRecurringSubscription,
  changeSubscriptionPlan,
  getPublicPlans,
  getSelfSubscriptionFull,
  resumeRecurringSubscription,
} from '@/features/subscriptions/api'
import { ChangeSubscriptionPlanDialog } from '@/features/subscriptions/components/dialogs/change-subscription-plan-dialog'
import { RecurringSubscriptionActionDialog } from '@/features/subscriptions/components/dialogs/recurring-subscription-action-dialog'
import { formatDuration, formatResetPeriod } from '@/features/subscriptions/lib'
import type {
  ChangePlanPaymentMode,
  PlanRecord,
  RecurringSubscription,
  SubscriptionContract,
  UserSubscriptionRecord,
} from '@/features/subscriptions/types'
import {
  type LifecyclePlanRecord,
  type WalletSelfSubscriptionData,
  getAllowedPaymentModes,
  getDisplayedPlanAction,
  normalizeSelfSubscriptionData,
} from '../lib/subscription-plan-lifecycle'
import type { TopupInfo } from '../types'

interface SubscriptionPlansCardProps {
  topupInfo: TopupInfo | null
  onAvailabilityChange?: (available: boolean) => void
  userQuota?: number
  onPurchaseSuccess?: () => void | Promise<void>
}

const EXTERNAL_RETURN_POLL_KEY = 'new-api:subscription-change-return-pending'

function createStableSubscriptionRequestId(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID()
  }
  return '10000000-1000-4000-8000-100000000000'.replace(/[018]/g, (c) =>
    (Number(c) ^ ((Math.random() * 16) >> (Number(c) / 4))).toString(16)
  )
}

function getCurrentSubscription(
  contract: SubscriptionContract | null | undefined,
  subscriptions: UserSubscriptionRecord[]
): UserSubscriptionRecord | undefined {
  if (contract?.current_entitlement_id) {
    const match = subscriptions.find(
      (item) => item.subscription.id === contract.current_entitlement_id
    )
    if (match) return match
  }
  if (contract?.current_plan_id) {
    const match = subscriptions.find(
      (item) => item.subscription.plan_id === contract.current_plan_id
    )
    if (match) return match
  }
  return subscriptions[0]
}

function formatTimestamp(ts: number | undefined): string {
  if (!ts) return '-'
  return new Date(ts * 1000).toLocaleString()
}

function rememberExternalSubscriptionReturn() {
  if (typeof window === 'undefined') return
  window.sessionStorage.setItem(EXTERNAL_RETURN_POLL_KEY, '1')
}

export function SubscriptionPlansCard(props: SubscriptionPlansCardProps) {
  const { t } = useTranslation()
  const { topupInfo, onAvailabilityChange, userQuota, onPurchaseSuccess } =
    props
  const [plans, setPlans] = useState<LifecyclePlanRecord[]>([])
  const [selfData, setSelfData] = useState<WalletSelfSubscriptionData>(() =>
    normalizeSelfSubscriptionData(undefined)
  )
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [changeTarget, setChangeTarget] = useState<{
    plan: PlanRecord
    requestId: string
  } | null>(null)
  const [changing, setChanging] = useState(false)
  const [recurringAction, setRecurringAction] = useState<{
    action: 'cancel' | 'resume'
    subscription: RecurringSubscription
  } | null>(null)
  const [pendingRecurringBindingId, setPendingRecurringBindingId] = useState<
    number | null
  >(null)

  const fetchPlans = useCallback(async () => {
    try {
      const res = await getPublicPlans()
      setPlans(res.success ? ((res.data || []) as LifecyclePlanRecord[]) : [])
    } catch {
      setPlans([])
    }
  }, [])

  const fetchSelfSubscription = useCallback(async () => {
    try {
      const res = await getSelfSubscriptionFull()
      setSelfData(
        normalizeSelfSubscriptionData(res.success ? res.data : undefined)
      )
    } catch {
      setSelfData(normalizeSelfSubscriptionData(undefined))
    }
  }, [])

  useEffect(() => {
    const init = async () => {
      setLoading(true)
      await Promise.all([fetchPlans(), fetchSelfSubscription()])
      setLoading(false)
    }
    void init()
  }, [fetchPlans, fetchSelfSubscription])

  useEffect(() => {
    if (typeof window === 'undefined') return
    if (!window.sessionStorage.getItem(EXTERNAL_RETURN_POLL_KEY)) return
    let cancelled = false
    let attempts = 0
    const poll = async () => {
      if (cancelled || attempts >= 5) return
      attempts += 1
      await fetchSelfSubscription()
      if (attempts >= 5) {
        window.sessionStorage.removeItem(EXTERNAL_RETURN_POLL_KEY)
        return
      }
      window.setTimeout(poll, 2000)
    }
    void poll()
    return () => {
      cancelled = true
    }
  }, [fetchSelfSubscription])

  useEffect(() => {
    if (typeof window === 'undefined') return
    const refreshAfterReturn = () => {
      if (window.sessionStorage.getItem(EXTERNAL_RETURN_POLL_KEY)) {
        void fetchSelfSubscription()
      }
    }
    window.addEventListener('focus', refreshAfterReturn)
    document.addEventListener('visibilitychange', refreshAfterReturn)
    return () => {
      window.removeEventListener('focus', refreshAfterReturn)
      document.removeEventListener('visibilitychange', refreshAfterReturn)
    }
  }, [fetchSelfSubscription])

  const handleRefresh = async () => {
    setRefreshing(true)
    try {
      await Promise.all([fetchPlans(), fetchSelfSubscription()])
    } finally {
      setRefreshing(false)
    }
  }

  const contract = selfData.contract ?? null
  const currentSubscription = getCurrentSubscription(
    contract,
    selfData.subscriptions
  )
  const currentPlanId =
    contract?.current_plan_id ||
    selfData.current_entitlement?.plan_id ||
    currentSubscription?.subscription.plan_id ||
    0
  const currentPlan = plans.find((item) => item.plan.id === currentPlanId)?.plan
  const pendingPlan = plans.find(
    (item) =>
      item.plan.id ===
      (selfData.pending_change?.to_plan_id || contract?.pending_plan_id)
  )?.plan
  const currentRecurring = selfData.recurring_subscriptions.find(
    (item) =>
      item.binding_id === contract?.current_provider_binding_id ||
      item.binding_id === selfData.current_entitlement?.provider_binding_id ||
      item.plan_id === currentPlanId
  )
  const isAvailable =
    loading || plans.length > 0 || !!contract || !!currentSubscription

  useEffect(() => {
    onAvailabilityChange?.(isAvailable)
  }, [isAvailable, onAvailabilityChange])

  const currentUsage = useMemo(() => {
    if (selfData.quota) {
      const total = Number(selfData.quota.amount_total || 0)
      const used = Number(selfData.quota.amount_used || 0)
      const remaining = Number(selfData.quota.amount_remaining || 0)
      const percent =
        total > 0 ? Math.min(100, Math.round((used / total) * 100)) : 0
      return {
        total,
        used,
        remaining,
        percent,
        unlimited: selfData.quota.unlimited,
      }
    }
    const subscription = currentSubscription?.subscription
    const total = Number(subscription?.amount_total || 0)
    const used = Number(subscription?.amount_used || 0)
    const remaining = total > 0 ? Math.max(0, total - used) : 0
    const percent =
      total > 0 ? Math.min(100, Math.round((used / total) * 100)) : 0
    return { total, used, remaining, percent, unlimited: total === 0 }
  }, [currentSubscription, selfData.quota])

  const handleRecurringAction = async () => {
    if (!recurringAction) return
    const bindingId = recurringAction.subscription.binding_id
    setPendingRecurringBindingId(bindingId)
    try {
      const res =
        recurringAction.action === 'cancel'
          ? await cancelRecurringSubscription(bindingId)
          : await resumeRecurringSubscription(bindingId)
      if (res.success) {
        toast.success(res.message || t('Updated successfully'))
        setRecurringAction(null)
        await fetchSelfSubscription()
        await onPurchaseSuccess?.()
      } else {
        toast.error(res.message || t('Update failed'))
      }
    } catch {
      toast.error(t('Request failed'))
    } finally {
      setPendingRecurringBindingId(null)
    }
  }

  const handleConfirmChange = async (paymentMode: ChangePlanPaymentMode) => {
    if (!changeTarget) return
    setChanging(true)
    try {
      const res = await changeSubscriptionPlan({
        plan_id: changeTarget.plan.plan.id,
        payment_mode: paymentMode,
        request_id: changeTarget.requestId,
      })
      if (!res.success || !res.data) {
        toast.error(res.message || t('Update failed'))
        return
      }
      if (res.data.status === 'checkout_required') {
        if (!res.data.checkout_url) {
          toast.error(t('Payment request failed'))
          return
        }
        rememberExternalSubscriptionReturn()
        window.location.assign(res.data.checkout_url)
        return
      }
      if (res.data.status === 'payment_action_required') {
        if (!res.data.hosted_invoice_url) {
          toast.error(t('Payment request failed'))
          return
        }
        rememberExternalSubscriptionReturn()
        window.location.assign(res.data.hosted_invoice_url)
        return
      }
      toast.success(
        res.data.status === 'scheduled'
          ? t('Plan change scheduled')
          : t('Updated successfully')
      )
      setChangeTarget(null)
      await fetchSelfSubscription()
      await onPurchaseSuccess?.()
    } catch {
      toast.error(t('Request failed'))
    } finally {
      setChanging(false)
    }
  }

  if (loading) {
    return (
      <Card className='gap-0 overflow-hidden py-0'>
        <CardHeader className='border-b p-3 !pb-3 sm:p-5 sm:!pb-5'>
          <Skeleton className='h-6 w-32' />
        </CardHeader>
        <CardContent className='space-y-4 p-3 sm:p-5'>
          <Skeleton className='h-28 w-full' />
          <div className='grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3'>
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className='h-48 w-full' />
            ))}
          </div>
        </CardContent>
      </Card>
    )
  }

  if (plans.length === 0 && !contract && !currentSubscription) return null

  const migrationRestrictsPlanChanges =
    selfData.migration.requires_admin_review ||
    selfData.capabilities.has_migration_conflict
  const migrationRestrictionMessage = selfData.migration.requires_admin_review
    ? selfData.migration.reason ||
      t('Subscription migration requires administrator review.')
    : t('Migration Conflict')
  const graceEnd = Math.max(
    Number(selfData.current_period?.grace_period_end || 0),
    Number(contract?.grace_period_end || 0),
    Number(currentRecurring?.grace_period_end || 0)
  )

  return (
    <>
      <TitledCard
        title={t('Subscription Plans')}
        description={t('Manage one current subscription contract')}
        icon={<Crown className='h-4 w-4' />}
        contentClassName='space-y-4 sm:space-y-5'
      >
        <div className='rounded-xl border p-3 sm:p-4'>
          <div className='flex flex-wrap items-center justify-between gap-2.5 sm:gap-3'>
            <div className='flex min-w-0 flex-wrap items-center gap-2'>
              <span className='text-sm font-medium'>
                {t('Current subscription')}
              </span>
              <span className='flex items-center gap-1.5 text-xs font-medium'>
                <span
                  className={cn(
                    'size-1.5 shrink-0 rounded-full',
                    currentPlan ? dotColorMap.success : dotColorMap.neutral
                  )}
                  aria-hidden='true'
                />
                {currentPlan ? (
                  <span className={cn(textColorMap.success)}>
                    {currentPlan.title}
                  </span>
                ) : (
                  <span className='text-muted-foreground'>
                    {t('No Active')}
                  </span>
                )}
              </span>
            </div>
            <Button
              variant='ghost'
              size='icon'
              className='h-8 w-8'
              onClick={handleRefresh}
              disabled={refreshing}
            >
              <RefreshCw
                className={`h-3.5 w-3.5 ${refreshing ? 'animate-spin' : ''}`}
              />
            </Button>
          </div>

          {contract?.status === 'grace' && (
            <Alert className='mt-3'>
              <AlertTriangle className='h-4 w-4' />
              <AlertDescription>
                {t('Subscription is in grace period until {{date}}.', {
                  date: formatTimestamp(graceEnd),
                })}
              </AlertDescription>
            </Alert>
          )}

          {migrationRestrictsPlanChanges && (
            <Alert variant='destructive' className='mt-3'>
              <AlertDescription>{migrationRestrictionMessage}</AlertDescription>
            </Alert>
          )}

          {currentPlan &&
          (selfData.current_entitlement || currentSubscription) ? (
            <>
              <Separator className='my-3' />
              <div className='grid gap-3 text-xs sm:grid-cols-2'>
                <div>
                  <div className='text-muted-foreground'>{t('Remaining')}</div>
                  <div className='mt-1 text-sm font-medium'>
                    {!currentUsage.unlimited
                      ? formatQuota(currentUsage.remaining)
                      : t('Unlimited')}
                  </div>
                </div>
                <div>
                  <div className='text-muted-foreground'>
                    {t('Current quota')}
                  </div>
                  <div className='mt-1 text-sm font-medium'>
                    {!currentUsage.unlimited
                      ? `${formatQuota(currentUsage.used)}/${formatQuota(
                          currentUsage.total
                        )}`
                      : t('Unlimited')}
                  </div>
                </div>
                <div>
                  <div className='text-muted-foreground'>
                    {contract?.payment_mode === 'stripe_recurring'
                      ? t('Renewal time')
                      : t('End time')}
                  </div>
                  <div className='mt-1 text-sm font-medium'>
                    {formatTimestamp(
                      selfData.current_period?.end ||
                        contract?.current_period_end ||
                        selfData.current_entitlement?.end_time ||
                        currentSubscription?.subscription.end_time
                    )}
                  </div>
                </div>
                <div>
                  <div className='text-muted-foreground'>
                    {t('Payment mode')}
                  </div>
                  <div className='mt-1 text-sm font-medium'>
                    {contract?.payment_mode === 'stripe_recurring'
                      ? t('Stripe recurring')
                      : t('Balance one period')}
                  </div>
                </div>
              </div>
              {currentUsage.total > 0 && (
                <Progress value={currentUsage.percent} className='mt-3 h-1.5' />
              )}
              <p className='text-muted-foreground mt-3 text-xs'>
                {contract?.payment_mode === 'stripe_recurring'
                  ? t(
                      'Renews automatically. You can cancel or resume auto-renewal from this wallet.'
                    )
                  : t(
                      'Uses wallet balance for one period with no automatic renewal.'
                    )}
              </p>
              {pendingPlan && (
                <Alert className='mt-3'>
                  <AlertDescription>
                    {t('Downgrade to {{plan}} is scheduled for {{date}}.', {
                      plan: pendingPlan.title,
                      date: formatTimestamp(
                        selfData.pending_change?.effective_at ||
                          contract?.pending_effective_at
                      ),
                    })}
                  </AlertDescription>
                </Alert>
              )}
              {currentRecurring?.can_cancel || currentRecurring?.can_resume ? (
                <div className='mt-3'>
                  <Button
                    size='sm'
                    variant={
                      currentRecurring.can_cancel ? 'outline' : 'default'
                    }
                    disabled={
                      pendingRecurringBindingId === currentRecurring.binding_id
                    }
                    onClick={() =>
                      setRecurringAction({
                        action: currentRecurring.can_resume
                          ? 'resume'
                          : 'cancel',
                        subscription: currentRecurring,
                      })
                    }
                  >
                    {currentRecurring.can_cancel ? (
                      <PauseCircle className='mr-1 h-3.5 w-3.5' />
                    ) : (
                      <PlayCircle className='mr-1 h-3.5 w-3.5' />
                    )}
                    {currentRecurring.can_cancel
                      ? t('Cancel auto-renewal')
                      : t('Resume auto-renewal')}
                  </Button>
                </div>
              ) : null}
            </>
          ) : (
            <p className='text-muted-foreground mt-2 text-xs'>
              {t('Subscribe to a plan for model access')}
            </p>
          )}
        </div>

        {plans.length > 0 ? (
          <div className='grid grid-cols-1 gap-3 2xl:grid-cols-2 2xl:gap-4'>
            {plans.map((item, index) => {
              const plan = item.plan
              const totalAmount = Number(plan.total_amount || 0)
              const price = Number(plan.price_amount || 0).toFixed(2)
              const isPopular = index === 0 && plans.length > 1
              const allowedPaymentModes = getAllowedPaymentModes(
                plan,
                topupInfo,
                selfData.capabilities
              )
              const action = getDisplayedPlanAction(
                item,
                currentPlanId,
                allowedPaymentModes,
                selfData
              )
              const benefits = [
                `${t('Validity Period')}: ${formatDuration(plan, t)}`,
                formatResetPeriod(plan, t) !== t('No Reset')
                  ? `${t('Quota Reset')}: ${formatResetPeriod(plan, t)}`
                  : null,
                totalAmount > 0
                  ? `${t('Total Quota')}: ${formatQuota(totalAmount)}`
                  : `${t('Total Quota')}: ${t('Unlimited')}`,
                plan.upgrade_group
                  ? `${t('Upgrade Group')}: ${plan.upgrade_group}`
                  : null,
              ].filter(Boolean) as string[]

              return (
                <Card
                  key={plan.id}
                  className={cn(
                    'transition-shadow hover:shadow-md',
                    isPopular && 'border-primary/70 shadow-sm'
                  )}
                >
                  <CardContent className='flex h-full flex-col p-3.5 sm:p-4'>
                    <div className='mb-2 flex items-start justify-between gap-3'>
                      <div className='min-w-0'>
                        <h4 className='truncate font-semibold'>
                          {plan.title || t('Subscription Plans')}
                        </h4>
                        {plan.subtitle && (
                          <p className='text-muted-foreground truncate text-xs'>
                            {plan.subtitle}
                          </p>
                        )}
                      </div>
                      {isPopular && (
                        <StatusBadge
                          variant='info'
                          copyable={false}
                          className='shrink-0'
                        >
                          <Sparkles className='h-3 w-3' />
                          {t('Recommended')}
                        </StatusBadge>
                      )}
                    </div>

                    <div className='py-2'>
                      <span className='text-primary text-2xl font-bold'>
                        ${price}
                      </span>
                    </div>

                    <div className='flex-1 space-y-1.5 pb-3'>
                      {benefits.map((label) => (
                        <div
                          key={label}
                          className='text-muted-foreground flex items-center gap-2 text-xs'
                        >
                          <Check className='text-primary h-3 w-3 shrink-0' />
                          <span>{label}</span>
                        </div>
                      ))}
                    </div>

                    <Separator className='mb-3' />

                    {action === 'current' ? (
                      <Button variant='outline' className='w-full' disabled>
                        {t('Current plan')}
                      </Button>
                    ) : action === 'unavailable' ? (
                      <Tooltip>
                        <TooltipTrigger render={<div />}>
                          <Button variant='outline' className='w-full' disabled>
                            {t('Unavailable')}
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>
                          {migrationRestrictsPlanChanges
                            ? migrationRestrictionMessage
                            : t('No available payment mode')}
                        </TooltipContent>
                      </Tooltip>
                    ) : (
                      <Button
                        variant={
                          action === 'upgrade_now' ? 'default' : 'outline'
                        }
                        className='w-full'
                        onClick={() =>
                          setChangeTarget({
                            plan: item,
                            requestId: createStableSubscriptionRequestId(),
                          })
                        }
                      >
                        {action === 'upgrade_now'
                          ? t('Upgrade now')
                          : t('Downgrade next period')}
                      </Button>
                    )}
                  </CardContent>
                </Card>
              )
            })}
          </div>
        ) : (
          <p className='text-muted-foreground py-4 text-center text-sm'>
            {t('No plans available')}
          </p>
        )}
      </TitledCard>

      <ChangeSubscriptionPlanDialog
        key={changeTarget?.requestId || 'closed'}
        open={!!changeTarget}
        onOpenChange={(open) => {
          if (!open && !changing) setChangeTarget(null)
        }}
        plan={changeTarget?.plan || null}
        contract={contract}
        allowedPaymentModes={
          changeTarget
            ? getAllowedPaymentModes(
                changeTarget.plan.plan,
                topupInfo,
                selfData.capabilities
              )
            : []
        }
        defaultPaymentMode={
          changeTarget &&
          getAllowedPaymentModes(
            changeTarget.plan.plan,
            topupInfo,
            selfData.capabilities
          ).includes('stripe_recurring')
            ? 'stripe_recurring'
            : 'balance_one_period'
        }
        userQuota={userQuota}
        isLoading={changing}
        onConfirm={handleConfirmChange}
      />
      <RecurringSubscriptionActionDialog
        open={!!recurringAction}
        onOpenChange={(open) => !open && setRecurringAction(null)}
        action={recurringAction?.action || 'cancel'}
        subscription={recurringAction?.subscription || null}
        planTitle={
          recurringAction
            ? plans.find(
                (item) => item.plan.id === recurringAction.subscription.plan_id
              )?.plan.title
            : undefined
        }
        isLoading={
          !!recurringAction &&
          pendingRecurringBindingId === recurringAction.subscription.binding_id
        }
        onConfirm={handleRecurringAction}
      />
    </>
  )
}
