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
import { Crown, RefreshCw, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  getPublicPlans,
  getSelfSubscriptionFull,
} from '@/features/subscriptions/api'
import { SubscriptionPurchaseDialog } from '@/features/subscriptions/components/dialogs/subscription-purchase-dialog'
import { formatMediaValue } from '@/features/subscriptions/lib'
import type {
  CurrentSubscriptionRecord,
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

function usagePercent(used: number, limit: number): number {
  if (limit <= 0) return 0
  return Math.min(100, Math.max(0, Math.round((used / limit) * 100)))
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
  const [currentSubscription, setCurrentSubscription] =
    useState<CurrentSubscriptionRecord | null>(null)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)

  const [purchaseOpen, setPurchaseOpen] = useState(false)
  const [selectedPlan, setSelectedPlan] = useState<PlanRecord | null>(null)

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
        setCurrentSubscription(res.data.current_subscription || null)
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

  const hasAny = allSubscriptions.length > 0
  const isAvailable = loading || plans.length > 0 || hasAny

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

  // 当前用户已持有的套餐 id 集合（用于在套餐网格标记「当前档」）
  const ownedPlanIds = useMemo(() => {
    const set = new Set<number>()
    for (const sub of activeSubscriptions) {
      const planId = sub?.subscription?.plan_id
      if (planId) set.add(planId)
    }
    return set
  }, [activeSubscriptions])

  const openPurchase = (p: PlanRecord) => {
    setSelectedPlan(p)
    setPurchaseOpen(true)
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

  const currentSub = currentSubscription?.subscription
  const currentPlan = currentSubscription?.plan
  const currentLimits = currentSubscription?.usage_limits
  const monthlyUsed = Number(currentSub?.amount_used || 0)
  const monthlyLimit = Number(currentSub?.amount_total || 0)
  const monthlyResetDetail =
    (currentSub?.next_reset_time || currentSub?.end_time || 0) > 0
      ? t('Resets {{time}}', {
          time: new Date(
            (currentSub?.next_reset_time || currentSub?.end_time || 0) * 1000
          ).toLocaleString(),
        })
      : ''
  interface UsageRow {
    key: string
    label: string
    used: number
    limit: number
    detail: string
    format?: (v: number) => string
  }
  const textUsageRows: UsageRow[] = currentSubscription
    ? [
        {
          key: 'monthly',
          label: t('Monthly usage'),
          used: monthlyUsed,
          limit: monthlyLimit,
          detail: monthlyResetDetail,
        },
        {
          key: 'five-hour',
          label: t('Rolling 5-hour usage'),
          used: Number(currentLimits?.window_5h_used || 0),
          limit: Number(currentPlan?.window_5h_amount || 0),
          detail: t('Updates continuously'),
        },
        {
          key: 'weekly',
          label: t('7-day usage'),
          used: Number(currentLimits?.window_week_used || 0),
          limit: Number(currentPlan?.window_week_amount || 0),
          detail:
            Number(currentLimits?.window_week_reset_at || 0) > 0
              ? t('Resets {{time}}', {
                  time: new Date(
                    Number(currentLimits?.window_week_reset_at || 0) * 1000
                  ).toLocaleString(),
                })
              : '',
        },
      ].filter((item) => item.limit > 0)
    : []
  const mediaUsageRows: UsageRow[] = currentSubscription
    ? [
        {
          key: 'media',
          label: t('Media credits'),
          used: Number(currentSub?.media_credits_used || 0),
          limit: Number(currentSub?.media_credits_total || 0),
          detail: monthlyResetDetail,
          // Media pool is counted in credits, not quota units.
          format: (v: number) => String(v),
        },
      ].filter((item) => item.limit > 0)
    : []

  const renderUsageRow = (item: UsageRow) => {
    const percent = usagePercent(item.used, item.limit)
    return (
      <div key={item.key} className='min-w-0'>
        <div className='flex items-center justify-between gap-3 text-xs'>
          <span className='font-medium'>{item.label}</span>
          <span className='text-muted-foreground tabular-nums'>
            {percent}% {t('used')}
          </span>
        </div>
        <Progress
          value={percent}
          className='mt-2 h-2 bg-[#e5eefb] dark:bg-white/10 [&_[data-slot=progress-indicator]]:bg-[#5b21b6]'
        />
        <div className='text-muted-foreground mt-1.5 flex items-center justify-between gap-3 text-[11px]'>
          <span>
            {(item.format ?? formatQuota)(item.used)} /{' '}
            {(item.format ?? formatQuota)(item.limit)}
          </span>
          <span className='truncate'>{item.detail}</span>
        </div>
      </div>
    )
  }

  return (
    <>
      <TitledCard
        title={t('Subscription Plans')}
        description={t(
          'One key, 328+ frontier models: GPT, Claude, Gemini, DeepSeek, GLM for text, plus Seedance 2.5 and more for image & video generation.'
        )}
        icon={<Crown className='h-4 w-4' />}
        iconClassName='bg-[#f0ebfa] text-[#4c1d95] dark:bg-[#5b21b6]/25 dark:text-[#c4b5fd]'
        contentClassName='flex flex-col gap-4 sm:gap-5'
      >
        {/* One current plan, one predictable billing order, three usage limits. */}
        <div className='order-3 rounded-xl border p-3 sm:p-4'>
          <div className='flex flex-wrap items-start justify-between gap-3'>
            <div>
              <div className='flex flex-wrap items-center gap-2'>
                <span className='text-sm font-semibold'>
                  {t('Current Plan')}
                </span>
                {currentPlan && (
                  <span className='text-base font-bold'>
                    {currentPlan.title}
                  </span>
                )}
              </div>
            </div>
            <Button
              variant='ghost'
              size='icon'
              className='h-8 w-8'
              onClick={handleRefresh}
              disabled={refreshing}
              aria-label={t('Refresh')}
            >
              <RefreshCw
                className={`h-3.5 w-3.5 ${refreshing ? 'animate-spin' : ''}`}
              />
            </Button>
          </div>

          {currentSubscription ? (
            <div className='mt-4 space-y-4'>
              {textUsageRows.length > 0 && (
                <div>
                  <p className='text-muted-foreground text-[10px] font-semibold tracking-widest uppercase'>
                    {t('Text models')}
                  </p>
                  <div className='mt-2 grid gap-4 md:grid-cols-3'>
                    {textUsageRows.map(renderUsageRow)}
                  </div>
                </div>
              )}
              {mediaUsageRows.length > 0 && (
                <div className='border-t pt-3'>
                  <p className='text-muted-foreground text-[10px] font-semibold tracking-widest uppercase'>
                    {t('Image & video models')}
                  </p>
                  <div className='mt-2 grid gap-4 md:grid-cols-3'>
                    {mediaUsageRows.map(renderUsageRow)}
                  </div>
                </div>
              )}
            </div>
          ) : (
            <p className='text-muted-foreground mt-3 text-xs'>
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
                plan.title.trim().toLowerCase() === 'go' && plans.length > 1
              const limit = Number(plan.max_purchase_per_user || 0)
              const count = planPurchaseCountMap.get(plan.id) || 0
              const reached = limit > 0 && count >= limit
              // Only the primary plan (highest-priced active subscription) is
              // "current"; other still-active lower tiers show as owned.
              const isCurrent = currentPlan?.id === plan.id
              const isOwned = ownedPlanIds.has(plan.id)
              const window5h = Number(plan.window_5h_amount || 0)
              const windowWeek = Number(plan.window_week_amount || 0)
              const mediaCredits = Number(plan.media_credits_monthly || 0)
              const audience =
                getPlanAudience(plan.title, t) || plan.subtitle || ''

              return (
                <Card
                  key={plan.id}
                  className={cn(
                    'border-border/70 relative transition-colors hover:border-[#8b5cf6]/45',
                    isRecommended &&
                      'border-[#8b5cf6]/45 shadow-[0_10px_30px_-28px_rgba(91,33,182,0.8)]'
                  )}
                >
                  <CardContent className='flex h-full flex-col p-5'>
                    <div className='flex items-start justify-between gap-3'>
                      <div className='min-w-0'>
                        <h4 className='text-xl font-semibold'>
                          {plan.title || t('Subscription Plans')}
                        </h4>
                        {audience && (
                          <p className='text-muted-foreground mt-0.5 text-xs'>
                            {audience}
                          </p>
                        )}
                      </div>
                      {isRecommended && (
                        <span className='inline-flex shrink-0 items-center gap-1 rounded-full bg-[#f0ebfa] px-2 py-1 text-[11px] font-semibold text-[#4c1d95] dark:bg-[#5b21b6]/25 dark:text-[#c4b5fd]'>
                          <Sparkles className='h-3 w-3' />
                          {t('Recommended')}
                        </span>
                      )}
                    </div>

                    <div className='mt-8 flex items-end gap-2'>
                      <span className='text-5xl font-semibold tracking-tight tabular-nums'>
                        {price}
                      </span>
                      <span className='text-muted-foreground mb-1 text-sm'>
                        {t('per month')}
                      </span>
                    </div>
                    <div className='mt-4 rounded-xl bg-[#f7f5fc] p-3.5 dark:bg-white/5'>
                      <p className='text-muted-foreground text-[10px] font-semibold tracking-widest uppercase'>
                        {t('Text models')}
                      </p>
                      <p className='mt-1 text-[15px] leading-snug font-bold text-[#5b21b6] dark:text-[#a78bfa]'>
                        {t('Up to {{value}} in model usage', {
                          value: includedValue,
                        })}
                      </p>
                      {(window5h > 0 || windowWeek > 0) && (
                        <p className='text-muted-foreground mt-0.5 text-xs'>
                          {t(
                            'Short-term cap: {{fiveHour}} / 5 h · {{weekly}} / 7 days',
                            {
                              fiveHour:
                                window5h > 0 ? formatQuota(window5h) : '—',
                              weekly:
                                windowWeek > 0 ? formatQuota(windowWeek) : '—',
                            }
                          )}
                        </p>
                      )}
                      {mediaCredits > 0 && (
                        <div className='mt-3 border-t border-[#5b21b6]/10 pt-3 dark:border-white/10'>
                          <p className='text-muted-foreground text-[10px] font-semibold tracking-widest uppercase'>
                            {t('Image & video models')}
                          </p>
                          <p className='mt-1 text-[15px] leading-snug font-bold text-[#5b21b6] dark:text-[#a78bfa]'>
                            {t('{{count}} media credits / month', {
                              count: mediaCredits,
                            })}
                          </p>
                          <p className='text-muted-foreground mt-0.5 text-xs'>
                            {formatMediaValue(mediaCredits, t)}
                          </p>
                        </div>
                      )}
                    </div>

                    <div className='flex-1' />

                    <div className='mt-5'>
                      {isCurrent ? (
                        <Button className='w-full' variant='secondary' disabled>
                          {t('Current Plan')}
                        </Button>
                      ) : isOwned ? (
                        <Button className='w-full' variant='secondary' disabled>
                          {t('Owned')}
                        </Button>
                      ) : reached ? (
                        <Tooltip>
                          <TooltipTrigger render={<div />}>
                            <Button
                              variant='outline'
                              className='w-full'
                              disabled
                            >
                              {t('Limit Reached')}
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent>
                            {t('Purchase limit reached')} ({count}/{limit})
                          </TooltipContent>
                        </Tooltip>
                      ) : (
                        <Button
                          variant={isRecommended ? 'default' : 'outline'}
                          className={cn(
                            'w-full',
                            isRecommended &&
                              'bg-[#070707] text-white hover:bg-[#4c1d95] dark:bg-white dark:text-black dark:hover:bg-[#ddd6fe]'
                          )}
                          onClick={() => openPurchase(p)}
                        >
                          {t('Subscribe Now')}
                        </Button>
                      )}
                    </div>
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
    </>
  )
}
