import { describe, expect, test } from 'vitest'
import { buildPhoneNumber } from './phone-verification'

describe('registration phone verification helpers', () => {
  test('combines country dial code and local number into E.164', () => {
    expect(buildPhoneNumber('+86', '138 0013 8000')).toBe('+8613800138000')
  })

  test('rejects a local number without enough digits', () => {
    expect(() => buildPhoneNumber('+1', '555')).toThrow()
  })
})
