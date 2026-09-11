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
import type { EChartsOption } from 'echarts'
import { EChart } from '@/components/charts/echart'
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
  type UsageReportGroup,
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
  id,
  no,
  title,
  hint,
}: {
  id: string
  no: number
  title: string
  hint: string
}) {
  return (
    <div id={`sec-${id}`} className='scroll-mt-28 pt-2'>
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
  // 初始状态从 URL 读取（刷新/分享链接后保持分组与天数）
  const initialParams =
    typeof window === 'undefined' ? new URLSearchParams() : new URLSearchParams(window.location.search)
  const initialGroup: UsageReportGroup = initialParams.get('group') === 'all' ? 'all' : 'plg'
  const initialDaysRaw = Number(initialParams.get('days'))
  const initialDays = DAY_OPTIONS.includes(initialDaysRaw) ? initialDaysRaw : 30
  const [days, setDays] = useState(initialDays)
  const [group, setGroup] = useState<UsageReportGroup>(initialGroup)
  const [modelMetric, setModelMetric] = useState<'tokens' | 'calls'>('calls')
  const [activeSec, setActiveSec] = useState('funnel')
  const { data: res, isLoading, refetch } = useQuery({
    queryKey: usageReportQueryKeys.report(days, group),
    queryFn: () => getUsageReport(days, group),
  })
  const payload: UsageReportData | undefined = res?.data
  const dayRows: UsageReportDayRow[] = payload
    ? [...payload.days].sort((a, b) => a.date.localeCompare(b.date))
    : []
  const filling = Boolean(payload?.filling)
  const incomplete = dayRows.length < days
  const modelRows: UsageReportModelRow[] = payload ? [...payload.models] : []
  const today = dayRows.length > 0 ? dayRows[dayRows.length - 1] : undefined
  const yesterday = dayRows.length > 1 ? dayRows[dayRows.length - 2] : undefined
  const latestDate = today ? today.date : '-'

  const sumDays = (pick: (r: UsageReportDayRow) => number): number =>
    dayRows.reduce((acc, r) => acc + pick(r), 0)
  const totalTokens = sumDays((r) => tokensOf(r))
  const totalCalls = sumDays((r) => r.calls)
  const totalRegistered = sumDays((r) => r.registered)
  const totalActivatedDay = sumDays((r) => r.activated_day)
  const totalPaidDay = sumDays((r) => r.paid_day)

  // ---------- ① 漏斗（当天口径，人） ----------
  const dayRegKeyRate =
    today && today.registered > 0 ? Math.round((today.activated_day / today.registered) * 1000) / 10 : null
  const dayRegPayRate =
    today && today.registered > 0 ? Math.round((today.paid_day / today.registered) * 10000) / 100 : null
  const kpiFunnel: KpiDatum[] = today
    ? [
        { k: t('Registered (today)'), v: num(today.registered), d: `环比 ${deltaPeople(today.registered, yesterday?.registered)}` },
        { k: t('Activated (same-day)'), v: num(today.activated_day), d: `当天注册者当天建Key·环比 ${deltaPeople(today.activated_day, yesterday?.activated_day)}` },
        { k: t('First Paid (same-day)'), v: num(today.paid_day), d: `当天注册者当天首付·环比 ${deltaPeople(today.paid_day, yesterday?.paid_day)}` },
        { k: t('Paid Amount (today)'), v: usd(today.paid_usd), d: `当日实收·环比 ${deltaUsd(today.paid_usd, yesterday?.paid_usd)}` },
        { k: t('Reg→Activate rate (same-day)'), v: dayRegKeyRate == null ? '-' : `${dayRegKeyRate}%`, d: '激活÷注册（当天）' },
        { k: t('Reg→Pay rate (same-day)'), v: dayRegPayRate == null ? '-' : `${dayRegPayRate}%`, d: '首付÷注册（当天）' },
      ]
    : []

  const totals = {
    registered: totalRegistered,
    activated_day: totalActivatedDay,
    paid_day: totalPaidDay,
    paid_usd: sumDays((r) => r.paid_usd),
    calls: totalCalls,
    tokens: totalTokens,
  }

  // 转化率趋势（当天口径柱状图；今天进行中，数值会随时间略涨；
  // 注册=0 的日子返回 null，不渲染柱，避免把“无分母”当 0%）
  const convSerie = dayRows.map((r) => ({
    date: r.date,
    '注册→激活率(当天)':
      r.registered > 0 ? Math.round((r.activated_day / r.registered) * 1000) / 10 : null,
    '注册→首付率(当天)':
      r.registered > 0 ? Math.round((r.paid_day / r.registered) * 10000) / 100 : null,
  }))

  const rangeRegKeyRate =
    totalRegistered > 0 ? (totalActivatedDay / totalRegistered) * 100 : null
  const rangeRegPayRate =
    totalRegistered > 0 ? (totalPaidDay / totalRegistered) * 100 : null

  // 区间汇总漏斗（当天口径，辅助）
  const funnelSummary = [
    { name: '注册(合计)', value: totals.registered, color: '#f5b942' },
    { name: '激活(当天)', value: totals.activated_day, color: '#2f6bff' },
    { name: '首付(当天)', value: totals.paid_day, color: '#0f9d58' },
  ]
  const maxStep = Math.max(1, funnelSummary[0]?.value ?? 1)

  // ---------- ② 用量总览派生 ----------
  const combo = dayRows.map((r) => ({ date: r.date, calls: r.calls, tokens: tokensOf(r) }))
  const kpiOverview: KpiDatum[] = today
    ? [
        { k: t('Calls (today)'), v: num(today.calls), d: `区间合计 ${num(totalCalls)}` },
        { k: t('Tokens (today)'), v: fmtBig(tokensOf(today)), d: `区间合计 ${fmtBig(totalTokens)}` },
        { k: t('Registered (today)'), v: num(today.registered), d: `区间合计 ${num(totalRegistered)}` },
        { k: t('Activated (same-day, today)'), v: num(today.activated_day), d: `区间合计 ${num(totalActivatedDay)} 人` },
        { k: t('First Paid (same-day, today)'), v: num(today.paid_day), d: `区间合计 ${num(totalPaidDay)} 人` },
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

  const modelDates = [...new Set(modelRows.map((m) => m.date))].sort()
  // 模型名可能含 `.`/`/` 等字符，不能直接作为 Recharts dataKey（会被当路径解析），
  // 统一映射成安全 key：topModels[i] -> s{i}，'other' -> other。一次遍历预聚合，
  // 避免每个日期都 filter 一遍 modelRows。
  const byDateModel = new Map<string, Map<string, { tok: number; calls: number }>>()
  modelRows.forEach((m) => {
    let byModel = byDateModel.get(m.date)
    if (!byModel) {
      byModel = new Map<string, { tok: number; calls: number }>()
      byDateModel.set(m.date, byModel)
    }
    const cur = byModel.get(m.model_name) ?? { tok: 0, calls: 0 }
    cur.tok += tokensOf(m)
    cur.calls += m.calls
    byModel.set(m.model_name, cur)
  })
  const topModelKeys = topModels.map((_, i) => `s${i}`)
  const hasOther = [...byDateModel.values()].some((byModel) =>
    [...byModel.keys()].some((name) => !topModels.includes(name))
  )
  const stackSafeKeys = [...topModelKeys, ...(hasOther ? ['other'] : [])]
  const stackedSerie = modelDates.map((date) => {
    const row: Record<string, number | string> = { date }
    const byModel = byDateModel.get(date) ?? new Map()
    let otherTok = 0
    let otherCalls = 0
    topModels.forEach((name, i) => {
      const cur = byModel.get(name)
      row[`s${i}_t`] = cur ? cur.tok : 0
      row[`s${i}_c`] = cur ? cur.calls : 0
    })
    byModel.forEach((cur, name) => {
      if (!topModels.includes(name)) {
        otherTok += cur.tok
        otherCalls += cur.calls
      }
    })
    if (hasOther) {
      row.other_t = otherTok
      row.other_c = otherCalls
    }
    return row
  })
  const metricSuffix = modelMetric === 'tokens' ? '_t' : '_c'
  const metricLabel = modelMetric === 'tokens' ? t('Tokens') : t('Calls')
  const stackDisplayNames = [...topModels, ...(hasOther ? ['other'] : [])]

  const top4 = topRankData.slice(0, 4).map((x) => x.name)
  const top5Serie = modelDates.map((date) => {
    const row: Record<string, string | number> = { date }
    top4.forEach((name, i) => {
      const hit = modelRows.find((m) => m.date === date && m.model_name === name)
      row[`t${i}`] = hit ? tokensOf(hit) : 0
    })
    return row
  })

  const lastDate = dayRows.length > 0 ? dayRows[dayRows.length - 1].date : ''
  const donutData = topRankData
    .filter(() => Boolean(lastDate))
    .map((x) => {
      const hit = modelRows.find((m) => m.date === lastDate && m.model_name === x.name)
      return { name: x.name, value: hit ? tokensOf(hit) : 0 }
    })
    .filter((x) => x.value > 0)

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

  // 轮询策略：
  // - 后台正在回填(filling)：每 4s 刷新，不设上限；
  // - 数据未满但后台已停(失败/中断)：最多自动重试 staleRetryLimit 次后停止，
  //   提示用户手动重试，避免无限打接口。
  const staleRetryLimit = 6
  const [staleTicks, setStaleTicks] = useState(0)
  const staleStopped = incomplete && !filling && staleTicks >= staleRetryLimit
  useEffect(() => {
    if (!incomplete) return
    if (filling) {
      const id = setTimeout(() => {
        void refetch()
      }, 4000)
      return () => clearTimeout(id)
    }
    if (staleTicks >= staleRetryLimit) return
    const id = setTimeout(() => {
      setStaleTicks((n) => n + 1)
      void refetch()
    }, 4000)
    return () => clearTimeout(id)
  }, [incomplete, filling, staleTicks, refetch])

  // 分组/天数写回 URL：刷新或把链接发给同事时保持一致
  useEffect(() => {
    if (typeof window === 'undefined') return
    const params = new URLSearchParams(window.location.search)
    params.set('group', group)
    params.set('days', String(days))
    const query = params.toString()
    window.history.replaceState(
      null,
      '',
      `${window.location.pathname}${query ? `?${query}` : ''}${window.location.hash}`
    )
  }, [group, days])

  const jump = (id: string) => {
    document.getElementById(`sec-${id}`)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    setActiveSec(id)
  }

  // ---------- ECharts options（图例默认可点击开关系列） ----------
  const dates = dayRows.map((r) => r.date)
  const dateLabel = (v: string): string => v.slice(5)
  const peopleFmt = (v: number): string => (v >= 10000 ? `${Math.round(v / 10000)}万` : String(v))
  const moneyFmt = (v: number): string => (v >= 1000 ? `$${Math.round(v / 1000)}k` : `$${v}`)

  const funnelOption: EChartsOption = {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    legend: { top: 0, type: 'scroll' },
    grid: { left: 56, right: 64, top: 40, bottom: 28 },
    xAxis: { type: 'category', data: dates, axisLabel: { formatter: dateLabel } },
    yAxis: [
      { type: 'value', name: t('人'), axisLabel: { formatter: peopleFmt } },
      { type: 'value', name: '$', splitLine: { show: false }, axisLabel: { formatter: moneyFmt } },
    ],
    series: [
      {
        name: t('Registered'),
        type: 'bar',
        barMaxWidth: 12,
        itemStyle: { color: '#f5b942' },
        data: dayRows.map((r) => r.registered),
      },
      {
        name: t('Activated (same-day)'),
        type: 'bar',
        barMaxWidth: 12,
        itemStyle: { color: '#2f6bff' },
        data: dayRows.map((r) => r.activated_day),
      },
      {
        name: t('First Paid (same-day)'),
        type: 'bar',
        barMaxWidth: 12,
        itemStyle: { color: '#0f9d58' },
        data: dayRows.map((r) => r.paid_day),
      },
      {
        name: t('Paid Amount $'),
        type: 'line',
        yAxisIndex: 1,
        smooth: true,
        symbol: 'none',
        lineStyle: { color: '#9c36b5', type: 'dashed', width: 2 },
        itemStyle: { color: '#9c36b5' },
        data: dayRows.map((r) => r.paid_usd),
      },
    ],
  }

  const convOption: EChartsOption = {
    tooltip: {
      trigger: 'axis',
      valueFormatter: (v) => (v == null ? '-' : `${Number(v).toFixed(2)}%`),
    },
    legend: { top: 0 },
    grid: { left: 46, right: 20, top: 34, bottom: 28 },
    xAxis: { type: 'category', data: dates, axisLabel: { formatter: dateLabel } },
    yAxis: { type: 'value', axisLabel: { formatter: '{value}%' } },
    series: [
      {
        name: t('注册→激活率(当天)'),
        type: 'bar',
        itemStyle: { color: '#2f6bff' },
        data: convSerie.map((c) => c['注册→激活率(当天)']),
      },
      {
        name: t('注册→首付率(当天)'),
        type: 'bar',
        itemStyle: { color: '#0f9d58' },
        data: convSerie.map((c) => c['注册→首付率(当天)']),
      },
    ],
  }

  const usageComboOption: EChartsOption = {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    legend: { top: 0, type: 'scroll' },
    grid: { left: 58, right: 62, top: 40, bottom: 28 },
    xAxis: { type: 'category', data: dates, axisLabel: { formatter: dateLabel } },
    yAxis: [
      { type: 'value', name: t('Calls'), axisLabel: { formatter: peopleFmt } },
      { type: 'value', name: t('Tokens'), splitLine: { show: false }, axisLabel: { formatter: (v: number) => fmtBig(v) } },
    ],
    series: [
      { name: t('Calls'), type: 'bar', barMaxWidth: 14, itemStyle: { color: '#7c8cf8' }, data: combo.map((c) => c.calls) },
      {
        name: t('Tokens'),
        type: 'line',
        yAxisIndex: 1,
        smooth: true,
        symbol: 'none',
        lineStyle: { color: '#2f6bff', width: 2 },
        itemStyle: { color: '#2f6bff' },
        data: combo.map((c) => c.tokens),
      },
    ],
  }

  const donutOption: EChartsOption = {
    tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
    legend: { type: 'scroll', orient: 'vertical', right: 8, top: 'middle' },
    series: [
      {
        type: 'pie',
        radius: ['42%', '68%'],
        center: ['38%', '50%'],
        label: { show: false },
        emphasis: { label: { show: true, formatter: '{b}\n{d}%' } },
        data: donutData.map((d, i) => ({
          name: d.name,
          value: d.value,
          itemStyle: { color: MODEL_PALETTE[i % MODEL_PALETTE.length] },
        })),
      },
    ],
  }

  const rankReversed = [...topRankData].reverse()
  const topRankOption: EChartsOption = {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    grid: { left: 140, right: 48, top: 16, bottom: 28 },
    xAxis: { type: 'value', axisLabel: { formatter: (v: number) => fmtBig(v) } },
    yAxis: { type: 'category', data: rankReversed.map((x) => x.name), axisLabel: { fontSize: 11 } },
    series: [
      {
        type: 'bar',
        barMaxWidth: 16,
        data: rankReversed.map((x, i) => ({
          value: x.tokens,
          itemStyle: {
            color: MODEL_PALETTE[(topRankData.length - 1 - i) % MODEL_PALETTE.length],
          },
        })),
      },
    ],
  }

  const stackOption: EChartsOption = {
    tooltip: { trigger: 'axis' },
    legend: { top: 0, type: 'scroll' },
    grid: { left: 62, right: 24, top: 42, bottom: 28 },
    xAxis: { type: 'category', boundaryGap: false, data: dates, axisLabel: { formatter: dateLabel } },
    yAxis: { type: 'value', axisLabel: { formatter: (v: number) => fmtBig(v) } },
    series: stackSafeKeys.map((key, i) => {
      const label = stackDisplayNames[i]
      const color = MODEL_PALETTE[i % MODEL_PALETTE.length]
      return {
        name: label === 'other' ? `${t('Other')} (${metricLabel})` : `${label} (${metricLabel})`,
        type: 'line',
        stack: 'm',
        symbol: 'none',
        lineStyle: { width: 0 },
        areaStyle: { opacity: 0.85, color },
        itemStyle: { color },
        data: stackedSerie.map((r) => Number(r[`${key}${metricSuffix}`] ?? 0)),
      }
    }),
  }

  const topTrendOption: EChartsOption = {
    tooltip: { trigger: 'axis' },
    legend: { top: 0, type: 'scroll' },
    grid: { left: 62, right: 24, top: 34, bottom: 28 },
    xAxis: { type: 'category', boundaryGap: false, data: dates, axisLabel: { formatter: dateLabel } },
    yAxis: { type: 'value', axisLabel: { formatter: (v: number) => fmtBig(v) } },
    series: top4.map((name, i) => ({
      name,
      type: 'line',
      smooth: true,
      symbol: 'none',
      lineStyle: { width: 2, color: MODEL_PALETTE[i % MODEL_PALETTE.length] },
      itemStyle: { color: MODEL_PALETTE[i % MODEL_PALETTE.length] },
      data: top5Serie.map((r) => Number(r[`t${i}`] ?? 0)),
    })),
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Usage Report')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <div className='flex flex-wrap items-center gap-1'>
          <Button
            size='sm'
            variant={group === 'plg' ? 'default' : 'outline'}
            onClick={() => setGroup('plg')}
          >
            {t('PLG 分组')}
          </Button>
          <Button
            size='sm'
            variant={group === 'all' ? 'default' : 'outline'}
            onClick={() => setGroup('all')}
          >
            {t('全部分组')}
          </Button>
        </div>
        <div className='flex flex-wrap items-center gap-1'>
          {DAY_OPTIONS.map((d) => (
            <Button key={d} size='sm' variant={days === d ? 'default' : 'outline'} onClick={() => setDays(d)}>
              {d}天
            </Button>
          ))}
        </div>
        <Button size='sm' variant='outline' onClick={() => void downloadUsageReportCSV(days, 'daily', group)}>
          ⬇ CSV(日漏斗)
        </Button>
        <Button size='sm' variant='outline' onClick={() => void downloadUsageReportCSV(days, 'models', group)}>
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
            {filling && (
              <div className='rounded-md bg-amber-50 px-3 py-2 text-xs leading-6 text-amber-900'>
                ⏳ {t('首次历史回填仍在后台进行（视数据量约数秒~1 分钟），本页每 4 秒自动刷新，已算好的日期先显示。')}
              </div>
            )}
            {staleStopped && (
              <div className='flex flex-wrap items-center gap-2 rounded-md bg-red-50 px-3 py-2 text-xs leading-6 text-red-900'>
                <span>{t('历史回填多次未能完成（当前仅部分日期有数据）。请检查服务日志，或点击重试。')}</span>
                <Button
                  size='sm'
                  variant='outline'
                  onClick={() => {
                    setStaleTicks(0)
                    void refetch()
                  }}
                >
                  {t('重试')}
                </Button>
              </div>
            )}
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

            {/* ============ ① 漏斗（当天口径） ============ */}
            <div className='space-y-4'>
              <SectionTitle
                id='funnel'
                no={1}
                title={t('Funnel (same-day, people)')}
                hint={`${t('注册=enabled+verified；激活=当天注册者当天首建Key(人)；首付=当天注册者当天首付(人)——C端当天口径')} · ${group === 'plg' ? t('PLG 分组') : t('全部分组')}`}
              />
              <KpiGrid items={kpiFunnel} />

              <Card>
                <CardHeader>
                  <CardTitle className='flex flex-wrap items-center justify-between gap-2'>
                    {t('Daily Detail')}
                    <span className='text-muted-foreground text-xs font-normal'>
                      {group === 'plg' ? t('PLG 分组') : t('全部分组')} · {t('数据截至')} {latestDate} (UTC+0) ·{' '}
                      {t('激活/首付列 = 该日注册的人中当天转化的(人)，恒 ≤ 注册；金额为当日实收；今日为进行中数据')}
                    </span>
                  </CardTitle>
                </CardHeader>
                <CardContent className='overflow-x-auto'>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('Date')}</TableHead>
                        <TableHead className='text-right'>{t('Registered')}</TableHead>
                        <TableHead className='text-right'>{t('Activated (same-day)')}</TableHead>
                        <TableHead className='text-right'>{t('First Paid (same-day)')}</TableHead>
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
                          <TableCell className='text-right'>{num(r.activated_day)}</TableCell>
                          <TableCell className='text-right'>{num(r.paid_day)}</TableCell>
                          <TableCell className='text-right'>{usd(r.paid_usd)}</TableCell>
                          <TableCell className='text-right'>{num(r.calls)}</TableCell>
                          <TableCell className='text-right'>{fmtBig(tokensOf(r))}</TableCell>
                        </TableRow>
                      ))}
                      <TableRow className='border-t-2 font-bold'>
                        <TableCell>{t('Total')} ({dayRows.length}天)</TableCell>
                        <TableCell className='text-right'>{num(totals.registered)}</TableCell>
                        <TableCell className='text-right'>{num(totals.activated_day)}</TableCell>
                        <TableCell className='text-right'>{num(totals.paid_day)}</TableCell>
                        <TableCell className='text-right'>{usd(totals.paid_usd)}</TableCell>
                        <TableCell className='text-right'>{num(totals.calls)}</TableCell>
                        <TableCell className='text-right'>{fmtBig(totals.tokens)}</TableCell>
                      </TableRow>
                    </TableBody>
                  </Table>
                </CardContent>
              </Card>

              {/* 双 Y 轴：注册/激活/首付柱（当天，人）+ 付费金额线 */}
              <Card>
                <CardHeader>
                  <CardTitle>{t('Daily Funnel — same-day people bars (left) + paid amount line (right)')}</CardTitle>
                </CardHeader>
                <CardContent>
                  <EChart option={funnelOption} height={330} />
                </CardContent>
              </Card>

              {/* 转化率趋势（当天口径，柱状图）+ 区间汇总漏斗 */}
              <div className='grid gap-6 xl:grid-cols-2'>
                <Card>
                  <CardHeader>
                    <CardTitle>{t('Same-day Conversion Trend (bars)')}</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <p className='text-muted-foreground mb-2 text-xs'>
                      {t('单位=人；注册→激活率(当天)=当天激活÷注册；注册→首付率(当天)=当天首付÷注册；今天为进行中。')}
                    </p>
                    <EChart option={convOption} height={230} />
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader>
                    <CardTitle>{t('Range Funnel Summary (same-day, auxiliary)')}</CardTitle>
                  </CardHeader>
                  <CardContent className='space-y-4'>
                    {funnelSummary.map((s) => (
                      <div key={s.name}>
                        <div className='mb-1 flex items-center justify-between text-xs'>
                          <span className='font-semibold'>{s.name}</span>
                          <span className='text-muted-foreground'>{num(s.value)}</span>
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
                      {t('区间(当天口径)：注册→激活率')} {rangeRegKeyRate == null ? '-' : rangeRegKeyRate.toFixed(2)}%
                      {' · '}
                      {t('注册→首付率')} {rangeRegPayRate == null ? '-' : rangeRegPayRate.toFixed(3)}%
                    </p>
                  </CardContent>
                </Card>
              </div>
            </div>

            {/* ============ ② 用量总览 ============ */}
            <div className='space-y-4'>
              <SectionTitle id='overview' no={2} title={t('Usage Overview')} hint={t('今日关键量 + 每日总量趋势 + 模型构成')} />
              <KpiGrid items={kpiOverview} />
              <div className='rounded-md bg-blue-50 px-3 py-2 text-xs leading-6 text-blue-900'>
                📌 {t('纯用户侧口径：只统计用户实际调用（次数/Tokens）与当天漏斗转化；成本未落地，可用 CSV 导出按外部价目离线核算。')}
              </div>

              <Card>
                <CardHeader>
                  <CardTitle>{t('Daily Calls & Tokens')}</CardTitle>
                </CardHeader>
                <CardContent>
                  <EChart option={usageComboOption} height={320} />
                </CardContent>
              </Card>

              <div className='grid gap-6 xl:grid-cols-2'>
                <Card>
                  <CardHeader>
                    <CardTitle>{t('Today Model Tokens Share')}</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <EChart option={donutOption} height={280} />
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader>
                    <CardTitle>{t('Range Top Models (tokens)')}</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <EChart option={topRankOption} height={280} />
                  </CardContent>
                </Card>
              </div>
            </div>

            {/* ============ ③ 模型用量 ============ */}
            <div className='space-y-4'>
              <SectionTitle id='models' no={3} title={t('Model Usage')} hint={t('各模型每日规模与消长 + 汇总 + CSV 导出')} />
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
                  <EChart option={stackOption} height={360} />
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle className='flex flex-wrap items-center justify-between gap-2'>
                    {t('Range Model Summary')}
                    <div className='flex gap-1'>
                      <Button size='sm' variant='outline' onClick={() => void downloadUsageReportCSV(days, 'daily', group)}>
                        ⬇ CSV(日漏斗)
                      </Button>
                      <Button size='sm' variant='outline' onClick={() => void downloadUsageReportCSV(days, 'models', group)}>
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

              <Card>
                <CardHeader>
                  <CardTitle>{t('Top Models Daily Trend (tokens)')}</CardTitle>
                </CardHeader>
                <CardContent>
                  <EChart option={topTrendOption} height={260} />
                </CardContent>
              </Card>
            </div>
          </div>
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
