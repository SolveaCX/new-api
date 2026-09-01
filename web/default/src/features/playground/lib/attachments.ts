import type { FileUIPart } from 'ai'
import type { PlaygroundAttachment } from '../types'
import { markTrustedAttachmentURL } from './message-utils'

export const MAX_ATTACHMENTS = 5
export const MAX_FILE_BYTES = 10 * 1024 * 1024
export const MAX_TEXT_BYTES = 1024 * 1024

const IMAGE_MEDIA_TYPES = new Set([
  'image/gif',
  'image/jpeg',
  'image/png',
  'image/webp',
])
const TEXT_EXTENSIONS = new Set(['.csv', '.json', '.md', '.txt'])
const IMAGE_EXTENSIONS = new Set(['.gif', '.jpeg', '.jpg', '.png', '.webp'])
const DOCUMENT_MEDIA_TYPES = new Set(['application/pdf'])
const DOCUMENT_EXTENSIONS = new Set(['.pdf'])
const VIDEO_MEDIA_TYPES = new Set(['video/mp4'])
const VIDEO_EXTENSIONS = new Set(['.mp4'])
const AUDIO_MEDIA_TYPES = new Set([
  'audio/mpeg',
  'audio/mp3',
  'audio/wav',
  'audio/x-wav',
])
const AUDIO_EXTENSIONS = new Set(['.mp3', '.wav'])

/**
 * Read a local video's duration without loading its media data into the
 * document. The helper deliberately treats metadata failures as an unknown
 * duration so callers can fall back to an explicit manual value.
 */
export function readPlaygroundVideoDuration(
  url: string
): Promise<number | undefined> {
  const normalizedURL = url.trim()
  if (
    !normalizedURL ||
    typeof document === 'undefined' ||
    typeof document.createElement !== 'function'
  ) {
    return Promise.resolve(undefined)
  }

  return new Promise((resolve) => {
    let video: HTMLVideoElement
    try {
      video = document.createElement('video')
    } catch {
      // A restricted WebView may expose `document` without allowing media
      // element creation; treat that the same as unreadable metadata.
      resolve(undefined)
      return
    }
    let settled = false
    const timeoutRef: { id?: ReturnType<typeof setTimeout> } = {}

    const cleanup = () => {
      video.removeEventListener?.('loadedmetadata', onLoadedMetadata)
      video.removeEventListener?.('error', onError)
      video.onloadedmetadata = null
      video.onerror = null
      if (timeoutRef.id !== undefined) clearTimeout(timeoutRef.id)
      try {
        video.removeAttribute?.('src')
        // Reset the media element so a browser can release any metadata
        // resources. This is intentionally best-effort for test doubles.
        video.load?.()
      } catch {
        // Cleanup must never turn an unknown duration into a submit error.
      }
    }

    const finish = (duration?: number) => {
      if (settled) return
      settled = true
      cleanup()
      resolve(duration)
    }

    const onLoadedMetadata = () => {
      const duration = Number(video.duration)
      if (!Number.isFinite(duration) || duration <= 0 || duration > 3600) {
        finish()
        return
      }
      finish(Math.round(duration * 10) / 10)
    }

    const onError = () => finish()

    video.preload = 'metadata'
    video.addEventListener?.('loadedmetadata', onLoadedMetadata)
    video.addEventListener?.('error', onError)
    // Property handlers keep this helper compatible with lightweight browser
    // shims and older WebViews that do not implement EventTarget fully.
    video.onloadedmetadata = onLoadedMetadata
    video.onerror = onError
    // Metadata can fail to arrive for a cross-origin signed URL. Bound the
    // wait so submission remains usable with the manual duration control.
    timeoutRef.id = setTimeout(() => finish(), 5000)
    try {
      video.src = normalizedURL
      video.load?.()
    } catch {
      finish()
    }
  })
}

/** Normalize equivalent browser/object-store MIME aliases at the UI boundary. */
export function normalizePlaygroundMediaType(value: string): string {
  const mediaType = value.split(';', 1)[0].trim().toLowerCase()
  switch (mediaType) {
    case 'image/jpg':
      return 'image/jpeg'
    case 'audio/mp3':
      return 'audio/mpeg'
    case 'audio/x-wav':
      return 'audio/wav'
    default:
      return mediaType
  }
}

interface ParsedDataUrl {
  mediaType: string
  bytes: Uint8Array
}

function parseDataUrl(url: string): ParsedDataUrl {
  const match = /^data:([^;,]+);base64,([a-z0-9+/=\r\n]*)$/i.exec(url)
  if (!match) throw new Error('Attachment data is invalid')

  const encoded = match[2].replace(/[\r\n]/g, '')
  const padding = encoded.endsWith('==') ? 2 : encoded.endsWith('=') ? 1 : 0
  const estimatedBytes = Math.floor((encoded.length * 3) / 4) - padding
  if (estimatedBytes > MAX_FILE_BYTES) {
    throw new Error('Attachment exceeds the maximum size')
  }

  try {
    const decoded = atob(encoded)
    const bytes = Uint8Array.from(decoded, (char) => char.charCodeAt(0))
    return { mediaType: match[1].toLowerCase(), bytes }
  } catch {
    throw new Error('Attachment data is invalid')
  }
}

function decodeUtf8(bytes: Uint8Array): string {
  try {
    return new TextDecoder('utf-8', { fatal: true })
      .decode(bytes)
      .replace(/^\uFEFF/, '')
  } catch {
    throw new Error('Text attachment is not valid UTF-8')
  }
}

function extensionOf(filename: string): string {
  const dot = filename.lastIndexOf('.')
  return dot === -1 ? '' : filename.slice(dot).toLowerCase()
}

function normalizeFilename(filename: string | undefined): string {
  const normalized = filename?.trim()
  return normalized || 'attachment'
}

function isImageAttachment(mediaType: string, filename: string): boolean {
  return (
    IMAGE_MEDIA_TYPES.has(mediaType) ||
    IMAGE_EXTENSIONS.has(extensionOf(filename))
  )
}

function isTextAttachment(mediaType: string, filename: string): boolean {
  return (
    mediaType.startsWith('text/') ||
    (mediaType === 'application/json' && extensionOf(filename) === '.json') ||
    TEXT_EXTENSIONS.has(extensionOf(filename))
  )
}

function isVideoAttachment(mediaType: string, filename: string): boolean {
  return (
    VIDEO_MEDIA_TYPES.has(mediaType) ||
    VIDEO_EXTENSIONS.has(extensionOf(filename))
  )
}

function isDocumentAttachment(mediaType: string, filename: string): boolean {
  return (
    DOCUMENT_MEDIA_TYPES.has(mediaType) ||
    DOCUMENT_EXTENSIONS.has(extensionOf(filename))
  )
}

function isAudioAttachment(mediaType: string, filename: string): boolean {
  return (
    AUDIO_MEDIA_TYPES.has(mediaType) ||
    AUDIO_EXTENSIONS.has(extensionOf(filename))
  )
}

export async function normalizePlaygroundAttachments(
  files: (FileUIPart & { assetId?: string })[]
): Promise<PlaygroundAttachment[]> {
  if (files.length > MAX_ATTACHMENTS) {
    throw new Error('Too many attachments')
  }

  const attachments: PlaygroundAttachment[] = []
  for (const file of files) {
    const filename = normalizeFilename(file.filename)
    const declaredMediaType = normalizePlaygroundMediaType(file.mediaType ?? '')
    const assetId = file.assetId?.trim()

    // Restored drafts carry a durable server asset and a short-lived preview
    // URL. They are intentionally not converted back to base64: the upload
    // layer refreshes the preview by asset ID immediately before dispatch.
    if (
      assetId &&
      (declaredMediaType.startsWith('image/') ||
        declaredMediaType.startsWith('video/') ||
        declaredMediaType.startsWith('audio/') ||
        declaredMediaType === 'application/pdf')
    ) {
      const durableAttachment: PlaygroundAttachment = {
        kind: declaredMediaType.startsWith('image/')
          ? 'image'
          : declaredMediaType.startsWith('video/')
            ? 'video'
            : declaredMediaType.startsWith('audio/')
              ? 'audio'
              : 'document',
        filename,
        mediaType: normalizePlaygroundMediaType(declaredMediaType),
        assetId,
        ...(file.url ? { url: file.url } : {}),
      }
      attachments.push(markTrustedAttachmentURL(durableAttachment))
      continue
    }

    const parsed = parseDataUrl(file.url)
    const parsedMediaType = normalizePlaygroundMediaType(parsed.mediaType)
    const mediaType = normalizePlaygroundMediaType(
      declaredMediaType || parsedMediaType
    )

    if (parsed.bytes.length === 0) {
      throw new Error('Attachment is empty')
    }
    if (parsed.bytes.length > MAX_FILE_BYTES) {
      throw new Error('Attachment exceeds the maximum size')
    }

    if (isImageAttachment(mediaType, filename)) {
      if (!IMAGE_MEDIA_TYPES.has(parsedMediaType)) {
        throw new Error('Attachment data is invalid')
      }
      attachments.push({
        kind: 'image',
        filename,
        mediaType: parsedMediaType,
        url: file.url,
      })
      continue
    }

    if (isVideoAttachment(mediaType, filename)) {
      if (!VIDEO_MEDIA_TYPES.has(parsedMediaType)) {
        throw new Error('Attachment data is invalid')
      }
      const durationSeconds = await readPlaygroundVideoDuration(file.url)
      attachments.push({
        kind: 'video',
        filename,
        mediaType: parsedMediaType,
        url: file.url,
        ...(durationSeconds !== undefined ? { durationSeconds } : {}),
      })
      continue
    }

    if (isDocumentAttachment(mediaType, filename)) {
      if (!DOCUMENT_MEDIA_TYPES.has(parsedMediaType)) {
        throw new Error('Attachment data is invalid')
      }
      attachments.push({
        kind: 'document',
        filename,
        mediaType: parsedMediaType,
        url: file.url,
      })
      continue
    }

    if (isAudioAttachment(mediaType, filename)) {
      if (!AUDIO_MEDIA_TYPES.has(parsedMediaType)) {
        throw new Error('Attachment data is invalid')
      }
      attachments.push({
        kind: 'audio',
        filename,
        mediaType: parsedMediaType,
        url: file.url,
        dataUrl: file.url,
      })
      continue
    }

    if (!isTextAttachment(mediaType, filename)) {
      throw new Error('Unsupported attachment type')
    }
    if (parsed.bytes.length > MAX_TEXT_BYTES) {
      throw new Error('Text attachment exceeds the maximum size')
    }

    const text = decodeUtf8(parsed.bytes)
    if (!text.trim()) {
      throw new Error('Attachment is empty')
    }
    attachments.push({ kind: 'text', filename, mediaType, text })
  }

  return attachments
}
