/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { describe, expect, test } from 'bun:test'
import { isVerificationRequiredError } from '@/lib/secure-verification'
import { createSupplyChainSecurityLifecycle } from './use-supply-chain-admin'

function verificationRequiredError(): unknown {
  return {
    response: {
      status: 403,
      data: { code: 'VERIFICATION_REQUIRED' },
    },
  }
}

function createVerificationBridge() {
  let capturedRetry: (() => Promise<unknown>) | null = null
  let resetCount = 0
  return {
    bridge: {
      withVerification: async (apiCall: () => Promise<unknown>) => {
        try {
          return await apiCall()
        } catch (error) {
          if (!isVerificationRequiredError(error)) throw error
          capturedRetry = apiCall
          return null
        }
      },
      reset: () => {
        resetCount += 1
      },
    },
    ready: async () => {
      for (let attempt = 0; attempt < 10 && !capturedRetry; attempt += 1) {
        await Promise.resolve()
      }
      if (!capturedRetry) throw new Error('Verification retry was not captured')
    },
    retry: async () => {
      for (let attempt = 0; attempt < 10 && !capturedRetry; attempt += 1) {
        await Promise.resolve()
      }
      if (!capturedRetry) throw new Error('Verification retry was not captured')
      return capturedRetry()
    },
    resetCount: () => resetCount,
  }
}

describe('supply-chain secure mutation lifecycle', () => {
  test('keeps one intent pending until verification succeeds exactly once', async () => {
    const lifecycle = createSupplyChainSecurityLifecycle()
    const verification = createVerificationBridge()
    const key = 'supplier-create-stable-key'
    const observedKeys: string[] = []
    let attempts = 0
    let committedSends = 0
    const result = lifecycle.execute(async () => {
      attempts += 1
      observedKeys.push(key)
      if (attempts === 1) throw verificationRequiredError()
      committedSends += 1
      return 'created'
    }, verification.bridge)

    await verification.ready()
    expect(lifecycle.isPending()).toBe(true)
    expect(committedSends).toBe(0)
    await expect(
      lifecycle.execute(async () => 'double submit', verification.bridge)
    ).rejects.toThrow('already pending')

    expect(await verification.retry()).toBe('created')
    expect(await result).toBe('created')
    expect(observedKeys).toEqual([key, key])
    expect(committedSends).toBe(1)
    expect(lifecycle.isPending()).toBe(false)
  })

  test('keeps the pending callback after a verification-method failure', async () => {
    const lifecycle = createSupplyChainSecurityLifecycle()
    const verification = createVerificationBridge()
    let attempts = 0
    const result = lifecycle.execute(async () => {
      attempts += 1
      if (attempts === 1) throw verificationRequiredError()
      return 'updated'
    }, verification.bridge)

    await verification.ready()
    lifecycle.handleVerificationError(new Error('Invalid authenticator code'))
    expect(lifecycle.isPending()).toBe(true)
    expect(await verification.retry()).toBe('updated')
    expect(await result).toBe('updated')
    expect(attempts).toBe(2)
  })

  for (const status of [409, 500]) {
    test(`settles the original intent when the verified mutation returns ${status}`, async () => {
      const lifecycle = createSupplyChainSecurityLifecycle()
      const verification = createVerificationBridge()
      const failure = { response: { status } }
      const key = `stable-key-${status}`
      const observedKeys: string[] = []
      let attempts = 0
      const result = lifecycle.execute(async () => {
        attempts += 1
        observedKeys.push(key)
        if (attempts === 1) throw verificationRequiredError()
        throw failure
      }, verification.bridge)

      await verification.ready()
      const settledResult = result.then(
        () => null,
        (error: unknown) => error
      )
      const retryError = await verification.retry().then(
        () => null,
        (error: unknown) => error
      )
      expect(retryError).toBe(failure)
      expect(await settledResult).toBe(failure)
      expect(observedKeys).toEqual([key, key])
      expect(verification.resetCount()).toBe(1)
      expect(lifecycle.isPending()).toBe(false)
    })
  }

  test('cancellation sends no committed mutation and releases the intent', async () => {
    const lifecycle = createSupplyChainSecurityLifecycle()
    const verification = createVerificationBridge()
    let attempts = 0
    let committedSends = 0
    const result = lifecycle.execute(async () => {
      attempts += 1
      if (attempts === 1) throw verificationRequiredError()
      committedSends += 1
      return 'should not run'
    }, verification.bridge)

    await verification.ready()
    lifecycle.cancel()
    await expect(result).rejects.toMatchObject({
      name: 'SupplyChainVerificationCancelledError',
    })
    expect(committedSends).toBe(0)
    expect(lifecycle.isPending()).toBe(false)
  })
})

describe('supply-chain management mutation inventory', () => {
  test('routes every exposed management mutation through the shared security wrapper', async () => {
    const componentRoot = `${import.meta.dir}/../components`
    const sources = await Promise.all(
      [
        'supplier-management.tsx',
        'contract-management.tsx',
        'channel-binding-management.tsx',
        'exclusion-management.tsx',
      ].map((file) => Bun.file(`${componentRoot}/${file}`).text())
    )
    const source = sources.join('\n')

    for (const mutation of [
      'createSupplier',
      'updateSupplier',
      'inactivateSupplier',
      'createContract',
      'updateContract',
      'inactivateContract',
      'createRateVersion',
      'createInventoryAdjustment',
      'bindChannel',
      'unbindChannel',
      'createExclusionRule',
    ]) {
      expect(source).toContain(mutation)
    }
    expect(source.match(/useSupplyChainAdminMutation</g)).toHaveLength(9)
    expect(source.match(/useIdempotentIntent\(\)/g)).toHaveLength(9)
    expect(
      source.match(
        /await (?:intent|inactivateIntent)\.reconcilePending\(\)\) === 'reconciled'/g
      )
    ).toHaveLength(9)
    expect(
      source.match(
        /function finish(?:Save|Append|Binding|Unbind|Inactivate)\(/g
      )
    ).toHaveLength(9)
    expect(source).not.toContain(
      'onReconcile={() => void intent.reconcilePending()}'
    )
    expect(source).not.toContain(
      'onReconcile={() => void inactivateIntent.reconcilePending()}'
    )
    expect(source.match(/security:/g)?.length).toBeGreaterThanOrEqual(9)
    expect(source).toContain('expected_version: props.supplier.row_version')
    expect(source).toContain('expected_version: supplier.row_version')
    expect(source).toContain('expected_version: props.contract.row_version')
    expect(source).toContain('expected_version: contract.row_version')
    for (const exactScope of [
      'supplier.update/',
      'supplier.inactivate/',
      'supplier_contract.update/',
      'supplier_contract.inactivate/',
      'supplier_channel.bind/',
      'supplier_channel.unbind/',
    ]) {
      expect(source).toContain(exactScope)
    }

    const adminHook = await Bun.file(
      `${import.meta.dir}/use-supply-chain-admin.ts`
    ).text()
    expect(adminHook).toContain('verification.withVerification')
    expect(adminHook).toContain('options.security.execute')
  })
})
