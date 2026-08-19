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

/**
 * Locales served by the official website (`website/src/lib/locales.ts`).
 *
 * The backend keeps the same list in
 * `setting/console_setting/official_website.go`; all three must stay in sync
 * when a new website language ships.
 */
export const OFFICIAL_WEBSITE_LOCALES = [
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

export type OfficialWebsiteLocale = (typeof OFFICIAL_WEBSITE_LOCALES)[number]

/** Display names shown next to each banner copy field. */
export const OFFICIAL_WEBSITE_LOCALE_LABELS: Record<
  OfficialWebsiteLocale,
  string
> = {
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

export const BANNER_CONTENT_MAX_LENGTH = 300
export const BANNER_HREF_MAX_LENGTH = 500
/** Matches `officialWebsiteBannerIconMaxBytes` in the Go validator. */
export const BANNER_ICON_MAX_BYTES = 64 * 1024

export type OfficialWebsiteBannerContent = Partial<
  Record<OfficialWebsiteLocale, string>
>

/**
 * Parse the stored `console_setting.official_website_banner_content` option.
 *
 * Returns an empty map for unset or malformed values so the form falls back to
 * empty inputs rather than crashing on a hand-edited option row.
 */
export function parseBannerContent(value: string): OfficialWebsiteBannerContent {
  const trimmed = (value ?? '').trim()
  if (!trimmed) return {}

  let parsed: unknown
  try {
    parsed = JSON.parse(trimmed)
  } catch {
    return {}
  }
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {}

  const source = parsed as Record<string, unknown>
  const content: OfficialWebsiteBannerContent = {}
  for (const locale of OFFICIAL_WEBSITE_LOCALES) {
    const copy = source[locale]
    if (typeof copy !== 'string') continue
    const value = copy.trim()
    if (value) content[locale] = value
  }
  return content
}

/**
 * Serialize the form values back into the option value.
 *
 * Blank locales are dropped, and an entirely blank form serializes to `''` so
 * the website falls back to its built-in default banner.
 */
export function serializeBannerContent(
  content: OfficialWebsiteBannerContent
): string {
  const normalized: Record<string, string> = {}
  for (const locale of OFFICIAL_WEBSITE_LOCALES) {
    const value = (content[locale] ?? '').trim()
    if (value) normalized[locale] = value
  }
  return Object.keys(normalized).length === 0 ? '' : JSON.stringify(normalized)
}
