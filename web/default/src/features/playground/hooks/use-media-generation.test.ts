/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of
the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/
import * as React from 'react'
import { afterAll, beforeEach, describe, expect, spyOn, test } from 'bun:test'
import { renderToStaticMarkup } from 'react-dom/server'
import * as playgroundApi from '../api'
import { MESSAGE_STATUS } from '../constants'
import * as playgroundAttachments from '../lib/playground-attachments'
import type { Message } from '../types'

const sendMediaGenerationMock = spyOn(playgroundApi, 'sendMediaGeneration')
const fetchPlaygroundVideoTaskMock = spyOn(
  playgroundApi,
  'fetchPlaygroundVideoTask'
)
const fetchVideoContentMock = spyOn(playgroundApi, 'fetchVideoContent')
const uploadPlaygroundAttachmentsMock = spyOn(
  playgroundAttachments,
  'uploadPlaygroundAttachments'
)
const { findResumableVideoMessage, useMediaGeneration, waitForVideoPoll } =
  await import('./use-media-generation')

const originalWindow = globalThis.window

beforeEach(() => {
  sendMediaGenerationMock.mockReset()
  fetchPlaygroundVideoTaskMock.mockReset()
  fetchVideoContentMock.mockReset()
  uploadPlaygroundAttachmentsMock.mockReset()
  uploadPlaygroundAttachmentsMock.mockImplementation(
    async (attachments) => attachments
  )
  Object.defineProperty(globalThis, 'window', {
    configurable: true,
    value: {
      clearTimeout: () => undefined,
      setTimeout: (callback: () => void) => {
        queueMicrotask(callback)
        return 1
      },
    },
  })
})

afterAll(() => {
  sendMediaGenerationMock.mockRestore()
  fetchPlaygroundVideoTaskMock.mockRestore()
  fetchVideoContentMock.mockRestore()
  uploadPlaygroundAttachmentsMock.mockRestore()
  if (originalWindow) {
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: originalWindow,
    })
  } else {
    Reflect.deleteProperty(globalThis, 'window')
  }
})

interface FakeSignal {
  aborted: boolean
  listeners: Set<(event: Event) => void>
  addEventListener: (
    type: string,
    listener: (event: Event) => void,
    options?: AddEventListenerOptions
  ) => void
  removeEventListener: (type: string, listener: (event: Event) => void) => void
}

function createFakeSignal(): FakeSignal {
  const signal: FakeSignal = {
    aborted: false,
    listeners: new Set(),
    addEventListener(type, listener) {
      if (type === 'abort') signal.listeners.add(listener)
    },
    removeEventListener(type, listener) {
      if (type === 'abort') signal.listeners.delete(listener)
    },
  }
  return signal
}

function createMediaMessages(): Message[] {
  return [
    {
      key: 'previous-assistant',
      from: 'assistant',
      status: MESSAGE_STATUS.COMPLETE,
      versions: [{ id: 'previous-version', content: 'Previous result' }],
    },
    {
      key: 'target-assistant',
      from: 'assistant',
      status: MESSAGE_STATUS.LOADING,
      versions: [{ id: 'target-version', content: '' }],
    },
  ]
}

function renderMediaGenerationHook(initialMessages: Message[]) {
  let messages = initialMessages
  const updates: Message[][] = []
  let hook: ReturnType<typeof useMediaGeneration> | undefined

  function Harness() {
    hook = useMediaGeneration({
      messages,
      onMessageUpdate: (updater) => {
        messages = updater(messages)
        updates.push(messages)
      },
    })
    return null
  }

  renderToStaticMarkup(React.createElement(Harness))
  if (!hook) throw new Error('Media generation hook was not rendered')

  return {
    hook,
    messages: () => messages,
    updates,
  }
}

const seedanceSettings = {
  resolution: '720p',
  aspectRatio: '16:9',
  duration: 5,
  generateAudio: false,
}

describe('waitForVideoPoll', () => {
  test('removes its abort listener after the poll timer resolves', async () => {
    const signal = createFakeSignal()
    let resolveTimer: (() => void) | undefined

    const wait = waitForVideoPoll(
      signal as unknown as AbortSignal,
      (callback) => {
        resolveTimer = callback
        return 1
      },
      () => undefined
    )

    expect(signal.listeners.size).toBe(1)
    resolveTimer?.()
    await wait
    expect(signal.listeners.size).toBe(0)
  })

  test('clears its poll timer and removes its listener when aborted', async () => {
    const signal = createFakeSignal()
    let clearCount = 0
    const wait = waitForVideoPoll(
      signal as unknown as AbortSignal,
      () => 1,
      () => {
        clearCount += 1
      }
    )

    const [listener] = [...signal.listeners]
    signal.aborted = true
    listener?.(new Event('abort'))
    await wait

    expect(clearCount).toBe(1)
    expect(signal.listeners.size).toBe(0)
  })
})

describe('useMediaGeneration video task lifecycle', () => {
  test('selects the newest in-flight persisted video task for reload recovery', () => {
    const messages: Message[] = [
      {
        key: 'older-task',
        from: 'assistant',
        status: MESSAGE_STATUS.STREAMING,
        versions: [{ id: 'older-version', content: 'Generating video...' }],
        videoTaskId: 'video-task-older',
      },
      {
        key: 'completed-task',
        from: 'assistant',
        status: MESSAGE_STATUS.COMPLETE,
        versions: [{ id: 'completed-version', content: 'Generated video' }],
        videoTaskId: 'video-task-completed',
      },
      {
        key: 'newer-task',
        from: 'assistant',
        status: MESSAGE_STATUS.LOADING,
        versions: [{ id: 'newer-version', content: '' }],
        videoTaskId: 'video-task-newer',
      },
    ]

    expect(findResumableVideoMessage(messages)?.key).toBe('newer-task')
  })

  test('downloads completed video and stores its durable asset reference', async () => {
    const video = new Blob(['ftyp'], { type: 'video/mp4' })
    sendMediaGenerationMock.mockResolvedValue({
      id: 'video-task-123',
      status: 'queued',
      progress: 10,
    })
    fetchPlaygroundVideoTaskMock.mockResolvedValue({
      id: 'video-task-123',
      status: 'completed',
      url: 'https://cdn.example.com/video.mp4',
    })
    fetchVideoContentMock.mockResolvedValue(video)
    uploadPlaygroundAttachmentsMock.mockResolvedValue([
      {
        kind: 'video',
        filename: 'generated-video.mp4',
        mediaType: 'video/mp4',
        assetId: 'ast_generated_video',
        url: 'https://storage.example/generated.mp4',
      },
    ])
    const originalCreateObjectURL = URL.createObjectURL
    const originalRevokeObjectURL = URL.revokeObjectURL
    const revoked: string[] = []
    URL.createObjectURL = (() =>
      'blob:generated-video') as typeof URL.createObjectURL
    URL.revokeObjectURL = ((url: string) => {
      revoked.push(url)
    }) as typeof URL.revokeObjectURL

    try {
      const harness = renderMediaGenerationHook(createMediaMessages())

      await harness.hook.generateMedia(
        'A ship at sea',
        'seedance-2.0',
        'default',
        seedanceSettings,
        'target-assistant'
      )

      const submitted = harness.updates[0]
      expect(submitted?.[0]?.versions[0]?.content).toBe('Previous result')
      expect(submitted?.[1]).toMatchObject({
        key: 'target-assistant',
        status: MESSAGE_STATUS.STREAMING,
        videoTaskId: 'video-task-123',
      })
      expect(fetchVideoContentMock).toHaveBeenCalledTimes(1)
      expect(fetchVideoContentMock.mock.calls[0]?.[0]).toBe('video-task-123')
      expect(uploadPlaygroundAttachmentsMock).toHaveBeenCalledTimes(1)
      expect(uploadPlaygroundAttachmentsMock.mock.calls[0]?.[0]).toEqual([
        {
          kind: 'video',
          filename: 'generated-video.mp4',
          mediaType: 'video/mp4',
          url: 'blob:generated-video',
        },
      ])

      const completed = harness.messages()[1]
      expect(completed).toMatchObject({
        key: 'target-assistant',
        status: MESSAGE_STATUS.COMPLETE,
        versions: [
          {
            generatedMedia: [
              {
                type: 'video',
                assetId: 'ast_generated_video',
                url: 'https://storage.example/generated.mp4',
                mimeType: 'video/mp4',
              },
            ],
          },
        ],
      })
      expect('videoTaskId' in completed).toBe(false)
      expect(revoked).toEqual(['blob:generated-video'])
    } finally {
      URL.createObjectURL = originalCreateObjectURL
      URL.revokeObjectURL = originalRevokeObjectURL
    }
  })

  test('releases the local video preview when durable upload fails', async () => {
    sendMediaGenerationMock.mockResolvedValue({
      id: 'video-task-upload-failed',
      status: 'queued',
    })
    fetchPlaygroundVideoTaskMock.mockResolvedValue({
      id: 'video-task-upload-failed',
      status: 'completed',
    })
    fetchVideoContentMock.mockResolvedValue(
      new Blob(['ftyp'], { type: 'video/mp4' })
    )
    uploadPlaygroundAttachmentsMock.mockRejectedValue(
      new Error('video upload failed')
    )
    const originalCreateObjectURL = URL.createObjectURL
    const originalRevokeObjectURL = URL.revokeObjectURL
    const revoked: string[] = []
    URL.createObjectURL = (() =>
      'blob:failed-generated-video') as typeof URL.createObjectURL
    URL.revokeObjectURL = ((url: string) => {
      revoked.push(url)
    }) as typeof URL.revokeObjectURL

    try {
      const harness = renderMediaGenerationHook(createMediaMessages())
      await harness.hook.generateMedia(
        'Release this video preview',
        'seedance-2.0',
        'default',
        seedanceSettings,
        'target-assistant'
      )

      expect(harness.messages()[1]?.status).toBe(MESSAGE_STATUS.ERROR)
      expect(harness.messages()[1]?.versions[0]?.generatedMedia).toBeUndefined()
      expect(revoked).toEqual(['blob:failed-generated-video'])
    } finally {
      URL.createObjectURL = originalCreateObjectURL
      URL.revokeObjectURL = originalRevokeObjectURL
    }
  })

  test('clears the persisted task id when the video task fails', async () => {
    sendMediaGenerationMock.mockResolvedValue({
      id: 'video-task-failed',
      status: 'queued',
    })
    fetchPlaygroundVideoTaskMock.mockResolvedValue({
      id: 'video-task-failed',
      status: 'failed',
      error: 'Upstream rejected the task',
    })
    const harness = renderMediaGenerationHook(createMediaMessages())

    await harness.hook.generateMedia(
      'A ship at sea',
      'seedance-2.0',
      'default',
      seedanceSettings,
      'target-assistant'
    )

    const failed = harness.messages()[1]
    expect(failed.status).toBe(MESSAGE_STATUS.ERROR)
    expect(failed.versions[0]?.content).toContain('Upstream rejected the task')
    expect('videoTaskId' in failed).toBe(false)
  })

  test('clears the persisted task id when the active generation is stopped', async () => {
    let resolveSubmission: ((value: unknown) => void) | undefined
    sendMediaGenerationMock.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveSubmission = resolve
        })
    )
    const messages = createMediaMessages()
    messages[1] = {
      ...messages[1],
      status: MESSAGE_STATUS.STREAMING,
      videoTaskId: 'video-task-stopped',
    }
    const harness = renderMediaGenerationHook(messages)

    const generation = harness.hook.generateMedia(
      'A ship at sea',
      'seedance-2.0',
      'default',
      seedanceSettings,
      'target-assistant'
    )
    harness.hook.stopMediaGeneration()
    resolveSubmission?.({ id: 'video-task-stopped', status: 'queued' })
    await generation

    const stopped = harness.messages()[1]
    expect(stopped.status).toBe(MESSAGE_STATUS.COMPLETE)
    expect('videoTaskId' in stopped).toBe(false)
  })

  test('does not replace an active media generation with a second request', async () => {
    let resolveSubmission: ((value: unknown) => void) | undefined
    sendMediaGenerationMock.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveSubmission = resolve
        })
    )
    const harness = renderMediaGenerationHook(createMediaMessages())

    const firstGeneration = harness.hook.generateMedia(
      'A ship at sea',
      'seedance-2.0',
      'default',
      seedanceSettings,
      'target-assistant'
    )
    void harness.hook.generateMedia(
      'A second ship at sea',
      'seedance-2.0',
      'default',
      seedanceSettings,
      'target-assistant'
    )

    expect(sendMediaGenerationMock).toHaveBeenCalledTimes(1)

    harness.hook.stopMediaGeneration()
    resolveSubmission?.({ id: 'video-task-stopped', status: 'queued' })
    await firstGeneration
  })
})

describe('useMediaGeneration audio lifecycle', () => {
  test('uploads generated audio and stores its durable asset reference', async () => {
    const audio = new Blob(['RIFF'], { type: 'audio/wav' })
    sendMediaGenerationMock.mockResolvedValue(audio)
    uploadPlaygroundAttachmentsMock.mockResolvedValue([
      {
        kind: 'audio',
        filename: 'generated-audio.wav',
        mediaType: 'audio/wav',
        assetId: 'ast_generated_audio',
        url: 'https://storage.example/generated.wav',
      },
    ])
    const originalCreateObjectURL = URL.createObjectURL
    URL.createObjectURL = (() =>
      'blob:generated-audio') as typeof URL.createObjectURL

    try {
      const harness = renderMediaGenerationHook(createMediaMessages())
      await harness.hook.generateMedia(
        'Persist this audio',
        'tts-1',
        'plg',
        { voice: 'alloy', responseFormat: 'wav' },
        'target-assistant'
      )

      expect(uploadPlaygroundAttachmentsMock).toHaveBeenCalledTimes(1)
      expect(uploadPlaygroundAttachmentsMock.mock.calls[0]?.[0]).toEqual([
        {
          kind: 'audio',
          filename: 'generated-audio.wav',
          mediaType: 'audio/wav',
          url: 'blob:generated-audio',
        },
      ])
      expect(harness.messages()[1]).toMatchObject({
        status: MESSAGE_STATUS.COMPLETE,
        versions: [
          {
            generatedMedia: [
              {
                type: 'audio',
                assetId: 'ast_generated_audio',
                url: 'https://storage.example/generated.wav',
                mimeType: 'audio/wav',
              },
            ],
          },
        ],
      })
    } finally {
      URL.createObjectURL = originalCreateObjectURL
    }
  })

  test('releases the local audio preview when durable upload fails', async () => {
    const audio = new Blob(['RIFF'], { type: 'audio/wav' })
    sendMediaGenerationMock.mockResolvedValue(audio)
    uploadPlaygroundAttachmentsMock.mockRejectedValue(
      new Error('upload failed')
    )
    const originalCreateObjectURL = URL.createObjectURL
    const originalRevokeObjectURL = URL.revokeObjectURL
    const revoked: string[] = []
    URL.createObjectURL = (() =>
      'blob:failed-generated-audio') as typeof URL.createObjectURL
    URL.revokeObjectURL = ((url: string) => {
      revoked.push(url)
    }) as typeof URL.revokeObjectURL

    try {
      const harness = renderMediaGenerationHook(createMediaMessages())
      await harness.hook.generateMedia(
        'Release failed audio',
        'tts-1',
        'plg',
        { voice: 'alloy' },
        'target-assistant'
      )

      expect(harness.messages()[1]?.status).toBe(MESSAGE_STATUS.ERROR)
      expect(revoked).toEqual(['blob:failed-generated-audio'])
    } finally {
      URL.createObjectURL = originalCreateObjectURL
      URL.revokeObjectURL = originalRevokeObjectURL
    }
  })

  test('turns the binary speech response into generated audio media', async () => {
    const audio = new Blob(['RIFF\x00\x00\x00\x00WAVE'], {
      type: 'audio/wav',
    })
    sendMediaGenerationMock.mockResolvedValue(audio)
    const createObjectURL = URL.createObjectURL
    URL.createObjectURL = (() =>
      'blob:audio-result') as typeof URL.createObjectURL

    try {
      const harness = renderMediaGenerationHook(createMediaMessages())
      await harness.hook.generateMedia(
        'Hello from Playground',
        'tts-1',
        'plg',
        { voice: 'alloy', responseFormat: 'wav', speed: 1 },
        'target-assistant'
      )

      expect(harness.messages()[1]).toMatchObject({
        status: MESSAGE_STATUS.COMPLETE,
        versions: [
          {
            generatedMedia: [{ type: 'audio', url: 'blob:audio-result' }],
          },
        ],
      })
    } finally {
      URL.createObjectURL = createObjectURL
    }
  })

  test('rejects a non-audio binary response', async () => {
    sendMediaGenerationMock.mockResolvedValue(
      new Blob(['not audio'], { type: 'application/json' })
    )
    const harness = renderMediaGenerationHook(createMediaMessages())

    await harness.hook.generateMedia(
      'Hello from Playground',
      'tts-1',
      'plg',
      { voice: 'alloy', responseFormat: 'wav', speed: 1 },
      'target-assistant'
    )

    expect(harness.messages()[1].status).toBe(MESSAGE_STATUS.ERROR)
    expect(harness.messages()[1].versions[0]?.content).toContain(
      'No audio was generated'
    )
  })

  test('releases a generated audio URL only once', async () => {
    const audio = new Blob(['RIFF'], { type: 'audio/wav' })
    sendMediaGenerationMock.mockResolvedValue(audio)
    const originalCreateObjectURL = URL.createObjectURL
    const originalRevokeObjectURL = URL.revokeObjectURL
    const revoked: string[] = []
    URL.createObjectURL = (() =>
      'blob:audio-release') as typeof URL.createObjectURL
    URL.revokeObjectURL = ((url: string) => {
      revoked.push(url)
    }) as typeof URL.revokeObjectURL

    try {
      const harness = renderMediaGenerationHook(createMediaMessages())
      await harness.hook.generateMedia(
        'Release this audio',
        'tts-1',
        'plg',
        { voice: 'alloy' },
        'target-assistant'
      )

      harness.hook.releaseMediaObjectURL('blob:audio-release')
      harness.hook.releaseMediaObjectURL('blob:audio-release')
      expect(revoked).toEqual(['blob:audio-release'])
    } finally {
      URL.createObjectURL = originalCreateObjectURL
      URL.revokeObjectURL = originalRevokeObjectURL
    }
  })
})
