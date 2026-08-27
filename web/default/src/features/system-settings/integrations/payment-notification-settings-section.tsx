/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { useEffect, useMemo, useRef } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOptionsBulk } from '../hooks/use-update-option'

const isValidHttpUrl = (value: string) => {
  try {
    const url = new URL(value)
    return url.protocol === 'http:' || url.protocol === 'https:'
  } catch {
    return false
  }
}

const schema = z
  .object({
    payment_notify_setting: z.object({
      dingtalk_alert_enabled: z.boolean(),
      dingtalk_alert_webhook_url: z.string(),
      dingtalk_alert_secret: z.string(),
    }),
  })
  .superRefine((values, ctx) => {
    const webhook = values.payment_notify_setting.dingtalk_alert_webhook_url.trim()
    if (values.payment_notify_setting.dingtalk_alert_enabled && webhook === '') {
      ctx.addIssue({
        code: 'custom',
        path: ['payment_notify_setting', 'dingtalk_alert_webhook_url'],
        message: 'DingTalk webhook URL is required when payment notifications are enabled',
      })
    }
    if (webhook !== '' && !isValidHttpUrl(webhook)) {
      ctx.addIssue({
        code: 'custom',
        path: ['payment_notify_setting', 'dingtalk_alert_webhook_url'],
        message: 'Enter a valid http or https URL',
      })
    }
  })

type FormValues = z.output<typeof schema>
type FormInput = z.input<typeof schema>

type Defaults = {
  'payment_notify_setting.dingtalk_alert_enabled': boolean
  'payment_notify_setting.dingtalk_alert_webhook_url': string
  'payment_notify_setting.dingtalk_alert_secret': string
}

function buildDefaults(defaults: Defaults): FormInput {
  return {
    payment_notify_setting: {
      dingtalk_alert_enabled:
        defaults['payment_notify_setting.dingtalk_alert_enabled'],
      dingtalk_alert_webhook_url:
        defaults['payment_notify_setting.dingtalk_alert_webhook_url'],
      dingtalk_alert_secret:
        defaults['payment_notify_setting.dingtalk_alert_secret'],
    },
  }
}

function normalize(values: FormValues): Defaults {
  return {
    'payment_notify_setting.dingtalk_alert_enabled':
      values.payment_notify_setting.dingtalk_alert_enabled,
    'payment_notify_setting.dingtalk_alert_webhook_url':
      values.payment_notify_setting.dingtalk_alert_webhook_url.trim(),
    'payment_notify_setting.dingtalk_alert_secret':
      values.payment_notify_setting.dingtalk_alert_secret.trim(),
  }
}

export function PaymentNotificationSettingsSection(props: {
  defaultValues: Defaults
}) {
  const { t } = useTranslation()
  const updateOptions = useUpdateOptionsBulk()
  const formDefaults = useMemo(
    () => buildDefaults(props.defaultValues),
    [props.defaultValues]
  )
  const form = useForm<FormInput, unknown, FormValues>({
    resolver: zodResolver(schema),
    defaultValues: formDefaults,
  })
  const baselineRef = useRef<Defaults>(props.defaultValues)
  const baselineSerializedRef = useRef(JSON.stringify(props.defaultValues))

  useEffect(() => {
    const serialized = JSON.stringify(props.defaultValues)
    if (serialized === baselineSerializedRef.current) return
    baselineRef.current = props.defaultValues
    baselineSerializedRef.current = serialized
    form.reset(buildDefaults(props.defaultValues))
  }, [form, props.defaultValues])

  const onSubmit = async (values: FormValues) => {
    const normalized = normalize(values)
    const changedKeys = (
      Object.keys(normalized) as Array<keyof Defaults>
    ).filter(
      (key) =>
        JSON.stringify(normalized[key]) !==
        JSON.stringify(baselineRef.current[key])
    )
    if (changedKeys.length === 0) {
      toast.info(t('No changes to save'))
      return
    }
    const response = await updateOptions.mutateAsync({
      options: changedKeys.map((key) => ({
        key,
        value:
          typeof normalized[key] === 'boolean'
            ? String(normalized[key])
            : normalized[key],
      })),
    })
    if (!response.success) return
    baselineRef.current = normalized
    baselineSerializedRef.current = JSON.stringify(normalized)
    form.reset(buildDefaults(normalized))
  }

  return (
    <Form {...form}>
      <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
        <SettingsPageFormActions
          onSave={form.handleSubmit(onSubmit)}
          isSaving={updateOptions.isPending}
        />
        <SettingsSection title={t('Payment Notifications')}>
          <div data-settings-form-span='full'>
            <h4 className='font-medium'>{t('Dedicated DingTalk robot')}</h4>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t(
                'Send a separate DingTalk message when a payment succeeds. This does not share the monitoring robot.'
              )}
            </p>
          </div>
          <FormField
            control={form.control}
            name='payment_notify_setting.dingtalk_alert_enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable payment DingTalk alerts')}</FormLabel>
                  <FormDescription>
                    {t('Use a dedicated robot for payment success notifications.')}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch checked={field.value} onCheckedChange={field.onChange} />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />
          <FormField
            control={form.control}
            name='payment_notify_setting.dingtalk_alert_webhook_url'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('DingTalk webhook URL')}</FormLabel>
                <FormControl>
                  <Input
                    autoComplete='off'
                    placeholder='https://oapi.dingtalk.com/robot/send?access_token=...'
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t('Paste the robot webhook URL for payment notifications.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='payment_notify_setting.dingtalk_alert_secret'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('DingTalk secret')}</FormLabel>
                <FormControl>
                  <Input autoComplete='off' placeholder={t('Optional')} {...field} />
                </FormControl>
                <FormDescription>
                  {t('Used when your DingTalk robot requires a signing secret.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsSection>
      </SettingsForm>
    </Form>
  )
}
