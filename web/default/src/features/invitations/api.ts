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
import { api } from '@/lib/api'
import type {
  AffiliateCodeResponse,
  AffiliateTransferResponse,
  InvitationPageResponse,
} from './types'

export function buildInvitationListPath(
  page: number,
  pageSize: number
): string {
  const searchParams = new URLSearchParams({
    page: page.toString(),
    page_size: pageSize.toString(),
  })

  return `/api/user/self/invitations?${searchParams.toString()}`
}

export async function getInvitations(
  page: number,
  pageSize: number
): Promise<InvitationPageResponse> {
  const res = await api.get<InvitationPageResponse>(
    buildInvitationListPath(page, pageSize)
  )
  return res.data
}

export async function getAffiliateCode(): Promise<AffiliateCodeResponse> {
  const res = await api.get<AffiliateCodeResponse>('/api/user/aff')
  return res.data
}

export async function transferAffiliateQuota(
  quota: number
): Promise<AffiliateTransferResponse> {
  const res = await api.post<AffiliateTransferResponse>(
    '/api/user/aff_transfer',
    { quota },
    { skipBusinessError: true }
  )
  return res.data
}
