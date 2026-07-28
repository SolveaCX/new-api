/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  getAccountingRuntimeSettings,
  updateAccountingRuntimeSettings,
} from '../api'
import { supplyChainQueryKeys } from '../query-keys'
import type { SupplierAccountingRuntimeSettingsRequest } from '../types'

export function useAccountingRuntimeSettings() {
  return useQuery({
    queryKey: supplyChainQueryKeys.runtimeSettings.detail(),
    queryFn: async () => (await getAccountingRuntimeSettings()).data,
    staleTime: 15_000,
    refetchInterval: (query) => {
      const settings = query.state.data
      return settings && settings.cutover_at > 0 && !settings.cutover_locked
        ? 15_000
        : false
    },
  })
}

export function useUpdateAccountingRuntimeSettings() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: SupplierAccountingRuntimeSettingsRequest) =>
      updateAccountingRuntimeSettings(data),
    retry: false,
    onSettled: async () => {
      await queryClient.invalidateQueries({
        queryKey: supplyChainQueryKeys.runtimeSettings.all(),
      })
    },
  })
}
