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
import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertTriangle, Clock3, Gauge, ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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
  createStatusOverride,
  getStatusSummary,
  statusCenterQueryKeys,
} from '../api'
import { formatCoverage, formatStatusTimestamp } from '../format'
import {
  getStatusLabelKey,
  resolveStatusMutationError,
  type StatusCenterRole,
  type StatusOverrideInput,
  type StatusValue,
  validateStatusOverride,
} from '../types'
import { EmptyState, ErrorState, LoadingState, StatusBadge } from './common'

type OverviewPanelProps = {
  active: boolean
  role: StatusCenterRole
  runSensitiveAction: (action: () => Promise<unknown>) => Promise<void>
}

function toDateTimeLocal(timestamp: number): string {
  const date = new Date(timestamp * 1000)
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
}

export function OverviewPanel(props: OverviewPanelProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [componentId, setComponentId] = useState(0)
  const [status, setStatus] = useState<StatusValue>('degraded')
  const [reason, setReason] = useState('')
  const [now] = useState(() => Math.floor(Date.now() / 1000))
  const [expiresAt, setExpiresAt] = useState(() =>
    toDateTimeLocal(now + 60 * 60)
  )

  const summaryQuery = useQuery({
    queryKey: statusCenterQueryKeys.summary(),
    queryFn: getStatusSummary,
    enabled: props.active,
    refetchInterval: 60_000,
  })
  const components = summaryQuery.data?.components ?? []
  const selectedComponent =
    components.find((component) => component.id === componentId) ??
    components[0]
  const expiryTimestamp = Math.floor(new Date(expiresAt).getTime() / 1000)
  const validationErrors = validateStatusOverride({
    status,
    reason,
    expiresAt: Number.isFinite(expiryTimestamp) ? expiryTimestamp : 0,
    now,
    role: props.role,
    secureVerified: status === 'operational' && props.role === 'root',
  })
  const expectedVersion = selectedComponent?.version

  const overrideMutation = useMutation({
    mutationFn: (input: StatusOverrideInput) => createStatusOverride(input),
    onSuccess: () => {
      toast.success(t('statusCenter.override.saved'))
      setReason('')
      void queryClient.invalidateQueries({
        queryKey: statusCenterQueryKeys.summary(),
      })
    },
    onError: async (error) => {
      const resolution = await resolveStatusMutationError(error, async () => {
        await queryClient.invalidateQueries({
          queryKey: statusCenterQueryKeys.summary(),
        })
      })
      toast.error(t(resolution.messageKey))
    },
  })

  const stale = useMemo(() => {
    const timestamp = summaryQuery.data?.last_trustworthy_update_at ?? 0
    const generatedAt = summaryQuery.data?.generated_at ?? 0
    return !timestamp || !generatedAt || generatedAt - timestamp >= 20 * 60
  }, [
    summaryQuery.data?.generated_at,
    summaryQuery.data?.last_trustworthy_update_at,
  ])

  const submitOverride = async () => {
    if (!selectedComponent || !expectedVersion || validationErrors.length > 0) {
      return
    }
    const input: StatusOverrideInput = {
      component_id: selectedComponent.id,
      expected_version: expectedVersion,
      status,
      reason: reason.trim(),
      expires_at: expiryTimestamp,
    }
    if (status === 'operational') {
      await props.runSensitiveAction(() => overrideMutation.mutateAsync(input))
      return
    }
    overrideMutation.mutate(input)
  }

  if (summaryQuery.isLoading) return <LoadingState />
  if (summaryQuery.isError) {
    return <ErrorState onRetry={() => void summaryQuery.refetch()} />
  }
  if (!summaryQuery.data || components.length === 0) {
    return <EmptyState descriptionKey='statusCenter.empty.components' />
  }

  const overallStatus =
    summaryQuery.data.status === 'monitoring_incomplete'
      ? 'unknown'
      : summaryQuery.data.status

  return (
    <div className='space-y-4'>
      {stale ? (
        <Alert>
          <AlertTriangle aria-hidden='true' />
          <AlertTitle>{t('statusCenter.stale.title')}</AlertTitle>
          <AlertDescription>
            {t('statusCenter.stale.description')}
          </AlertDescription>
        </Alert>
      ) : null}

      <div className='grid gap-3 md:grid-cols-3'>
        <Card size='sm'>
          <CardHeader>
            <CardDescription>{t('statusCenter.overallStatus')}</CardDescription>
            <CardTitle>
              <StatusBadge status={overallStatus} />
            </CardTitle>
          </CardHeader>
        </Card>
        <Card size='sm'>
          <CardHeader>
            <CardDescription className='flex items-center gap-2'>
              <Gauge aria-hidden='true' className='size-4' />
              {t('statusCenter.coverage')}
            </CardDescription>
            <CardTitle>{formatCoverage(summaryQuery.data.coverage)}</CardTitle>
          </CardHeader>
        </Card>
        <Card size='sm'>
          <CardHeader>
            <CardDescription className='flex items-center gap-2'>
              <Clock3 aria-hidden='true' className='size-4' />
              {t('statusCenter.lastTrustworthyUpdate')}
            </CardDescription>
            <CardTitle className='text-sm'>
              {formatStatusTimestamp(
                summaryQuery.data.last_trustworthy_update_at
              )}
            </CardTitle>
          </CardHeader>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t('statusCenter.components')}</CardTitle>
          <CardDescription>
            {t('statusCenter.components.description')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('statusCenter.component')}</TableHead>
                <TableHead>{t('statusCenter.status')}</TableHead>
                <TableHead>{t('statusCenter.coverage')}</TableHead>
                <TableHead>{t('statusCenter.lastTrustworthyUpdate')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {components.map((component) => (
                <TableRow key={component.id}>
                  <TableCell>
                    <div className='font-medium'>{component.display_name}</div>
                    <div className='text-muted-foreground text-xs'>
                      {component.kind}
                    </div>
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={component.status} />
                  </TableCell>
                  <TableCell>{formatCoverage(component.coverage)}</TableCell>
                  <TableCell>
                    {formatStatusTimestamp(
                      component.last_trustworthy_update_at
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('statusCenter.override.title')}</CardTitle>
          <CardDescription>
            {t('statusCenter.override.description')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          {!expectedVersion ? (
            <Alert>
              <AlertTriangle aria-hidden='true' />
              <AlertTitle>
                {t('statusCenter.override.versionUnavailable.title')}
              </AlertTitle>
              <AlertDescription>
                {t('statusCenter.override.versionUnavailable.description')}
              </AlertDescription>
            </Alert>
          ) : null}
          <div className='grid gap-4 md:grid-cols-3'>
            <div className='space-y-2'>
              <Label htmlFor='status-component'>
                {t('statusCenter.component')}
              </Label>
              <NativeSelect
                id='status-component'
                className='w-full'
                value={selectedComponent?.id ?? ''}
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
              <Label htmlFor='status-value'>
                {t('statusCenter.override.status')}
              </Label>
              <NativeSelect
                id='status-value'
                className='w-full'
                value={status}
                onChange={(event) =>
                  setStatus(event.target.value as StatusValue)
                }
              >
                {(
                  ['degraded', 'outage', 'unknown', 'maintenance'] as const
                ).map((value) => (
                  <NativeSelectOption key={value} value={value}>
                    {t(getStatusLabelKey(value))}
                  </NativeSelectOption>
                ))}
                {props.role === 'root' ? (
                  <NativeSelectOption value='operational'>
                    {t(getStatusLabelKey('operational'))}
                  </NativeSelectOption>
                ) : null}
              </NativeSelect>
            </div>
            <div className='space-y-2'>
              <Label htmlFor='status-expiry'>
                {t('statusCenter.override.expiry')}
              </Label>
              <Input
                id='status-expiry'
                type='datetime-local'
                value={expiresAt}
                onChange={(event) => setExpiresAt(event.target.value)}
              />
            </div>
          </div>
          <div className='space-y-2'>
            <Label htmlFor='status-reason'>
              {t('statusCenter.override.reason')}
            </Label>
            <Textarea
              id='status-reason'
              value={reason}
              onChange={(event) => setReason(event.target.value)}
              placeholder={t('statusCenter.override.reasonPlaceholder')}
            />
          </div>
          {validationErrors.length > 0 ? (
            <ul
              className='text-destructive list-disc space-y-1 pl-5 text-sm'
              role='alert'
            >
              {validationErrors.map((key) => (
                <li key={key}>{t(key)}</li>
              ))}
            </ul>
          ) : null}
          <Button
            type='button'
            disabled={
              !expectedVersion ||
              validationErrors.length > 0 ||
              overrideMutation.isPending
            }
            onClick={() => void submitOverride()}
          >
            {status === 'operational' ? (
              <ShieldCheck aria-hidden='true' />
            ) : null}
            {status === 'operational'
              ? t('statusCenter.override.verifyAndApply')
              : t('statusCenter.override.apply')}
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}
