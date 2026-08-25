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
export type CostCalculationAssumptions = {
  listPrice: number
  bonus: number
  coupon: number
  seedanceSellFactor: number
  otherSellFactor: number
  inputTokens: number
  outputTokens: number
  nonTokenUsage: number
}

export type CostCalculationDiscounts = {
  openai: number | null
  anthropic: number | null
  moonshot: number | null
  national: number | null
  bytedance: number | null
  other: number | null
}

export type CostCalculationModel = {
  name: string
  vendor: string
  billingUnit: string
  officialInput: number
  officialOutput: number
  officialNonToken: number
  costDiscount: number | null
  note: string
  customFields?: Record<string, string>
}

export type CostCalculationConfig = {
  assumptions: CostCalculationAssumptions
  discounts: CostCalculationDiscounts
  fields: string[]
  models: CostCalculationModel[]
}

export type CostCalculationResult = {
  officialFee: number
  sellFactor: number
  flatkeyPrice: number
  cashIncome: number
  upstreamCost: number | null
  profit: number | null
  margin: number | null
  status: 'profit' | 'loss' | 'missing-discount'
}
