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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeAll, describe, expect, test } from 'bun:test'
import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { CampaignEditor } from './campaign-editor'

const commonHelp =
  'Audience templates define the base audience. The rules below narrow it further, and every condition must match. Preview the audience before activation.'
const firstPurchaseHelp =
  'Targets registered users who have never paid, for campaigns that encourage a first purchase.'
const testI18n = createInstance()

beforeAll(async () => {
  await testI18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    resources: {
      en: {
        translation: {
          [commonHelp]: commonHelp,
          [firstPurchaseHelp]: firstPurchaseHelp,
        },
      },
    },
    interpolation: { escapeValue: false },
  })
})

describe('CampaignEditor audience help', () => {
  test('explains the selected audience before its rules', () => {
    const html = renderToStaticMarkup(
      <QueryClientProvider client={new QueryClient()}>
        <I18nextProvider i18n={testI18n}>
          <CampaignEditor />
        </I18nextProvider>
      </QueryClientProvider>
    )

    expect(html).toContain(commonHelp)
    expect(html).toContain(firstPurchaseHelp)
  })
})
