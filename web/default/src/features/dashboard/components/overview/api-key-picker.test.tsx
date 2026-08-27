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
import { readFileSync } from 'node:fs'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import type { ApiKey } from '@/features/keys/types'
import { ApiKeyPicker } from './api-key-picker'

const testI18n = createInstance()

beforeAll(async () => {
  await testI18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })
})

function apiKey(id: number): ApiKey {
  return {
    id,
    name: `Key ${id}`,
    key: `masked-${id}`,
    status: 1,
    remain_quota: 1_000_000,
    used_quota: 0,
    unlimited_quota: true,
    expired_time: -1,
    created_time: 1_700_000_000 + id,
    accessed_time: 1_700_000_000 + id,
    group: '',
    cross_group_retry: false,
    model_limits_enabled: false,
    model_limits: '',
    model_blacklist_enabled: false,
    model_blacklist: '',
    allow_ips: '',
  }
}

function renderPicker() {
  return renderToStaticMarkup(
    <I18nextProvider i18n={testI18n}>
      <QueryClientProvider client={new QueryClient()}>
        <ApiKeyPicker
          keys={[apiKey(1), apiKey(2), apiKey(3)]}
          loading={false}
          selectedKeyId={1}
          resolvedKeys={{}}
          loadingKeys={{}}
          resolveKey={async () => 'sk-real'}
          onSelect={() => undefined}
        />
      </QueryClientProvider>
    </I18nextProvider>
  )
}

describe('overview API key picker', () => {
  test('renders one independently discoverable copy control per key', () => {
    const html = renderPicker()

    expect((html.match(/aria-label="Copy API key"/g) ?? []).length).toBe(3)
    expect(html).toContain('sk-masked-1')
    expect(html).toContain('sk-masked-2')
    expect(html).toContain('sk-masked-3')
  })

  test('resolves an unmasked value before copying an unresolved row', () => {
    const source = readFileSync(
      new URL('./api-key-picker.tsx', import.meta.url),
      'utf8'
    )

    expect(source).toContain(
      'props.fullKey ?? (await props.resolveKey(props.keyId))'
    )
    expect(source).not.toContain('value={`sk-${key.key}`')
  })
})
