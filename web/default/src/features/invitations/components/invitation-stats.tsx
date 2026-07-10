/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useTranslation } from 'react-i18next'
import { formatQuota } from '@/lib/format'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import type { InvitationSummary } from '../types'

interface InvitationStatsProps {
  summary: InvitationSummary | null
  loading: boolean
}

export function InvitationStats({ summary, loading }: InvitationStatsProps) {
  const { t } = useTranslation()
  const pending = loading || summary === null
  const stats = [
    {
      label: t('Total earned'),
      value: formatQuota(summary?.history_quota ?? 0),
    },
    {
      label: t('Available to transfer'),
      value: formatQuota(summary?.transferable_quota ?? 0),
    },
    {
      label: t('Successful referrals'),
      value: String(summary?.granted_count ?? 0),
    },
    {
      label: t('Waiting for first top-up'),
      value: String(summary?.pending_count ?? 0),
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
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
