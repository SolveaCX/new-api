import { describe, expect, test } from 'bun:test'
import * as z from 'zod'
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

describe('banner form schema', () => {
  // Regression: the form schema used `z.record(z.string(), z.string())`, which
  // rejects locales the operator never typed into (they are `undefined` in the
  // form state). Every language showed "Invalid input" and the whole form —
  // including the on/off switch — could not be saved.
  const contentSchema = z.record(z.string(), z.string().optional())

  test('accepts untouched locales left as undefined', () => {
    const untouched = Object.fromEntries(
      OFFICIAL_WEBSITE_LOCALES.map((locale) => [locale, undefined])
    )
    expect(contentSchema.safeParse(untouched).success).toBe(true)
  })

  test('accepts a form seeded with empty strings for every locale', () => {
    const seeded = Object.fromEntries(
      OFFICIAL_WEBSITE_LOCALES.map((locale) => [locale, ''])
    )
    expect(contentSchema.safeParse(seeded).success).toBe(true)
  })

  test('accepts a partially filled form', () => {
    expect(
      contentSchema.safeParse({
        en: 'Launch credits are live.',
        zh: '上线额度已开放。',
        ja: undefined,
        de: '',
      }).success
    ).toBe(true)
  })

  test('seeding every locale keeps serialization free of blank entries', () => {
    const seeded = Object.fromEntries(
      OFFICIAL_WEBSITE_LOCALES.map((locale) => [locale, ''])
    )
    expect(serializeBannerContent({ ...seeded, en: 'Launch credits.' })).toBe(
      '{"en":"Launch credits."}'
    )
    expect(serializeBannerContent(seeded)).toBe('')
  })
})
