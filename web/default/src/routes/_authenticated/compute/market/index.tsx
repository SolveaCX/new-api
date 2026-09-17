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
import { createFileRoute } from '@tanstack/react-router'
import { ComputeMarket } from '@/features/compute-market'
import {
  COMPUTE_MARKET_TABS,
  type ComputeMarketTab,
} from '@/features/compute-market/keys'

function validateSearch(search: Record<string, unknown>): {
  tab?: ComputeMarketTab
} {
  const tab = typeof search.tab === 'string' ? search.tab : ''
  return (COMPUTE_MARKET_TABS as readonly string[]).includes(tab)
    ? { tab: tab as ComputeMarketTab }
    : {}
}

// Compute Market: buyers post GPU requests, verified suppliers bid, flatkey
// matches and escrows. Any signed-in user may post; bidding needs a verified
// supplier profile.
export const Route = createFileRoute('/_authenticated/compute/market/')({
  validateSearch,
  component: ComputeMarketRoute,
})

function ComputeMarketRoute() {
  const { tab } = Route.useSearch()
  return <ComputeMarket tab={tab ?? 'requests'} />
}
