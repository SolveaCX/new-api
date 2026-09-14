import { describe, expect, test } from 'bun:test'
import { shouldRequirePhoneBinding } from './phone-binding'

describe('phone binding gate', () => {
  test('follows the backend decision', () => {
    expect(
      shouldRequirePhoneBinding({ phone_verification_required: true })
    ).toBe(true)
    expect(
      shouldRequirePhoneBinding({ phone_verification_required: false })
    ).toBe(false)
  })

  test('treats a missing field as unknown and never opens the dialog', () => {
    expect(shouldRequirePhoneBinding({})).toBe(false)
    expect(shouldRequirePhoneBinding(null)).toBe(false)
    expect(shouldRequirePhoneBinding(undefined)).toBe(false)
  })

  test('does not require binding when SMS verification is disabled', () => {
    expect(
      shouldRequirePhoneBinding({ phone_verification_required: true }, false)
    ).toBe(false)
  })
})
