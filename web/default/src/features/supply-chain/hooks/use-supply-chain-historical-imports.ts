import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { listHistoricalImportSummaries, listHistoricalImports } from '../api'
import { supplyChainQueryKeys } from '../query-keys'
import type { SupplierHistoricalImport } from '../types'

const HISTORICAL_REFRESH_INTERVAL = 10_000

export function useHistoricalImportList(page: number, pageSize: number) {
  const params = { p: page, page_size: pageSize }
  return useQuery({
    queryKey: supplyChainQueryKeys.historicalImports.list(params),
    queryFn: async () => (await listHistoricalImports(params)).data,
    refetchInterval: (query) => {
      const data = query.state.data
      return data?.items.some((item) =>
        ['pending', 'running'].includes(item.status)
      )
        ? HISTORICAL_REFRESH_INTERVAL
        : false
    },
  })
}

export function useCompletedHistoricalSeries(
  item: SupplierHistoricalImport | undefined
) {
  return useInfiniteQuery({
    queryKey: supplyChainQueryKeys.historicalImports.series(item?.id ?? 0),
    queryFn: async ({ pageParam }) => {
      const cursor = pageParam
      return (
        await listHistoricalImportSummaries(item!.id, {
          start_date: item!.start_date,
          end_date: item!.end_date,
          limit: 200,
          after_date: cursor?.date,
          after_scope: cursor?.statistics_scope,
          after_supplier_id: cursor?.supplier_id,
        })
      ).data
    },
    initialPageParam: null as {
      date: string
      statistics_scope: string
      supplier_id: number
    } | null,
    getNextPageParam: (lastPage) =>
      lastPage.has_more ? lastPage.next_cursor : undefined,
    enabled: item?.status === 'completed',
  })
}
