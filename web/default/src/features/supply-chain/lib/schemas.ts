/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { z } from 'zod'

const requiredText = z.string().trim().min(1, 'This field is required')
const optionalText = z.string().trim()
const nonNegativeInteger = z
  .number()
  .int()
  .min(0, 'Enter zero or a positive integer')
const positiveInteger = z.number().int().positive('Enter a positive integer')

export const supplierFormSchema = z.object({
  name: requiredText,
  remark: optionalText,
})

export const contractFormSchema = z.object({
  supplier_id: positiveInteger,
  name: requiredText,
  contract_no: requiredText,
  remark: optionalText,
  rpm_limit: nonNegativeInteger,
  tpm_limit: nonNegativeInteger,
  max_concurrency: nonNegativeInteger,
})

export const rateVersionFormSchema = z.object({
  procurement_multiplier_ppm: z
    .number()
    .int()
    .min(0, 'The multiplier cannot be negative')
    .max(1_000_000, 'The multiplier cannot exceed 100%'),
  reason: optionalText,
})

export function usdInputToMicroUsd(value: string): number | null {
  const match = /^([+-]?)(\d+)(?:\.(\d{1,6}))?$/.exec(value.trim())
  if (!match) return null
  const sign = match[1] === '-' ? -1n : 1n
  const whole = BigInt(match[2])
  const fraction = BigInt((match[3] ?? '').padEnd(6, '0'))
  const microUsd = sign * (whole * 1_000_000n + fraction)
  const result = Number(microUsd)
  return Number.isSafeInteger(result) && result !== 0 ? result : null
}

export const inventoryAdjustmentFormSchema = z.object({
  delta_usd: z.string().refine((value) => usdInputToMicroUsd(value) !== null, {
    message: 'Enter a non-zero USD amount with up to 6 decimal places',
  }),
  type: z.enum(['initial', 'replenishment', 'correction', 'reversal']),
  reason: optionalText,
})

export const exclusionFormSchema = z.object({
  user_id: positiveInteger,
  action: z.enum(['exclude', 'include']),
  reason: optionalText,
})

export const channelBindingFormSchema = z.object({
  contract_id: positiveInteger,
  skip_internal_accounting: z.boolean(),
})

const jsonIntegerArray = z.string().refine((value) => {
  try {
    const parsed: unknown = JSON.parse(value)
    return (
      Array.isArray(parsed) &&
      parsed.every((item) => Number.isInteger(item) && item > 0)
    )
  } catch {
    return false
  }
}, 'Enter a JSON array of positive integers')

const historicalMappings = z.string().refine((value) => {
  try {
    const parsed: unknown = JSON.parse(value)
    return (
      Array.isArray(parsed) &&
      parsed.every((item) => {
        if (!item || typeof item !== 'object') return false
        const mapping = item as Record<string, unknown>
        return (
          Number.isInteger(mapping.channel_id) &&
          Number(mapping.channel_id) > 0 &&
          Number.isInteger(mapping.supplier_id) &&
          Number(mapping.supplier_id) > 0 &&
          Number.isInteger(mapping.contract_id) &&
          Number(mapping.contract_id) > 0 &&
          Number.isInteger(mapping.rate_version_id) &&
          Number(mapping.rate_version_id) > 0 &&
          Number.isInteger(mapping.procurement_multiplier_ppm) &&
          Number(mapping.procurement_multiplier_ppm) >= 0 &&
          Number(mapping.procurement_multiplier_ppm) <= 1_000_000
        )
      })
    )
  } catch {
    return false
  }
}, 'Enter a valid channel mapping JSON array')

export const historicalImportFormSchema = z
  .object({
    start_date: z.string().regex(/^\d{4}-\d{2}-\d{2}$/, 'Enter a valid date'),
    end_date: z.string().regex(/^\d{4}-\d{2}-\d{2}$/, 'Enter a valid date'),
    quota_per_unit: z
      .string()
      .trim()
      .regex(/^\d+(?:\.\d+)?$/, 'Enter a positive quota per unit')
      .refine((value) => Number(value) > 0, 'Enter a positive quota per unit'),
    excluded_user_ids_json: jsonIntegerArray,
    channel_mappings_json: historicalMappings,
    reason: requiredText,
  })
  .refine((value) => value.start_date < value.end_date, {
    path: ['end_date'],
    message: 'End date must be after start date',
  })

export type SupplierFormValues = z.infer<typeof supplierFormSchema>
export type ContractFormValues = z.infer<typeof contractFormSchema>
export type RateVersionFormValues = z.infer<typeof rateVersionFormSchema>
export type InventoryAdjustmentFormValues = z.infer<
  typeof inventoryAdjustmentFormSchema
>
export type ExclusionFormValues = z.infer<typeof exclusionFormSchema>
export type ChannelBindingFormValues = z.infer<typeof channelBindingFormSchema>
export type HistoricalImportFormValues = z.infer<
  typeof historicalImportFormSchema
>
