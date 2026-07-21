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
import { describe, expect, test } from 'bun:test'
import en from '../../i18n/locales/en.json'
import es from '../../i18n/locales/es.json'
import fr from '../../i18n/locales/fr.json'
import ja from '../../i18n/locales/ja.json'
import pt from '../../i18n/locales/pt.json'
import ru from '../../i18n/locales/ru.json'
import vi from '../../i18n/locales/vi.json'
import zh from '../../i18n/locales/zh.json'
import { audienceTemplateDescriptionKeys } from './copy'

const localeTranslations: Record<string, Record<string, string>> = {
  en: en.translation,
  es: es.translation,
  fr: fr.translation,
  ja: ja.translation,
  pt: pt.translation,
  ru: ru.translation,
  vi: vi.translation,
  zh: zh.translation,
}

const recallHelpKeys = [
  'Subject must be 200 characters or fewer',
  'Body text must be 2000 characters or fewer',
  'Stripe does not convert fixed Coupon amounts automatically. Configure each checkout currency explicitly.',
  'Audience templates define the base audience. The rules shown below narrow it further, and built-in eligibility filters also apply. Preview the audience before activation.',
  "Email content is translated automatically when saved, sent in each user's language, and falls back to English when unavailable.",
  ...Object.values(audienceTemplateDescriptionKeys),
] as const

describe('recall campaign copy', () => {
  test('maps each audience template to its explanation', () => {
    expect(audienceTemplateDescriptionKeys).toEqual({
      first_purchase:
        'Targets registered users in the PLG group who have never paid, for campaigns that encourage a first purchase.',
      lapsed_payer:
        'Targets previous payers who have not paid or used the API recently.',
      expired_subscription:
        'Targets previous subscribers whose subscription is no longer active and expired long enough ago.',
    })
  })

  for (const [locale, translations] of Object.entries(localeTranslations)) {
    test(`${locale} contains translated recall configuration help`, () => {
      for (const key of recallHelpKeys) {
        expect(
          Object.prototype.hasOwnProperty.call(translations, key),
          `${locale} is missing ${key}`
        ).toBe(true)
        expect(
          translations[key],
          `${locale} has an empty value for ${key}`
        ).toBeTruthy()

        if (locale !== 'en') {
          expect(
            translations[key],
            `${locale} should translate ${key}`
          ).not.toBe(key)
        }
      }
    })
  }
})
