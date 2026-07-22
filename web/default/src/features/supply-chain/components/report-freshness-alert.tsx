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
import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { SHANGHAI_TIME_ZONE } from '../lib/time'
import type {
  SupplierReportBatchStatus,
  SupplierReportFreshness,
} from '../types'

interface ReportFreshnessAlertProps {
  freshness: SupplierReportFreshness
}

function statusLabel(status: SupplierReportBatchStatus, translate: TFunction) {
  if (status === 'completed') return translate('Completed')
  if (status === 'running') return translate('Running')
  if (status === 'failed') return translate('Failed')
  return translate('Unavailable')
}

function formatShanghaiDateTime(value: number, fallback: string): string {
  if (!Number.isSafeInteger(value) || value <= 0) return fallback
  return new Intl.DateTimeFormat(undefined, {
    timeZone: SHANGHAI_TIME_ZONE,
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value * 1000))
}

export function ReportFreshnessAlert(props: ReportFreshnessAlertProps) {
  const { t } = useTranslation()
  const status = props.freshness.batch_status
  const coverageStartAvailable = props.freshness.coverage_start_at !== null
  const destructive =
    status === 'failed' ||
    !props.freshness.fresh_through ||
    !props.freshness.sync_only ||
    !coverageStartAvailable
  let title = t('Supplier report data is not ready')
  if (status === 'completed') title = t('Supplier report data is current')
  if (status === 'running') title = t('Supplier report data is updating')
  if (status === 'failed') title = t('Supplier report data update failed')
  if (!coverageStartAvailable) {
    title = t('Supplier report coverage is incomplete')
  }
  if (!props.freshness.sync_only) {
    title = t('Supplier report coverage mode is unsupported')
  }

  const dataThrough = formatShanghaiDateTime(
    props.freshness.fresh_through ?? 0,
    t('Unavailable')
  )
  const coverageStart = formatShanghaiDateTime(
    props.freshness.coverage_start_at ?? 0,
    t('Unavailable')
  )
  const freshnessHours =
    props.freshness.freshness_lag_seconds === null
      ? null
      : Math.max(0, Math.floor(props.freshness.freshness_lag_seconds / 3600))

  return (
    <Alert variant={destructive ? 'destructive' : 'default'}>
      <AlertTitle className='flex flex-wrap items-center gap-2'>
        {title}
        <Badge variant={destructive ? 'destructive' : 'secondary'}>
          {statusLabel(status, t)}
        </Badge>
        <Badge variant={props.freshness.sync_only ? 'outline' : 'destructive'}>
          {props.freshness.sync_only
            ? t('Synchronous final logs only')
            : t('Coverage mode mismatch')}
        </Badge>
      </AlertTitle>
      <AlertDescription className='flex flex-col gap-1'>
        <span>
          {t('Coverage starts at {{date}} (Asia/Shanghai).', {
            date: coverageStart,
          })}
        </span>
        <span>
          {t(
            'Only final consumption logs settled synchronously on or after this time are included.'
          )}
        </span>
        <span>{t('Usage before this time is outside report coverage.')}</span>
        <span>
          {t('Latest batch: {{date}}', {
            date: props.freshness.latest_batch_date || t('Unavailable'),
          })}
        </span>
        <span>
          {t('Data is available through {{date}} (Asia/Shanghai).', {
            date: dataThrough,
          })}
        </span>
        {freshnessHours === null ? null : (
          <span>
            {t('Latest completed data is {{count}} hours old.', {
              count: freshnessHours,
            })}
          </span>
        )}
        {props.freshness.error_message ? (
          <span>
            {t('Latest batch error: {{error}}', {
              error: props.freshness.error_message,
            })}
          </span>
        ) : null}
      </AlertDescription>
    </Alert>
  )
}
