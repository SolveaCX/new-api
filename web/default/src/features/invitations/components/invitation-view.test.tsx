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
import type { ComponentProps } from 'react'
import { beforeAll, describe, expect, test } from 'bun:test'
import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { InvitationView } from '../index'
import type { InvitationPageData } from '../types'
import { isValidTransferAmount } from './transfer-dialog'

const testI18n = createInstance()

const fixture: InvitationPageData = {
  summary: {
    inviter_reward_quota: 500000,
    invitee_reward_quota: 250000,
    inviter_reward_max_count: 10,
    history_quota: 1500000,
    transferable_quota: 500000,
    granted_count: 3,
    pending_count: 2,
    transfer_enabled: true,
  },
  items: [
    {
      id: 1,
      masked_identity: 'a***@example.com',
      registered_at: 1783612800,
      status: 'granted',
      granted_at: 1783699200,
      reward_quota: 500000,
      reason: '',
    },
    {
      id: 2,
      masked_identity: 'b***@example.com',
      registered_at: 1783526400,
      status: 'pending',
      granted_at: 0,
      reward_quota: 0,
      reason: '',
    },
  ],
  page: 1,
  page_size: 10,
  total: 12,
}

beforeAll(async () => {
  await testI18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })
})

function renderView(
  overrides: Partial<ComponentProps<typeof InvitationView>> = {}
): string {
  return renderToStaticMarkup(
    <I18nextProvider i18n={testI18n}>
      <InvitationView
        data={fixture}
        affiliateLink='https://console.example.com/sign-up?aff=ABCD'
        loading={false}
        recordsLoading={false}
        affiliateLoading={false}
        affiliateError={false}
        error={false}
        transferring={false}
        page={1}
        onPageChange={() => undefined}
        onRetry={() => undefined}
        onTransfer={async () => true}
        {...overrides}
      />
    </I18nextProvider>
  )
}

describe('InvitationView', () => {
  test('explains and renders the populated first-top-up reward flow', () => {
    const html = renderView()

    expect(html).toContain('Total earned')
    expect(html).toContain('Available to transfer')
    expect(html).toContain('Successful referrals')
    expect(html).toContain('Waiting for first top-up')
    expect(html).toContain('Your Referral Link')
    expect(html).toContain('https://console.example.com/sign-up?aff=ABCD')
    expect(html).toContain('Share by email')
    expect(html).toContain('Share on X')
    expect(html).toContain('Share on LinkedIn')
    expect(html).toContain('Share your referral link')
    expect(html).toContain('Your friend signs up')
    expect(html).toContain('first successful top-up')
    expect(html).toContain('Transfer Rewards')
    expect(html).toContain('Recent referrals')
    expect(html).toContain('a***@example.com')
    expect(html).toContain('Reward granted')
    expect(html).toContain('Awaiting top-up')
    expect(html).not.toContain('first API key')
    expect(html).not.toContain('successfully calls the API')
  })

  test('renders the empty invitation state', () => {
    const html = renderView({
      data: { ...fixture, items: [], total: 0 },
    })

    expect(html).toContain('No referrals yet')
    expect(html).toContain('Share your referral link to get started.')
    expect(html).not.toContain('Previous')
  })

  test('lets users return from an empty later page', () => {
    const html = renderView({
      data: { ...fixture, items: [], page: 2 },
      page: 2,
    })

    expect(html).toContain('No referrals yet')
    expect(html).toContain('Previous')
  })

  test('renders a retry action when invitation records fail', () => {
    const html = renderView({ data: null, error: true })

    expect(html).toContain('load your referrals.')
    expect(html).toContain('Retry')
    expect(html).not.toContain('Previous')
  })

  test('lets users retry or return when a later page fails', () => {
    const html = renderView({ data: null, error: true, page: 2 })

    expect(html).toContain('load your referrals.')
    expect(html).toContain('Retry')
    expect(html).toContain('Previous')
  })

  test('keeps a limit-reached invitation granted with an explicit explanation', () => {
    const html = renderView({
      data: {
        ...fixture,
        items: [
          {
            ...fixture.items[0],
            reward_quota: 0,
            reason: 'inviter_limit_reached',
          },
        ],
        total: 1,
      },
    })

    expect(html).toContain('Reward granted')
    expect(html).toContain('You reached the referral reward limit')
  })

  test('renders loading placeholders without empty or error copy', () => {
    const html = renderView({ data: null, loading: true })

    expect(html).toContain('data-slot="skeleton"')
    expect(html).not.toContain('No referrals yet')
    expect(html).not.toContain('load your referrals.')
    expect(html).not.toMatch(/>\$?0</)
    expect(html).not.toContain(
      'There is currently no limit on the number of referral rewards you can earn.'
    )
    expect(html).not.toContain('What are the current referral rewards?')
    expect(html).not.toContain('Is there a referral reward limit?')
    expect(html).not.toContain(
      'Referral reward transfer is disabled until the administrator confirms compliance terms.'
    )
  })

  test('keeps summary content while paginated records load', () => {
    const html = renderView({ recordsLoading: true })

    expect(html).toContain('Total earned')
    expect(html).toContain('What are the current referral rewards?')
    expect(html).toContain('data-slot="skeleton"')
    expect(html).not.toContain('a***@example.com')
    expect(html).not.toContain('No referrals yet')
  })

  test('uses local-only pagination links', () => {
    const html = renderView()

    expect(html).toContain('href="#"')
    expect(html).not.toContain('?page=')
  })

  test('does not fabricate summary facts when invitation loading fails', () => {
    const html = renderView({ data: null, loading: false, error: true })

    expect(html).toContain('data-slot="skeleton"')
    expect(html).not.toMatch(/>\$?0</)
    expect(html).not.toContain(
      'There is currently no limit on the number of referral rewards you can earn.'
    )
    expect(html).not.toContain('What are the current referral rewards?')
    expect(html).not.toContain('Is there a referral reward limit?')
    expect(html).not.toContain(
      'Referral reward transfer is disabled until the administrator confirms compliance terms.'
    )
  })

  test('keeps records usable when only the referral link fails', () => {
    const html = renderView({
      affiliateLink: '',
      affiliateError: true,
    })

    expect(html).toContain('Failed to load: Your Referral Link')
    expect(html).not.toContain('load your referrals.')
    expect(html).toContain('a***@example.com')
    expect(html).not.toContain('Copy referral link')
  })

  test('renders accessible icon anchors for every share target', () => {
    const html = renderView()

    expect(html).toMatch(
      /<a[^>]*aria-label="Share by email"[^>]*>\s*<svg[^>]*>/
    )
    expect(html).toMatch(
      /<a[^>]*aria-label="Share on X"[^>]*target="_blank"[^>]*rel="noreferrer noopener"[^>]*>\s*<svg[^>]*>/
    )
    expect(html).toMatch(
      /<a[^>]*aria-label="Share on LinkedIn"[^>]*target="_blank"[^>]*rel="noreferrer noopener"[^>]*>\s*<svg[^>]*>/
    )
  })

  test('disables reward transfer when compliance is not confirmed', () => {
    const html = renderView({
      data: {
        ...fixture,
        summary: { ...fixture.summary, transfer_enabled: false },
      },
    })

    expect(html).toContain(
      'Referral reward transfer is disabled until the administrator confirms compliance terms.'
    )
    expect(html).toMatch(/<button[^>]*disabled[^>]*>[^<]*Transfer to Balance/)
  })
})

describe('transfer amount validation', () => {
  test('accepts only finite amounts within the configured bounds', () => {
    expect(isValidTransferAmount(500000, 500000, 1000000)).toBe(true)
    expect(isValidTransferAmount(Number.NaN, 500000, 1000000)).toBe(false)
    expect(
      isValidTransferAmount(Number.POSITIVE_INFINITY, 500000, 1000000)
    ).toBe(false)
    expect(isValidTransferAmount(499999, 500000, 1000000)).toBe(false)
    expect(isValidTransferAmount(1000001, 500000, 1000000)).toBe(false)
    expect(isValidTransferAmount(500000.5, 500000, 1000000)).toBe(false)
  })
})
