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
import { Input } from '@/components/ui/input'
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
  createStatusMaintenance,
  getStatusMaintenance,
  getStatusSummary,
  reconcileStatusMaintenance,
  statusCenterQueryKeys,
} from '../api'
import { formatStatusTimestamp } from '../format'
import {
  resolveStatusMutationError,
  type StatusMaintenanceInput,
} from '../types'
import { EmptyState, ErrorState, LoadingState } from './common'

function localDateTime(minutesFromNow: number): string {
  const date = new Date(Date.now() + minutesFromNow * 60_000)
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
}

export function MaintenancePanel(props: { active: boolean }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [title, setTitle] = useState('')
  const [body, setBody] = useState('')
  const [reason, setReason] = useState('')
  const [componentId, setComponentId] = useState(0)
  const [now] = useState(() => Math.floor(Date.now() / 1000))
  const [startAt, setStartAt] = useState(() => localDateTime(60))
  const [endAt, setEndAt] = useState(() => localDateTime(120))

  const maintenanceQuery = useQuery({
    queryKey: statusCenterQueryKeys.maintenance(),
    queryFn: getStatusMaintenance,
    enabled: props.active,
  })
  const summaryQuery = useQuery({
    queryKey: statusCenterQueryKeys.summary(),
    queryFn: getStatusSummary,
    enabled: props.active,
  })
  const components = summaryQuery.data?.components ?? []
  const selectedComponentId = componentId || components[0]?.id || 0

  const reloadMaintenance = async () => {
    await queryClient.invalidateQueries({
      queryKey: statusCenterQueryKeys.maintenance(),
    })
  }
  const onMutationError = async (error: unknown) => {
    const resolution = await resolveStatusMutationError(
      error,
      reloadMaintenance
    )
    toast.error(t(resolution.messageKey))
  }

  const createMutation = useMutation({
    mutationFn: (input: StatusMaintenanceInput) =>
      createStatusMaintenance(input),
    onSuccess: () => {
      setTitle('')
      setBody('')
      setReason('')
      toast.success(t('statusCenter.maintenance.created'))
      void reloadMaintenance()
    },
    onError: onMutationError,
  })
  const reconcileMutation = useMutation({
    mutationFn: (input: { id: number; version: number }) =>
      reconcileStatusMaintenance(input.id, input.version),
    onSuccess: () => {
      toast.success(t('statusCenter.maintenance.reconciled'))
      void reloadMaintenance()
    },
    onError: onMutationError,
  })

  const startTimestamp = Math.floor(new Date(startAt).getTime() / 1000)
  const endTimestamp = Math.floor(new Date(endAt).getTime() / 1000)
  const validSchedule =
    selectedComponentId > 0 &&
    startTimestamp > now &&
    endTimestamp > startTimestamp

  const createMaintenance = () => {
    if (!title.trim() || !body.trim() || !reason.trim() || !validSchedule) {
      return
    }
    createMutation.mutate({
      title: title.trim(),
      body: body.trim(),
      impact: 'maintenance',
      idempotency_key: `console_${globalThis.crypto.randomUUID()}`,
      component_ids: [selectedComponentId],
      scheduled_start_at: startTimestamp,
      scheduled_end_at: endTimestamp,
      reason: reason.trim(),
    })
  }

  if (maintenanceQuery.isLoading || summaryQuery.isLoading) {
    return <LoadingState />
  }
  if (maintenanceQuery.isError || summaryQuery.isError) {
    return (
      <ErrorState
        onRetry={() => {
          void maintenanceQuery.refetch()
          void summaryQuery.refetch()
        }}
      />
    )
  }

  const records = maintenanceQuery.data ?? []
  return (
    <div className='space-y-4'>
      <Card>
        <CardHeader>
          <CardTitle>{t('statusCenter.maintenance.title')}</CardTitle>
          <CardDescription>
            {t('statusCenter.maintenance.description')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {records.length === 0 ? (
            <EmptyState descriptionKey='statusCenter.empty.maintenance' />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('statusCenter.maintenance.window')}</TableHead>
                  <TableHead>{t('statusCenter.status')}</TableHead>
                  <TableHead>
                    {t('statusCenter.maintenance.components')}
                  </TableHead>
                  <TableHead>{t('statusCenter.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {records.map((record) => (
                  <TableRow key={record.incident.id}>
                    <TableCell className='min-w-64 whitespace-normal'>
                      <div className='font-medium'>{record.incident.title}</div>
                      <div className='text-muted-foreground text-xs'>
                        {formatStatusTimestamp(
                          record.incident.scheduled_start_at ?? 0
                        )}
                        {' — '}
                        {formatStatusTimestamp(
                          record.incident.scheduled_end_at ?? 0
                        )}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant='outline'>
                        {t(
                          `statusCenter.recordStatus.${record.incident.status}`
                        )}
                      </Badge>
                    </TableCell>
                    <TableCell>{record.component_ids.join(', ')}</TableCell>
                    <TableCell>
                      <Button
                        type='button'
                        size='sm'
                        variant='outline'
                        disabled={reconcileMutation.isPending}
                        onClick={() =>
                          reconcileMutation.mutate({
                            id: record.incident.id,
                            version: record.incident.version,
                          })
                        }
                      >
                        {t('statusCenter.maintenance.reconcile')}
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('statusCenter.maintenance.schedule')}</CardTitle>
          <CardDescription>
            {t('statusCenter.maintenance.scheduleDescription')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='grid gap-4 md:grid-cols-2'>
            <div className='space-y-2'>
              <Label htmlFor='maintenance-title'>
                {t('statusCenter.maintenance.name')}
              </Label>
              <Input
                id='maintenance-title'
                value={title}
                onChange={(event) => setTitle(event.target.value)}
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='maintenance-component'>
                {t('statusCenter.component')}
              </Label>
              <NativeSelect
                id='maintenance-component'
                className='w-full'
                value={selectedComponentId}
                onChange={(event) => setComponentId(Number(event.target.value))}
              >
                {components.map((component) => (
                  <NativeSelectOption key={component.id} value={component.id}>
                    {component.display_name}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </div>
            <div className='space-y-2'>
              <Label htmlFor='maintenance-start'>
                {t('statusCenter.maintenance.start')}
              </Label>
              <Input
                id='maintenance-start'
                type='datetime-local'
                value={startAt}
                onChange={(event) => setStartAt(event.target.value)}
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='maintenance-end'>
                {t('statusCenter.maintenance.end')}
              </Label>
              <Input
                id='maintenance-end'
                type='datetime-local'
                value={endAt}
                onChange={(event) => setEndAt(event.target.value)}
              />
            </div>
          </div>
          <div className='space-y-2'>
            <Label htmlFor='maintenance-body'>
              {t('statusCenter.maintenance.publicDescription')}
            </Label>
            <Textarea
              id='maintenance-body'
              value={body}
              onChange={(event) => setBody(event.target.value)}
            />
          </div>
          <div className='space-y-2'>
            <Label htmlFor='maintenance-reason'>
              {t('statusCenter.maintenance.auditReason')}
            </Label>
            <Textarea
              id='maintenance-reason'
              value={reason}
              onChange={(event) => setReason(event.target.value)}
            />
          </div>
          {!validSchedule ? (
            <p className='text-destructive text-sm' role='alert'>
              {t('statusCenter.maintenance.invalidSchedule')}
            </p>
          ) : null}
          <Button
            type='button'
            disabled={
              !title.trim() ||
              !body.trim() ||
              !reason.trim() ||
              !validSchedule ||
              createMutation.isPending
            }
            onClick={createMaintenance}
          >
            {t('statusCenter.maintenance.createDraft')}
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}
