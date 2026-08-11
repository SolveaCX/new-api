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
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, test } from 'bun:test'

const LANGUAGES = ['en', 'zh', 'es', 'fr', 'ja', 'pt', 'ru', 'vi'] as const
const NON_EN_LANGUAGES = LANGUAGES.filter((language) => language !== 'en')

const MCP_OAUTH_I18N_KEYS = [
  'Connected Apps',
  'Connected app authorizations',
  'Wallet, connected apps, and personal preferences.',
  'Review and revoke apps connected to your account.',
  'Connected app revoked',
  'Connected apps unavailable',
  'No connected apps',
  'Approved apps will appear here.',
  'Active',
  'Revoked',
  'Revoke',
  'Revoke {{name}}',
  'Created',
  'Last used',
  'Revoke connected app',
  'Revoke access for {{name}}? This app will no longer be able to use your authorization grant.',
  'Authorize MCP access',
  'Review the app and permissions before continuing.',
  'Authorization unavailable',
  'Application',
  'Unknown application',
  'Flatkey Tools resource',
  'No resource provided',
  'Requested scopes',
  'No scopes requested',
  'Deny',
  'Submitting...',
  'Approve',
] as const

const REVOKE_CONFIRMATION_KEY =
  'Revoke access for {{name}}? This app will no longer be able to use your authorization grant.'

// Whole-value English copies are not allowed unless the localized term is
// intentionally identical in that language.
const ALLOW_ENGLISH_VALUE_COPY = new Set<string>(['fr:Application'])

type Locale = {
  translation: Record<string, string>
  [key: string]: unknown
}

function readLocale(language: (typeof LANGUAGES)[number]): Locale {
  const path = join(
    process.cwd(),
    'src',
    'i18n',
    'locales',
    `${language}.json`
  )
  return JSON.parse(readFileSync(path, 'utf8')) as Locale
}

describe('MCP OAuth locale coverage', () => {
  test('keeps the connected app and authorization keys translated', () => {
    const locales = Object.fromEntries(
      LANGUAGES.map((language) => [language, readLocale(language)])
    ) as Record<(typeof LANGUAGES)[number], Locale>

    for (const key of MCP_OAUTH_I18N_KEYS) {
      for (const language of LANGUAGES) {
        expect(locales[language].translation[key]).toBeTruthy()
        expect(locales[language][key]).toBeUndefined()
      }

      for (const language of NON_EN_LANGUAGES) {
        const value = locales[language].translation[key]
        const asciiQuestionMarkCount = value.match(/\?/g)?.length ?? 0
        if (key === REVOKE_CONFIRMATION_KEY) {
          expect(asciiQuestionMarkCount).toBe(1)
        } else {
          expect(asciiQuestionMarkCount).toBe(0)
        }
        expect(value).not.toContain('\uFFFD')
        if (!ALLOW_ENGLISH_VALUE_COPY.has(`${language}:${key}`)) {
          expect(value).not.toBe(locales.en.translation[key])
        }
      }
    }
  })
})
