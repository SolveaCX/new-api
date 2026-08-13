/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import { describe, expect, test } from 'bun:test'
import { buildGoogleOneTapLoginUri } from '../lib/google-one-tap'

describe('buildGoogleOneTapLoginUri', () => {
  test('preserves a safe console redirect', () => {
    expect(buildGoogleOneTapLoginUri('/dashboard?tab=usage')).toBe(
      '/api/oauth/google/one-tap?return_to=%2Fdashboard%3Ftab%3Dusage'
    )
  })

  test('rejects external and protocol-relative redirects', () => {
    expect(buildGoogleOneTapLoginUri('https://example.com')).toBe(
      '/api/oauth/google/one-tap?return_to=%2Fdashboard'
    )
    expect(buildGoogleOneTapLoginUri('//example.com')).toBe(
      '/api/oauth/google/one-tap?return_to=%2Fdashboard'
    )
  })
})
