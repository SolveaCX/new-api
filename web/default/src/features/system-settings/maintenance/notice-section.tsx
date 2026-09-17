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
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Markdown } from '@/components/ui/markdown'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { api } from '@/lib/api'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const NOTICE_LOCALES = ['zh', 'en', 'es', 'fr', 'pt', 'ru', 'ja', 'vi'] as const
type NoticeLocale = (typeof NOTICE_LOCALES)[number]

const NOTICE_LOCALE_LABELS: Record<NoticeLocale, string> = {
  zh: '中文',
  en: 'English',
  es: 'Español',
  fr: 'Français',
  pt: 'Português',
  ru: 'Русский',
  ja: '日本語',
  vi: 'Tiếng Việt',
}

type NoticeTranslation = { content: string; extra?: string }
type LocalizedNotice = {
  content: string
  translations: Record<string, NoticeTranslation>
}

function parseNotice(value: string): LocalizedNotice {
  try {
    const parsed: unknown = JSON.parse(value)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      const notice = parsed as Record<string, unknown>
      if (typeof notice.content === 'string') {
        const translations: LocalizedNotice['translations'] = {}
        if (notice.translations && typeof notice.translations === 'object') {
          const stored = notice.translations as Record<string, unknown>
          for (const [locale, entry] of Object.entries(stored)) {
            if (entry && typeof entry === 'object') {
              const translation = entry as Record<string, unknown>
              const content = translation.content
              if (typeof content === 'string' && content.trim()) {
                translations[locale] = {
                  content,
                  ...(typeof translation.extra === 'string'
                    ? { extra: translation.extra }
                    : {}),
                }
              }
            }
          }
        }
        return { content: notice.content, translations }
      }
    }
  } catch {
    // Older notices are plain text.
  }
  return { content: value ?? '', translations: {} }
}

type NoticeSectionProps = { defaultValue: string }

export function NoticeSection({ defaultValue }: NoticeSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [notice, setNotice] = useState<LocalizedNotice>(() =>
    parseNotice(defaultValue ?? '')
  )
  const [activeLocale, setActiveLocale] = useState<NoticeLocale>('zh')
  const [isTranslating, setIsTranslating] = useState(false)

  useEffect(() => {
    setNotice(parseNotice(defaultValue ?? ''))
  }, [defaultValue])

  const currentContent =
    activeLocale === 'zh'
      ? notice.content
      : (notice.translations[activeLocale]?.content ?? '')

  const setContent = (content: string) => {
    setNotice((previous) => {
      if (activeLocale === 'zh') return { ...previous, content }
      const translations = { ...previous.translations }
      if (content.trim()) {
        translations[activeLocale] = {
          ...translations[activeLocale],
          content,
        }
      } else {
        delete translations[activeLocale]
      }
      return { ...previous, translations }
    })
  }

  const generateTranslations = async () => {
    const sourceContent = notice.content.trim()
    if (!sourceContent) {
      toast.error(t('Enter Chinese notice content first'))
      setActiveLocale('zh')
      return
    }
    setIsTranslating(true)
    try {
      const response = await api.post<{
        success: boolean
        message?: string
        data?: Record<string, NoticeTranslation>
      }>('/api/option/translate-announcement', { content: sourceContent })
      if (!response.data.success || !response.data.data) {
        throw new Error(response.data.message)
      }
      setNotice((previous) => {
        if (previous.content.trim() !== sourceContent) return previous
        const translations = { ...previous.translations }
        for (const locale of NOTICE_LOCALES) {
          const generated = response.data.data?.[locale]?.content
          if (locale !== 'zh' && generated?.trim()) {
            translations[locale] = { content: generated }
          }
        }
        return { ...previous, translations }
      })
      toast.success(t('Translations generated. Review and save to publish.'))
    } catch {
      toast.error(t('Translation generation failed'))
    } finally {
      setIsTranslating(false)
    }
  }

  const saveNotice = async () => {
    const content = notice.content.trim()
    const translations = Object.fromEntries(
      Object.entries(notice.translations)
        .filter(([locale, value]) => locale !== 'zh' && value?.content.trim())
        .map(([locale, value]) => [
          locale,
          { ...value, content: value.content.trim() },
        ])
    )
    const value = content ? JSON.stringify({ content, translations }) : ''
    if (value === defaultValue || (!content && !defaultValue)) return
    await updateOption.mutateAsync({ key: 'Notice', value })
  }

  return (
    <SettingsSection title={t('System Notice')}>
      <SettingsPageFormActions
        onSave={saveNotice}
        isSaving={updateOption.isPending}
        isSaveDisabled={isTranslating}
        saveLabel='Save notice'
      />
      <div className='space-y-5'>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Publish one notice in multiple languages. Missing translations fall back to Chinese.'
          )}
        </p>
        <Tabs
          value={activeLocale}
          onValueChange={(value) => setActiveLocale(value as NoticeLocale)}
        >
          <TabsList className='flex h-auto w-full max-w-full justify-start gap-1 overflow-x-auto'>
            {NOTICE_LOCALES.map((locale) => (
              <TabsTrigger key={locale} value={locale}>
                {NOTICE_LOCALE_LABELS[locale]}
                {(
                  locale === 'zh'
                    ? notice.content.trim()
                    : notice.translations[locale]?.content.trim()
                )
                  ? ' ✓'
                  : ''}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
        <div className='space-y-2'>
          <label htmlFor='notice-content' className='text-sm font-medium'>
            {t('Announcement content')} · {NOTICE_LOCALE_LABELS[activeLocale]}
          </label>
          <Textarea
            id='notice-content'
            rows={8}
            value={currentContent}
            onChange={(event) => setContent(event.target.value)}
            placeholder={t('Planned maintenance on Friday at 22:00 UTC...')}
          />
          <p className='text-muted-foreground text-sm'>
            {t('Announcement displayed to users (supports Markdown & HTML)')}
          </p>
        </div>
        {currentContent.trim() ? (
          <section aria-label={t('Preview')} className='space-y-2'>
            <p className='text-sm font-medium'>{t('Preview')}</p>
            <div className='rounded-lg border bg-muted/30 p-4'>
              <Markdown
                className={
                  '[&_h1]:mt-0 [&_h1]:mb-4 [&_h2]:mt-4 [&_h2]:mb-3 [&_h3]:mt-3 [&_h3]:mb-2 ' +
                  '[&_p]:my-3 [&_ul]:my-3 [&_ul]:list-disc [&_ul]:pl-6 ' +
                  '[&_ol]:my-3 [&_ol]:list-decimal [&_ol]:pl-6 [&_li]:my-1'
                }
              >
                {currentContent}
              </Markdown>
            </div>
          </section>
        ) : null}
        <Button
          type='button'
          variant='outline'
          disabled={isTranslating || updateOption.isPending}
          onClick={generateTranslations}
        >
          {isTranslating
            ? t('Generating translations')
            : t('Generate translations')}
        </Button>
      </div>
    </SettingsSection>
  )
}
