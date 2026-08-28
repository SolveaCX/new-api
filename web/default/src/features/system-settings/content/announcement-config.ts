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

export const WEBSITE_LOCALES = [
  'en',
  'zh',
  'es',
  'fr',
  'pt',
  'ru',
  'ja',
  'vi',
  'de',
  'id',
] as const

export type WebsiteLocale = (typeof WEBSITE_LOCALES)[number]

export const WEBSITE_LOCALE_LABELS: Record<WebsiteLocale, string> = {
  en: 'English',
  zh: '中文',
  es: 'Español',
  fr: 'Français',
  pt: 'Português',
  ru: 'Русский',
  ja: '日本語',
  vi: 'Tiếng Việt',
  de: 'Deutsch',
  id: 'Bahasa Indonesia',
}

/** Public-site assets that can be selected without uploading a file. */
export const BUILT_IN_MODEL_LOGOS = [
  { label: 'Flatkey', value: '/assets/logos/flatkey-mark-white.svg' },
  { label: 'OpenAI', value: '/assets/logos/openai.svg' },
  { label: 'Claude', value: '/assets/logos/claude.svg' },
  { label: 'Gemini', value: '/assets/logos/googlegemini.svg' },
  { label: 'DeepSeek', value: '/assets/logos/deepseek.svg' },
  { label: 'Qwen', value: '/assets/logos/qwen.svg' },
  { label: 'Kimi', value: '/assets/logos/moonshotai.svg' },
  { label: 'ByteDance / Seedance', value: '/assets/logos/bytedance.svg' },
  { label: 'MiniMax', value: '/assets/logos/minimax.svg' },
  { label: 'Z.ai / GLM', value: '/assets/logos/zai.svg' },
  { label: 'xAI / Grok', value: '/assets/logos/xai.svg' },
  { label: 'Mistral', value: '/assets/logos/mistralai.svg' },
  { label: 'Meta / Llama', value: '/assets/logos/meta.svg' },
  { label: 'Perplexity', value: '/assets/logos/perplexity.svg' },
  { label: 'Ollama', value: '/assets/logos/ollama.svg' },
  { label: 'Hugging Face', value: '/assets/logos/huggingface.svg' },
  { label: 'Baidu', value: '/logos/baidu.svg' },
  { label: 'Kuaishou / Kling', value: '/assets/logos/kuaishou.svg' },
  { label: 'ElevenLabs', value: '/assets/logos/elevenlabs.svg' },
] as const

/** Keep this in sync with the server-side announcement logo validator. */
export const ANNOUNCEMENT_LOGO_MAX_BYTES = 64 * 1024

const ANNOUNCEMENT_LOGO_PATH_PATTERN =
  /^\/(?:assets\/logos|logos)\/[A-Za-z0-9._/-]+$/
const ANNOUNCEMENT_LOGO_DATA_PATTERN =
  /^data:image\/(?:png|jpe?g|webp|gif|svg\+xml);base64,[A-Za-z0-9+/]+={0,2}$/i

/**
 * Validate the two logo forms that can safely be rendered by the public site:
 * a built-in asset path or a bounded base64 image data URL. Relative paths
 * are intentionally restricted to the public logo directories so an admin
 * cannot make every visitor fetch arbitrary third-party content.
 */
export function isSupportedAnnouncementLogo(value: string): boolean {
  const logo = value.trim()
  if (!logo) return true
  if (logo.length > ANNOUNCEMENT_LOGO_MAX_BYTES) return false

  if (ANNOUNCEMENT_LOGO_PATH_PATTERN.test(logo)) {
    return !logo.includes('..') && !logo.includes('?') && !logo.includes('#')
  }

  return ANNOUNCEMENT_LOGO_DATA_PATTERN.test(logo)
}

export type LocaleTextMap = Record<WebsiteLocale, string>

export type AnnouncementType =
  | 'default'
  | 'ongoing'
  | 'success'
  | 'warning'
  | 'error'

export type Announcement = {
  id: number
  content: string
  content_i18n: LocaleTextMap
  publishDate: string
  type: AnnouncementType
  extra?: string
  intro_i18n: LocaleTextMap
  link?: string
  link_label?: string
  link_label_i18n: LocaleTextMap
  logo?: string
  [key: string]: unknown
}

export type AnnouncementFormValues = {
  content_i18n: Record<string, string>
  intro_i18n: Record<string, string>
  link_label_i18n: Record<string, string>
  publishDate: string
  type: AnnouncementType
  link?: string
  logo?: string
}

export function emptyLocaleTextMap(): LocaleTextMap {
  return Object.fromEntries(
    WEBSITE_LOCALES.map((locale) => [locale, ''])
  ) as LocaleTextMap
}

function readLocaleTextMap(...values: unknown[]): LocaleTextMap {
  const result = emptyLocaleTextMap()

  for (const value of values) {
    if (!value || typeof value !== 'object' || Array.isArray(value)) continue
    const map = value as Record<string, unknown>
    for (const locale of WEBSITE_LOCALES) {
      if (!result[locale] && typeof map[locale] === 'string') {
        result[locale] = map[locale].trim()
      }
    }
  }

  return result
}

function readString(...values: unknown[]): string {
  for (const value of values) {
    if (typeof value === 'string' && value.trim()) return value.trim()
  }
  return ''
}

function firstLocalizedValue(map: LocaleTextMap): string {
  return (
    map.en || WEBSITE_LOCALES.map((locale) => map[locale]).find(Boolean) || ''
  )
}

export function normalizeAnnouncement(
  value: unknown,
  fallbackId: number
): Announcement | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  const item = value as Record<string, unknown>
  const legacyContent = readString(item.content)
  const contentI18n = readLocaleTextMap(
    item.content_i18n,
    item.contentI18n,
    item.contentByLocale
  )
  if (!contentI18n.en && legacyContent) contentI18n.en = legacyContent
  const content = firstLocalizedValue(contentI18n)
  if (!content) return null

  const legacyIntro = readString(item.intro, item.extra, item.summary)
  const introI18n = readLocaleTextMap(
    item.intro_i18n,
    item.introI18n,
    item.extra_i18n,
    item.extraI18n,
    item.summary_i18n,
    item.summaryI18n
  )
  if (!introI18n.en && legacyIntro) introI18n.en = legacyIntro

  const legacyLinkLabel = readString(
    item.link_label,
    item.linkLabel,
    item.link_text,
    item.linkText
  )
  const linkLabelI18n = readLocaleTextMap(
    item.link_label_i18n,
    item.linkLabelI18n,
    item.link_text_i18n,
    item.linkTextI18n
  )
  if (!linkLabelI18n.en && legacyLinkLabel) {
    linkLabelI18n.en = legacyLinkLabel
  }

  const rawId = item.id
  const id =
    typeof rawId === 'number' && Number.isFinite(rawId) ? rawId : fallbackId
  const publishDate = readString(item.publishDate) || new Date().toISOString()
  const type = isAnnouncementType(item.type) ? item.type : 'default'
  const link = readString(item.link, item.href, item.url)
  const logo = readString(item.logo, item.icon)
  const extra = firstLocalizedValue(introI18n)
  const linkLabel = firstLocalizedValue(linkLabelI18n)

  return {
    ...item,
    id,
    content,
    content_i18n: contentI18n,
    publishDate,
    type,
    ...(extra ? { extra } : {}),
    intro_i18n: introI18n,
    ...(link ? { link } : {}),
    ...(linkLabel ? { link_label: linkLabel } : {}),
    link_label_i18n: linkLabelI18n,
    ...(logo ? { logo } : {}),
  }
}

function isAnnouncementType(value: unknown): value is AnnouncementType {
  return (
    typeof value === 'string' &&
    ['default', 'ongoing', 'success', 'warning', 'error'].includes(value)
  )
}

export function announcementToFormValues(
  announcement?: Announcement | null
): AnnouncementFormValues {
  return {
    content_i18n: announcement?.content_i18n ?? emptyLocaleTextMap(),
    intro_i18n: announcement?.intro_i18n ?? emptyLocaleTextMap(),
    link_label_i18n: announcement?.link_label_i18n ?? emptyLocaleTextMap(),
    publishDate: announcement?.publishDate ?? new Date().toISOString(),
    type: announcement?.type ?? 'default',
    link: announcement?.link ?? '',
    logo: announcement?.logo ?? '',
  }
}

export function serializeAnnouncement(
  values: AnnouncementFormValues,
  id: number,
  previous?: Announcement | null
): Announcement {
  const contentI18n = {
    ...emptyLocaleTextMap(),
    ...values.content_i18n,
  } as LocaleTextMap
  const introI18n = {
    ...emptyLocaleTextMap(),
    ...values.intro_i18n,
  } as LocaleTextMap
  const linkLabelI18n = {
    ...emptyLocaleTextMap(),
    ...values.link_label_i18n,
  } as LocaleTextMap
  const content = firstLocalizedValue(contentI18n)
  const extra = firstLocalizedValue(introI18n)
  const linkLabel = firstLocalizedValue(linkLabelI18n)
  const link = values.link?.trim() ?? ''
  const logo = values.logo?.trim() ?? ''

  return {
    ...(previous ?? {}),
    id,
    content,
    content_i18n: contentI18n,
    publishDate: values.publishDate,
    type: values.type,
    extra,
    intro_i18n: introI18n,
    link,
    link_label: linkLabel,
    link_label_i18n: linkLabelI18n,
    logo,
  }
}
