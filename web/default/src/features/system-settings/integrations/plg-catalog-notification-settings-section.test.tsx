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
import { beforeAll, describe, expect, mock, test } from 'bun:test'
import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider, initReactI18next } from 'react-i18next'

// The unsaved-changes guard needs a live router; static rendering only cares
// about the form markup, so the blocker is stubbed as "never blocking".
mock.module('@tanstack/react-router', () => ({
  useBlocker: () => ({ status: 'idle', proceed: () => {}, reset: () => {} }),
}))

const { PLGCatalogNotificationSettingsSection } = await import(
  './plg-catalog-notification-settings-section'
)

const i18n = createInstance()

describe('PLG catalog notification settings section', () => {
  beforeAll(async () => {
    await i18n.use(initReactI18next).init({
      lng: 'en',
      fallbackLng: 'en',
      resources: { en: { translation: {} } },
      interpolation: { escapeValue: false },
    })
  })

  test('renders the dedicated robot fields with saved values and hides the secret', () => {
    const html = renderToStaticMarkup(
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={new QueryClient()}>
          <PLGCatalogNotificationSettingsSection
            defaultValues={{
              'plg_catalog_notify_setting.dingtalk_alert_enabled': true,
              'plg_catalog_notify_setting.dingtalk_alert_webhook_url':
                'https://oapi.dingtalk.com/robot/send?access_token=abc',
              'plg_catalog_notify_setting.dingtalk_alert_secret': 'SEC-saved',
              'plg_catalog_notify_setting.check_interval_minutes': 10,
            }}
          />
        </QueryClientProvider>
      </I18nextProvider>
    )

    expect(html).toContain('PLG Catalog Notifications')
    expect(html).toContain('Enable PLG catalog DingTalk alerts')
    expect(html).toContain('access_token=abc')
    expect(html).toContain('Check interval (minutes)')
    expect(html).toContain('value="10"')
    expect(html).not.toContain('SEC-saved')
    expect(html).not.toContain('monitor_setting')
  })
})
