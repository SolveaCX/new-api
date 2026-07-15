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
import { readFileSync } from 'node:fs'
import { runInNewContext } from 'node:vm'

type BootstrapWindow = {
  location: {
    pathname: string
    search: string
    hash: string
  }
  history: {
    state: unknown
    replaceState: (state: unknown, title: string, url: string) => void
  }
  __consumeRegistrationEmailVerificationToken?: () => string | null
  __registrationEmailVerificationRequest?: Promise<unknown>
  fetch: (
    input: string,
    init: {
      body?: string
      credentials?: string
      headers?: unknown
      method?: string
    }
  ) => Promise<{ json: () => Promise<unknown> }>
}

const indexHtml = readFileSync(
  new URL('../../../../index.html', import.meta.url),
  'utf8'
)
const routeTreeSource = readFileSync(
  new URL('../../../routeTree.gen.ts', import.meta.url),
  'utf8'
)

function getBootstrapScript(): string {
  const match = indexHtml.match(
    /<script data-registration-email-verification-bootstrap>([\s\S]*?)<\/script>/
  )
  expect(match).not.toBeNull()
  return match?.[1] || ''
}

describe('registration email verification bootstrap', () => {
  test('runs before every analytics injection point', () => {
    const bootstrapIndex = indexHtml.indexOf(
      '<script data-registration-email-verification-bootstrap>'
    )

    expect(bootstrapIndex).toBeGreaterThan(-1)
    expect(bootstrapIndex).toBeLessThan(indexHtml.indexOf('<!--umami-->'))
    expect(bootstrapIndex).toBeLessThan(
      indexHtml.indexOf('<!--Google Analytics-->')
    )
    expect(bootstrapIndex).toBeLessThan(
      indexHtml.indexOf('<!-- Google Tag Manager -->')
    )
  })

  test('scrubs the credential fragment and starts exchange before third-party scripts', async () => {
    let replacedUrl = ''
    const requests: Array<{ input: string; body?: string; method?: string }> =
      []
    const exchangeResponse = {
      success: true,
      message: '',
      data: { verified: true },
    }
    const window: BootstrapWindow = {
      location: {
        pathname: '/sign-up/verify',
        search: '?lng=en',
        hash: '#token=a%2Fb',
      },
      history: {
        state: { preserved: true },
        replaceState: (_state, _title, url) => {
          replacedUrl = url
        },
      },
      fetch: async (input, init) => {
        requests.push({ input, body: init.body, method: init.method })
        return { json: async () => exchangeResponse }
      },
    }

    runInNewContext(getBootstrapScript(), {
      JSON,
      Promise,
      URLSearchParams,
      window,
    })

    expect(replacedUrl).toBe('/sign-up/verify?lng=en')
    expect(window.__consumeRegistrationEmailVerificationToken).toBeUndefined()
    expect(requests).toEqual([
      {
        input: '/api/registration/email-verification/exchange',
        body: JSON.stringify({ token: 'a/b' }),
        method: 'POST',
      },
    ])
    expect(await window.__registrationEmailVerificationRequest).toEqual(
      exchangeResponse
    )
  })

  test('keeps the verification page independent from the sign-up form route', () => {
    const routeDefinition = routeTreeSource.match(
      /const authSignUpVerifyRoute = authSignUpVerifyRouteImport\.update\(\{[\s\S]*?\} as any\)/
    )?.[0]

    expect(routeDefinition).toContain('getParentRoute: () => authRouteRoute')
    expect(routeDefinition).not.toContain(
      'getParentRoute: () => authSignUpRoute'
    )
  })
})
