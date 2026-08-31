import { describe, expect, test } from 'vitest'
import {
  getModelPromotions,
  modelPromotionPriority,
  sortModelsByPromotion,
} from './model-promotions'

describe('model promotions', () => {
  test('recognizes the requested free, limited, new and hot models', () => {
    expect(getModelPromotions('glm-5.3')).toEqual(['new'])
    expect(getModelPromotions('glm-5.3-flash')).toEqual(['limited', 'new'])
    expect(getModelPromotions('glm-5.3-0813')).toEqual([])
    expect(getModelPromotions('deepseek-v4-pro')).toEqual(['limited'])
    expect(getModelPromotions('deepseek-v4-flash')).toEqual(['free'])
    expect(getModelPromotions('qwen3.8-max')).not.toContain('limited')
    expect(getModelPromotions('qwen/qwen3.8-max-free')).toEqual([])
    expect(getModelPromotions('kimi-k3')).not.toContain('limited')
    expect(getModelPromotions('kimi-k3')).toEqual(['hot'])
    expect(getModelPromotions('ling-3.0-flash-fin')).toEqual([])
    expect(getModelPromotions('seedance-2.5')).toEqual(['hot'])
    expect(getModelPromotions('doubao-seedance-2-5-260628')).toEqual(['hot'])
    expect(getModelPromotions('seedance-2.0')).not.toContain('hot')
    expect(getModelPromotions('gpt-5.6-sol')).toEqual(['hot'])
    expect(getModelPromotions('claude-opus-4-7')).not.toContain('hot')
    expect(getModelPromotions('claude-opus-4-8')).toContain('hot')
    expect(getModelPromotions('claude-opus-5')).toContain('hot')
    expect(getModelPromotions('claude-sonnet-5')).toContain('hot')
  })

  test('sorts promoted models first while preserving stable order', () => {
    const models = [
      { id: 'plain' },
      { id: 'deepseek-v4-pro' },
      { id: 'gpt-5.6-sol' },
      { id: 'deepseek-v4-flash' },
    ] as never[]
    expect(sortModelsByPromotion(models).map((model) => model.id)).toEqual([
      'deepseek-v4-flash',
      'deepseek-v4-pro',
      'gpt-5.6-sol',
      'plain',
    ])
    expect(modelPromotionPriority('plain')).toBe(Number.POSITIVE_INFINITY)
  })
})
