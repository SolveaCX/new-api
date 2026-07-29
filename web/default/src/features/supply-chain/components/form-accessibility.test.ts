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
*/
import { describe, expect, test } from 'bun:test'

const componentDirectory = import.meta.dir

async function readComponent(name: string): Promise<string> {
  return Bun.file(`${componentDirectory}/${name}`).text()
}

function expectLabelAssociation(source: string, idExpression: string): void {
  expect(source).toContain(`htmlFor={\`${idExpression}\`}`)
  expect(source).toContain(`id={\`${idExpression}\`}`)
}

describe('supply-chain form accessibility', () => {
  test('keeps every visible form label associated for getByRole name lookup', async () => {
    const contract = await readComponent('contract-management.tsx')
    const exclusion = await readComponent('exclusion-management.tsx')
    const binding = await readComponent('channel-binding-management.tsx')
    const batchBinding = await readComponent('channel-batch-binding-dialog.tsx')
    const policy = await readComponent('accounting-policy-activation.tsx')

    for (const id of [
      "contract-supplier-${props.contract?.id ?? 'new'}",
      "contract-number-${props.contract?.id ?? 'new'}",
      "contract-name-${props.contract?.id ?? 'new'}",
      "contract-concurrency-${props.contract?.id ?? 'new'}",
      "contract-rpm-${props.contract?.id ?? 'new'}",
      "contract-tpm-${props.contract?.id ?? 'new'}",
      "contract-remark-${props.contract?.id ?? 'new'}",
      'rate-ppm-${props.contract.id}',
      'rate-reason-${props.contract.id}',
      'inventory-delta-${props.contract.id}',
      'inventory-type-${props.contract.id}',
      'inventory-reason-${props.contract.id}',
    ]) {
      expectLabelAssociation(contract, id)
    }

    for (const id of [
      "exclusion-user-${props.row?.user_id ?? 'new'}",
      "exclusion-reason-${props.row?.user_id ?? 'new'}",
    ]) {
      expectLabelAssociation(exclusion, id)
    }
    expect(exclusion).not.toContain("form.register('action')")
    expect(exclusion).toContain("return row?.excluded ? 'include' : 'exclude'")

    expectLabelAssociation(
      binding,
      'binding-contract-${props.binding.channel_id}'
    )
    expect(batchBinding).toContain("htmlFor='channel-batch-binding-contract'")
    expect(batchBinding).toContain("id='channel-batch-binding-contract'")
    expect(batchBinding).toContain("htmlFor='channel-batch-binding-policy'")
    expect(batchBinding).toContain("id='channel-batch-binding-policy'")
    expect(policy).toContain("htmlFor='supplier-accounting-policy-activation'")
    expect(policy).toContain("id='supplier-accounting-policy-activation'")
  })

  test('wires validation state to contract capacity controls and messages', async () => {
    const contract = await readComponent('contract-management.tsx')

    for (const field of ['max_concurrency', 'rpm_limit', 'tpm_limit']) {
      expect(contract).toContain(
        `data-invalid={Boolean(form.formState.errors.${field})}`
      )
      expect(contract).toMatch(
        new RegExp(
          `aria-invalid=\\{Boolean\\(\\s*form\\.formState\\.errors\\.${field}\\s*\\)\\}`
        )
      )
      expect(contract).toContain(`form.formState.errors.${field}.message`)
    }
  })

  test('wires validation state to every supply-chain form control', async () => {
    const sources = await Promise.all([
      readComponent('supplier-management.tsx'),
      readComponent('contract-management.tsx'),
      readComponent('exclusion-management.tsx'),
      readComponent('channel-binding-management.tsx'),
      readComponent('channel-batch-binding-dialog.tsx'),
    ])

    for (const source of sources) {
      const validatedFields = source.match(/data-invalid=/g)?.length ?? 0
      const validatedControls = source.match(/aria-invalid=/g)?.length ?? 0
      expect(validatedControls).toBe(validatedFields)
    }
  })

  test('disables complete skip until the global policy is configurable', async () => {
    const binding = await readComponent('channel-binding-management.tsx')
    const batchBinding = await readComponent('channel-batch-binding-dialog.tsx')

    expect(binding).toContain('disabled={!props.policyConfigurable}')
    expect(batchBinding).toContain('disabled={!props.policyConfigurable}')
    expect(binding).toContain('isAccountingPolicyConfigurable(')
    expect(binding).toContain(
      'The saved skip setting cannot be verified because the global policy status is unavailable.'
    )
    expect(binding).toContain(
      'Configured to skip; currently recording internal costs'
    )
  })
})
