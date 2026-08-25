/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import type {
  CostCalculationAssumptions,
  CostCalculationDiscounts,
  CostCalculationModel,
  CostCalculationResult,
} from './types'

export function getCashInputs(assumptions: CostCalculationAssumptions) {
  const actualCash = Math.max(assumptions.listPrice - assumptions.coupon, 0)
  const totalQuota = assumptions.listPrice + assumptions.bonus
  const cashFactor = totalQuota > 0 ? actualCash / totalQuota : 0
  return { actualCash, totalQuota, cashFactor }
}

export function calculateModel(
  model: CostCalculationModel,
  assumptions: CostCalculationAssumptions,
  discounts: CostCalculationDiscounts
): CostCalculationResult {
  const { cashFactor } = getCashInputs(assumptions)
  const officialFee =
    (model.officialInput * assumptions.inputTokens) / 1_000_000 +
    (model.officialOutput * assumptions.outputTokens) / 1_000_000 +
    model.officialNonToken * assumptions.nonTokenUsage
  const isSeedance = model.name.toLowerCase().startsWith('seedance')
  const sellFactor = isSeedance
    ? assumptions.seedanceSellFactor
    : assumptions.otherSellFactor
  const flatkeyPrice = officialFee * sellFactor
  const cashIncome = flatkeyPrice * cashFactor
  const discount = getDiscountForModel(model, discounts)
  const upstreamCost = discount === null ? null : officialFee * discount
  const profit = upstreamCost === null ? null : cashIncome - upstreamCost
  const margin =
    profit === null || cashIncome === 0 ? null : profit / cashIncome

  return {
    officialFee,
    sellFactor,
    flatkeyPrice,
    cashIncome,
    upstreamCost,
    profit,
    margin,
    status:
      upstreamCost === null
        ? 'missing-discount'
        : profit !== null && profit >= 0
          ? 'profit'
          : 'loss',
  }
}

export function getDiscountForModel(
  model: CostCalculationModel,
  discounts: CostCalculationDiscounts
) {
  if (model.costDiscount !== null) return model.costDiscount
  const name = model.name.toLowerCase()
  if (name.startsWith('seedance')) return discounts.bytedance
  if (model.vendor === 'OpenAI' || name.startsWith('gpt'))
    return discounts.openai
  if (model.vendor === 'Anthropic' || name.startsWith('claude'))
    return discounts.anthropic
  if (model.vendor === 'Moonshot AI' || name.startsWith('kimi'))
    return discounts.moonshot
  if (['Alibaba', 'Zhipu AI', 'DeepSeek', 'MiniMax'].includes(model.vendor)) {
    return discounts.national
  }
  return discounts.other
}

export function calculateSummary(
  models: CostCalculationModel[],
  assumptions: CostCalculationAssumptions,
  discounts: CostCalculationDiscounts
) {
  const results = models.map((model) =>
    calculateModel(model, assumptions, discounts)
  )
  return {
    priced: models.length,
    profit: results.filter((result) => result.status === 'profit').length,
    loss: results.filter((result) => result.status === 'loss').length,
    missingDiscount: results.filter(
      (result) => result.status === 'missing-discount'
    ).length,
  }
}
