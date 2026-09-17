import { describe, expect, test } from 'bun:test'
import type { ModelAccessModel } from '../types'
import {
  getCustomModelTags,
  getModelPromotions,
  modelPromotionPriority,
  sortModelsByPromotion,
} from './model-promotions'

describe('metadata-controlled model tags', () => {
  test('recognizes configured tags independently of the model name', () => {
    expect(
      getModelPromotions('custom', ' Free, Limited discount, HOT, New release ')
    ).toEqual(['free', 'limited', 'hot', 'new'])
    expect(getModelPromotions('custom', 'limited,new')).toEqual([
      'limited',
      'new',
    ])
  })
  test('clearing tags removes every previously hardcoded campaign', () => {
    for (const id of [
      'glm-5.3',
      'deepseek-v4-pro',
      'qwen3.8-max-free',
      'seedance-2.5',
      'gpt-5.6-sol',
      'claude-opus-5',
      'claude-fable-5.1',
    ]) {
      expect(getModelPromotions(id)).toEqual([])
      expect(getModelPromotions(id, '')).toEqual([])
    }
  })
  test('keeps custom labels without duplicating recognized campaigns', () => {
    expect(
      getCustomModelTags(
        'HOT, Featured, New release, Featured, 中文标签, , Free'
      )
    ).toEqual(['Featured', '中文标签'])
    expect(getCustomModelTags('')).toEqual([])
  })
  test('orders tagged models by configured weight and preserves untagged order', () => {
    const models = [
      { id: 'plain', display_weight: 999 },
      { id: 'custom', tags: 'Featured', display_weight: 10 },
      { id: 'free', tags: 'Free', display_weight: 1 },
      { id: 'hot', tags: 'HOT', display_weight: 20 },
      { id: 'plain-two' },
    ] as ModelAccessModel[]
    expect(sortModelsByPromotion(models).map((m) => m.id)).toEqual([
      'hot',
      'custom',
      'free',
      'plain',
      'plain-two',
    ])
    expect(models[0].id).toBe('plain')
    expect(modelPromotionPriority('plain')).toBe(Infinity)
  })
})
