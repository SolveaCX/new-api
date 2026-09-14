import { describe, expect, test } from 'bun:test'
import { shouldRequirePhoneBinding } from './phone-binding'

describe('phone binding gate', () => {
  test('requires an unverified PLG user to bind a phone', () => {
    expect(shouldRequirePhoneBinding({ group: 'plg', role: 1 })).toBe(true)
    expect(
      shouldRequirePhoneBinding({
        group: 'plg',
        role: 1,
        phone_number: '+14155550123',
        phone_verified_at: 1,
      })
    ).toBe(false)
  })

  test('does not require binding for other user groups', () => {
    expect(shouldRequirePhoneBinding({ group: 'enterprise', role: 1 })).toBe(
      false
    )
    expect(shouldRequirePhoneBinding(null)).toBe(false)
  })

  test('does not require binding for administrators', () => {
    expect(shouldRequirePhoneBinding({ group: 'plg', role: 10 })).toBe(false)
    expect(shouldRequirePhoneBinding({ group: 'plg', role: 100 })).toBe(false)
  })

  test('does not require binding when SMS verification is disabled', () => {
    expect(shouldRequirePhoneBinding({ group: 'plg', role: 1 }, false)).toBe(
      false
    )
  })
})
