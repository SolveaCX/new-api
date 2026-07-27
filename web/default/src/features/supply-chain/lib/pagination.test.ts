/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { describe, expect, test } from 'bun:test'
import {
  getNextAdminPage,
  getNextOffset,
  mergeAdminPages,
  mergeOffsetPages,
} from './pagination'

describe('progressive supply-chain pagination', () => {
  test('accumulates admin pages until the backend total is reached', () => {
    const first = { page: 1, page_size: 2, total: 3, items: [1, 2] }
    const second = { page: 2, page_size: 2, total: 3, items: [3] }

    expect(getNextAdminPage(first)).toBe(2)
    expect(getNextAdminPage(second)).toBeUndefined()
    expect(mergeAdminPages([first, second]).items).toEqual([1, 2, 3])
  })

  test('uses each report response has_more and offset independently', () => {
    const first = { items: ['a'], limit: 1, offset: 0, has_more: true }
    const second = { items: ['b'], limit: 1, offset: 1, has_more: false }

    expect(getNextOffset(first)).toBe(1)
    expect(getNextOffset(second)).toBeUndefined()
    expect(mergeOffsetPages([first, second])).toEqual({
      items: ['a', 'b'],
      limit: 1,
      offset: 0,
      has_more: false,
    })
  })
})
