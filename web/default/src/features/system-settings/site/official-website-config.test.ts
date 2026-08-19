import { describe, expect, test } from 'bun:test'
import {
  OFFICIAL_WEBSITE_LOCALES,
  parseBannerContent,
  serializeBannerContent,
} from './official-website-config'

describe('parseBannerContent', () => {
  test('reads configured locales and drops blank or unknown ones', () => {
    expect(
      parseBannerContent(
        '{"en":"  Launch credits are live.  ","zh":"上线额度已开放。","de":"   ","klingon":"nuqneH","ja":42}'
      )
    ).toEqual({
      en: 'Launch credits are live.',
      zh: '上线额度已开放。',
    })
  })

  test('returns an empty map for unset or malformed values', () => {
    for (const value of ['', '   ', 'not json', '[]', 'null', '"text"']) {
      expect(parseBannerContent(value)).toEqual({})
    }
  })
})

describe('serializeBannerContent', () => {
  test('serializes only the locales that have copy', () => {
    expect(
      serializeBannerContent({
        en: '  Launch credits are live.  ',
        zh: '上线额度已开放。',
        de: '   ',
      })
    ).toBe('{"en":"Launch credits are live.","zh":"上线额度已开放。"}')
  })

  test('serializes an empty form to an empty option value', () => {
    expect(serializeBannerContent({})).toBe('')
    expect(serializeBannerContent({ en: '   ', zh: '' })).toBe('')
  })

  test('round-trips every supported locale', () => {
    const content = Object.fromEntries(
      OFFICIAL_WEBSITE_LOCALES.map((locale) => [locale, `copy-${locale}`])
    )
    expect(parseBannerContent(serializeBannerContent(content))).toEqual(content)
  })
})
