import type { TFunction } from 'i18next'
import type { ModelAccessModel } from '../types'

export type ModelPromotion = 'free' | 'limited' | 'hot' | 'new'

const PROMOTION_PRIORITY: Record<ModelPromotion, number> = {
  free: 0,
  limited: 1,
  hot: 2,
  new: Number.POSITIVE_INFINITY,
}

export function getModelPromotions(modelId: string): ModelPromotion[] {
  const name = modelId.toLowerCase()
  const promotions: ModelPromotion[] = []

  if (
    name.includes('free') ||
    /(^|[/])deepseek[-_.]?v4[-_.]?flash$/.test(name) ||
    /(^|[/_.-])ling[-_.]?3\.0[-_.]?flash[-_.]?fin/.test(name)
  ) {
    promotions.push('free')
  }
  if (
    /(^|[/])glm[-_.]?5[-_.]?3[-_.]?flash$/.test(name) ||
    /(^|[/])deepseek[-_.]?v4[-_.]?pro$/.test(name)
  ) {
    promotions.push('limited')
  }
  if (
    /(^|[/])seedance[-_.]?2[-_.]?5(?:[-_.]|$)/.test(name) ||
    /(^|[/])kimi[-_.]?k3(?:[-_.]|$)/.test(name) ||
    /(^|[/])gpt[-_.]?5[-_.]?6[-_.]?sol(?:[-_.]|$)/.test(name) ||
    /(^|[/])claude[-_.]?(?:opus[-_.]?4[-_.]?8|opus[-_.]?4[-_.]?7|sonnet[-_.]?4[-_.]?6|haiku[-_.]?4[-_.]?5(?:[-_.]?20251001)?)(?:[-_.]|$)/.test(name)
  ) {
    promotions.push('hot')
  }
  if (/(^|[/])glm[-_.]?5[-_.]?3[-_.]?flash$/.test(name)) {
    promotions.push('new')
  }
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

export function modelPromotionPriority(modelId: string): number {
  const promotions = getModelPromotions(modelId)
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
      priority: modelPromotionPriority(model.id),
    }))
    .sort((a, b) => a.priority - b.priority || a.index - b.index)
    .map(({ model }) => model)
}
