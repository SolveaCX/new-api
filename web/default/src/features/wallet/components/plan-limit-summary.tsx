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
import { useTranslation } from 'react-i18next'
import { getCurrencyDisplay } from '@/lib/currency'
import { cn } from '@/lib/utils'
import type { SubscriptionPlan } from '@/features/subscriptions/types'

type PlanLimitSummaryProps = {
  plan: Pick<SubscriptionPlan, 'window_5h_amount' | 'window_week_amount'>
  className?: string
}

function formatUSDQuota(quota: number): string {
  const { config } = getCurrencyDisplay()
  const amountUSD = quota / config.quotaPerUnit
  return `$${Intl.NumberFormat(undefined, {
    minimumFractionDigits: 0,
    maximumFractionDigits: 6,
  }).format(amountUSD)}`
}

export function PlanLimitSummary(props: PlanLimitSummaryProps) {
  const { t } = useTranslation()

  const items = [
    {
      key: '5h',
      label: t('5-hour window limit (USD)'),
      amount: Number(props.plan.window_5h_amount || 0),
    },
    {
      key: '7d',
      label: t('7-day window limit (USD)'),
      amount: Number(props.plan.window_week_amount || 0),
    },
  ].filter((item) => item.amount > 0)

  if (items.length === 0) return null

  return (
    <dl
      className={cn(
        'grid grid-cols-1 gap-2 text-xs sm:grid-cols-2',
        props.className
      )}
    >
      {items.map((item) => (
        <div
          key={item.key}
          data-plan-limit={item.key}
          className='border-border/60 bg-muted/30 rounded-lg border px-3 py-2'
        >
          <dt className='text-muted-foreground truncate'>{item.label}</dt>
          <dd className='mt-1 font-medium tabular-nums'>
            {formatUSDQuota(item.amount)}
          </dd>
        </div>
      ))}
    </dl>
  )
}
