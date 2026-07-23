/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useRef, useState } from 'react'
import axios from 'axios'
import { Alert02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { handleServerError } from '@/lib/handle-server-error'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Field, FieldDescription, FieldLabel } from '@/components/ui/field'
import { Skeleton } from '@/components/ui/skeleton'
import { Spinner } from '@/components/ui/spinner'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { useIdempotentIntent } from '../hooks/use-idempotent-intent'
import { useSupplyChainSecurity } from '../hooks/use-supply-chain-admin'
import {
  useSupplyChainDailyReportRerun,
  useSupplyChainDailyReports,
} from '../hooks/use-supply-chain-report'
import {
  canRerunDailyReport,
  createDailyReportRerunIntent,
  isDailyRerunVerificationUnavailable,
  shouldReleaseDailyReportRerunIntentOnDismiss,
  shouldRetainDailyReportRerunIntent,
} from '../lib/daily-report'
import type {
  SupplierAccountingCoverageGap,
  SupplierDailyReportDay,
  SupplierDailyReportProjection,
  SupplierDailyReportRerunVariables,
  SupplierReportQuery,
} from '../types'
import {
  PendingConfirmationAlert,
  SupplyChainVerificationDialog,
} from './management-common'

interface DailyReportSectionProps {
  query: SupplierReportQuery
  enabled?: boolean
}

interface DailyReportTableProps {
  data?: SupplierDailyReportProjection
  isLoading?: boolean
  isError?: boolean
  onRerun?: (day: SupplierDailyReportDay) => void
}

type DailyRerunReconciliationOutcome = 'success' | 'conflict' | 'failed'

function formatShanghaiTime(timestamp: number | null): string {
  if (!timestamp) return '—'
  return new Intl.DateTimeFormat(undefined, {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(new Date(timestamp * 1000))
}

function warningLabel(t: TFunction, code: string): string {
  switch (code) {
    case 'producer_error':
      return t('Producer reported an accounting evidence error')
    case 'unknown_producer_capability':
      return t('Producer capability was unknown')
    case 'absent_marker_after_cutover':
      return t('Required producer marker was absent after cutover')
    case 'incompatible_producer':
      return t('Producer capability was incompatible')
    case 'invalid_captured_snapshot':
      return t('Captured accounting snapshot was invalid')
    case 'unknown_official_amount':
      return t('Official amount could not be determined')
    default:
      return t('Unknown evidence warning')
  }
}

function gapReasonLabel(t: TFunction, reason: string): string {
  switch (reason) {
    case 'log_write_failure':
      return t('Log write failure')
    case 'logging_disabled':
      return t('Logging disabled')
    case 'producer_capability_mismatch':
      return t('Producer capability mismatch')
    case 'emergency_rollback':
      return t('Emergency rollback')
    case 'operator_declared':
      return t('Operator-declared gap')
    default:
      return t('Known coverage gap')
  }
}

function financeDispositionLabel(t: TFunction, disposition: string): string {
  switch (disposition) {
    case 'pending':
      return t('Finance review pending')
    case 'no_impact':
      return t('No finance impact')
    case 'reconciled':
      return t('Finance reconciled')
    case 'accepted_loss':
      return t('Accepted finance loss')
    default:
      return t('Finance disposition unavailable')
  }
}

function completenessPresentation(
  t: TFunction,
  completeness: SupplierDailyReportDay['persisted_log_snapshot_completeness']
): { label: string; variant: 'destructive' | 'secondary' } {
  switch (completeness) {
    case 'complete':
      return { label: t('Complete'), variant: 'secondary' }
    case 'incomplete':
      return { label: t('Incomplete'), variant: 'destructive' }
    case 'not_scanned':
      return { label: t('Not scanned'), variant: 'destructive' }
  }
}

function failureCount(day: SupplierDailyReportDay): number {
  return Object.values(day.failure_counts).reduce(
    (total, count) => total + count,
    0
  )
}

function CoverageGapList(props: { gaps: SupplierAccountingCoverageGap[] }) {
  const { t } = useTranslation()
  if (props.gaps.length === 0) {
    return <span className='text-muted-foreground'>{t('No known gaps')}</span>
  }
  return (
    <div className='flex min-w-72 flex-col gap-2 whitespace-normal'>
      {props.gaps.map((gap) => (
        <div key={gap.id} className='flex flex-col gap-0.5'>
          <div className='flex flex-wrap items-center gap-1'>
            <Badge variant='destructive'>
              {gapReasonLabel(t, gap.reason_category)}
            </Badge>
            <Badge variant='outline'>
              {financeDispositionLabel(t, gap.finance_disposition)}
            </Badge>
          </div>
          <span>{gap.reason_text}</span>
          <span className='text-muted-foreground text-xs'>
            {t('Gap window: {{start}} to {{end}}', {
              start: formatShanghaiTime(gap.start_at),
              end: formatShanghaiTime(gap.end_at),
            })}
          </span>
        </div>
      ))}
    </div>
  )
}

function WarningList(props: { day: SupplierDailyReportDay }) {
  const { t } = useTranslation()
  if (props.day.warnings.length === 0) {
    return <span className='text-muted-foreground'>{t('No warnings')}</span>
  }
  return (
    <div className='flex min-w-64 flex-col gap-1 whitespace-normal'>
      {props.day.warnings.map((warning) => (
        <span key={`${warning.code}:${warning.count}`}>
          {warningLabel(t, warning.code)}{' '}
          <span className='text-muted-foreground tabular-nums'>
            ×{warning.count}
          </span>
        </span>
      ))}
    </div>
  )
}

export function DailyReportTable(props: DailyReportTableProps) {
  const { t } = useTranslation()
  if (props.isLoading && !props.data) {
    return (
      <Card size='sm'>
        <CardHeader>
          <Skeleton className='h-5 w-48' />
          <Skeleton className='h-4 w-80 max-w-full' />
        </CardHeader>
        <CardContent
          className='flex flex-col gap-2'
          aria-label={t('Loading daily reports')}
        >
          <Skeleton className='h-10 w-full' />
          <Skeleton className='h-16 w-full' />
          <Skeleton className='h-16 w-full' />
        </CardContent>
      </Card>
    )
  }

  if (props.isError && !props.data) {
    return (
      <Alert variant='destructive'>
        <HugeiconsIcon icon={Alert02Icon} strokeWidth={2} aria-hidden='true' />
        <AlertTitle>{t('Unable to load published daily reports')}</AlertTitle>
        <AlertDescription>
          {t(
            'No daily publication evidence is shown. Retry before making finance decisions.'
          )}
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <Card size='sm'>
      <CardHeader>
        <CardTitle>{t('Published daily evidence')}</CardTitle>
        <CardDescription>
          {t(
            'Immutable publication evidence and fresh coverage-gap records for each eligible closed Shanghai day.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='flex flex-col gap-3'>
        {props.isError && props.data ? (
          <Alert variant='destructive'>
            <HugeiconsIcon
              icon={Alert02Icon}
              strokeWidth={2}
              aria-hidden='true'
            />
            <AlertTitle>{t('Daily report refresh failed')}</AlertTitle>
            <AlertDescription>
              {t(
                'The previous published view is preserved. Retry before treating coverage gaps as current.'
              )}
            </AlertDescription>
          </Alert>
        ) : null}
        <Alert>
          <AlertTitle>{t('V1 persisted-log universe')}</AlertTitle>
          <AlertDescription>
            {t(
              'V1 includes only successfully persisted consume logs from final successful settlements. Absent or unpersisted logs are outside historical completeness.'
            )}
          </AlertDescription>
        </Alert>

        {props.data?.days.length ? (
          <Table aria-label={t('Published daily supply-chain evidence')}>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Batch date')}</TableHead>
                <TableHead>{t('Publication')}</TableHead>
                <TableHead>{t('Persisted-log completeness')}</TableHead>
                <TableHead>{t('Evidence counters')}</TableHead>
                <TableHead>{t('Published warnings')}</TableHead>
                <TableHead>{t('Fresh coverage gaps')}</TableHead>
                <TableHead>{t('Finance state')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {props.data.days.map((day) => (
                <TableRow key={day.batch_date}>
                  <TableCell className='font-medium'>
                    {day.batch_date}
                  </TableCell>
                  <TableCell>
                    <div className='flex min-w-52 flex-col gap-1 whitespace-normal'>
                      <Badge variant={day.published ? 'outline' : 'secondary'}>
                        {day.published ? t('Published') : t('Not published')}
                      </Badge>
                      {day.published ? (
                        <>
                          <span>{formatShanghaiTime(day.published_at)}</span>
                          <span className='text-muted-foreground text-xs'>
                            {t('Published fence: {{token}}', {
                              token: day.published_fence_token,
                            })}
                          </span>
                        </>
                      ) : null}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={
                        completenessPresentation(
                          t,
                          day.persisted_log_snapshot_completeness
                        ).variant
                      }
                    >
                      {
                        completenessPresentation(
                          t,
                          day.persisted_log_snapshot_completeness
                        ).label
                      }
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <dl className='grid min-w-52 grid-cols-[1fr_auto] gap-x-3 gap-y-1 whitespace-normal'>
                      <dt>{t('Logs scanned')}</dt>
                      <dd>{day.logs_scanned}</dd>
                      <dt>{t('Producer markers')}</dt>
                      <dd>{day.producer_markers_present}</dd>
                      <dt>{t('Captured snapshots')}</dt>
                      <dd>{day.captured_snapshot_count}</dd>
                      <dt>{t('Evidence failures')}</dt>
                      <dd>{failureCount(day)}</dd>
                    </dl>
                  </TableCell>
                  <TableCell>
                    <WarningList day={day} />
                  </TableCell>
                  <TableCell>
                    <CoverageGapList gaps={day.known_coverage_gaps} />
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={
                        day.finance_attention_required
                          ? 'destructive'
                          : 'secondary'
                      }
                    >
                      {day.finance_attention_required
                        ? t('Finance attention required')
                        : t('No finance attention required')}
                    </Badge>
                  </TableCell>
                  <TableCell className='text-right'>
                    {canRerunDailyReport(day) && props.onRerun ? (
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => props.onRerun?.(day)}
                      >
                        {t('Rerun incomplete day')}
                      </Button>
                    ) : null}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        ) : (
          <Empty>
            <EmptyHeader>
              <EmptyMedia variant='icon'>
                <HugeiconsIcon
                  icon={Alert02Icon}
                  strokeWidth={2}
                  aria-hidden='true'
                />
              </EmptyMedia>
              <EmptyTitle>{t('No eligible daily reports')}</EmptyTitle>
              <EmptyDescription>
                {t(
                  'This range has no eligible closed Shanghai days to display.'
                )}
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        )}
      </CardContent>
    </Card>
  )
}

export function DailyReportSection(props: DailyReportSectionProps) {
  const { t } = useTranslation()
  const report = useSupplyChainDailyReports(props.query, props.enabled)
  const rerun = useSupplyChainDailyReportRerun()
  const [selectedDay, setSelectedDay] = useState<SupplierDailyReportDay | null>(
    null
  )
  const [reason, setReason] = useState('')
  const [rerunOpen, setRerunOpen] = useState(false)
  const [verificationUnavailable, setVerificationUnavailable] = useState(false)
  const intentRef = useRef<SupplierDailyReportRerunVariables | null>(null)
  const lastRerunErrorRef = useRef<unknown>(null)
  const reconciliationOutcomeRef =
    useRef<DailyRerunReconciliationOutcome | null>(null)
  const intent = useIdempotentIntent()
  const security = useSupplyChainSecurity()
  const verification = security.verification

  function openRerun(day: SupplierDailyReportDay): void {
    if (intentRef.current) {
      setRerunOpen(true)
      return
    }
    if (!canRerunDailyReport(day) || verification.open) return
    setSelectedDay(day)
    setReason('')
    setVerificationUnavailable(false)
    setRerunOpen(true)
  }

  function resetRerunEditor(): void {
    intentRef.current = null
    lastRerunErrorRef.current = null
    reconciliationOutcomeRef.current = null
    setSelectedDay(null)
    setReason('')
    setVerificationUnavailable(false)
  }

  function closeRerun(): void {
    if (rerun.isPending || intent.isSubmitting) return
    setRerunOpen(false)
    if (shouldReleaseDailyReportRerunIntentOnDismiss(intentRef.current)) {
      resetRerunEditor()
    }
  }

  function handleRerunFailure(error: unknown): void {
    if (
      error instanceof Error &&
      error.name === 'SupplyChainVerificationCancelledError'
    ) {
      intent.clearIntent()
      setRerunOpen(false)
      resetRerunEditor()
      return
    }
    if (isDailyRerunVerificationUnavailable(error)) {
      intent.clearIntent()
      intentRef.current = null
      setVerificationUnavailable(true)
      setRerunOpen(true)
      return
    }
    if (!shouldRetainDailyReportRerunIntent(error)) {
      intent.clearIntent()
      verification.reset()
      setRerunOpen(false)
      resetRerunEditor()
      void report.refetch()
      if (axios.isAxiosError(error) && error.response?.status === 409) {
        toast.error(
          t(
            'Published evidence changed before the rerun started. The daily view is being refreshed.'
          )
        )
        return
      }
    } else {
      setRerunOpen(true)
    }
    handleServerError(error)
  }

  function settleRerun(outcome: DailyRerunReconciliationOutcome): void {
    const terminalError = lastRerunErrorRef.current
    intent.clearIntent()
    verification.reset()
    setRerunOpen(false)
    resetRerunEditor()
    if (outcome === 'success') {
      toast.success(t('Daily report rerun started'))
      return
    }
    void report.refetch()
    if (outcome === 'conflict') {
      toast.error(
        t(
          'Published evidence changed before the rerun started. The daily view is being refreshed.'
        )
      )
      return
    }
    if (terminalError) {
      handleServerError(terminalError)
    }
  }

  async function reconcileExactRerun(key: string): Promise<boolean> {
    const variables = intentRef.current
    if (!variables || variables.idempotencyKey !== key) {
      throw new Error('Daily report rerun intent anchor is unavailable')
    }
    try {
      await security.execute(() => rerun.mutateAsync(variables))
      reconciliationOutcomeRef.current = 'success'
      return true
    } catch (error) {
      lastRerunErrorRef.current = error
      if (
        (error instanceof Error &&
          error.name === 'SupplyChainVerificationCancelledError') ||
        isDailyRerunVerificationUnavailable(error)
      ) {
        throw error
      }
      if (axios.isAxiosError(error) && error.response?.status === 409) {
        reconciliationOutcomeRef.current = 'conflict'
        return true
      }
      if (
        axios.isAxiosError(error) &&
        !shouldRetainDailyReportRerunIntent(error)
      ) {
        reconciliationOutcomeRef.current = 'failed'
        return true
      }
      throw error
    }
  }

  async function reconcilePendingRerun(): Promise<void> {
    reconciliationOutcomeRef.current = null
    lastRerunErrorRef.current = null
    await intent.reconcilePending()
    const outcome = reconciliationOutcomeRef.current
    if (outcome) {
      settleRerun(outcome)
      return
    }
    setRerunOpen(true)
    if (lastRerunErrorRef.current) {
      handleServerError(lastRerunErrorRef.current)
    }
  }

  async function submitRerun(): Promise<void> {
    if (intent.isPendingConfirmation) {
      await reconcilePendingRerun()
      return
    }
    const trimmedReason = reason.trim()
    if (!selectedDay || !trimmedReason || !canRerunDailyReport(selectedDay))
      return
    lastRerunErrorRef.current = null
    reconciliationOutcomeRef.current = null
    setVerificationUnavailable(false)
    setRerunOpen(false)
    const result = await intent.run({
      execute: async (key) => {
        const variables = createDailyReportRerunIntent(
          selectedDay,
          trimmedReason,
          intentRef.current,
          () => key
        )
        if (!variables || variables.idempotencyKey !== key) {
          throw new Error('Daily report rerun intent anchor is unavailable')
        }
        intentRef.current = variables
        try {
          return await security.execute(() => rerun.mutateAsync(variables))
        } catch (error) {
          lastRerunErrorRef.current = error
          throw error
        }
      },
      reconcile: reconcileExactRerun,
    })

    if (result === 'success') {
      settleRerun('success')
      return
    }
    if (result === 'reconciled') {
      settleRerun(reconciliationOutcomeRef.current ?? 'success')
      return
    }
    if (result === 'conflict') {
      settleRerun('conflict')
      return
    }
    if (result === 'pending_confirmation' || result === 'rate_limited') {
      setRerunOpen(true)
      if (lastRerunErrorRef.current) {
        handleServerError(lastRerunErrorRef.current)
      }
      return
    }
    if (lastRerunErrorRef.current) {
      handleRerunFailure(lastRerunErrorRef.current)
    }
  }

  return (
    <>
      <DailyReportTable
        data={report.data}
        isLoading={report.isLoading || report.isFetching}
        isError={report.isError}
        onRerun={openRerun}
      />

      <Dialog open={rerunOpen} onOpenChange={(open) => !open && closeRerun()}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Rerun incomplete published day')}</DialogTitle>
            <DialogDescription>
              {t(
                'The rerun uses an exact compare-and-swap fence. If another publication wins first, this request is rejected without replacing newer evidence.'
              )}
            </DialogDescription>
          </DialogHeader>
          {verificationUnavailable ? (
            <Alert variant='destructive'>
              <AlertTitle>
                {t('Security verification is not configured')}
              </AlertTitle>
              <AlertDescription>
                {t(
                  'Enable Two-factor Authentication or Passkey in your profile, then retry this rerun with a new secure intent.'
                )}
              </AlertDescription>
            </Alert>
          ) : null}
          <PendingConfirmationAlert
            visible={intent.isPendingConfirmation}
            onReconcile={() => void reconcilePendingRerun()}
          />
          <Field>
            <FieldLabel htmlFor='daily-report-rerun-reason'>
              {t('Reason')}
            </FieldLabel>
            <Textarea
              id='daily-report-rerun-reason'
              value={reason}
              maxLength={512}
              aria-required='true'
              onChange={(event) => setReason(event.target.value)}
              placeholder={t(
                'Explain why finance needs this published day rerun'
              )}
            />
            <FieldDescription>
              {selectedDay
                ? t('Expected published fence: {{token}}', {
                    token: selectedDay.published_fence_token,
                  })
                : null}
            </FieldDescription>
          </Field>
          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              disabled={rerun.isPending || intent.isSubmitting}
              onClick={closeRerun}
            >
              {t('Cancel')}
            </Button>
            <Button
              type='button'
              disabled={
                !reason.trim() ||
                rerun.isPending ||
                intent.isSubmitting ||
                intent.isPendingConfirmation
              }
              onClick={() => void submitRerun()}
            >
              {rerun.isPending || intent.isSubmitting ? (
                <Spinner data-icon='inline-start' />
              ) : null}
              {t('Verify and rerun')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <SupplyChainVerificationDialog security={security} />
    </>
  )
}
