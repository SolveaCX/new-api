/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  getAccountingPolicyCapability,
  updateAccountingPolicyCapability,
} from '../api'
import { supplyChainQueryKeys } from '../query-keys'
import type { SupplierAccountingPolicyCapability } from '../types'

const POLICY_STALE_TIME = 15_000
const POLICY_PENDING_REFETCH_INTERVAL = 5_000

export function isAccountingPolicyConfigurable(
  capability: SupplierAccountingPolicyCapability | undefined,
  isError: boolean
): boolean {
  return !isError && Boolean(capability?.activated || capability?.active)
}

export function useAccountingPolicyCapability() {
  return useQuery({
    queryKey: supplyChainQueryKeys.accountingPolicy.capability(),
    queryFn: async () => (await getAccountingPolicyCapability()).data,
    staleTime: POLICY_STALE_TIME,
    refetchInterval: (query) => {
      const capability = query.state.data
      return capability && capability.activated !== capability.active
        ? POLICY_PENDING_REFETCH_INTERVAL
        : false
    },
  })
}

export function useUpdateAccountingPolicyCapability() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (activated: boolean) =>
      updateAccountingPolicyCapability({ activated }),
    retry: false,
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: supplyChainQueryKeys.accountingPolicy.all(),
      })
    },
  })
}
