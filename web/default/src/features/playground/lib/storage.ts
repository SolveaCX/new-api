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
import { STORAGE_KEYS } from '../constants'
import type { Message, ParameterEnabled, PlaygroundConfig } from '../types'
import { sanitizeMessagesOnLoad } from './message-utils'

function scopedKey(base: string, userId: number): string {
  return `${base}:v2:${userId}`
}

function isValidUserId(userId: number): boolean {
  return Number.isInteger(userId) && userId > 0
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value)
}

/**
 * Signed preview URLs are transport-only and can expire while a conversation
 * sits in localStorage. Keep the durable asset ID, then let restore hydration
 * request a fresh preview URL. Blob URLs are process-local and must not be
 * revived after a reload; an in-flight video task can still resume via its
 * persisted task ID.
 */
function sanitizeMessagesForLocalStorage(messages: Message[]): Message[] {
  return messages.map((message) => {
    let messageChanged = false
    const versions = message.versions.map((version) => {
      if (!version.attachments?.length) return version
      const attachments = version.attachments.map((attachment) => {
        const isDurableMedia =
          !!attachment.assetId &&
          (attachment.kind === 'image' || attachment.kind === 'video')
        const isTransientVideoURL =
          attachment.kind === 'video' && attachment.url?.startsWith('blob:')
        if (!isDurableMedia && !isTransientVideoURL) return attachment

        const sanitized = { ...attachment }
        delete sanitized.url
        messageChanged = true
        return sanitized
      })
      return messageChanged ? { ...version, attachments } : version
    })

    let sanitizedMessage = message
    if (messageChanged) sanitizedMessage = { ...message, versions }
    if (message.videoUrl?.startsWith('blob:')) {
      if (sanitizedMessage === message) sanitizedMessage = { ...message }
      delete sanitizedMessage.videoUrl
    }
    return sanitizedMessage
  })
}

export interface LocalConversationPriority {
  conversationId: string
  markedAt: number
}

/**
 * Load playground config from localStorage
 */
export function loadConfig(userId: number): Partial<PlaygroundConfig> {
  if (!isValidUserId(userId)) return {}
  try {
    const saved = localStorage.getItem(scopedKey(STORAGE_KEYS.CONFIG, userId))
    if (saved) {
      const parsed: unknown = JSON.parse(saved)
      return isRecord(parsed) ? parsed : {}
    }
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to load config:', error)
  }
  return {}
}

/**
 * Save playground config to localStorage
 */
export function saveConfig(
  userId: number,
  config: Partial<PlaygroundConfig>
): void {
  if (!isValidUserId(userId)) return
  try {
    localStorage.setItem(
      scopedKey(STORAGE_KEYS.CONFIG, userId),
      JSON.stringify(config)
    )
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to save config:', error)
  }
}

/**
 * Load parameter enabled state from localStorage
 */
export function loadParameterEnabled(
  userId: number
): Partial<ParameterEnabled> {
  if (!isValidUserId(userId)) return {}
  try {
    const saved = localStorage.getItem(
      scopedKey(STORAGE_KEYS.PARAMETER_ENABLED, userId)
    )
    if (saved) {
      const parsed: unknown = JSON.parse(saved)
      return isRecord(parsed) ? parsed : {}
    }
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to load parameter enabled:', error)
  }
  return {}
}

/**
 * Save parameter enabled state to localStorage
 */
export function saveParameterEnabled(
  userId: number,
  parameterEnabled: Partial<ParameterEnabled>
): void {
  if (!isValidUserId(userId)) return
  try {
    localStorage.setItem(
      scopedKey(STORAGE_KEYS.PARAMETER_ENABLED, userId),
      JSON.stringify(parameterEnabled)
    )
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to save parameter enabled:', error)
  }
}

/**
 * Load messages from localStorage
 */
export function loadMessages(userId: number): Message[] | null {
  if (!isValidUserId(userId)) return null
  try {
    const saved = localStorage.getItem(scopedKey(STORAGE_KEYS.MESSAGES, userId))
    if (saved) {
      const parsed: unknown = JSON.parse(saved)
      if (!Array.isArray(parsed)) {
        return null
      }
      const sanitized = sanitizeMessagesOnLoad(parsed as Message[])
      // Persist sanitized result to avoid re-sanitizing on subsequent loads
      if (sanitized !== parsed) {
        saveMessages(userId, sanitized)
      }
      return sanitized
    }
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to load messages:', error)
  }
  return null
}

/**
 * Save messages to localStorage
 */
export function saveMessages(userId: number, messages: Message[]): void {
  if (!isValidUserId(userId)) return
  try {
    localStorage.setItem(
      scopedKey(STORAGE_KEYS.MESSAGES, userId),
      JSON.stringify(sanitizeMessagesForLocalStorage(messages))
    )
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to save messages:', error)
  }
}

/**
 * Load the active conversation id from localStorage
 */
export function loadConversationId(userId: number): string | null {
  if (!isValidUserId(userId)) return null
  try {
    const saved = localStorage.getItem(
      scopedKey(STORAGE_KEYS.CONVERSATION, userId)
    )
    return saved?.trim() || null
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to load conversation id:', error)
    return null
  }
}

/**
 * Save the active conversation id to localStorage
 */
export function saveConversationId(
  userId: number,
  conversationId: string
): void {
  if (!isValidUserId(userId)) return
  try {
    localStorage.setItem(
      scopedKey(STORAGE_KEYS.CONVERSATION, userId),
      conversationId
    )
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to save conversation id:', error)
  }
}

export function loadLocalConversationPriority(
  userId: number
): LocalConversationPriority | null {
  if (!isValidUserId(userId)) return null
  try {
    const saved = localStorage.getItem(
      scopedKey(STORAGE_KEYS.LOCAL_CONVERSATION_PRIORITY, userId)
    )
    if (!saved) return null
    const parsed: unknown = JSON.parse(saved)
    if (
      !isRecord(parsed) ||
      typeof parsed.conversationId !== 'string' ||
      !parsed.conversationId.trim() ||
      !Number.isInteger(parsed.markedAt) ||
      Number(parsed.markedAt) <= 0
    ) {
      return null
    }
    return {
      conversationId: parsed.conversationId,
      markedAt: Number(parsed.markedAt),
    }
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to load local Playground priority:', error)
    return null
  }
}

export function saveLocalConversationPriority(
  userId: number,
  priority: LocalConversationPriority
): void {
  if (!isValidUserId(userId)) return
  try {
    localStorage.setItem(
      scopedKey(STORAGE_KEYS.LOCAL_CONVERSATION_PRIORITY, userId),
      JSON.stringify(priority)
    )
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to save local Playground priority:', error)
  }
}

export function clearLocalConversationPriority(userId: number): void {
  if (!isValidUserId(userId)) return
  try {
    localStorage.removeItem(
      scopedKey(STORAGE_KEYS.LOCAL_CONVERSATION_PRIORITY, userId)
    )
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to clear local Playground priority:', error)
  }
}

/**
 * Remove all versioned Playground state for one user
 */
export function clearUserPlaygroundData(userId: number): void {
  if (!isValidUserId(userId)) return
  try {
    const keys = [
      STORAGE_KEYS.CONFIG,
      STORAGE_KEYS.PARAMETER_ENABLED,
      STORAGE_KEYS.MESSAGES,
      STORAGE_KEYS.CONVERSATION,
      STORAGE_KEYS.LOCAL_CONVERSATION_PRIORITY,
    ]
    keys.forEach((key) => localStorage.removeItem(scopedKey(key, userId)))
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to clear Playground data:', error)
  }
}
