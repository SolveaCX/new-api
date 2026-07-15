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
import {
  resolveRegistrationEmailVerification,
  startRegistrationEmailVerificationEffect,
  type EmailVerificationScreenState,
} from '../../lib/registration-email-verification'
import { EmailVerificationStatusContent } from './email-verification-screen'

const testI18n = createInstance()

beforeAll(async () => {
  await testI18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })
})

function renderState(state: EmailVerificationScreenState): string {
  return renderToStaticMarkup(
    <I18nextProvider i18n={testI18n}>
      <EmailVerificationStatusContent state={state} />
    </I18nextProvider>
  )
}

describe('EmailVerificationStatusContent', () => {
  test('renders progress, success, and unavailable states', () => {
    expect(renderState('verifying')).toContain('Verifying your email')
    expect(renderState('verified')).toContain('Email verified')
    expect(renderState('unavailable')).toContain(
      'Verification link unavailable'
    )
  })
})

describe('resolveRegistrationEmailVerification', () => {
  test('does not exchange a missing token', async () => {
    let exchangeCalls = 0
    const state = await resolveRegistrationEmailVerification(null, {
      exchangeToken: async () => {
        exchangeCalls += 1
        return { success: true, message: '', data: { verified: true } }
      },
    })

    expect(state).toBe('unavailable')
    expect(exchangeCalls).toBe(0)
  })

  test('exchanges the bootstrapped token and maps success to verified', async () => {
    let finishExchange: ((value: unknown) => void) | undefined
    const exchangeResponse = new Promise((resolve) => {
      finishExchange = resolve
    })

    const statePromise = resolveRegistrationEmailVerification('abc', {
      exchangeToken: async () => exchangeResponse,
    })

    finishExchange?.({
      success: true,
      message: '',
      data: { verified: true },
    })
    expect(await statePromise).toBe('verified')
  })

  test('maps business and network failures to unavailable', async () => {
    const businessFailure = await resolveRegistrationEmailVerification(
      'expired',
      {
        exchangeToken: async () => ({
          success: false,
          message: 'expired',
        }),
      }
    )
    const networkFailure = await resolveRegistrationEmailVerification(
      'network',
      {
        exchangeToken: async () => {
          throw new Error('offline')
        },
      }
    )

    expect(businessFailure).toBe('unavailable')
    expect(networkFailure).toBe('unavailable')
  })

  test('lets the active StrictMode effect publish after the first cleanup', async () => {
    const resolvers: Array<(value: unknown) => void> = []
    const states: EmailVerificationScreenState[] = []
    const dependencies = {
      exchangeToken: () =>
        new Promise((resolve) => {
          resolvers.push(resolve)
        }),
    }

    const cleanupFirst = startRegistrationEmailVerificationEffect(
      'abc',
      dependencies,
      (state) => states.push(state)
    )
    cleanupFirst()
    startRegistrationEmailVerificationEffect('abc', dependencies, (state) =>
      states.push(state)
    )

    for (const resolve of resolvers) {
      resolve({ success: true, message: '', data: { verified: true } })
    }
    await Promise.resolve()
    await Promise.resolve()

    expect(states).toEqual(['verified'])
  })
})
