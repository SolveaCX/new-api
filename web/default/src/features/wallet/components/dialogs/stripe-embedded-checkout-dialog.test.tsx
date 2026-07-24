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

For commercial licensing, please contact support@quantumnous.com
*/
import { describe, expect, test } from 'bun:test'

const isolatedFallbackProbe = String.raw`
  import { mock } from 'bun:test'

  const element = (type, props, key) => ({ type, props, key })
  mock.module('react/jsx-dev-runtime', () => ({
    Fragment: Symbol.for('react.fragment'),
    jsxDEV: element,
  }))
  mock.module('react', () => ({
    useCallback: (callback) => callback,
    useEffect: (effect) => effect(),
    useRef: () => ({ current: {} }),
    useState: (initialState) => [initialState, () => undefined],
  }))
  mock.module('react-i18next', () => ({
    useTranslation: () => ({ t: (key) => key }),
  }))
  mock.module('sonner', () => ({
    toast: { error: () => undefined, success: () => undefined },
  }))
  mock.module('lucide-react', () => ({
    Gift: () => null,
    Loader2: () => null,
  }))
  mock.module('@/components/dialog', () => ({
    Dialog: ({ children }) => children,
  }))
  mock.module('@stripe/stripe-js', () => ({
    loadStripe: async () => ({
      createEmbeddedCheckoutPage: async () => ({
        mount: () => { throw new Error('mount failed') },
        destroy: () => undefined,
      }),
    }),
  }))

  globalThis.window = {
    location: { assign: (url) => console.log(url) },
  }

  const { StripeEmbeddedCheckoutDialog } = await import(
    './stripe-embedded-checkout-dialog.tsx'
  )
  const dialog = StripeEmbeddedCheckoutDialog({
    session: {
      clientSecret: 'cs_test',
      publishableKey: 'pk_test',
      summary: null,
      fallbackUrl: 'https://checkout.example.com/session',
    },
    onOpenChange: () => undefined,
  })
  const frame = dialog.type(dialog.props)
  frame.type(frame.props)
  await Promise.resolve()
  await Promise.resolve()
`

describe('StripeEmbeddedCheckoutDialog', () => {
  test('opens the fallback URL when embedded checkout mount fails', async () => {
    const probe = Bun.spawn([process.execPath, '-e', isolatedFallbackProbe], {
      cwd: import.meta.dir,
      stdout: 'pipe',
      stderr: 'pipe',
    })
    const [exitCode, stdout, stderr] = await Promise.all([
      probe.exited,
      new Response(probe.stdout).text(),
      new Response(probe.stderr).text(),
    ])

    expect(stderr).toBe('')
    expect(exitCode).toBe(0)
    expect(stdout.trim()).toBe('https://checkout.example.com/session')
  })
})
