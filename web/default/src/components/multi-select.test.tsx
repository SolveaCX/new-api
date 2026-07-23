import * as React from 'react'
import { beforeAll, beforeEach, describe, expect, mock, test } from 'bun:test'
import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
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
  renderToStaticMarkup(
    <I18nextProvider i18n={testI18n}>
      <MultiSelect {...props} />
    </I18nextProvider>
  )
}

describe('MultiSelect search notifications', () => {
  test('notifies when the combobox input value changes', () => {
    const searches: string[] = []
    renderMultiSelect({
      options: [{ label: 'Ada Lovelace', value: '1' }],
      selected: [],
      onChange: () => undefined,
      onSearchChange: (value) => searches.push(value),
      placeholder: 'Search users',
    })

    latestInputChange?.('ada')

    expect(searches).toEqual(['ada'])
  })

  test('notifies with an empty search after selecting an option', () => {
    const searches: string[] = []
    const selected: string[][] = []
    renderMultiSelect({
      options: [{ label: 'Ada Lovelace', value: '1' }],
      selected: [],
      onChange: (values) => selected.push(values),
      onSearchChange: (value) => searches.push(value),
      placeholder: 'Search users',
    })

    latestInputChange?.('ada')
    selectOption?.('1')

    expect(selected).toEqual([['1']])
    expect(searches.at(-1)).toBe('')
  })
})
