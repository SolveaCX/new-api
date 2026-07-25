import type { SupplierHistoricalSeriesPoint } from '../types'

export function historicalImportProgress(
  processedCount: number,
  candidateCount: number
): number {
  if (candidateCount <= 0) return 0
  return Math.min(100, Math.max(0, (processedCount / candidateCount) * 100))
}

export interface HistoricalSeriesRollup {
  date: string
  sourceCount: number
  unknownCount: number
  unassignedCount: number
  salesUnknownCount: number
  costUnknownCount: number
  grossUnknownCount: number
  salesMicroUsd: bigint
  costMicroUsd: bigint
  grossMicroUsd: bigint
}

export function rollupHistoricalSeries(
  points: Array<
    Pick<
      SupplierHistoricalSeriesPoint,
      | 'date'
      | 'source_request_count'
      | 'unassigned_request_count'
      | 'official_list_unknown_count'
      | 'sales_unknown_count'
      | 'procurement_cost_unknown_count'
      | 'gross_profit_unknown_count'
      | 'sales_micro_usd'
      | 'procurement_cost_micro_usd'
      | 'gross_profit_micro_usd'
    >
  >
): HistoricalSeriesRollup[] {
  const byDate = new Map<string, HistoricalSeriesRollup>()
  for (const point of points) {
    const current = byDate.get(point.date) ?? {
      date: point.date,
      sourceCount: 0,
      unknownCount: 0,
      unassignedCount: 0,
      salesUnknownCount: 0,
      costUnknownCount: 0,
      grossUnknownCount: 0,
      salesMicroUsd: 0n,
      costMicroUsd: 0n,
      grossMicroUsd: 0n,
    }
    current.sourceCount += point.source_request_count
    current.unknownCount += point.official_list_unknown_count
    current.unassignedCount += point.unassigned_request_count
    current.salesUnknownCount += point.sales_unknown_count
    current.costUnknownCount += point.procurement_cost_unknown_count
    current.grossUnknownCount += point.gross_profit_unknown_count
    current.salesMicroUsd += BigInt(point.sales_micro_usd)
    current.costMicroUsd += BigInt(point.procurement_cost_micro_usd)
    current.grossMicroUsd += BigInt(point.gross_profit_micro_usd)
    byDate.set(point.date, current)
  }
  return [...byDate.values()].sort((a, b) => a.date.localeCompare(b.date))
}
