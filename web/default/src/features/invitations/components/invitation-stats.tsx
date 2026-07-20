/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useTranslation } from 'react-i18next'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { formatInvitationUSD } from '../lib/usd'
import type { InvitationSummary } from '../types'

interface InvitationStatsProps {
  summary: InvitationSummary | null
  registeredCount: number
  loading: boolean
}

export function InvitationStats(props: InvitationStatsProps) {
  const { t } = useTranslation()
  const pending = props.loading || props.summary === null
  const hasPendingReferrals = (props.summary?.pending_count ?? 0) > 0
  const subscriptionMode = props.summary?.reward_mode === 'subscription'
  const stats = [
    {
      label: t('Total earned'),
      value: formatInvitationUSD(props.summary?.history_usd ?? 0),
      description: t('Lifetime'),
    },
    subscriptionMode
      ? {
          label: t('Locked credits'),
          value: formatInvitationUSD(props.summary?.locked_reward_usd ?? 0),
          description: t("Unlocks {{days}} days after your friend's payment", {
            days: props.summary?.unlock_delay_days ?? 7,
          }),
        }
      : {
          label: t('Pending credits'),
          value: formatInvitationUSD(props.summary?.pending_reward_usd ?? 0),
          description: t("Released after your friend's first top-up"),
        },
    {
      label: t('Registered friends'),
      value: String(props.registeredCount),
      description: subscriptionMode
        ? t('You earn what they pay for their first month')
        : t('{{reward}} each after first top-up', {
            reward: formatInvitationUSD(props.summary?.inviter_reward_usd ?? 0),
          }),
    },
    {
      label: t('Status'),
      value: hasPendingReferrals ? t('Active') : '--',
      description: hasPendingReferrals
        ? t('Tracking')
        : t('Share your link to start earning'),
    },
  ]

  return (
    <div className='grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4'>
      {stats.map((stat) => (
        <Card key={stat.label} size='sm'>
          <CardContent>
            <p className='text-muted-foreground text-xs font-medium'>
              {stat.label}
            </p>
            {pending ? (
              <Skeleton className='mt-2 h-7 w-24' />
            ) : (
              <p className='mt-2 text-2xl font-semibold tabular-nums'>
                {stat.value}
              </p>
            )}
            {!pending && (
              <p className='text-muted-foreground mt-1 text-xs'>
                {stat.description}
              </p>
            )}
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
