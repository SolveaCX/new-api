/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import type { ReactNode } from 'react'
import { AxiosHeaders, type InternalAxiosRequestConfig } from 'axios'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeAll, describe, expect, test } from 'bun:test'
import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { api } from '@/lib/api'
import {
  listChannelBindings,
  listContracts,
  listEffectiveExclusions,
} from '../api'
import type { SupplyChainManagementProps } from '../contracts'
import { supplyChainQueryKeys } from '../query-keys'
import { ChannelBindingManagement } from './channel-binding-management'
import { ContractManagement } from './contract-management'
import { ExclusionManagement } from './exclusion-management'

const testI18n = createInstance()
const originalAdapter = api.defaults.adapter

afterEach(() => {
  api.defaults.adapter = originalAdapter
})

beforeAll(async () => {
  await testI18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })
})

const search: SupplyChainManagementProps['search'] = {
  tab: 'exclusions',
  month: '2026-07',
  page: 1,
  pageSize: 20,
  filter: '',
}

function renderWithQuery(queryClient: QueryClient, element: ReactNode): string {
  return renderToStaticMarkup(
    <I18nextProvider i18n={testI18n}>
      <QueryClientProvider client={queryClient}>{element}</QueryClientProvider>
    </I18nextProvider>
  )
}

describe('supply-chain management presentation', () => {
  test('renders empty contract and sibling tabs when the Go page contains items null', async () => {
    api.defaults.adapter = async (config: InternalAxiosRequestConfig) => ({
      data: {
        success: true,
        data: { page: 1, page_size: 20, total: 0, items: null },
      },
      status: 200,
      statusText: 'OK',
      headers: new AxiosHeaders(),
      config,
    })
    const contractParams = {
      p: 1,
      page_size: 20,
      supplier_id: undefined,
      status: undefined,
      keyword: undefined,
    }
    const exclusionParams = {
      p: 1,
      page_size: 20,
      user_id: undefined,
      keyword: undefined,
    }
    const bindingParams = {
      p: 1,
      page_size: 20,
      contract_id: undefined,
      keyword: undefined,
      bound_state: undefined,
    }
    const [contracts, exclusions, bindings] = await Promise.all([
      listContracts(contractParams),
      listEffectiveExclusions(exclusionParams),
      listChannelBindings(bindingParams),
    ])
    const cases = [
      {
        key: supplyChainQueryKeys.contracts.list(contractParams),
        page: contracts.data,
        element: (
          <ContractManagement
            search={{ ...search, tab: 'contracts' }}
            onSearchChange={() => undefined}
          />
        ),
      },
      {
        key: supplyChainQueryKeys.exclusions.effective(exclusionParams),
        page: exclusions.data,
        element: (
          <ExclusionManagement
            search={search}
            onSearchChange={() => undefined}
          />
        ),
      },
      {
        key: supplyChainQueryKeys.channelBindings.list(bindingParams),
        page: bindings.data,
        element: (
          <ChannelBindingManagement
            search={{ ...search, tab: 'channel-bindings' }}
            onSearchChange={() => undefined}
          />
        ),
      },
    ]

    for (const item of cases) {
      const client = new QueryClient()
      client.setQueryData(item.key, item.page)
      expect(renderWithQuery(client, item.element)).toContain('No data')
    }
  })

  test('shows missing identities without exposing deleted account data or email', () => {
    const client = new QueryClient()
    const params = {
      p: 1,
      page_size: 20,
      user_id: undefined,
      keyword: undefined,
    }
    client.setQueryData(supplyChainQueryKeys.exclusions.effective(params), {
      page: 1,
      page_size: 20,
      total: 1,
      items: [
        {
          rule_id: 4,
          user_id: 99,
          username: '',
          display_name: '',
          role: null,
          status: null,
          identity_present: false,
          action: 'exclude',
          excluded: true,
          effective_at: 1_789_488_000,
          reason: 'internal traffic',
          created_by: 1,
          created_at: 1_789_488_000,
        },
      ],
    })

    const html = renderWithQuery(
      client,
      <ExclusionManagement search={search} onSearchChange={() => undefined} />
    )

    expect(html).toContain('Identity unavailable')
    expect(html).toContain('User ID: 99')
    expect(html).toContain('internal traffic')
    expect(html).not.toContain('@')
  })

  test('renders server-projected contract and rate for a bound channel', () => {
    const client = new QueryClient()
    const bindingSearch = { ...search, tab: 'channel-bindings' as const }
    const params = {
      p: 1,
      page_size: 20,
      contract_id: undefined,
      keyword: undefined,
      bound_state: undefined,
    }
    client.setQueryData(supplyChainQueryKeys.channelBindings.list(params), {
      page: 1,
      page_size: 20,
      total: 1,
      items: [
        {
          channel_id: 8,
          channel_name: 'Claude upstream',
          channel_status: 1,
          supplier_contract_id: 12,
          contract_name: 'Annual frame',
          contract_no: 'FRAME-2026',
          supplier_id: 3,
          supplier_name: 'Provider A',
          current_rate_version_id: 7,
          current_procurement_multiplier_ppm: 650_000,
        },
      ],
    })

    const html = renderWithQuery(
      client,
      <ChannelBindingManagement
        search={bindingSearch}
        onSearchChange={() => undefined}
      />
    )

    expect(html).toContain('Claude upstream')
    expect(html).toContain('FRAME-2026')
    expect(html).toContain('65%')
    expect(html).toContain('Rebind')
  })
})
