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
})

export type SupplierFormValues = z.infer<typeof supplierFormSchema>
export type ContractFormValues = z.infer<typeof contractFormSchema>
export type RateVersionFormValues = z.infer<typeof rateVersionFormSchema>
export type InventoryAdjustmentFormValues = z.infer<
  typeof inventoryAdjustmentFormSchema
>
export type ExclusionFormValues = z.infer<typeof exclusionFormSchema>
export type ChannelBindingFormValues = z.infer<typeof channelBindingFormSchema>
