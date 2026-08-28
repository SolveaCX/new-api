import { describe, expect, test } from 'vitest'
import { getModelPromotions } from './model-promotions'

describe('model promotions', () => {
  test('marks GLM-5.3 Flash as a new release only', () => {
    expect(getModelPromotions('glm-5.3')).toEqual([])
    expect(getModelPromotions('glm-5.3-flash')).toEqual(['new'])
  })
})
