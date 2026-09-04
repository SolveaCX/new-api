import { describe, expect, test } from 'bun:test'
import { getTallyEmbedUrl, getTallyFormId, getTallyFormUrl } from './tally'

describe('Tally form routing', () => {
  test('uses dedicated forms for Chinese and Japanese', () => {
    expect(getTallyFormId('zh-CN')).toBe('BzRylN')
    expect(getTallyFormId('ja-JP')).toBe('vG9Z1Q')
  })

  test('uses the default form for other languages', () => {
    expect(getTallyFormId('en')).toBe('1A6gM4')
    expect(getTallyFormUrl('fr')).toBe('https://tally.so/r/1A6gM4')
  })

  test('preserves the wallet origin page on embed URLs', () => {
    expect(getTallyEmbedUrl('zh', '/contact')).toContain(
      'https://tally.so/embed/BzRylN?'
    )
    expect(getTallyEmbedUrl('zh', '/contact')).toContain(
      '&originPage=%2Fcontact'
    )
  })
})
