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
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  getStatusAudit,
  getStatusDeliveries,
  getStatusSubscribers,
  statusCenterQueryKeys,
} from '../api'
import { formatStatusTimestamp } from '../format'
import { EmptyState, ErrorState, LoadingState } from './common'

export function SubscribersPanel(props: { active: boolean }) {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: statusCenterQueryKeys.subscribers(),
    queryFn: getStatusSubscribers,
    enabled: props.active,
  })
  if (query.isLoading) return <LoadingState />
  if (query.isError) return <ErrorState onRetry={() => void query.refetch()} />
  const subscribers = query.data ?? []
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('statusCenter.subscribers.title')}</CardTitle>
        <CardDescription>
          {t('statusCenter.subscribers.description')}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {subscribers.length === 0 ? (
          <EmptyState descriptionKey='statusCenter.empty.subscribers' />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>
                  {t('statusCenter.subscribers.destination')}
                </TableHead>
                <TableHead>{t('statusCenter.subscribers.kind')}</TableHead>
                <TableHead>{t('statusCenter.status')}</TableHead>
                <TableHead>{t('statusCenter.subscribers.failures')}</TableHead>
                <TableHead>{t('statusCenter.updatedAt')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {subscribers.map((subscriber) => (
                <TableRow key={subscriber.id}>
                  <TableCell>
                    {subscriber.display_address || t('statusCenter.configured')}
                  </TableCell>
                  <TableCell>
                    {t(`statusCenter.destination.${subscriber.kind}`)}
                  </TableCell>
                  <TableCell>
                    {t(`statusCenter.recordStatus.${subscriber.status}`)}
                  </TableCell>
                  <TableCell>{subscriber.failure_count}</TableCell>
                  <TableCell>
                    {formatStatusTimestamp(subscriber.updated_at)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  )
}

export function DeliveriesPanel(props: { active: boolean }) {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: statusCenterQueryKeys.deliveries(),
    queryFn: getStatusDeliveries,
    enabled: props.active,
  })
  if (query.isLoading) return <LoadingState />
  if (query.isError) return <ErrorState onRetry={() => void query.refetch()} />
  const deliveries = query.data ?? []
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('statusCenter.deliveries.title')}</CardTitle>
        <CardDescription>
          {t('statusCenter.deliveries.description')}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {deliveries.length === 0 ? (
          <EmptyState descriptionKey='statusCenter.empty.deliveries' />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('statusCenter.deliveries.event')}</TableHead>
                <TableHead>
                  {t('statusCenter.deliveries.destination')}
                </TableHead>
                <TableHead>{t('statusCenter.status')}</TableHead>
                <TableHead>{t('statusCenter.deliveries.attempts')}</TableHead>
                <TableHead>{t('statusCenter.updatedAt')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {deliveries.map((delivery) => (
                <TableRow key={delivery.id}>
                  <TableCell className='max-w-80 truncate'>
                    {delivery.event_id}
                  </TableCell>
                  <TableCell>
                    {t(`statusCenter.destination.${delivery.destination_type}`)}{' '}
                    #{delivery.destination_id}
                  </TableCell>
                  <TableCell>
                    {t(`statusCenter.recordStatus.${delivery.status}`)}
                  </TableCell>
                  <TableCell>{delivery.attempts}</TableCell>
                  <TableCell>
                    {formatStatusTimestamp(delivery.updated_at)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  )
}

export function AuditPanel(props: { active: boolean }) {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: statusCenterQueryKeys.audit(),
    queryFn: getStatusAudit,
    enabled: props.active,
  })
  if (query.isLoading) return <LoadingState />
  if (query.isError) return <ErrorState onRetry={() => void query.refetch()} />
  const events = query.data ?? []
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('statusCenter.audit.title')}</CardTitle>
        <CardDescription>{t('statusCenter.audit.description')}</CardDescription>
      </CardHeader>
      <CardContent>
        {events.length === 0 ? (
          <EmptyState descriptionKey='statusCenter.empty.audit' />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('statusCenter.audit.action')}</TableHead>
                <TableHead>{t('statusCenter.audit.object')}</TableHead>
                <TableHead>{t('statusCenter.audit.actor')}</TableHead>
                <TableHead>{t('statusCenter.override.reason')}</TableHead>
                <TableHead>{t('statusCenter.audit.time')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {events.map((event) => (
                <TableRow key={event.id}>
                  <TableCell>{event.action}</TableCell>
                  <TableCell>
                    {event.object_type} #{event.object_id}
                  </TableCell>
                  <TableCell>
                    {t(`statusCenter.actor.${event.actor_type}`)} #
                    {event.actor_id}
                  </TableCell>
                  <TableCell className='max-w-96 whitespace-normal'>
                    {event.reason || '—'}
                  </TableCell>
                  <TableCell>
                    {formatStatusTimestamp(event.created_at)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  )
}
