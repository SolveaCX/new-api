/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import type {
  SupplierInventoryAdjustmentType,
  SupplierStatisticsAction,
} from '../types'

export const STATISTICS_ACTION_LABEL_KEYS: Record<
  SupplierStatisticsAction,
  'Marked internal' | 'Restored as business'
> = {
  exclude: 'Marked internal',
  include: 'Restored as business',
}

export const INVENTORY_ADJUSTMENT_LABEL_KEYS: Record<
  SupplierInventoryAdjustmentType,
  'Initial' | 'Replenishment' | 'Correction' | 'Reversal'
> = {
  initial: 'Initial',
  replenishment: 'Replenishment',
  correction: 'Correction',
  reversal: 'Reversal',
}
