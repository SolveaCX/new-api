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
import { sendChatCompletion } from '@/features/playground/api'
import {
  WEBSITE_LOCALES,
  type LocaleTextMap,
  type WebsiteLocale,
} from './announcement-config'

/** The inexpensive text model used when the caller does not provide one. */
export const ANNOUNCEMENT_TRANSLATION_MODEL = 'gpt-4o-mini'

export type AnnouncementTranslationInput = {
  /** Source announcement copy, normally the English field. */
  content: string
  /** Optional source CTA. Blank values deliberately disable CTA translation. */
  linkLabel?: string
}

export type AnnouncementTranslationOptions = {
  model?: string
  group?: string
  signal?: AbortSignal
}

export type AnnouncementTranslationResult = {
  content: LocaleTextMap
  linkLabel?: LocaleTextMap
}

export type AnnouncementTranslationMessage = {
  role: 'system' | 'user'
  content: string
}

export type AnnouncementTranslationRequest = {
  model: string
  group?: string
  messages: AnnouncementTranslationMessage[]
  stream: false
  temperature: number
  max_tokens: number
}

export type AnnouncementTranslationResponse = {
  choices?: Array<{
    message?: {
      content?: unknown
    }
  }>
}

export type AnnouncementTranslationClient = (
  request: AnnouncementTranslationRequest,
  signal?: AbortSignal
) => Promise<AnnouncementTranslationResponse>

const WEBSITE_LOCALE_LIST = WEBSITE_LOCALES.join(', ')

const SYSTEM_PROMPT = `You are a professional product UI translator.
Translate the supplied announcement into exactly these locales: ${WEBSITE_LOCALE_LIST}.
Return only one JSON object, with no Markdown or explanatory text. The object
must contain a "content" object with exactly one non-empty string for every
listed locale. Preserve URLs, Markdown/HTML syntax, interpolation tokens,
product names, and numbers unless they should naturally be localized.
If and only if the source includes a non-empty "linkLabel", also return a
"linkLabel" object with exactly the same locale keys. Never invent a linkLabel
when the source does not provide one. Do not return any other top-level keys.`

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function hasOwn(object: Record<string, unknown>, key: string): boolean {
  return Object.prototype.hasOwnProperty.call(object, key)
}

/**
 * Normalize input and decide whether the optional CTA should be translated.
 * Keeping this decision in one place prevents an empty source CTA from being
 * accidentally generated or overwriting an existing value.
 */
function normalizeInput(input: AnnouncementTranslationInput): {
  content: string
  linkLabel?: string
  includeLinkLabel: boolean
} {
  if (!isRecord(input) || typeof input.content !== 'string') {
    throw new Error('Announcement content is required')
  }

  const content = input.content.trim()
  if (!content) throw new Error('Announcement content is required')

  if (input.linkLabel !== undefined && typeof input.linkLabel !== 'string') {
    throw new Error('Announcement linkLabel must be a string')
  }

  const linkLabel = input.linkLabel?.trim() ?? ''
  return {
    content,
    ...(linkLabel ? { linkLabel } : {}),
    includeLinkLabel: Boolean(linkLabel),
  }
}

/**
 * Build the two-message request body. Only content and a non-empty linkLabel
 * are included; all other announcement fields stay with the caller.
 */
export function buildAnnouncementTranslationMessages(
  input: AnnouncementTranslationInput
): AnnouncementTranslationMessage[] {
  const normalized = normalizeInput(input)
  const source = {
    content: normalized.content,
    ...(normalized.linkLabel ? { linkLabel: normalized.linkLabel } : {}),
  }

  return [
    { role: 'system', content: SYSTEM_PROMPT },
    {
      role: 'user',
      content: `Translate this source JSON:\n${JSON.stringify(source)}`,
    },
  ]
}

function extractJsonText(value: string): string {
  const trimmed = value.trim()
  if (!trimmed) return trimmed

  // Models commonly wrap the object in a fenced block. Accept an optional
  // language tag (including `application/json`) and ignore surrounding prose.
  const fenced = trimmed.match(
    /```(?:json|application\/json)?\s*([\s\S]*?)\s*```/i
  )
  if (fenced?.[1]) return fenced[1].trim()

  // Be tolerant of a short preamble/trailing sentence when the model omitted
  // a fence. Validation below still rejects anything outside our schema.
  const firstBrace = trimmed.indexOf('{')
  const lastBrace = trimmed.lastIndexOf('}')
  if (firstBrace >= 0 && lastBrace > firstBrace) {
    return trimmed.slice(firstBrace, lastBrace + 1).trim()
  }

  return trimmed
}

function validateLocaleTextMap(
  value: unknown,
  fieldName: 'content' | 'linkLabel'
): LocaleTextMap {
  if (!isRecord(value)) {
    throw new Error(`Announcement translation ${fieldName} must be an object`)
  }

  const supported = new Set<string>(WEBSITE_LOCALES)
  for (const key of Object.keys(value)) {
    if (!supported.has(key)) {
      throw new Error(
        `Announcement translation ${fieldName} contains unsupported locale: ${key}`
      )
    }
  }

  const result = {} as LocaleTextMap
  for (const locale of WEBSITE_LOCALES) {
    if (!hasOwn(value, locale)) {
      throw new Error(
        `Announcement translation ${fieldName} is missing locale: ${locale}`
      )
    }

    const copy = value[locale]
    if (typeof copy !== 'string' || !copy.trim()) {
      throw new Error(
        `Announcement translation ${fieldName}.${locale} must be a non-empty string`
      )
    }
    result[locale] = copy.trim()
  }

  return result
}

/**
 * Parse and strictly validate a completion body. Fenced JSON and a short
 * prose prefix/suffix are accepted, but every supported locale is required.
 */
export function parseAnnouncementTranslation(
  value: string,
  options: { includeLinkLabel: boolean }
): AnnouncementTranslationResult {
  const jsonText = extractJsonText(value)
  let parsed: unknown
  try {
    parsed = JSON.parse(jsonText)
  } catch {
    throw new Error('Announcement translation returned invalid JSON')
  }

  if (!isRecord(parsed)) {
    throw new Error('Announcement translation must be a JSON object')
  }

  // `linkLabel` remains an allowed response key even when it was not
  // requested. We intentionally discard it below so an over-helpful model can
  // never create or overwrite a CTA, while still validating the envelope.
  const allowedKeys = new Set(['content', 'linkLabel'])
  for (const key of Object.keys(parsed)) {
    if (!allowedKeys.has(key)) {
      throw new Error(
        `Announcement translation contains unsupported field: ${key}`
      )
    }
  }

  const content = validateLocaleTextMap(parsed.content, 'content')
  if (!options.includeLinkLabel) {
    // A source without a CTA must never gain one from an over-helpful model.
    return { content }
  }

  if (!hasOwn(parsed, 'linkLabel')) {
    throw new Error('Announcement translation is missing linkLabel')
  }
  const linkLabel = validateLocaleTextMap(parsed.linkLabel, 'linkLabel')
  return { content, linkLabel }
}

function responseContent(
  response: AnnouncementTranslationResponse | null | undefined
): string {
  const content = response?.choices?.[0]?.message?.content
  if (typeof content !== 'string') {
    throw new Error('Announcement translation returned no message content')
  }
  return content
}

/**
 * Translate through an injected client. The production adapter can pass the
 * existing chat-completion function here without coupling this pure helper to
 * transport code; tests can provide a deterministic local client.
 */
export async function translateAnnouncementWithClient(
  input: AnnouncementTranslationInput,
  client: AnnouncementTranslationClient,
  options: AnnouncementTranslationOptions = {}
): Promise<AnnouncementTranslationResult> {
  const normalized = normalizeInput(input)
  const response = await client(
    {
      model: options.model?.trim() || ANNOUNCEMENT_TRANSLATION_MODEL,
      ...(options.group?.trim() ? { group: options.group.trim() } : {}),
      messages: buildAnnouncementTranslationMessages(input),
      stream: false,
      temperature: 0.2,
      max_tokens: 4096,
    },
    options.signal
  )

  return parseAnnouncementTranslation(responseContent(response), {
    includeLinkLabel: normalized.includeLinkLabel,
  })
}

/**
 * Translate announcement copy using the authenticated Playground completion
 * endpoint. The request is deliberately routed through the shared Playground
 * API helper so it keeps the console's cookie/user headers, error handling,
 * and abort semantics. `buildAnnouncementTranslationMessages` limits the
 * payload to the announcement body and optional CTA; metadata such as links,
 * dates, and logos never leaves the console.
 */
export async function translateAnnouncement(
  input: AnnouncementTranslationInput,
  options: AnnouncementTranslationOptions = {}
): Promise<AnnouncementTranslationResult> {
  return translateAnnouncementWithClient(
    input,
    (request, signal) => sendChatCompletion(request, signal),
    options
  )
}

/** Keep the locale types visible to consumers without re-declaring the list. */
export type { LocaleTextMap, WebsiteLocale }
