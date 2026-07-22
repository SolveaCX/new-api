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
import { Edit02Icon, PlusSignIcon } from '@hugeicons/core-free-icons'
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
import {
  createContract,
  createInventoryAdjustment,
  createRateVersion,
  inactivateContract,
  isSupplyChainCommandCommitted,
  updateContract,
} from '../api'
import type { SupplyChainManagementProps } from '../contracts'
import { useIdempotentIntent } from '../hooks/use-idempotent-intent'
import {
  useContractAdminList,
  useInventoryAdjustmentList,
  useRateVersionList,
  useSupplierAdminInfiniteList,
  useSupplyChainAdminMutation,
} from '../hooks/use-supply-chain-admin'
import {
  formatMicroUsd,
  formatPpmDiscount,
  formatPpmPercent,
} from '../lib/format'
import {
  contractFormSchema,
  inventoryAdjustmentFormSchema,
  rateVersionFormSchema,
  usdInputToMicroUsd,
  type ContractFormValues,
  type InventoryAdjustmentFormValues,
  type RateVersionFormValues,
} from '../lib/schemas'
import { formatTime } from '../lib/time'
import { supplyChainQueryKeys } from '../query-keys'
import type { SupplierContract } from '../types'
import {
  ConfirmAction,
  ManagementPagination,
  ManagementStatus,
  ManagementToolbar,
  PendingConfirmationAlert,
  StatusBadge,
} from './management-common'
import { ProgressiveList } from './progressive-list'

const EMPTY_CONTRACT: ContractFormValues = {
  supplier_id: 0,
  name: '',
  contract_no: '',
  remark: '',
  rpm_limit: 0,
  tpm_limit: 0,
  max_concurrency: 0,
}

function ContractFormDialog(props: {
  contract?: SupplierContract
  onSaved: () => void
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const intent = useIdempotentIntent()
  const suppliers = useSupplierAdminInfiniteList(
    { page_size: 50, status: 'active' },
    open
  )
  const form = useForm<ContractFormValues>({
    resolver: zodResolver(contractFormSchema),
    defaultValues: EMPTY_CONTRACT,
  })
  const mutation = useSupplyChainAdminMutation<{
    values: ContractFormValues
    idempotencyKey?: string
  }>({
    mutationFn: async ({ values, idempotencyKey }) =>
      props.contract
        ? updateContract(props.contract.id, values)
        : createContract({
            data: values,
            idempotencyKey: idempotencyKey ?? '',
          }),
    invalidate: [
      supplyChainQueryKeys.contracts.all(),
      supplyChainQueryKeys.suppliers.all(),
    ],
  })

  useEffect(() => {
    if (!open) return
    form.reset(
      props.contract
        ? {
            supplier_id: props.contract.supplier_id,
            name: props.contract.name,
            contract_no: props.contract.contract_no,
            remark: props.contract.remark,
            rpm_limit: props.contract.rpm_limit,
            tpm_limit: props.contract.tpm_limit,
            max_concurrency: props.contract.max_concurrency,
          }
        : EMPTY_CONTRACT
    )
  }, [form, open, props.contract])

  async function submit(values: ContractFormValues): Promise<void> {
    if (props.contract) {
      try {
        await mutation.mutateAsync({ values })
        toast.success(t('Contract updated'))
        setOpen(false)
        props.onSaved()
      } catch {
        toast.error(t('Unable to save contract'))
      }
      return
    }
    const result = await intent.run({
      execute: async (idempotencyKey) =>
        mutation.mutateAsync({ values, idempotencyKey }),
      reconcile: (key) =>
        isSupplyChainCommandCommitted('supplier_contract.create', key),
    })
    if (result === 'success' || result === 'reconciled') {
      toast.success(t('Contract created'))
      setOpen(false)
      props.onSaved()
    } else if (result === 'rate_limited') {
      toast.error(t('Too many requests. Retry later with the same operation.'))
    } else if (result === 'pending_confirmation') {
      toast.warning(t('The result is pending confirmation.'))
    } else if (result !== 'blocked') {
      toast.error(t('Unable to save contract'))
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger
        render={
          <Button
            variant={props.contract ? 'ghost' : 'default'}
            size={props.contract ? 'icon-sm' : 'default'}
          />
        }
      >
        <HugeiconsIcon
          icon={props.contract ? Edit02Icon : PlusSignIcon}
          strokeWidth={2}
        />
        {props.contract ? (
          <span className='sr-only'>{t('Edit')}</span>
        ) : (
          t('Create contract')
        )}
      </DialogTrigger>
      <DialogContent className='max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-xl'>
        <DialogHeader>
          <DialogTitle>
            {props.contract ? t('Edit contract') : t('Create contract')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'Inventory is tracked in official list-price USD for this contract.'
            )}
          </DialogDescription>
        </DialogHeader>
        <PendingConfirmationAlert
          visible={intent.isPendingConfirmation}
          onReconcile={() => void intent.reconcilePending()}
        />
        <form
          id={`contract-form-${props.contract?.id ?? 'new'}`}
          onSubmit={form.handleSubmit(submit)}
        >
          <FieldGroup className='grid gap-4 sm:grid-cols-2'>
            <Field data-invalid={Boolean(form.formState.errors.supplier_id)}>
              <FieldLabel
                htmlFor={`contract-supplier-${props.contract?.id ?? 'new'}`}
              >
                {t('Supplier')}
              </FieldLabel>
              <ProgressiveList
                isLoading={suppliers.isLoading}
                isError={suppliers.isError}
                isEmpty={!suppliers.data?.items.length}
                hasMore={suppliers.hasNextPage}
                isLoadingMore={suppliers.isFetchingNextPage}
                onLoadMore={() => void suppliers.fetchNextPage()}
              >
                <NativeSelect
                  id={`contract-supplier-${props.contract?.id ?? 'new'}`}
                  className='w-full'
                  disabled={Boolean(props.contract)}
                  value={form.watch('supplier_id') || ''}
                  onChange={(event) =>
                    form.setValue('supplier_id', Number(event.target.value), {
                      shouldValidate: true,
                    })
                  }
                >
                  <NativeSelectOption value=''>
                    {t('Select supplier')}
                  </NativeSelectOption>
                  {suppliers.data?.items.map((supplier) => (
                    <NativeSelectOption key={supplier.id} value={supplier.id}>
                      {supplier.name}
                    </NativeSelectOption>
                  ))}
                </NativeSelect>
              </ProgressiveList>
              <FieldError>
                {form.formState.errors.supplier_id
                  ? t(form.formState.errors.supplier_id.message ?? '')
                  : null}
              </FieldError>
            </Field>
            <Field data-invalid={Boolean(form.formState.errors.contract_no)}>
              <FieldLabel
                htmlFor={`contract-number-${props.contract?.id ?? 'new'}`}
              >
                {t('Contract number')}
              </FieldLabel>
              <Input
                id={`contract-number-${props.contract?.id ?? 'new'}`}
                {...form.register('contract_no')}
              />
              <FieldError>
                {form.formState.errors.contract_no
                  ? t(form.formState.errors.contract_no.message ?? '')
                  : null}
              </FieldError>
            </Field>
            <Field data-invalid={Boolean(form.formState.errors.name)}>
              <FieldLabel
                htmlFor={`contract-name-${props.contract?.id ?? 'new'}`}
              >
                {t('Name')}
              </FieldLabel>
              <Input
                id={`contract-name-${props.contract?.id ?? 'new'}`}
                {...form.register('name')}
              />
              <FieldError>
                {form.formState.errors.name
                  ? t(form.formState.errors.name.message ?? '')
                  : null}
              </FieldError>
            </Field>
            <Field>
              <FieldLabel
                htmlFor={`contract-concurrency-${props.contract?.id ?? 'new'}`}
              >
                {t('Maximum concurrency')}
              </FieldLabel>
              <Input
                id={`contract-concurrency-${props.contract?.id ?? 'new'}`}
                type='number'
                min={0}
                {...form.register('max_concurrency', { valueAsNumber: true })}
              />
            </Field>
            <Field>
              <FieldLabel
                htmlFor={`contract-rpm-${props.contract?.id ?? 'new'}`}
              >
                {t('RPM limit')}
              </FieldLabel>
              <Input
                id={`contract-rpm-${props.contract?.id ?? 'new'}`}
                type='number'
                min={0}
                {...form.register('rpm_limit', { valueAsNumber: true })}
              />
            </Field>
            <Field>
              <FieldLabel
                htmlFor={`contract-tpm-${props.contract?.id ?? 'new'}`}
              >
                {t('TPM limit')}
              </FieldLabel>
              <Input
                id={`contract-tpm-${props.contract?.id ?? 'new'}`}
                type='number'
                min={0}
                {...form.register('tpm_limit', { valueAsNumber: true })}
              />
            </Field>
            <Field className='sm:col-span-2'>
              <FieldLabel
                htmlFor={`contract-remark-${props.contract?.id ?? 'new'}`}
              >
                {t('Remark')}
              </FieldLabel>
              <Textarea
                id={`contract-remark-${props.contract?.id ?? 'new'}`}
                {...form.register('remark')}
              />
            </Field>
          </FieldGroup>
        </form>
        <DialogFooter showCloseButton>
          <Button
            type='submit'
            form={`contract-form-${props.contract?.id ?? 'new'}`}
            disabled={
              mutation.isPending ||
              intent.isSubmitting ||
              intent.isPendingConfirmation
            }
          >
            {mutation.isPending || intent.isSubmitting ? <Spinner /> : null}
            {t('Save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function RateVersionDialog(props: {
  contract: SupplierContract
  onSaved: () => void
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [confirmation, setConfirmation] =
    useState<RateVersionFormValues | null>(null)
  const intent = useIdempotentIntent()
  const form = useForm<RateVersionFormValues>({
    resolver: zodResolver(rateVersionFormSchema),
    defaultValues: {
      procurement_multiplier_ppm:
        props.contract.current_procurement_multiplier_ppm ?? 650_000,
      reason: '',
    },
  })
  const mutation = useSupplyChainAdminMutation<{
    values: RateVersionFormValues
    key: string
  }>({
    mutationFn: ({ values, key }) =>
      createRateVersion(props.contract.id, {
        data: values,
        idempotencyKey: key,
      }),
    invalidate: [
      supplyChainQueryKeys.contracts.all(),
      supplyChainQueryKeys.channelBindings.all(),
    ],
  })

  async function confirm(): Promise<void> {
    if (!confirmation) return
    const result = await intent.run({
      execute: (key) => mutation.mutateAsync({ values: confirmation, key }),
      reconcile: (key) =>
        isSupplyChainCommandCommitted('supplier_rate.create', key),
    })
    if (result === 'success' || result === 'reconciled') {
      toast.success(t('Procurement multiplier version appended'))
      setConfirmation(null)
      setOpen(false)
      props.onSaved()
    } else if (result === 'rate_limited')
      toast.error(t('Too many requests. Retry later with the same operation.'))
    else if (result === 'pending_confirmation')
      toast.warning(t('The result is pending confirmation.'))
    else if (result !== 'blocked')
      toast.error(t('Unable to append rate version'))
  }

  return (
    <>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogTrigger render={<Button size='sm' variant='outline' />}>
          {t('New rate')}
        </DialogTrigger>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Append procurement multiplier')}</DialogTitle>
            <DialogDescription>
              {t(
                'Historical profit keeps the multiplier effective when each request was settled.'
              )}
            </DialogDescription>
          </DialogHeader>
          <PendingConfirmationAlert
            visible={intent.isPendingConfirmation}
            onReconcile={() => void intent.reconcilePending()}
          />
          <form
            id={`rate-form-${props.contract.id}`}
            onSubmit={form.handleSubmit(setConfirmation)}
          >
            <FieldGroup>
              <Field
                data-invalid={Boolean(
                  form.formState.errors.procurement_multiplier_ppm
                )}
              >
                <FieldLabel htmlFor={`rate-ppm-${props.contract.id}`}>
                  {t('Procurement multiplier (PPM)')}
                </FieldLabel>
                <Input
                  id={`rate-ppm-${props.contract.id}`}
                  type='number'
                  min={0}
                  max={1_000_000}
                  {...form.register('procurement_multiplier_ppm', {
                    valueAsNumber: true,
                  })}
                />
                <div className='text-muted-foreground text-sm'>
                  {formatPpmPercent(
                    form.watch('procurement_multiplier_ppm'),
                    t('Unknown')
                  )}{' '}
                  ·{' '}
                  {formatPpmDiscount(
                    form.watch('procurement_multiplier_ppm'),
                    t('discount unit'),
                    t('Unknown')
                  )}
                </div>
                <FieldError>
                  {form.formState.errors.procurement_multiplier_ppm
                    ? t(
                        form.formState.errors.procurement_multiplier_ppm
                          .message ?? ''
                      )
                    : null}
                </FieldError>
              </Field>
              <Field>
                <FieldLabel htmlFor={`rate-reason-${props.contract.id}`}>
                  {t('Reason')}
                </FieldLabel>
                <Textarea
                  id={`rate-reason-${props.contract.id}`}
                  {...form.register('reason')}
                />
              </Field>
            </FieldGroup>
          </form>
          <DialogFooter showCloseButton>
            <Button type='submit' form={`rate-form-${props.contract.id}`}>
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
        title={t('Append rate version')}
        description={
          <span>
            {t('Current')}:{' '}
            {formatPpmPercent(
              props.contract.current_procurement_multiplier_ppm,
              t('Unknown')
            )}{' '}
            → {t('New')}:{' '}
            {formatPpmPercent(
              confirmation?.procurement_multiplier_ppm,
              t('Unknown')
            )}
            .{' '}
            {t(
              'This append-only change affects future successful settlements only.'
            )}
          </span>
        }
        confirmLabel={t('Append version')}
        pending={mutation.isPending || intent.isSubmitting}
        onConfirm={confirm}
      />
    </>
  )
}

function InventoryDialog(props: {
  contract: SupplierContract
  onSaved: () => void
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [confirmation, setConfirmation] =
    useState<InventoryAdjustmentFormValues | null>(null)
  const intent = useIdempotentIntent()
  const form = useForm<InventoryAdjustmentFormValues>({
    resolver: zodResolver(inventoryAdjustmentFormSchema),
    defaultValues: { delta_usd: '', type: 'replenishment', reason: '' },
  })
  const mutation = useSupplyChainAdminMutation<{
    values: InventoryAdjustmentFormValues
    key: string
  }>({
    mutationFn: ({ values, key }) =>
      createInventoryAdjustment(props.contract.id, {
        data: {
          delta_micro_usd: usdInputToMicroUsd(values.delta_usd) ?? 0,
          type: values.type,
          reason: values.reason,
        },
        idempotencyKey: key,
      }),
    invalidate: [
      supplyChainQueryKeys.contracts.all(),
      supplyChainQueryKeys.suppliers.all(),
    ],
  })

  async function confirm(): Promise<void> {
    if (!confirmation) return
    const result = await intent.run({
      execute: (key) => mutation.mutateAsync({ values: confirmation, key }),
      reconcile: (key) =>
        isSupplyChainCommandCommitted(
          `supplier_inventory.create/${props.contract.id}`,
          key
        ),
    })
    if (result === 'success' || result === 'reconciled') {
      toast.success(t('Inventory adjustment appended'))
      setConfirmation(null)
      setOpen(false)
      props.onSaved()
    } else if (result === 'rate_limited')
      toast.error(t('Too many requests. Retry later with the same operation.'))
    else if (result === 'pending_confirmation')
      toast.warning(t('The result is pending confirmation.'))
    else if (result !== 'blocked')
      toast.error(t('Unable to append inventory adjustment'))
  }

  const delta = confirmation ? usdInputToMicroUsd(confirmation.delta_usd) : null
  return (
    <>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogTrigger render={<Button size='sm' variant='outline' />}>
          {t('Adjust inventory')}
        </DialogTrigger>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Append inventory adjustment')}</DialogTitle>
            <DialogDescription>
              {t(
                'Inventory uses official list-price USD. Positive values add inventory; negative values reduce it.'
              )}
            </DialogDescription>
          </DialogHeader>
          <PendingConfirmationAlert
            visible={intent.isPendingConfirmation}
            onReconcile={() => void intent.reconcilePending()}
          />
          <form
            id={`inventory-form-${props.contract.id}`}
            onSubmit={form.handleSubmit(setConfirmation)}
          >
            <FieldGroup>
              <Field data-invalid={Boolean(form.formState.errors.delta_usd)}>
                <FieldLabel htmlFor={`inventory-delta-${props.contract.id}`}>
                  {t('Adjustment amount (USD)')}
                </FieldLabel>
                <Input
                  id={`inventory-delta-${props.contract.id}`}
                  inputMode='decimal'
                  placeholder='200000'
                  {...form.register('delta_usd')}
                />
                <FieldError>
                  {form.formState.errors.delta_usd
                    ? t(form.formState.errors.delta_usd.message ?? '')
                    : null}
                </FieldError>
              </Field>
              <Field>
                <FieldLabel htmlFor={`inventory-type-${props.contract.id}`}>
                  {t('Adjustment type')}
                </FieldLabel>
                <NativeSelect
                  id={`inventory-type-${props.contract.id}`}
                  className='w-full'
                  {...form.register('type')}
                >
                  <NativeSelectOption value='initial'>
                    {t('Initial')}
                  </NativeSelectOption>
                  <NativeSelectOption value='replenishment'>
                    {t('Replenishment')}
                  </NativeSelectOption>
                  <NativeSelectOption value='correction'>
                    {t('Correction')}
                  </NativeSelectOption>
                  <NativeSelectOption value='reversal'>
                    {t('Reversal')}
                  </NativeSelectOption>
                </NativeSelect>
              </Field>
              <Field>
                <FieldLabel htmlFor={`inventory-reason-${props.contract.id}`}>
                  {t('Reason')}
                </FieldLabel>
                <Textarea
                  id={`inventory-reason-${props.contract.id}`}
                  {...form.register('reason')}
                />
              </Field>
            </FieldGroup>
          </form>
          <DialogFooter showCloseButton>
            <Button type='submit' form={`inventory-form-${props.contract.id}`}>
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
        title={t('Append inventory adjustment')}
        description={
          <span>
            {t('Current inventory')}:{' '}
            {formatMicroUsd(
              props.contract.inventory_total_micro_usd,
              t('Unknown')
            )}
            ; {t('Adjustment')}: {formatMicroUsd(delta, t('Unknown'))}.{' '}
            {t('This record cannot be edited after it is appended.')}
          </span>
        }
        confirmLabel={t('Append adjustment')}
        pending={mutation.isPending || intent.isSubmitting}
        onConfirm={confirm}
      />
    </>
  )
}

function ContractHistoryDialog(props: { contract: SupplierContract }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const params = { contract_id: props.contract.id, p: 1, page_size: 50 }
  const rates = useRateVersionList(params, open)
  const inventory = useInventoryAdjustmentList(params, open)
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size='sm' variant='ghost' />}>
        {t('History')}
      </DialogTrigger>
      <DialogContent className='max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('Contract history')}</DialogTitle>
          <DialogDescription>
            {props.contract.name} · {props.contract.contract_no}
          </DialogDescription>
        </DialogHeader>
        <div className='flex flex-col gap-4'>
          <section>
            <h3 className='mb-2 font-medium'>
              {t('Procurement multiplier versions')}
            </h3>
            <ProgressiveList
              isLoading={rates.isLoading}
              isError={rates.isError}
              isEmpty={!rates.data?.items.length}
              hasMore={rates.hasNextPage}
              isLoadingMore={rates.isFetchingNextPage}
              onLoadMore={() => void rates.fetchNextPage()}
            >
              <div className='flex flex-col gap-2'>
                {rates.data?.items.map((rate) => (
                  <div
                    key={rate.id}
                    className='flex justify-between rounded-lg border p-2'
                  >
                    <span>
                      {formatPpmPercent(
                        rate.procurement_multiplier_ppm,
                        t('Unknown')
                      )}{' '}
                      ·{' '}
                      {formatPpmDiscount(
                        rate.procurement_multiplier_ppm,
                        t('discount unit'),
                        t('Unknown')
                      )}
                    </span>
                    <span className='text-muted-foreground'>
                      {formatTime(rate.effective_at)}
                    </span>
                  </div>
                ))}
              </div>
            </ProgressiveList>
          </section>
          <section>
            <h3 className='mb-2 font-medium'>{t('Inventory adjustments')}</h3>
            <ProgressiveList
              isLoading={inventory.isLoading}
              isError={inventory.isError}
              isEmpty={!inventory.data?.items.length}
              hasMore={inventory.hasNextPage}
              isLoadingMore={inventory.isFetchingNextPage}
              onLoadMore={() => void inventory.fetchNextPage()}
            >
              <div className='flex flex-col gap-2'>
                {inventory.data?.items.map((item) => (
                  <div
                    key={item.id}
                    className='flex justify-between rounded-lg border p-2'
                  >
                    <span>
                      {formatMicroUsd(item.delta_micro_usd, t('Unknown'))} ·{' '}
                      {t(item.type)}
                    </span>
                    <span className='text-muted-foreground'>
                      {formatTime(item.created_at)}
                    </span>
                  </div>
                ))}
              </div>
            </ProgressiveList>
          </section>
        </div>
        <DialogFooter showCloseButton />
      </DialogContent>
    </Dialog>
  )
}

export function ContractManagement(props: SupplyChainManagementProps) {
  const { t } = useTranslation()
  const [inactivateTarget, setInactivateTarget] =
    useState<SupplierContract | null>(null)
  const query = useContractAdminList({
    p: props.search.page,
    page_size: props.search.pageSize,
    supplier_id: props.search.supplierId,
    status: props.search.status,
    keyword: props.search.filter || undefined,
  })
  const inactivate = useSupplyChainAdminMutation<number>({
    mutationFn: inactivateContract,
    invalidate: [
      supplyChainQueryKeys.contracts.all(),
      supplyChainQueryKeys.suppliers.all(),
    ],
  })

  async function confirmInactivate(): Promise<void> {
    if (!inactivateTarget) return
    try {
      await inactivate.mutateAsync(inactivateTarget.id)
      toast.success(t('Contract inactivated'))
      setInactivateTarget(null)
    } catch {
      toast.error(t('Unbind every channel before inactivating this contract.'))
    }
  }

  return (
    <div className='flex flex-col gap-3'>
      <ManagementToolbar
        search={props.search}
        onSearchChange={props.onSearchChange}
        actions={<ContractFormDialog onSaved={() => query.refetch()} />}
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
                <TableHead>{t('Contract')}</TableHead>
                <TableHead>{t('Supplier')}</TableHead>
                <TableHead>{t('Rate')}</TableHead>
                <TableHead>{t('Inventory')}</TableHead>
                <TableHead>{t('Channels')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {query.data?.items.map((contract) => (
                <TableRow key={contract.id}>
                  <TableCell>
                    <div className='font-medium'>{contract.name}</div>
                    <div className='text-muted-foreground'>
                      {contract.contract_no}
                    </div>
                  </TableCell>
                  <TableCell>{contract.supplier_name}</TableCell>
                  <TableCell>
                    {formatPpmPercent(
                      contract.current_procurement_multiplier_ppm,
                      t('Unknown')
                    )}
                    <div className='text-muted-foreground'>
                      {formatTime(contract.current_rate_effective_at)}
                    </div>
                  </TableCell>
                  <TableCell>
                    {formatMicroUsd(
                      contract.inventory_total_micro_usd,
                      t('Unknown')
                    )}
                  </TableCell>
                  <TableCell>{contract.linked_channel_count}</TableCell>
                  <TableCell>
                    <StatusBadge status={contract.status} />
                  </TableCell>
                  <TableCell>
                    <div className='flex justify-end gap-1'>
                      <ContractHistoryDialog contract={contract} />
                      <RateVersionDialog
                        contract={contract}
                        onSaved={() => query.refetch()}
                      />
                      <InventoryDialog
                        contract={contract}
                        onSaved={() => query.refetch()}
                      />
                      <ContractFormDialog
                        contract={contract}
                        onSaved={() => query.refetch()}
                      />
                      {contract.status === 'active' ? (
                        <Button
                          size='sm'
                          variant='outline'
                          onClick={() => setInactivateTarget(contract)}
                        >
                          {t('Inactivate')}
                        </Button>
                      ) : (
                        <Badge variant='secondary'>{t('Inactive')}</Badge>
                      )}
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
      <ConfirmAction
        open={inactivateTarget !== null}
        onOpenChange={(next) => {
          if (!next) setInactivateTarget(null)
        }}
        title={t('Inactivate contract')}
        description={t(
          'Status will change from Active to Inactive. Bound channels must be removed first.'
        )}
        confirmLabel={t('Inactivate')}
        pending={inactivate.isPending}
        destructive
        onConfirm={confirmInactivate}
      />
    </div>
  )
}
