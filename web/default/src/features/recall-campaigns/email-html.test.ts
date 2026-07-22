import { describe, expect, test } from 'bun:test'
import {
  RECALL_EMAIL_ACTIONS,
  RECALL_EMAIL_STARTER_HTML,
  convertRecallBodyTextToHtml,
  insertRecallEmailAction,
} from './email-html'

describe('recall email HTML helpers', () => {
  test('exports the required recall email actions', () => {
    expect(RECALL_EMAIL_ACTIONS).toEqual([
      '{{.RecipientName}}',
      '{{.PromotionCodeMasked}}',
      '{{.ProductSummary}}',
      '{{.ExpiresAt}}',
      '{{.ClaimURL}}',
      '{{.UnsubscribeURL}}',
    ])
  })

  test('provides editable starter HTML with required action links', () => {
    expect(RECALL_EMAIL_STARTER_HTML).toContain('https://example.com')
    expect(RECALL_EMAIL_STARTER_HTML).toContain('href="{{.ClaimURL}}"')
    expect(RECALL_EMAIL_STARTER_HTML).toContain('href="{{.UnsubscribeURL}}"')
  })

  test('converts legacy text paragraphs into escaped editable HTML', () => {
    const html = convertRecallBodyTextToHtml(
      'Hello\r\nSecond line\r\n\r\n<>&"\''
    )

    expect(html).toContain('<p>Hello</p>')
    expect(html).toContain('<p>Second line</p>')
    expect(html).toContain('&lt;&gt;&amp;&quot;&#39;')
    expect(html).toContain('{{.ClaimURL}}')
    expect(html).toContain('{{.UnsubscribeURL}}')
  })

  test('inserts recall actions at normalized selections', () => {
    expect(insertRecallEmailAction('abc', 1, 2, '{{.ClaimURL}}')).toEqual({
      value: 'a{{.ClaimURL}}c',
      selection: 14,
    })
    expect(insertRecallEmailAction('abc', 5, -1, '{{.ClaimURL}}')).toEqual({
      value: '{{.ClaimURL}}',
      selection: 13,
    })
  })
})
