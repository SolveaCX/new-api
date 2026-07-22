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
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { CartesianGrid, Line, LineChart, XAxis, YAxis } from 'recharts'
import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { formatMicroUsd, knownMoneyValue } from '../lib/format'
import type { MicroUsd, SupplierReportTrend } from '../types'

type TrendMode = 'business' | 'internal'

interface ReportTrendProps {
  data?: SupplierReportTrend | null
  isLoading?: boolean
}

interface TrendChartRow {
  date: string
  sales: MicroUsd | null
  procurement: MicroUsd | null
  profit: MicroUsd | null
  internalProcurement: MicroUsd | null
}

function TrendSkeleton() {
  return (
    <Card aria-busy='true'>
      <CardHeader>
        <Skeleton className='h-5 w-40' />
        <Skeleton className='h-4 w-64 max-w-full' />
      </CardHeader>
      <CardContent>
        <Skeleton className='h-72 w-full' />
      </CardContent>
    </Card>
  )
}

export function ReportTrend(props: ReportTrendProps) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<TrendMode>('business')
  const unknown = t('—')
  const chartConfig = useMemo<ChartConfig>(
    () => ({
      sales: { label: t('Sales'), color: 'var(--chart-1)' },
      procurement: {
        label: t('Procurement cost'),
        color: 'var(--chart-2)',
      },
      profit: { label: t('Gross profit'), color: 'var(--chart-3)' },
      internalProcurement: {
        label: t('Internal procurement cost'),
        color: 'var(--chart-4)',
      },
    }),
    [t]
  )
  const rows = useMemo<TrendChartRow[]>(() => {
    if (!props.data) return []
    return props.data.points.map((point) => ({
      date: point.date,
      sales: knownMoneyValue(point.business.sales, point.business),
      procurement: knownMoneyValue(
        point.business.procurement_cost,
        point.business
      ),
      profit: knownMoneyValue(point.business.gross_profit, point.business),
      internalProcurement: point.internal_dimension_available
        ? knownMoneyValue(point.internal.procurement_cost, point.internal)
        : null,
    }))
  }, [props.data])

  if (props.isLoading) return <TrendSkeleton />

  return (
    <Card>
      <CardHeader>
        <div className='flex flex-wrap items-start justify-between gap-3'>
          <div className='flex min-w-0 flex-col gap-1'>
            <CardTitle>{t('Settlement trend')}</CardTitle>
            <CardDescription>
              {t('Daily buckets in Asia/Shanghai; unknown values remain gaps.')}
            </CardDescription>
          </div>
          {props.data?.range.month ? (
            <Badge variant='outline'>
              {t('Natural month {{month}}', {
                month: props.data.range.month,
              })}
            </Badge>
          ) : (
            <Badge variant='outline'>{t('Custom date range')}</Badge>
          )}
        </div>
      </CardHeader>
      <CardContent className='flex flex-col gap-4'>
        <Tabs
          value={mode}
          onValueChange={(value) => {
            if (value === 'business' || value === 'internal') {
              setMode(value)
            }
          }}
        >
          <TabsList aria-label={t('Trend series')}>
            <TabsTrigger value='business'>{t('Business')}</TabsTrigger>
            <TabsTrigger value='internal'>{t('Internal / test')}</TabsTrigger>
          </TabsList>
        </Tabs>

        {rows.length === 0 ? (
          <Empty>
            <EmptyHeader>
              <EmptyTitle>{t('No trend data')}</EmptyTitle>
              <EmptyDescription>
                {t('No settled daily buckets exist for this reporting window.')}
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : (
          <ChartContainer
            config={chartConfig}
            className='aspect-auto h-72 w-full min-w-0'
          >
            <LineChart
              data={rows}
              accessibilityLayer
              margin={{ left: 8, right: 8 }}
            >
              <CartesianGrid vertical={false} />
              <XAxis
                dataKey='date'
                tickLine={false}
                axisLine={false}
                minTickGap={24}
              />
              <YAxis
                tickLine={false}
                axisLine={false}
                width={72}
                tickFormatter={(value: number) =>
                  formatMicroUsd(value, unknown)
                }
              />
              <ChartTooltip
                content={
                  <ChartTooltipContent
                    formatter={(value, name) => (
                      <div className='flex min-w-40 items-center justify-between gap-4'>
                        <span className='text-muted-foreground'>
                          {chartConfig[String(name)]?.label ?? String(name)}
                        </span>
                        <span className='font-mono font-medium tabular-nums'>
                          {typeof value === 'number'
                            ? formatMicroUsd(value, unknown)
                            : unknown}
                        </span>
                      </div>
                    )}
                  />
                }
              />
              {mode === 'business' ? (
                <>
                  <Line
                    dataKey='sales'
                    type='monotone'
                    stroke='var(--color-sales)'
                    strokeWidth={2}
                    dot={false}
                    connectNulls={false}
                  />
                  <Line
                    dataKey='procurement'
                    type='monotone'
                    stroke='var(--color-procurement)'
                    strokeWidth={2}
                    dot={false}
                    connectNulls={false}
                  />
                  <Line
                    dataKey='profit'
                    type='monotone'
                    stroke='var(--color-profit)'
                    strokeWidth={2}
                    dot={false}
                    connectNulls={false}
                  />
                </>
              ) : (
                <Line
                  dataKey='internalProcurement'
                  type='monotone'
                  stroke='var(--color-internalProcurement)'
                  strokeWidth={2}
                  dot={false}
                  connectNulls={false}
                />
              )}
            </LineChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  )
}
