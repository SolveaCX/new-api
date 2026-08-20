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
import * as React from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterAll, beforeEach, describe, expect, mock, test } from 'bun:test'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import type { StripeCheckoutSession } from '@stripe/stripe-js'

type MountedRecord = {
  clientSecret: string
  destroy: ReturnType<typeof mock>
}

const mountedRecords: MountedRecord[] = []
const discountRequests: unknown[] = []
let latestPromotionControlProps:
  | {
      value: string
      busyAction?: 'apply' | 'restore' | null
      onValueChange: (value: string) => void
      onApply: () => void
      onRemove: () => void
    }
  | undefined

let latestSessionChange:
  | ((session: StripeCheckoutSession) => void)
  | undefined
let discountResponse:
  | {
      success: boolean
      data: {
        client_secret: string
        publishable_key: string
        fallback_url: string
        checkout_context: string
        checkout_revision: number
        discount_state: {
          source: 'manual'
          display_name: string
          promotion_code_masked: string
          replaced_source: 'invitation'
        }
        topup_summary: null
      }
    }
  | null = null
let discountResponsePromise:
  | Promise<typeof discountResponse>
  | null = null

function checkoutSession(): StripeCheckoutSession {
  return {
    canConfirm: true,
    currency: 'usd',
    currencyOptions: null,
    email: 'buyer@example.com',
    lineItems: [
      {
        name: 'Flatkey',
        description: 'Wallet top-up',
        subtotal: { amount: 'US$30.00', minorUnitsAmount: 3000 },
      },
    ],
    total: {
      discount: { amount: 'US$0.00', minorUnitsAmount: 0 },
      subtotal: { amount: 'US$30.00', minorUnitsAmount: 3000 },
      surcharge: { amount: 'US$0.00', minorUnitsAmount: 0 },
      taxExclusive: { amount: 'US$0.00', minorUnitsAmount: 0 },
      taxInclusive: { amount: 'US$0.00', minorUnitsAmount: 0 },
      total: { amount: 'US$30.00', minorUnitsAmount: 3000 },
    },
  } as unknown as StripeCheckoutSession
}

const updateStripeCheckoutDiscount = mock(async (request: unknown) => {
  discountRequests.push(request)
  if (discountResponsePromise) return await discountResponsePromise
  return discountResponse
})

const mountStripeCheckoutElements = mock(
  async (options: {
    clientSecret: string
    onSessionChange: (session: StripeCheckoutSession) => void
  }) => {
    latestSessionChange = options.onSessionChange
    const record = {
      clientSecret: options.clientSecret,
      destroy: mock(() => undefined),
    }
    mountedRecords.push(record)
    options.onSessionChange(checkoutSession())
    return {
      confirm: mock(async () => ({ type: 'success' })),
      destroy: record.destroy,
    }
  }
)

mock.module('../../api', () => ({
  updateStripeCheckoutDiscount,
}))

mock.module('../../api.ts', () => ({
  updateStripeCheckoutDiscount,
}))

mock.module('../../lib/stripe-checkout-elements', () => ({
  mountStripeCheckoutElements,
}))

mock.module('../../lib/stripe-checkout-elements.ts', () => ({
  mountStripeCheckoutElements,
}))

mock.module('sonner', () => ({
  toast: {
    error: mock(() => undefined),
    success: mock(() => undefined),
  },
}))

mock.module('lucide-react', () => ({
  Gift: () => null,
  Loader2: () => null,
  LockKeyhole: () => null,
  Tag: () => null,
  ShieldCheck: () => null,
  X: () => null,
}))

mock.module('@/components/ui/dialog', () => ({
  Dialog: (props: { children: React.ReactNode; open?: boolean }) =>
    props.open ? <>{props.children}</> : null,
  DialogClose: (props: { children: React.ReactNode }) => <>{props.children}</>,
  DialogContent: (props: { children: React.ReactNode }) => (
    <div>{props.children}</div>
  ),
  DialogDescription: (props: { children: React.ReactNode }) => (
    <p>{props.children}</p>
  ),
  DialogTitle: (props: { children: React.ReactNode }) => (
    <h2>{props.children}</h2>
  ),
}))

mock.module('./stripe-promotion-code-control', () => ({
  StripePromotionCodeControl: (props: {
    value: string
    busyAction?: 'apply' | 'restore' | null
    onValueChange: (value: string) => void
    onApply: () => void
    onRemove: () => void
  }) => {
    latestPromotionControlProps = props
    return (
      <div data-slot='stripe-promotion-code-control'>
        {props.busyAction === 'apply' ? 'Applying promotion code...' : null}
        {props.busyAction === 'restore'
          ? 'Restoring previous discount...'
          : null}
        <button type='button' onClick={props.onApply}>
          Apply
        </button>
      </div>
    )
  },
}))

const { StripeCheckoutDialog } = await import('./stripe-checkout-dialog')
const testI18n = createInstance()
await testI18n.use(initReactI18next).init({
  lng: 'en',
  fallbackLng: 'en',
  resources: {
    en: {
      translation: {
        Apply: 'Apply',
        Continue: 'Continue',
        'Promotion code': 'Promotion code',
        'Promotion code applied. Previous discount replaced.':
          'Promotion code applied. Previous discount replaced.',
      },
    },
  },
  interpolation: { escapeValue: false },
})

const originalGlobalPropertyDescriptors = new Map<
  PropertyKey,
  PropertyDescriptor | undefined
>()

function defineTestGlobal(key: PropertyKey, value: unknown) {
  if (!originalGlobalPropertyDescriptors.has(key)) {
    originalGlobalPropertyDescriptors.set(
      key,
      Object.getOwnPropertyDescriptor(globalThis, key)
    )
  }
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value,
    writable: true,
  })
}

function restoreTestGlobals() {
  for (const [key, descriptor] of originalGlobalPropertyDescriptors) {
    if (descriptor) {
      Object.defineProperty(globalThis, key, descriptor)
    } else {
      Reflect.deleteProperty(globalThis, key)
    }
  }
}

function setupDom() {
  class NodeShim {
    childNodes: NodeShim[] = []
    nodeType = 0
    nodeName = ''
    parentNode: NodeShim | null = null
    ownerDocument = globalThis.document
    private listeners: Record<string, EventListener[]> = {}

    appendChild(node: NodeShim) {
      this.childNodes.push(node)
      node.parentNode = this
      return node
    }

    insertBefore(node: NodeShim, before: NodeShim | null) {
      const index = before ? this.childNodes.indexOf(before) : -1
      if (index < 0) return this.appendChild(node)
      this.childNodes.splice(index, 0, node)
      node.parentNode = this
      return node
    }

    removeChild(node: NodeShim) {
      this.childNodes = this.childNodes.filter((child) => child !== node)
      node.parentNode = null
      return node
    }

    addEventListener(type: string, listener: EventListener) {
      this.listeners[type] ??= []
      this.listeners[type].push(listener)
    }

    removeEventListener(type: string, listener: EventListener) {
      this.listeners[type] = (this.listeners[type] ?? []).filter(
        (current) => current !== listener
      )
    }

    dispatchEvent(event: Event) {
      Object.defineProperty(event, 'target', {
        configurable: true,
        value: this,
      })
      Object.defineProperty(event, 'currentTarget', {
        configurable: true,
        value: this,
      })
      for (const listener of this.listeners[event.type] ?? []) {
        listener.call(this, event)
      }
      if (event.bubbles && this.parentNode) {
        this.parentNode.dispatchEvent(event)
      }
      return !event.defaultPrevented
    }
  }

  class ElementShim extends NodeShim {
    attributes: Record<string, string> = {}
    disabled = false
    localName: string
    namespaceURI = 'http://www.w3.org/1999/xhtml'
    style = {}
    tagName: string
    type = ''
    value = ''
    private text = ''

    constructor(tagName: string) {
      super()
      this.nodeType = 1
      this.localName = tagName
      this.tagName = tagName.toUpperCase()
      this.nodeName = this.tagName
    }

    set textContent(value: string) {
      this.text = String(value)
      this.childNodes = []
    }

    get textContent() {
      return (
        this.text ||
        this.childNodes
          .map((node) => ('textContent' in node ? node.textContent : ''))
          .join('')
      )
    }

    setAttribute(key: string, value: string) {
      this.attributes[key] = String(value)
      if (key === 'disabled') this.disabled = true
      if (key === 'value') this.value = String(value)
      if (key === 'type') this.type = String(value)
    }

    removeAttribute(key: string) {
      delete this.attributes[key]
      if (key === 'disabled') this.disabled = false
    }

    querySelector(selector: string): ElementShim | null {
      if (matchesSelector(this, selector)) return this
      for (const child of this.childNodes) {
        if (child instanceof ElementShim) {
          const match = child.querySelector(selector)
          if (match) return match
        }
      }
      return null
    }

    querySelectorAll(selector: string): ElementShim[] {
      const matches: ElementShim[] = []
      if (matchesSelector(this, selector)) matches.push(this)
      for (const child of this.childNodes) {
        if (child instanceof ElementShim) {
          matches.push(...child.querySelectorAll(selector))
        }
      }
      return matches
    }
  }

  class TextShim extends NodeShim {
    textContent: string

    constructor(text: string) {
      super()
      this.nodeType = 3
      this.nodeName = '#text'
      this.textContent = text
    }
  }

  const body = new ElementShim('body')
  const head = new ElementShim('head')
  defineTestGlobal('document', {
    nodeType: 9,
    body,
    head,
    createElement: (tagName: string) => new ElementShim(tagName),
    createElementNS: (_namespace: string, tagName: string) =>
      new ElementShim(tagName),
    createTextNode: (text: string) => new TextShim(text),
    getElementsByTagName: (tagName: string) =>
      tagName.toLowerCase() === 'head' ? [head] : [],
    addEventListener() {},
    removeEventListener() {},
    defaultView: globalThis,
  } as unknown as Document)
  defineTestGlobal(
    'window',
    Object.assign(globalThis, {
      location: {
        href: 'http://localhost/wallet',
        origin: 'http://localhost',
        assign: mock(() => undefined),
      },
    }) as unknown as Window & typeof globalThis
  )
  defineTestGlobal('navigator', { userAgent: 'Chrome' } as Navigator)
  defineTestGlobal('HTMLElement', ElementShim as unknown as typeof HTMLElement)
  defineTestGlobal('HTMLIFrameElement', class {} as typeof HTMLIFrameElement)
  defineTestGlobal('Node', NodeShim as unknown as typeof Node)
  defineTestGlobal('IS_REACT_ACT_ENVIRONMENT', true)
  defineTestGlobal('crypto', { randomUUID: () => 'request-1' })
}

function matchesSelector(element: { tagName: string }, selector: string) {
  return element.tagName.toLowerCase() === selector.toLowerCase()
}

setupDom()

function renderDialog() {
  const container = document.createElement('div')
  const root = createRoot(container)

  React.act(() => {
    root.render(
      <I18nextProvider i18n={testI18n}>
        <StripeCheckoutDialog
          session={{
            clientSecret: 'cs_initial',
            publishableKey: 'pk_initial',
            checkoutContext: 'ctx-1',
            checkoutRevision: 1,
            discountState: { source: 'invitation', display_name: 'Invite' },
            summary: null,
          }}
          onOpenChange={() => undefined}
        />
      </I18nextProvider>
    )
  })

  return { container, root }
}

function pendingDiscountResponse() {
  let resolve!: (value: typeof discountResponse) => void
  discountResponsePromise = new Promise<typeof discountResponse>((next) => {
    resolve = next
  })
  return {
    resolve: () => {
      resolve(discountResponse)
      discountResponsePromise = null
    },
  }
}

function dispose(root: Root) {
  React.act(() => {
    root.unmount()
  })
}

beforeEach(() => {
  mountedRecords.length = 0
  discountRequests.length = 0
  latestPromotionControlProps = undefined
  discountResponse = {
    success: true,
    data: {
      client_secret: 'cs_next',
      publishable_key: 'pk_next',
      fallback_url: 'https://checkout.example.test/next',
      checkout_context: 'ctx-2',
      checkout_revision: 2,
      discount_state: {
        source: 'manual',
        display_name: 'SAVE20',
        promotion_code_masked: 'SAVE20',
        replaced_source: 'invitation',
      },
      topup_summary: null,
    },
  }
  discountResponsePromise = null
  latestSessionChange = undefined
  mountStripeCheckoutElements.mockClear()
  updateStripeCheckoutDiscount.mockClear()
})

afterAll(() => {
  mock.restore()
  restoreTestGlobals()
})

describe('StripeCheckoutDialog promotion code interactions', () => {
  test('passes the active apply mutation as promotion control busy action', async () => {
    const { root } = renderDialog()
    const pending = pendingDiscountResponse()

    try {
      await React.act(async () => {
        await Promise.resolve()
        latestSessionChange?.(checkoutSession())
        await Promise.resolve()
      })

      await React.act(async () => {
        latestPromotionControlProps?.onValueChange('SAVE20')
        await Promise.resolve()
      })
      React.act(() => {
        latestPromotionControlProps?.onApply()
      })

      await React.act(async () => {
        await Promise.resolve()
      })

      expect(latestPromotionControlProps?.busyAction).toBe('apply')

      await React.act(async () => {
        pending.resolve()
        await Promise.resolve()
        await Promise.resolve()
      })
    } finally {
      dispose(root)
    }
  })

  test('submits a trimmed code and remounts Checkout Elements after success', async () => {
    const { container, root } = renderDialog()

    try {
      await React.act(async () => {
        await Promise.resolve()
        latestSessionChange?.(checkoutSession())
        await Promise.resolve()
      })

      await React.act(async () => {
        latestPromotionControlProps?.onValueChange('  SAVE20  ')
        await Promise.resolve()
      })
      await React.act(async () => {
        latestPromotionControlProps?.onApply()
        await Promise.resolve()
        await Promise.resolve()
      })

      expect(discountRequests).toEqual([
        {
          action: 'apply',
          checkout_context: 'ctx-1',
          expected_revision: 1,
          promotion_code: 'SAVE20',
          request_id: 'request-1',
        },
      ])
      expect(mountedRecords.map((record) => record.clientSecret)).toEqual([
        'cs_initial',
        'cs_next',
      ])
      expect(mountedRecords[0]?.destroy).toHaveBeenCalledTimes(1)
    } finally {
      dispose(root)
    }
  })
})
