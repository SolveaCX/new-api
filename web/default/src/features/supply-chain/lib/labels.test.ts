/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import en from '@/i18n/locales/en.json'
import es from '@/i18n/locales/es.json'
import fr from '@/i18n/locales/fr.json'
import ja from '@/i18n/locales/ja.json'
import pt from '@/i18n/locales/pt.json'
import ru from '@/i18n/locales/ru.json'
import vi from '@/i18n/locales/vi.json'
import zh from '@/i18n/locales/zh.json'
import { describe, expect, test } from 'bun:test'
import {
  INVENTORY_ADJUSTMENT_LABEL_KEYS,
  STATISTICS_ACTION_LABEL_KEYS,
} from './labels'

const locales = { en, es, fr, ja, pt, ru, vi, zh }
const runtimeLabelKeys = [
  ...Object.values(STATISTICS_ACTION_LABEL_KEYS),
  ...Object.values(INVENTORY_ADJUSTMENT_LABEL_KEYS),
  'Oversold',
]

describe('supply-chain runtime labels', () => {
  test('exist in every supported locale', () => {
    for (const [locale, resources] of Object.entries(locales)) {
      for (const key of runtimeLabelKeys) {
        expect(resources.translation[key], `${locale}:${key}`).toBeTruthy()
      }
    }
  })

  test('translates Oversold instead of copying the English placeholder', () => {
    for (const [locale, resources] of Object.entries(locales)) {
      if (locale === 'en') continue
      expect(resources.translation.Oversold, locale).not.toBe('Oversold')
    }
  })
})
