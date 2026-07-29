/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { describe, expect, test } from 'bun:test'

describe('supply-chain report completeness presentation', () => {
  test('warns only for incomplete days and explains published zeroes', async () => {
    const source = await Bun.file(
      new URL('./index.tsx', import.meta.url)
    ).text()

    expect(source).toContain('trend.data?.has_incomplete_days')
    expect(source).toContain('trend.data.incomplete_day_count')
    expect(source).toContain('trend.data.latest_completed_date')
    expect(source).toContain(
      'Completed days with zero usage remain published as zero.'
    )
  })
})
