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
import { AlertTriangle, Inbox, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { getStatusLabelKey, type StatusValue } from '../types'

export function StatusBadge(props: { status: string }) {
  const { t } = useTranslation()
  const variants: Record<
    StatusValue,
    React.ComponentProps<typeof Badge>['variant']
  > = {
    operational: 'secondary',
    degraded: 'default',
    outage: 'destructive',
    unknown: 'outline',
    maintenance: 'outline',
  }
  const knownStatus =
    props.status === 'operational' ||
    props.status === 'degraded' ||
    props.status === 'outage' ||
    props.status === 'maintenance'
      ? props.status
      : 'unknown'

  return (
    <Badge variant={variants[knownStatus]}>
      {t(getStatusLabelKey(props.status))}
    </Badge>
  )
}

export function LoadingState() {
  const { t } = useTranslation()
  return (
    <div
      className='space-y-3'
      role='status'
      aria-label={t('statusCenter.loading')}
    >
      <Skeleton className='h-16 w-full' />
      <Skeleton className='h-16 w-full' />
      <Skeleton className='h-16 w-full' />
    </div>
  )
}

export function ErrorState(props: { onRetry: () => void }) {
  const { t } = useTranslation()
  return (
    <Alert variant='destructive'>
      <AlertTriangle aria-hidden='true' />
      <AlertTitle>{t('statusCenter.error.title')}</AlertTitle>
      <AlertDescription className='flex flex-wrap items-center justify-between gap-3'>
        <span>{t('statusCenter.error.description')}</span>
        <Button
          type='button'
          size='sm'
          variant='outline'
          onClick={props.onRetry}
        >
          <RefreshCw aria-hidden='true' />
          {t('statusCenter.retry')}
        </Button>
      </AlertDescription>
    </Alert>
  )
}

export function EmptyState(props: { descriptionKey: string }) {
  const { t } = useTranslation()
  return (
    <Empty className='border'>
      <EmptyHeader>
        <EmptyMedia variant='icon'>
          <Inbox aria-hidden='true' />
        </EmptyMedia>
        <EmptyTitle>{t('statusCenter.empty.title')}</EmptyTitle>
        <EmptyDescription>{t(props.descriptionKey)}</EmptyDescription>
      </EmptyHeader>
    </Empty>
  )
}

export function ForbiddenState() {
  const { t } = useTranslation()
  return (
    <Alert>
      <AlertTriangle aria-hidden='true' />
      <AlertTitle>{t('statusCenter.forbidden.title')}</AlertTitle>
      <AlertDescription>
        {t('statusCenter.forbidden.description')}
      </AlertDescription>
    </Alert>
  )
}
