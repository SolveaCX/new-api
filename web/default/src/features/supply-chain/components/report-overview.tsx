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
import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import {
  formatMicroUsd,
  formatNullableRatioPercent,
  knownMoneyValue,
} from '../lib/format'
import type { SupplierReportOverview } from '../types'

interface ReportOverviewProps {
  data?: SupplierReportOverview | null
  isLoading?: boolean
}

interface LedgerRowProps {
  label: string
  value: string
  detail?: string
}

function LedgerRow(props: LedgerRowProps) {
  return (
    <div className='grid grid-cols-[minmax(0,1fr)_auto] items-baseline gap-4 py-2'>
      <div className='min-w-0'>
        <div className='text-muted-foreground truncate'>{props.label}</div>
        {props.detail ? (
          <div className='text-muted-foreground text-xs'>{props.detail}</div>
        ) : null}
      </div>
      <div className='text-right font-medium tabular-nums'>{props.value}</div>
    </div>
  )
}

function OverviewSkeleton() {
  return (
    <Card aria-busy='true'>
      <CardHeader>
        <Skeleton className='h-5 w-48' />
        <Skeleton className='h-4 w-72 max-w-full' />
      </CardHeader>
      <CardContent className='grid gap-6 lg:grid-cols-3'>
        {[0, 1, 2].map((item) => (
          <div key={item} className='flex flex-col gap-3'>
            <Skeleton className='h-4 w-32' />
            <Skeleton className='h-16 w-full' />
            <Skeleton className='h-16 w-full' />
          </div>
        ))}
      </CardContent>
    </Card>
  )
}

export function ReportOverview(props: ReportOverviewProps) {
  const { t } = useTranslation()
  const unknown = t('—')

  if (props.isLoading) return <OverviewSkeleton />

  if (!props.data) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t('Supply chain overview')}</CardTitle>
          <CardDescription>
            {t('Official-list inventory, sales, cost, and profit ledger.')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Empty>
            <EmptyHeader>
              <EmptyTitle>{t('No report data')}</EmptyTitle>
              <EmptyDescription>
                {t('Choose a reporting window with settled supplier usage.')}
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        </CardContent>
      </Card>
    )
  }

  const business = props.data.business
  const internal = props.data.internal

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Supply chain overview')}</CardTitle>
        <CardDescription>
          {t(
            'Official-list inventory and frozen settlement values for the selected Asia/Shanghai window.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='grid gap-6 lg:grid-cols-3'>
        <section aria-labelledby='supply-inventory-heading'>
          <div className='mb-2 flex flex-wrap items-center justify-between gap-2'>
            <h3 id='supply-inventory-heading' className='font-medium'>
              {t('Inventory remaining')}
            </h3>
            <Badge variant='outline'>{t('Official list price')}</Badge>
          </div>
          <LedgerRow
            label={t('Configured inventory')}
            value={formatMicroUsd(
              props.data.total_inventory_micro_usd,
              unknown
            )}
          />
          <Separator />
          <LedgerRow
            label={t('Consumed inventory')}
            value={formatMicroUsd(
              props.data.official_list_consumed_micro_usd,
              unknown
            )}
          />
          <Separator />
          <LedgerRow
            label={t('Remaining inventory')}
            value={formatMicroUsd(
              props.data.remaining_inventory_micro_usd,
              unknown
            )}
          />
        </section>

        <section aria-labelledby='supply-business-heading'>
          <div className='mb-2 flex flex-wrap items-center justify-between gap-2'>
            <h3 id='supply-business-heading' className='font-medium'>
              {t('Business profit')}
            </h3>
            <Badge variant='secondary'>
              {t('{{count}} requests', { count: business.request_count })}
            </Badge>
          </div>
          <LedgerRow
            label={t('Sales')}
            value={formatMicroUsd(
              knownMoneyValue(business.sales, business),
              unknown
            )}
            detail={t('{{known}} settled values known', {
              known: business.sales.known_count,
            })}
          />
          <Separator />
          <LedgerRow
            label={t('Procurement cost')}
            value={formatMicroUsd(
              knownMoneyValue(business.procurement_cost, business),
              unknown
            )}
          />
          <Separator />
          <LedgerRow
            label={t('Gross profit')}
            value={formatMicroUsd(
              knownMoneyValue(business.gross_profit, business),
              unknown
            )}
            detail={t('Gross margin {{margin}}', {
              margin: formatNullableRatioPercent(
                business.gross_margin,
                unknown
              ),
            })}
          />
        </section>

        <section aria-labelledby='supply-internal-heading'>
          <div className='mb-2 flex flex-wrap items-center justify-between gap-2'>
            <h3 id='supply-internal-heading' className='font-medium'>
              {t('Internal and test consumption')}
            </h3>
            <Badge variant='outline'>
              {props.data.internal_dimension_available
                ? t('Available')
                : t('Unavailable')}
            </Badge>
          </div>
          <LedgerRow
            label={t('Internal requests')}
            value={
              props.data.internal_dimension_available
                ? String(internal.request_count)
                : unknown
            }
          />
          <Separator />
          <LedgerRow
            label={t('Official-list consumption')}
            value={
              props.data.internal_dimension_available
                ? formatMicroUsd(
                    knownMoneyValue(internal.official_list, internal),
                    unknown
                  )
                : unknown
            }
          />
          <Separator />
          <LedgerRow
            label={t('Estimated procurement cost')}
            value={
              props.data.internal_dimension_available
                ? formatMicroUsd(
                    knownMoneyValue(internal.procurement_cost, internal),
                    unknown
                  )
                : unknown
            }
          />
        </section>
      </CardContent>
    </Card>
  )
}
