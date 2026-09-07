import { describe, expect, test } from 'bun:test'
import { shouldRequirePhoneBinding } from './phone-binding'

describe('phone binding gate', () => {
  test('requires an unverified PLG user to bind a phone', () => {
    expect(shouldRequirePhoneBinding({ group: 'plg' })).toBe(true)
    expect(
      shouldRequirePhoneBinding({
        group: 'plg',
        phone_number: '+14155550123',
        phone_verified_at: 1,
      })
    ).toBe(false)
  })

  test('does not require binding for other user groups', () => {
    expect(shouldRequirePhoneBinding({ group: 'enterprise' })).toBe(false)
    expect(shouldRequirePhoneBinding(null)).toBe(false)
  })
})
