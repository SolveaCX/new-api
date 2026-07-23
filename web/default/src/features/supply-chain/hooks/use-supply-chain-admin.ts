/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useCallback, useState } from 'react'
import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import { isVerificationRequiredError } from '@/lib/secure-verification'
import { useSecureVerification } from '@/features/auth/secure-verification'
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

const VERIFICATION_CANCELLED_ERROR = 'SupplyChainVerificationCancelledError'

interface PendingVerification {
  promise: Promise<unknown>
  resolve: (value: unknown) => void
  reject: (reason: unknown) => void
  verificationActive: boolean
}

interface VerificationBridge {
  withVerification: (apiCall: () => Promise<unknown>) => Promise<unknown | null>
  reset: () => void
}

export function createSupplyChainSecurityLifecycle(
  onPendingChange: (pending: boolean) => void = () => undefined
) {
  let pending: PendingVerification | null = null

  function finish(
    target: PendingVerification,
    result: 'resolve' | 'reject',
    value: unknown
  ): void {
    if (pending !== target) return
    pending = null
    onPendingChange(false)
    if (result === 'resolve') target.resolve(value)
    else target.reject(value)
  }

  function execute(
    apiCall: () => Promise<unknown>,
    verification: VerificationBridge
  ): Promise<unknown> {
    if (pending) {
      return Promise.reject(new Error('A secure mutation is already pending'))
    }

    let resolvePromise!: (value: unknown) => void
    let rejectPromise!: (reason: unknown) => void
    const promise = new Promise<unknown>((resolve, reject) => {
      resolvePromise = resolve
      rejectPromise = reject
    })
    const target: PendingVerification = {
      promise,
      resolve: resolvePromise,
      reject: rejectPromise,
      verificationActive: false,
    }
    pending = target
    onPendingChange(true)

    void verification
      .withVerification(async () => {
        try {
          const result = await apiCall()
          finish(target, 'resolve', result)
          return result
        } catch (error) {
          if (!isVerificationRequiredError(error)) {
            finish(target, 'reject', error)
            verification.reset()
          }
          throw error
        }
      })
      .then((result) => {
        if (result === null) {
          if (pending === target) target.verificationActive = true
          return
        }
        finish(target, 'resolve', result)
      })
      .catch((error: unknown) => {
        finish(target, 'reject', error)
      })

    return promise
  }

  function handleVerificationError(error: unknown): void {
    if (pending && !pending.verificationActive) {
      finish(pending, 'reject', error)
    }
  }

  function cancel(): void {
    if (!pending) return
    const error = new Error('Secure verification was cancelled')
    error.name = VERIFICATION_CANCELLED_ERROR
    finish(pending, 'reject', error)
  }

  return {
    execute,
    handleVerificationError,
    cancel,
    isPending: () => pending !== null,
  }
}

export function useSupplyChainSecurity() {
  const [isPending, setIsPending] = useState(false)
  const [lifecycle] = useState(() =>
    createSupplyChainSecurityLifecycle(setIsPending)
  )

  const verification = useSecureVerification({
    onError: lifecycle.handleVerificationError,
  })

  const execute = useCallback(
    (apiCall: () => Promise<unknown>) =>
      lifecycle.execute(apiCall, {
        withVerification: verification.withVerification,
        reset: verification.reset,
      }),
    [lifecycle, verification]
  )

  const cancel = useCallback(() => {
    lifecycle.cancel()
    verification.cancel()
  }, [lifecycle, verification])

  return { execute, cancel, verification, isPending }
}

export type SupplyChainSecurity = ReturnType<typeof useSupplyChainSecurity>

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
  security: SupplyChainSecurity
}) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (variables: TVariables) =>
      options.security.execute(() => options.mutationFn(variables)),
    retry: false,
    onSuccess: async () => {
      await Promise.all(
        options.invalidate.map((queryKey) =>
          queryClient.invalidateQueries({ queryKey })
        )
      )
    },
  })
}
