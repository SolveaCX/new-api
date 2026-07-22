import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeAll, describe, expect, test } from 'bun:test'
import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { recallCampaignKeys } from '../api'
import { CampaignSpecifiedUsersSelector } from './campaign-specified-users-selector'

const testI18n = createInstance()

beforeAll(async () => {
  await testI18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })
})

function renderSelector(options?: {
  userIDs?: number[]
  emails?: string[]
  immutable?: boolean
  resolvedUsers?: unknown[]
  searchUsers?: unknown[]
  search?: string
}): { html: string; client: QueryClient } {
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        enabled: false,
        retry: false,
        retryOnMount: false,
        refetchOnMount: false,
      },
    },
  })
  const ids = options?.userIDs ?? []
  const search = options?.search ?? ''
  if (ids.length > 0) {
    client.setQueryData(recallCampaignKeys.audienceUsers({ ids }), {
      success: true,
      data: options?.resolvedUsers ?? [],
    })
  }
  if (search) {
    client.setQueryData(recallCampaignKeys.audienceUsers({ keyword: search }), {
      success: true,
      data: options?.searchUsers ?? [],
    })
  }

  const html = renderToStaticMarkup(
    <QueryClientProvider client={client}>
      <I18nextProvider i18n={testI18n}>
        <CampaignSpecifiedUsersSelector
          userIDs={ids}
          emails={options?.emails ?? []}
          onUserIDsChange={() => undefined}
          onEmailsChange={() => undefined}
          immutable={options?.immutable ?? false}
        />
      </I18nextProvider>
    </QueryClientProvider>
  )
  return { html, client }
}

describe('CampaignSpecifiedUsersSelector', () => {
  test('uses independent selected-user and keyword query keys without empty broad calls', () => {
    expect(recallCampaignKeys.audienceUsers({ ids: [3, 9] })).toEqual([
      'recall-campaigns',
      'audience-options',
      'users',
      { ids: [3, 9] },
    ])
    expect(recallCampaignKeys.audienceUsers({ keyword: 'ada' })).toEqual([
      'recall-campaigns',
      'audience-options',
      'users',
      { keyword: 'ada' },
    ])

    const { client } = renderSelector()

    expect(client.getQueryCache().findAll()).toHaveLength(0)
  })

  test('renders selected users first, keeps cached search options stable, and preserves unavailable IDs', async () => {
    const { html, client } = renderSelector({
      userIDs: [42, 99],
      resolvedUsers: [
        {
          id: 42,
          username: 'ada',
          display_name: 'Ada',
          email: 'ada@example.com',
          status: 1,
        },
      ],
      search: 'grace',
      searchUsers: [
        {
          id: 7,
          username: 'grace',
          display_name: '',
          email: 'grace@example.com',
          status: 1,
        },
        {
          id: 42,
          username: 'ada',
          display_name: 'Ada',
          email: 'ada@example.com',
          status: 1,
        },
      ],
    })

    expect(html).toContain('Ada')
    expect(html).toContain('ada@example.com')
    expect(html).toContain('#42')
    expect(html).toContain('Unavailable')
    expect(html).toContain('#99')
    expect(
      client.getQueryData(
        recallCampaignKeys.audienceUsers({ keyword: 'grace' })
      )
    ).toEqual({
      success: true,
      data: [
        {
          id: 7,
          username: 'grace',
          display_name: '',
          email: 'grace@example.com',
          status: 1,
        },
        {
          id: 42,
          username: 'ada',
          display_name: 'Ada',
          email: 'ada@example.com',
          status: 1,
        },
      ],
    })

    const source = await Bun.file(
      new URL('./campaign-specified-users-selector.tsx', import.meta.url)
    ).text()
    expect(source).toContain('mergeRecallAudienceUserOptions')
    expect(source).toContain('cachedSearchUsers')
  })

  test('shows manual email parsing feedback, invalid tokens, and combined normalized count', () => {
    const { html } = renderSelector({
      userIDs: [42],
      emails: ['Ada@Example.com', 'bad-token', 'ada@example.com'],
      resolvedUsers: [
        {
          id: 42,
          username: 'ada',
          display_name: 'Ada',
          email: 'ada@example.com',
          status: 1,
        },
      ],
    })

    expect(html).toContain('Manual emails')
    expect(html).toContain('bad-token')
    expect(html).toContain('Invalid email entries')
    expect(html).toContain('2 / 500')
  })

  test('disables picker and textarea for immutable campaigns', () => {
    const { html } = renderSelector({ immutable: true, emails: ['a@b.co'] })

    expect(html).toMatch(
      /<input(?=[^>]*id="recall-specified-users")(?=[^>]*disabled="")[^>]*>/
    )
    expect(html).toMatch(
      /<textarea(?=[^>]*id="recall-specified-emails")(?=[^>]*disabled="")[^>]*>/
    )
  })
})
