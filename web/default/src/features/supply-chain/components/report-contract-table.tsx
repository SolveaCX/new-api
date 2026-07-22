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
import type { SupplierReportContractList, SupplierStatus } from '../types'
import { ProgressiveLoadMoreButton } from './progressive-list'

interface ReportContractTableProps {
  data?: SupplierReportContractList | null
  isLoading?: boolean
  hasMore?: boolean
  isLoadingMore?: boolean
  onLoadMore?: () => void
}

function ContractTableSkeleton() {
  return (
    <div className='flex flex-col gap-2' aria-busy='true'>
      {[0, 1, 2, 3].map((item) => (
        <Skeleton key={item} className='h-12 w-full' />
      ))}
    </div>
  )
}

function contractStateLabel(
  oversold: boolean,
  status: SupplierStatus,
  translate: TFunction
): string {
  if (oversold) return translate('Oversold')
  return status === 'active' ? translate('Active') : translate('Inactive')
}

export function ReportContractTable(props: ReportContractTableProps) {
  const { t } = useTranslation()
  const unknown = t('—')
  let content: ReactNode

  if (props.isLoading) {
    content = <ContractTableSkeleton />
  } else if (!props.data || props.data.items.length === 0) {
    content = (
      <Empty>
        <EmptyHeader>
          <EmptyTitle>{t('No contracts in this report')}</EmptyTitle>
          <EmptyDescription>
            {t('No supplier contract matched the selected filters.')}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  } else {
    content = (
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Supplier / contract')}</TableHead>
            <TableHead>{t('Procurement rate')}</TableHead>
            <TableHead className='text-right'>{t('Remaining')}</TableHead>
            <TableHead className='text-right'>{t('Business sales')}</TableHead>
            <TableHead className='text-right'>{t('Gross profit')}</TableHead>
            <TableHead className='text-right'>{t('Internal cost')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.data.items.map((item) => (
            <TableRow key={item.contract_id}>
              <TableCell>
                <div className='flex min-w-48 flex-col gap-1'>
                  <div className='flex flex-wrap items-center gap-2'>
                    <span className='font-medium'>{item.contract_name}</span>
                    <Badge variant={item.oversold ? 'destructive' : 'outline'}>
                      {contractStateLabel(
                        item.oversold,
                        item.contract_status,
                        t
                      )}
                    </Badge>
                  </div>
                  <span className='text-muted-foreground text-xs'>
                    {item.supplier_name} · {item.contract_no}
                  </span>
                </div>
              </TableCell>
              <TableCell className='tabular-nums'>
                {formatPpmPercent(item.procurement_multiplier_ppm, unknown)}
              </TableCell>
              <TableCell className='text-right'>
                {formatMicroUsd(item.remaining_inventory_micro_usd, unknown)}
              </TableCell>
              <TableCell className='text-right'>
                {formatMicroUsd(
                  knownMoneyValue(item.business.sales, item.business),
                  unknown
                )}
              </TableCell>
              <TableCell className='text-right'>
                <div className='flex flex-col items-end gap-1'>
                  <span>
                    {formatMicroUsd(
                      knownMoneyValue(
                        item.business.gross_profit,
                        item.business
                      ),
                      unknown
                    )}
                  </span>
                  <span className='text-muted-foreground text-xs'>
                    {formatNullableRatioPercent(
                      item.business.gross_margin,
                      unknown
                    )}
                  </span>
                </div>
              </TableCell>
              <TableCell className='text-right'>
                {item.internal_dimension_available
                  ? formatMicroUsd(
                      knownMoneyValue(
                        item.internal.procurement_cost,
                        item.internal
                      ),
                      unknown
                    )
                  : unknown}
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
        <CardTitle>{t('Contract ledger')}</CardTitle>
        <CardDescription>
          {t(
            'Inventory, procurement rate, business profit, and internal consumption by supplier contract.'
          )}
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
