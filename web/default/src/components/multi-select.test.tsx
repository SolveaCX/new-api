import * as React from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { beforeAll, beforeEach, describe, expect, mock, test } from 'bun:test'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'

type ComboboxRootProps = {
  children: React.ReactNode
  inputValue: string
  items: string[]
  onInputValueChange: (value: string) => void
  onValueChange: (value: string[]) => void
  value: string[]
}

let latestComboboxProps: ComboboxRootProps | undefined
let latestInputChange: ((value: string) => void) | undefined
let selectOption: ((value: string) => void) | undefined

function setupDom() {
  if (typeof document !== 'undefined') {
    globalThis.IS_REACT_ACT_ENVIRONMENT = true
    return
  }

  class NodeShim {
    childNodes: NodeShim[] = []
    nodeType = 0
    nodeName = ''
    parentNode: NodeShim | null = null
    ownerDocument = globalThis.document

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

    addEventListener() {}
    removeEventListener() {}
  }

  class ElementShim extends NodeShim {
    attributes: Record<string, string> = {}
    localName: string
    namespaceURI = 'http://www.w3.org/1999/xhtml'
    style = {}
    tagName: string
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
    }

    removeAttribute(key: string) {
      delete this.attributes[key]
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
  const shimDocument = {
    nodeType: 9,
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
  }
  globalThis.document = shimDocument as unknown as Document
  globalThis.window = globalThis as unknown as Window & typeof globalThis
  globalThis.HTMLElement = ElementShim as unknown as typeof HTMLElement
  globalThis.HTMLIFrameElement = class {} as typeof HTMLIFrameElement
  globalThis.Node = NodeShim as unknown as typeof Node
  globalThis.IS_REACT_ACT_ENVIRONMENT = true
}

setupDom()

mock.module('@/components/ui/combobox', () => ({
  Combobox: (props: ComboboxRootProps) => {
    latestComboboxProps = props
    return <div>{props.children}</div>
  },
  ComboboxChip: ({ children }: { children: React.ReactNode }) => (
    <span>{children}</span>
  ),
  ComboboxChips: React.forwardRef<
    HTMLDivElement,
    { children: React.ReactNode }
  >(({ children }, _ref) => <div>{children}</div>),
  ComboboxChipsInput: (props: { disabled?: boolean; id?: string }) => {
    latestInputChange = (value: string) =>
      latestComboboxProps?.onInputValueChange(value)
    return <input id={props.id} disabled={props.disabled} />
  },
  ComboboxCollection: ({
    children,
  }: {
    children: (value: string) => React.ReactNode
  }) => <>{latestComboboxProps?.items.map(children)}</>,
  ComboboxContent: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  ComboboxEmpty: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  ComboboxItem: ({
    children,
    value,
  }: {
    children: React.ReactNode
    value: string
  }) => {
    selectOption = (nextValue: string) =>
      latestComboboxProps?.onValueChange([
        ...(latestComboboxProps.value ?? []),
        nextValue,
      ])
    return <button value={value}>{children}</button>
  },
  ComboboxList: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  ComboboxValue: ({
    children,
  }: {
    children: (value: string[]) => React.ReactNode
  }) => <>{children(latestComboboxProps?.value ?? [])}</>,
  useComboboxAnchor: () => React.createRef<HTMLDivElement>(),
}))

const { MultiSelect } = await import('./multi-select')
const testI18n = createInstance()

beforeAll(async () => {
  await testI18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })
})

beforeEach(() => {
  latestComboboxProps = undefined
  latestInputChange = undefined
  selectOption = undefined
})

function renderMultiSelect(props: React.ComponentProps<typeof MultiSelect>) {
  const container = document.createElement('div')
  const root = createRoot(container)
  React.act(() => {
    root.render(
      <I18nextProvider i18n={testI18n}>
        <MultiSelect {...props} />
      </I18nextProvider>
    )
  })
  return root
}

function wait(ms = 0) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

async function waitFor(predicate: () => boolean, timeout = 1000) {
  const startedAt = Date.now()
  while (!predicate()) {
    if (Date.now() - startedAt > timeout) {
      throw new Error('Timed out waiting for assertion')
    }
    await wait(10)
  }
}

function dispose(root: Root) {
  React.act(() => {
    root.unmount()
  })
}

describe('MultiSelect search notifications', () => {
  test('notifies when the combobox input value changes', async () => {
    const searches: string[] = []
    const root = renderMultiSelect({
      options: [{ label: 'Ada Lovelace', value: '1' }],
      selected: [],
      onChange: () => undefined,
      onSearchChange: (value) => searches.push(value),
      placeholder: 'Search users',
    })

    await waitFor(() => latestInputChange !== undefined)
    React.act(() => {
      latestInputChange?.('ada')
    })

    expect(searches).toEqual(['ada'])
    dispose(root)
  })

  test('notifies with an empty search after selecting an option', async () => {
    const searches: string[] = []
    const selected: string[][] = []
    const root = renderMultiSelect({
      options: [{ label: 'Ada Lovelace', value: '1' }],
      selected: [],
      onChange: (values) => selected.push(values),
      onSearchChange: (value) => searches.push(value),
      placeholder: 'Search users',
    })

    await waitFor(
      () => latestInputChange !== undefined && selectOption !== undefined
    )
    React.act(() => {
      latestInputChange?.('ada')
      selectOption?.('1')
    })

    expect(selected).toEqual([['1']])
    expect(searches.at(-1)).toBe('')
    dispose(root)
  })
})
