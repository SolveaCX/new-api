import { createElement } from 'react'
import { beforeAll, describe, expect, test } from 'bun:test'
import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { MultiSelect } from './multi-select'

const testI18n = createInstance()

beforeAll(async () => {
  await testI18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })
})

describe('MultiSelect search notifications', () => {
  test('accepts an optional search-change callback without changing server markup', () => {
    const html = renderToStaticMarkup(
      createElement(
        I18nextProvider,
        { i18n: testI18n },
        createElement(MultiSelect, {
          options: [{ label: 'Ada Lovelace', value: '1' }],
          selected: [],
          onChange: () => undefined,
          onSearchChange: () => undefined,
          placeholder: 'Search users',
        })
      )
    )

    expect(html).toContain('aria-label="Search users"')
  })

  test('contains the callback hook for typed search and selection clears', async () => {
    const source = await Bun.file(
      new URL('./multi-select.tsx', import.meta.url)
    ).text()

    expect(source).toContain('onSearchChange?: (value: string) => void')
    expect(source).toMatch(/onSearchChange\?\.\(value\)/)
    expect(source).toMatch(/onSearchChange\?\.\(''\)/)
  })
})
