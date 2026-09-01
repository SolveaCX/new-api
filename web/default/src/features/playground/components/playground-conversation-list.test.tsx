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
import {
  afterAll,
  afterEach,
  beforeAll,
  beforeEach,
  describe,
  expect,
  mock,
  spyOn,
  test,
} from 'bun:test'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import type { PlaygroundConversationSummary } from '../types'
import * as playgroundExportModule from '../lib/playground-export'

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
  if (typeof document !== 'undefined' && document.body) {
    defineTestGlobal('IS_REACT_ACT_ENVIRONMENT', true)
    return
  }

  class NodeShim {
    childNodes: NodeShim[] = []
    nodeType = 0
    nodeName = ''
    parentNode: NodeShim | null = null
    ownerDocument?: Document
    private listeners: Record<string, EventListener[]> = {}

    appendChild(node: NodeShim) {
      this.childNodes.push(node)
      node.parentNode = this
      return node
    }

    insertBefore(node: NodeShim, before: NodeShim | null) {
      if (!before) return this.appendChild(node)
      const index = this.childNodes.indexOf(before)
      if (index === -1) return this.appendChild(node)
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
      if (!('target' in event) || event.target === null) {
        Object.defineProperty(event, 'target', { value: this })
      }
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
    style: Record<string, string> = {}
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

    getAttribute(key: string) {
      return this.attributes[key] ?? null
    }

    removeAttribute(key: string) {
      delete this.attributes[key]
      if (key === 'disabled') this.disabled = false
    }

    click() {
      if (this.disabled) return
      this.dispatchEvent(
        new Event('click', { bubbles: true, cancelable: true })
      )
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

  const head = new ElementShim('head')
  const body = new ElementShim('body')
  const documentShim = {
    nodeType: 9,
    body,
    head,
    createElement: (tagName: string) => {
      const element = new ElementShim(tagName)
      element.ownerDocument = documentShim as unknown as Document
      return element
    },
    createElementNS: (_namespace: string, tagName: string) => {
      const element = new ElementShim(tagName)
      element.ownerDocument = documentShim as unknown as Document
      return element
    },
    createTextNode: (text: string) => new TextShim(text),
    addEventListener() {},
    removeEventListener() {},
    defaultView: globalThis,
  }

  body.ownerDocument = documentShim as unknown as Document
  head.ownerDocument = documentShim as unknown as Document

  defineTestGlobal('document', documentShim as unknown as Document)
  defineTestGlobal(
    'window',
    globalThis as unknown as Window & typeof globalThis
  )
  defineTestGlobal('matchMedia', (query: string) => ({
    matches: query === '(min-width: 768px)' ? currentMatchMediaMatches : false,
    media: query,
    onchange: null,
    addListener: () => undefined,
    removeListener: () => undefined,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    dispatchEvent: () => false,
  }))
  defineTestGlobal('navigator', { userAgent: 'Chrome' } as Navigator)
  defineTestGlobal('HTMLElement', ElementShim as unknown as typeof HTMLElement)
  defineTestGlobal('HTMLIFrameElement', class {} as typeof HTMLIFrameElement)
  defineTestGlobal('Node', NodeShim as unknown as typeof Node)
  defineTestGlobal('IS_REACT_ACT_ENVIRONMENT', true)
}

setupDom()

type DownloadResult = { blob: Blob; filename: string }

let currentIsAdmin = true
let currentMatchMediaMatches = true
let currentQueryData: PlaygroundConversationSummary[] = []
let currentDownloadOutcome:
  | { kind: 'resolve'; result: DownloadResult }
  | { kind: 'reject'; error: Error }
  | { kind: 'pending'; promise: Promise<DownloadResult> } = {
  kind: 'resolve',
  result: {
    blob: new Blob(['xlsx']),
    filename: 'playground-records.xlsx',
  },
}

const downloadCalls: number[] = []
const triggerCalls: DownloadResult[] = []
const toastCalls = {
  success: [] as string[],
  error: [] as string[],
}

class PlaygroundRecordExportError extends Error {
  status?: number

  constructor(message: string, status?: number) {
    super(message)
    this.name = 'PlaygroundRecordExportError'
    this.status = status
  }
}

const downloadPlaygroundRecords = async () => {
  downloadCalls.push(downloadCalls.length + 1)
  if (currentDownloadOutcome.kind === 'reject') {
    throw currentDownloadOutcome.error
  }
  if (currentDownloadOutcome.kind === 'pending') {
    return await currentDownloadOutcome.promise
  }
  return currentDownloadOutcome.result
}

const listPlaygroundConversations = async () => currentQueryData
const deletePlaygroundConversations = async () => undefined
const renamePlaygroundConversation = async () => undefined
const triggerPlaygroundExport = (blob: Blob, filename: string) => {
  triggerCalls.push({ blob, filename })
}

mock.module('@/components/ui/button', () => ({
  Button: (props: React.ButtonHTMLAttributes<HTMLButtonElement>) => (
    <button {...props} />
  ),
}))

mock.module('@/components/ui/checkbox', () => ({
  Checkbox: (props: React.InputHTMLAttributes<HTMLInputElement>) => (
    <input type='checkbox' {...props} />
  ),
}))

mock.module('@/components/ui/input', () => ({
  Input: (props: React.InputHTMLAttributes<HTMLInputElement>) => (
    <input {...props} />
  ),
}))

mock.module('@/components/ui/scroll-area', () => ({
  ScrollArea: (props: React.HTMLAttributes<HTMLDivElement>) => (
    <div {...props} />
  ),
}))

mock.module('@/components/ui/alert-dialog', () => ({
  AlertDialog: (props: { open?: boolean; children: React.ReactNode }) =>
    props.open ? <div>{props.children}</div> : null,
  AlertDialogAction: (props: React.ButtonHTMLAttributes<HTMLButtonElement>) => (
    <button {...props} />
  ),
  AlertDialogCancel: (props: React.ButtonHTMLAttributes<HTMLButtonElement>) => (
    <button {...props} />
  ),
  AlertDialogContent: (props: React.HTMLAttributes<HTMLDivElement>) => (
    <div {...props} />
  ),
  AlertDialogDescription: (
    props: React.HTMLAttributes<HTMLParagraphElement>
  ) => <p {...props} />,
  AlertDialogFooter: (props: React.HTMLAttributes<HTMLDivElement>) => (
    <div {...props} />
  ),
  AlertDialogHeader: (props: React.HTMLAttributes<HTMLDivElement>) => (
    <div {...props} />
  ),
  AlertDialogTitle: (props: React.HTMLAttributes<HTMLHeadingElement>) => (
    <h2 {...props} />
  ),
}))

mock.module('@/hooks/use-admin', () => ({
  useIsAdmin: () => currentIsAdmin,
}))

mock.module('../api', () => ({
  PlaygroundRecordExportError,
  deletePlaygroundConversations,
  downloadPlaygroundRecords,
  listPlaygroundConversations,
  renamePlaygroundConversation,
}))

spyOn(playgroundExportModule, 'triggerPlaygroundExport').mockImplementation(
  triggerPlaygroundExport
)

mock.module('sonner', () => ({
  toast: {
    success: (message: string) => {
      toastCalls.success.push(message)
    },
    error: (message: string) => {
      toastCalls.error.push(message)
    },
  },
}))

const reactQueryModule = await import('@tanstack/react-query')
const useQueryMock = spyOn(reactQueryModule, 'useQuery')
useQueryMock.mockImplementation(
  () =>
    ({
      data: currentQueryData,
      isLoading: false,
    }) as never
)

const { PlaygroundConversationList } =
  await import('./playground-conversation-list')

const testI18n = createInstance()

beforeAll(async () => {
  await testI18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    resources: {
      en: {
        translation: {
          New: 'New',
          'Batch export': 'Batch export',
          'Exporting Playground records...': 'Exporting Playground records...',
          'Playground records export started':
            'Playground records export started',
          'Failed to export Playground records':
            'Failed to export Playground records',
          'You do not have permission to export Playground records':
            'You do not have permission to export Playground records',
          'Batch operations': 'Batch operations',
          Done: 'Done',
          Conversations: 'Conversations',
          'Expand conversations': 'Expand conversations',
          'Collapse conversations': 'Collapse conversations',
          'Loading conversations...': 'Loading conversations...',
          'Select all': 'Select all',
          'Deselect all': 'Deselect all',
          Delete: 'Delete',
          Rename: 'Rename',
          Save: 'Save',
          Cancel: 'Cancel',
          'Select conversation': 'Select conversation',
          'No conversations yet': 'No conversations yet',
          'Delete selected conversations?': 'Delete selected conversations?',
          'Delete this conversation?': 'Delete this conversation?',
          'Deleted conversations cannot be recovered.':
            'Deleted conversations cannot be recovered.',
        },
      },
    },
    interpolation: { escapeValue: false },
  })
})

afterAll(() => {
  mock.restore()
  restoreTestGlobals()
})

let mountedRoots: Array<{ root: Root; container: HTMLElement }> = []

function resetState() {
  currentIsAdmin = true
  currentMatchMediaMatches = true
  currentQueryData = []
  currentDownloadOutcome = {
    kind: 'resolve',
    result: {
      blob: new Blob(['xlsx']),
      filename: 'playground-records.xlsx',
    },
  }
  downloadCalls.length = 0
  triggerCalls.length = 0
  toastCalls.success.length = 0
  toastCalls.error.length = 0
  useQueryMock.mockClear()
  useQueryMock.mockImplementation(
    () =>
      ({
        data: currentQueryData,
        isLoading: false,
      }) as never
  )
}

beforeEach(() => {
  resetState()
})

afterEach(async () => {
  for (const render of mountedRoots) {
    await React.act(async () => {
      render.root.unmount()
    })
  }
  mountedRoots = []
})

function renderRail(options?: {
  isAdmin?: boolean
  matchMediaMatches?: boolean
  conversations?: PlaygroundConversationSummary[]
  currentConversationId?: string
}) {
  currentIsAdmin = options?.isAdmin ?? true
  currentMatchMediaMatches = options?.matchMediaMatches ?? true
  currentQueryData = options?.conversations ?? []
  useQueryMock.mockImplementation(
    () =>
      ({
        data: currentQueryData,
        isLoading: false,
      }) as never
  )

  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)

  React.act(() => {
    root.render(
      <I18nextProvider i18n={testI18n}>
        <PlaygroundConversationList
          currentConversationId={options?.currentConversationId ?? 'current'}
          onNew={() => undefined}
          onSelect={() => undefined}
        />
      </I18nextProvider>
    )
  })

  mountedRoots.push({ root, container: container as unknown as HTMLElement })
  return {
    container: container as unknown as HTMLElement,
    root,
  }
}

function findButton(
  container: HTMLElement,
  predicate: (button: HTMLButtonElement) => boolean
) {
  const stack: unknown[] = [container]
  while (stack.length > 0) {
    const node = stack.shift()
    if (!node || typeof node !== 'object') continue
    const element = node as {
      nodeType?: number
      tagName?: string
      textContent?: string | null
      getAttribute?: (name: string) => string | null
      childNodes?: unknown[]
    }
    if (
      element.nodeType === 1 &&
      element.tagName === 'BUTTON' &&
      predicate(element as HTMLButtonElement)
    ) {
      return element as HTMLButtonElement
    }
    for (const child of element.childNodes ?? []) {
      stack.push(child)
    }
  }
  throw new Error('Button not found')
}

function buttonByText(container: HTMLElement, text: string) {
  return findButton(container, (button) =>
    (button.textContent ?? '').includes(text)
  )
}

function buttonByLabel(container: HTMLElement, label: string) {
  return findButton(
    container,
    (button) => button.getAttribute('aria-label') === label
  )
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

describe('PlaygroundConversationList', () => {
  test('renders Batch export above New only for administrators', () => {
    const adminRender = renderRail({ isAdmin: true, matchMediaMatches: true })
    const adminText = adminRender.container.textContent ?? ''

    expect(adminText).toContain('Batch export')
    expect(adminText).toContain('New')
    expect(adminText.indexOf('Batch export')).toBeLessThan(
      adminText.indexOf('New')
    )

    const userRender = renderRail({ isAdmin: false, matchMediaMatches: true })
    const userText = userRender.container.textContent ?? ''
    expect(userText).not.toContain('Batch export')
    expect(userText).toContain('New')
  })

  test('keeps the export action hidden until the rail is expanded', async () => {
    const render = renderRail({ isAdmin: true, matchMediaMatches: false })
    expect(render.container.textContent ?? '').not.toContain('Batch export')

    await React.act(async () => {
      buttonByLabel(render.container, 'Expand conversations').click()
    })

    const text = render.container.textContent ?? ''
    expect(text).toContain('Batch export')
    expect(text.indexOf('Batch export')).toBeLessThan(text.indexOf('New'))
  })

  test('marks the export action busy, disables rail actions, and prevents duplicate requests', async () => {
    const exportRequest = deferred<DownloadResult>()
    currentDownloadOutcome = {
      kind: 'pending',
      promise: exportRequest.promise,
    }

    const render = renderRail({
      isAdmin: true,
      matchMediaMatches: true,
      conversations: [
        {
          conversation_id: 'conversation-2',
          name: 'Conversation 2',
        },
      ],
      currentConversationId: 'conversation-1',
    })

    const exportButton = buttonByText(render.container, 'Batch export')
    const newButton = buttonByText(render.container, 'New')
    const batchOperationsButton = buttonByText(
      render.container,
      'Batch operations'
    )
    const conversationButton = buttonByText(render.container, 'Conversation 2')
    const renameButton = buttonByLabel(render.container, 'Rename')
    const deleteButton = buttonByLabel(render.container, 'Delete')

    await React.act(async () => {
      exportButton.click()
      exportButton.click()
      await Promise.resolve()
    })

    expect(downloadCalls).toHaveLength(1)
    expect(
      buttonByText(render.container, 'Batch export').getAttribute('aria-busy')
    ).toBe('true')
    expect(buttonByText(render.container, 'Batch export').disabled).toBe(true)
    expect(
      buttonByText(render.container, 'Batch export').getAttribute('title')
    ).toBe('Exporting Playground records...')
    expect(newButton.disabled).toBe(true)
    expect(batchOperationsButton.disabled).toBe(true)
    expect(conversationButton.disabled).toBe(true)
    expect(renameButton.disabled).toBe(true)
    expect(deleteButton.disabled).toBe(true)

    await React.act(async () => {
      exportRequest.resolve({
        blob: new Blob(['xlsx-bytes']),
        filename: 'playground-records-20260901-010203.xlsx',
      })
      await exportRequest.promise
    })

    expect(triggerCalls).toHaveLength(1)
    expect(triggerCalls[0]?.filename).toBe(
      'playground-records-20260901-010203.xlsx'
    )
    expect(triggerCalls[0]?.blob).toBeInstanceOf(Blob)
    expect(toastCalls.success).toContain('Playground records export started')
  })

  test('shows a permission toast for 403 export failures', async () => {
    currentDownloadOutcome = {
      kind: 'reject',
      error: new PlaygroundRecordExportError('Forbidden export', 403),
    }

    const render = renderRail({
      isAdmin: true,
      matchMediaMatches: true,
      conversations: [
        {
          conversation_id: 'conversation-2',
          name: 'Conversation 2',
        },
      ],
      currentConversationId: 'conversation-1',
    })

    await React.act(async () => {
      buttonByText(render.container, 'Batch export').click()
      await Promise.resolve()
    })

    expect(toastCalls.error).toContain(
      'You do not have permission to export Playground records'
    )
    expect(triggerCalls).toHaveLength(0)
  })

  test('shows the backend error message for ordinary export failures', async () => {
    currentDownloadOutcome = {
      kind: 'reject',
      error: new Error('Network exploded'),
    }

    const render = renderRail({
      isAdmin: true,
      matchMediaMatches: true,
      conversations: [
        {
          conversation_id: 'conversation-2',
          name: 'Conversation 2',
        },
      ],
      currentConversationId: 'conversation-1',
    })

    await React.act(async () => {
      buttonByText(render.container, 'Batch export').click()
      await Promise.resolve()
    })

    expect(toastCalls.error).toContain('Network exploded')
  })
})
