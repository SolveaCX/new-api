/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/
import { describe, expect, test } from 'bun:test'
import { WEBSITE_LOCALES } from './announcement-config'
import {
  buildAnnouncementTranslationMessages,
  parseAnnouncementTranslation,
  translateAnnouncementWithClient,
  type AnnouncementTranslationResponse,
} from './announcement-translation'

function localeMap(prefix: string): Record<string, string> {
  return Object.fromEntries(
    WEBSITE_LOCALES.map((locale) => [locale, `${prefix}-${locale}`])
  )
}

function response(body: unknown): AnnouncementTranslationResponse {
  return {
    choices: [{ message: { content: String(body) } }],
  }
}

describe('buildAnnouncementTranslationMessages', () => {
  test('sends only copy fields and lists every website locale', () => {
    const messages = buildAnnouncementTranslationMessages({
      content: '  New models are live.  ',
      linkLabel: '  Learn more  ',
    })

    expect(messages).toHaveLength(2)
    expect(messages[0]?.role).toBe('system')
    for (const locale of WEBSITE_LOCALES) {
      expect(messages[0]?.content).toContain(locale)
    }
    expect(messages[1]?.content).toContain(
      '{"content":"New models are live.","linkLabel":"Learn more"}'
    )
    expect(messages[1]?.content).not.toContain('publishDate')
    expect(messages[1]?.content).not.toContain('logo')
    expect(messages[1]?.content).not.toContain('link":')
  })

  test('omits an empty source linkLabel so it cannot be generated', () => {
    const messages = buildAnnouncementTranslationMessages({
      content: 'New models are live.',
      linkLabel: '   ',
    })

    expect(messages[1]?.content).toContain('{"content":"New models are live."}')
    expect(messages[1]?.content).not.toContain('linkLabel')
  })
})

describe('parseAnnouncementTranslation', () => {
  test('accepts fenced JSON and trims translated values', () => {
    const body = JSON.stringify({
      content: localeMap('copy'),
      linkLabel: localeMap('cta'),
    })

    expect(
      parseAnnouncementTranslation(
        `Here you go:\n\n\`\`\`json\n${body}\n\`\`\``,
        {
          includeLinkLabel: true,
        }
      )
    ).toEqual({
      content: localeMap('copy'),
      linkLabel: localeMap('cta'),
    })
  })

  test('requires every locale and rejects unknown locales', () => {
    const incomplete = localeMap('copy')
    delete incomplete.id
    expect(() =>
      parseAnnouncementTranslation(JSON.stringify({ content: incomplete }), {
        includeLinkLabel: false,
      })
    ).toThrow('missing locale: id')

    expect(() =>
      parseAnnouncementTranslation(
        JSON.stringify({ content: { ...localeMap('copy'), klingon: 'x' } }),
        { includeLinkLabel: false }
      )
    ).toThrow('unsupported locale: klingon')
  })

  test('does not return a model-generated linkLabel when source was blank', () => {
    const parsed = parseAnnouncementTranslation(
      JSON.stringify({
        content: localeMap('copy'),
        linkLabel: localeMap('should-not-be-applied'),
      }),
      { includeLinkLabel: false }
    )

    expect(parsed).toEqual({ content: localeMap('copy') })
    expect(parsed).not.toHaveProperty('linkLabel')
  })

  test('requires a complete linkLabel map when source supplied one', () => {
    const linkLabel = localeMap('cta')
    delete linkLabel.de

    expect(() =>
      parseAnnouncementTranslation(
        JSON.stringify({ content: localeMap('copy'), linkLabel }),
        { includeLinkLabel: true }
      )
    ).toThrow('linkLabel is missing locale: de')
  })
})

describe('translateAnnouncementWithClient', () => {
  test('uses one injected completion and returns only translated copy fields', async () => {
    let calls = 0
    const result = await translateAnnouncementWithClient(
      { content: 'New models are live.', linkLabel: 'Learn more' },
      async (request) => {
        calls += 1
        expect(request.model).toBe('gpt-4o-mini')
        expect(request.stream).toBe(false)
        expect(request.messages).toHaveLength(2)
        return response(
          `\`\`\`json\n${JSON.stringify({
            content: localeMap('copy'),
            linkLabel: localeMap('cta'),
          })}\n\`\`\``
        )
      }
    )

    expect(calls).toBe(1)
    expect(result.content).toEqual(localeMap('copy'))
    expect(result.linkLabel).toEqual(localeMap('cta'))
  })

  test('does not overwrite linkLabel when source is empty', async () => {
    const result = await translateAnnouncementWithClient(
      { content: 'New models are live.', linkLabel: '' },
      async (request) => {
        expect(request.messages[1]?.content).not.toContain('linkLabel')
        return response(
          JSON.stringify({
            content: localeMap('copy'),
            linkLabel: localeMap('model-generated'),
          })
        )
      }
    )

    expect(result).toEqual({ content: localeMap('copy') })
  })
})
