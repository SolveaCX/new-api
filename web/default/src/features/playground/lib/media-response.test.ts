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
import { describe, expect, test } from 'bun:test'
import {
  extractGeneratedImages,
  parseVideoTaskResponse,
  parseVideoToMusicTaskResponse,
  sanitizeGeneratedMediaUrl,
} from './media-response'

describe('sanitizeGeneratedMediaUrl', () => {
  test('allows same-origin, http(s), and generated image data URLs', () => {
    expect(sanitizeGeneratedMediaUrl('/v1/videos/task_123/content')).toBe(
      '/v1/videos/task_123/content'
    )
    expect(sanitizeGeneratedMediaUrl('https://cdn.example/video.mp4')).toBe(
      'https://cdn.example/video.mp4'
    )
    expect(sanitizeGeneratedMediaUrl('data:image/png;base64,QUJDRA==')).toBe(
      'data:image/png;base64,QUJDRA=='
    )
  })

  test('rejects executable, non-image, and protocol-relative URLs', () => {
    expect(sanitizeGeneratedMediaUrl('javascript:alert(1)')).toBeUndefined()
    expect(sanitizeGeneratedMediaUrl('data:text/html,<script>x</script>')).toBe(
      undefined
    )
    expect(sanitizeGeneratedMediaUrl('//tracking.example/image.png')).toBe(
      undefined
    )
    expect(sanitizeGeneratedMediaUrl('/\\tracking.example/image.png')).toBe(
      undefined
    )
    expect(
      sanitizeGeneratedMediaUrl('https://\\tracking.example/image.png')
    ).toBeUndefined()
    expect(
      sanitizeGeneratedMediaUrl('https://user:password@cdn.example/image.png')
    ).toBeUndefined()
    expect(sanitizeGeneratedMediaUrl('https://')).toBeUndefined()
  })

  test('accepts safe blob URLs created for generated audio', () => {
    const url =
      'blob:https://playground.example/550e8400-e29b-41d4-a716-446655440000'
    expect(sanitizeGeneratedMediaUrl(url)).toBe(url)
    expect(
      sanitizeGeneratedMediaUrl('blob:javascript:alert(1)')
    ).toBeUndefined()
    expect(
      sanitizeGeneratedMediaUrl('blob:https://user:pass@playground.example/id')
    ).toBeUndefined()
  })
})

describe('extractGeneratedImages', () => {
  test('extracts URL and base64 image responses without dropping order', () => {
    expect(
      extractGeneratedImages({
        data: [
          { url: 'https://cdn.example/one.png' },
          { b64_json: 'aGVsbG8=', output_format: 'webp' },
        ],
      })
    ).toEqual([
      {
        type: 'image',
        url: 'https://cdn.example/one.png',
      },
      {
        type: 'image',
        url: 'data:image/webp;base64,aGVsbG8=',
      },
    ])
  })

  test('ignores malformed image entries', () => {
    expect(extractGeneratedImages({ data: [{}, null, 'invalid'] })).toEqual([])
  })

  test('drops image URLs that are not safe media sources', () => {
    expect(
      extractGeneratedImages({
        data: [
          { url: 'javascript:alert(1)' },
          { url: 'https://cdn.example/safe.png' },
        ],
      })
    ).toEqual([{ type: 'image', url: 'https://cdn.example/safe.png' }])
  })
})

describe('parseVideoTaskResponse', () => {
  test('parses the OpenAI-style submission response', () => {
    expect(
      parseVideoTaskResponse({
        id: 'task_123',
        status: 'queued',
        progress: 0,
      })
    ).toEqual({
      taskId: 'task_123',
      status: 'queued',
      progress: 0,
    })
  })

  test('parses the wrapped Playground fetch response and result URL', () => {
    expect(
      parseVideoTaskResponse({
        code: 'success',
        data: {
          task_id: 'task_123',
          status: 'SUCCESS',
          progress: '100%',
          result_url: '/v1/videos/task_123/content',
        },
      })
    ).toEqual({
      taskId: 'task_123',
      status: 'completed',
      progress: 100,
      url: '/v1/videos/task_123/content',
    })
  })

  test('normalizes failed task details', () => {
    expect(
      parseVideoTaskResponse({
        data: {
          task_id: 'task_456',
          status: 'FAILURE',
          fail_reason: 'upstream rejected prompt',
        },
      })
    ).toEqual({
      taskId: 'task_456',
      status: 'failed',
      error: 'upstream rejected prompt',
    })
  })

  test('omits unsafe completed-task result URLs', () => {
    expect(
      parseVideoTaskResponse({
        data: {
          task_id: 'task_789',
          status: 'completed',
          result_url: 'javascript:alert(1)',
        },
      })
    ).toEqual({
      taskId: 'task_789',
      status: 'completed',
    })
  })
})

describe('parseVideoToMusicTaskResponse', () => {
  test('parses a processing Sonilo task', () => {
    expect(
      parseVideoToMusicTaskResponse({
        task_id: 'task_1',
        status: 'processing',
      })
    ).toEqual({
      taskId: 'task_1',
      status: 'in_progress',
    })
  })

  test('parses succeeded audio from a direct or wrapped response', () => {
    expect(
      parseVideoToMusicTaskResponse({
        data: {
          task_id: 'task_1',
          status: 'succeeded',
          audio: [
            {
              url: '/v1/video-to-music/task_1/content?variant=0',
              content_type: 'audio/mpeg',
            },
          ],
        },
      })
    ).toEqual({
      taskId: 'task_1',
      status: 'completed',
      audio: [
        {
          type: 'audio',
          url: '/v1/video-to-music/task_1/content?variant=0',
          mimeType: 'audio/mpeg',
        },
      ],
    })
  })

  test('normalizes failed task details', () => {
    expect(
      parseVideoToMusicTaskResponse({
        task_id: 'task_1',
        status: 'failed',
        error: { message: 'task failed' },
      })
    ).toEqual({
      taskId: 'task_1',
      status: 'failed',
      error: 'task failed',
    })
  })

  test('redacts provider URLs from failed task details', () => {
    const parsed = parseVideoToMusicTaskResponse({
      task_id: 'task_1',
      status: 'failed',
      error: {
        message: 'https://api.sonilo.com/internal?token=secret failed',
      },
    })
    expect(parsed?.error).not.toContain('api.sonilo.com')
    expect(parsed?.error).not.toContain('token=secret')
  })

  test('redacts bare provider hosts and their paths from failed task details', () => {
    const parsed = parseVideoToMusicTaskResponse({
      task_id: 'task_1',
      status: 'failed',
      error: { message: 'api.sonilo.com/internal?token=secret failed' },
    })
    expect(parsed?.error).not.toContain('sonilo.com')
    expect(parsed?.error).not.toContain('token=secret')
  })

  test('does not expose a successful state when every audio URL is unsafe', () => {
    const parsed = parseVideoToMusicTaskResponse({
      task_id: 'task_1',
      status: 'succeeded',
      audio: [{ url: 'javascript:alert(1)', content_type: 'audio/mpeg' }],
    })
    expect(parsed?.status).toBe('completed')
    expect(parsed?.audio).toBeUndefined()
  })

  test('does not expose a Sonilo provider URL to the Playground message', () => {
    const parsed = parseVideoToMusicTaskResponse({
      task_id: 'task_1',
      status: 'succeeded',
      audio: [{ url: 'https://api.sonilo.com/v1/audio/task_1.mp3' }],
    })
    expect(parsed?.status).toBe('completed')
    expect(parsed?.audio).toBeUndefined()
  })

  test('requires HTTPS for external Sonilo audio URLs', () => {
    const parsed = parseVideoToMusicTaskResponse({
      task_id: 'task_1',
      status: 'succeeded',
      audio: [{ url: 'http://cdn.example/audio.mp3' }],
    })
    expect(parsed?.audio).toBeUndefined()
  })

  test('keeps completed tasks without audio out of the success media path', () => {
    const parsed = parseVideoToMusicTaskResponse({
      task_id: 'task_1',
      status: 'completed',
      audio: [],
    })
    expect(parsed?.status).toBe('completed')
    expect(parsed?.audio).toEqual([])
  })
})
