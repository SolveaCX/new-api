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
import {
  afterAll,
  beforeEach,
  describe,
  expect,
  setSystemTime,
  spyOn,
  test,
} from 'bun:test'
import { api } from '@/lib/api'
import { PLAYGROUND_RECORDS_EXPORT } from './constants'
import type { PlaygroundRecordPayload } from './types'

const get = spyOn(api, 'get').mockResolvedValue({
  data: { success: true },
} as never)
const post = spyOn(api, 'post').mockResolvedValue({
  data: { success: true },
} as never)

const {
  completePlaygroundAttachmentUpload,
  clearCurrentPlaygroundRecord,
  createPlaygroundAttachmentUploadSession,
  downloadPlaygroundRecords,
  fetchVideoContent,
  getPlaygroundAttachmentPreview,
  getCurrentPlaygroundRecord,
  getUserModels: fetchUserModels,
  sendMediaGeneration,
  savePlaygroundRecord,
} = await import('./api')

const payload = {
  record_id: '550e8400-e29b-41d4-a716-446655440000',
  conversation_id: '550e8400-e29b-41d4-a716-446655440001',
} as PlaygroundRecordPayload

afterAll(() => {
  get.mockRestore()
  post.mockRestore()
})

describe('Playground persistence API', () => {
  beforeEach(() => {
    get.mockClear()
    post.mockClear()
  })

  test('saves a terminal record through the authenticated endpoint', async () => {
    post.mockResolvedValueOnce({ data: { success: true } })

    await savePlaygroundRecord(payload)

    expect(post).toHaveBeenCalledWith('/api/playground/records', payload)
  })

  test('returns the current conversation snapshot', async () => {
    const current = {
      conversation_id: payload.conversation_id,
      messages: [],
    }
    get.mockResolvedValueOnce({ data: { success: true, data: current } })

    await expect(getCurrentPlaygroundRecord()).resolves.toEqual(current)
    expect(get).toHaveBeenCalledWith('/api/playground/records/current')
  })

  test('preserves an explicit null current conversation', async () => {
    get.mockResolvedValueOnce({ data: { success: true, data: null } })

    await expect(getCurrentPlaygroundRecord()).resolves.toBeNull()
  })

  test('surfaces an API failure message', async () => {
    post.mockResolvedValueOnce({
      data: { success: false, message: 'save failed' },
    })

    await expect(savePlaygroundRecord(payload)).rejects.toThrow('save failed')
  })

  test('clears only after the authenticated API accepts the marker', async () => {
    post.mockResolvedValueOnce({ data: { success: true } })

    await clearCurrentPlaygroundRecord(
      payload.record_id,
      payload.conversation_id,
      2500
    )

    expect(post).toHaveBeenCalledWith('/api/playground/records/clear', {
      record_id: payload.record_id,
      conversation_id: payload.conversation_id,
      client_completed_at: 2500,
    })
  })
})

describe('Playground model API', () => {
  beforeEach(() => {
    get.mockClear()
  })

  test('asks the backend to exclude administratively hidden models', async () => {
    get.mockResolvedValueOnce({
      data: {
        success: true,
        data: ['gpt-4o', ' seedance-2.5 ', null],
      },
    })

    await expect(fetchUserModels('plg')).resolves.toEqual([
      'gpt-4o',
      'seedance-2.5',
    ])
    expect(get).toHaveBeenCalledWith('/api/user/models', {
      params: { group: 'plg', exclude_hidden: true },
    })
  })
})

describe('Playground media API', () => {
  beforeEach(() => {
    get.mockClear()
    post.mockClear()
  })

  test('requests generated audio as a binary blob', async () => {
    const audio = new Blob(['RIFF'], { type: 'audio/wav' })
    post.mockResolvedValueOnce({ data: audio })

    const response = await sendMediaGeneration({
      kind: 'audio',
      endpoint: '/pg/audio/speech',
      payload: { model: 'tts-1', input: 'hello' },
    })

    expect(response).toBe(audio)
    expect(post).toHaveBeenCalledWith(
      '/pg/audio/speech',
      { model: 'tts-1', input: 'hello' },
      expect.objectContaining({ responseType: 'blob', skipErrorHandler: true })
    )
  })

  test('requests generated video content with the caller cancellation signal', async () => {
    const video = new Blob(['ftyp'], { type: 'video/mp4' })
    const signal = new AbortController().signal
    get.mockResolvedValueOnce({ data: video })

    await expect(fetchVideoContent('task_video_123', signal)).resolves.toBe(
      video
    )

    expect(get).toHaveBeenCalledWith(
      '/v1/videos/task_video_123/content',
      expect.objectContaining({
        responseType: 'blob',
        disableDuplicate: true,
        skipErrorHandler: true,
        signal,
      })
    )
  })
})

describe('Playground attachment API', () => {
  beforeEach(() => {
    get.mockClear()
    post.mockClear()
  })

  test('creates an authenticated upload session', async () => {
    post.mockResolvedValueOnce({
      data: {
        success: true,
        data: {
          upload_id: 'upl_1',
          asset_id: 'ast_1',
          object: 'asset.upload',
          status: 'pending',
          upload_url: 'https://storage.example/put',
          upload_headers: { 'Content-Type': 'image/png' },
          expires_at: 100,
        },
      },
    })

    await expect(
      createPlaygroundAttachmentUploadSession({
        assetType: 'Image',
        contentType: 'image/png',
        sizeBytes: 42,
      })
    ).resolves.toMatchObject({ upload_id: 'upl_1', asset_id: 'ast_1' })
    expect(post).toHaveBeenCalledWith(
      '/api/playground/attachments/uploads',
      {
        asset_type: 'Image',
        content_type: 'image/png',
        size_bytes: 42,
      },
      expect.any(Object)
    )
  })

  test('completes an upload and restores a preview by durable asset id', async () => {
    post.mockResolvedValueOnce({
      data: {
        success: true,
        data: {
          asset_id: 'ast_1',
          asset_type: 'Image',
          content_type: 'image/png',
          size_bytes: 42,
          preview_url: 'https://storage.example/read',
          expires_at: 200,
        },
      },
    })
    get.mockResolvedValueOnce({
      data: {
        success: true,
        data: {
          asset_id: 'ast_1',
          asset_type: 'Image',
          content_type: 'image/png',
          size_bytes: 42,
          preview_url: 'https://storage.example/read-2',
          expires_at: 300,
        },
      },
    })

    await expect(
      completePlaygroundAttachmentUpload('upl_1')
    ).resolves.toMatchObject({
      asset_id: 'ast_1',
    })
    await expect(
      getPlaygroundAttachmentPreview('ast_1')
    ).resolves.toMatchObject({
      preview_url: 'https://storage.example/read-2',
    })
    expect(post.mock.calls[0]?.[0]).toBe(
      '/api/playground/attachments/uploads/upl_1/complete'
    )
    expect(get.mock.calls[0]?.[0]).toBe(
      '/api/playground/attachments/ast_1/preview'
    )
  })
})

describe('Playground record export API', () => {
  beforeEach(() => {
    get.mockClear()
  })

  test('requests the xlsx export with duplicate suppression and error bypass', async () => {
    const blob = new Blob(['xlsx-bytes'], {
      type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    })
    get.mockResolvedValueOnce({
      data: blob,
      headers: {
        'content-disposition':
          'attachment; filename="playground-records-20260831-010203.xlsx"',
      },
    })

    await expect(downloadPlaygroundRecords()).resolves.toEqual({
      blob,
      filename: 'playground-records-20260831-010203.xlsx',
    })
    expect(get).toHaveBeenCalledWith(PLAYGROUND_RECORDS_EXPORT, {
      params: { format: 'xlsx' },
      responseType: 'blob',
      disableDuplicate: true,
      skipErrorHandler: true,
    })
  })

  test.each([
    ['missing', undefined],
    [
      'unsafe',
      'attachment; filename="../../playground-records-20260831-010203.xlsx"',
    ],
  ])(
    'falls back to a timestamped filename when the Content-Disposition header is %s',
    async (_name, contentDisposition) => {
      const blob = new Blob(['xlsx-bytes'], {
        type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      })
      get.mockResolvedValueOnce({
        data: blob,
        headers: contentDisposition
          ? { 'content-disposition': contentDisposition }
          : {},
      })

      setSystemTime(new Date('2026-08-31T01:02:03Z'))
      try {
        await expect(downloadPlaygroundRecords()).resolves.toEqual({
          blob,
          filename: 'playground-records-20260831-010203.xlsx',
        })
      } finally {
        setSystemTime()
      }
    }
  )

  test('parses a blob error response and preserves the HTTP status', async () => {
    get.mockRejectedValueOnce({
      response: {
        status: 403,
        data: new Blob([JSON.stringify({ message: 'Forbidden export' })], {
          type: 'application/json',
        }),
      },
    })

    try {
      await downloadPlaygroundRecords()
      throw new Error('Expected export to fail')
    } catch (error) {
      expect(error).toBeInstanceOf(Error)
      expect((error as Error).message).toBe('Forbidden export')
      expect((error as { status?: number }).status).toBe(403)
    }
  })

  test.each([
    ['csv', 'attachment; filename="report.csv"'],
    ['no extension', 'attachment; filename="playground-records"'],
    [
      'encoded path',
      "attachment; filename*=UTF-8''..%2Fplayground-records.xlsx",
    ],
  ])(
    'falls back to the timestamped filename when the Content-Disposition header is a non-xlsx %s',
    async (_name, contentDisposition) => {
      const blob = new Blob(['xlsx-bytes'], {
        type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      })
      get.mockResolvedValueOnce({
        data: blob,
        headers: { 'content-disposition': contentDisposition },
      })

      setSystemTime(new Date('2026-08-31T01:02:03Z'))
      try {
        await expect(downloadPlaygroundRecords()).resolves.toEqual({
          blob,
          filename: 'playground-records-20260831-010203.xlsx',
        })
      } finally {
        setSystemTime()
      }
    }
  )

  test('rejects successful responses that are not blobs', async () => {
    get.mockResolvedValueOnce({
      data: { success: true },
    })

    await expect(downloadPlaygroundRecords()).rejects.toThrow(
      'Playground records export returned non-Blob response'
    )
  })
})
