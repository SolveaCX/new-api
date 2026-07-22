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
import { formatMicroUsd, knownMoneyValue } from '../lib/format'
import type { SupplierReportChannelList } from '../types'
import { ProgressiveLoadMoreButton } from './progressive-list'

interface ReportChannelTableProps {
  data?: SupplierReportChannelList | null
  isLoading?: boolean
  hasMore?: boolean
  isLoadingMore?: boolean
  onLoadMore?: () => void
}

export function ReportChannelTable(props: ReportChannelTableProps) {
  const { t } = useTranslation()
  const unknown = t('—')
  let content: ReactNode

  if (props.isLoading) {
    content = (
      <div className='flex flex-col gap-2' aria-busy='true'>
        {[0, 1, 2].map((item) => (
          <Skeleton key={item} className='h-12 w-full' />
        ))}
      </div>
    )
  } else if (!props.data || props.data.items.length === 0) {
    content = (
      <Empty>
        <EmptyHeader>
          <EmptyTitle>{t('No channel data')}</EmptyTitle>
          <EmptyDescription>
            {t('No channel matched the selected report filters.')}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  } else {
    content = (
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Channel')}</TableHead>
            <TableHead>{t('Contract')}</TableHead>
            <TableHead className='text-right'>{t('Requests')}</TableHead>
            <TableHead className='text-right'>{t('Sales')}</TableHead>
            <TableHead className='text-right'>{t('Procurement')}</TableHead>
            <TableHead className='text-right'>{t('Gross profit')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.data.items.map((item) => (
            <TableRow key={`${item.contract_id}:${item.channel_id}`}>
              <TableCell>
                <div className='flex min-w-40 items-center gap-2'>
                  <span className='font-medium'>{item.channel_name}</span>
                  <Badge
                    variant={
                      item.channel_status === 1 ? 'default' : 'secondary'
                    }
                  >
                    {item.channel_status === 1 ? t('Enabled') : t('Disabled')}
                  </Badge>
                </div>
              </TableCell>
              <TableCell className='tabular-nums'>
                #{item.contract_id}
              </TableCell>
              <TableCell className='text-right'>
                {item.business.request_count}
              </TableCell>
              <TableCell className='text-right'>
                {formatMicroUsd(
                  knownMoneyValue(item.business.sales, item.business),
                  unknown
                )}
              </TableCell>
              <TableCell className='text-right'>
                {formatMicroUsd(
                  knownMoneyValue(
                    item.business.procurement_cost,
                    item.business
                  ),
                  unknown
                )}
              </TableCell>
              <TableCell className='text-right'>
                {formatMicroUsd(
                  knownMoneyValue(item.business.gross_profit, item.business),
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
        <CardTitle>{t('Channel performance')}</CardTitle>
        <CardDescription>
          {t('Settled business values attributed to each upstream channel.')}
        </CardDescription>
      </CardHeader>
      <CardContent>{content}</CardContent>
      {props.hasMore ? (
        <CardFooter>
          <ProgressiveLoadMoreButton
            visible
            isLoading={props.isLoadingMore}
            onClick={props.onLoadMore}
          />
        </CardFooter>
      ) : null}
    </Card>
  )
}
