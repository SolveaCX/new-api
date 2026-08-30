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

  const windows = [
    { key: '5h' as const, amount: Number(props.plan.window_5h_amount || 0) },
    {
      key: '7d' as const,
      amount: Number(props.plan.window_week_amount || 0),
    },
  ].filter((window) => window.amount > 0)

  if (windows.length === 0) return null

  let summary: string
  if (windows.length === 2) {
    summary = t('Short-term caps: {{fiveHour}} / 5h · {{week}} / 7d', {
      fiveHour: formatUSDQuota(windows[0].amount),
      week: formatUSDQuota(windows[1].amount),
    })
  } else if (windows[0].key === '5h') {
    summary = t('Short-term cap: {{value}} / 5h', {
      value: formatUSDQuota(windows[0].amount),
    })
  } else {
    summary = t('Short-term cap: {{value}} / 7d', {
      value: formatUSDQuota(windows[0].amount),
    })
  }

  return (
    <div
      data-plan-limit-summary='true'
      className={cn(
        'border-primary/20 rounded-lg border bg-[#f0ebfa] px-4 py-3 dark:bg-[#5b21b6]/20',
        props.className
      )}
    >
      <div
        data-plan-limit-label='all-models'
        className='font-mono text-[10px] font-semibold tracking-[0.14em] text-[#4c1d95] uppercase dark:text-[#c4b5fd]'
      >
        {t('All models')}
      </div>
      <p className='text-foreground mt-2 text-sm leading-relaxed wrap-break-word'>
        {summary}
      </p>
    </div>
  )
}
