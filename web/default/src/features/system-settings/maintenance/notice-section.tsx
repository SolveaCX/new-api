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
import { useEffect, useState } from 'react'
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
import { Markdown } from '@/components/ui/markdown'
import { Textarea } from '@/components/ui/textarea'
import { api } from '@/lib/api'
import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const noticeSchema = z.object({
  Notice: z.string().optional(),
})

type NoticeFormValues = z.infer<typeof noticeSchema>

type NoticeTranslation = {
  content: string
  extra?: string
}

type LocalizedNotice = {
  content: string
  translations?: Record<string, NoticeTranslation>
}

function parseNotice(value: string): LocalizedNotice {
  try {
    const parsed = JSON.parse(value) as LocalizedNotice
    if (typeof parsed.content === 'string') return parsed
  } catch {
    // Legacy Notice values are plain text.
  }
  return { content: value ?? '' }
}

type NoticeSectionProps = {
  defaultValue: string
}

export function NoticeSection({ defaultValue }: NoticeSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [isTranslating, setIsTranslating] = useState(false)
  const initialNotice = parseNotice(defaultValue ?? '')
  const form = useForm<NoticeFormValues>({
    resolver: zodResolver(noticeSchema),
    defaultValues: {
      Notice: initialNotice.content,
    },
  })
  const noticeContent = form.watch('Notice')?.trim()

  useEffect(() => {
    form.reset({ Notice: parseNotice(defaultValue ?? '').content })
  }, [defaultValue, form])

  const onSubmit = async (values: NoticeFormValues) => {
    const normalized = values.Notice ?? ''
    if (normalized === parseNotice(defaultValue ?? '').content) {
      return
    }

    setIsTranslating(true)
    let value = normalized
    try {
      const response = await api.post<{
        success: boolean
        message?: string
        data?: Record<string, NoticeTranslation>
      }>('/api/option/translate-announcement', { content: normalized })
      if (!response.data.success || !response.data.data) {
        throw new Error(response.data.message)
      }
      value = JSON.stringify({
        content: normalized,
        translations: response.data.data,
      })
    } catch {
      toast.error(t('Translation generation failed'))
    } finally {
      setIsTranslating(false)
    }
    await updateOption.mutateAsync({
      key: 'Notice',
      value,
    })
  }

  return (
    <SettingsSection title={t('System Notice')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel={
              isTranslating ? t('Generating translations') : 'Save notice'
            }
          />
          <FormField
            control={form.control}
            name='Notice'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Announcement content')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={8}
                    placeholder={t(
                      'Planned maintenance on Friday at 22:00 UTC...'
                    )}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t('Announcement displayed to users (supports Markdown & HTML)')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          {noticeContent ? (
            <section aria-label={t('Preview')} className='space-y-2'>
              <p className='text-sm font-medium'>{t('Preview')}</p>
              <div className='rounded-lg border bg-muted/30 p-4'>
                <Markdown>{noticeContent}</Markdown>
              </div>
            </section>
          ) : null}
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
