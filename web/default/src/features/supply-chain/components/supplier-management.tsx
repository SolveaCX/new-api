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
import { createSupplier, inactivateSupplier, updateSupplier } from '../api'
import type { SupplyChainManagementProps } from '../contracts'
import {
  useSupplierAdminList,
  useSupplyChainAdminMutation,
} from '../hooks/use-supply-chain-admin'
import { formatMicroUsd } from '../lib/format'
import { supplierFormSchema, type SupplierFormValues } from '../lib/schemas'
import { supplyChainQueryKeys } from '../query-keys'
import type { UpstreamSupplier } from '../types'
import {
  ConfirmAction,
  ManagementPagination,
  ManagementStatus,
  ManagementToolbar,
  StatusBadge,
} from './management-common'

const EMPTY_FORM: SupplierFormValues = { name: '', remark: '' }

function SupplierFormDialog(props: {
  supplier?: UpstreamSupplier
  onSaved: () => void
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const form = useForm<SupplierFormValues>({
    resolver: zodResolver(supplierFormSchema),
    defaultValues: EMPTY_FORM,
  })
  const mutation = useSupplyChainAdminMutation<SupplierFormValues>({
    mutationFn: async (values) =>
      props.supplier
        ? updateSupplier(props.supplier.id, {
            ...values,
            expected_version: props.supplier.row_version,
          })
        : createSupplier(values),
    invalidate: [supplyChainQueryKeys.suppliers.all()],
  })

  useEffect(() => {
    if (!open) return
    form.reset(
      props.supplier
        ? { name: props.supplier.name, remark: props.supplier.remark }
        : EMPTY_FORM
    )
  }, [form, open, props.supplier])

  function finishSave(): void {
    toast.success(
      props.supplier ? t('Supplier updated') : t('Supplier created')
    )
    setOpen(false)
    props.onSaved()
  }

  async function submit(values: SupplierFormValues): Promise<void> {
    try {
      await mutation.mutateAsync(values)
      finishSave()
    } catch {
      toast.error(t('Unable to save supplier'))
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger
        render={
          <Button
            variant={props.supplier ? 'ghost' : 'default'}
            size={props.supplier ? 'icon-sm' : 'default'}
          />
        }
      >
        <HugeiconsIcon
          icon={props.supplier ? Edit02Icon : PlusSignIcon}
          strokeWidth={2}
        />
        {props.supplier ? (
          <span className='sr-only'>{t('Edit')}</span>
        ) : (
          t('Create supplier')
        )}
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {props.supplier ? t('Edit supplier') : t('Create supplier')}
          </DialogTitle>
          <DialogDescription>
            {t('A supplier can own multiple contracts and channels.')}
          </DialogDescription>
        </DialogHeader>
        <form id='supplier-form' onSubmit={form.handleSubmit(submit)}>
          <FieldGroup>
            <Field data-invalid={Boolean(form.formState.errors.name)}>
              <FieldLabel htmlFor='supplier-name'>{t('Name')}</FieldLabel>
              <Input
                id='supplier-name'
                aria-invalid={Boolean(form.formState.errors.name)}
                {...form.register('name')}
              />
              <FieldError>
                {form.formState.errors.name
                  ? t(form.formState.errors.name.message ?? '')
                  : null}
              </FieldError>
            </Field>
            <Field data-invalid={Boolean(form.formState.errors.remark)}>
              <FieldLabel htmlFor='supplier-remark'>{t('Remark')}</FieldLabel>
              <Textarea
                id='supplier-remark'
                aria-invalid={Boolean(form.formState.errors.remark)}
                {...form.register('remark')}
              />
              <FieldError>
                {form.formState.errors.remark
                  ? t(form.formState.errors.remark.message ?? '')
                  : null}
              </FieldError>
            </Field>
          </FieldGroup>
        </form>
        <DialogFooter showCloseButton>
          <Button
            type='submit'
            form='supplier-form'
            disabled={mutation.isPending}
          >
            {mutation.isPending ? <Spinner /> : null}
            {t('Save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export function SupplierManagement(props: SupplyChainManagementProps) {
  const { t } = useTranslation()
  const [inactivateTarget, setInactivateTarget] =
    useState<UpstreamSupplier | null>(null)
  const params = {
    p: props.search.page,
    page_size: props.search.pageSize,
    status: props.search.status,
    keyword: props.search.filter || undefined,
  }
  const query = useSupplierAdminList(params)
  const inactivate = useSupplyChainAdminMutation<UpstreamSupplier>({
    mutationFn: (supplier) =>
      inactivateSupplier(supplier.id, {
        expected_version: supplier.row_version,
      }),
    invalidate: [
      supplyChainQueryKeys.suppliers.all(),
      supplyChainQueryKeys.contracts.all(),
    ],
  })

  function finishInactivate(): void {
    toast.success(t('Supplier inactivated'))
    setInactivateTarget(null)
    void query.refetch()
  }

  async function confirmInactivate(): Promise<void> {
    if (!inactivateTarget) return
    const target = inactivateTarget
    try {
      await inactivate.mutateAsync(target)
      finishInactivate()
    } catch {
      toast.error(
        t(
          'Unbind channels and inactivate every contract before inactivating this supplier.'
        )
      )
    }
  }

  return (
    <div className='flex flex-col gap-3'>
      <ManagementToolbar
        search={props.search}
        onSearchChange={props.onSearchChange}
        actions={<SupplierFormDialog onSaved={() => query.refetch()} />}
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
                <TableHead>{t('Supplier')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Contracts')}</TableHead>
                <TableHead>{t('Channels')}</TableHead>
                <TableHead>{t('Inventory')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {query.data?.items.map((supplier) => (
                <TableRow key={supplier.id}>
                  <TableCell>
                    <div className='font-medium'>{supplier.name}</div>
                    {supplier.remark ? (
                      <div className='text-muted-foreground max-w-64 truncate'>
                        {supplier.remark}
                      </div>
                    ) : null}
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={supplier.status} />
                  </TableCell>
                  <TableCell>
                    {supplier.active_contract_count}/{supplier.contract_count}
                  </TableCell>
                  <TableCell>{supplier.linked_channel_count}</TableCell>
                  <TableCell>
                    {formatMicroUsd(
                      supplier.inventory_total_micro_usd,
                      t('Unknown')
                    )}
                  </TableCell>
                  <TableCell>
                    <div className='flex justify-end gap-1'>
                      <SupplierFormDialog
                        supplier={supplier}
                        onSaved={() => query.refetch()}
                      />
                      {supplier.status === 'active' ? (
                        <Button
                          type='button'
                          size='sm'
                          variant='outline'
                          onClick={() => setInactivateTarget(supplier)}
                        >
                          {t('Inactivate')}
                        </Button>
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
      <ConfirmAction
        open={inactivateTarget !== null}
        onOpenChange={(open) => {
          if (!open) setInactivateTarget(null)
        }}
        title={t('Inactivate supplier')}
        description={t(
          'Status will change from Active to Inactive. This is only allowed after all contracts are inactive and channels are unbound.'
        )}
        confirmLabel={t('Inactivate')}
        pending={inactivate.isPending}
        destructive
        onConfirm={confirmInactivate}
      />
    </div>
  )
}
