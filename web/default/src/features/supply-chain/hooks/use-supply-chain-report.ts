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
import axios from 'axios'
import {
  keepPreviousData,
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import {
  getDailyReports,
  getReportFreshness,
  getReportOverview,
  getReportTrend,
  listReportBreakdown,
  listReportChannels,
  listReportContracts,
  rerunDailyReport,
} from '../api'
import { getNextOffset, mergeOffsetPages } from '../lib/pagination'
import { supplyChainQueryKeys } from '../query-keys'
import type {
  SupplierDailyReportRerunVariables,
  SupplierReportPageQuery,
  SupplierReportQuery,
} from '../types'

const REPORT_STALE_TIME = 30_000
const FRESHNESS_REFRESH_INTERVAL = 60_000

export function useSupplyChainReportOverview(
  query: SupplierReportQuery,
  enabled = true
) {
  return useQuery({
    queryKey: supplyChainQueryKeys.reports.overview(query),
    queryFn: async () => (await getReportOverview(query)).data,
    enabled,
    staleTime: REPORT_STALE_TIME,
  })
}

export function useSupplyChainReportTrend(
  query: SupplierReportQuery,
  enabled = true
) {
  return useQuery({
    queryKey: supplyChainQueryKeys.reports.trend(query),
    queryFn: async () => (await getReportTrend(query)).data,
    enabled,
    staleTime: REPORT_STALE_TIME,
  })
}

export function useSupplyChainReportContracts(
  query: SupplierReportPageQuery,
  enabled = true
) {
  const initialOffset = query.offset ?? 0
  return useInfiniteQuery({
    queryKey: supplyChainQueryKeys.reports.contracts(query),
    queryFn: async ({ pageParam }) =>
      (await listReportContracts({ ...query, offset: pageParam })).data,
    initialPageParam: initialOffset,
    getNextPageParam: getNextOffset,
    select: (data) => mergeOffsetPages(data.pages),
    enabled,
    staleTime: REPORT_STALE_TIME,
  })
}

export function useSupplyChainReportChannels(
  query: SupplierReportPageQuery,
  enabled = true
) {
  const initialOffset = query.offset ?? 0
  return useInfiniteQuery({
    queryKey: supplyChainQueryKeys.reports.channels(query),
    queryFn: async ({ pageParam }) =>
      (await listReportChannels({ ...query, offset: pageParam })).data,
    initialPageParam: initialOffset,
    getNextPageParam: getNextOffset,
    select: (data) => mergeOffsetPages(data.pages),
    enabled,
    staleTime: REPORT_STALE_TIME,
  })
}

export function useSupplyChainReportBreakdown(
  query: SupplierReportPageQuery,
  enabled = true
) {
  const initialOffset = query.offset ?? 0
  return useInfiniteQuery({
    queryKey: supplyChainQueryKeys.reports.breakdown(query),
    queryFn: async ({ pageParam }) =>
      (await listReportBreakdown({ ...query, offset: pageParam })).data,
    initialPageParam: initialOffset,
    getNextPageParam: getNextOffset,
    select: (data) => mergeOffsetPages(data.pages),
    enabled,
    staleTime: REPORT_STALE_TIME,
  })
}

export function useSupplyChainReportFreshness(enabled = true) {
  return useQuery({
    queryKey: supplyChainQueryKeys.reports.freshness(),
    queryFn: async () => (await getReportFreshness()).data,
    enabled,
    staleTime: REPORT_STALE_TIME,
    refetchInterval: enabled ? FRESHNESS_REFRESH_INTERVAL : false,
  })
}

export function useSupplyChainDailyReports(
  query: SupplierReportQuery,
  enabled = true
) {
  return useQuery({
    queryKey: supplyChainQueryKeys.reports.daily.list(query),
    queryFn: async () => (await getDailyReports(query)).data,
    enabled,
    staleTime: REPORT_STALE_TIME,
    placeholderData: keepPreviousData,
  })
}

export function useSupplyChainDailyReportRerun() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (variables: SupplierDailyReportRerunVariables) =>
      rerunDailyReport(variables),
    retry: (failureCount, error) =>
      failureCount < 1 &&
      axios.isAxiosError(error) &&
      error.response === undefined,
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: supplyChainQueryKeys.reports.daily.all(),
      })
    },
  })
}
