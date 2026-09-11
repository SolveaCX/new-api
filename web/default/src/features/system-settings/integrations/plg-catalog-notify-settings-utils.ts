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
import { z } from 'zod'

export const PLG_CATALOG_DEFAULT_INTERVAL_MINUTES = 5
export const PLG_CATALOG_MIN_INTERVAL_MINUTES = 1

const isValidHttpUrl = (value: string) => {
  try {
    const url = new URL(value)
    return url.protocol === 'http:' || url.protocol === 'https:'
  } catch {
    return false
  }
}

export const plgCatalogNotifySchema = z
  .object({
    plg_catalog_notify_setting: z.object({
      dingtalk_alert_enabled: z.boolean(),
      dingtalk_alert_webhook_url: z.string(),
      dingtalk_alert_secret: z.string(),
      check_interval_minutes: z.coerce
        .number()
        .min(PLG_CATALOG_MIN_INTERVAL_MINUTES, 'Check interval must be at least 1 minute'),
    }),
  })
  .superRefine((values, ctx) => {
    const webhook = values.plg_catalog_notify_setting.dingtalk_alert_webhook_url.trim()
    if (values.plg_catalog_notify_setting.dingtalk_alert_enabled && webhook === '') {
      ctx.addIssue({
        code: 'custom',
        path: ['plg_catalog_notify_setting', 'dingtalk_alert_webhook_url'],
        message: 'DingTalk webhook URL is required when catalog notifications are enabled',
      })
    }
    if (webhook !== '' && !isValidHttpUrl(webhook)) {
      ctx.addIssue({
        code: 'custom',
        path: ['plg_catalog_notify_setting', 'dingtalk_alert_webhook_url'],
        message: 'Enter a valid http or https URL',
      })
    }
  })

export type PLGCatalogNotifyFormValues = z.output<typeof plgCatalogNotifySchema>

export type PLGCatalogNotifyDefaults = {
  'plg_catalog_notify_setting.dingtalk_alert_enabled': boolean
  'plg_catalog_notify_setting.dingtalk_alert_webhook_url': string
  'plg_catalog_notify_setting.dingtalk_alert_secret': string
  'plg_catalog_notify_setting.check_interval_minutes': number
}

export function buildPLGCatalogNotifyDefaults(
  defaults: PLGCatalogNotifyDefaults
): PLGCatalogNotifyFormValues {
  return {
    plg_catalog_notify_setting: {
      dingtalk_alert_enabled:
        defaults['plg_catalog_notify_setting.dingtalk_alert_enabled'] ?? false,
      dingtalk_alert_webhook_url:
        defaults['plg_catalog_notify_setting.dingtalk_alert_webhook_url'] ?? '',
      // The server never echoes the saved secret, so the field always starts
      // empty; leaving it empty keeps the stored value.
      dingtalk_alert_secret: '',
      check_interval_minutes:
        defaults['plg_catalog_notify_setting.check_interval_minutes'] ??
        PLG_CATALOG_DEFAULT_INTERVAL_MINUTES,
    },
  }
}

export function normalizePLGCatalogNotifyValues(
  values: PLGCatalogNotifyFormValues
): PLGCatalogNotifyDefaults {
  return {
    'plg_catalog_notify_setting.dingtalk_alert_enabled':
      values.plg_catalog_notify_setting.dingtalk_alert_enabled,
    'plg_catalog_notify_setting.dingtalk_alert_webhook_url':
      values.plg_catalog_notify_setting.dingtalk_alert_webhook_url.trim(),
    'plg_catalog_notify_setting.dingtalk_alert_secret':
      values.plg_catalog_notify_setting.dingtalk_alert_secret.trim(),
    'plg_catalog_notify_setting.check_interval_minutes':
      values.plg_catalog_notify_setting.check_interval_minutes,
  }
}

/**
 * Option updates for the keys the form reports as changed. Values are sent as
 * strings, matching the option store. An empty secret is skipped so leaving
 * the field blank never wipes the stored signing secret.
 */
export function buildPLGCatalogNotifyOptionUpdates(
  values: PLGCatalogNotifyFormValues,
  changedFields: Record<string, unknown>
): Array<{ key: keyof PLGCatalogNotifyDefaults; value: string }> {
  const normalized = normalizePLGCatalogNotifyValues(values)
  return (Object.keys(normalized) as Array<keyof PLGCatalogNotifyDefaults>)
    .filter((key) => key in changedFields)
    .filter(
      (key) =>
        key !== 'plg_catalog_notify_setting.dingtalk_alert_secret' ||
        normalized[key] !== ''
    )
    .map((key) => ({ key, value: String(normalized[key]) }))
}
