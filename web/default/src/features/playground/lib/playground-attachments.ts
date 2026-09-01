/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/
import {
  completePlaygroundAttachmentUpload,
  createPlaygroundAttachmentUploadSession,
  getPlaygroundAttachmentPreview,
  type PlaygroundAttachmentPreview,
} from '../api'
import type { GeneratedMedia, Message, PlaygroundAttachment } from '../types'
import {
  MAX_ATTACHMENTS,
  MAX_FILE_BYTES,
  normalizePlaygroundMediaType,
} from './attachments'
import { markTrustedAttachmentURL } from './message-utils'

const DATA_URL_PATTERN = /^data:([^;,\s]+);base64,([a-z0-9+/=\r\n]*)$/i
export function isPlaygroundAttachmentUploadAvailable(): boolean {
  return (
    typeof window !== 'undefined' &&
    !!window.location &&
    typeof window.location.origin === 'string'
  )
}

function isMediaAttachment(
  attachment: PlaygroundAttachment
): attachment is PlaygroundAttachment & {
  kind: 'image' | 'video' | 'audio' | 'document'
} {
  return (
    attachment.kind === 'image' ||
    attachment.kind === 'video' ||
    attachment.kind === 'audio' ||
    attachment.kind === 'document'
  )
}

function isBase64DataURL(value: unknown): boolean {
  return (
    typeof value === 'string' && value.trim().toLowerCase().startsWith('data:')
  )
}

/** Decode a normalized browser data URL into a Blob for direct object upload. */
export function playgroundDataURLToBlob(value: string): Blob {
  const match = DATA_URL_PATTERN.exec(value.trim())
  if (!match) throw new Error('Attachment data is invalid')

  const encoded = match[2].replace(/[\r\n]/g, '')
  const estimatedBytes =
    Math.floor((encoded.length * 3) / 4) -
    (encoded.endsWith('==') ? 2 : encoded.endsWith('=') ? 1 : 0)
  if (estimatedBytes <= 0 || estimatedBytes > MAX_FILE_BYTES) {
    throw new Error('Attachment exceeds the maximum size')
  }

  let decoded: string
  try {
    decoded = atob(encoded)
  } catch {
    throw new Error('Attachment data is invalid')
  }
  const bytes = Uint8Array.from(decoded, (character) => character.charCodeAt(0))
  if (bytes.length === 0 || bytes.length > MAX_FILE_BYTES) {
    throw new Error('Attachment exceeds the maximum size')
  }
  return new Blob([bytes], {
    type: normalizePlaygroundMediaType(match[1]),
  })
}

async function attachmentToBlob(
  attachment: PlaygroundAttachment,
  signal?: AbortSignal
): Promise<Blob> {
  const url = attachment.url?.trim()
  if (!url) throw new Error(`Attachment ${attachment.filename} has no data`)
  if (isBase64DataURL(url)) return playgroundDataURLToBlob(url)

  // Blob URLs are emitted by PromptInput before it converts them to data URLs;
  // accepting them here also keeps this helper usable by programmatic callers.
  if (url.startsWith('blob:')) {
    const response = await fetch(url, { signal })
    if (!response.ok)
      throw new Error(`Unable to read attachment ${attachment.filename}`)
    const blob = await response.blob()
    if (blob.size <= 0 || blob.size > MAX_FILE_BYTES) {
      throw new Error('Attachment exceeds the maximum size')
    }
    return blob
  }

  throw new Error('Attachment must be uploaded from local data')
}

function expectedUploadAssetType(
  kind: 'image' | 'video' | 'audio' | 'document'
): 'Image' | 'Video' | 'Audio' | 'Document' {
  switch (kind) {
    case 'image':
      return 'Image'
    case 'video':
      return 'Video'
    case 'audio':
      return 'Audio'
    case 'document':
      return 'Document'
  }
}

function contentTypeFor(attachment: PlaygroundAttachment, blob: Blob): string {
  return normalizePlaygroundMediaType(attachment.mediaType || blob.type || '')
}

async function blobToDataUrl(blob: Blob): Promise<string> {
  if (blob.size <= 0 || blob.size > MAX_FILE_BYTES) {
    throw new Error('Attachment exceeds the maximum size')
  }
  const bytes = new Uint8Array(await blob.arrayBuffer())
  let binary = ''
  const chunkSize = 0x8000
  for (let index = 0; index < bytes.length; index += chunkSize) {
    binary += String.fromCharCode(...bytes.subarray(index, index + chunkSize))
  }
  return `data:${normalizePlaygroundMediaType(
    blob.type || 'application/octet-stream'
  )};base64,${btoa(binary)}`
}

function assertPreviewMatchesAttachment(
  attachment: PlaygroundAttachment & {
    kind: 'image' | 'video' | 'audio' | 'document'
  },
  preview: PlaygroundAttachmentPreview,
  expectedAssetId?: string
): void {
  const previewAssetId = preview.asset_id?.trim()
  const previewURL = preview.preview_url?.trim()
  if (!previewAssetId || !previewURL) {
    throw new Error('Invalid Playground attachment preview response')
  }
  if (expectedAssetId?.trim() && previewAssetId !== expectedAssetId.trim()) {
    throw new Error('Playground attachment preview mismatch')
  }
  const expected = expectedUploadAssetType(attachment.kind)
  if (
    preview.asset_type &&
    preview.asset_type.toLowerCase() !== expected.toLowerCase()
  ) {
    throw new Error('Playground attachment type mismatch')
  }
}

function assertPreviewMatchesGeneratedMedia(
  media: GeneratedMedia,
  preview: PlaygroundAttachmentPreview,
  expectedAssetId?: string
): void {
  const previewAssetId = preview.asset_id?.trim()
  const previewURL = preview.preview_url?.trim()
  if (!previewAssetId || !previewURL) {
    throw new Error('Invalid Playground generated media preview response')
  }
  if (expectedAssetId?.trim() && previewAssetId !== expectedAssetId.trim()) {
    throw new Error('Playground generated media preview mismatch')
  }
  let expected: 'Image' | 'Video' | 'Audio'
  switch (media.type) {
    case 'image':
      expected = 'Image'
      break
    case 'video':
      expected = 'Video'
      break
    case 'audio':
      expected = 'Audio'
      break
    default:
      throw new Error('Unsupported Playground generated media type')
  }
  if (
    preview.asset_type &&
    preview.asset_type.toLowerCase() !== expected.toLowerCase()
  ) {
    throw new Error('Playground generated media type mismatch')
  }
}

function hydratedGeneratedMedia(
  media: GeneratedMedia,
  preview: PlaygroundAttachmentPreview
): GeneratedMedia {
  return {
    ...media,
    assetId: preview.asset_id.trim(),
    url: preview.preview_url.trim(),
    ...(preview.content_type
      ? { mimeType: preview.content_type }
      : media.mimeType
        ? { mimeType: media.mimeType }
        : {}),
  }
}

async function hydrateGeneratedMediaList(
  media: GeneratedMedia[] | undefined,
  previewFor: (assetId: string) => Promise<PlaygroundAttachmentPreview>
): Promise<{ media: GeneratedMedia[] | undefined; changed: boolean }> {
  if (!media?.length) return { media, changed: false }

  let changed = false
  const hydrated = await Promise.all(
    media.map(async (item) => {
      if (!item.assetId?.trim()) return item
      try {
        const assetId = item.assetId.trim()
        const preview = await previewFor(assetId)
        assertPreviewMatchesGeneratedMedia(item, preview, assetId)
        const next = hydratedGeneratedMedia(item, preview)
        if (
          next.assetId !== item.assetId ||
          next.url !== item.url ||
          next.mimeType !== item.mimeType
        ) {
          changed = true
        }
        return next
      } catch {
        // Keep the durable ID, but never revive a stale signed or process-local
        // URL after a failed preview lookup.
        if (item.url) {
          changed = true
          const next = { ...item }
          delete next.url
          return next
        }
        return item
      }
    })
  )

  return { media: hydrated, changed }
}

async function putPlaygroundAttachment(
  uploadURL: string,
  headers: Record<string, string> | undefined,
  blob: Blob,
  signal?: AbortSignal
): Promise<void> {
  let response: Response
  try {
    response = await fetch(uploadURL, {
      method: 'PUT',
      headers: headers ?? {},
      body: blob,
      credentials: 'omit',
      signal,
    })
  } catch {
    throw new Error('Unable to upload Playground attachment')
  }
  if (!response.ok) {
    let detail = ''
    try {
      detail = (await response.text()).trim()
    } catch {
      // Ignore a non-text storage error body.
    }
    throw new Error(detail || 'Unable to upload Playground attachment')
  }
}

/**
 * Upload local media attachments and replace transient URLs with durable asset
 * IDs plus a short-lived signed preview URL. Generated audio reuses this same
 * path so its output can be restored after a reload.
 */
export async function uploadPlaygroundAttachments(
  attachments: PlaygroundAttachment[],
  signal?: AbortSignal
): Promise<PlaygroundAttachment[]> {
  if (attachments.length > MAX_ATTACHMENTS)
    throw new Error('Too many attachments')

  // Playground is client-only. Keep static-render/test environments
  // side-effect free; a real browser always exposes a location object and
  // therefore takes the durable upload path below.
  if (!isPlaygroundAttachmentUploadAvailable()) {
    return attachments
  }

  const uploaded: PlaygroundAttachment[] = []
  const previews = new Map<string, Promise<PlaygroundAttachmentPreview>>()
  const previewFor = (assetId: string) => {
    let request = previews.get(assetId)
    if (!request) {
      request = getPlaygroundAttachmentPreview(assetId, signal)
      previews.set(assetId, request)
    }
    return request
  }
  for (const attachment of attachments) {
    if (signal?.aborted) throw new DOMException('Aborted', 'AbortError')
    if (!isMediaAttachment(attachment)) {
      uploaded.push({ ...attachment })
      continue
    }

    // Asset IDs are durable, while every preview URL is deliberately
    // short-lived. Refresh by ID on every send/replay path so a stale URL can
    // never be submitted to a media provider after a long-lived conversation.
    const assetId = attachment.assetId?.trim()
    if (assetId) {
      const preview = await previewFor(assetId)
      assertPreviewMatchesAttachment(attachment, preview, assetId)
      uploaded.push({
        ...attachment,
        assetId: preview.asset_id.trim(),
        url: preview.preview_url.trim(),
        mediaType: preview.content_type || attachment.mediaType,
        ...(attachment.kind === 'audio' && attachment.dataUrl
          ? { dataUrl: attachment.dataUrl }
          : {}),
      })
      markTrustedAttachmentURL(uploaded[uploaded.length - 1]!)
      continue
    }

    const blob = await attachmentToBlob(attachment, signal)
    const contentType = contentTypeFor(attachment, blob)
    const session = await createPlaygroundAttachmentUploadSession(
      {
        assetType: expectedUploadAssetType(attachment.kind),
        contentType,
        sizeBytes: blob.size,
      },
      signal
    )
    await putPlaygroundAttachment(
      session.upload_url,
      session.upload_headers,
      blob,
      signal
    )
    const preview = await completePlaygroundAttachmentUpload(
      session.upload_id,
      signal
    )
    assertPreviewMatchesAttachment(attachment, preview, session.asset_id)
    uploaded.push({
      ...attachment,
      assetId: preview.asset_id.trim(),
      url: preview.preview_url.trim(),
      mediaType: preview.content_type || contentType || attachment.mediaType,
      ...(attachment.kind === 'audio' && attachment.url
        ? { dataUrl: attachment.url }
        : {}),
    })
    markTrustedAttachmentURL(uploaded[uploaded.length - 1]!)
  }
  return uploaded
}

function hydratedAttachment(
  attachment: PlaygroundAttachment,
  preview: PlaygroundAttachmentPreview
): PlaygroundAttachment {
  return {
    ...attachment,
    assetId: preview.asset_id,
    url: preview.preview_url,
    mediaType: preview.content_type || attachment.mediaType,
    ...(attachment.kind === 'audio' && attachment.dataUrl
      ? { dataUrl: attachment.dataUrl }
      : {}),
  }
}

async function hydrateAudioAttachment(
  attachment: PlaygroundAttachment,
  preview: PlaygroundAttachmentPreview
): Promise<PlaygroundAttachment> {
  const hydrated = markTrustedAttachmentURL(
    hydratedAttachment(attachment, preview)
  )
  if (hydrated.dataUrl) return hydrated

  try {
    const response = await fetch(preview.preview_url)
    if (!response.ok) return hydrated

    const dataUrl = await blobToDataUrl(await response.blob())
    return markTrustedAttachmentURL({ ...hydrated, dataUrl })
  } catch {
    return hydrated
  }
}

/**
 * Resolve all durable attachment references in a restored message snapshot.
 * Requests are deduplicated by asset ID; a stale reference remains visible as
 * metadata but never reintroduces an inline base64 payload.
 */
export async function hydratePlaygroundMessages(
  messages: Message[]
): Promise<Message[]> {
  const previews = new Map<string, Promise<PlaygroundAttachmentPreview>>()
  const previewFor = (assetId: string) => {
    let request = previews.get(assetId)
    if (!request) {
      request = getPlaygroundAttachmentPreview(assetId)
      previews.set(assetId, request)
    }
    return request
  }

  let changed = false
  const hydratedMessages = await Promise.all(
    messages.map(async (message) => {
      let messageChanged = false
      let legacyGeneratedMedia = message.generatedMedia
      const versions = await Promise.all(
        message.versions.map(async (version) => {
          if (!version.attachments?.length && !version.generatedMedia?.length) {
            return version
          }
          let versionChanged = false
          const attachments = version.attachments?.length
            ? await Promise.all(
                version.attachments.map(async (attachment) => {
                  if (!isMediaAttachment(attachment) || !attachment.assetId) {
                    return attachment
                  }
                  try {
                    const assetId = attachment.assetId.trim()
                    const preview = await previewFor(assetId)
                    assertPreviewMatchesAttachment(attachment, preview, assetId)
                    const next =
                      attachment.kind === 'audio'
                        ? await hydrateAudioAttachment(attachment, preview)
                        : markTrustedAttachmentURL(
                            hydratedAttachment(attachment, preview)
                          )
                    if (
                      next.url !== attachment.url ||
                      next.mediaType !== attachment.mediaType ||
                      next.dataUrl !== attachment.dataUrl
                    ) {
                      versionChanged = true
                    }
                    return next
                  } catch {
                    // Keep only the durable ID. In particular, do not retain a
                    // base64 URL that would be rejected on the next persistence.
                    if (attachment.url) {
                      versionChanged = true
                      const next = { ...attachment }
                      delete next.url
                      return next
                    }
                    return attachment
                  }
                })
              )
            : version.attachments
          const generatedResult = await hydrateGeneratedMediaList(
            version.generatedMedia,
            previewFor
          )
          if (generatedResult.changed) versionChanged = true
          if (!versionChanged) return version
          messageChanged = true
          changed = true
          return {
            ...version,
            ...(attachments ? { attachments } : {}),
            ...(generatedResult.media
              ? { generatedMedia: generatedResult.media }
              : {}),
          }
        })
      )
      // Some older server snapshots stored generated media directly on the
      // message. Hydrate that fallback too when the current version has no
      // authoritative media, so those records survive a reload as well.
      if (
        legacyGeneratedMedia &&
        message.versions[0]?.generatedMedia === undefined
      ) {
        const legacyResult = await hydrateGeneratedMediaList(
          legacyGeneratedMedia,
          previewFor
        )
        if (legacyResult.changed) {
          legacyGeneratedMedia = legacyResult.media
          messageChanged = true
          changed = true
        }
      }
      if (!messageChanged) return message
      return {
        ...message,
        versions,
        ...(legacyGeneratedMedia !== message.generatedMedia
          ? { generatedMedia: legacyGeneratedMedia }
          : {}),
      }
    })
  )

  return changed ? hydratedMessages : messages
}
