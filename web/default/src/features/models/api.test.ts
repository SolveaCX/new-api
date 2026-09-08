import { afterEach, describe, expect, mock, spyOn, test } from 'bun:test'
import { api } from '@/lib/api'
import {
  getWebsiteFeaturedModels,
  updateWebsiteFeaturedModels,
  uploadWebsiteFeaturedBackgroundImage,
} from './api'

afterEach(() => {
  mock.restore()
})

describe('website featured models API', () => {
  test('loads featured rows and public candidates from the admin endpoint', async () => {
    const response = {
      success: true,
      data: { featured: [], candidates: [] },
    }
    const get = spyOn(api, 'get').mockResolvedValue({ data: response } as never)

    await expect(getWebsiteFeaturedModels()).resolves.toEqual(response)
    expect(get).toHaveBeenCalledWith('/api/models/website-featured')
  })

  test('replaces the complete featured order', async () => {
    const response = {
      success: true,
      data: { model_names: ['gpt-5.5', 'claude-opus-4.7'] },
    }
    const put = spyOn(api, 'put').mockResolvedValue({ data: response } as never)

    await expect(
      updateWebsiteFeaturedModels(['gpt-5.5', 'claude-opus-4.7'])
    ).resolves.toEqual(response)
    expect(put).toHaveBeenCalledWith('/api/models/website-featured', {
      model_names: ['gpt-5.5', 'claude-opus-4.7'],
    })
  })

  test('saves banner presentation fields for selected models', async () => {
    const response = {
      success: true,
      data: { model_names: ['gpt-5.5'] },
    }
    const put = spyOn(api, 'put').mockResolvedValue({ data: response } as never)

    await updateWebsiteFeaturedModels([
      {
        model_name: 'gpt-5.5',
        sort_order: 0,
        available: true,
        display_name: 'GPT launch',
        description: 'Banner copy',
        tags: 'Coding, Agents',
        background_image_url: 'https://cdn.example/banner.png',
        background_image: '',
        fallback_background_image: '/assets/fallback.png',
      },
    ])

    expect(put).toHaveBeenCalledWith('/api/models/website-featured', {
      items: [
        {
          model_name: 'gpt-5.5',
          display_name: 'GPT launch',
          description: 'Banner copy',
          tags: 'Coding, Agents',
          background_image_url: 'https://cdn.example/banner.png',
          background_image: '',
          fallback_background_image: '/assets/fallback.png',
        },
      ],
    })
  })

  test('uploads a banner image as multipart form data', async () => {
    const response = {
      success: true,
      data: {
        url: '/media/website-featured/hash.png',
        sha256: 'hash',
        content_type: 'image/png',
        size: 7,
      },
    }
    const post = spyOn(api, 'post').mockResolvedValue({
      data: response,
    } as never)
    const file = new File(['payload'], 'banner.png', { type: 'image/png' })

    await expect(uploadWebsiteFeaturedBackgroundImage(file)).resolves.toEqual(
      response
    )
    expect(post).toHaveBeenCalledTimes(1)
    const [url, form, config] = post.mock.calls[0]
    expect(url).toBe('/api/models/website-featured/media')
    expect(form).toBeInstanceOf(FormData)
    const uploaded = (form as FormData).get('file') as File
    expect(uploaded.name).toBe(file.name)
    expect(uploaded.type).toBe(file.type)
    expect(await uploaded.text()).toBe('payload')
    expect(config).toEqual({
      skipBusinessError: true,
      skipErrorHandler: true,
    })
  })
})
