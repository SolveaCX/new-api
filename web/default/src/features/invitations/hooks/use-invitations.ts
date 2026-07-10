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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import i18next from 'i18next'
import { toast } from 'sonner'
import { getSelf } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'
import {
  getAffiliateCode,
  getInvitations,
  transferAffiliateQuota,
} from '../api'
import { INVITATION_PAGE_SIZE } from '../types'

export function useInvitations(page: number) {
  const queryClient = useQueryClient()

  const invitationsQuery = useQuery({
    queryKey: ['invitations', page],
    queryFn: () => getInvitations(page, INVITATION_PAGE_SIZE),
  })

  const codeQuery = useQuery({
    queryKey: ['affiliate-code'],
    queryFn: getAffiliateCode,
  })

  const transferMutation = useMutation({
    mutationFn: async (quota: number) => {
      const response = await transferAffiliateQuota(quota)
      if (response.success === false) {
        throw new Error(response.message || i18next.t('Transfer failed'))
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
    onError: (error) => {
      toast.error(
        error instanceof Error && error.message
          ? i18next.t(error.message)
          : i18next.t('Transfer failed')
      )
    },
  })

  return {
    invitationsQuery,
    codeQuery,
    transferMutation,
  }
}
