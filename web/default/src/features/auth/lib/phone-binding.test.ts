import { describe, expect, test } from 'bun:test'
import {
  shouldRequirePhoneBinding,
  shouldSuggestPhoneBinding,
} from './phone-binding'

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

  test('suggests binding to every unbound non-admin user', () => {
    const unboundStatus = {
      phone_bound: false,
      phone_number: '',
      phone_verified_at: 0,
      verification_required: false,
      sms_verification_enabled: true,
      rollout_start_at: 0,
    }

    expect(
      shouldSuggestPhoneBinding(
        { role: 1, phone_verification_required: false },
        unboundStatus
      )
    ).toBe(true)
    expect(
      shouldSuggestPhoneBinding(
        { role: 10, phone_verification_required: false },
        unboundStatus
      )
    ).toBe(false)
    expect(
      shouldSuggestPhoneBinding(
        { role: 1, phone_verification_required: false },
        { ...unboundStatus, phone_bound: true }
      )
    ).toBe(false)
  })
})
