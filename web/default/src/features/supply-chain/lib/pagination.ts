/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import type { SupplyChainAdminPage } from '../types'

interface OffsetPage<T> {
  items: T[]
  limit: number
  offset: number
  has_more: boolean
}

export function getNextAdminPage<T>(
  page: SupplyChainAdminPage<T>
): number | undefined {
  return page.page * page.page_size < page.total ? page.page + 1 : undefined
}

export function mergeAdminPages<T>(
  pages: SupplyChainAdminPage<T>[]
): SupplyChainAdminPage<T> {
  const lastPage = pages.at(-1)
  if (!lastPage) {
    return { page: 1, page_size: 0, total: 0, items: [] }
  }
  return {
    ...lastPage,
    page: 1,
    items: pages.flatMap((page) => page.items),
  }
}

export function getNextOffset<T>(page: OffsetPage<T>): number | undefined {
  return page.has_more ? page.offset + page.limit : undefined
}

export function mergeOffsetPages<TPage extends OffsetPage<TItem>, TItem>(
  pages: TPage[]
): TPage | undefined {
  const lastPage = pages.at(-1)
  if (!lastPage) return undefined
  return {
    ...lastPage,
    offset: 0,
    items: pages.flatMap((page) => page.items),
  }
}
