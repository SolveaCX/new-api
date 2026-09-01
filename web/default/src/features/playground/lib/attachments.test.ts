import type { FileUIPart } from 'ai'
import { describe, expect, spyOn, test } from 'bun:test'
import {
  MAX_FILE_BYTES,
  normalizePlaygroundAttachments,
  readPlaygroundVideoDuration,
} from './attachments'

function textDataUrl(text: string): string {
  return `data:text/plain;base64,${Buffer.from(text, 'utf8').toString('base64')}`
}

function videoDataUrl(): string {
  return 'data:video/mp4;base64,AA=='
}

function file(filename: string, mediaType: string, url: string): FileUIPart {
  return { type: 'file', filename, mediaType, url }
}

describe('normalizePlaygroundAttachments', () => {
  test('reads finite video metadata duration and cleans up the element', async () => {
    const originalDocument = globalThis.document
    const listeners = new Map<string, () => void>()
    let removedSource = false
    const fakeVideo = {
      duration: 12,
      preload: '',
      src: '',
      addEventListener: (name: string, listener: () => void) => {
        listeners.set(name, listener)
      },
      removeEventListener: (name: string) => {
        listeners.delete(name)
      },
      removeAttribute: (name: string) => {
        if (name === 'src') removedSource = true
      },
      load: () => undefined,
    }

    Object.defineProperty(globalThis, 'document', {
      configurable: true,
      value: {
        createElement: (tag: string) => {
          expect(tag).toBe('video')
          return fakeVideo
        },
      },
    })

    try {
      const resultPromise = readPlaygroundVideoDuration(
        'data:video/mp4;base64,AA=='
      )
      listeners.get('loadedmetadata')?.()
      const result = await resultPromise
      expect(result).toBe(12)
      expect(fakeVideo.preload).toBe('metadata')
      expect(removedSource).toBe(true)
      expect(listeners.size).toBe(0)
    } finally {
      Object.defineProperty(globalThis, 'document', {
        configurable: true,
        value: originalDocument,
      })
    }
  })

  test('normalizes image data URLs and supported text files', async () => {
    const result = await normalizePlaygroundAttachments([
      file('photo.png', 'image/png', 'data:image/png;base64,AA=='),
      file('notes.md', 'text/markdown', textDataUrl('# Notes')),
    ])

    expect(result).toEqual([
      {
        kind: 'image',
        filename: 'photo.png',
        mediaType: 'image/png',
        url: 'data:image/png;base64,AA==',
      },
      {
        kind: 'text',
        filename: 'notes.md',
        mediaType: 'text/markdown',
        text: '# Notes',
      },
    ])
  })

  test('canonicalizes the image/jpg alias for image-to-video uploads', async () => {
    await expect(
      normalizePlaygroundAttachments([
        file('frame.jpg', 'image/jpg', 'data:image/jpg;base64,/9j/2w=='),
      ])
    ).resolves.toEqual([
      {
        kind: 'image',
        filename: 'frame.jpg',
        mediaType: 'image/jpeg',
        url: 'data:image/jpg;base64,/9j/2w==',
      },
    ])
  })

  test('normalizes supported audio attachments for audio-capable chat models', async () => {
    await expect(
      normalizePlaygroundAttachments([
        file('voice.mp3', 'audio/mp3', 'data:audio/mp3;base64,AA=='),
      ])
    ).resolves.toEqual([
      {
        kind: 'audio',
        filename: 'voice.mp3',
        mediaType: 'audio/mpeg',
        url: 'data:audio/mp3;base64,AA==',
        dataUrl: 'data:audio/mp3;base64,AA==',
      },
    ])
  })

  test('normalizes supported mp4 attachments as video files', async () => {
    await expect(
      normalizePlaygroundAttachments([
        file('reference.mp4', 'video/mp4', videoDataUrl()),
      ])
    ).resolves.toEqual([
      {
        kind: 'video',
        filename: 'reference.mp4',
        mediaType: 'video/mp4',
        url: videoDataUrl(),
      },
    ])
  })

  test('normalizes PDF attachments as durable document candidates', async () => {
    await expect(
      normalizePlaygroundAttachments([
        file(
          'report.pdf',
          'application/pdf',
          'data:application/pdf;base64,JVBERi0xLjQ='
        ),
      ])
    ).resolves.toEqual([
      {
        kind: 'document',
        filename: 'report.pdf',
        mediaType: 'application/pdf',
        url: 'data:application/pdf;base64,JVBERi0xLjQ=',
      },
    ])
  })

  test('normalizes audio attachments as durable audio candidates', async () => {
    await expect(
      normalizePlaygroundAttachments([
        file('clip.mp3', 'audio/mpeg', 'data:audio/mpeg;base64,AA=='),
      ])
    ).resolves.toEqual([
      {
        kind: 'audio',
        filename: 'clip.mp3',
        mediaType: 'audio/mpeg',
        url: 'data:audio/mpeg;base64,AA==',
        dataUrl: 'data:audio/mpeg;base64,AA==',
      },
    ])
  })

  test('keeps durable asset references when a draft is restored', async () => {
    await expect(
      normalizePlaygroundAttachments([
        {
          type: 'file',
          filename: 'reference.mp4',
          mediaType: 'video/mp4',
          assetId: 'ast_video',
          url: 'https://storage.example/preview',
        },
      ])
    ).resolves.toEqual([
      {
        kind: 'video',
        filename: 'reference.mp4',
        mediaType: 'video/mp4',
        assetId: 'ast_video',
        url: 'https://storage.example/preview',
      },
    ])
  })

  test('rejects unsupported extensions and empty files', async () => {
    await expect(
      normalizePlaygroundAttachments([
        file(
          'archive.bin',
          'application/octet-stream',
          'data:application/octet-stream;base64,AA=='
        ),
      ])
    ).rejects.toThrow('Unsupported attachment type')

    await expect(
      normalizePlaygroundAttachments([
        file('empty.txt', 'text/plain', textDataUrl('')),
      ])
    ).rejects.toThrow('Attachment is empty')
  })

  test('limits the number of attachments', async () => {
    const files = Array.from({ length: 6 }, (_, index) =>
      file(`file-${index}.txt`, 'text/plain', textDataUrl('x'))
    )

    await expect(normalizePlaygroundAttachments(files)).rejects.toThrow(
      'Too many attachments'
    )
  })

  test('rejects oversized data before decoding the base64 payload', async () => {
    const atobSpy = spyOn(globalThis, 'atob')
    const encoded = 'A'.repeat(Math.ceil((MAX_FILE_BYTES * 4) / 3) + 4)

    await expect(
      normalizePlaygroundAttachments([
        file('large.png', 'image/png', `data:image/png;base64,${encoded}`),
      ])
    ).rejects.toThrow('Attachment exceeds the maximum size')
    expect(atobSpy).not.toHaveBeenCalled()
    atobSpy.mockRestore()
  })
})
