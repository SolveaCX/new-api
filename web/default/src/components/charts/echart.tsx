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
import { useEffect, useRef } from 'react'
import * as echarts from 'echarts'
import type { EChartsOption } from 'echarts'
import { useTheme } from '@/context/theme-provider'

export interface EChartProps {
  option: EChartsOption
  /** CSS height of the chart box (default 320px) */
  height?: number | string
  className?: string
}

/**
 * Minimal ECharts binding: initialises once, follows the app theme, resizes
 * with its container and disposes on unmount. Legends are interactive by
 * default in ECharts (click to toggle a series), which is why the usage report
 * uses this instead of the Recharts wrappers.
 */
export function EChart({ option, height = 320, className }: EChartProps) {
  const boxRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<echarts.ECharts | null>(null)
  const { resolvedTheme } = useTheme()

  useEffect(() => {
    const el = boxRef.current
    if (!el) return
    const chart = echarts.init(el, resolvedTheme === 'dark' ? 'dark' : undefined)
    chartRef.current = chart
    const observer = new ResizeObserver(() => chart.resize())
    observer.observe(el)
    return () => {
      observer.disconnect()
      chart.dispose()
      chartRef.current = null
    }
  }, [resolvedTheme])

  useEffect(() => {
    chartRef.current?.setOption(option, true)
  }, [option])

  return <div ref={boxRef} className={className} style={{ width: '100%', height }} />
}
