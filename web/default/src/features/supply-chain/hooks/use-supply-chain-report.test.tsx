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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { describe, expect, test } from 'bun:test'
import { renderToStaticMarkup } from 'react-dom/server'
import type { SupplierReportPageQuery } from '../types'
import {
  useSupplyChainReportBreakdown,
  useSupplyChainReportChannels,
  useSupplyChainReportContracts,
  useSupplyChainReportFreshness,
  useSupplyChainReportOverview,
  useSupplyChainReportTrend,
} from './use-supply-chain-report'

const query: SupplierReportPageQuery = {
  month: '2026-09',
  limit: 20,
  offset: 0,
}

function DisabledReportProbe() {
  const overview = useSupplyChainReportOverview(query, false)
  const trend = useSupplyChainReportTrend(query, false)
  const contracts = useSupplyChainReportContracts(query, false)
  const channels = useSupplyChainReportChannels(query, false)
  const breakdown = useSupplyChainReportBreakdown(query, false)
  const freshness = useSupplyChainReportFreshness(false)
  const disabled = [
    overview,
    trend,
    contracts,
    channels,
    breakdown,
    freshness,
  ].every((result) => !result.isEnabled && result.fetchStatus === 'idle')
  return <span>{disabled ? 'all-disabled' : 'query-enabled'}</span>
}

describe('supply-chain report queries', () => {
  test('disables every report query and freshness polling outside the report tab', () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    const html = renderToStaticMarkup(
      <QueryClientProvider client={queryClient}>
        <DisabledReportProbe />
      </QueryClientProvider>
    )

    expect(html).toContain('all-disabled')
    expect(
      queryClient
        .getQueryCache()
        .getAll()
        .every(
          (cachedQuery) =>
            cachedQuery.options.enabled === false &&
            cachedQuery.state.fetchStatus === 'idle'
        )
    ).toBe(true)
  })
})
