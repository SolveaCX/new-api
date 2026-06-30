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
import { ExternalLink, Gift, Loader2, WalletCards } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
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
  redemptionCode: string
  onRedemptionCodeChange: (code: string) => void
  onRedeem: () => void
  redeeming: boolean
  topupLink?: string
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

function formatUsdAmount(amount: number): string {
  return `$${formatNumber(amount)} USD`
}

export function RechargeFormCard(props: RechargeFormCardProps) {
  const { t } = useTranslation()
  const stripeEnabled =
    props.topupInfo?.enable_stripe_topup ||
    props.topupInfo?.pay_methods?.some((method) => method.type === 'stripe')
  const redemptionEnabled = props.topupInfo?.enable_redemption !== false

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
                    {formatUsdAmount(preset.value)}
                  </div>
                  <p className='text-sm font-medium'>{t(planCopy.caption)}</p>
                  {preset.bonus && preset.bonus > 0 ? (
                    <p className='text-xs font-semibold text-[#FF2D78]'>
                      {t('Get {{bonus}} free', {
                        bonus: formatUsdAmount(preset.bonus),
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

      {redemptionEnabled ? (
        <div className='space-y-2.5 border-t pt-4 sm:space-y-3 sm:pt-6'>
          <div className='flex items-center gap-2'>
            <Gift className='text-muted-foreground h-4 w-4' />
            <Label
              htmlFor='redemption-code'
              className='text-muted-foreground text-xs font-medium tracking-wider uppercase'
            >
              {t('Have a Code?')}
            </Label>
          </div>
          <div className='grid grid-cols-[minmax(0,1fr)_auto] gap-2'>
            <Input
              id='redemption-code'
              value={props.redemptionCode}
              onChange={(event) =>
                props.onRedemptionCodeChange(event.target.value)
              }
              placeholder={t('Enter your redemption code')}
              className='h-9 min-w-0'
            />
            <Button
              onClick={props.onRedeem}
              disabled={props.redeeming}
              variant='outline'
              className='h-9 px-4'
            >
              {props.redeeming ? (
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
              ) : null}
              {t('Redeem')}
            </Button>
          </div>
          {props.topupLink ? (
            <p className='text-muted-foreground text-xs'>
              {t('Need a redemption code?')}{' '}
              <a
                href={props.topupLink}
                target='_blank'
                rel='noopener noreferrer'
                className='inline-flex items-center gap-1 underline-offset-4 hover:underline'
              >
                {t('Get one here')}
                <ExternalLink className='h-3 w-3' />
              </a>
            </p>
          ) : null}
        </div>
      ) : null}
    </TitledCard>
  )
}
