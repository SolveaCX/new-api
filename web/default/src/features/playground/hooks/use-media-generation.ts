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
import { useCallback, useEffect, useRef, useState } from 'react'
import i18next from 'i18next'
import { toast } from 'sonner'
import {
  fetchPlaygroundVideoTask,
  fetchVideoContent,
  fetchPlaygroundVideoToMusicTask,
  sendMediaGeneration,
} from '../api'
import { MESSAGE_ROLES, MESSAGE_STATUS } from '../constants'
import {
  buildMediaGenerationRequest,
  extractGeneratedImages,
  parseVideoTaskResponse,
  parseVideoToMusicTaskResponse,
  updateAssistantMessageWithError,
  updateCurrentVersionContent,
  updateCurrentVersionMedia,
  type MediaGenerationSettings,
} from '../lib'
import { uploadPlaygroundAttachments } from '../lib/playground-attachments'
import type { GeneratedMedia, Message, PlaygroundAttachment } from '../types'

interface UseMediaGenerationOptions {
  messages: Message[]
  onMessageUpdate: (updater: (prev: Message[]) => Message[]) => void
}

const VIDEO_POLL_INTERVAL_MS = 3000
const VIDEO_POLL_LIMIT = 200

type PollTimeoutScheduler = (callback: () => void, delay: number) => number
type PollTimeoutCanceller = (timer: number) => void

function clearVideoTaskId(message: Message): Message {
  if (
    !Object.prototype.hasOwnProperty.call(message, 'videoTaskId') &&
    !Object.prototype.hasOwnProperty.call(message, 'mediaTaskType')
  ) {
    return message
  }
  const updated = { ...message }
  delete updated.videoTaskId
  delete updated.mediaTaskType
  return updated
}

function responseMessageContent(response: unknown): string {
  if (!response || typeof response !== 'object') return ''
  const choices = (response as { choices?: unknown }).choices
  if (!Array.isArray(choices) || choices.length === 0) return ''
  const first = choices[0]
  if (!first || typeof first !== 'object') return ''
  const message = (first as { message?: unknown }).message
  if (!message || typeof message !== 'object') return ''
  const content = (message as { content?: unknown }).content
  return typeof content === 'string' ? content : ''
}

function responseAudioBlob(response: unknown): Blob | undefined {
  if (typeof Blob !== 'undefined' && response instanceof Blob) {
    const mediaType = response.type.trim().toLowerCase()
    return response.size > 0 && mediaType.startsWith('audio/')
      ? response
      : undefined
  }
  if (typeof ArrayBuffer !== 'undefined' && response instanceof ArrayBuffer) {
    return response.byteLength > 0
      ? new Blob([response], { type: 'audio/wav' })
      : undefined
  }
  return undefined
}

function collectLiveObjectURLs(messages: Message[]): Set<string> {
  const urls = new Set<string>()
  messages.forEach((message) => {
    if (message.videoUrl?.startsWith('blob:')) urls.add(message.videoUrl)
    message.generatedMedia?.forEach((media) => {
      if (media.url?.startsWith('blob:')) urls.add(media.url)
    })
    message.versions.forEach((version) => {
      version.generatedMedia?.forEach((media) => {
        if (media.url?.startsWith('blob:')) urls.add(media.url)
      })
    })
  })
  return urls
}

function errorMessage(error: unknown): string {
  if (!error || typeof error !== 'object') {
    return i18next.t('Request error occurred')
  }
  const candidate = error as {
    message?: string
    response?: {
      data?: {
        message?: string
        error?: { message?: string }
      }
    }
  }
  return (
    candidate.response?.data?.error?.message ||
    candidate.response?.data?.message ||
    candidate.message ||
    i18next.t('Request error occurred')
  )
}

export function waitForVideoPoll(
  signal: AbortSignal,
  schedule: PollTimeoutScheduler = (callback, delay) =>
    window.setTimeout(callback, delay),
  cancel: PollTimeoutCanceller = (timer) => window.clearTimeout(timer)
): Promise<void> {
  return new Promise((resolve) => {
    let settled = false

    function cleanup() {
      signal.removeEventListener('abort', onAbort)
    }
    function finish() {
      if (settled) return
      settled = true
      cleanup()
      resolve()
    }
    function onAbort() {
      cancel(timer)
      finish()
    }

    const timer = schedule(finish, VIDEO_POLL_INTERVAL_MS)
    if (settled) {
      cancel(timer)
      return
    }
    signal.addEventListener('abort', onAbort, { once: true })
    if (signal.aborted) onAbort()
  })
}

export function findResumableVideoMessage(
  messages: Message[]
): Message | undefined {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index]
    if (
      message?.from === MESSAGE_ROLES.ASSISTANT &&
      (message.status === MESSAGE_STATUS.LOADING ||
        message.status === MESSAGE_STATUS.STREAMING) &&
      typeof message.videoTaskId === 'string' &&
      message.videoTaskId.trim()
    ) {
      return message
    }
  }
  return undefined
}

export function useMediaGeneration(props: UseMediaGenerationOptions) {
  const { messages, onMessageUpdate } = props
  const abortControllerRef = useRef<AbortController | null>(null)
  const activeMessageKeyRef = useRef<string | null>(null)
  const generatedObjectURLsRef = useRef<Set<string>>(new Set())
  const [isGeneratingMedia, setIsGeneratingMedia] = useState(false)

  const releaseMediaObjectURL = useCallback((url?: string) => {
    if (
      typeof url !== 'string' ||
      !url.startsWith('blob:') ||
      !generatedObjectURLsRef.current.has(url)
    ) {
      return
    }
    generatedObjectURLsRef.current.delete(url)
    if (
      typeof URL !== 'undefined' &&
      typeof URL.revokeObjectURL === 'function'
    ) {
      URL.revokeObjectURL(url)
    }
  }, [])

  const updateProgress = useCallback(
    (
      messageKey: string,
      content: string,
      progress?: number,
      videoTaskId?: string,
      mediaTaskType: Message['mediaTaskType'] = 'video'
    ) => {
      onMessageUpdate((messages) =>
        messages.map((message) =>
          message.key === messageKey
            ? {
                ...updateCurrentVersionContent(
                  message,
                  progress === undefined ? content : `${content} ${progress}%`
                ),
                status: MESSAGE_STATUS.STREAMING,
                ...(videoTaskId ? { videoTaskId } : {}),
                ...(videoTaskId ? { mediaTaskType } : {}),
              }
            : message
        )
      )
    },
    [onMessageUpdate]
  )

  const completeMedia = useCallback(
    (
      messageKey: string,
      content: string,
      generatedMedia?: GeneratedMedia[]
    ) => {
      onMessageUpdate((messages) =>
        messages.map((message) =>
          message.key === messageKey
            ? clearVideoTaskId({
                ...updateCurrentVersionMedia(
                  updateCurrentVersionContent(message, content),
                  generatedMedia
                ),
                status: MESSAGE_STATUS.COMPLETE,
                isContentComplete: true,
              })
            : message
        )
      )
    },
    [onMessageUpdate]
  )

  const failMedia = useCallback(
    (messageKey: string, error: string) => {
      toast.error(error)
      onMessageUpdate((messages) =>
        messages.map((message) =>
          message.key === messageKey
            ? clearVideoTaskId(
                updateAssistantMessageWithError([message], error)[0]
              )
            : message
        )
      )
    },
    [onMessageUpdate]
  )

  const pollVideoTask = useCallback(
    async (
      taskId: string,
      messageKey: string,
      controller: AbortController,
      initialTask?: ReturnType<typeof parseVideoTaskResponse>
    ) => {
      let task = initialTask

      for (let attempt = 0; attempt < VIDEO_POLL_LIMIT; attempt += 1) {
        if (!task) {
          const taskResponse = await fetchPlaygroundVideoTask(
            taskId,
            controller.signal
          )
          task = parseVideoTaskResponse(taskResponse)
          if (!task) {
            await waitForVideoPoll(controller.signal)
            if (controller.signal.aborted) return
            continue
          }
        }

        if (task.status === 'failed') {
          throw new Error(task.error || i18next.t('Video generation failed'))
        }
        if (task.status === 'completed') {
          const video = await fetchVideoContent(taskId, controller.signal)
          if (
            typeof Blob === 'undefined' ||
            !(video instanceof Blob) ||
            video.size <= 0
          ) {
            throw new Error(i18next.t('Video generation failed'))
          }
          if (
            typeof URL === 'undefined' ||
            typeof URL.createObjectURL !== 'function'
          ) {
            throw new Error(i18next.t('Video generation failed'))
          }

          const url = URL.createObjectURL(video)
          generatedObjectURLsRef.current.add(url)
          try {
            const mediaType = video.type || 'video/mp4'
            const [uploadedVideo] = await uploadPlaygroundAttachments(
              [
                {
                  kind: 'video',
                  filename: 'generated-video.mp4',
                  mediaType,
                  url,
                },
              ],
              controller.signal
            )
            if (controller.signal.aborted) {
              releaseMediaObjectURL(url)
              return
            }
            if (!uploadedVideo) {
              throw new Error(
                i18next.t('Unable to upload Playground attachment') ||
                  'Unable to upload Playground attachment'
              )
            }
            const assetId = uploadedVideo.assetId?.trim()
            const durableURL =
              assetId && uploadedVideo.url ? uploadedVideo.url : url
            if (durableURL !== url) releaseMediaObjectURL(url)
            completeMedia(messageKey, i18next.t('Generated video'), [
              {
                type: 'video',
                url: durableURL,
                ...(assetId ? { assetId } : {}),
                mimeType: uploadedVideo.mediaType || mediaType,
              },
            ])
          } catch (error) {
            // A failed upload must not leave a process-local URL attached to
            // the message; only the durable asset can be restored later.
            releaseMediaObjectURL(url)
            throw error
          }
          return
        }

        updateProgress(
          messageKey,
          i18next.t('Generating video...'),
          task.progress,
          taskId
        )
        await waitForVideoPoll(controller.signal)
        if (controller.signal.aborted) return
        task = undefined
      }

      throw new Error(i18next.t('Video generation timed out'))
    },
    [completeMedia, releaseMediaObjectURL, updateProgress]
  )

  const pollVideoToMusicTask = useCallback(
    async (
      taskId: string,
      messageKey: string,
      controller: AbortController,
      initialTask?: ReturnType<typeof parseVideoToMusicTaskResponse>
    ) => {
      let task = initialTask

      for (let attempt = 0; attempt < VIDEO_POLL_LIMIT; attempt += 1) {
        if (!task) {
          const taskResponse = await fetchPlaygroundVideoToMusicTask(
            taskId,
            controller.signal
          )
          task = parseVideoToMusicTaskResponse(taskResponse)
          if (!task) {
            await waitForVideoPoll(controller.signal)
            if (controller.signal.aborted) return
            continue
          }
        }

        if (task.status === 'failed') {
          throw new Error(
            task.error ||
              i18next.t('Audio generation failed') ||
              'Audio generation failed'
          )
        }
        if (task.status === 'completed') {
          if (!task.audio?.length) {
            throw new Error(
              i18next.t('No audio was generated') || 'No audio was generated'
            )
          }
          completeMedia(messageKey, i18next.t('Audio'), task.audio)
          return
        }

        updateProgress(
          messageKey,
          i18next.t('Generating audio...'),
          task.progress,
          taskId,
          'video-to-music'
        )
        await waitForVideoPoll(controller.signal)
        if (controller.signal.aborted) return
        task = undefined
      }

      throw new Error(
        i18next.t('Audio generation timed out') || 'Audio generation timed out'
      )
    },
    [completeMedia, updateProgress]
  )

  const runVideoPolling = useCallback(
    async (
      taskId: string,
      messageKey: string,
      controller: AbortController,
      initialTask?: ReturnType<typeof parseVideoTaskResponse>
    ) => {
      try {
        await pollVideoTask(taskId, messageKey, controller, initialTask)
      } catch (error) {
        if (!controller.signal.aborted) {
          failMedia(messageKey, errorMessage(error))
        }
      } finally {
        if (abortControllerRef.current === controller) {
          abortControllerRef.current = null
          activeMessageKeyRef.current = null
          setIsGeneratingMedia(false)
        }
      }
    },
    [failMedia, pollVideoTask]
  )

  const runVideoToMusicPolling = useCallback(
    async (
      taskId: string,
      messageKey: string,
      controller: AbortController,
      initialTask?: ReturnType<typeof parseVideoToMusicTaskResponse>
    ) => {
      try {
        await pollVideoToMusicTask(taskId, messageKey, controller, initialTask)
      } catch (error) {
        if (!controller.signal.aborted) {
          failMedia(messageKey, errorMessage(error))
        }
      } finally {
        if (abortControllerRef.current === controller) {
          abortControllerRef.current = null
          activeMessageKeyRef.current = null
          setIsGeneratingMedia(false)
        }
      }
    },
    [failMedia, pollVideoToMusicTask]
  )

  const generateMedia = useCallback(
    async (
      prompt: string,
      model: string,
      group: string,
      settings: MediaGenerationSettings,
      assistantMessageKey: string,
      attachments: PlaygroundAttachment[] = []
    ) => {
      if (abortControllerRef.current) return

      const request = buildMediaGenerationRequest(
        prompt,
        model,
        group,
        settings,
        attachments
      )
      if (!request) {
        failMedia(
          assistantMessageKey,
          i18next.t('This model is not supported in Playground')
        )
        return
      }

      const controller = new AbortController()
      abortControllerRef.current = controller
      activeMessageKeyRef.current = assistantMessageKey
      setIsGeneratingMedia(true)

      try {
        const response = await sendMediaGeneration(request, controller.signal)
        if (controller.signal.aborted) return

        if (request.kind === 'video-to-music') {
          const submitted = parseVideoToMusicTaskResponse(response)
          if (!submitted) {
            throw new Error(
              i18next.t('Audio task could not be created') ||
                'Audio task could not be created'
            )
          }
          await pollVideoToMusicTask(
            submitted.taskId,
            assistantMessageKey,
            controller,
            submitted
          )
          return
        }

        if (request.kind === 'image') {
          if (request.endpoint === '/pg/chat/completions') {
            const content = responseMessageContent(response)
            if (!content) throw new Error(i18next.t('No image was generated'))
            completeMedia(assistantMessageKey, content, undefined)
            return
          }

          const outputFormat = request.payload.output_format
          const images = extractGeneratedImages(
            response,
            typeof outputFormat === 'string' ? outputFormat : 'png'
          )
          if (images.length === 0) {
            throw new Error(i18next.t('No image was generated'))
          }
          completeMedia(
            assistantMessageKey,
            i18next.t('Generated image'),
            images
          )
          return
        }

        if (request.kind === 'audio') {
          const audio = responseAudioBlob(response)
          if (!audio) {
            throw new Error(
              i18next.t('No audio was generated') || 'No audio was generated'
            )
          }
          if (
            typeof URL === 'undefined' ||
            typeof URL.createObjectURL !== 'function'
          ) {
            throw new Error(
              i18next.t('Audio playback failed') || 'Audio playback failed'
            )
          }
          const url = URL.createObjectURL(audio)
          generatedObjectURLsRef.current.add(url)
          try {
            const mediaType = audio.type || 'audio/wav'
            const extension =
              mediaType.split('/', 2)[1]?.split(';', 1)[0] || 'wav'
            const [uploadedAudio] = await uploadPlaygroundAttachments(
              [
                {
                  kind: 'audio',
                  filename: `generated-audio.${extension}`,
                  mediaType,
                  url,
                },
              ],
              controller.signal
            )
            if (controller.signal.aborted) {
              releaseMediaObjectURL(url)
              return
            }
            if (!uploadedAudio) {
              throw new Error(
                i18next.t('Unable to upload Playground attachment') ||
                  'Unable to upload Playground attachment'
              )
            }
            const assetId = uploadedAudio.assetId?.trim()
            const durableURL =
              assetId && uploadedAudio.url ? uploadedAudio.url : url
            if (durableURL !== url) releaseMediaObjectURL(url)
            completeMedia(assistantMessageKey, i18next.t('Audio'), [
              {
                type: 'audio',
                url: durableURL,
                ...(assetId ? { assetId } : {}),
                mimeType: uploadedAudio.mediaType || mediaType,
              },
            ])
          } catch (error) {
            // A failed upload never reaches the message state, so release the
            // local preview here instead of retaining it until unmount.
            releaseMediaObjectURL(url)
            throw error
          }
          return
        }

        const submitted = parseVideoTaskResponse(response)
        if (!submitted) {
          throw new Error(i18next.t('Video task could not be created'))
        }
        await pollVideoTask(
          submitted.taskId,
          assistantMessageKey,
          controller,
          submitted
        )
      } catch (error) {
        if (!controller.signal.aborted) {
          failMedia(assistantMessageKey, errorMessage(error))
        }
      } finally {
        if (abortControllerRef.current === controller) {
          abortControllerRef.current = null
          activeMessageKeyRef.current = null
          setIsGeneratingMedia(false)
        }
      }
    },
    [
      completeMedia,
      failMedia,
      pollVideoTask,
      pollVideoToMusicTask,
      releaseMediaObjectURL,
    ]
  )

  useEffect(() => {
    if (abortControllerRef.current) return
    const pendingMessage = findResumableVideoMessage(messages)
    if (!pendingMessage?.videoTaskId) return

    const controller = new AbortController()
    abortControllerRef.current = controller
    activeMessageKeyRef.current = pendingMessage.key
    queueMicrotask(() => {
      if (
        !controller.signal.aborted &&
        abortControllerRef.current === controller
      ) {
        setIsGeneratingMedia(true)
      }
    })
    if (pendingMessage.mediaTaskType === 'video-to-music') {
      void runVideoToMusicPolling(
        pendingMessage.videoTaskId,
        pendingMessage.key,
        controller
      )
    } else {
      void runVideoPolling(
        pendingMessage.videoTaskId,
        pendingMessage.key,
        controller
      )
    }
  }, [messages, runVideoPolling, runVideoToMusicPolling])

  useEffect(() => {
    const liveObjectURLs = collectLiveObjectURLs(messages)
    generatedObjectURLsRef.current.forEach((url) => {
      if (!liveObjectURLs.has(url)) releaseMediaObjectURL(url)
    })
  }, [messages, releaseMediaObjectURL])

  useEffect(() => {
    const generatedObjectURLs = generatedObjectURLsRef.current
    return () => {
      const controller = abortControllerRef.current
      abortControllerRef.current = null
      activeMessageKeyRef.current = null
      controller?.abort()
      if (
        typeof URL !== 'undefined' &&
        typeof URL.revokeObjectURL === 'function'
      ) {
        generatedObjectURLs.forEach((url) => URL.revokeObjectURL(url))
        generatedObjectURLs.clear()
      }
    }
  }, [])

  const stopMediaGeneration = useCallback(() => {
    const activeMessageKey = activeMessageKeyRef.current
    abortControllerRef.current?.abort()
    abortControllerRef.current = null
    activeMessageKeyRef.current = null
    setIsGeneratingMedia(false)
    onMessageUpdate((messages) =>
      messages.map((message) =>
        message.key === activeMessageKey
          ? clearVideoTaskId({
              ...updateCurrentVersionContent(
                message,
                i18next.t('Generation was interrupted')
              ),
              status: MESSAGE_STATUS.COMPLETE,
            })
          : message
      )
    )
  }, [onMessageUpdate])

  return {
    generateMedia,
    isGeneratingMedia,
    releaseMediaObjectURL,
    stopMediaGeneration,
  }
}
