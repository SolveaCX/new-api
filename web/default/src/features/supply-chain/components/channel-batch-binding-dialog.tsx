/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useState } from 'react'
import { AxiosError } from 'axios'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQueryClient } from '@tanstack/react-query'
import { Link01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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
import { Spinner } from '@/components/ui/spinner'
import { bindChannel } from '../api'
import { useContractAdminInfiniteList } from '../hooks/use-supply-chain-admin'
import {
  coordinateChannelBindings,
  type ChannelBatchBindingResult,
} from '../lib/channel-batch-binding'
import {
  channelBindingFormSchema,
  type ChannelBindingFormValues,
} from '../lib/schemas'
import { supplyChainQueryKeys } from '../query-keys'
import type { SupplierChannelBinding } from '../types'
import { ProgressiveList } from './progressive-list'

function batchBindingErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof AxiosError) {
    const message = error.response?.data?.message
    if (typeof message === 'string' && message.trim()) return message
  }
  return fallback
}

export function ChannelBatchBindingDialog(props: {
  bindings: readonly SupplierChannelBinding[]
  policyConfigurable: boolean
  policyActive: boolean
  policyStatusAvailable: boolean
  onFinished: (failedChannelIds: number[]) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [result, setResult] = useState<ChannelBatchBindingResult>()
  const contracts = useContractAdminInfiniteList(
    { page_size: 50, status: 'active' },
    open
  )
  const form = useForm<ChannelBindingFormValues>({
    resolver: zodResolver(channelBindingFormSchema),
    defaultValues: {
      contract_id: 0,
      skip_internal_accounting: false,
    },
  })

  function handleOpenChange(nextOpen: boolean): void {
    if (isSubmitting) return
    setOpen(nextOpen)
    if (nextOpen) {
      setResult(undefined)
      form.reset({ contract_id: 0, skip_internal_accounting: false })
    }
  }

  async function submit(values: ChannelBindingFormValues): Promise<void> {
    setIsSubmitting(true)
    setResult(undefined)
    try {
      const nextResult = await coordinateChannelBindings({
        bindings: props.bindings,
        contractId: values.contract_id,
        skipInternalAccounting: values.skip_internal_accounting,
        request: bindChannel,
      })
      setResult(nextResult)
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: supplyChainQueryKeys.channelBindings.all(),
        }),
        queryClient.invalidateQueries({
          queryKey: supplyChainQueryKeys.contracts.all(),
        }),
        queryClient.invalidateQueries({
          queryKey: supplyChainQueryKeys.suppliers.all(),
        }),
      ])

      const failedIds = nextResult.failed.map(
        (failure) => failure.binding.channel_id
      )
      props.onFinished(failedIds)
      if (nextResult.failed.length === 0) {
        toast.success(
          t('Bound {{count}} channels', { count: nextResult.succeeded.length })
        )
        setOpen(false)
        return
      }
      if (nextResult.succeeded.length > 0) {
        toast.warning(
          t(
            'Bound {{successCount}} channels; {{failureCount}} failed and remain selected.',
            {
              successCount: nextResult.succeeded.length,
              failureCount: nextResult.failed.length,
            }
          )
        )
        return
      }
      toast.error(
        t('Failed to bind {{count}} channels. Review the errors and retry.', {
          count: nextResult.failed.length,
        })
      )
    } finally {
      setIsSubmitting(false)
    }
  }

  const skipInternalAccounting = form.watch('skip_internal_accounting')
  let policyDescription = t(
    'Internal accounts record procurement cost and inventory consumption, but are excluded from sales and profit.'
  )
  if (skipInternalAccounting) {
    if (!props.policyStatusAvailable) {
      policyDescription = t(
        'The saved skip setting cannot be verified because the global policy status is unavailable.'
      )
    } else if (props.policyActive) {
      policyDescription = t(
        'Internal accounts will not create supplier accounting facts on this channel. Business accounts remain fully accounted.'
      )
    } else {
      policyDescription = t(
        'The channel is configured to skip, but internal costs continue to be recorded until the global policy becomes active.'
      )
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger
        render={
          <Button
            type='button'
            size='sm'
            disabled={props.bindings.length === 0}
          />
        }
      >
        <HugeiconsIcon icon={Link01Icon} strokeWidth={2} />
        {t('Batch bind')}
      </DialogTrigger>
      <DialogContent showCloseButton={!isSubmitting}>
        <DialogHeader>
          <DialogTitle>
            {t('Batch bind {{count}} channels', {
              count: props.bindings.length,
            })}
          </DialogTitle>
          <DialogDescription>
            {t(
              'Apply one contract and internal account handling policy to the selected channels.'
            )}{' '}
            {t(
              'Selected channels may already be bound. Each channel is updated independently using its current binding version.'
            )}
          </DialogDescription>
        </DialogHeader>
        <form
          id='channel-batch-binding-form'
          onSubmit={form.handleSubmit(submit)}
        >
          <FieldGroup>
            <Field
              data-invalid={Boolean(form.formState.errors.contract_id)}
              data-disabled={isSubmitting || undefined}
            >
              <FieldLabel htmlFor='channel-batch-binding-contract'>
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
                  id='channel-batch-binding-contract'
                  className='w-full'
                  aria-invalid={Boolean(form.formState.errors.contract_id)}
                  value={form.watch('contract_id') || ''}
                  disabled={isSubmitting}
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
              data-disabled={isSubmitting || undefined}
            >
              <FieldLabel htmlFor='channel-batch-binding-policy'>
                {t('Internal request accounting')}
              </FieldLabel>
              <NativeSelect
                id='channel-batch-binding-policy'
                className='w-full'
                aria-invalid={Boolean(
                  form.formState.errors.skip_internal_accounting
                )}
                value={skipInternalAccounting ? 'skip' : 'record'}
                disabled={isSubmitting}
                onChange={(event) =>
                  form.setValue(
                    'skip_internal_accounting',
                    event.target.value === 'skip',
                    { shouldValidate: true }
                  )
                }
              >
                <NativeSelectOption value='record'>
                  {t('Record internal procurement cost and inventory')}
                </NativeSelectOption>
                <NativeSelectOption
                  value='skip'
                  disabled={!props.policyConfigurable}
                >
                  {t('Do not create internal supplier accounting facts')}
                </NativeSelectOption>
              </NativeSelect>
              <FieldDescription>{policyDescription}</FieldDescription>
              <FieldError>
                {form.formState.errors.skip_internal_accounting
                  ? t(
                      form.formState.errors.skip_internal_accounting.message ??
                        ''
                    )
                  : null}
              </FieldError>
            </Field>
            {skipInternalAccounting ? (
              <Alert variant='destructive'>
                <AlertTitle>
                  {t('Internal supplier accounting facts will not be created')}
                </AlertTitle>
                <AlertDescription>
                  {t(
                    'Supplier accounting facts will not be created, so internal procurement costs and inventory consumption cannot be recovered later.'
                  )}
                </AlertDescription>
              </Alert>
            ) : null}
            {result?.failed.length ? (
              <Alert variant='destructive'>
                <AlertTitle>{t('Channels that failed to bind')}</AlertTitle>
                <AlertDescription>
                  <ul className='list-disc pl-5'>
                    {result.failed.map((failure) => (
                      <li key={failure.binding.channel_id}>
                        {failure.binding.channel_name} (#
                        {failure.binding.channel_id}
                        ):{' '}
                        {batchBindingErrorMessage(
                          failure.error,
                          t('Unable to update channel binding')
                        )}
                      </li>
                    ))}
                  </ul>
                </AlertDescription>
              </Alert>
            ) : null}
          </FieldGroup>
        </form>
        <DialogFooter showCloseButton={!isSubmitting}>
          <Button
            type='submit'
            form='channel-batch-binding-form'
            disabled={isSubmitting || props.bindings.length === 0}
          >
            {isSubmitting ? <Spinner /> : null}
            {t('Bind selected channels')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
