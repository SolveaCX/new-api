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
import { useState, useEffect, useMemo, useCallback } from 'react'
import {
  Crown,
  RefreshCw,
  Sparkles,
  Check,
  ArrowRight,
  Boxes,
  Gauge,
  Layers,
  Settings2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
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
  getPublicPlans,
  getSelfSubscriptionFull,
  updateBillingPreference,
} from '@/features/subscriptions/api'
import { SubscriptionManageDialog } from '@/features/subscriptions/components/dialogs/subscription-manage-dialog'
import { SubscriptionPurchaseDialog } from '@/features/subscriptions/components/dialogs/subscription-purchase-dialog'
import { formatModelCount } from '@/features/subscriptions/lib'
import type {
  PlanRecord,
  UserSubscriptionRecord,
} from '@/features/subscriptions/types'
import { PAYMENT_TYPES } from '../constants'
import type { PaymentMethod, TopupInfo } from '../types'

interface SubscriptionPlansCardProps {
  topupInfo: TopupInfo | null
  onAvailabilityChange?: (available: boolean) => void
  userQuota?: number
  onPurchaseSuccess?: () => void | Promise<void>
}

function getEpayMethods(payMethods: PaymentMethod[] = []): PaymentMethod[] {
  return payMethods.filter(
    (m) =>
      m?.type &&
      m.type !== PAYMENT_TYPES.STRIPE &&
      m.type !== PAYMENT_TYPES.CREEM &&
      m.type !== PAYMENT_TYPES.PADDLE
  )
}

function getBillingPreferenceLabel(
  preference: string,
  t: (key: string) => string
): string {
  switch (preference) {
    case 'subscription_first':
      return t('Subscription First')
    case 'wallet_first':
      return t('Wallet First')
    case 'subscription_only':
      return t('Subscription Only')
    case 'wallet_only':
      return t('Wallet Only')
    default:
      return preference
  }
}

const PLAN_DISPLAY_ORDER: Record<string, number> = {
  go: 0,
  pro: 1,
  max: 2,
}

function getPlanDisplayOrder(title: string): number {
  return PLAN_DISPLAY_ORDER[title.trim().toLowerCase()] ?? 99
}

function formatPlanPrice(amount: number): string {
  return `$${Number.isInteger(amount) ? amount.toFixed(0) : amount.toFixed(2)}`
}

function getPlanAudience(title: string, t: (key: string) => string): string {
  switch (title.trim().toLowerCase()) {
    case 'go':
      return t('For individuals and light everyday use')
    case 'pro':
      return t('For daily development and frequent requests')
    case 'max':
      return t('For teams and high-intensity workloads')
    default:
      return ''
  }
}

export function SubscriptionPlansCard({
  topupInfo,
  onAvailabilityChange,
  userQuota,
  onPurchaseSuccess,
}: SubscriptionPlansCardProps) {
  const { t } = useTranslation()

  const [plans, setPlans] = useState<PlanRecord[]>([])
  const [activeSubscriptions, setActiveSubscriptions] = useState<
    UserSubscriptionRecord[]
  >([])
  const [allSubscriptions, setAllSubscriptions] = useState<
    UserSubscriptionRecord[]
  >([])
  const [billingPreference, setBillingPreference] =
    useState('subscription_first')
  const [subscriptionSnapshotTime, setSubscriptionSnapshotTime] = useState(0)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)

  const [purchaseOpen, setPurchaseOpen] = useState(false)
  const [selectedPlan, setSelectedPlan] = useState<PlanRecord | null>(null)
  const [manageOpen, setManageOpen] = useState(false)
  const [manageSub, setManageSub] = useState<UserSubscriptionRecord | null>(
    null
  )

  const enableStripe = !!topupInfo?.enable_stripe_topup
  const enableCreem = !!topupInfo?.enable_creem_topup
  const enableWaffoPancake = !!topupInfo?.enable_waffo_pancake_topup
  const enableOnlineTopUp = !!topupInfo?.enable_online_topup
  const epayMethods = useMemo(
    () => getEpayMethods(topupInfo?.pay_methods),
    [topupInfo?.pay_methods]
  )

  const fetchPlans = useCallback(async () => {
    try {
      const res = await getPublicPlans()
      if (res.success) {
        setPlans(res.data || [])
      }
    } catch {
      setPlans([])
    }
  }, [])

  const fetchSelfSubscription = useCallback(async () => {
    try {
      const res = await getSelfSubscriptionFull()
      if (res.success && res.data) {
        setSubscriptionSnapshotTime(Math.floor(Date.now() / 1000))
        setBillingPreference(
          res.data.billing_preference || 'subscription_first'
        )
        setActiveSubscriptions(res.data.subscriptions || [])
        setAllSubscriptions(res.data.all_subscriptions || [])
      }
    } catch {
      // ignore
    }
  }, [])

  useEffect(() => {
    const init = async () => {
      setLoading(true)
      await Promise.all([fetchPlans(), fetchSelfSubscription()])
      setLoading(false)
    }
    init()
  }, [fetchPlans, fetchSelfSubscription])

  const handleRefresh = async () => {
    setRefreshing(true)
    try {
      await fetchSelfSubscription()
    } finally {
      setRefreshing(false)
    }
  }

  const handlePreferenceChange = async (pref: string) => {
    const previous = billingPreference
    setBillingPreference(pref)
    try {
      const res = await updateBillingPreference(pref)
      if (res.success) {
        toast.success(t('Updated successfully'))
        const normalized = res.data?.billing_preference || pref
        setBillingPreference(normalized)
      } else {
        toast.error(res.message || t('Update failed'))
        setBillingPreference(previous)
      }
    } catch {
      toast.error(t('Request failed'))
      setBillingPreference(previous)
    }
  }

  const hasActive = activeSubscriptions.length > 0
  const hasAny = allSubscriptions.length > 0
  const isAvailable = loading || plans.length > 0 || hasAny
  const disablePref = !hasActive
  const isSubPref =
    billingPreference === 'subscription_first' ||
    billingPreference === 'subscription_only'
  const displayPref =
    disablePref && isSubPref ? 'wallet_first' : billingPreference

  const planPurchaseCountMap = useMemo(() => {
    const map = new Map<number, number>()
    for (const sub of allSubscriptions) {
      const planId = sub?.subscription?.plan_id
      if (!planId) continue
      map.set(planId, (map.get(planId) || 0) + 1)
    }
    return map
  }, [allSubscriptions])

  const orderedPlans = useMemo(
    () =>
      [...plans].sort((a, b) => {
        const orderDiff =
          getPlanDisplayOrder(a?.plan?.title || '') -
          getPlanDisplayOrder(b?.plan?.title || '')
        if (orderDiff !== 0) return orderDiff
        return (
          Number(a?.plan?.price_amount || 0) -
          Number(b?.plan?.price_amount || 0)
        )
      }),
    [plans]
  )

  useEffect(() => {
    onAvailabilityChange?.(isAvailable)
  }, [isAvailable, onAvailabilityChange])

  const planTitleMap = useMemo(() => {
    const map = new Map<number, string>()
    for (const p of plans) {
      if (p?.plan?.id) {
        map.set(p.plan.id, p.plan.title || '')
      }
    }
    return map
  }, [plans])

  const planById = useMemo(() => {
    const map = new Map<number, PlanRecord>()
    for (const p of plans) {
      if (p?.plan?.id) map.set(p.plan.id, p)
    }
    return map
  }, [plans])

  // 当前用户已持有的套餐 id 集合（用于在套餐网格标记「当前档」）
  const ownedPlanIds = useMemo(() => {
    const set = new Set<number>()
    for (const sub of activeSubscriptions) {
      const planId = sub?.subscription?.plan_id
      if (planId) set.add(planId)
    }
    return set
  }, [activeSubscriptions])

  const preferenceOptions = useMemo(
    () => [
      {
        value: 'subscription_first',
        label:
          getBillingPreferenceLabel('subscription_first', t) +
          (disablePref ? ` (${t('No Active')})` : ''),
      },
      {
        value: 'wallet_first',
        label: getBillingPreferenceLabel('wallet_first', t),
      },
      {
        value: 'subscription_only',
        label:
          getBillingPreferenceLabel('subscription_only', t) +
          (disablePref ? ` (${t('No Active')})` : ''),
      },
      {
        value: 'wallet_only',
        label: getBillingPreferenceLabel('wallet_only', t),
      },
    ],
    [t, disablePref]
  )

  const openManage = (sub: UserSubscriptionRecord) => {
    setManageSub(sub)
    setManageOpen(true)
  }

  const openPurchase = (p: PlanRecord) => {
    setSelectedPlan(p)
    setManageOpen(false)
    setPurchaseOpen(true)
  }

  const getRemainingDays = (sub: UserSubscriptionRecord) => {
    const endTime = sub?.subscription?.end_time || 0
    if (!endTime || !subscriptionSnapshotTime) return 0
    return Math.max(0, Math.ceil((endTime - subscriptionSnapshotTime) / 86400))
  }

  const getUsagePercent = (sub: UserSubscriptionRecord) => {
    const total = Number(sub?.subscription?.amount_total || 0)
    const used = Number(sub?.subscription?.amount_used || 0)
    if (total <= 0) return 0
    return Math.round((used / total) * 100)
  }

  if (loading) {
    return (
      <Card className='gap-0 overflow-hidden py-0'>
        <CardHeader className='border-b p-3 !pb-3 sm:p-5 sm:!pb-5'>
          <Skeleton className='h-6 w-32' />
        </CardHeader>
        <CardContent className='space-y-4 p-3 sm:p-5'>
          <Skeleton className='h-20 w-full' />
          <div className='grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3'>
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className='h-48 w-full' />
            ))}
          </div>
        </CardContent>
      </Card>
    )
  }

  if (plans.length === 0 && !hasAny) {
    return null
  }

  const entryPlan = orderedPlans[0]?.plan
  const entryPrice = entryPlan
    ? formatPlanPrice(Number(entryPlan.price_amount || 0))
    : ''
  const entryValue = entryPlan
    ? formatQuota(Number(entryPlan.total_amount || 0))
    : ''

  return (
    <>
      <TitledCard
        title={t('Subscription Plans')}
        description={t(
          'All plans include the same models. Choose based on included usage and request speed.'
        )}
        icon={<Crown className='h-4 w-4' />}
        iconClassName='bg-primary/10 text-primary'
        contentClassName='flex flex-col gap-4 sm:gap-5'
      >
        {entryPlan && (
          <div className='from-primary/15 via-primary/5 border-primary/20 order-1 overflow-hidden rounded-xl border bg-gradient-to-r to-transparent p-4 sm:p-5'>
            <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
              <div>
                <div className='text-primary text-xs font-semibold tracking-wide uppercase'>
                  {t('More model usage for one fixed monthly price')}
                </div>
                <div className='mt-1.5 flex flex-wrap items-center gap-2 text-xl font-bold tracking-tight sm:text-2xl'>
                  <span>
                    {entryPrice} · {t('Monthly')}
                  </span>
                  <ArrowRight
                    className='text-primary h-5 w-5'
                    aria-hidden='true'
                  />
                  <span className='text-primary'>
                    {t('Up to {{value}} in model usage', {
                      value: entryValue,
                    })}
                  </span>
                </div>
                <p className='text-muted-foreground mt-1.5 max-w-3xl text-xs sm:text-sm'>
                  {t(
                    'Every plan includes all available models. Higher plans add more usage and let you send more requests faster.'
                  )}
                </p>
              </div>
              <div className='border-primary/20 bg-background/70 text-primary shrink-0 rounded-lg border px-3 py-2 text-xs font-medium'>
                {t('Start with Go')}
              </div>
            </div>
          </div>
        )}

        {/* My subscriptions & billing preference */}
        <div className='order-3 rounded-xl border p-3 sm:p-4'>
          <div className='flex flex-wrap items-center justify-between gap-2.5 sm:gap-3'>
            <div className='flex min-w-0 flex-wrap items-center gap-2'>
              <span className='text-sm font-medium'>
                {t('My Subscriptions')}
              </span>
              <span className='flex items-center gap-1.5 text-xs font-medium'>
                <span
                  className={cn(
                    'size-1.5 shrink-0 rounded-full',
                    hasActive ? dotColorMap.success : dotColorMap.neutral
                  )}
                  aria-hidden='true'
                />
                {hasActive ? (
                  <span className={cn(textColorMap.success)}>
                    {activeSubscriptions.length} {t('active')}
                  </span>
                ) : (
                  <span className='text-muted-foreground'>
                    {t('No Active')}
                  </span>
                )}
                {allSubscriptions.length > activeSubscriptions.length && (
                  <>
                    <span className='text-muted-foreground/30'>·</span>
                    <span className='text-muted-foreground'>
                      {allSubscriptions.length - activeSubscriptions.length}{' '}
                      {t('expired')}
                    </span>
                  </>
                )}
              </span>
            </div>
            <div className='flex w-full items-center gap-2 sm:w-auto'>
              <Select
                items={[
                  {
                    value: 'subscription_first',
                    label: (
                      <>
                        {getBillingPreferenceLabel('subscription_first', t)}
                        {disablePref ? ` (${t('No Active')})` : ''}
                      </>
                    ),
                  },
                  {
                    value: 'wallet_first',
                    label: getBillingPreferenceLabel('wallet_first', t),
                  },
                  {
                    value: 'subscription_only',
                    label: (
                      <>
                        {getBillingPreferenceLabel('subscription_only', t)}
                        {disablePref ? ` (${t('No Active')})` : ''}
                      </>
                    ),
                  },
                  {
                    value: 'wallet_only',
                    label: getBillingPreferenceLabel('wallet_only', t),
                  },
                ]}
                value={displayPref}
                onValueChange={(v) => v !== null && handlePreferenceChange(v)}
              >
                <SelectTrigger className='h-8 flex-1 text-xs sm:w-[140px] sm:flex-none'>
                  <SelectValue>
                    {getBillingPreferenceLabel(displayPref, t)}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem
                      value='subscription_first'
                      disabled={disablePref}
                    >
                      {getBillingPreferenceLabel('subscription_first', t)}
                      {disablePref ? ` (${t('No Active')})` : ''}
                    </SelectItem>
                    <SelectItem value='wallet_first'>
                      {getBillingPreferenceLabel('wallet_first', t)}
                    </SelectItem>
                    <SelectItem
                      value='subscription_only'
                      disabled={disablePref}
                    >
                      {getBillingPreferenceLabel('subscription_only', t)}
                      {disablePref ? ` (${t('No Active')})` : ''}
                    </SelectItem>
                    <SelectItem value='wallet_only'>
                      {getBillingPreferenceLabel('wallet_only', t)}
                    </SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
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
          </div>

          {disablePref && isSubPref && (
            <p className='text-muted-foreground mt-2 text-xs'>
              {t(
                'Preference saved as {{pref}}, but no active subscription. Wallet will be used automatically.',
                {
                  pref:
                    billingPreference === 'subscription_only'
                      ? t('Subscription Only')
                      : t('Subscription First'),
                }
              )}
            </p>
          )}

          {hasAny && (
            <>
              <Separator className='my-3' />
              <div className='max-h-64 space-y-3 overflow-y-auto pr-1'>
                {allSubscriptions.map((sub) => {
                  const subscription = sub.subscription
                  const totalAmount = Number(subscription?.amount_total || 0)
                  const usedAmount = Number(subscription?.amount_used || 0)
                  const remainAmount =
                    totalAmount > 0 ? Math.max(0, totalAmount - usedAmount) : 0
                  const planTitle =
                    planTitleMap.get(subscription?.plan_id) ||
                    (subscription?.source === 'free' ? 'Free' : '')
                  const remainDays = getRemainingDays(sub)
                  const usagePercent = getUsagePercent(sub)
                  const isExpired =
                    subscriptionSnapshotTime > 0 &&
                    (subscription?.end_time || 0) < subscriptionSnapshotTime
                  const isCancelled = subscription?.status === 'cancelled'
                  const isActive =
                    subscription?.status === 'active' && !isExpired

                  return (
                    <div
                      key={subscription?.id}
                      className='bg-background rounded-md border p-3 text-xs'
                    >
                      <div className='flex items-center justify-between'>
                        <div className='flex items-center gap-2'>
                          <span className='font-medium'>
                            {planTitle
                              ? `${planTitle} · ${t('Subscription')} #${subscription?.id}`
                              : `${t('Subscription')} #${subscription?.id}`}
                          </span>
                          {isActive ? (
                            <StatusBadge
                              label={t('Active')}
                              variant='success'
                              copyable={false}
                            />
                          ) : isCancelled ? (
                            <StatusBadge
                              label={t('Cancelled')}
                              variant='neutral'
                              copyable={false}
                            />
                          ) : (
                            <StatusBadge
                              label={t('Expired')}
                              variant='neutral'
                              copyable={false}
                            />
                          )}
                        </div>
                        {isActive && (
                          <div className='flex items-center gap-2'>
                            <span className='text-muted-foreground'>
                              {t('{{count}} days remaining', {
                                count: remainDays,
                              })}
                            </span>
                            <Button
                              variant='outline'
                              size='sm'
                              className='h-7 px-2 text-xs'
                              onClick={() => openManage(sub)}
                            >
                              <Settings2 className='mr-1 h-3 w-3' />
                              {t('Manage')}
                            </Button>
                          </div>
                        )}
                      </div>
                      <div className='text-muted-foreground mt-1.5'>
                        {isActive
                          ? t('Until')
                          : isCancelled
                            ? t('Cancelled at')
                            : t('Expired at')}{' '}
                        {new Date(
                          (subscription?.end_time || 0) * 1000
                        ).toLocaleString()}
                      </div>
                      {isActive && (subscription?.next_reset_time ?? 0) > 0 && (
                        <div className='text-muted-foreground mt-1'>
                          {t('Next reset')}:{' '}
                          {new Date(
                            subscription!.next_reset_time! * 1000
                          ).toLocaleString()}
                        </div>
                      )}
                      <div className='text-muted-foreground mt-1'>
                        {t('Total Quota')}:{' '}
                        {totalAmount > 0 ? (
                          <Tooltip>
                            <TooltipTrigger
                              render={<span className='cursor-help' />}
                            >
                              {formatQuota(usedAmount)}/
                              {formatQuota(totalAmount)} · {t('Remaining')}{' '}
                              {formatQuota(remainAmount)}
                            </TooltipTrigger>
                            <TooltipContent>
                              {t('Raw Quota')}: {usedAmount}/{totalAmount} ·{' '}
                              {t('Remaining')} {remainAmount}
                            </TooltipContent>
                          </Tooltip>
                        ) : (
                          t('Unlimited')
                        )}
                        {totalAmount > 0 && (
                          <span className='ml-2'>
                            {t('Used')} {usagePercent}%
                          </span>
                        )}
                      </div>
                      {totalAmount > 0 && isActive && (
                        <Progress value={usagePercent} className='mt-2 h-1.5' />
                      )}
                    </div>
                  )
                })}
              </div>
            </>
          )}

          {!hasAny && (
            <p className='text-muted-foreground mt-2 text-xs'>
              {t('Subscribe to a plan for model access')}
            </p>
          )}
        </div>

        {/* Available plans grid — 价值突出：模型数 + 速度 + 窗口 + 卖点 */}
        {plans.length > 0 ? (
          <div className='order-2 grid grid-cols-1 gap-3 md:grid-cols-3 xl:gap-4'>
            {orderedPlans.map((p) => {
              const plan = p?.plan
              if (!plan) return null
              const totalAmount = Number(plan.total_amount || 0)
              const price = formatPlanPrice(Number(plan.price_amount || 0))
              const includedValue =
                totalAmount > 0 ? formatQuota(totalAmount) : t('Unlimited')
              const isRecommended =
                plan.title.trim().toLowerCase() === 'pro' && plans.length > 1
              const limit = Number(plan.max_purchase_per_user || 0)
              const count = planPurchaseCountMap.get(plan.id) || 0
              const reached = limit > 0 && count >= limit
              const isOwned = ownedPlanIds.has(plan.id)
              const rpm = Number(plan.rpm || 0)
              const concurrency = Number(plan.concurrency || 0)
              const window5h = Number(plan.window_5h_amount || 0)
              const windowWeek = Number(plan.window_week_amount || 0)
              const audience =
                getPlanAudience(plan.title, t) || plan.subtitle || ''

              return (
                <Card
                  key={plan.id}
                  className={cn(
                    'relative transition-shadow hover:shadow-md',
                    isRecommended && 'border-primary shadow-md',
                    isOwned && 'border-emerald-500/60'
                  )}
                >
                  <CardContent className='flex h-full flex-col p-4'>
                    <div className='mb-3 flex items-start justify-between gap-3'>
                      <div className='min-w-0'>
                        <h4 className='text-lg font-semibold'>
                          {plan.title || t('Subscription Plans')}
                        </h4>
                        {audience && (
                          <p className='text-muted-foreground mt-0.5 text-xs'>
                            {audience}
                          </p>
                        )}
                      </div>
                      {isOwned ? (
                        <StatusBadge
                          variant='success'
                          copyable={false}
                          className='shrink-0'
                        >
                          <Check className='h-3 w-3' />
                          {t('Current')}
                        </StatusBadge>
                      ) : (
                        isRecommended && (
                          <StatusBadge
                            variant='info'
                            copyable={false}
                            className='shrink-0'
                          >
                            <Sparkles className='h-3 w-3' />
                            {t('Recommended')}
                          </StatusBadge>
                        )
                      )}
                    </div>

                    {/* 价格 → 可用价值是卡片的第一视觉重点 */}
                    <div
                      className={cn(
                        'mb-3 grid grid-cols-[1fr_auto_1.25fr] items-center gap-2 rounded-xl border p-3',
                        isRecommended
                          ? 'border-primary/30 bg-primary/10'
                          : 'bg-muted/35'
                      )}
                    >
                      <div>
                        <div className='text-muted-foreground text-[11px]'>
                          {t('Monthly price')}
                        </div>
                        <div className='text-xl font-bold'>{price}</div>
                      </div>
                      <ArrowRight
                        className='text-primary h-4 w-4'
                        aria-hidden='true'
                      />
                      <div>
                        <div className='text-muted-foreground text-[11px]'>
                          {t('Included model usage')}
                        </div>
                        <div className='text-primary text-lg font-bold'>
                          {t('Up to {{value}}', { value: includedValue })}
                        </div>
                      </div>
                    </div>

                    {/* 共同模型 + 白话速度差异 */}
                    <div className='flex-1 space-y-2 pb-3 text-xs'>
                      <div className='flex items-center gap-2'>
                        <Boxes className='text-primary h-3.5 w-3.5 shrink-0' />
                        <span>
                          {t('Includes {{models}}', {
                            models: formatModelCount(plan, t),
                          })}
                        </span>
                      </div>
                      {rpm > 0 && (
                        <div className='flex items-center gap-2'>
                          <Gauge className='text-primary h-3.5 w-3.5 shrink-0' />
                          <span>
                            {t('Up to {{count}} requests per minute', {
                              count: rpm,
                            })}
                          </span>
                        </div>
                      )}
                      {concurrency > 0 && (
                        <div className='flex items-center gap-2'>
                          <Layers className='text-primary h-3.5 w-3.5 shrink-0' />
                          <span>
                            {t(
                              'Run up to {{count}} requests at the same time',
                              {
                                count: concurrency,
                              }
                            )}
                          </span>
                        </div>
                      )}
                      {(window5h > 0 || windowWeek > 0) && (
                        <div className='bg-muted/40 text-muted-foreground mt-2 rounded-lg px-2.5 py-2 text-[11px] leading-relaxed'>
                          {t(
                            'Short-term usage limits: {{fiveHour}} per 5 hours · {{weekly}} per 7 days',
                            {
                              fiveHour:
                                window5h > 0 ? formatQuota(window5h) : '—',
                              weekly:
                                windowWeek > 0 ? formatQuota(windowWeek) : '—',
                            }
                          )}
                        </div>
                      )}
                    </div>

                    <Separator className='mb-3' />

                    {reached ? (
                      <Tooltip>
                        <TooltipTrigger render={<div />}>
                          <Button variant='outline' className='w-full' disabled>
                            {t('Limit Reached')}
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>
                          {t('Purchase limit reached')} ({count}/{limit})
                        </TooltipContent>
                      </Tooltip>
                    ) : (
                      <Button
                        variant={
                          isRecommended && !isOwned ? 'default' : 'outline'
                        }
                        className='w-full'
                        onClick={() => openPurchase(p)}
                      >
                        {isOwned ? t('Buy Again') : t('Subscribe Now')}
                      </Button>
                    )}
                  </CardContent>
                </Card>
              )
            })}
          </div>
        ) : (
          <p className='text-muted-foreground order-2 py-4 text-center text-sm'>
            {t('No plans available')}
          </p>
        )}
      </TitledCard>

      <SubscriptionPurchaseDialog
        open={purchaseOpen}
        onOpenChange={(open) => {
          setPurchaseOpen(open)
          if (!open) {
            fetchSelfSubscription()
          }
        }}
        plan={selectedPlan}
        enableStripe={enableStripe}
        enableCreem={enableCreem}
        enableWaffoPancake={enableWaffoPancake}
        enableOnlineTopUp={enableOnlineTopUp}
        epayMethods={epayMethods}
        userQuota={userQuota}
        onPurchaseSuccess={onPurchaseSuccess}
        purchaseLimit={
          selectedPlan?.plan?.max_purchase_per_user
            ? Number(selectedPlan.plan.max_purchase_per_user)
            : undefined
        }
        purchaseCount={
          selectedPlan?.plan?.id
            ? planPurchaseCountMap.get(selectedPlan.plan.id)
            : undefined
        }
      />

      <SubscriptionManageDialog
        open={manageOpen}
        onOpenChange={setManageOpen}
        currentSub={manageSub}
        currentPlanTitle={
          manageSub?.subscription?.plan_id
            ? planTitleMap.get(manageSub.subscription.plan_id) ||
              (manageSub.subscription.source === 'free' ? 'Free' : '')
            : ''
        }
        currentPlan={
          manageSub?.subscription?.plan_id
            ? planById.get(manageSub.subscription.plan_id) || null
            : null
        }
        plans={orderedPlans}
        snapshotTime={subscriptionSnapshotTime}
        billingPreference={displayPref}
        preferenceOptions={preferenceOptions}
        preferenceLabel={(pref) => getBillingPreferenceLabel(pref, t)}
        onPreferenceChange={handlePreferenceChange}
        onSelectPlan={openPurchase}
        purchaseCountMap={planPurchaseCountMap}
      />
    </>
  )
}
