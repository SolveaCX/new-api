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
import { useEffect, useMemo } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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
import { MultiSelect, type Option } from '@/components/multi-select'
import { StatusBadge } from '@/components/status-badge'
import { getModels } from '@/features/models/api'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOptionsBulk } from '../hooks/use-update-option'
import {
  AUTO_MODEL_ROUTES,
  autoModelFormSchema,
  buildAutoModelOptions,
  parseAutoModelConfig,
  type AutoModelFormValues,
  type AutoModelRoute,
} from './auto-model-settings'

type AutoModelSettingsCardProps = {
  config: string
}

function AutoModelErrorMessage(props: { message?: string }) {
  const { t } = useTranslation()
  if (!props.message) return null

  const messages: Record<string, string> = {
    'Classifier Base URL must use HTTPS': t(
      'Classifier Base URL must use HTTPS'
    ),
    'Classifier model is required': t('Classifier model is required'),
    'Classifier API key is required when enabling Auto Model': t(
      'Classifier API key is required when enabling Auto Model'
    ),
    'Each route must contain at least one model': t(
      'Each route must contain at least one model'
    ),
    'Configure 5 to 10 unique candidate models': t(
      'Configure 5 to 10 unique candidate models'
    ),
    'Default model must be one of the candidate models': t(
      'Default model must be one of the candidate models'
    ),
    'The virtual auto model cannot be a candidate': t(
      'The virtual auto model cannot be a candidate'
    ),
    'Classifier timeout must be between 200 and 2000 ms': t(
      'Classifier timeout must be between 200 and 2000 ms'
    ),
    'Maximum classifier input characters must be between 1000 and 32000': t(
      'Maximum classifier input characters must be between 1000 and 32000'
    ),
  }

  return <FormMessage>{messages[props.message] ?? props.message}</FormMessage>
}

function getRouteLabel(route: AutoModelRoute, t: (key: string) => string) {
  switch (route) {
    case 'general':
      return t('General tasks')
    case 'coding':
      return t('Coding tasks')
    case 'reasoning':
      return t('Reasoning tasks')
    case 'translation':
      return t('Translation tasks')
  }
}

export function AutoModelSettingsCard(props: AutoModelSettingsCardProps) {
  const { t } = useTranslation()
  const updateOptions = useUpdateOptionsBulk()
  const defaultConfig = useMemo(
    () => parseAutoModelConfig(props.config),
    [props.config]
  )
  const form = useForm<AutoModelFormValues>({
    resolver: zodResolver(autoModelFormSchema),
    defaultValues: {
      ...defaultConfig,
      classifier_api_key: '',
      credential_configured:
        (defaultConfig.credential_version ?? '').trim().length > 0,
    },
  })
  const modelsQuery = useQuery({
    queryKey: ['models', 'auto-model-candidates'],
    queryFn: () => getModels({ p: 1, page_size: 1000, status: '1' }),
    staleTime: 5 * 60 * 1000,
  })

  useEffect(() => {
    form.reset({
      ...defaultConfig,
      classifier_api_key: '',
      credential_configured:
        (defaultConfig.credential_version ?? '').trim().length > 0,
    })
  }, [defaultConfig, form])

  const modelOptions = useMemo<Option[]>(() => {
    const names =
      modelsQuery.data?.data?.items
        .map((model) => model.model_name.trim())
        .filter((model) => model && model !== 'auto') ?? []
    return Array.from(new Set(names))
      .sort((left, right) => left.localeCompare(right))
      .map((model) => ({ label: model, value: model }))
  }, [modelsQuery.data?.data?.items])

  const routes = form.watch('routes')
  const credentialConfigured = form.watch('credential_configured')
  const uniqueCandidateCount = useMemo(
    () => new Set(AUTO_MODEL_ROUTES.flatMap((route) => routes[route])).size,
    [routes]
  )
  const onSubmit = async (values: AutoModelFormValues) => {
    await updateOptions.mutateAsync({
      options: buildAutoModelOptions(values),
    })
    form.reset({
      ...values,
      classifier_api_key: '',
      credential_configured:
        values.credential_configured ||
        values.classifier_api_key.trim().length > 0,
    })
  }

  return (
    <SettingsSection title={t('Auto Model')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOptions.isPending}
            isSaveDisabled={!form.formState.isDirty}
            saveLabel={t('Save Auto Model settings')}
          />

          <Alert>
            <AlertTitle>{t('Routes text requests before relay')}</AlertTitle>
            <AlertDescription>
              {t(
                'When enabled, prompts are sent to the configured classifier to choose a route. Only eligible real models can be selected.'
              )}
            </AlertDescription>
          </Alert>

          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable Auto Model')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Expose the virtual auto model and classify supported text requests.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='classifier_base_url'
            render={({ field, fieldState }) => (
              <FormItem>
                <FormLabel>{t('Classifier Base URL')}</FormLabel>
                <FormControl>
                  <Input placeholder='https://example.com/v1' {...field} />
                </FormControl>
                <FormDescription>
                  {t('Use an HTTPS OpenAI-compatible API endpoint.')}
                </FormDescription>
                <AutoModelErrorMessage message={fieldState.error?.message} />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='classifier_model'
            render={({ field, fieldState }) => (
              <FormItem>
                <FormLabel>{t('Classifier model')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder={t('Enter classifier model name')}
                    {...field}
                  />
                </FormControl>
                <AutoModelErrorMessage message={fieldState.error?.message} />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='classifier_api_key'
            render={({ field, fieldState }) => (
              <FormItem>
                <div className='flex flex-wrap items-center gap-2'>
                  <FormLabel>{t('Classifier API key')}</FormLabel>
                  <StatusBadge
                    variant={credentialConfigured ? 'success' : 'neutral'}
                    label={
                      credentialConfigured
                        ? t('Configured')
                        : t('Not configured')
                    }
                    copyable={false}
                  />
                </div>
                <FormControl>
                  <Input
                    type='password'
                    autoComplete='new-password'
                    placeholder={t(
                      'Enter a new API key, or leave blank to keep current'
                    )}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Saved classifier API keys are never shown. Enter a new value only when rotating the key.'
                  )}
                </FormDescription>
                <AutoModelErrorMessage message={fieldState.error?.message} />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='classifier_timeout_ms'
            render={({ field, fieldState }) => (
              <FormItem>
                <FormLabel>{t('Classifier timeout (ms)')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={200}
                    max={2000}
                    step={100}
                    value={field.value}
                    onChange={(event) =>
                      field.onChange(event.target.valueAsNumber)
                    }
                  />
                </FormControl>
                <FormDescription>
                  {t('Allowed range: 200–2000 ms.')}
                </FormDescription>
                <AutoModelErrorMessage message={fieldState.error?.message} />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='classifier_input_max_chars'
            render={({ field, fieldState }) => (
              <FormItem>
                <FormLabel>
                  {t('Maximum classifier input characters')}
                </FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1000}
                    max={32000}
                    step={100}
                    value={field.value}
                    onChange={(event) =>
                      field.onChange(event.target.valueAsNumber)
                    }
                  />
                </FormControl>
                <FormDescription>
                  {t('Longer prompt text is truncated before classification.')}
                </FormDescription>
                <AutoModelErrorMessage message={fieldState.error?.message} />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='default_model'
            render={({ field, fieldState }) => (
              <FormItem>
                <FormLabel>{t('Default fallback model')}</FormLabel>
                <FormControl>
                  <Input
                    list='auto-model-candidates'
                    placeholder={t('Select or enter a candidate model')}
                    {...field}
                  />
                </FormControl>
                <datalist id='auto-model-candidates'>
                  {modelOptions.map((option) => (
                    <option key={option.value} value={option.value} />
                  ))}
                </datalist>
                <FormDescription>
                  {t(
                    'Used when classification fails. It must appear in the candidate lists.'
                  )}
                </FormDescription>
                <AutoModelErrorMessage message={fieldState.error?.message} />
              </FormItem>
            )}
          />

          <div className='flex flex-col gap-5 lg:col-span-2'>
            <div className='flex flex-wrap items-center justify-between gap-2'>
              <div className='flex flex-col gap-1'>
                <h3 className='text-sm font-medium'>
                  {t('Ordered route candidates')}
                </h3>
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'Earlier models have higher priority. Configure 5–10 unique models across all routes.'
                  )}
                </p>
              </div>
              <StatusBadge
                variant={
                  uniqueCandidateCount >= 5 && uniqueCandidateCount <= 10
                    ? 'success'
                    : 'warning'
                }
                label={t('{{count}} unique candidates', {
                  count: uniqueCandidateCount,
                })}
                copyable={false}
              />
            </div>

            <div className='grid gap-5 lg:grid-cols-2'>
              {AUTO_MODEL_ROUTES.map((route) => (
                <FormField
                  key={route}
                  control={form.control}
                  name={`routes.${route}`}
                  render={({ field, fieldState }) => (
                    <FormItem>
                      <FormLabel>{getRouteLabel(route, t)}</FormLabel>
                      <FormControl>
                        <MultiSelect
                          options={modelOptions}
                          selected={field.value}
                          onChange={field.onChange}
                          placeholder={t(
                            'Select or add models in priority order'
                          )}
                          allowCreate
                          createLabel='Add custom model "{{value}}"'
                          emptyText={t('No matching models')}
                        />
                      </FormControl>
                      <FormDescription>
                        {t('The first eligible model is selected.')}
                      </FormDescription>
                      <AutoModelErrorMessage
                        message={fieldState.error?.message}
                      />
                    </FormItem>
                  )}
                />
              ))}
            </div>
            <AutoModelErrorMessage
              message={form.formState.errors.routes?.message}
            />
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
