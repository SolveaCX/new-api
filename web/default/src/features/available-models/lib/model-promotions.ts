import type { TFunction } from 'i18next'
import type { ModelAccessModel } from '../types'

export type ModelPromotion = 'free' | 'limited' | 'hot' | 'new'

const PROMOTION_PRIORITY: Record<ModelPromotion, number> = {
  free: 0,
  limited: 1,
  hot: 2,
  new: 3,
}

export function getModelPromotions(_modelId: string, tags = ''): ModelPromotion[] {
  const normalized = tags.split(',').map((tag) => tag.trim().toLowerCase())
  const promotions: ModelPromotion[] = []
  if (normalized.includes('free')) promotions.push('free')
  if (normalized.some((tag) => tag === 'limited' || tag === 'limited discount')) promotions.push('limited')
  if (normalized.includes('hot')) promotions.push('hot')
  if (normalized.some((tag) => tag === 'new' || tag === 'new release')) promotions.push('new')
  return promotions
}

export function getModelPromotionLabel(
  promotion: ModelPromotion,
  t: TFunction
): string {
  if (promotion === 'free') return t('Free')
  if (promotion === 'limited') return t('Limited discount')
  if (promotion === 'hot') return t('HOT')
  return t('New release')
}

export function modelPromotionPriority(modelId: string, tags = ''): number {
  if (!tags.trim()) return Number.POSITIVE_INFINITY
  const promotions = getModelPromotions(modelId, tags)
  return promotions.length === 0
    ? Number.POSITIVE_INFINITY
    : Math.min(...promotions.map((promotion) => PROMOTION_PRIORITY[promotion]))
}

export function sortModelsByPromotion(
  models: readonly ModelAccessModel[]
): ModelAccessModel[] {
  return models
    .map((model, index) => ({
      model,
      index,
      priority: modelPromotionPriority(model.id, model.tags ?? ''),
    }))
    .sort((a, b) => {
      const aTagged = Boolean(a.model.tags?.trim())
      const bTagged = Boolean(b.model.tags?.trim())
      return (
        Number(bTagged) - Number(aTagged) ||
        (bTagged ? b.model.display_weight ?? 0 : 0) -
          (aTagged ? a.model.display_weight ?? 0 : 0) ||
        a.priority - b.priority ||
        a.index - b.index
      )
    })
    .map(({ model }) => model)
}
