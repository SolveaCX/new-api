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
import z from 'zod'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { SupplyChain } from '@/features/supply-chain'
import {
  SUPPLY_CHAIN_TABS,
  type SupplyChainSearchState,
} from '@/features/supply-chain/contracts'
import {
  getShanghaiNaturalMonth,
  isNaturalMonth,
} from '@/features/supply-chain/lib/time'

const positiveId = z.number().int().positive().optional().catch(undefined)

export const supplyChainSearchSchema = z.object({
  tab: z.enum(SUPPLY_CHAIN_TABS).default('report').catch('report'),
  month: z
    .string()
    .refine(isNaturalMonth)
    .default(getShanghaiNaturalMonth())
    .catch(getShanghaiNaturalMonth()),
  page: z.number().int().min(1).max(100_000).default(1).catch(1),
  pageSize: z.number().int().min(1).max(100).default(20).catch(20),
  filter: z.string().trim().max(128).default('').catch(''),
  status: z.enum(['active', 'inactive']).optional().catch(undefined),
  supplierId: positiveId,
  contractId: positiveId,
  channelId: positiveId,
  userId: positiveId,
})

export const Route = createFileRoute('/_authenticated/supply-chain/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()
    if (!auth.user || auth.user.role < ROLE.ADMIN) {
      throw redirect({ to: '/403' })
    }
  },
  validateSearch: supplyChainSearchSchema,
  component: SupplyChainRoute,
})

function SupplyChainRoute() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  return (
    <SupplyChain
      search={search}
      onSearchChange={(patch: Partial<SupplyChainSearchState>) => {
        void navigate({
          replace: true,
          search: (previous) => ({ ...previous, ...patch }),
        })
      }}
    />
  )
}
