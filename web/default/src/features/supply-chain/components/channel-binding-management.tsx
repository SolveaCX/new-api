/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useEffect, useState } from 'react'
import { AxiosError } from 'axios'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Link01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { bindChannel, unbindChannel } from '../api'
import type { SupplyChainManagementProps } from '../contracts'
import {
  useChannelBindingAdminList,
  useContractAdminInfiniteList,
  useSupplyChainAdminMutation,
} from '../hooks/use-supply-chain-admin'
import { formatPpmPercent } from '../lib/format'
import {
  channelBindingFormSchema,
  type ChannelBindingFormValues,
} from '../lib/schemas'
import { supplyChainQueryKeys } from '../query-keys'
import type { SupplierChannelBinding } from '../types'
import {
  ConfirmAction,
  ManagementPagination,
  ManagementStatus,
  ManagementToolbar,
} from './management-common'
import { ProgressiveList } from './progressive-list'

function bindingErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof AxiosError) {
    const message = error.response?.data?.message
    if (typeof message === 'string' && message.trim()) return message
  }
  return fallback
}

function BindingDialog(props: { binding: SupplierChannelBinding }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const contracts = useContractAdminInfiniteList(
    { page_size: 50, status: 'active' },
    open
  )
  const form = useForm<ChannelBindingFormValues>({
    resolver: zodResolver(channelBindingFormSchema),
    defaultValues: {
      contract_id: props.binding.supplier_contract_id ?? 0,
      skip_internal_accounting: props.binding.skip_internal_accounting ?? false,
    },
  })
  const mutation = useSupplyChainAdminMutation<ChannelBindingFormValues>({
    mutationFn: (values) =>
      bindChannel(props.binding.channel_id, {
        contract_id: values.contract_id,
        expected_contract_id: props.binding.supplier_contract_id ?? 0,
        skip_internal_accounting: values.skip_internal_accounting,
        expected_skip_internal_accounting:
          props.binding.skip_internal_accounting ?? false,
      }),
    invalidate: [
      supplyChainQueryKeys.channelBindings.all(),
      supplyChainQueryKeys.contracts.all(),
      supplyChainQueryKeys.suppliers.all(),
    ],
    onError: (error) =>
      toast.error(
        bindingErrorMessage(error, t('Unable to update channel binding'))
      ),
  })

  useEffect(() => {
    if (open) {
      form.reset({
        contract_id: props.binding.supplier_contract_id ?? 0,
        skip_internal_accounting:
          props.binding.skip_internal_accounting ?? false,
      })
    }
  }, [
    form,
    open,
    props.binding.skip_internal_accounting,
    props.binding.supplier_contract_id,
  ])

  function finishBinding(): void {
    toast.success(t('Channel binding updated'))
    setOpen(false)
  }

  async function submit(values: ChannelBindingFormValues): Promise<void> {
    try {
      await mutation.mutateAsync(values)
      finishBinding()
    } catch {
      return
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size='sm' variant='outline' />}>
        <HugeiconsIcon icon={Link01Icon} strokeWidth={2} />
        {props.binding.supplier_contract_id ? t('Rebind') : t('Bind')}
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('Bind channel to contract')}</DialogTitle>
          <DialogDescription>
            {props.binding.channel_name} · {t('Current')}:{' '}
            {props.binding.contract_name ?? t('Unbound')}
          </DialogDescription>
        </DialogHeader>
        <form
          id={`binding-form-${props.binding.channel_id}`}
          onSubmit={form.handleSubmit(submit)}
        >
          <FieldGroup>
            <Field data-invalid={Boolean(form.formState.errors.contract_id)}>
              <FieldLabel
                htmlFor={`binding-contract-${props.binding.channel_id}`}
              >
                {t('Contract')}
              </FieldLabel>
              <ProgressiveList
                isLoading={contracts.isLoading}
                isError={contracts.isError}
                isEmpty={!contracts.data?.items.length}
                hasMore={contracts.hasNextPage}
                isLoadingMore={contracts.isFetchingNextPage}
                onLoadMore={() => void contracts.fetchNextPage()}
              >
                <NativeSelect
                  id={`binding-contract-${props.binding.channel_id}`}
                  className='w-full'
                  aria-invalid={Boolean(form.formState.errors.contract_id)}
                  value={form.watch('contract_id') || ''}
                  onChange={(event) =>
                    form.setValue('contract_id', Number(event.target.value), {
                      shouldValidate: true,
                    })
                  }
                >
                  <NativeSelectOption value=''>
                    {t('Select contract')}
                  </NativeSelectOption>
                  {contracts.data?.items.map((contract) => (
                    <NativeSelectOption
                      key={contract.id}
                      value={contract.id}
                      disabled={contract.current_rate_version_id === null}
                    >
                      {contract.supplier_name} · {contract.name} ·{' '}
                      {contract.contract_no}
                      {contract.current_rate_version_id === null
                        ? ` · ${t('Rate required')}`
                        : null}
                    </NativeSelectOption>
                  ))}
                </NativeSelect>
              </ProgressiveList>
              <FieldDescription>
                {t(
                  'Contracts without a current procurement rate cannot be bound. Create a rate first.'
                )}
              </FieldDescription>
              <FieldError>
                {form.formState.errors.contract_id
                  ? t(form.formState.errors.contract_id.message ?? '')
                  : null}
              </FieldError>
            </Field>
            <Field
              data-invalid={Boolean(
                form.formState.errors.skip_internal_accounting
              )}
            >
              <FieldLabel
                htmlFor={`binding-internal-policy-${props.binding.channel_id}`}
              >
                {t('Internal request accounting')}
              </FieldLabel>
              <NativeSelect
                id={`binding-internal-policy-${props.binding.channel_id}`}
                className='w-full'
                aria-invalid={Boolean(
                  form.formState.errors.skip_internal_accounting
                )}
                value={
                  form.watch('skip_internal_accounting') ? 'skip' : 'record'
                }
                onChange={(event) =>
                  form.setValue(
                    'skip_internal_accounting',
                    event.target.value === 'skip',
                    { shouldValidate: true }
                  )
                }
              >
                <NativeSelectOption value='record'>
                  {t('Record internal costs')}
                </NativeSelectOption>
                <NativeSelectOption value='skip'>
                  {t('Skip completely')}
                </NativeSelectOption>
              </NativeSelect>
              <FieldDescription>
                {t(
                  'This policy applies only to requests from accounts excluded from supplier statistics.'
                )}
              </FieldDescription>
              <FieldError>
                {form.formState.errors.skip_internal_accounting
                  ? t(
                      form.formState.errors.skip_internal_accounting.message ??
                        ''
                    )
                  : null}
              </FieldError>
            </Field>
            {form.watch('skip_internal_accounting') ? (
              <Alert variant='destructive'>
                <AlertTitle>
                  {t('Internal request data will not be recorded')}
                </AlertTitle>
                <AlertDescription>
                  {t(
                    'Supplier accounting facts will not be created, so internal procurement costs and inventory consumption cannot be recovered later.'
                  )}
                </AlertDescription>
              </Alert>
            ) : null}
          </FieldGroup>
        </form>
        <DialogFooter showCloseButton>
          <Button
            type='submit'
            form={`binding-form-${props.binding.channel_id}`}
            disabled={mutation.isPending}
          >
            {t('Save binding')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function UnbindAction(props: { binding: SupplierChannelBinding }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const mutation = useSupplyChainAdminMutation<void>({
    mutationFn: () =>
      unbindChannel(props.binding.channel_id, {
        expectedContractId: props.binding.supplier_contract_id ?? 0,
        expectedSkipInternalAccounting:
          props.binding.skip_internal_accounting ?? false,
      }),
    invalidate: [
      supplyChainQueryKeys.channelBindings.all(),
      supplyChainQueryKeys.contracts.all(),
      supplyChainQueryKeys.suppliers.all(),
    ],
    onError: (error) =>
      toast.error(bindingErrorMessage(error, t('Unable to unbind channel'))),
  })

  function finishUnbind(): void {
    toast.success(t('Channel unbound'))
    setOpen(false)
  }

  async function confirm(): Promise<void> {
    try {
      await mutation.mutateAsync()
      finishUnbind()
    } catch {
      return
    }
  }

  return (
    <>
      <Button
        type='button'
        size='sm'
        variant='ghost'
        onClick={() => setOpen(true)}
      >
        {t('Unbind')}
      </Button>
      <ConfirmAction
        open={open}
        onOpenChange={setOpen}
        title={t('Unbind channel')}
        description={
          <span>
            {props.binding.channel_name}: {props.binding.contract_name} (
            {props.binding.contract_no}) → {t('Unbound')}.{' '}
            {t(
              'Future successful requests will no longer be attributed to this contract.'
            )}
          </span>
        }
        confirmLabel={t('Unbind')}
        pending={mutation.isPending}
        destructive
        onConfirm={confirm}
      />
    </>
  )
}

export function ChannelBindingManagement(props: SupplyChainManagementProps) {
  const { t } = useTranslation()
  const [boundState, setBoundState] = useState<'bound' | 'unbound' | ''>('')
  const query = useChannelBindingAdminList({
    p: props.search.page,
    page_size: props.search.pageSize,
    contract_id: props.search.contractId,
    keyword: props.search.filter || undefined,
    bound_state: boundState || undefined,
  })

  return (
    <div className='flex flex-col gap-3'>
      <ManagementToolbar
        search={props.search}
        onSearchChange={props.onSearchChange}
        filters={
          <NativeSelect
            aria-label={t('Binding state')}
            value={boundState}
            onChange={(event) => {
              setBoundState(event.target.value as '' | 'bound' | 'unbound')
              props.onSearchChange({ page: 1 })
            }}
          >
            <NativeSelectOption value=''>
              {t('All binding states')}
            </NativeSelectOption>
            <NativeSelectOption value='bound'>{t('Bound')}</NativeSelectOption>
            <NativeSelectOption value='unbound'>
              {t('Unbound')}
            </NativeSelectOption>
          </NativeSelect>
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
                <TableHead>{t('Channel')}</TableHead>
                <TableHead>{t('Channel status')}</TableHead>
                <TableHead>{t('Supplier')}</TableHead>
                <TableHead>{t('Contract')}</TableHead>
                <TableHead>{t('Procurement multiplier')}</TableHead>
                <TableHead>{t('Internal requests')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {query.data?.items.map((binding) => (
                <TableRow key={binding.channel_id}>
                  <TableCell>
                    <div className='font-medium'>{binding.channel_name}</div>
                    <div className='text-muted-foreground'>
                      #{binding.channel_id}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={
                        binding.channel_status === 1 ? 'default' : 'secondary'
                      }
                    >
                      {binding.channel_status === 1
                        ? t('Enabled')
                        : t('Disabled')}
                    </Badge>
                  </TableCell>
                  <TableCell>{binding.supplier_name ?? '—'}</TableCell>
                  <TableCell>
                    {binding.contract_name ? (
                      <>
                        <div>{binding.contract_name}</div>
                        <div className='text-muted-foreground'>
                          {binding.contract_no}
                        </div>
                      </>
                    ) : (
                      <Badge variant='outline'>{t('Unbound')}</Badge>
                    )}
                  </TableCell>
                  <TableCell>
                    {formatPpmPercent(
                      binding.current_procurement_multiplier_ppm,
                      t('Unknown')
                    )}
                  </TableCell>
                  <TableCell>
                    {binding.supplier_contract_id ? (
                      <Badge
                        variant={
                          binding.skip_internal_accounting
                            ? 'destructive'
                            : 'secondary'
                        }
                      >
                        {binding.skip_internal_accounting
                          ? t('Skip completely')
                          : t('Record costs')}
                      </Badge>
                    ) : (
                      '—'
                    )}
                  </TableCell>
                  <TableCell>
                    <div className='flex justify-end gap-1'>
                      <BindingDialog binding={binding} />
                      {binding.supplier_contract_id ? (
                        <UnbindAction binding={binding} />
                      ) : null}
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
    </div>
  )
}
