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
import { beforeAll, describe, expect, test } from 'bun:test'
import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import type { ModelAccessModel } from '../types'
import {
  getEffectiveVisibleModelCount,
  getNextVisibleModelCount,
  MODEL_ACCESS_PAGE_SIZE,
  ModelAccessList,
} from './model-access-list'

const testI18n = createInstance()

beforeAll(async () => {
  await testI18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })
})

function renderList(models: ModelAccessModel[], scopeIsEmpty: boolean) {
  return renderToStaticMarkup(
    <I18nextProvider i18n={testI18n}>
      <ModelAccessList
        models={models}
        scopeIsEmpty={scopeIsEmpty}
        onClearFilters={() => {}}
      />
    </I18nextProvider>
  )
}

describe('ModelAccessList', () => {
  test('renders an accessible copy action and explicit missing endpoint state', () => {
    const html = renderList(
      [
        {
          id: 'gpt-example',
          allowlist_match_key: 'gpt-example',
          vendor: null,
          supported_endpoint_types: [],
          availability_status: 'unknown',
        },
      ],
      false
    )

    expect(html).toContain('aria-label="Copy to clipboard"')
    expect(html).toContain('Endpoint not specified')
    expect(html).toContain('Unknown failure')
  })

  test('distinguishes filter misses from a truly empty scope', () => {
    const filteredHtml = renderList([], false)
    const emptyScopeHtml = renderList([], true)

    expect(filteredHtml).toContain('No models match the selected filters')
    expect(filteredHtml).toContain('No models match your current filters.')
    expect(filteredHtml).not.toContain(
      'No models are available in the current scope.'
    )
    expect(emptyScopeHtml).toContain('No available models')
    expect(emptyScopeHtml).toContain(
      'No models are available in the current scope.'
    )
  })

  test('renders only the first page and offers incremental expansion', () => {
    const models = Array.from(
      { length: MODEL_ACCESS_PAGE_SIZE + 1 },
      (_, i) => ({
        id: `model-${i + 1}`,
        allowlist_match_key: `model-${i + 1}`,
        vendor: null,
        supported_endpoint_types: [],
        availability_status: 'available' as const,
      })
    )

    const html = renderList(models, false)

    expect(html).toContain(`model-${MODEL_ACCESS_PAGE_SIZE}`)
    expect(html).not.toContain(`model-${MODEL_ACCESS_PAGE_SIZE + 1}`)
    expect(html).toContain('>More</button>')
    expect(
      getNextVisibleModelCount(MODEL_ACCESS_PAGE_SIZE, models.length)
    ).toBe(models.length)
  })

  test('resets effective pagination when the model dataset changes', () => {
    const previousModels = []
    const nextModels = []
    const pagination = {
      models: previousModels,
      scopeIsEmpty: false,
      visibleCount: 150,
    }

    expect(
      getEffectiveVisibleModelCount(pagination, previousModels, false)
    ).toBe(150)
    expect(getEffectiveVisibleModelCount(pagination, nextModels, false)).toBe(
      MODEL_ACCESS_PAGE_SIZE
    )
    expect(
      getEffectiveVisibleModelCount(pagination, previousModels, true)
    ).toBe(MODEL_ACCESS_PAGE_SIZE)
  })
})
