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
import {
  buildPLGCatalogNotifyDefaults,
  buildPLGCatalogNotifyOptionUpdates,
  normalizePLGCatalogNotifyValues,
  plgCatalogNotifySchema,
} from './plg-catalog-notify-settings-utils'

describe('plg catalog notify settings utils', () => {
  test('buildDefaults falls back for missing values and never echoes the secret', () => {
    const built = buildPLGCatalogNotifyDefaults({
      'plg_catalog_notify_setting.dingtalk_alert_enabled': undefined as unknown as boolean,
      'plg_catalog_notify_setting.dingtalk_alert_webhook_url': undefined as unknown as string,
      'plg_catalog_notify_setting.dingtalk_alert_secret': 'SEC-should-not-leak',
      'plg_catalog_notify_setting.check_interval_minutes': undefined as unknown as number,
    })
    expect(built.plg_catalog_notify_setting).toEqual({
      dingtalk_alert_enabled: false,
      dingtalk_alert_webhook_url: '',
      dingtalk_alert_secret: '',
      check_interval_minutes: 5,
    })
  })

  test('schema requires webhook when enabled', () => {
    const result = plgCatalogNotifySchema.safeParse({
      plg_catalog_notify_setting: {
        dingtalk_alert_enabled: true,
        dingtalk_alert_webhook_url: '   ',
        dingtalk_alert_secret: '',
        check_interval_minutes: 5,
      },
    })
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues[0]?.path).toEqual([
        'plg_catalog_notify_setting',
        'dingtalk_alert_webhook_url',
      ])
    }
  })

  test('schema rejects non-http webhook and sub-minute interval', () => {
    const result = plgCatalogNotifySchema.safeParse({
      plg_catalog_notify_setting: {
        dingtalk_alert_enabled: false,
        dingtalk_alert_webhook_url: 'ftp://example.com/hook',
        dingtalk_alert_secret: '',
        check_interval_minutes: '0.5',
      },
    })
    expect(result.success).toBe(false)
    if (!result.success) {
      const paths = result.error.issues.map((issue) => issue.path.join('.'))
      expect(paths).toContain('plg_catalog_notify_setting.dingtalk_alert_webhook_url')
      expect(paths).toContain('plg_catalog_notify_setting.check_interval_minutes')
    }
  })

  test('schema coerces interval string from the input field', () => {
    const result = plgCatalogNotifySchema.safeParse({
      plg_catalog_notify_setting: {
        dingtalk_alert_enabled: false,
        dingtalk_alert_webhook_url: '',
        dingtalk_alert_secret: '',
        check_interval_minutes: '10',
      },
    })
    expect(result.success).toBe(true)
    if (result.success) {
      expect(result.data.plg_catalog_notify_setting.check_interval_minutes).toBe(10)
    }
  })

  test('normalize trims url and secret', () => {
    const normalized = normalizePLGCatalogNotifyValues({
      plg_catalog_notify_setting: {
        dingtalk_alert_enabled: true,
        dingtalk_alert_webhook_url: '  https://oapi.dingtalk.com/robot/send?access_token=abc ',
        dingtalk_alert_secret: ' SEC-new ',
        check_interval_minutes: 3,
      },
    })
    expect(normalized['plg_catalog_notify_setting.dingtalk_alert_webhook_url']).toBe(
      'https://oapi.dingtalk.com/robot/send?access_token=abc'
    )
    expect(normalized['plg_catalog_notify_setting.dingtalk_alert_secret']).toBe('SEC-new')
    expect(normalized['plg_catalog_notify_setting.check_interval_minutes']).toBe(3)
  })

  test('option updates only include changed keys as strings and skip a blank secret', () => {
    const values = {
      plg_catalog_notify_setting: {
        dingtalk_alert_enabled: true,
        dingtalk_alert_webhook_url: 'https://oapi.dingtalk.com/robot/send?access_token=abc',
        dingtalk_alert_secret: '',
        check_interval_minutes: 10,
      },
    }
    const updates = buildPLGCatalogNotifyOptionUpdates(values, {
      'plg_catalog_notify_setting.dingtalk_alert_enabled': true,
      'plg_catalog_notify_setting.dingtalk_alert_secret': '',
      'plg_catalog_notify_setting.check_interval_minutes': 10,
    })
    expect(updates).toEqual([
      { key: 'plg_catalog_notify_setting.dingtalk_alert_enabled', value: 'true' },
      { key: 'plg_catalog_notify_setting.check_interval_minutes', value: '10' },
    ])
  })

  test('option updates send a newly typed secret', () => {
    const updates = buildPLGCatalogNotifyOptionUpdates(
      {
        plg_catalog_notify_setting: {
          dingtalk_alert_enabled: false,
          dingtalk_alert_webhook_url: '',
          dingtalk_alert_secret: ' SEC-new ',
          check_interval_minutes: 5,
        },
      },
      { 'plg_catalog_notify_setting.dingtalk_alert_secret': ' SEC-new ' }
    )
    expect(updates).toEqual([
      { key: 'plg_catalog_notify_setting.dingtalk_alert_secret', value: 'SEC-new' },
    ])
  })
})
