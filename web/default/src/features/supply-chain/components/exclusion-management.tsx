/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { PlusSignIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
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
import { createExclusionRule, isSupplyChainCommandCommitted } from '../api'
import type { SupplyChainManagementProps } from '../contracts'
import { useIdempotentIntent } from '../hooks/use-idempotent-intent'
import {
  useEffectiveExclusionList,
  useExclusionHistoryList,
  useSupplyChainAdminMutation,
  useSupplyChainSecurity,
  type SupplyChainSecurity,
} from '../hooks/use-supply-chain-admin'
import { STATISTICS_ACTION_LABEL_KEYS } from '../lib/labels'
import { exclusionFormSchema, type ExclusionFormValues } from '../lib/schemas'
import { formatTime } from '../lib/time'
import { supplyChainQueryKeys } from '../query-keys'
import type {
  SupplierEffectiveExclusion,
  SupplierStatisticsAction,
} from '../types'
import {
  ConfirmAction,
  ManagementPagination,
  ManagementStatus,
  ManagementToolbar,
  PendingConfirmationAlert,
  SupplyChainVerificationDialog,
} from './management-common'
import { ProgressiveList } from './progressive-list'

function ExclusionRuleDialog(props: {
  row?: SupplierEffectiveExclusion
  defaultAction?: SupplierStatisticsAction
  security: SupplyChainSecurity
  onSaved: () => void
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [confirmation, setConfirmation] = useState<ExclusionFormValues | null>(
    null
  )
  const intent = useIdempotentIntent()
  const form = useForm<ExclusionFormValues>({
    resolver: zodResolver(exclusionFormSchema),
    defaultValues: {
      user_id: props.row?.user_id ?? 0,
      action: props.defaultAction ?? 'exclude',
      reason: '',
    },
  })
  const mutation = useSupplyChainAdminMutation<{
    values: ExclusionFormValues
    key: string
  }>({
    mutationFn: ({ values, key }) =>
      createExclusionRule({ data: values, idempotencyKey: key }),
    invalidate: [supplyChainQueryKeys.exclusions.all()],
    security: props.security,
  })

  useEffect(() => {
    if (open) {
      form.reset({
        user_id: props.row?.user_id ?? 0,
        action:
          props.defaultAction ?? (props.row?.excluded ? 'include' : 'exclude'),
        reason: '',
      })
    }
  }, [form, open, props.defaultAction, props.row])

  function finishAppend(): void {
    toast.success(t('Exclusion rule appended'))
    setConfirmation(null)
    setOpen(false)
    props.onSaved()
  }

  async function reconcilePending(): Promise<void> {
    if ((await intent.reconcilePending()) === 'reconciled') finishAppend()
  }

  async function confirm(): Promise<void> {
    if (!confirmation) return
    const result = await intent.run({
      execute: (key) => mutation.mutateAsync({ values: confirmation, key }),
      reconcile: (key) =>
        isSupplyChainCommandCommitted('supplier_exclusion.create', key),
    })
    if (result === 'success' || result === 'reconciled') {
      finishAppend()
    } else if (result === 'rate_limited') {
      toast.error(t('Too many requests. Retry later with the same operation.'))
    } else if (result === 'pending_confirmation') {
      toast.warning(t('The result is pending confirmation.'))
    } else if (result !== 'blocked') {
      toast.error(t('Unable to append exclusion rule'))
    }
  }

  const oldAction = props.row?.action
  return (
    <>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogTrigger
          render={
            <Button
              size={props.row ? 'sm' : 'default'}
              variant={props.row ? 'outline' : 'default'}
            />
          }
        >
          {!props.row ? (
            <HugeiconsIcon icon={PlusSignIcon} strokeWidth={2} />
          ) : null}
          {props.row
            ? props.row.excluded
              ? t('Include')
              : t('Exclude')
            : t('Add exclusion rule')}
        </DialogTrigger>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Append account statistics rule')}</DialogTitle>
            <DialogDescription>
              {t(
                'Only final successful settlements are classified. This rule changes future statistics and keeps history intact.'
              )}
            </DialogDescription>
          </DialogHeader>
          <PendingConfirmationAlert
            visible={intent.isPendingConfirmation}
            onReconcile={() => void reconcilePending()}
          />
          <form
            id={`exclusion-form-${props.row?.user_id ?? 'new'}`}
            onSubmit={form.handleSubmit(setConfirmation)}
          >
            <FieldGroup>
              <Field data-invalid={Boolean(form.formState.errors.user_id)}>
                <FieldLabel
                  htmlFor={`exclusion-user-${props.row?.user_id ?? 'new'}`}
                >
                  {t('User ID')}
                </FieldLabel>
                <Input
                  id={`exclusion-user-${props.row?.user_id ?? 'new'}`}
                  type='number'
                  min={1}
                  disabled={Boolean(props.row)}
                  aria-invalid={Boolean(form.formState.errors.user_id)}
                  {...form.register('user_id', { valueAsNumber: true })}
                />
                <FieldError>
                  {form.formState.errors.user_id
                    ? t(form.formState.errors.user_id.message ?? '')
                    : null}
                </FieldError>
              </Field>
              <Field data-invalid={Boolean(form.formState.errors.action)}>
                <FieldLabel
                  htmlFor={`exclusion-action-${props.row?.user_id ?? 'new'}`}
                >
                  {t('Action')}
                </FieldLabel>
                <NativeSelect
                  id={`exclusion-action-${props.row?.user_id ?? 'new'}`}
                  className='w-full'
                  aria-invalid={Boolean(form.formState.errors.action)}
                  {...form.register('action')}
                >
                  <NativeSelectOption value='exclude'>
                    {t('Exclude from profit statistics')}
                  </NativeSelectOption>
                  <NativeSelectOption value='include'>
                    {t('Include in profit statistics')}
                  </NativeSelectOption>
                </NativeSelect>
                <FieldError>
                  {form.formState.errors.action
                    ? t(form.formState.errors.action.message ?? '')
                    : null}
                </FieldError>
              </Field>
              <Field data-invalid={Boolean(form.formState.errors.reason)}>
                <FieldLabel
                  htmlFor={`exclusion-reason-${props.row?.user_id ?? 'new'}`}
                >
                  {t('Reason')}
                </FieldLabel>
                <Textarea
                  id={`exclusion-reason-${props.row?.user_id ?? 'new'}`}
                  aria-invalid={Boolean(form.formState.errors.reason)}
                  {...form.register('reason')}
                />
                <FieldError>
                  {form.formState.errors.reason
                    ? t(form.formState.errors.reason.message ?? '')
                    : null}
                </FieldError>
              </Field>
            </FieldGroup>
          </form>
          <DialogFooter showCloseButton>
            <Button
              type='submit'
              form={`exclusion-form-${props.row?.user_id ?? 'new'}`}
            >
              {t('Review change')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <ConfirmAction
        open={confirmation !== null}
        onOpenChange={(next) => {
          if (!next) setConfirmation(null)
        }}
        title={t('Append exclusion rule')}
        description={
          <span>
            {t('User ID')}: {confirmation?.user_id}. {t('Current')}:{' '}
            {oldAction
              ? t(STATISTICS_ACTION_LABEL_KEYS[oldAction])
              : t('No rule')}{' '}
            → {t('New')}:{' '}
            {confirmation
              ? t(STATISTICS_ACTION_LABEL_KEYS[confirmation.action])
              : '—'}
            . {t('This append-only record cannot be edited later.')}
          </span>
        }
        confirmLabel={t('Append rule')}
        pending={mutation.isPending || intent.isSubmitting}
        onConfirm={confirm}
      />
    </>
  )
}

function ExclusionHistoryDialog(props: { row: SupplierEffectiveExclusion }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const query = useExclusionHistoryList(
    { p: 1, page_size: 50, user_id: props.row.user_id },
    open
  )
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size='sm' variant='ghost' />}>
        {t('History')}
      </DialogTrigger>
      <DialogContent className='max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-xl'>
        <DialogHeader>
          <DialogTitle>{t('Exclusion rule history')}</DialogTitle>
          <DialogDescription>
            {t('User ID')}: {props.row.user_id}
          </DialogDescription>
        </DialogHeader>
        <ProgressiveList
          isLoading={query.isLoading}
          isError={query.isError}
          isEmpty={!query.data?.items.length}
          hasMore={query.hasNextPage}
          isLoadingMore={query.isFetchingNextPage}
          onLoadMore={() => void query.fetchNextPage()}
        >
          <div className='flex flex-col gap-2'>
            {query.data?.items.map((rule) => (
              <div key={rule.id} className='rounded-lg border p-3'>
                <div className='flex justify-between gap-3'>
                  <Badge
                    variant={
                      rule.action === 'exclude' ? 'destructive' : 'secondary'
                    }
                  >
                    {t(STATISTICS_ACTION_LABEL_KEYS[rule.action])}
                  </Badge>
                  <span className='text-muted-foreground'>
                    {formatTime(rule.effective_at)}
                  </span>
                </div>
                {rule.reason ? (
                  <div className='text-muted-foreground mt-2'>
                    {rule.reason}
                  </div>
                ) : null}
              </div>
            ))}
          </div>
        </ProgressiveList>
        <DialogFooter showCloseButton />
      </DialogContent>
    </Dialog>
  )
}

function identityLabel(
  row: SupplierEffectiveExclusion,
  unknown: string
): string {
  if (!row.identity_present) return unknown
  return row.display_name || row.username || unknown
}

export function ExclusionManagement(props: SupplyChainManagementProps) {
  const { t } = useTranslation()
  const security = useSupplyChainSecurity()
  const params = {
    p: props.search.page,
    page_size: props.search.pageSize,
    user_id: props.search.userId,
    keyword: props.search.filter || undefined,
  }
  const query = useEffectiveExclusionList(params)

  return (
    <div className='flex flex-col gap-3'>
      <ManagementToolbar
        search={props.search}
        onSearchChange={props.onSearchChange}
        actions={
          <ExclusionRuleDialog
            security={security}
            onSaved={() => query.refetch()}
          />
        }
      />
      <ManagementStatus
        isLoading={query.isLoading}
        isError={query.isError}
        isEmpty={!query.data?.items.length}
      >
        <div className='overflow-hidden rounded-xl border'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Account')}</TableHead>
                <TableHead>{t('Role')}</TableHead>
                <TableHead>{t('Account status')}</TableHead>
                <TableHead>{t('Statistics')}</TableHead>
                <TableHead>{t('Effective at')}</TableHead>
                <TableHead>{t('Reason')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {query.data?.items.map((row) => (
                <TableRow key={row.rule_id}>
                  <TableCell>
                    <div className='font-medium'>
                      {identityLabel(row, t('Identity unavailable'))}
                    </div>
                    <div className='text-muted-foreground'>
                      {t('User ID')}: {row.user_id}
                      {row.identity_present && row.username
                        ? ` · ${row.username}`
                        : ''}
                    </div>
                  </TableCell>
                  <TableCell>
                    {row.identity_present
                      ? (row.role ?? '—')
                      : t('Unavailable')}
                  </TableCell>
                  <TableCell>
                    {row.identity_present
                      ? (row.status ?? '—')
                      : t('Unavailable')}
                  </TableCell>
                  <TableCell>
                    <Badge variant={row.excluded ? 'destructive' : 'secondary'}>
                      {row.excluded ? t('Excluded') : t('Included')}
                    </Badge>
                  </TableCell>
                  <TableCell>{formatTime(row.effective_at)}</TableCell>
                  <TableCell className='max-w-64 truncate'>
                    {row.reason || '—'}
                  </TableCell>
                  <TableCell>
                    <div className='flex justify-end gap-1'>
                      <ExclusionHistoryDialog row={row} />
                      <ExclusionRuleDialog
                        row={row}
                        defaultAction={row.excluded ? 'include' : 'exclude'}
                        security={security}
                        onSaved={() => query.refetch()}
                      />
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </ManagementStatus>
      <ManagementPagination
        page={props.search.page}
        pageSize={props.search.pageSize}
        total={query.data?.total ?? 0}
        onSearchChange={props.onSearchChange}
      />
      <SupplyChainVerificationDialog security={security} />
    </div>
  )
}
