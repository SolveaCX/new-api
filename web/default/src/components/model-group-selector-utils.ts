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

export function sortModelOptionsForSearch(
  models: readonly ModelSelectorOption[]
): ModelSelectorOption[] {
  return [...models].sort((a, b) => {
    const aRelease = a.releaseDate ? Date.parse(a.releaseDate) : Number.NaN
    const bRelease = b.releaseDate ? Date.parse(b.releaseDate) : Number.NaN
    const aHasRelease = Number.isFinite(aRelease)
    const bHasRelease = Number.isFinite(bRelease)
    if (aHasRelease || bHasRelease) {
      if (!aHasRelease) return 1
      if (!bHasRelease) return -1
      if (aRelease !== bRelease) return bRelease - aRelease
    }

    const aIsNew = Number(a.promotions?.includes('new') ?? false)
    const bIsNew = Number(b.promotions?.includes('new') ?? false)
    if (aIsNew !== bIsNew) return bIsNew - aIsNew

    const aPrice = a.price ?? Number.POSITIVE_INFINITY
    const bPrice = b.price ?? Number.POSITIVE_INFINITY
    if (aPrice !== bPrice) return aPrice - bPrice

    const aFeatured = a.featuredOrder ?? Number.POSITIVE_INFINITY
    const bFeatured = b.featuredOrder ?? Number.POSITIVE_INFINITY
    if (aFeatured !== bFeatured) return aFeatured - bFeatured

    return a.label.localeCompare(b.label, undefined, { numeric: true })
  })
}
