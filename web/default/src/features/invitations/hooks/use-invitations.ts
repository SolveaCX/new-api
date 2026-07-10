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
import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
} from '@tanstack/react-query'
import i18next from 'i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { getSelf } from '@/lib/api'
import { getAffiliateCode, getInvitations, transferAffiliateUSD } from '../api'
import { INVITATION_PAGE_SIZE } from '../types'

class InvitationTransferBusinessError extends Error {}

export function createInvitationQueryOptions(page: number) {
  return {
    queryKey: ['invitations', page],
    queryFn: () => getInvitations(page, INVITATION_PAGE_SIZE),
    placeholderData: keepPreviousData,
  }
}

export function createInvitationTransferMutationOptions(
  queryClient: QueryClient
) {
  return {
    mutationFn: async (amountUSD: number) => {
      const response = await transferAffiliateUSD(amountUSD)
      if (response.success !== true) {
        throw new InvitationTransferBusinessError(
          response.message || i18next.t('Transfer failed')
        )
      }
      return response
    },
    onSuccess: async () => {
      const selfResponse = await getSelf().catch(() => null)
      if (selfResponse?.data) {
        useAuthStore.getState().auth.setUser(selfResponse.data)
      }

      await queryClient.invalidateQueries({ queryKey: ['invitations'] })
      toast.success(i18next.t('Transfer successful'))
    },
    onError: (error: Error) => {
      toast.error(
        error instanceof InvitationTransferBusinessError
          ? error.message
          : i18next.t('Transfer failed')
      )
    },
  }
}

export function useInvitations(page: number) {
  const queryClient = useQueryClient()

  const invitationsQuery = useQuery(createInvitationQueryOptions(page))

  const codeQuery = useQuery({
    queryKey: ['affiliate-code'],
    queryFn: getAffiliateCode,
  })

  const transferMutation = useMutation(
    createInvitationTransferMutationOptions(queryClient)
  )

  return {
    invitationsQuery,
    codeQuery,
    transferMutation,
  }
}
