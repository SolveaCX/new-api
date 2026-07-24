/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import {
  listChannelBindings,
  listContracts,
  listEffectiveExclusions,
  listExclusionRules,
  listInventoryAdjustments,
  listRateVersions,
  listSuppliers,
} from '../api'
import { getNextAdminPage, mergeAdminPages } from '../lib/pagination'
import { supplyChainQueryKeys } from '../query-keys'
import type {
  SupplierChannelBindingListParams,
  SupplierContractChildListParams,
  SupplierContractListParams,
  SupplierExclusionListParams,
  SupplierListParams,
} from '../types'

const ADMIN_STALE_TIME = 15_000

export function useSupplierAdminList(params: SupplierListParams) {
  return useQuery({
    queryKey: supplyChainQueryKeys.suppliers.list(params),
    queryFn: async () => (await listSuppliers(params)).data,
    staleTime: ADMIN_STALE_TIME,
  })
}

export function useSupplierAdminInfiniteList(
  params: Omit<SupplierListParams, 'p'>,
  enabled = true
) {
  return useInfiniteQuery({
    queryKey: supplyChainQueryKeys.suppliers.list({ ...params, p: 1 }),
    queryFn: async ({ pageParam }) =>
      (await listSuppliers({ ...params, p: pageParam })).data,
    initialPageParam: 1,
    getNextPageParam: getNextAdminPage,
    select: (data) => mergeAdminPages(data.pages),
    enabled,
    staleTime: ADMIN_STALE_TIME,
  })
}

export function useContractAdminList(params: SupplierContractListParams) {
  return useQuery({
    queryKey: supplyChainQueryKeys.contracts.list(params),
    queryFn: async () => (await listContracts(params)).data,
    staleTime: ADMIN_STALE_TIME,
  })
}

export function useContractAdminInfiniteList(
  params: Omit<SupplierContractListParams, 'p'>,
  enabled = true
) {
  return useInfiniteQuery({
    queryKey: supplyChainQueryKeys.contracts.list({ ...params, p: 1 }),
    queryFn: async ({ pageParam }) =>
      (await listContracts({ ...params, p: pageParam })).data,
    initialPageParam: 1,
    getNextPageParam: getNextAdminPage,
    select: (data) => mergeAdminPages(data.pages),
    enabled,
    staleTime: ADMIN_STALE_TIME,
  })
}

export function useRateVersionList(
  params: SupplierContractChildListParams,
  enabled = true
) {
  return useInfiniteQuery({
    queryKey: supplyChainQueryKeys.contracts.rates(params),
    queryFn: async ({ pageParam }) =>
      (await listRateVersions({ ...params, p: pageParam })).data,
    initialPageParam: params.p,
    getNextPageParam: getNextAdminPage,
    select: (data) => mergeAdminPages(data.pages),
    enabled,
    staleTime: ADMIN_STALE_TIME,
  })
}

export function useInventoryAdjustmentList(
  params: SupplierContractChildListParams,
  enabled = true
) {
  return useInfiniteQuery({
    queryKey: supplyChainQueryKeys.contracts.inventoryAdjustments(params),
    queryFn: async ({ pageParam }) =>
      (await listInventoryAdjustments({ ...params, p: pageParam })).data,
    initialPageParam: params.p,
    getNextPageParam: getNextAdminPage,
    select: (data) => mergeAdminPages(data.pages),
    enabled,
    staleTime: ADMIN_STALE_TIME,
  })
}

export function useEffectiveExclusionList(params: SupplierExclusionListParams) {
  return useQuery({
    queryKey: supplyChainQueryKeys.exclusions.effective(params),
    queryFn: async () => (await listEffectiveExclusions(params)).data,
    staleTime: ADMIN_STALE_TIME,
  })
}

export function useExclusionHistoryList(
  params: SupplierExclusionListParams,
  enabled: boolean
) {
  return useInfiniteQuery({
    queryKey: supplyChainQueryKeys.exclusions.history(params),
    queryFn: async ({ pageParam }) =>
      (await listExclusionRules({ ...params, p: pageParam })).data,
    initialPageParam: params.p,
    getNextPageParam: getNextAdminPage,
    select: (data) => mergeAdminPages(data.pages),
    enabled,
    staleTime: ADMIN_STALE_TIME,
  })
}

export function useChannelBindingAdminList(
  params: SupplierChannelBindingListParams
) {
  return useQuery({
    queryKey: supplyChainQueryKeys.channelBindings.list(params),
    queryFn: async () => (await listChannelBindings(params)).data,
    staleTime: ADMIN_STALE_TIME,
  })
}

export function useSupplyChainAdminMutation<TVariables>(options: {
  mutationFn: (variables: TVariables) => Promise<unknown>
  invalidate: readonly (readonly unknown[])[]
}) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: options.mutationFn,
    retry: false,
    onSettled: async () => {
      await Promise.all(
        options.invalidate.map((queryKey) =>
          queryClient.invalidateQueries({ queryKey })
        )
      )
    },
  })
}
