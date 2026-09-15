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
import { afterEach, beforeEach, describe, expect, test } from 'bun:test'
import {
  hasDismissedPhoneBindingSuggestion,
  markPhoneBindingSuggestionDismissed,
} from './phone-binding-dismissal'

const USER_A = 101
const USER_B = 202

const originalWindow = globalThis.window

function installWindowStorage(overrides?: {
  getItem?: (key: string) => string | null
  setItem?: (key: string, value: string) => void
}) {
  const values = new Map<string, string>()
  const localStorage = {
    getItem: overrides?.getItem ?? ((key: string) => values.get(key) ?? null),
    removeItem: (key: string) => {
      values.delete(key)
    },
    setItem:
      overrides?.setItem ??
      ((key: string, value: string) => {
        values.set(key, value)
      }),
    values,
  }

  Object.defineProperty(globalThis, 'window', {
    configurable: true,
    value: { localStorage },
  })
  return localStorage
}

beforeEach(() => {
  installWindowStorage()
})

afterEach(() => {
  Object.defineProperty(globalThis, 'window', {
    configurable: true,
    value: originalWindow,
  })
})

describe('phone binding suggestion dismissal', () => {
  test('not dismissed before the user closes the prompt', () => {
    expect(hasDismissedPhoneBindingSuggestion(USER_A)).toBe(false)
  })

  test('a dismissal survives the next reload', () => {
    markPhoneBindingSuggestionDismissed(USER_A)
    expect(hasDismissedPhoneBindingSuggestion(USER_A)).toBe(true)
  })

  test('the flag is scoped per user on a shared browser', () => {
    markPhoneBindingSuggestionDismissed(USER_A)
    expect(hasDismissedPhoneBindingSuggestion(USER_B)).toBe(false)
  })

  test('a missing user id never writes a shared global flag', () => {
    const storage = installWindowStorage()
    markPhoneBindingSuggestionDismissed(null)
    markPhoneBindingSuggestionDismissed(undefined)
    expect(storage.values.size).toBe(0)
    expect(hasDismissedPhoneBindingSuggestion(null)).toBe(false)
    expect(hasDismissedPhoneBindingSuggestion(undefined)).toBe(false)
  })

  test('a throwing storage reads as not dismissed instead of crashing', () => {
    installWindowStorage({
      getItem: () => {
        throw new Error('storage disabled')
      },
      setItem: () => {
        throw new Error('storage disabled')
      },
    })

    expect(() => markPhoneBindingSuggestionDismissed(USER_A)).not.toThrow()
    expect(hasDismissedPhoneBindingSuggestion(USER_A)).toBe(false)
  })
})
