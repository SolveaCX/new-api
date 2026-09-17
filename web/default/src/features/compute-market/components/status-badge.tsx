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
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import type { RFQStatus } from '../types'

export function RFQStatusBadge({ status }: { status: RFQStatus }) {
  const { t } = useTranslation()
  const label: Record<RFQStatus, string> = {
    matching: t('Matching'),
    choosing: t('Choose from top 3'),
    matched: t('Matched · awaiting contract'),
    contracted: t('Contracted'),
    live: t('Live'),
    completed: t('Completed'),
    cancelled: t('Cancelled'),
  }
  const variant =
    status === 'matching' || status === 'choosing'
      ? 'default'
      : status === 'cancelled'
        ? 'destructive'
        : status === 'matched'
          ? 'secondary'
          : 'outline'
  return (
    <Badge variant={variant} className='whitespace-nowrap'>
      {(status === 'matching' || status === 'choosing') && (
        <span className='mr-1 inline-block size-1.5 animate-pulse rounded-full bg-current' />
      )}
      {label[status]}
    </Badge>
  )
}
