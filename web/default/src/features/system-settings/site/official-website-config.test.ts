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

describe('banner save rules', () => {
  // Regression: the english-required rule used to be attached to the copy
  // field itself, so switching the banner off could not be saved whenever the
  // copy was half-written (e.g. only Chinese filled in).
  const requiresEnglish = (values: {
    enabled: boolean
    content: Record<string, string>
  }) => {
    if (!values.enabled) return false
    const hasAnyCopy = Object.values(values.content).some(
      (copy) => copy.trim().length > 0
    )
    return hasAnyCopy && (values.content.en ?? '').trim().length === 0
  }

  test('lets a disabled banner save regardless of missing english copy', () => {
    expect(
      requiresEnglish({ enabled: false, content: { zh: '上线额度已开放。' } })
    ).toBe(false)
    expect(requiresEnglish({ enabled: false, content: {} })).toBe(false)
  })

  test('still requires english copy while the banner is enabled', () => {
    expect(
      requiresEnglish({ enabled: true, content: { zh: '上线额度已开放。' } })
    ).toBe(true)
    expect(
      requiresEnglish({
        enabled: true,
        content: { en: 'Launch credits are live.', zh: '上线额度已开放。' },
      })
    ).toBe(false)
    // An entirely empty form is fine — the website shows its default banner.
    expect(requiresEnglish({ enabled: true, content: {} })).toBe(false)
  })
})
