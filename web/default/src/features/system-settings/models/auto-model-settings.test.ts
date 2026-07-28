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
  AUTO_MODEL_API_KEY,
  AUTO_MODEL_CONFIG_KEY,
  autoModelFormSchema,
  buildAutoModelOptions,
  parseAutoModelConfig,
  type AutoModelFormValues,
} from './auto-model-settings'

const validValues: AutoModelFormValues = {
  version: 1,
  enabled: true,
  classifier_base_url: 'https://classifier.example.com/v1',
  classifier_model: 'classifier-mini',
  classifier_timeout_ms: 800,
  classifier_input_max_chars: 8000,
  classifier_api_key: '',
  credential_configured: true,
  default_model: 'model-a',
  routes: {
    general: ['model-a', 'model-b'],
    coding: ['model-c', 'model-a'],
    reasoning: ['model-d'],
    translation: ['model-e'],
  },
}

describe('Auto Model settings', () => {
  test('parses stored config and normalizes route models', () => {
    const config = parseAutoModelConfig(
      JSON.stringify({
        ...validValues,
        credential_version: '0123456789abcdef0123456789abcdef',
        routes: { ...validValues.routes, general: [' model-a ', 'model-a'] },
      })
    )

    expect(config.credential_version).toBe('0123456789abcdef0123456789abcdef')
    expect(config.routes.general).toEqual(['model-a'])
  })

  test('requires five to ten unique candidates when enabled', () => {
    const result = autoModelFormSchema.safeParse({
      ...validValues,
      routes: {
        general: ['model-a'],
        coding: ['model-a'],
        reasoning: ['model-a'],
        translation: ['model-a'],
      },
    })

    expect(result.success).toBe(false)
  })

  test('matches the backend classifier input character limits', () => {
    expect(
      autoModelFormSchema.safeParse({
        ...validValues,
        classifier_input_max_chars: 999,
      }).success
    ).toBe(false)
    expect(
      autoModelFormSchema.safeParse({
        ...validValues,
        classifier_input_max_chars: 32000,
      }).success
    ).toBe(true)
  })

  test('requires a new key when enabling without an existing credential', () => {
    const result = autoModelFormSchema.safeParse({
      ...validValues,
      credential_configured: false,
    })

    expect(result.success).toBe(false)
  })

  test('omits the write-only key when no replacement was entered', () => {
    const options = buildAutoModelOptions(validValues)

    expect(options.map((option) => option.key)).toEqual([AUTO_MODEL_CONFIG_KEY])
  })

  test('submits only the exact config and secret option keys', () => {
    const credentialVersion = '0123456789abcdef0123456789abcdef'
    const options = buildAutoModelOptions({
      ...validValues,
      credential_version: credentialVersion,
      classifier_api_key: 'new-secret',
    })

    expect(options.map((option) => option.key)).toEqual([
      AUTO_MODEL_CONFIG_KEY,
      AUTO_MODEL_API_KEY,
    ])
    expect(JSON.parse(options[0].value).credential_version).toBe(
      credentialVersion
    )
    expect(options[1].value).toBe('new-secret')
  })
})
