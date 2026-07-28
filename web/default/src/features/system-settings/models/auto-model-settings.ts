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
import * as z from 'zod'

export const AUTO_MODEL_CONFIG_KEY = 'auto_model.config'
export const AUTO_MODEL_API_KEY = 'auto_model.classifier_api_key'

export const AUTO_MODEL_ROUTES = [
  'general',
  'coding',
  'reasoning',
  'translation',
] as const

export type AutoModelRoute = (typeof AUTO_MODEL_ROUTES)[number]

export type AutoModelConfig = {
  version: number
  credential_version?: string
  enabled: boolean
  classifier_base_url: string
  classifier_model: string
  classifier_timeout_ms: number
  classifier_input_max_chars: number
  default_model: string
  routes: Record<AutoModelRoute, string[]>
}

export type AutoModelFormValues = AutoModelConfig & {
  classifier_api_key: string
  credential_configured: boolean
}

export const DEFAULT_AUTO_MODEL_CONFIG: AutoModelConfig = {
  version: 1,
  enabled: false,
  classifier_base_url: '',
  classifier_model: '',
  classifier_timeout_ms: 800,
  classifier_input_max_chars: 8000,
  default_model: '',
  routes: {
    general: [],
    coding: [],
    reasoning: [],
    translation: [],
  },
}

const storedConfigSchema = z.object({
  version: z.number().int().nonnegative().optional(),
  credential_version: z.string().optional(),
  enabled: z.boolean().optional(),
  classifier_base_url: z.string().optional(),
  classifier_model: z.string().optional(),
  classifier_timeout_ms: z.number().optional(),
  classifier_input_max_chars: z.number().optional(),
  default_model: z.string().optional(),
  routes: z
    .object({
      general: z.array(z.string()).optional(),
      coding: z.array(z.string()).optional(),
      reasoning: z.array(z.string()).optional(),
      translation: z.array(z.string()).optional(),
    })
    .optional(),
})

const uniqueModels = (routes: Record<AutoModelRoute, string[]>) =>
  new Set(AUTO_MODEL_ROUTES.flatMap((route) => routes[route]))

function isHttpsUrl(value: string) {
  try {
    return new URL(value).protocol === 'https:'
  } catch {
    return false
  }
}

export const autoModelFormSchema = z
  .object({
    version: z.number().int().positive(),
    credential_version: z.string().optional(),
    enabled: z.boolean(),
    classifier_base_url: z.string(),
    classifier_model: z.string(),
    classifier_timeout_ms: z
      .number({
        error: 'Classifier timeout must be between 200 and 2000 ms',
      })
      .int()
      .min(200, 'Classifier timeout must be between 200 and 2000 ms')
      .max(2000, 'Classifier timeout must be between 200 and 2000 ms'),
    classifier_input_max_chars: z
      .number({
        error:
          'Maximum classifier input characters must be between 1000 and 32000',
      })
      .int()
      .min(
        1000,
        'Maximum classifier input characters must be between 1000 and 32000'
      )
      .max(
        32000,
        'Maximum classifier input characters must be between 1000 and 32000'
      ),
    default_model: z.string(),
    classifier_api_key: z.string(),
    credential_configured: z.boolean(),
    routes: z.object({
      general: z.array(z.string()),
      coding: z.array(z.string()),
      reasoning: z.array(z.string()),
      translation: z.array(z.string()),
    }),
  })
  .superRefine((values, context) => {
    if (!values.enabled) return

    if (!isHttpsUrl(values.classifier_base_url.trim())) {
      context.addIssue({
        code: 'custom',
        path: ['classifier_base_url'],
        message: 'Classifier Base URL must use HTTPS',
      })
    }
    if (!values.classifier_model.trim()) {
      context.addIssue({
        code: 'custom',
        path: ['classifier_model'],
        message: 'Classifier model is required',
      })
    }
    if (!values.credential_configured && !values.classifier_api_key.trim()) {
      context.addIssue({
        code: 'custom',
        path: ['classifier_api_key'],
        message: 'Classifier API key is required when enabling Auto Model',
      })
    }

    for (const route of AUTO_MODEL_ROUTES) {
      if (values.routes[route].length === 0) {
        context.addIssue({
          code: 'custom',
          path: ['routes', route],
          message: 'Each route must contain at least one model',
        })
      }
    }

    const candidates = uniqueModels(values.routes)
    if (candidates.size < 5 || candidates.size > 10) {
      context.addIssue({
        code: 'custom',
        path: ['routes'],
        message: 'Configure 5 to 10 unique candidate models',
      })
    }
    if (!candidates.has(values.default_model.trim())) {
      context.addIssue({
        code: 'custom',
        path: ['default_model'],
        message: 'Default model must be one of the candidate models',
      })
    }
    if (candidates.has('auto')) {
      context.addIssue({
        code: 'custom',
        path: ['routes'],
        message: 'The virtual auto model cannot be a candidate',
      })
    }
  })

function normalizeModels(models: string[] | undefined) {
  return Array.from(
    new Set((models ?? []).map((model) => model.trim()).filter(Boolean))
  )
}

export function parseAutoModelConfig(value: string): AutoModelConfig {
  if (!value.trim()) return DEFAULT_AUTO_MODEL_CONFIG

  try {
    const parsed = storedConfigSchema.safeParse(JSON.parse(value))
    if (!parsed.success) return DEFAULT_AUTO_MODEL_CONFIG
    const config = parsed.data
    return {
      version: config.version ?? DEFAULT_AUTO_MODEL_CONFIG.version,
      credential_version: config.credential_version,
      enabled: config.enabled ?? DEFAULT_AUTO_MODEL_CONFIG.enabled,
      classifier_base_url:
        config.classifier_base_url ??
        DEFAULT_AUTO_MODEL_CONFIG.classifier_base_url,
      classifier_model:
        config.classifier_model ?? DEFAULT_AUTO_MODEL_CONFIG.classifier_model,
      classifier_timeout_ms:
        config.classifier_timeout_ms ??
        DEFAULT_AUTO_MODEL_CONFIG.classifier_timeout_ms,
      classifier_input_max_chars:
        config.classifier_input_max_chars ??
        DEFAULT_AUTO_MODEL_CONFIG.classifier_input_max_chars,
      default_model:
        config.default_model ?? DEFAULT_AUTO_MODEL_CONFIG.default_model,
      routes: {
        general: normalizeModels(config.routes?.general),
        coding: normalizeModels(config.routes?.coding),
        reasoning: normalizeModels(config.routes?.reasoning),
        translation: normalizeModels(config.routes?.translation),
      },
    }
  } catch {
    return DEFAULT_AUTO_MODEL_CONFIG
  }
}

export function buildAutoModelOptions(values: AutoModelFormValues) {
  const config: AutoModelConfig = {
    version: values.version,
    credential_version: values.credential_version,
    enabled: values.enabled,
    classifier_base_url: values.classifier_base_url.trim(),
    classifier_model: values.classifier_model.trim(),
    classifier_timeout_ms: values.classifier_timeout_ms,
    classifier_input_max_chars: values.classifier_input_max_chars,
    default_model: values.default_model.trim(),
    routes: {
      general: normalizeModels(values.routes.general),
      coding: normalizeModels(values.routes.coding),
      reasoning: normalizeModels(values.routes.reasoning),
      translation: normalizeModels(values.routes.translation),
    },
  }
  const options = [
    { key: AUTO_MODEL_CONFIG_KEY, value: JSON.stringify(config) },
  ]
  const apiKey = values.classifier_api_key.trim()
  if (apiKey) {
    options.push({
      key: AUTO_MODEL_API_KEY,
      value: apiKey,
    })
  }
  return options
}
