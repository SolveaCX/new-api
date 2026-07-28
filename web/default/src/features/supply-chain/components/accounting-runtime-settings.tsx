/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useEffect, useState } from 'react'
import { zodResolver } from '@hookform/resolvers/zod'
import { AxiosError } from 'axios'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Spinner } from '@/components/ui/spinner'
import {
  useAccountingRuntimeSettings,
  useUpdateAccountingRuntimeSettings,
} from '../hooks/use-accounting-runtime-settings'
import type {
  SupplierAccountingRuntimeSettings,
  SupplierAccountingRuntimeSettingsRequest,
} from '../types'
import { ConfirmAction } from './management-common'

const runtimeSettingsSchema = z.object({
  cutoverDate: z
    .string()
    .refine((value) => value === '' || /^\d{4}-\d{2}-\d{2}$/.test(value), {
      message: 'Enter a valid cutover date',
    }),
  retentionDays: z
    .number({ message: 'Enter a valid retention period' })
    .int('Retention days must be a whole number')
    .min(0, 'Retention days cannot be negative')
    .max(36500, 'Retention days cannot exceed 36500'),
})

type RuntimeSettingsForm = z.infer<typeof runtimeSettingsSchema>

function shanghaiDate(timestamp: number): string {
  if (timestamp <= 0) return ''

  const parts = new Intl.DateTimeFormat('en', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).formatToParts(timestamp * 1000)
  const value = (type: Intl.DateTimeFormatPartTypes) =>
    parts.find((part) => part.type === type)?.value ?? ''
  return `${value('year')}-${value('month')}-${value('day')}`
}

function shanghaiMidnight(date: string): number {
  if (!date) return 0
  return Math.floor(Date.parse(`${date}T00:00:00+08:00`) / 1000)
}

function runtimeSettingsError(error: unknown, fallback: string): string {
  if (error instanceof AxiosError) {
    const message = error.response?.data?.message
    if (typeof message === 'string' && message.trim()) return message
  }
  return fallback
}

function sourceLabel(
  source: SupplierAccountingRuntimeSettings['source'],
  t: (key: string) => string
): string {
  if (source === 'database') return t('Page configuration')
  if (source === 'environment') return t('Legacy environment variable')
  return t('Default value')
}

export function AccountingRuntimeSettings() {
  const { t } = useTranslation()
  const query = useAccountingRuntimeSettings()
  const mutation = useUpdateAccountingRuntimeSettings()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [pendingRequest, setPendingRequest] =
    useState<SupplierAccountingRuntimeSettingsRequest | null>(null)
  const form = useForm<RuntimeSettingsForm>({
    resolver: zodResolver(runtimeSettingsSchema),
    defaultValues: { cutoverDate: '', retentionDays: 0 },
  })

  useEffect(() => {
    if (!query.data) return
    form.reset({
      cutoverDate: shanghaiDate(query.data.cutover_at),
      retentionDays: query.data.retention_days,
    })
  }, [form, query.data])

  if (query.isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t('Supplier accounting runtime settings')}</CardTitle>
          <CardDescription>
            {t('Configure authoritative accounting and fact retention.')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-3'>
          <Skeleton className='h-8 w-40' />
          <Skeleton className='h-24 w-full' />
        </CardContent>
      </Card>
    )
  }

  if (query.isError || !query.data) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t('Supplier accounting runtime settings')}</CardTitle>
        </CardHeader>
        <CardContent>
          <Alert variant='destructive'>
            <AlertTitle>{t('Unable to load runtime settings')}</AlertTitle>
            <AlertDescription>
              {t('Authoritative accounting settings were not changed.')}
            </AlertDescription>
          </Alert>
        </CardContent>
      </Card>
    )
  }

  const settings = query.data
  const cutoverActive = settings.cutover_at > 0 && settings.cutover_locked
  const cutoverPending = settings.cutover_at > 0 && !settings.cutover_locked
  const statusLabel = cutoverActive
    ? t('Active')
    : cutoverPending
      ? t('Scheduled')
      : t('Not configured')

  function prepareSave(values: RuntimeSettingsForm): void {
    setPendingRequest({
      expected_revision: settings.revision,
      cutover_at: shanghaiMidnight(values.cutoverDate),
      retention_days: values.retentionDays,
    })
    setConfirmOpen(true)
  }

  async function confirmSave(): Promise<void> {
    if (!pendingRequest) return
    try {
      await mutation.mutateAsync(pendingRequest)
      toast.success(t('Runtime settings saved'))
      setConfirmOpen(false)
      setPendingRequest(null)
    } catch (error) {
      toast.error(
        runtimeSettingsError(error, t('Unable to save runtime settings'))
      )
    }
  }

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>{t('Supplier accounting runtime settings')}</CardTitle>
          <CardDescription>
            {t('Configure authoritative accounting and fact retention.')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-5'>
          <div className='flex flex-wrap items-center gap-2'>
            <Badge variant={cutoverActive ? 'default' : 'secondary'}>
              {statusLabel}
            </Badge>
            <Badge variant='outline'>
              {t('Source: {{source}}', {
                source: sourceLabel(settings.source, t),
              })}
            </Badge>
          </div>
          {settings.source === 'environment' ? (
            <Alert>
              <AlertTitle>{t('Legacy configuration detected')}</AlertTitle>
              <AlertDescription>
                {t(
                  'The current values come from environment variables. Saving this form migrates them to shared page configuration.'
                )}
              </AlertDescription>
            </Alert>
          ) : null}
          <form
            id='supplier-accounting-runtime-settings-form'
            onSubmit={form.handleSubmit(prepareSave)}
          >
            <FieldGroup>
              <Field
                data-invalid={Boolean(form.formState.errors.cutoverDate)}
              >
                <FieldLabel htmlFor='supplier-accounting-cutover-date'>
                  {t('Authoritative accounting start date')}
                </FieldLabel>
                <Input
                  id='supplier-accounting-cutover-date'
                  type='date'
                  disabled={settings.cutover_locked || mutation.isPending}
                  aria-invalid={Boolean(form.formState.errors.cutoverDate)}
                  {...form.register('cutoverDate')}
                />
                <FieldDescription>
                  {t(
                    'Facts are recorded from 00:00 Asia/Shanghai on this date. The first authoritative daily report is generated after 02:00 the next day.'
                  )}
                </FieldDescription>
                {settings.cutover_locked ? (
                  <FieldDescription>
                    {t(
                      'This date is locked because authoritative accounting is already active.'
                    )}
                  </FieldDescription>
                ) : null}
                <FieldError>
                  {form.formState.errors.cutoverDate
                    ? t(form.formState.errors.cutoverDate.message ?? '')
                    : null}
                </FieldError>
              </Field>
              <Field
                data-invalid={Boolean(form.formState.errors.retentionDays)}
              >
                <FieldLabel htmlFor='supplier-accounting-retention-days'>
                  {t('Fact retention days')}
                </FieldLabel>
                <Input
                  id='supplier-accounting-retention-days'
                  type='number'
                  min={0}
                  max={36500}
                  disabled={mutation.isPending}
                  aria-invalid={Boolean(form.formState.errors.retentionDays)}
                  {...form.register('retentionDays', { valueAsNumber: true })}
                />
                <FieldDescription>
                  {t(
                    'Enter 0 to keep facts permanently. A positive value deletes facts only after the corresponding daily report is safely published.'
                  )}
                </FieldDescription>
                <FieldError>
                  {form.formState.errors.retentionDays
                    ? t(form.formState.errors.retentionDays.message ?? '')
                    : null}
                </FieldError>
              </Field>
            </FieldGroup>
          </form>
          <Alert
            variant={
              form.watch('retentionDays') > 0 ? 'destructive' : 'default'
            }
          >
            <AlertTitle>{t('Historical boundary')}</AlertTitle>
            <AlertDescription>
              {t(
                'Dates before the authoritative start date are available only through published historical estimates.'
              )}
            </AlertDescription>
          </Alert>
        </CardContent>
        <CardFooter className='justify-end'>
          <Button
            type='submit'
            form='supplier-accounting-runtime-settings-form'
            disabled={mutation.isPending || !form.formState.isDirty}
          >
            {mutation.isPending ? <Spinner /> : null}
            {t('Save runtime settings')}
          </Button>
        </CardFooter>
      </Card>
      <ConfirmAction
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t('Confirm runtime settings')}
        description={t(
          'These values are shared by all Console and Router nodes. Enabling retention may permanently delete published fact rows.'
        )}
        confirmLabel={t('Save settings')}
        pending={mutation.isPending}
        destructive={Boolean(pendingRequest && pendingRequest.retention_days > 0)}
        onConfirm={() => void confirmSave()}
      />
    </>
  )
}
