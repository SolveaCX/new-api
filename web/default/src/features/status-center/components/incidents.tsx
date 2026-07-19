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
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { LockKeyhole } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import {
  getStatusIncidents,
  publishStatusIncident,
  statusCenterQueryKeys,
} from '../api'
import { formatStatusTimestamp } from '../format'
import {
  buildPublishedUpdateRows,
  resolveStatusMutationError,
  type StatusIncidentPublishInput,
  type StatusIncidentUpdate,
} from '../types'
import { EmptyState, ErrorState, LoadingState } from './common'

type IncidentState = StatusIncidentPublishInput['state']

export function PublishedIncidentHistory(props: {
  updates: StatusIncidentUpdate[]
  correctionTargetId: string
}) {
  const { t } = useTranslation()
  const publishedRows = buildPublishedUpdateRows(props.updates)

  if (publishedRows.length === 0) {
    return <EmptyState descriptionKey='statusCenter.empty.publishedUpdates' />
  }

  return (
    <ol className='space-y-3'>
      {publishedRows.map((update) => (
        <li key={update.id} className='rounded-lg border p-3'>
          <div className='flex flex-wrap items-center justify-between gap-2'>
            <Badge variant='outline'>
              {t(`statusCenter.incidents.state.${update.state}`)}
            </Badge>
            <time className='text-muted-foreground text-xs'>
              {formatStatusTimestamp(update.publishedAt)}
            </time>
          </div>
          <p className='mt-2 whitespace-pre-wrap'>{update.body}</p>
          <div className='mt-2 flex flex-wrap items-center justify-between gap-2 text-xs'>
            <p className='text-muted-foreground'>
              {t('statusCenter.incidents.correctionsAppendOnly')}
            </p>
            <a
              className='text-primary font-medium underline-offset-4 hover:underline'
              href={`#${props.correctionTargetId}`}
            >
              {t('statusCenter.incidents.appendCorrection')}
            </a>
          </div>
        </li>
      ))}
    </ol>
  )
}

export function IncidentsPanel(props: { active: boolean }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [incidentId, setIncidentId] = useState(0)
  const [state, setState] = useState<IncidentState>('investigating')
  const [body, setBody] = useState('')
  const [reason, setReason] = useState('')

  const incidentsQuery = useQuery({
    queryKey: statusCenterQueryKeys.incidents(),
    queryFn: getStatusIncidents,
    enabled: props.active,
  })
  const records = incidentsQuery.data ?? []
  const selected =
    records.find((record) => record.incident.id === incidentId) ?? records[0]

  const publishMutation = useMutation({
    mutationFn: (input: StatusIncidentPublishInput) => {
      if (!selected) throw new Error('incident unavailable')
      return publishStatusIncident(selected.incident.id, input)
    },
    onSuccess: () => {
      setBody('')
      setReason('')
      toast.success(t('statusCenter.incidents.updatePublished'))
      void queryClient.invalidateQueries({
        queryKey: statusCenterQueryKeys.incidents(),
      })
    },
    onError: async (error) => {
      const resolution = await resolveStatusMutationError(error, async () => {
        await queryClient.invalidateQueries({
          queryKey: statusCenterQueryKeys.incidents(),
        })
      })
      toast.error(t(resolution.messageKey))
    },
  })

  const publish = () => {
    if (!selected || !body.trim() || !reason.trim()) return
    publishMutation.mutate({
      expected_version: selected.incident.version,
      state,
      body: body.trim(),
      event_id: `console_${globalThis.crypto.randomUUID()}`,
      reason: reason.trim(),
      destinations: [],
    })
  }

  if (incidentsQuery.isLoading) return <LoadingState />
  if (incidentsQuery.isError) {
    return <ErrorState onRetry={() => void incidentsQuery.refetch()} />
  }
  if (records.length === 0) {
    return <EmptyState descriptionKey='statusCenter.empty.incidents' />
  }

  return (
    <div className='space-y-4'>
      <Card>
        <CardHeader>
          <CardTitle>{t('statusCenter.incidents.title')}</CardTitle>
          <CardDescription>
            {t('statusCenter.incidents.description')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('statusCenter.incidents.incident')}</TableHead>
                <TableHead>{t('statusCenter.status')}</TableHead>
                <TableHead>{t('statusCenter.incidents.visibility')}</TableHead>
                <TableHead>{t('statusCenter.updatedAt')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {records.map((record) => (
                <TableRow key={record.incident.id}>
                  <TableCell className='max-w-96 whitespace-normal'>
                    <div className='font-medium'>{record.incident.title}</div>
                    <div className='text-muted-foreground text-xs'>
                      {record.incident.public_id}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant='outline'>
                      {t(`statusCenter.recordStatus.${record.incident.status}`)}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    {t(
                      `statusCenter.recordStatus.${record.incident.visibility}`
                    )}
                  </TableCell>
                  <TableCell>
                    {formatStatusTimestamp(record.incident.updated_at)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('statusCenter.incidents.publishedHistory')}</CardTitle>
          <CardDescription className='flex items-center gap-2'>
            <LockKeyhole aria-hidden='true' className='size-4' />
            {t('statusCenter.incidents.immutableDescription')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='space-y-2'>
            <Label htmlFor='incident-record'>
              {t('statusCenter.incidents.incident')}
            </Label>
            <NativeSelect
              id='incident-record'
              className='w-full'
              value={selected?.incident.id ?? ''}
              onChange={(event) => setIncidentId(Number(event.target.value))}
            >
              {records.map((record) => (
                <NativeSelectOption
                  key={record.incident.id}
                  value={record.incident.id}
                >
                  {record.incident.title}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </div>
          <PublishedIncidentHistory
            updates={selected?.updates ?? []}
            correctionTargetId='incident-new-update'
          />
        </CardContent>
      </Card>

      <Card id='incident-new-update'>
        <CardHeader>
          <CardTitle>{t('statusCenter.incidents.addUpdate')}</CardTitle>
          <CardDescription>
            {t('statusCenter.incidents.addUpdateDescription')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='space-y-2'>
            <Label htmlFor='incident-state'>
              {t('statusCenter.incidents.state')}
            </Label>
            <NativeSelect
              id='incident-state'
              value={state}
              onChange={(event) =>
                setState(event.target.value as IncidentState)
              }
            >
              {(
                [
                  'investigating',
                  'identified',
                  'monitoring',
                  'resolved',
                ] as const
              ).map((value) => (
                <NativeSelectOption key={value} value={value}>
                  {t(`statusCenter.incidents.state.${value}`)}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </div>
          <div className='space-y-2'>
            <Label htmlFor='incident-body'>
              {t('statusCenter.incidents.publicUpdate')}
            </Label>
            <Textarea
              id='incident-body'
              value={body}
              onChange={(event) => setBody(event.target.value)}
              placeholder={t('statusCenter.incidents.publicUpdatePlaceholder')}
            />
          </div>
          <div className='space-y-2'>
            <Label htmlFor='incident-reason'>
              {t('statusCenter.incidents.auditReason')}
            </Label>
            <Textarea
              id='incident-reason'
              value={reason}
              onChange={(event) => setReason(event.target.value)}
              placeholder={t('statusCenter.incidents.auditReasonPlaceholder')}
            />
          </div>
          <Button
            type='button'
            disabled={
              !body.trim() || !reason.trim() || publishMutation.isPending
            }
            onClick={publish}
          >
            {t('statusCenter.incidents.publishNewUpdate')}
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}
