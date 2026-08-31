/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import type { ModelPromotion } from '@/features/available-models/lib/model-promotions'

export type ModelSelectorOption = {
  label: string
  value: string
  category?: string
  description?: string
  promotions?: ModelPromotion[]
  price?: number
  releaseDate?: string
  featuredOrder?: number
}

function promotionPriority(
  model: ModelSelectorOption,
  order: readonly ModelPromotion[]
) {
  const priority = order.findIndex((promotion) =>
    model.promotions?.includes(promotion)
  )
  return priority === -1 ? order.length : priority
}

function compareSearchMetadata(a: ModelSelectorOption, b: ModelSelectorOption) {
  const aRelease = a.releaseDate ? Date.parse(a.releaseDate) : Number.NaN
  const bRelease = b.releaseDate ? Date.parse(b.releaseDate) : Number.NaN
  const aHasRelease = Number.isFinite(aRelease)
  const bHasRelease = Number.isFinite(bRelease)
  if (aHasRelease || bHasRelease) {
    if (!aHasRelease) return 1
    if (!bHasRelease) return -1
    if (aRelease !== bRelease) return bRelease - aRelease
  }

  const aPrice = a.price ?? Number.POSITIVE_INFINITY
  const bPrice = b.price ?? Number.POSITIVE_INFINITY
  if (aPrice !== bPrice) return aPrice - bPrice

  const aFeatured = a.featuredOrder ?? Number.POSITIVE_INFINITY
  const bFeatured = b.featuredOrder ?? Number.POSITIVE_INFINITY
  if (aFeatured !== bFeatured) return aFeatured - bFeatured

  return a.label.localeCompare(b.label, undefined, { numeric: true })
}

/** Match the Available Models list: free, discounted, then hot campaigns. */
export function sortModelOptionsForDefault(
  models: readonly ModelSelectorOption[]
): ModelSelectorOption[] {
  const order: ModelPromotion[] = ['free', 'limited', 'hot']
  return [...models].sort(
    (a, b) => promotionPriority(a, order) - promotionPriority(b, order)
  )
}

/** Search ordering: new, free, discounted, hot, then the remaining models. */
export function sortModelOptionsForSearch(
  models: readonly ModelSelectorOption[]
): ModelSelectorOption[] {
  const order: ModelPromotion[] = ['new', 'free', 'limited', 'hot']
  return [...models].sort((a, b) => {
    const rankDiff = promotionPriority(a, order) - promotionPriority(b, order)
    return rankDiff || compareSearchMetadata(a, b)
  })
}
