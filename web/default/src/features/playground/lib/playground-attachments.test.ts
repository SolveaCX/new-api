import { afterEach, describe, expect, spyOn, test } from 'bun:test'
import { api } from '@/lib/api'
import type { Message, PlaygroundAttachment } from '../types'
import { createUserMessage, formatMessageForAPI } from './message-utils'
import {
  hydratePlaygroundMessages,
  isPlaygroundAttachmentUploadAvailable,
  uploadPlaygroundAttachments,
} from './playground-attachments'

const originalWindow = globalThis.window
const originalFetch = globalThis.fetch

function enableBrowserUpload() {
  Object.defineProperty(globalThis, 'window', {
    configurable: true,
    value: { location: { origin: 'https://playground.example' } },
  })
}

afterEach(() => {
  if (originalWindow === undefined) {
    Reflect.deleteProperty(globalThis, 'window')
  } else {
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: originalWindow,
    })
  }
  globalThis.fetch = originalFetch
})

describe('Playground durable attachments', () => {
  test('uploads media before returning a durable asset reference', async () => {
    enableBrowserUpload()
    const post = spyOn(api, 'post')
      .mockResolvedValueOnce({
        data: {
          success: true,
          data: {
            upload_id: 'upl_123',
            asset_id: 'ast_123',
            upload_url: 'https://storage.example/upload',
            upload_headers: { 'Content-Type': 'image/png' },
            expires_at: 9999999999,
          },
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          success: true,
          data: {
            asset_id: 'ast_123',
            asset_type: 'Image',
            content_type: 'image/png',
            size_bytes: 1,
            preview_url: 'https://storage.example/preview',
            expires_at: 9999999999,
          },
        },
      } as never)
    globalThis.fetch = (async () => ({ ok: true, status: 200 })) as typeof fetch

    const attachment: PlaygroundAttachment = {
      kind: 'image',
      filename: 'frame.png',
      mediaType: 'image/png',
      url: 'data:image/png;base64,AA==',
    }
    const uploaded = await uploadPlaygroundAttachments([attachment])
    expect(uploaded).toEqual([
      {
        ...attachment,
        assetId: 'ast_123',
        url: 'https://storage.example/preview',
      },
    ])
    expect(
      formatMessageForAPI(createUserMessage('describe this', uploaded))
    ).toEqual({
      role: 'user',
      content: [
        { type: 'text', text: 'describe this' },
        {
          type: 'image_url',
          image_url: { url: 'https://storage.example/preview' },
        },
      ],
    })
    expect(post).toHaveBeenCalledTimes(2)
    expect(post.mock.calls[0]?.[0]).toBe('/api/playground/attachments/uploads')
    expect(post.mock.calls[1]?.[0]).toBe(
      '/api/playground/attachments/uploads/upl_123/complete'
    )
    post.mockRestore()
  })

  test('uploads PDF attachments and returns a durable document URL', async () => {
    enableBrowserUpload()
    const post = spyOn(api, 'post')
      .mockResolvedValueOnce({
        data: {
          success: true,
          data: {
            upload_id: 'upl_pdf',
            asset_id: 'ast_pdf',
            upload_url: 'https://storage.example/upload-pdf',
            upload_headers: { 'Content-Type': 'application/pdf' },
            expires_at: 9999999999,
          },
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          success: true,
          data: {
            asset_id: 'ast_pdf',
            asset_type: 'Document',
            content_type: 'application/pdf',
            size_bytes: 8,
            preview_url: 'https://storage.example/report.pdf',
            expires_at: 9999999999,
          },
        },
      } as never)
    let uploadedBody: BodyInit | null = null
    globalThis.fetch = (async (_input, init) => {
      uploadedBody = init?.body ?? null
      return { ok: true, status: 200 } as Response
    }) as typeof fetch

    const attachment: PlaygroundAttachment = {
      kind: 'document',
      filename: 'report.pdf',
      mediaType: 'application/pdf',
      url: 'data:application/pdf;base64,JVBERi0xLjQ=',
    }
    await expect(uploadPlaygroundAttachments([attachment])).resolves.toEqual([
      {
        ...attachment,
        assetId: 'ast_pdf',
        url: 'https://storage.example/report.pdf',
      },
    ])
    expect(uploadedBody).toBeInstanceOf(Blob)
    expect(post.mock.calls[0]?.[1]).toMatchObject({
      asset_type: 'Document',
      content_type: 'application/pdf',
      size_bytes: 8,
    })
    post.mockRestore()
  })

  test('uploads audio attachments and preserves the inline audio payload', async () => {
    enableBrowserUpload()
    const post = spyOn(api, 'post')
      .mockResolvedValueOnce({
        data: {
          success: true,
          data: {
            upload_id: 'upl_audio',
            asset_id: 'ast_audio',
            upload_url: 'https://storage.example/upload-audio',
            upload_headers: { 'Content-Type': 'audio/mpeg' },
            expires_at: 9999999999,
          },
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          success: true,
          data: {
            asset_id: 'ast_audio',
            asset_type: 'Audio',
            content_type: 'audio/mpeg',
            size_bytes: 4,
            preview_url: 'https://storage.example/audio.mp3',
            expires_at: 9999999999,
          },
        },
      } as never)
    globalThis.fetch = (async (_input, init) => ({
      ok: true,
      status: 200,
      body: init?.body ?? null,
    })) as typeof fetch

    const attachment: PlaygroundAttachment = {
      kind: 'audio',
      filename: 'clip.mp3',
      mediaType: 'audio/mpeg',
      url: 'data:audio/mpeg;base64,AA==',
    }
    await expect(uploadPlaygroundAttachments([attachment])).resolves.toEqual([
      {
        ...attachment,
        assetId: 'ast_audio',
        url: 'https://storage.example/audio.mp3',
        dataUrl: 'data:audio/mpeg;base64,AA==',
      },
    ])
    expect(post.mock.calls[0]?.[1]).toMatchObject({
      asset_type: 'Audio',
      content_type: 'audio/mpeg',
      size_bytes: 1,
    })
    post.mockRestore()
  })

  test('rehydrates audio attachments with inline data for later sends', async () => {
    enableBrowserUpload()
    const get = spyOn(api, 'get').mockResolvedValue({
      data: {
        success: true,
        data: {
          asset_id: 'ast_audio',
          asset_type: 'Audio',
          content_type: 'audio/mpeg',
          size_bytes: 1,
          preview_url: 'https://storage.example/audio.mp3',
          expires_at: 9999999999,
        },
      },
    } as never)
    globalThis.fetch = (async () =>
      new Response(
        new Blob([Uint8Array.from([0])], { type: 'audio/mpeg' })
      )) as typeof fetch

    const messages: Message[] = [
      {
        key: 'user-audio',
        from: 'user',
        versions: [
          {
            id: 'v1',
            content: 'transcribe this',
            attachments: [
              {
                kind: 'audio',
                filename: 'clip.mp3',
                mediaType: 'audio/mpeg',
                assetId: 'ast_audio',
                url: 'https://storage.example/audio.mp3',
              },
            ],
          },
        ],
      },
    ]

    const hydrated = await hydratePlaygroundMessages(messages)

    expect(hydrated[0]?.versions[0]?.attachments?.[0]).toMatchObject({
      kind: 'audio',
      filename: 'clip.mp3',
      mediaType: 'audio/mpeg',
      assetId: 'ast_audio',
      url: 'https://storage.example/audio.mp3',
      dataUrl: 'data:audio/mpeg;base64,AA==',
    })
    expect(get).toHaveBeenCalledTimes(1)
    get.mockRestore()
  })

  test('hydrates duplicate asset IDs once after a server restore', async () => {
    enableBrowserUpload()
    const get = spyOn(api, 'get').mockResolvedValue({
      data: {
        success: true,
        data: {
          asset_id: 'ast_123',
          asset_type: 'Video',
          content_type: 'video/mp4',
          size_bytes: 1,
          preview_url: 'https://storage.example/video.mp4',
          expires_at: 9999999999,
        },
      },
    } as never)
    const attachment = {
      kind: 'video' as const,
      filename: 'reference.mp4',
      mediaType: 'video/mp4',
      assetId: 'ast_123',
    }
    const messages: Message[] = [
      {
        key: 'user-1',
        from: 'user',
        versions: [{ id: 'v1', content: 'animate', attachments: [attachment] }],
      },
      {
        key: 'user-2',
        from: 'user',
        versions: [{ id: 'v2', content: 'again', attachments: [attachment] }],
      },
    ]

    const hydrated = await hydratePlaygroundMessages(messages)

    expect(hydrated[0]?.versions[0]?.attachments?.[0]).toMatchObject({
      assetId: 'ast_123',
      url: 'https://storage.example/video.mp4',
    })
    expect(hydrated[1]?.versions[0]?.attachments?.[0]).toMatchObject({
      assetId: 'ast_123',
      url: 'https://storage.example/video.mp4',
    })
    expect(get).toHaveBeenCalledTimes(1)
    get.mockRestore()
  })

  test('keeps video metadata visible when preview hydration fails', async () => {
    enableBrowserUpload()
    const get = spyOn(api, 'get').mockRejectedValue(new Error('preview 404'))
    const messages: Message[] = [
      {
        key: 'user-video-unavailable',
        from: 'user',
        versions: [
          {
            id: 'v1',
            content: 'describe this',
            attachments: [
              {
                kind: 'video',
                filename: 'reference.mp4',
                mediaType: 'video/mp4',
                assetId: 'ast_missing_preview',
                url: 'https://storage.example/expired-preview',
              },
            ],
          },
        ],
      },
    ]

    const hydrated = await hydratePlaygroundMessages(messages)

    expect(hydrated[0]?.versions[0]?.attachments).toEqual([
      {
        kind: 'video',
        filename: 'reference.mp4',
        mediaType: 'video/mp4',
        assetId: 'ast_missing_preview',
      },
    ])
    expect(get).toHaveBeenCalledTimes(1)
    get.mockRestore()
  })

  test('refreshes a durable asset preview before a later send', async () => {
    enableBrowserUpload()
    const get = spyOn(api, 'get').mockResolvedValue({
      data: {
        success: true,
        data: {
          asset_id: 'ast_123',
          asset_type: 'Image',
          content_type: 'image/png',
          size_bytes: 1,
          preview_url: 'https://storage.example/fresh-preview',
          expires_at: 9999999999,
        },
      },
    } as never)
    const attachment: PlaygroundAttachment = {
      kind: 'image',
      filename: 'frame.png',
      mediaType: 'image/png',
      assetId: 'ast_123',
      url: 'https://storage.example/expired-preview',
    }

    await expect(uploadPlaygroundAttachments([attachment])).resolves.toEqual([
      {
        ...attachment,
        url: 'https://storage.example/fresh-preview',
      },
    ])
    expect(get).toHaveBeenCalledTimes(1)
    expect(get.mock.calls[0]?.[0]).toBe(
      '/api/playground/attachments/ast_123/preview'
    )
    get.mockRestore()
  })

  test('rejects a preview response for a different durable asset', async () => {
    enableBrowserUpload()
    const get = spyOn(api, 'get').mockResolvedValue({
      data: {
        success: true,
        data: {
          asset_id: 'ast_other',
          asset_type: 'Image',
          content_type: 'image/png',
          size_bytes: 1,
          preview_url: 'https://storage.example/other',
          expires_at: 9999999999,
        },
      },
    } as never)
    const attachment: PlaygroundAttachment = {
      kind: 'image',
      filename: 'frame.png',
      mediaType: 'image/png',
      assetId: 'ast_123',
    }

    await expect(uploadPlaygroundAttachments([attachment])).rejects.toThrow(
      'Playground attachment preview mismatch'
    )
    get.mockRestore()
  })

  test('does not enable network upload in a static-render environment', () => {
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: { localStorage: {} },
    })
    expect(isPlaygroundAttachmentUploadAvailable()).toBe(false)
  })
})
