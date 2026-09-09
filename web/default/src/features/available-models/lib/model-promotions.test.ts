import { describe, expect, test } from 'vitest'
import {
  getModelPromotions,
  modelPromotionPriority,
  sortModelsByPromotion,
} from './model-promotions'

describe('model promotions', () => {
  test('recognizes the requested free, limited, new and hot models', () => {
    expect(getModelPromotions('glm-5.3', 'Limited discount,New release')).toEqual(['limited', 'new'])
    expect(getModelPromotions('glm-5.3-flash', 'Limited discount,New release')).toEqual(['limited', 'new'])
    expect(getModelPromotions('deepseek-v4-pro', 'Limited discount')).toEqual(['limited'])
    expect(getModelPromotions('deepseek-v4-flash', 'Free')).toEqual(['free'])
    expect(getModelPromotions('qwen3.8-max')).not.toContain('limited')
    expect(getModelPromotions('qwen3.8-max-free')).not.toContain('free')
    expect(getModelPromotions('kimi-k3')).not.toContain('limited')
    expect(getModelPromotions('kimi-k3')).not.toContain('hot')
    expect(getModelPromotions('seedance-2.5')).toEqual(['hot'])
    expect(getModelPromotions('doubao-seedance-2-5-260628')).toEqual(['hot'])
    expect(getModelPromotions('seedance-2.0')).not.toContain('hot')
    expect(getModelPromotions('gpt-5.6-sol')).toEqual(['hot'])
    expect(getModelPromotions('claude-opus-4-7')).not.toContain('hot')
    expect(getModelPromotions('claude-opus-4-8')).toContain('hot')
    expect(getModelPromotions('claude-opus-5')).toContain('hot')
    expect(getModelPromotions('claude-sonnet-5')).toContain('hot')
    expect(getModelPromotions('claude-fable-5.1')).toEqual(['new'])
  })

  test('sorts promoted models first while preserving stable order', () => {
    const models = [
      { id: 'plain' },
      { id: 'glm-5.3' },
      { id: 'deepseek-v4-pro' },
      { id: 'gpt-5.6-sol' },
      { id: 'deepseek-v4-flash' },
    ] as never[]
    expect(sortModelsByPromotion(models).map((model) => model.id)).toEqual([
      'deepseek-v4-flash',
      'glm-5.3',
      'deepseek-v4-pro',
      'gpt-5.6-sol',
      'plain',
    ])
    expect(modelPromotionPriority('plain')).toBe(Number.POSITIVE_INFINITY)
  })
})
