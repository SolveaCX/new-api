/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useMemo } from 'react'
import { zodResolver } from '@hookform/resolvers/zod'
import type { Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
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
import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOptionsBulk } from '../hooks/use-update-option'
import {
  buildPLGCatalogNotifyDefaults,
  buildPLGCatalogNotifyOptionUpdates,
  plgCatalogNotifySchema,
  type PLGCatalogNotifyDefaults,
  type PLGCatalogNotifyFormValues,
} from './plg-catalog-notify-settings-utils'

export function PLGCatalogNotificationSettingsSection(props: {
  defaultValues: PLGCatalogNotifyDefaults
}) {
  const { t } = useTranslation()
  const updateOptions = useUpdateOptionsBulk()
  const defaultValues = useMemo(
    () => buildPLGCatalogNotifyDefaults(props.defaultValues),
    [props.defaultValues]
  )

  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
    useSettingsForm<PLGCatalogNotifyFormValues>({
      resolver: zodResolver(plgCatalogNotifySchema) as Resolver<
        PLGCatalogNotifyFormValues,
        unknown,
        PLGCatalogNotifyFormValues
      >,
      defaultValues,
      onSubmit: async (values, changedFields) => {
        const options = buildPLGCatalogNotifyOptionUpdates(values, changedFields)
        if (options.length === 0) return
        await updateOptions.mutateAsync({ options })
      },
    })

  return (
    <>
      <FormNavigationGuard when={isDirty} />

      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsPageFormActions
            onSave={handleSubmit}
            onReset={handleReset}
            isSaving={updateOptions.isPending || isSubmitting}
            isResetDisabled={!isDirty}
          />
          <FormDirtyIndicator isDirty={isDirty} />
          <SettingsSection title={t('PLG Catalog Notifications')}>
            <div data-settings-form-span='full'>
              <h4 className='font-medium'>{t('Dedicated DingTalk robot')}</h4>
              <p className='text-muted-foreground mt-1 text-xs'>
                {t(
                  'Send a DingTalk message when the models shown on the PLG Available Models page are added or removed. This does not share the monitoring robot.'
                )}
              </p>
            </div>
            <FormField
              control={form.control}
              name='plg_catalog_notify_setting.dingtalk_alert_enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Enable PLG catalog DingTalk alerts')}</FormLabel>
                    <FormDescription>
                      {t(
                        'The catalog is still tracked while disabled, so enabling later does not replay old changes.'
                      )}
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
              name='plg_catalog_notify_setting.dingtalk_alert_webhook_url'
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
                    {t('Paste the robot webhook URL for PLG catalog notifications.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='plg_catalog_notify_setting.dingtalk_alert_secret'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('DingTalk secret')}</FormLabel>
                  <FormControl>
                    <Input autoComplete='off' placeholder={t('Optional')} {...field} />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Saved DingTalk secrets are not shown. Enter a new signing secret to update it, or leave blank to keep the current one.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='plg_catalog_notify_setting.check_interval_minutes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Check interval (minutes)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      step={1}
                      inputMode='numeric'
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'How often the PLG catalog is compared with the last snapshot. Minimum 1 minute.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </SettingsSection>
        </SettingsForm>
      </Form>
    </>
  )
}
