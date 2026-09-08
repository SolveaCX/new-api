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
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import {
  Area,
  AreaChart,
  Bar,
  CartesianGrid,
  ComposedChart,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { SectionPageLayout } from '@/components/layout'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  downloadUsageReportCSV,
  getUsageReport,
  usageReportQueryKeys,
} from './api'
import type { UsageReportData, UsageReportDayRow, UsageReportModelRow } from './types'

const DAY_OPTIONS = [7, 14, 30, 60, 90]
const MODEL_PALETTE = [
  '#1971c2',
  '#4dabf7',
  '#12b886',
  '#82c91e',
  '#f76707',
  '#9c36b5',
  '#f59f00',
  '#e64980',
  '#868e96',
]

const num = (v: number | null | undefined): string =>
  v == null ? '-' : v.toLocaleString('en-US')
const usd = (v: number | null | undefined): string =>
  v == null ? '-' : `$${v.toLocaleString('en-US', { maximumFractionDigits: 2 })}`
const fmtBig = (v: number): string =>
  Math.abs(v) >= 1e8 ? `${(v / 1e8).toFixed(1)}亿` : Math.abs(v) >= 1e4 ? `${(v / 1e4).toFixed(1)}万` : num(v)

interface TodayKpi {
  label: string
  value: string
  note: string
}

export function UsageReport() {
  const { t } = useTranslation()
  const [days, setDays] = useState(30)
  const { data: res, isLoading } = useQuery({
    queryKey: usageReportQueryKeys.report(days),
    queryFn: () => getUsageReport(days),
  })
  const payload: UsageReportData | undefined = res?.data

  const dayRows: UsageReportDayRow[] = payload ? [...payload.days].sort((a, b) => a.date.localeCompare(b.date)) : []
  const modelRows: UsageReportModelRow[] = payload ? [...payload.models] : []
  const today = dayRows.length > 0 ? dayRows[dayRows.length - 1] : undefined
  const yesterday = dayRows.length > 1 ? dayRows[dayRows.length - 2] : undefined

  const kpis: TodayKpi[] = today
    ? [
        { label: t('Registered'), value: num(today.registered), note: `环比 ${delta(today.registered, yesterday?.registered)}` },
        { label: t('Activated (Key)'), value: num(today.activated_key), note: `环比 ${delta(today.activated_key, yesterday?.activated_key)}` },
        { label: t('First Paid'), value: num(today.first_paid), note: `环比 ${delta(today.first_paid, yesterday?.first_paid)}` },
        { label: t('Paid Amount (Today)'), value: usd(today.paid_usd), note: `环比 ${deltaUsd(today.paid_usd, yesterday?.paid_usd)}` },
        { label: t('Calls'), value: num(today.calls), note: `${t('date')}: ${today.date} (UTC+0)` },
        { label: t('Tokens'), value: fmtBig(today.prompt_tokens + today.completion_tokens), note: t('prompt + completion') },
      ]
    : []

  // 每日明细表 + 合计
  const totals = dayRows.reduce(
    (acc, r) => {
      acc.registered += r.registered
      acc.activated_key += r.activated_key
      acc.first_paid += r.first_paid
      acc.paid_usd += r.paid_usd
      acc.calls += r.calls
      acc.tokens += r.prompt_tokens + r.completion_tokens
      return acc
    },
    { registered: 0, activated_key: 0, first_paid: 0, paid_usd: 0, calls: 0, tokens: 0 }
  )

  // 各模型按日 tokens（prompt+completion），取总量 Top 8，其余归 other
  const modelTotals = new Map<string, number>()
  modelRows.forEach((m) => {
    modelTotals.set(m.model_name, (modelTotals.get(m.model_name) ?? 0) + m.prompt_tokens + m.completion_tokens)
  })
  const topModels = [...modelTotals.entries()].sort((a, b) => b[1] - a[1]).slice(0, 8).map(([k]) => k)
  const modelDates = [...new Set(modelRows.map((m) => m.date))].sort()
  const modelSerie = modelDates.map((date) => {
    const row: Record<string, number | string> = { date }
    const byModel = new Map<string, { tok: number; calls: number }>()
    modelRows.filter((m) => m.date === date).forEach((m) => {
      const cur = byModel.get(m.model_name) ?? { tok: 0, calls: 0 }
      cur.tok += m.prompt_tokens + m.completion_tokens
      cur.calls += m.calls
      byModel.set(m.model_name, cur)
    })
    let other = 0
    topModels.forEach((name) => {
      const cur = byModel.get(name)
      row[name] = cur ? cur.tok : 0
    })
    byModel.forEach((cur, name) => {
      if (!topModels.includes(name)) other += cur.tok
    })
    if (other > 0) row.other = other
    return row
  })

  // 转化率：近 7 日滚动（Σ激活/Σ注册、Σ首付/Σ激活），避免注册量骤降日的假高值
  const convSerie = dayRows.map((_, i) => {
    const lo = Math.max(0, i - 6)
    let regs = 0
    let keys = 0
    let paid = 0
    for (let k = lo; k <= i; k++) {
      regs += dayRows[k].registered
      keys += dayRows[k].activated_key
      paid += dayRows[k].first_paid
    }
    return {
      date: dayRows[i].date,
      '注册→激活率': regs > 0 ? Math.round((keys / regs) * 1000) / 10 : 0,
      '激活→首付率': keys > 0 ? Math.round((paid / keys) * 10000) / 100 : 0,
    }
  })

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Usage Report')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <div className='flex items-center gap-1'>
          {DAY_OPTIONS.map((d) => (
            <Button key={d} size='sm' variant={days === d ? 'default' : 'outline'} onClick={() => setDays(d)}>
              {d}天
            </Button>
          ))}
        </div>
        <Button size='sm' variant='outline' onClick={() => void downloadUsageReportCSV(days, 'daily')}>
          ⬇ CSV(日漏斗)
        </Button>
        <Button size='sm' variant='outline' onClick={() => void downloadUsageReportCSV(days, 'models')}>
          ⬇ CSV(日×模型)
        </Button>
      </SectionPageLayout.Actions>

      <SectionPageLayout.Content>
        {isLoading || !payload ? (
          <div className='space-y-4'>
            <Skeleton className='h-28 w-full' />
            <Skeleton className='h-96 w-full' />
          </div>
        ) : (
          <div className='space-y-6'>
            {/* KPI */}
            <div className='grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-6'>
              {kpis.map((k) => (
                <Card key={k.label}>
                  <CardContent className='pt-5'>
                    <div className='text-muted-foreground text-xs'>{k.label}</div>
                    <div className='mt-1 text-2xl font-bold'>{k.value}</div>
                    <div className='text-muted-foreground mt-1 text-xs'>{k.note}</div>
                  </CardContent>
                </Card>
              ))}
            </div>

            {/* 每日明细表 */}
            <Card>
              <CardHeader>
                <CardTitle className='flex flex-wrap items-center justify-between gap-2'>
                  {t('Daily Detail')}
                  <span className='text-muted-foreground text-xs font-normal'>
                    {t('UTC+0 days, registered = enabled + email-verified, activated = first key')}
                  </span>
                </CardTitle>
              </CardHeader>
              <CardContent className='overflow-x-auto'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Date')}</TableHead>
                      <TableHead className='text-right'>{t('Registered')}</TableHead>
                      <TableHead className='text-right'>{t('Activated (Key)')}</TableHead>
                      <TableHead className='text-right'>{t('First Paid')}</TableHead>
                      <TableHead className='text-right'>{t('Paid Amount $')}</TableHead>
                      <TableHead className='text-right'>{t('Calls')}</TableHead>
                      <TableHead className='text-right'>{t('Tokens')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {dayRows.map((r, idx) => (
                      <TableRow key={r.date} className={idx === dayRows.length - 1 ? 'bg-accent/40 font-semibold' : ''}>
                        <TableCell>{r.date}</TableCell>
                        <TableCell className='text-right'>{num(r.registered)}</TableCell>
                        <TableCell className='text-right'>{num(r.activated_key)}</TableCell>
                        <TableCell className='text-right'>{num(r.first_paid)}</TableCell>
                        <TableCell className='text-right'>{usd(r.paid_usd)}</TableCell>
                        <TableCell className='text-right'>{num(r.calls)}</TableCell>
                        <TableCell className='text-right'>{fmtBig(r.prompt_tokens + r.completion_tokens)}</TableCell>
                      </TableRow>
                    ))}
                    <TableRow className='border-t-2 font-bold'>
                      <TableCell>{t('Total')} ({dayRows.length}天)</TableCell>
                      <TableCell className='text-right'>{num(totals.registered)}</TableCell>
                      <TableCell className='text-right'>{num(totals.activated_key)}</TableCell>
                      <TableCell className='text-right'>{num(totals.first_paid)}</TableCell>
                      <TableCell className='text-right'>{usd(totals.paid_usd)}</TableCell>
                      <TableCell className='text-right'>{num(totals.calls)}</TableCell>
                      <TableCell className='text-right'>{fmtBig(totals.tokens)}</TableCell>
                    </TableRow>
                  </TableBody>
                </Table>
              </CardContent>
            </Card>

            {/* 双Y轴：注册/激活/首付 柱 + 付费金额 线 */}
            <Card>
              <CardHeader>
                <CardTitle>{t('Daily Funnel — bars (left) + paid amount line (right)')}</CardTitle>
              </CardHeader>
              <CardContent>
                <ResponsiveContainer width='100%' height={320}>
                  <ComposedChart data={dayRows} margin={{ top: 8, right: 8, left: 8, bottom: 8 }}>
                    <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' />
                    <XAxis dataKey='date' tick={{ fontSize: 11 }} tickFormatter={(d: string) => d.slice(5)} />
                    <YAxis yAxisId='people' tick={{ fontSize: 11 }} />
                    <YAxis yAxisId='usd' orientation='right' tick={{ fontSize: 11 }} tickFormatter={(v: number) => (v >= 1000 ? `${Math.round(v / 1000)}k` : String(v))} />
                    <Tooltip />
                    <Legend />
                    <Bar yAxisId='people' dataKey='registered' name={t('Registered')} fill='#f5b942' barSize={10} />
                    <Bar yAxisId='people' dataKey='activated_key' name={t('Activated (Key)')} fill='#2f6bff' barSize={10} />
                    <Bar yAxisId='people' dataKey='first_paid' name={t('First Paid')} fill='#0f9d58' barSize={10} />
                    <Line yAxisId='usd' type='monotone' dataKey='paid_usd' name={t('Paid Amount $')} stroke='#9c36b5' strokeDasharray='6 3' dot={false} />
                  </ComposedChart>
                </ResponsiveContainer>
              </CardContent>
            </Card>

            <div className='grid gap-6 xl:grid-cols-2'>
              {/* 转化率趋势 */}
              <Card>
                <CardHeader>
                  <CardTitle>{t('Conversion Trend (7d rolling)')}</CardTitle>
                </CardHeader>
                <CardContent>
                  <ResponsiveContainer width='100%' height={260}>
                    <LineChart data={convSerie} margin={{ top: 8, right: 8, left: 8, bottom: 8 }}>
                      <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' />
                      <XAxis dataKey='date' tick={{ fontSize: 11 }} tickFormatter={(d: string) => d.slice(5)} />
                      <YAxis tick={{ fontSize: 11 }} tickFormatter={(v: number) => `${v}%`} />
                      <Tooltip formatter={(value: unknown) => (value == null ? '' : `${String(value)}%`)} />
                      <Legend />
                      <Line type='monotone' dataKey='注册→激活率' name={t('注册→激活率')} stroke='#2f6bff' dot={false} />
                      <Line type='monotone' dataKey='激活→首付率' name={t('激活→首付率')} stroke='#0f9d58' dot={false} />
                    </LineChart>
                  </ResponsiveContainer>
                </CardContent>
              </Card>

              {/* 各模型堆叠面积 */}
              <Card>
                <CardHeader>
                  <CardTitle>{t('Model Usage (stacked tokens)')}</CardTitle>
                </CardHeader>
                <CardContent>
                  <ResponsiveContainer width='100%' height={260}>
                    <AreaChart data={modelSerie} margin={{ top: 8, right: 8, left: 8, bottom: 8 }}>
                      <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' />
                      <XAxis dataKey='date' tick={{ fontSize: 11 }} tickFormatter={(d: string) => d.slice(5)} />
                      <YAxis tick={{ fontSize: 11 }} tickFormatter={(v: number) => (v >= 1e8 ? `${(v / 1e8).toFixed(0)}亿` : num(v))} />
                      <Tooltip />
                      <Legend wrapperStyle={{ fontSize: 12 }} />
                      {[...topModels, ...(modelSerie.some((r) => r.other) ? ['other'] : [])].map((name, i) => (
                        <Area
                          key={name}
                          type='monotone'
                          dataKey={name}
                          stackId='tok'
                          stroke={MODEL_PALETTE[i % MODEL_PALETTE.length]}
                          fill={MODEL_PALETTE[i % MODEL_PALETTE.length]}
                          fillOpacity={0.85}
                        />
                      ))}
                    </AreaChart>
                  </ResponsiveContainer>
                </CardContent>
              </Card>
            </div>
          </div>
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

function delta(cur: number | undefined, prev: number | undefined): string {
  if (cur == null || prev == null) return '-'
  const d = cur - prev
  return d >= 0 ? `+${d.toLocaleString('en-US')}` : d.toLocaleString('en-US')
}
function deltaUsd(cur: number | undefined, prev: number | undefined): string {
  if (cur == null || prev == null) return '-'
  const d = cur - prev
  return d >= 0 ? `+$${d.toFixed(2)}` : `-$${Math.abs(d).toFixed(2)}`
}
