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

For commercial licensing, please contact support@quantumnous.com
*/
import { api } from '@/lib/api'
import { API_ENDPOINTS, PLAYGROUND_RECORDS_EXPORT } from './constants'
import type { MediaGenerationRequest } from './lib/media-generation'
import type { PlaygroundConversationSnapshot } from './lib/playground-persistence'
import type {
  ChatCompletionRequest,
  ChatCompletionResponse,
  GroupOption,
  VideoTask,
  PlaygroundRecordPayload,
  PlaygroundConversationSummary,
} from './types'

export interface PlaygroundAttachmentUploadSession {
  upload_id: string
  asset_id: string
  object: string
  status: string
  upload_url: string
  upload_headers?: Record<string, string>
  expires_at: number
}

export interface PlaygroundAttachmentPreview {
  asset_id: string
  asset_type: string
  content_type: string
  size_bytes: number
  preview_url: string
  expires_at: number
}

interface PlaygroundApiResponse<T = unknown> {
  success: boolean
  data?: T
  message?: string
}

export interface PlaygroundRecordExportResult {
  blob: Blob
  filename: string
}

export class PlaygroundRecordExportError extends Error {
  status?: number

  constructor(message: string, status?: number, cause?: unknown) {
    super(message, { cause })
    this.name = 'PlaygroundRecordExportError'
    this.status = status
  }
}

function assertPlaygroundApiSuccess(
  response: PlaygroundApiResponse,
  fallbackMessage: string
): void {
  if (!response.success) {
    throw new Error(response.message || fallbackMessage)
  }
}

function getPlaygroundHeaderValue(
  headers: unknown,
  name: string
): string | undefined {
  if (!headers || typeof headers !== 'object') return undefined

  const normalizedName = name.toLowerCase()
  const headerBag = headers as {
    get?: (key: string) => unknown
    [key: string]: unknown
  }

  if (typeof headerBag.get === 'function') {
    const value = headerBag.get(name)
    if (typeof value === 'string') return value
    if (Array.isArray(value)) {
      const joined = value.filter((item) => typeof item === 'string').join(', ')
      return joined || undefined
    }
    if (value != null) return String(value)
  }

  for (const [key, value] of Object.entries(headerBag)) {
    if (key.toLowerCase() !== normalizedName) continue
    if (typeof value === 'string') return value
    if (Array.isArray(value)) {
      const joined = value.filter((item) => typeof item === 'string').join(', ')
      return joined || undefined
    }
    if (value != null) return String(value)
  }

  return undefined
}

function decodePlaygroundRecordExportFilename(value: string): string | null {
  const candidates = [
    /filename\*\s*=\s*(?:UTF-8''|)([^;]+)/i,
    /filename\s*=\s*([^;]+)/i,
  ]

  for (const pattern of candidates) {
    const match = pattern.exec(value)
    if (!match?.[1]) continue
    const raw = match[1].trim().replace(/^"|"$/g, '')
    if (!raw) continue

    try {
      return decodeURIComponent(raw)
    } catch {
      return raw
    }
  }

  return null
}

function isSafePlaygroundRecordExportFilename(filename: string): boolean {
  if (!filename || filename !== filename.trim()) return false
  if (filename === '.' || filename === '..') return false
  if (
    [...filename].some((character) => {
      const code = character.charCodeAt(0)
      return code <= 0x1f || code === 0x7f
    })
  ) {
    return false
  }
  if (/[\\/:*?"<>|]/.test(filename)) return false
  if (filename.split(/[\\/]/).pop() !== filename) return false
  return filename.toLowerCase().endsWith('.xlsx')
}

function buildPlaygroundRecordExportFallbackFilename(
  date = new Date()
): string {
  const pad = (value: number) => String(value).padStart(2, '0')
  return (
    [
      'playground-records',
      `${date.getFullYear()}${pad(date.getMonth() + 1)}${pad(date.getDate())}`,
      `${pad(date.getHours())}${pad(date.getMinutes())}${pad(date.getSeconds())}`,
    ].join('-') + '.xlsx'
  )
}

function resolvePlaygroundRecordExportFilename(headers: unknown): string {
  const fallback = buildPlaygroundRecordExportFallbackFilename()
  const contentDisposition = getPlaygroundHeaderValue(
    headers,
    'content-disposition'
  )
  if (!contentDisposition) return fallback

  const filename = decodePlaygroundRecordExportFilename(contentDisposition)
  if (!filename || !isSafePlaygroundRecordExportFilename(filename)) {
    return fallback
  }

  return filename
}

async function readPlaygroundRecordExportBlobMessage(
  blob: Blob
): Promise<string> {
  const text = (await blob.text()).trim()
  if (!text) return 'Playground records export failed'

  try {
    const payload = JSON.parse(text) as unknown
    if (payload && typeof payload === 'object') {
      const body = payload as {
        message?: unknown
        error?: unknown
      }
      if (typeof body.message === 'string' && body.message.trim()) {
        return body.message
      }
      if (typeof body.error === 'string' && body.error.trim()) {
        return body.error
      }
      if (body.error && typeof body.error === 'object') {
        const nestedMessage = (body.error as { message?: unknown }).message
        if (typeof nestedMessage === 'string' && nestedMessage.trim()) {
          return nestedMessage
        }
      }
    }
  } catch {
    return text
  }

  return text
}

/**
 * Send chat completion request (non-streaming)
 */
export async function sendChatCompletion(
  payload: ChatCompletionRequest,
  signal?: AbortSignal
): Promise<ChatCompletionResponse> {
  const res = await api.post(API_ENDPOINTS.CHAT_COMPLETIONS, payload, {
    signal,
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

export async function sendMediaGeneration(
  request: MediaGenerationRequest,
  signal?: AbortSignal
): Promise<unknown> {
  const requestConfig: Record<string, unknown> = {
    signal,
    skipErrorHandler: true,
  }
  if (request.kind === 'audio') {
    // Synchronous text-to-speech is the only media request that returns raw
    // binary data. Sonilo returns a JSON task envelope, despite producing
    // audio later.
    requestConfig.responseType = 'blob'
  }
  // Do not set Content-Type here: Axios/browser must generate the multipart
  // boundary for the Sonilo FormData payload.
  const res = await api.post(request.endpoint, request.payload, requestConfig)
  return res.data
}

export async function fetchPlaygroundVideoTask(
  taskId: string,
  signal?: AbortSignal
): Promise<unknown> {
  const res = await api.get(`/pg/videos/${encodeURIComponent(taskId)}`, {
    signal,
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

export async function fetchPlaygroundVideoToMusicTask(
  taskId: string,
  signal?: AbortSignal
): Promise<unknown> {
  const res = await api.get(
    `/pg/video-to-music/${encodeURIComponent(taskId)}`,
    {
      signal,
      disableDuplicate: true,
      skipErrorHandler: true,
    } as Record<string, unknown>
  )
  return res.data
}

/**
 * Submit an async video-generation task (veo models).
 * POST /v1/videos { model, prompt } -> { id, status, progress, ... }
 */
export async function submitVideo(
  model: string,
  prompt: string
): Promise<VideoTask> {
  const res = await api.post(API_ENDPOINTS.VIDEOS, { model, prompt }, {
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Poll a video task's status.
 * GET /v1/videos/{id} -> { status, progress, ... }
 * `disableDuplicate` opts out of the GET dedupe cache so each poll is a fresh
 * request rather than a possibly-stale in-flight promise.
 */
export async function fetchVideoStatus(id: string): Promise<VideoTask> {
  const res = await api.get(
    `${API_ENDPOINTS.VIDEOS}/${encodeURIComponent(id)}`,
    {
      disableDuplicate: true,
      skipErrorHandler: true,
    }
  )
  return res.data
}

/**
 * Fetch the finished video as a binary blob.
 * GET /v1/videos/{id}/content -> raw MP4 (video/mp4). Returned as a Blob so the
 * caller can URL.createObjectURL(...) it into a <video> element (the endpoint
 * needs the auth header, so the bare URL can't be used as a <video src>).
 */
export async function fetchVideoContent(
  id: string,
  signal?: AbortSignal
): Promise<Blob> {
  const res = await api.get(
    `${API_ENDPOINTS.VIDEOS}/${encodeURIComponent(id)}/content`,
    {
      responseType: 'blob',
      disableDuplicate: true,
      skipErrorHandler: true,
      signal,
    }
  )
  return res.data as Blob
}

export async function downloadPlaygroundRecords(): Promise<PlaygroundRecordExportResult> {
  try {
    const res = await api.get(PLAYGROUND_RECORDS_EXPORT, {
      params: { format: 'xlsx' },
      responseType: 'blob',
      disableDuplicate: true,
      skipErrorHandler: true,
    } as Record<string, unknown>)

    if (!(res.data instanceof Blob)) {
      throw new Error('Playground records export returned non-Blob response')
    }

    return {
      blob: res.data,
      filename: resolvePlaygroundRecordExportFilename(res.headers),
    }
  } catch (error) {
    const response = (
      error as {
        response?: { status?: unknown; data?: unknown }
      }
    )?.response

    if (response?.data instanceof Blob) {
      throw new PlaygroundRecordExportError(
        await readPlaygroundRecordExportBlobMessage(response.data),
        typeof response.status === 'number' ? response.status : undefined,
        error
      )
    }

    throw error
  }
}

/**
 * Get models available to the user for Playground display. The opt-in flag
 * applies the administrator's hidden-model policy without changing the default
 * behavior of the shared backend endpoint.
 */
export async function getUserModels(group?: string): Promise<string[]> {
  const res = await api.get(API_ENDPOINTS.USER_MODELS, {
    params: {
      ...(group ? { group } : {}),
      exclude_hidden: true,
    },
  })
  const { data } = res

  if (!data.success || !Array.isArray(data.data)) {
    return []
  }

  return data.data
    .filter((model: unknown): model is string => typeof model === 'string')
    .map((model: string) => model.trim())
    .filter(Boolean)
}

export type PlaygroundModelPricing = {
  model_name: string
  tags?: string
  display_weight?: number
  model_price?: number
  featured_order?: number
  release_date?: string
  directory_metadata?: { released_at?: string }
  display_pricing?: {
    prices?: Record<string, { plg?: number | string } | undefined>
  }
}

type PlaygroundPricingResponse = {
  success?: boolean
  data?: PlaygroundModelPricing[]
  display_pricing?: Record<string, PlaygroundModelPricing['display_pricing']>
}

export async function getPlaygroundModelPricing(): Promise<
  PlaygroundModelPricing[]
> {
  const res = await api.get<PlaygroundPricingResponse>('/api/website/pricing', {
    params: { group: 'plg' },
  })
  const payload = res.data
  if (!payload?.success || !Array.isArray(payload.data)) return []
  return payload.data.map((model) => ({
    ...model,
    display_pricing:
      model.display_pricing ?? payload.display_pricing?.[model.model_name],
  }))
}
/**
 * Get user groups
 */
export async function getUserGroups(): Promise<GroupOption[]> {
  const res = await api.get(API_ENDPOINTS.USER_GROUPS)
  const { data } = res

  if (!data.success || !data.data) {
    return []
  }

  const groupData = data.data as Record<string, { desc: string; ratio: number }>

  // label is for button display (name only); desc is for dropdown content
  return Object.entries(groupData).map(([group, info]) => ({
    label: group,
    value: group,
    ratio: info.ratio,
    desc: info.desc,
  }))
}

export async function savePlaygroundRecord(
  payload: PlaygroundRecordPayload
): Promise<void> {
  const res = await api.post(API_ENDPOINTS.PLAYGROUND_RECORDS, payload)
  assertPlaygroundApiSuccess(res.data, 'Failed to save Playground record')
}

export async function getCurrentPlaygroundRecord(): Promise<PlaygroundConversationSnapshot | null> {
  const res = await api.get(API_ENDPOINTS.PLAYGROUND_RECORDS_CURRENT)
  const response =
    res.data as PlaygroundApiResponse<PlaygroundConversationSnapshot | null>
  assertPlaygroundApiSuccess(
    response,
    'Failed to restore the current Playground conversation'
  )

  if (response.data === null) return null
  if (
    !response.data ||
    typeof response.data.conversation_id !== 'string' ||
    !Array.isArray(response.data.messages)
  ) {
    throw new Error('Invalid current Playground conversation response')
  }

  return response.data
}

export async function clearCurrentPlaygroundRecord(
  recordId: string,
  conversationId: string,
  clientCompletedAt: number
): Promise<void> {
  const res = await api.post(API_ENDPOINTS.PLAYGROUND_RECORDS_CLEAR, {
    record_id: recordId,
    conversation_id: conversationId,
    client_completed_at: clientCompletedAt,
  })
  assertPlaygroundApiSuccess(
    res.data,
    'Failed to clear Playground conversation'
  )
}

export async function listPlaygroundConversations(): Promise<
  PlaygroundConversationSummary[]
> {
  const res = await api.get(API_ENDPOINTS.PLAYGROUND_CONVERSATIONS)
  const response = res.data as PlaygroundApiResponse<
    PlaygroundConversationSummary[]
  >
  assertPlaygroundApiSuccess(
    response,
    'Failed to load Playground conversations'
  )
  return Array.isArray(response.data) ? response.data : []
}

export async function getPlaygroundConversation(
  conversationId: string
): Promise<PlaygroundConversationSnapshot | null> {
  const res = await api.get(
    `${API_ENDPOINTS.PLAYGROUND_CONVERSATIONS}/${encodeURIComponent(conversationId)}`
  )
  const response =
    res.data as PlaygroundApiResponse<PlaygroundConversationSnapshot | null>
  assertPlaygroundApiSuccess(response, 'Failed to load Playground conversation')
  return response.data ?? null
}

export async function renamePlaygroundConversation(
  conversationId: string,
  name: string
): Promise<void> {
  const res = await api.patch(
    `${API_ENDPOINTS.PLAYGROUND_CONVERSATIONS}/${encodeURIComponent(conversationId)}`,
    { name }
  )
  assertPlaygroundApiSuccess(res.data, 'Failed to rename conversation')
}

export async function deletePlaygroundConversations(
  conversationIds: string[]
): Promise<void> {
  const res = await api.post(
    `${API_ENDPOINTS.PLAYGROUND_CONVERSATIONS}/delete`,
    { conversation_ids: conversationIds }
  )
  assertPlaygroundApiSuccess(res.data, 'Failed to delete conversations')
}

function playgroundApiErrorMessage(
  error: unknown,
  fallbackMessage: string
): string {
  const responseData = (error as { response?: { data?: unknown } } | undefined)
    ?.response?.data
  if (responseData && typeof responseData === 'object') {
    const body = responseData as Record<string, unknown>
    if (typeof body.message === 'string' && body.message.trim()) {
      return body.message
    }
    const nested = body.error
    if (nested && typeof nested === 'object') {
      const message = (nested as Record<string, unknown>).message
      if (typeof message === 'string' && message.trim()) return message
    }
  }
  if (error instanceof Error && error.message.trim()) return error.message
  return fallbackMessage
}

/** Create a short-lived signed upload session for a Playground attachment. */
export async function createPlaygroundAttachmentUploadSession(
  request: {
    assetType: 'Image' | 'Video' | 'Audio' | 'Document'
    contentType: string
    sizeBytes: number
  },
  signal?: AbortSignal
): Promise<PlaygroundAttachmentUploadSession> {
  try {
    const res = await api.post(
      API_ENDPOINTS.PLAYGROUND_ATTACHMENT_UPLOADS,
      {
        asset_type: request.assetType,
        content_type: request.contentType,
        size_bytes: request.sizeBytes,
      },
      { skipErrorHandler: true, signal } as Record<string, unknown>
    )
    const response =
      res.data as PlaygroundApiResponse<PlaygroundAttachmentUploadSession>
    assertPlaygroundApiSuccess(
      response,
      'Failed to create Playground attachment upload session'
    )
    if (
      !response.data ||
      typeof response.data.upload_id !== 'string' ||
      typeof response.data.asset_id !== 'string' ||
      typeof response.data.upload_url !== 'string'
    ) {
      throw new Error('Invalid Playground attachment upload session response')
    }
    return response.data
  } catch (error) {
    throw new Error(
      playgroundApiErrorMessage(
        error,
        'Failed to create Playground attachment upload session'
      ),
      { cause: error }
    )
  }
}

/** Mark an uploaded object complete and return its first signed preview URL. */
export async function completePlaygroundAttachmentUpload(
  uploadId: string,
  signal?: AbortSignal
): Promise<PlaygroundAttachmentPreview> {
  try {
    const res = await api.post(
      `${API_ENDPOINTS.PLAYGROUND_ATTACHMENT_UPLOADS}/${encodeURIComponent(uploadId)}/complete`,
      undefined,
      { skipErrorHandler: true, signal } as Record<string, unknown>
    )
    const response =
      res.data as PlaygroundApiResponse<PlaygroundAttachmentPreview>
    assertPlaygroundApiSuccess(
      response,
      'Failed to complete Playground attachment upload'
    )
    if (!response.data || typeof response.data.asset_id !== 'string') {
      throw new Error('Invalid Playground attachment completion response')
    }
    return response.data
  } catch (error) {
    throw new Error(
      playgroundApiErrorMessage(
        error,
        'Failed to complete Playground attachment upload'
      ),
      { cause: error }
    )
  }
}

/** Resolve a fresh signed preview URL for a durable asset reference. */
export async function getPlaygroundAttachmentPreview(
  assetId: string,
  signal?: AbortSignal
): Promise<PlaygroundAttachmentPreview> {
  try {
    const res = await api.get(
      `${API_ENDPOINTS.PLAYGROUND_ATTACHMENT_PREVIEW}/${encodeURIComponent(assetId)}/preview`,
      { skipErrorHandler: true, disableDuplicate: true, signal } as Record<
        string,
        unknown
      >
    )
    const response =
      res.data as PlaygroundApiResponse<PlaygroundAttachmentPreview>
    assertPlaygroundApiSuccess(
      response,
      'Failed to restore Playground attachment preview'
    )
    if (!response.data || typeof response.data.asset_id !== 'string') {
      throw new Error('Invalid Playground attachment preview response')
    }
    return response.data
  } catch (error) {
    throw new Error(
      playgroundApiErrorMessage(
        error,
        'Failed to restore Playground attachment preview'
      ),
      { cause: error }
    )
  }
}
