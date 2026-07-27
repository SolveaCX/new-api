/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useState } from 'react'
import { AxiosError } from 'axios'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Field, FieldDescription, FieldLabel } from '@/components/ui/field'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { useUpdateAccountingPolicyCapability } from '../hooks/use-accounting-policy-capability'
import type { SupplierAccountingPolicyCapability } from '../types'
import { ConfirmAction } from './management-common'

function policyErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof AxiosError) {
    const message = error.response?.data?.message
    if (typeof message === 'string' && message.trim()) return message
  }
  return fallback
}

function effectiveTime(timestamp: number, language: string): string {
  return new Intl.DateTimeFormat(language, {
    dateStyle: 'medium',
    timeStyle: 'medium',
  }).format(timestamp * 1000)
}

function capabilityStatus(
  capability: SupplierAccountingPolicyCapability
): 'active' | 'inactive' | 'activating' | 'deactivating' {
  if (capability.activated !== capability.active) {
    return capability.activated ? 'activating' : 'deactivating'
  }
  return capability.active ? 'active' : 'inactive'
}

export function AccountingPolicyActivation(props: {
  capability?: SupplierAccountingPolicyCapability
  isLoading: boolean
  isError: boolean
}) {
  const { t, i18n } = useTranslation()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [requestedActivation, setRequestedActivation] = useState(false)
  const mutation = useUpdateAccountingPolicyCapability()

  if (props.isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t('Internal request skip policy')}</CardTitle>
          <CardDescription>
            {t(
              'Manual safety gate for omitting supplier accounting facts from excluded accounts.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className='flex flex-col gap-3'>
          <Skeleton className='h-5 w-28' />
          <Skeleton className='h-16 w-full' />
        </CardContent>
      </Card>
    )
  }

  if (props.isError || !props.capability) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t('Internal request skip policy')}</CardTitle>
          <CardDescription>
            {t(
              'Manual safety gate for omitting supplier accounting facts from excluded accounts.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Alert variant='destructive'>
            <AlertTitle>{t('Unable to load accounting policy')}</AlertTitle>
            <AlertDescription>
              {t(
                'The activation state is unavailable, so complete skip configuration is disabled.'
              )}
            </AlertDescription>
          </Alert>
        </CardContent>
      </Card>
    )
  }

  const capability = props.capability
  const status = capabilityStatus(capability)
  const pending = status === 'activating' || status === 'deactivating'
  let statusLabel = t('Inactive')
  let statusVariant: 'default' | 'outline' | 'secondary' = 'secondary'
  if (status === 'activating') {
    statusLabel = t('Activation pending')
    statusVariant = 'outline'
  } else if (status === 'deactivating') {
    statusLabel = t('Deactivation pending')
    statusVariant = 'outline'
  } else if (status === 'active') {
    statusLabel = t('Active')
    statusVariant = 'default'
  }
  let statusDescription = t(
    'The global guard is off. Channel bindings cannot enable complete skip.'
  )
  if (status === 'active') {
    statusDescription = t(
      'Excluded-account requests can be omitted on channels configured for complete skip.'
    )
  } else if (status === 'activating') {
    statusDescription = t(
      'Activation is scheduled for {{time}}. Routers keep the previous behavior until then.',
      { time: effectiveTime(capability.effective_at, i18n.language) }
    )
  } else if (status === 'deactivating') {
    statusDescription = t(
      'Deactivation is scheduled for {{time}}. Routers keep the previous behavior until then.',
      { time: effectiveTime(capability.effective_at, i18n.language) }
    )
  }

  async function confirmChange(): Promise<void> {
    try {
      await mutation.mutateAsync(requestedActivation)
      toast.success(
        requestedActivation
          ? t('Policy activation scheduled')
          : t('Policy deactivation scheduled')
      )
      setConfirmOpen(false)
    } catch (error) {
      toast.error(
        policyErrorMessage(error, t('Unable to update accounting policy'))
      )
    }
  }

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>{t('Internal request skip policy')}</CardTitle>
          <CardDescription>
            {t(
              'Manual safety gate for omitting supplier accounting facts from excluded accounts.'
            )}
          </CardDescription>
          <CardAction>
            <Field orientation='horizontal' data-disabled={mutation.isPending}>
              <FieldLabel htmlFor='supplier-accounting-policy-activation'>
                {t('Activate skip policy')}
              </FieldLabel>
              <Switch
                id='supplier-accounting-policy-activation'
                checked={capability.activated}
                disabled={mutation.isPending}
                onCheckedChange={(checked) => {
                  setRequestedActivation(checked)
                  setConfirmOpen(true)
                }}
              />
            </Field>
          </CardAction>
        </CardHeader>
        <CardContent className='flex flex-col gap-3'>
          <div className='flex flex-wrap items-center gap-2'>
            <Badge variant={statusVariant}>{statusLabel}</Badge>
            {pending ? (
              <span className='text-muted-foreground text-sm'>
                {t('Effective at {{time}}', {
                  time: effectiveTime(capability.effective_at, i18n.language),
                })}
              </span>
            ) : null}
          </div>
          <Alert variant={capability.activated ? 'destructive' : 'default'}>
            <AlertTitle>
              {t(
                'This switch is a manual deployment confirmation, not an automatic Router health check.'
              )}
            </AlertTitle>
            <AlertDescription>{statusDescription}</AlertDescription>
          </Alert>
        </CardContent>
        <CardFooter>
          <FieldDescription>
            {t(
              'The change is written to shared configuration and takes effect after the propagation delay shown above.'
            )}
          </FieldDescription>
        </CardFooter>
      </Card>
      <ConfirmAction
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={
          requestedActivation
            ? t('Confirm policy activation')
            : t('Confirm policy deactivation')
        }
        description={
          requestedActivation
            ? t(
                'Confirm that every Router node has been upgraded before scheduling activation. After the delay, excluded-account requests on configured channels will no longer create supplier accounting facts.'
              )
            : t(
                'Schedule deactivation of internal request skipping. Existing channel settings remain stored but stop taking effect after the delay.'
              )
        }
        confirmLabel={
          requestedActivation ? t('Activate policy') : t('Deactivate policy')
        }
        pending={mutation.isPending}
        destructive={requestedActivation}
        onConfirm={() => void confirmChange()}
      />
    </>
  )
}
