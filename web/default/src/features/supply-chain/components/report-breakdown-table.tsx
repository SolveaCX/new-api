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
import type { ReactNode } from 'react'
import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  formatMicroUsd,
  formatNullableRatioPercent,
  formatPpmPercent,
  knownMoneyValue,
} from '../lib/format'
import type { SupplierReportBreakdownList, SupplierDataQuality } from '../types'
import { ProgressiveLoadMoreButton } from './progressive-list'
import { ReportQueryError } from './report-query-error'

interface ReportBreakdownTableProps {
  data?: SupplierReportBreakdownList | null
  isLoading?: boolean
  isError?: boolean
  hasMore?: boolean
  isLoadingMore?: boolean
  onLoadMore?: () => void
}

function dataQualityLabel(
  value: SupplierDataQuality,
  translate: TFunction
): string {
  if (value === 'authoritative') return translate('Authoritative')
  return translate('Unattributed')
}

export function ReportBreakdownTable(props: ReportBreakdownTableProps) {
  const { t } = useTranslation()
  const unknown = t('—')
  let content: ReactNode

  if (props.isError && !props.data) {
    content = <ReportQueryError hasData={false} />
  } else if (props.isLoading) {
    content = (
      <div className='flex flex-col gap-2' aria-busy='true'>
        {[0, 1, 2, 3].map((item) => (
          <Skeleton key={item} className='h-12 w-full' />
        ))}
      </div>
    )
  } else if (!props.data || props.data.items.length === 0) {
    content = (
      <Empty>
        <EmptyHeader>
          <EmptyTitle>{t('No pricing breakdown')}</EmptyTitle>
          <EmptyDescription>
            {t('No eligible business dimensions exist in this window.')}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  } else {
    content = (
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Channel / model')}</TableHead>
            <TableHead>{t('Frozen versions')}</TableHead>
            <TableHead>{t('Quality')}</TableHead>
            <TableHead className='text-right'>{t('Requests')}</TableHead>
            <TableHead className='text-right'>{t('Sales')}</TableHead>
            <TableHead className='text-right'>{t('Procurement')}</TableHead>
            <TableHead className='text-right'>{t('Gross profit')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.data.items.map((item) => (
            <TableRow
              key={`${item.contract_id}:${item.channel_id}:${item.model_name}:${item.rate_version_id}:${item.sales_multiplier_ppm}:${item.pricing_mode}:${item.data_quality}`}
            >
              <TableCell>
                <div className='flex min-w-44 flex-col gap-1'>
                  <span className='font-medium'>{item.model_name}</span>
                  <span className='text-muted-foreground text-xs'>
                    {t('Channel #{{channel}} · Contract #{{contract}}', {
                      channel: item.channel_id,
                      contract: item.contract_id,
                    })}
                  </span>
                </div>
              </TableCell>
              <TableCell>
                <div className='flex min-w-40 flex-col gap-1 text-xs'>
                  <span>
                    {t('Rate #{{version}}', {
                      version: item.rate_version_id,
                    })}
                  </span>
                  <span className='text-muted-foreground'>
                    {t('Sales {{multiplier}} · {{mode}}', {
                      multiplier: formatPpmPercent(
                        item.sales_multiplier_ppm,
                        unknown
                      ),
                      mode: item.pricing_mode,
                    })}
                  </span>
                </div>
              </TableCell>
              <TableCell>
                <Badge
                  variant={
                    item.data_quality === 'unattributed'
                      ? 'destructive'
                      : 'secondary'
                  }
                >
                  {dataQualityLabel(item.data_quality, t)}
                </Badge>
              </TableCell>
              <TableCell className='text-right'>
                {item.metrics.request_count}
              </TableCell>
              <TableCell className='text-right'>
                {formatMicroUsd(
                  knownMoneyValue(item.metrics.sales, item.metrics),
                  unknown
                )}
              </TableCell>
              <TableCell className='text-right'>
                {formatMicroUsd(
                  knownMoneyValue(item.metrics.procurement_cost, item.metrics),
                  unknown
                )}
              </TableCell>
              <TableCell className='text-right'>
                {formatMicroUsd(
                  knownMoneyValue(item.metrics.gross_profit, item.metrics),
                  unknown
                )}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Pricing breakdown')}</CardTitle>
        <CardDescription>
          {t(
            'Frozen channel, model, procurement version, and sales multiplier dimensions.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='flex flex-col gap-4'>
        {props.isError && props.data ? <ReportQueryError hasData /> : null}
        {content}
      </CardContent>
      {props.data ? (
        <CardFooter className='text-muted-foreground flex flex-wrap items-center justify-between gap-2 text-xs'>
          <div className='flex flex-col gap-1'>
            <span>
              {t(
                '{{eligible}} of {{total}} business requests have breakdown dimensions.',
                {
                  eligible: props.data.breakdown_eligible_count,
                  total: props.data.total_business_count,
                }
              )}
            </span>
            <span>
              {t('Breakdown coverage: {{value}}', {
                value: props.data.breakdown_coverage_available
                  ? formatNullableRatioPercent(
                      props.data.breakdown_coverage_rate,
                      unknown
                    )
                  : unknown,
              })}
            </span>
          </div>
          <ProgressiveLoadMoreButton
            visible={Boolean(props.hasMore)}
            isLoading={props.isLoadingMore}
            onClick={props.onLoadMore}
          />
        </CardFooter>
      ) : null}
    </Card>
  )
}
