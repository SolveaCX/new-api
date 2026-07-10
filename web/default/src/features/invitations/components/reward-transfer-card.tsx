/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useTranslation } from 'react-i18next'
import { formatQuota } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import type { InvitationSummary } from '../types'

interface RewardTransferCardProps {
  summary: InvitationSummary | null
  loading: boolean
  onOpen: () => void
}

export function RewardTransferCard({
  summary,
  loading,
  onOpen,
}: RewardTransferCardProps) {
  const { t } = useTranslation()
  const availableQuota = summary?.transferable_quota ?? 0
  const transferEnabled = summary?.transfer_enabled === true
  const canTransfer = transferEnabled && availableQuota > 0
  const pending = loading || summary === null

  return (
    <TitledCard
      title={t('Transfer Rewards')}
      description={t('Move available referral rewards to your main balance.')}
      action={
        <Button
          onClick={onOpen}
          disabled={pending || !canTransfer}
          className='w-full sm:w-auto'
        >
          {t('Transfer to Balance')}
        </Button>
      }
    >
      {pending ? (
        <Skeleton className='h-8 w-32' />
      ) : (
        <p className='text-2xl font-semibold tabular-nums'>
          {formatQuota(availableQuota)}
        </p>
      )}
      {!pending && !transferEnabled ? (
        <p className='text-muted-foreground mt-2 text-sm'>
          {t(
            'Referral reward transfer is disabled until the administrator confirms compliance terms.'
          )}
        </p>
      ) : null}
    </TitledCard>
  )
}
