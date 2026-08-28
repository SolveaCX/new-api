import { describe, expect, test } from 'vitest'
import { getModelPromotions } from './model-promotions'

describe('model promotions', () => {
  test('marks only the requested models with free and limited discount', () => {
    expect(getModelPromotions('glm-5.3')).toEqual([])
    expect(getModelPromotions('glm-5.3-flash')).toEqual(['limited', 'new'])
    expect(getModelPromotions('deepseek-v4-pro')).toEqual(['limited'])
    expect(getModelPromotions('deepseek-v4-flash')).toEqual(['free'])
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
  })
})
