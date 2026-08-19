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
import { useEffect, useMemo, useRef, type ChangeEvent } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
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
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  SettingsForm,
  SettingsSwitchField,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import {
  BANNER_CONTENT_MAX_LENGTH,
  BANNER_HREF_MAX_LENGTH,
  BANNER_ICON_MAX_BYTES,
  OFFICIAL_WEBSITE_LOCALE_LABELS,
  OFFICIAL_WEBSITE_LOCALES,
  type OfficialWebsiteBannerContent,
  parseBannerContent,
  serializeBannerContent,
} from './official-website-config'

const BANNER_ENABLED_KEY = 'console_setting.official_website_banner_enabled'
const BANNER_CONTENT_KEY = 'console_setting.official_website_banner_content'
const BANNER_HREF_KEY = 'console_setting.official_website_banner_href'
const BANNER_ICON_KEY = 'console_setting.official_website_banner_icon'

const ACCEPTED_ICON_TYPES = 'image/png,image/jpeg,image/webp,image/svg+xml,image/gif'

const bannerSchema = z.object({
  enabled: z.boolean(),
  href: z.string(),
  icon: z.string(),
  content: z.record(z.string(), z.string()),
})

type BannerFormValues = z.infer<typeof bannerSchema>

type OfficialWebsiteSectionProps = {
  defaultValues: {
    enabled: boolean
    content: string
    href: string
    icon: string
  }
}

export function OfficialWebsiteSection({
  defaultValues,
}: OfficialWebsiteSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const iconFileInputRef = useRef<HTMLInputElement>(null)

  const formDefaults = useMemo<BannerFormValues>(
    () => ({
      enabled: defaultValues.enabled,
      href: defaultValues.href ?? '',
      icon: defaultValues.icon ?? '',
      content: parseBannerContent(defaultValues.content ?? ''),
    }),
    [defaultValues]
  )

  const bannerSchemaWithI18n = useMemo(
    () =>
      z.object({
        enabled: z.boolean(),
        href: z
          .string()
          .max(
            BANNER_HREF_MAX_LENGTH,
            t('Link must be {{max}} characters or fewer', {
              max: BANNER_HREF_MAX_LENGTH,
            })
          )
          .refine(
            (value) => {
              const trimmed = value.trim()
              if (!trimmed) return true
              if (trimmed.startsWith('//')) return false
              if (trimmed.startsWith('/')) return true
              return /^https?:\/\/\S+$/i.test(trimmed)
            },
            t('Enter a site path starting with / or a full http(s) link')
          ),
        icon: z.string(),
        content: z
          .record(z.string(), z.string())
          .refine(
            (value) =>
              Object.values(value).every(
                (copy) => copy.trim().length <= BANNER_CONTENT_MAX_LENGTH
              ),
            t('Banner text must be {{max}} characters or fewer', {
              max: BANNER_CONTENT_MAX_LENGTH,
            })
          )
          .refine(
            (value) => {
              const hasAnyCopy = Object.values(value).some(
                (copy) => copy.trim().length > 0
              )
              return !hasAnyCopy || (value.en ?? '').trim().length > 0
            },
            t(
              'English text is required — other languages fall back to it when left empty'
            )
          ),
      }),
    [t]
  )

  const form = useForm<BannerFormValues>({
    resolver: zodResolver(bannerSchemaWithI18n) as never,
    defaultValues: formDefaults,
  })

  useEffect(() => {
    form.reset(formDefaults)
  }, [formDefaults, form])

  const iconValue = form.watch('icon')

  // The cross-locale rules (english required, max length) attach to the whole
  // `content` record rather than to one field, so surface them separately.
  const contentErrorMessage = form.formState.errors.content?.message
  const contentError =
    typeof contentErrorMessage === 'string' ? contentErrorMessage : ''

  const handleIconFileChange = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (!file) return

    // The icon is stored inline as a data URL and shipped on every
    // /api/status response, so it has to stay small.
    if (file.size > BANNER_ICON_MAX_BYTES) {
      toast.error(
        t('Icon file must be {{max}} KB or smaller', {
          max: BANNER_ICON_MAX_BYTES / 1024,
        })
      )
      event.target.value = ''
      return
    }

    const reader = new FileReader()
    reader.onload = (loadEvent) => {
      const result =
        typeof loadEvent.target?.result === 'string'
          ? loadEvent.target.result
          : ''
      if (result.length > BANNER_ICON_MAX_BYTES) {
        toast.error(
          t('Icon file must be {{max}} KB or smaller', {
            max: BANNER_ICON_MAX_BYTES / 1024,
          })
        )
        return
      }
      form.setValue('icon', result, { shouldDirty: true })
    }
    reader.readAsDataURL(file)
    event.target.value = ''
  }

  const onSubmit = async (values: BannerFormValues) => {
    const serializedContent = serializeBannerContent(
      values.content as OfficialWebsiteBannerContent
    )
    const trimmedHref = values.href.trim()

    // Each option is its own row, so only push the ones that actually changed.
    const updates: Array<{ key: string; value: string | boolean }> = []
    if (values.enabled !== defaultValues.enabled) {
      updates.push({ key: BANNER_ENABLED_KEY, value: values.enabled })
    }
    if (serializedContent !== (defaultValues.content ?? '')) {
      updates.push({ key: BANNER_CONTENT_KEY, value: serializedContent })
    }
    if (trimmedHref !== (defaultValues.href ?? '')) {
      updates.push({ key: BANNER_HREF_KEY, value: trimmedHref })
    }
    if (values.icon !== (defaultValues.icon ?? '')) {
      updates.push({ key: BANNER_ICON_KEY, value: values.icon })
    }

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }
  }

  return (
    <SettingsSection title={t('Official website content')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save banner'
          />

          <Alert>
            <AlertDescription className='text-xs'>
              {t(
                'This banner appears at the top of every official website page. Leave all text empty to show the built-in default banner. Changes reach visitors within about two minutes.'
              )}
            </AlertDescription>
          </Alert>

          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchField
                checked={field.value}
                onCheckedChange={field.onChange}
                label={t('Show website banner')}
                description={t(
                  'Turn off to hide the banner across the whole official website.'
                )}
              />
            )}
          />

          <FormField
            control={form.control}
            name='href'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Banner link')}</FormLabel>
                <FormControl>
                  <Input placeholder='/blog/product-launch' {...field} />
                </FormControl>
                <FormDescription>
                  {t(
                    'Site paths starting with / are localized automatically. Leave empty for a text-only banner.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormItem>
            <FormLabel>{t('Banner icon')}</FormLabel>
            <div className='flex items-center gap-3'>
              {iconValue ? (
                <img
                  src={iconValue}
                  alt={t('Banner icon')}
                  className='h-10 w-10 rounded border object-contain p-1'
                />
              ) : (
                <div className='bg-muted text-muted-foreground flex h-10 w-10 items-center justify-center rounded border text-xs'>
                  {t('Icon')}
                </div>
              )}
              <input
                ref={iconFileInputRef}
                type='file'
                accept={ACCEPTED_ICON_TYPES}
                className='hidden'
                onChange={handleIconFileChange}
              />
              <Button
                type='button'
                variant='outline'
                onClick={() => iconFileInputRef.current?.click()}
              >
                {t('Upload')}
              </Button>
              {iconValue ? (
                <Button
                  type='button'
                  variant='outline'
                  onClick={() =>
                    form.setValue('icon', '', { shouldDirty: true })
                  }
                >
                  {t('Clear')}
                </Button>
              ) : null}
            </div>
            <FormDescription>
              {t(
                'PNG, JPEG, WebP, SVG, or GIF, up to {{max}} KB. Leave empty to show no icon.',
                { max: BANNER_ICON_MAX_BYTES / 1024 }
              )}
            </FormDescription>
          </FormItem>

          <div data-settings-form-span='full' className='flex flex-col gap-4'>
            <div className='flex flex-col gap-1'>
              <Label className='text-sm font-medium'>
                {t('Banner text by language')}
              </Label>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'English is required. Languages left empty fall back to the English text.'
                )}
              </p>
            </div>
            <div className='grid gap-4 lg:grid-cols-2'>
              {OFFICIAL_WEBSITE_LOCALES.map((locale) => (
                <FormField
                  key={locale}
                  control={form.control}
                  name={`content.${locale}` as const}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {OFFICIAL_WEBSITE_LOCALE_LABELS[locale]}
                        {locale === 'en' ? ' *' : ''}
                      </FormLabel>
                      <FormControl>
                        <Textarea
                          rows={3}
                          maxLength={BANNER_CONTENT_MAX_LENGTH}
                          placeholder={
                            locale === 'en'
                              ? t('New model is live. Join our Discord for free credits.')
                              : ''
                          }
                          {...field}
                          value={field.value ?? ''}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              ))}
            </div>
            {contentError ? (
              <p className='text-destructive text-sm'>{contentError}</p>
            ) : null}
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
