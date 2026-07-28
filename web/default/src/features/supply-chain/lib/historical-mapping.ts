import type {
  SupplierChannelBinding,
  SupplierContractRateVersion,
  SupplierHistoricalChannelMapping,
} from '../types'

export function buildHistoricalMappings(
  bindings: SupplierChannelBinding[]
): SupplierHistoricalChannelMapping[] {
  return bindings
    .flatMap((binding) => {
      if (
        binding.supplier_id === null ||
        binding.supplier_contract_id === null ||
        binding.current_rate_version_id === null ||
        binding.current_procurement_multiplier_ppm === null
      ) {
        return []
      }
      return [
        {
          channel_id: binding.channel_id,
          supplier_id: binding.supplier_id,
          contract_id: binding.supplier_contract_id,
          rate_version_id: binding.current_rate_version_id,
          procurement_multiplier_ppm:
            binding.current_procurement_multiplier_ppm,
        },
      ]
    })
    .sort((left, right) => left.channel_id - right.channel_id)
}

export function parseHistoricalMappings(
  value: string
): SupplierHistoricalChannelMapping[] {
  try {
    const parsed: unknown = JSON.parse(value)
    if (!Array.isArray(parsed)) return []
    return parsed.filter((item): item is SupplierHistoricalChannelMapping => {
      if (!item || typeof item !== 'object') return false
      const mapping = item as Record<string, unknown>
      return (
        Number.isInteger(mapping.channel_id) &&
        Number.isInteger(mapping.supplier_id) &&
        Number.isInteger(mapping.contract_id) &&
        Number.isInteger(mapping.rate_version_id) &&
        Number.isInteger(mapping.procurement_multiplier_ppm)
      )
    })
  } catch {
    return []
  }
}

export function replaceHistoricalMappingRateVersion(
  mappings: SupplierHistoricalChannelMapping[],
  channelId: number,
  rateVersion: SupplierContractRateVersion
): SupplierHistoricalChannelMapping[] {
  return mappings.map((mapping) => {
    if (
      mapping.channel_id !== channelId ||
      mapping.contract_id !== rateVersion.contract_id
    ) {
      return mapping
    }
    return {
      ...mapping,
      rate_version_id: rateVersion.id,
      procurement_multiplier_ppm: rateVersion.procurement_multiplier_ppm,
    }
  })
}
