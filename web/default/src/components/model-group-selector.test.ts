import { describe, expect, test } from 'bun:test'
import {
  sortModelOptionsForDefault,
  sortModelOptionsForSearch,
} from './model-group-selector-utils'

test('default ordering matches the model list campaign order', () => {
  const models = [
    { label: 'plain', value: 'plain' },
    { label: 'hot', value: 'hot', promotions: ['hot' as const] },
    { label: 'discount', value: 'discount', promotions: ['limited' as const] },
    { label: 'free', value: 'free', promotions: ['free' as const] },
  ]

  expect(
    sortModelOptionsForDefault(models).map((model) => model.value)
  ).toEqual(['free', 'discount', 'hot', 'plain'])
})

describe('sortModelOptionsForSearch', () => {
  test('sorts newest releases first, then cheapest prices', () => {
    const models = [
      {
        label: 'expensive-new',
        value: 'expensive-new',
        releaseDate: '2026-08-20',
        price: 1,
      },
      {
        label: 'cheap-old',
        value: 'cheap-old',
        releaseDate: '2026-01-01',
        price: 0.01,
      },
      {
        label: 'cheap-new',
        value: 'cheap-new',
        releaseDate: '2026-08-20',
        price: 0.01,
      },
    ]

    expect(
      sortModelOptionsForSearch(models).map((model) => model.value)
    ).toEqual(['cheap-new', 'expensive-new', 'cheap-old'])
  })

  test('puts new promotions first when release metadata is missing', () => {
    const models = [
      { label: 'zeta', value: 'zeta', price: 0.1 },
      { label: 'alpha', value: 'alpha', price: 0.1 },
      { label: 'new', value: 'new', promotions: ['new' as const], price: 10 },
    ]

    expect(
      sortModelOptionsForSearch(models).map((model) => model.value)
    ).toEqual(['new', 'alpha', 'zeta'])
  })
})
