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
import { Loader2, WalletCards } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import type { PresetAmount, TopupInfo } from '../types'

interface RechargeFormCardProps {
  topupInfo: TopupInfo | null
  presetAmounts: PresetAmount[]
  selectedPreset: number | null
  onSelectPreset: (preset: PresetAmount) => void
  onStripeTopUp: (preset: PresetAmount) => void
  paymentLoadingAmount?: number | null
  loading?: boolean
}

type PlanCopy = {
  caption: string
  badge?: string
  discount?: string
  featured?: boolean
}

const WEBSITE_PLAN_COPY_BY_AMOUNT: Record<number, PlanCopy> = {
  10: {
    caption: 'Lowest entry to get started',
  },
  20: {
    caption: '3X more usage than the official plan',
    badge: 'Most Popular',
    discount: '40% OFF',
    featured: true,
  },
  200: {
    caption: '40X more usage than the official plan',
    discount: '50% OFF',
  },
}

const DEFAULT_PLAN_COPY: PlanCopy = {
  caption: 'Prepaid balance for top AI models',
}

function getPlanCopy(amount: number): PlanCopy {
  return WEBSITE_PLAN_COPY_BY_AMOUNT[amount] || DEFAULT_PLAN_COPY
}

function formatTopUpAmount(amount: number): string {
  return `$${formatNumber(amount)}`
}

export function RechargeFormCard(props: RechargeFormCardProps) {
  const { t } = useTranslation()
  const stripeEnabled =
    props.topupInfo?.enable_stripe_topup ||
    props.topupInfo?.pay_methods?.some((method) => method.type === 'stripe')

  if (props.loading) {
    return (
      <TitledCard
        title={t('Top-up Packages')}
        description={t('Choose a prepaid USD package and checkout with Stripe')}
        icon={<WalletCards className='h-4 w-4' />}
        contentClassName='space-y-4 sm:space-y-6'
      >
        <div className='grid gap-3 sm:grid-cols-3'>
          {Array.from({ length: 3 }).map((_, index) => (
            <Skeleton key={index} className='h-56 rounded-lg' />
          ))}
        </div>
      </TitledCard>
    )
  }

  const handleTopUpClick = (preset: PresetAmount): void => {
    props.onSelectPreset(preset)
    props.onStripeTopUp(preset)
  }

  return (
    <TitledCard
      title={t('Top-up Packages')}
      description={t('Choose a prepaid USD package and checkout with Stripe')}
      icon={<WalletCards className='h-4 w-4' />}
      contentClassName='space-y-4 sm:space-y-6'
    >
      {stripeEnabled && props.presetAmounts.length > 0 ? (
        <div className='grid gap-3 md:grid-cols-3'>
          {props.presetAmounts.map((preset) => {
            const planCopy = getPlanCopy(preset.value)
            const selected = props.selectedPreset === preset.value
            const loading = props.paymentLoadingAmount === preset.value

            return (
              <div
                key={preset.value}
                className={cn(
                  'bg-background relative flex min-h-[236px] flex-col rounded-lg border p-4 transition-colors',
                  selected
                    ? 'border-foreground bg-foreground/[0.03]'
                    : 'border-border',
                  planCopy.featured && 'border-primary/60 shadow-sm'
                )}
              >
                <div className='mb-3 flex min-h-6 items-center gap-2'>
                  {planCopy.badge ? (
                    <span className='bg-primary text-primary-foreground rounded-full px-2 py-0.5 text-[11px] font-semibold'>
                      {t(planCopy.badge)}
                    </span>
                  ) : null}
                  {planCopy.discount ? (
                    <span className='rounded-full border border-[#FF2D78]/30 bg-[#FF2D78]/10 px-2 py-0.5 text-[11px] font-semibold text-[#D80D5D] dark:text-[#FF78A9]'>
                      {t(planCopy.discount)}
                    </span>
                  ) : null}
                </div>

                <div className='space-y-2'>
                  <div className='text-3xl font-semibold tracking-normal'>
                    {formatTopUpAmount(preset.value)}
                  </div>
                  <p className='text-sm font-medium'>{t(planCopy.caption)}</p>
                  {preset.bonus && preset.bonus > 0 ? (
                    <p className='text-xs font-semibold text-[#FF2D78]'>
                      {t('Get {{bonus}} free', {
                        bonus: formatTopUpAmount(preset.bonus),
                      })}
                    </p>
                  ) : null}
                </div>

                <Button
                  className='mt-auto w-full'
                  onClick={() => handleTopUpClick(preset)}
                  disabled={!!props.paymentLoadingAmount}
                >
                  {loading ? (
                    <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                  ) : null}
                  {t('Top up for {{amount}}', {
                    amount: `$${formatNumber(preset.value)}`,
                  })}
                </Button>
              </div>
            )
          })}
        </div>
      ) : (
        <Alert>
          <AlertDescription>
            {stripeEnabled
              ? t('No top-up packages available. Please contact administrator.')
              : t(
                  'Stripe top-up is not enabled. Please contact administrator.'
                )}
          </AlertDescription>
        </Alert>
      )}
    </TitledCard>
  )
}
