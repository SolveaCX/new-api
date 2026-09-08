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
import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  ComposedChart,
  Legend,
  Line,
  LineChart,
  Pie,
  PieChart,
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
  Math.abs(v) >= 1e8
    ? `${(v / 1e8).toFixed(1)}亿`
    : Math.abs(v) >= 1e4
      ? `${(v / 1e4).toFixed(1)}万`
      : num(v)
const fmtPct = (v: number | null): string => (v == null ? '-' : `${v.toFixed(v >= 10 ? 1 : 2)}%`)
const tokensOf = (r: { prompt_tokens: number; completion_tokens: number }): number =>
  r.prompt_tokens + r.completion_tokens

interface KpiDatum {
  k: string
  v: string
  d: string
}

function KpiGrid({ items }: { items: KpiDatum[] }) {
  return (
    <div className='grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-6'>
      {items.map((x) => (
        <Card key={x.k}>
          <CardContent className='pt-5'>
            <div className='text-muted-foreground text-xs'>{x.k}</div>
            <div className='mt-1 text-2xl font-bold'>{x.v}</div>
            <div className='text-muted-foreground mt-1 text-xs'>{x.d}</div>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}

function SectionTitle({
  no,
  title,
  hint,
}: {
  no: number
  title: string
  hint: string
}) {
  return (
    <div id={`sec-${no === 1 ? 'funnel' : no === 2 ? 'overview' : 'models'}`} className='scroll-mt-28 pt-2'>
      <div className='flex flex-wrap items-baseline gap-x-3 gap-y-1 py-2'>
        <span className='bg-primary flex h-6 w-6 items-center justify-center rounded-md text-xs font-bold text-white'>
          {no}
        </span>
        <h2 className='text-lg font-bold'>{title}</h2>
        <span className='text-muted-foreground text-xs'>{hint}</span>
      </div>
    </div>
  )
}

function deltaPeople(cur: number | undefined, prev: number | undefined): string {
  if (cur == null || prev == null) return '-'
  const d = cur - prev
  return d >= 0 ? `+${d.toLocaleString('en-US')} 人` : `${d.toLocaleString('en-US')} 人`
}
function deltaUsd(cur: number | undefined, prev: number | undefined): string {
  if (cur == null || prev == null) return '-'
  const d = cur - prev
  return d >= 0 ? `+$${d.toFixed(2)}` : `-$${Math.abs(d).toFixed(2)}`
}

export function UsageReport() {
  const { t } = useTranslation()
  const [days, setDays] = useState(30)
  const [modelMetric, setModelMetric] = useState<'tokens' | 'calls'>('calls')
  const [activeSec, setActiveSec] = useState('funnel')
  const { data: res, isLoading } = useQuery({
    queryKey: usageReportQueryKeys.report(days),
    queryFn: () => getUsageReport(days),
  })
  const payload: UsageReportData | undefined = res?.data
  const dayRows: UsageReportDayRow[] = payload
    ? [...payload.days].sort((a, b) => a.date.localeCompare(b.date))
    : []
  const modelRows: UsageReportModelRow[] = payload ? [...payload.models] : []
  const today = dayRows.length > 0 ? dayRows[dayRows.length - 1] : undefined
  const yesterday = dayRows.length > 1 ? dayRows[dayRows.length - 2] : undefined

  const sumDays = (pick: (r: UsageReportDayRow) => number): number =>
    dayRows.reduce((acc, r) => acc + pick(r), 0)
  const totalTokens = sumDays((r) => tokensOf(r))
  const totalCalls = sumDays((r) => r.calls)
  const totalRegistered = sumDays((r) => r.registered)

  // cohort 完成窗口：前 7/14 天内的行已到期（先算，供 KPI 与图表复用）
  const completed7 = Math.max(0, dayRows.length - 7)
  const completed14 = Math.max(0, dayRows.length - 14)
  const completedRows7 = dayRows.slice(0, completed7)
  const completedRows14 = dayRows.slice(0, completed14)
  const regKeyRate =
    completedRows7.reduce((a, r) => a + r.registered, 0) > 0
      ? (completedRows7.reduce((a, r) => a + r.activated_c7, 0) /
          completedRows7.reduce((a, r) => a + r.registered, 0)) *
        100
      : null
  const keyPayRate =
    completedRows14.reduce((a, r) => a + r.activated_key, 0) > 0
      ? (completedRows14.reduce((a, r) => a + r.paid_c14, 0) /
          completedRows14.reduce((a, r) => a + r.activated_key, 0)) *
        100
      : null
  const funnelRates = { regKey: fmtPct(regKeyRate), keyPay: fmtPct(keyPayRate) }

  // ---- funnel KPI（人）----
  const kpiFunnel: KpiDatum[] = today
    ? [
        { k: t('Registered'), v: num(today.registered), d: `环比 ${deltaPeople(today.registered, yesterday?.registered)}` },
        { k: t('Activated (Key)'), v: num(today.activated_key), d: `环比 ${deltaPeople(today.activated_key, yesterday?.activated_key)}` },
        { k: t('First Paid'), v: num(today.first_paid), d: `环比 ${deltaPeople(today.first_paid, yesterday?.first_paid)}` },
        { k: t('Paid Amount (Today)'), v: usd(today.paid_usd), d: `环比 ${deltaUsd(today.paid_usd, yesterday?.paid_usd)}` },
        {
          k: t('Reg→Activate 7d cohort'),
          v: funnelRates.regKey,
          d: t('completed cohorts only'),
        },
        {
          k: t('Act→Pay 14d cohort'),
          v: funnelRates.keyPay,
          d: t('completed cohorts only'),
        },
      ]
    : []

  // 每日明细表合计
  const totals = {
    registered: totalRegistered,
    activated_key: sumDays((r) => r.activated_key),
    first_paid: sumDays((r) => r.first_paid),
    paid_usd: sumDays((r) => r.paid_usd),
    calls: totalCalls,
    tokens: totalTokens,
  }

  // 转化趋势（队列，人；未到期置 null）
  const convSerie = dayRows.map((r, i) => ({
    date: r.date,
    '注册→激活率(7日队列)':
      i < completed7 && r.registered > 0 ? Math.round((r.activated_c7 / r.registered) * 1000) / 10 : null,
    '激活→首付率(14日队列)':
      i < completed14 && r.activated_key > 0 ? Math.round((r.paid_c14 / r.activated_key) * 10000) / 100 : null,
  }))

  // ---- 用量总览派生 ----
  const combo = dayRows.map((r) => ({ date: r.date, calls: r.calls, tokens: tokensOf(r) }))
  const kpiOverview: KpiDatum[] = today
    ? [
        { k: t('Calls (today)'), v: num(today.calls), d: `区间合计 ${num(totalCalls)}` },
        { k: t('Tokens (today)'), v: fmtBig(tokensOf(today)), d: `区间合计 ${fmtBig(totalTokens)}` },
        { k: t('Registered (today)'), v: num(today.registered), d: `区间合计 ${num(totalRegistered)}` },
        { k: t('Activated Key (today)'), v: num(today.activated_key), d: `区间 ${num(sumDays((r) => r.activated_key))} 人` },
        { k: t('First Paid (today)'), v: num(today.first_paid), d: `区间 ${num(sumDays((r) => r.first_paid))} 人` },
      ]
    : []

  // 模型聚合 & Top
  const modelTotalsMap = new Map<string, { tokens: number; calls: number }>()
  modelRows.forEach((m) => {
    const cur = modelTotalsMap.get(m.model_name) ?? { tokens: 0, calls: 0 }
    cur.tokens += tokensOf(m)
    cur.calls += m.calls
    modelTotalsMap.set(m.model_name, cur)
  })
  const topModels = [...modelTotalsMap.entries()]
    .sort((a, b) => b[1].tokens - a[1].tokens)
    .slice(0, 8)
    .map(([name]) => name)
  const topRankData = [...modelTotalsMap.entries()]
    .sort((a, b) => b[1].tokens - a[1].tokens)
    .slice(0, 8)
    .map(([name, v]) => ({ name, tokens: v.tokens, calls: v.calls }))

  // 模型堆叠（tokens / calls 两套，按 modelMetric 切换）
  const modelDates = [...new Set(modelRows.map((m) => m.date))].sort()
  const stackedSerie = modelDates.map((date) => {
    const row: Record<string, number | string> = { date }
    const byModel = new Map<string, { tok: number; calls: number }>()
    modelRows
      .filter((m) => m.date === date)
      .forEach((m) => {
        const cur = byModel.get(m.model_name) ?? { tok: 0, calls: 0 }
        cur.tok += tokensOf(m)
        cur.calls += m.calls
        byModel.set(m.model_name, cur)
      })
    let otherTok = 0
    let otherCalls = 0
    topModels.forEach((name) => {
      const cur = byModel.get(name)
      row[`${name}_t`] = cur ? cur.tok : 0
      row[`${name}_c`] = cur ? cur.calls : 0
    })
    byModel.forEach((cur, name) => {
      if (!topModels.includes(name)) {
        otherTok += cur.tok
        otherCalls += cur.calls
      }
    })
    if (otherTok > 0) {
      row.other_t = otherTok
      row.other_c = otherCalls
    }
    return row
  })
  const stackKeys = [...topModels, ...(stackedSerie.some((r) => r.other_t) ? ['other'] : [])]
  const metricSuffix = modelMetric === 'tokens' ? '_t' : '_c'
  const metricLabel = modelMetric === 'tokens' ? t('Tokens') : t('Calls')

  // Top5 趋势（按 tokens 选 Top4）
  const top4 = topRankData.slice(0, 4).map((x) => x.name)
  const top5Serie = modelDates.map((date) => {
    const row: Record<string, string | number> = { date }
    top4.forEach((name) => {
      const hit = modelRows.find((m) => m.date === date && m.model_name === name)
      row[name] = hit ? tokensOf(hit) : 0
    })
    return row
  })

  // 今日模型占比（donut）
  const lastDate = dayRows.length > 0 ? dayRows[dayRows.length - 1].date : ''
  const donutData = topRankData
    .filter(() => Boolean(lastDate))
    .map((x) => {
      const hit = modelRows.find((m) => m.date === lastDate && m.model_name === x.name)
      return { name: x.name, value: hit ? tokensOf(hit) : 0 }
    })
    .filter((x) => x.value > 0)

  // 区间汇总漏斗（辅助；转化率用队列口径，避免跨窗假高值）
  const funnelSummary = [
    { name: t('Registered'), value: totals.registered, color: '#f5b942', note: '' },
    { name: t('Activated (Key)'), value: totals.activated_key, color: '#2f6bff', note: `注册→激活(7日队列) ${funnelRates.regKey}` },
    { name: t('First Paid'), value: totals.first_paid, color: '#0f9d58', note: `激活→首付(14日队列) ${funnelRates.keyPay}` },
  ]
  const maxStep = Math.max(1, funnelSummary[0]?.value ?? 1)

  // 滚动锚点高亮
  useEffect(() => {
    const onScroll = () => {
      const ids = ['funnel', 'overview', 'models']
      let cur = ids[0]
      for (const id of ids) {
        const el = document.getElementById(`sec-${id}`)
        if (el && el.getBoundingClientRect().top <= 160) cur = id
      }
      setActiveSec(cur)
    }
    window.addEventListener('scroll', onScroll, { passive: true })
    onScroll()
    return () => window.removeEventListener('scroll', onScroll)
  }, [dayRows.length])

  const jump = (id: string) => {
    document.getElementById(`sec-${id}`)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    setActiveSec(id)
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Usage Report')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <div className='flex flex-wrap items-center gap-1'>
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
            <Skeleton className='h-96 w-full' />
          </div>
        ) : (
          <div className='space-y-8'>
            {/* 顶部锚点导航（整屏滚动） */}
            <div className='bg-muted/60 flex flex-wrap gap-1 rounded-lg p-1'>
              {[
                { id: 'funnel', label: '① 漏斗' },
                { id: 'overview', label: '② 用量总览' },
                { id: 'models', label: '③ 模型用量' },
              ].map((x) => (
                <Button
                  key={x.id}
                  size='sm'
                  variant={activeSec === x.id ? 'default' : 'ghost'}
                  onClick={() => jump(x.id)}
                >
                  {x.label}
                </Button>
              ))}
            </div>

            {/* ================= ① 漏斗 ================= */}
            <div className='space-y-4'>
              <SectionTitle no={1} title={t('Funnel (Register → Key → First Paid)')} hint={t('注册=enabled+verified；激活=首次建Key(人)；单位=人')} />
              <KpiGrid items={kpiFunnel} />

              {/* 每日明细表 */}
              <Card>
                <CardHeader>
                  <CardTitle className='flex flex-wrap items-center justify-between gap-2'>
                    {t('Daily Detail')}
                    <span className='text-muted-foreground text-xs font-normal'>
                      {t('UTC+0，末行=今日；金额=当日订单实收')}
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
                          <TableCell className='text-right'>{fmtBig(tokensOf(r))}</TableCell>
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

              {/* 双Y轴：注册/激活/首付柱 + 付费金额线 */}
              <Card>
                <CardHeader>
                  <CardTitle>{t('Daily Funnel — bars (people, left) + paid amount line (right)')}</CardTitle>
                </CardHeader>
                <CardContent>
                  <ResponsiveContainer width='100%' height={330}>
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

              {/* 转化率趋势（队列）+ 区间汇总漏斗 */}
              <div className='grid gap-6 xl:grid-cols-2'>
                <Card>
                  <CardHeader>
                    <CardTitle>{t('Conversion Trend (cohort, people)')}</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <p className='text-muted-foreground mb-2 text-xs'>
                      {t('单位=人；注册→激活率(7日队列)与激活→首付率(14日队列)，近 7/14 天队列未到期，柱末段留空。')}
                    </p>
                    <ResponsiveContainer width='100%' height={230}>
                      <BarChart data={convSerie} margin={{ top: 8, right: 8, left: 8, bottom: 8 }}>
                        <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' vertical={false} />
                        <XAxis dataKey='date' tick={{ fontSize: 11 }} tickFormatter={(d: string) => d.slice(5)} />
                        <YAxis tick={{ fontSize: 11 }} tickFormatter={(v: number) => `${v}%`} />
                        <Tooltip formatter={(value: unknown) => (value == null ? '' : `${String(value)}%`)} />
                        <Legend />
                        <Bar dataKey='注册→激活率(7日队列)' name={t('注册→激活率(7日队列)')} fill='#2f6bff' radius={[3, 3, 0, 0]} />
                        <Bar dataKey='激活→首付率(14日队列)' name={t('激活→首付率(14日队列)')} fill='#0f9d58' radius={[3, 3, 0, 0]} />
                      </BarChart>
                    </ResponsiveContainer>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader>
                    <CardTitle>{t('Range Funnel Summary (auxiliary)')}</CardTitle>
                  </CardHeader>
                  <CardContent className='space-y-4'>
                    {funnelSummary.map((s) => (
                      <div key={s.name}>
                        <div className='mb-1 flex items-center justify-between text-xs'>
                          <span className='font-semibold'>{s.name}</span>
                          <span className='text-muted-foreground'>
                            {num(s.value)} {s.note ? `· ${s.note}` : ''}
                          </span>
                        </div>
                        <div className='bg-muted h-5 w-full overflow-hidden rounded'>
                          <div
                            className='flex h-full items-center justify-end rounded pr-1 text-[10px] text-white'
                            style={{ width: `${Math.max(2, (s.value / maxStep) * 100)}%`, background: s.color }}
                          >
                            {num(s.value)}
                          </div>
                        </div>
                      </div>
                    ))}
                    <p className='text-muted-foreground text-xs'>
                      {t('转化率取已完成队列口径（7/14日），恒 ≤100%；宽度按区间事件数示意。')}
                    </p>
                  </CardContent>
                </Card>
              </div>
            </div>

            {/* ================= ② 用量总览 ================= */}
            <div className='space-y-4'>
              <SectionTitle no={2} title={t('Usage Overview')} hint={t('今日关键量 + 每日总量趋势 + 模型构成')} />
              <KpiGrid items={kpiOverview} />
              <div className='note-banner rounded-md bg-blue-50 px-3 py-2 text-xs leading-6 text-blue-900'>
                📌 {t('纯用户侧口径：只统计用户实际调用（次数/Tokens）与转化漏斗；成本未落地，可用 CSV 导出按外部价目离线核算。')}
              </div>

              <Card>
                <CardHeader>
                  <CardTitle>{t('Daily Calls & Tokens')}</CardTitle>
                </CardHeader>
                <CardContent>
                  <ResponsiveContainer width='100%' height={320}>
                    <ComposedChart data={combo} margin={{ top: 8, right: 8, left: 8, bottom: 8 }}>
                      <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' />
                      <XAxis dataKey='date' tick={{ fontSize: 11 }} tickFormatter={(d: string) => d.slice(5)} />
                      <YAxis yAxisId='l' tick={{ fontSize: 11 }} />
                      <YAxis yAxisId='r' orientation='right' tick={{ fontSize: 11 }} tickFormatter={(v: number) => fmtBig(v)} />
                      <Tooltip />
                      <Legend />
                      <Bar yAxisId='l' dataKey='calls' name={t('Calls')} fill='#7c8cf8' barSize={14} />
                      <Line yAxisId='r' type='monotone' dataKey='tokens' name={t('Tokens')} stroke='#2f6bff' strokeWidth={2} dot={false} />
                    </ComposedChart>
                  </ResponsiveContainer>
                </CardContent>
              </Card>

              <div className='grid gap-6 xl:grid-cols-2'>
                <Card>
                  <CardHeader>
                    <CardTitle>{t('Today Model Tokens Share')}</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <ResponsiveContainer width='100%' height={280}>
                      <PieChart>
                        <Pie data={donutData} dataKey='value' nameKey='name' innerRadius={60} outerRadius={95} paddingAngle={2}>
                          {donutData.map((_, i) => (
                            <Cell key={i} fill={MODEL_PALETTE[i % MODEL_PALETTE.length]} />
                          ))}
                        </Pie>
                        <Tooltip />
                        <Legend wrapperStyle={{ fontSize: 12 }} />
                      </PieChart>
                    </ResponsiveContainer>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader>
                    <CardTitle>{t('Range Top Models (tokens)')}</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <ResponsiveContainer width='100%' height={280}>
                      <BarChart data={topRankData} layout='vertical' margin={{ top: 8, right: 24, left: 8, bottom: 8 }}>
                        <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' horizontal={false} />
                        <XAxis type='number' tick={{ fontSize: 11 }} tickFormatter={(v: number) => fmtBig(v)} />
                        <YAxis type='category' dataKey='name' width={140} tick={{ fontSize: 11 }} />
                        <Tooltip />
                        <Bar dataKey='tokens' name={t('Tokens')} radius={[0, 4, 4, 0]}>
                          {topRankData.map((_, i) => (
                            <Cell key={i} fill={MODEL_PALETTE[i % MODEL_PALETTE.length]} />
                          ))}
                        </Bar>
                      </BarChart>
                    </ResponsiveContainer>
                  </CardContent>
                </Card>
              </div>
            </div>

            {/* ================= ③ 模型用量 ================= */}
            <div className='space-y-4'>
              <SectionTitle no={3} title={t('Model Usage')} hint={t('各模型每日规模与消长 + 汇总 + CSV 导出')} />
              <Card>
                <CardHeader>
                  <CardTitle className='flex flex-wrap items-center justify-between gap-2'>
                    {t('Daily Model Usage (stacked)')}
                    <span className='flex items-center gap-1'>
                      <Button size='sm' variant={modelMetric === 'calls' ? 'default' : 'outline'} onClick={() => setModelMetric('calls')}>
                        {t('Calls')}
                      </Button>
                      <Button size='sm' variant={modelMetric === 'tokens' ? 'default' : 'outline'} onClick={() => setModelMetric('tokens')}>
                        {t('Tokens')}
                      </Button>
                    </span>
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <ResponsiveContainer width='100%' height={360}>
                    <AreaChart data={stackedSerie} margin={{ top: 8, right: 8, left: 8, bottom: 8 }}>
                      <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' />
                      <XAxis dataKey='date' tick={{ fontSize: 11 }} tickFormatter={(d: string) => d.slice(5)} />
                      <YAxis tick={{ fontSize: 11 }} tickFormatter={(v: number) => fmtBig(v)} />
                      <Tooltip />
                      <Legend wrapperStyle={{ fontSize: 12 }} />
                      {stackKeys.map((name, i) => (
                        <Area
                          key={name}
                          type='monotone'
                          dataKey={`${name}${metricSuffix}`}
                          name={name === 'other' ? `${t('Other')} (${metricLabel})` : `${name} (${metricLabel})`}
                          stackId='m'
                          stroke={MODEL_PALETTE[i % MODEL_PALETTE.length]}
                          fill={MODEL_PALETTE[i % MODEL_PALETTE.length]}
                          fillOpacity={0.85}
                        />
                      ))}
                    </AreaChart>
                  </ResponsiveContainer>
                </CardContent>
              </Card>

              {/* 区间模型汇总 + CSV */}
              <Card>
                <CardHeader>
                  <CardTitle className='flex flex-wrap items-center justify-between gap-2'>
                    {t('Range Model Summary')}
                    <div className='flex gap-1'>
                      <Button size='sm' variant='outline' onClick={() => void downloadUsageReportCSV(days, 'daily')}>
                        ⬇ CSV(日漏斗)
                      </Button>
                      <Button size='sm' variant='outline' onClick={() => void downloadUsageReportCSV(days, 'models')}>
                        ⬇ CSV(日×模型)
                      </Button>
                    </div>
                  </CardTitle>
                </CardHeader>
                <CardContent className='overflow-x-auto'>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('Model')}</TableHead>
                        <TableHead className='text-right'>{t('Calls')}</TableHead>
                        <TableHead className='text-right'>{t('Calls Share')}</TableHead>
                        <TableHead className='text-right'>{t('Tokens')}</TableHead>
                        <TableHead className='text-right'>{t('Tokens Share')}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {topRankData.map((r) => (
                        <TableRow key={r.name}>
                          <TableCell>
                            <span
                              className='mr-2 inline-block h-2 w-2 rounded-sm'
                              style={{ background: MODEL_PALETTE[topRankData.indexOf(r) % MODEL_PALETTE.length] }}
                            />
                            {r.name}
                          </TableCell>
                          <TableCell className='text-right'>{num(r.calls)}</TableCell>
                          <TableCell className='text-right'>
                            {totalCalls > 0 ? `${((r.calls / totalCalls) * 100).toFixed(1)}%` : '-'}
                          </TableCell>
                          <TableCell className='text-right'>{fmtBig(r.tokens)}</TableCell>
                          <TableCell className='text-right'>
                            {totalTokens > 0 ? `${((r.tokens / totalTokens) * 100).toFixed(1)}%` : '-'}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </CardContent>
              </Card>

              {/* Top 模型日趋势 */}
              <Card>
                <CardHeader>
                  <CardTitle>{t('Top Models Daily Trend (tokens)')}</CardTitle>
                </CardHeader>
                <CardContent>
                  <ResponsiveContainer width='100%' height={260}>
                    <LineChart data={top5Serie} margin={{ top: 8, right: 8, left: 8, bottom: 8 }}>
                      <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' />
                      <XAxis dataKey='date' tick={{ fontSize: 11 }} tickFormatter={(d: string) => d.slice(5)} />
                      <YAxis tick={{ fontSize: 11 }} tickFormatter={(v: number) => fmtBig(v)} />
                      <Tooltip />
                      <Legend wrapperStyle={{ fontSize: 12 }} />
                      {top4.map((name, i) => (
                        <Line
                          key={name}
                          type='monotone'
                          dataKey={name}
                          name={name}
                          stroke={MODEL_PALETTE[i % MODEL_PALETTE.length]}
                          strokeWidth={2}
                          dot={false}
                        />
                      ))}
                    </LineChart>
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
