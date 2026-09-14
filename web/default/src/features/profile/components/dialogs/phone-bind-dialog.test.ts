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
import en from '../../../../i18n/locales/en.json'
import es from '../../../../i18n/locales/es.json'
import fr from '../../../../i18n/locales/fr.json'
import ja from '../../../../i18n/locales/ja.json'
import pt from '../../../../i18n/locales/pt.json'
import ru from '../../../../i18n/locales/ru.json'
import vi from '../../../../i18n/locales/vi.json'
import zh from '../../../../i18n/locales/zh.json'
import { maskPhoneNumber } from '../../lib/format'

const phoneBindingKeys = [
  'Back to verification',
  'Bind Phone',
  'Bind a phone number to your account.',
  'Change Phone',
  'Enter the code sent to {{phone}}',
  'Enter your phone number',
  'Phone Number',
  'Phone binding preview updated.',
  'Please enter a valid phone number',
  'Prototype only: no verification code was sent.',
  'Prototype only: verification codes are not sent and changes are not saved.',
  'Verify Current Phone',
  'Verify your current phone before binding a new one.',
] as const

const translations: Record<
  string,
  { translation: Record<string, string> }
> = { en, es, fr, ja, pt, ru, vi, zh }

describe('maskPhoneNumber', () => {
  test('masks a Chinese mobile number and keeps the country code', () => {
    expect(maskPhoneNumber('+86 138-1234-8000')).toBe('+86 138****8000')
  })

  test('does not expose short phone values', () => {
    expect(maskPhoneNumber('123456')).toBe('****')
  })
})

describe('phone binding translations', () => {
  test('defines translated copy in every interface locale', () => {
    for (const [locale, resource] of Object.entries(translations)) {
      for (const key of phoneBindingKeys) {
        const value = resource.translation[key]
        expect(value, `${locale} is missing ${key}`).toBeTruthy()
        if (locale !== 'en') {
          expect(value, `${locale} should translate ${key}`).not.toBe(key)
        }
      }
    }
  })
})
