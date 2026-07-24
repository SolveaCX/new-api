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
import { Link01Icon } from '@hugeicons/core-free-icons'
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
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  bindChannel,
  isSupplyChainCommandCommitted,
  unbindChannel,
} from '../api'
import type { SupplyChainManagementProps } from '../contracts'
import { useIdempotentIntent } from '../hooks/use-idempotent-intent'
import {
  useChannelBindingAdminList,
  useContractAdminInfiniteList,
  useSupplyChainAdminMutation,
  useSupplyChainSecurity,
  type SupplyChainSecurity,
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
  PendingConfirmationAlert,
  SupplyChainVerificationDialog,
} from './management-common'
import { ProgressiveList } from './progressive-list'

function BindingDialog(props: {
  binding: SupplierChannelBinding
  reconcile: (channelId: number, contractId: number) => Promise<boolean>
  security: SupplyChainSecurity
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const intent = useIdempotentIntent()
  const contracts = useContractAdminInfiniteList(
    { page_size: 50, status: 'active' },
    open
  )
  const form = useForm<ChannelBindingFormValues>({
    resolver: zodResolver(channelBindingFormSchema),
    defaultValues: { contract_id: props.binding.supplier_contract_id ?? 0 },
  })
  const mutation = useSupplyChainAdminMutation<{
    values: ChannelBindingFormValues
    idempotencyKey: string
  }>({
    mutationFn: ({ values, idempotencyKey }) =>
      bindChannel(props.binding.channel_id, {
        data: {
          contract_id: values.contract_id,
          expected_contract_id: props.binding.supplier_contract_id ?? 0,
        },
        idempotencyKey,
      }),
    invalidate: [
      supplyChainQueryKeys.channelBindings.all(),
      supplyChainQueryKeys.contracts.all(),
      supplyChainQueryKeys.suppliers.all(),
    ],
    security: props.security,
  })

  useEffect(() => {
    if (open)
      form.reset({ contract_id: props.binding.supplier_contract_id ?? 0 })
  }, [form, open, props.binding.supplier_contract_id])

  function finishBinding(): void {
    toast.success(t('Channel binding updated'))
    setOpen(false)
  }

  async function reconcilePending(): Promise<void> {
    if ((await intent.reconcilePending()) === 'reconciled') finishBinding()
  }

  async function submit(values: ChannelBindingFormValues): Promise<void> {
    const result = await intent.run({
      execute: (idempotencyKey) =>
        mutation.mutateAsync({ values, idempotencyKey }),
      reconcile: async (key) => {
        const committed = await isSupplyChainCommandCommitted(
          `supplier_channel.bind/${props.binding.channel_id}`,
          key
        )
        await props.reconcile(props.binding.channel_id, values.contract_id)
        return committed
      },
    })
    if (result === 'success' || result === 'reconciled') {
      finishBinding()
    } else if (result === 'conflict') {
      toast.error(
        t(
          'The binding changed elsewhere. Latest data was loaded; review your selection and try again.'
        )
      )
    } else if (result === 'rate_limited') {
      toast.error(t('Too many requests. Retry later with the same operation.'))
    } else if (result === 'pending_confirmation') {
      toast.warning(t('The result is pending confirmation.'))
    } else if (result !== 'blocked') {
      toast.error(t('Unable to update channel binding'))
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
        <PendingConfirmationAlert
          visible={intent.isPendingConfirmation}
          onReconcile={() => void reconcilePending()}
        />
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
                    <NativeSelectOption key={contract.id} value={contract.id}>
                      {contract.supplier_name} · {contract.name} ·{' '}
                      {contract.contract_no}
                    </NativeSelectOption>
                  ))}
                </NativeSelect>
              </ProgressiveList>
              <FieldError>
                {form.formState.errors.contract_id
                  ? t(form.formState.errors.contract_id.message ?? '')
                  : null}
              </FieldError>
            </Field>
          </FieldGroup>
        </form>
        <DialogFooter showCloseButton>
          <Button
            type='submit'
            form={`binding-form-${props.binding.channel_id}`}
            disabled={
              mutation.isPending ||
              intent.isSubmitting ||
              intent.isPendingConfirmation
            }
          >
            {t('Save binding')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function UnbindAction(props: {
  binding: SupplierChannelBinding
  reconcile: (channelId: number, contractId: number | null) => Promise<boolean>
  security: SupplyChainSecurity
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const intent = useIdempotentIntent()
  const mutation = useSupplyChainAdminMutation<{ idempotencyKey: string }>({
    mutationFn: ({ idempotencyKey }) =>
      unbindChannel(props.binding.channel_id, {
        expectedContractId: props.binding.supplier_contract_id ?? 0,
        idempotencyKey,
      }),
    invalidate: [
      supplyChainQueryKeys.channelBindings.all(),
      supplyChainQueryKeys.contracts.all(),
      supplyChainQueryKeys.suppliers.all(),
    ],
    security: props.security,
  })

  function finishUnbind(): void {
    toast.success(t('Channel unbound'))
    setOpen(false)
  }

  async function reconcilePending(): Promise<void> {
    if ((await intent.reconcilePending()) === 'reconciled') finishUnbind()
  }

  async function confirm(): Promise<void> {
    const result = await intent.run({
      execute: (idempotencyKey) => mutation.mutateAsync({ idempotencyKey }),
      reconcile: async (key) => {
        const committed = await isSupplyChainCommandCommitted(
          `supplier_channel.unbind/${props.binding.channel_id}`,
          key
        )
        await props.reconcile(props.binding.channel_id, null)
        return committed
      },
    })
    if (result === 'success' || result === 'reconciled') {
      finishUnbind()
    } else if (result === 'conflict') {
      toast.error(
        t(
          'The binding changed elsewhere. Latest data was loaded and no change was made.'
        )
      )
    } else if (result === 'pending_confirmation') {
      toast.warning(t('The result is pending confirmation.'))
    } else if (result === 'rate_limited') {
      toast.error(t('Too many requests. Retry later with the same operation.'))
    } else if (result !== 'blocked') {
      toast.error(t('Unable to unbind channel'))
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
      <PendingConfirmationAlert
        visible={intent.isPendingConfirmation}
        onReconcile={() => void reconcilePending()}
      />
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
        pending={mutation.isPending || intent.isSubmitting}
        destructive
        onConfirm={confirm}
      />
    </>
  )
}

export function ChannelBindingManagement(props: SupplyChainManagementProps) {
  const { t } = useTranslation()
  const security = useSupplyChainSecurity()
  const [boundState, setBoundState] = useState<'bound' | 'unbound' | ''>('')
  const query = useChannelBindingAdminList({
    p: props.search.page,
    page_size: props.search.pageSize,
    contract_id: props.search.contractId,
    keyword: props.search.filter || undefined,
    bound_state: boundState || undefined,
  })

  async function reconcile(
    channelId: number,
    contractId: number | null
  ): Promise<boolean> {
    const result = await query.refetch()
    const binding = result.data?.items.find(
      (item) => item.channel_id === channelId
    )
    return Boolean(binding && binding.supplier_contract_id === contractId)
  }

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
                    <div className='flex justify-end gap-1'>
                      <BindingDialog
                        binding={binding}
                        reconcile={reconcile}
                        security={security}
                      />
                      {binding.supplier_contract_id ? (
                        <UnbindAction
                          binding={binding}
                          reconcile={reconcile}
                          security={security}
                        />
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
      <SupplyChainVerificationDialog security={security} />
    </div>
  )
}
