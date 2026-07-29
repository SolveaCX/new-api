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
/**
 * Zod schemas for common logs
 * This file should only contain Zod schemas and types inferred from them
 */
import { z } from 'zod'

export const supplierAccountingPricingEvidenceSchema = z.object({
  mode: z.enum(['ratio', 'fixed', 'tiered']),
  model_ratio_ppm: z.number().optional(),
  group_multiplier_ppm: z.number().optional(),
  source: z.string().optional(),
  key: z.string().optional(),
  expression_version: z.number().optional(),
  expression_fingerprint: z.string().optional(),
  dimensions: z.array(z.enum(['audio', 'tool', 'image'])).optional(),
})

export const supplierAccountingProjectionSchema = z.object({
  binding_version_id: z.number(),
  supplier_id: z.number(),
  contract_id: z.number(),
  rate_version_id: z.number(),
  procurement_multiplier_ppm: z.number(),
  sales_multiplier_ppm: z.number().optional(),
  official_list_micro_usd: z.string().optional(),
  sales_micro_usd: z.string().optional(),
  procurement_cost_micro_usd: z.string().optional(),
  gross_profit_micro_usd: z.string().optional(),
  statistics_scope: z.enum(['business', 'internal']),
  exclusion_decision: z.enum(['included', 'excluded']),
  exclusion_rule_id: z.number().optional(),
  financially_committed_at: z.number(),
  pricing_evidence: supplierAccountingPricingEvidenceSchema.optional(),
})

// Usage log schema
export const usageLogSchema = z.object({
  id: z.number(),
  user_id: z.number(),
  created_at: z.number(),
  type: z.number(),
  content: z.string(),
  username: z.string().default(''),
  token_name: z.string().default(''),
  model_name: z.string().default(''),
  quota: z.number().default(0),
  prompt_tokens: z.number().default(0),
  completion_tokens: z.number().default(0),
  use_time: z.number().default(0),
  is_stream: z.boolean().default(false),
  channel: z.number().default(0),
  channel_name: z.string().nullish().default(''),
  token_id: z.number().default(0),
  group: z.string().default(''),
  ip: z.string().default(''),
  other: z.string().default(''),
  request_id: z.string().default(''),
  upstream_request_id: z.string().default(''),
  supplier_accounting: supplierAccountingProjectionSchema.optional(),
})

export type UsageLog = z.infer<typeof usageLogSchema>
export type SupplierAccountingProjection = z.infer<
  typeof supplierAccountingProjectionSchema
>
