import { describe, expect, test } from 'bun:test'
import {
  ANNOUNCEMENT_LOGO_MAX_BYTES,
  BUILT_IN_MODEL_LOGOS,
  WEBSITE_LOCALES,
  announcementToFormValues,
  isSupportedAnnouncementLogo,
  normalizeAnnouncement,
  serializeAnnouncement,
} from './announcement-config'

describe('announcement configuration', () => {
  test('includes the Flatkey brand mark in the built-in logo picker', () => {
    expect(BUILT_IN_MODEL_LOGOS[0]).toEqual({
      label: 'Flatkey',
      value: '/assets/logos/flatkey-mark-white.svg',
    })
  })

  test('accepts safe built-in logo paths and supported image data URLs', () => {
    expect(isSupportedAnnouncementLogo('/assets/logos/openai.svg')).toBe(true)
    expect(isSupportedAnnouncementLogo('/logos/baidu.svg')).toBe(true)
    expect(isSupportedAnnouncementLogo('data:image/png;base64,QUJD')).toBe(true)
    expect(
      isSupportedAnnouncementLogo('data:image/svg+xml;base64,PHN2Zy8+')
    ).toBe(true)
  })

  test('rejects unsafe paths, remote URLs, malformed data, and oversized logos', () => {
    expect(isSupportedAnnouncementLogo('/assets/images/logo.svg')).toBe(false)
    expect(isSupportedAnnouncementLogo('/assets/logos/../secret.svg')).toBe(
      false
    )
    expect(isSupportedAnnouncementLogo('//cdn.example/logo.svg')).toBe(false)
    expect(isSupportedAnnouncementLogo('https://cdn.example/logo.svg')).toBe(
      false
    )
    expect(isSupportedAnnouncementLogo('data:image/png;base64,not valid')).toBe(
      false
    )
    expect(
      isSupportedAnnouncementLogo(
        `data:image/png;base64,${'A'.repeat(ANNOUNCEMENT_LOGO_MAX_BYTES)}`
      )
    ).toBe(false)
  })

  test('normalizes legacy scalar fields without losing localized copy', () => {
    const announcement = normalizeAnnouncement(
      {
        id: 4,
        content: 'Legacy copy',
        extra: 'Legacy extra',
        intro: 'Canonical intro',
        summary: 'Legacy summary',
        link: '/pricing',
        content_i18n: { zh: '中文公告' },
        intro_i18n: { zh: '中文简介' },
        link_label_i18n: { zh: '查看价格' },
        publishDate: '2026-08-27T00:00:00Z',
        type: 'success',
      },
      1
    )

    expect(announcement).not.toBeNull()
    expect(announcement?.content).toBe('Legacy copy')
    expect(announcement?.content_i18n.zh).toBe('中文公告')
    expect(announcement?.content_i18n.en).toBe('Legacy copy')
    expect(announcement?.intro).toBe('Canonical intro')
    expect(announcement?.extra).toBe('Canonical intro')
    expect(announcement?.intro_i18n.zh).toBe('中文简介')
    expect(announcement?.link_label_i18n.zh).toBe('查看价格')
  })

  test('serializes all website locales and keeps non-copy fields unchanged', () => {
    const previous = normalizeAnnouncement(
      {
        id: 9,
        content: 'Old',
        publishDate: '2026-08-27T00:00:00Z',
        type: 'warning',
        link: '/old',
        logo: 'data:image/png;base64,QUJD',
      },
      9
    )
    const values = announcementToFormValues(previous)
    values.content_i18n.en = 'New'
    values.content_i18n.zh = '新公告'
    values.link_label_i18n.en = 'Read more'

    const next = serializeAnnouncement(values, 9, previous)
    expect(Object.keys(next.content_i18n)).toEqual([...WEBSITE_LOCALES])
    expect(next.content).toBe('New')
    expect(next.content_i18n.zh).toBe('新公告')
    expect(next.publishDate).toBe('2026-08-27T00:00:00Z')
    expect(next.type).toBe('warning')
    expect(next.link).toBe('/old')
    expect(next.logo).toBe('data:image/png;base64,QUJD')
  })
})
