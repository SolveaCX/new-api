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
import en from '@/i18n/locales/en.json'
import es from '@/i18n/locales/es.json'
import fr from '@/i18n/locales/fr.json'
import ja from '@/i18n/locales/ja.json'
import pt from '@/i18n/locales/pt.json'
import ru from '@/i18n/locales/ru.json'
import vi from '@/i18n/locales/vi.json'
import zh from '@/i18n/locales/zh.json'
import { describe, expect, test } from 'bun:test'

const EDIT_API_KEY = 'Edit API key'
const API_KEY_STATISTICS = 'API Key Statistics'

const expectedTranslations = {
  en: 'Edit API key',
  es: 'Editar clave API',
  fr: 'Modifier la clé API',
  ja: 'APIキーを編集',
  pt: 'Editar chave de API',
  ru: 'Редактировать ключ API',
  vi: 'Chỉnh sửa khóa API',
  zh: '编辑 API 密钥',
} as const

const translations = {
  en: en.translation,
  es: es.translation,
  fr: fr.translation,
  ja: ja.translation,
  pt: pt.translation,
  ru: ru.translation,
  vi: vi.translation,
  zh: zh.translation,
} as const

const expectedStatisticsTranslations = {
  en: 'API Key Statistics',
  es: 'Estadísticas de claves API',
  fr: 'Statistiques des clés API',
  ja: 'APIキー統計',
  pt: 'Estatísticas das chaves de API',
  ru: 'Статистика API-ключей',
  vi: 'Thống kê khóa API',
  zh: 'API 密钥统计',
} as const

describe('API key dialog translations', () => {
  test('provides a reviewed edit title in every supported locale', () => {
    for (const locale of Object.keys(expectedTranslations)) {
      const typedLocale = locale as keyof typeof expectedTranslations
      expect(translations[typedLocale][EDIT_API_KEY]).toBe(
        expectedTranslations[typedLocale]
      )
    }
  })

  test('does not copy the English edit title into translated locales', () => {
    for (const locale of Object.keys(expectedTranslations)) {
      if (locale === 'en') continue
      const typedLocale = locale as Exclude<
        keyof typeof expectedTranslations,
        'en'
      >
      expect(translations[typedLocale][EDIT_API_KEY]).not.toBe(
        expectedTranslations.en
      )
    }
  })
})

describe('API key statistics translations', () => {
  test('provides a reviewed title in every supported locale', () => {
    for (const locale of Object.keys(expectedStatisticsTranslations)) {
      const typedLocale = locale as keyof typeof expectedStatisticsTranslations
      expect(translations[typedLocale][API_KEY_STATISTICS]).toBe(
        expectedStatisticsTranslations[typedLocale]
      )
    }
  })
})
