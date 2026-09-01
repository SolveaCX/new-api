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
import { afterEach, describe, expect, mock, test } from 'bun:test'
import { triggerPlaygroundExport } from './playground-export'

describe('triggerPlaygroundExport', () => {
  const originalDocument = globalThis.document

  afterEach(() => {
    if (originalDocument === undefined) {
      delete (globalThis as { document?: Document }).document
    } else {
      globalThis.document = originalDocument
    }
    mock.restore()
  })

  test('starts a download and delays object URL revocation', () => {
    const blob = new Blob(['xlsx-bytes'], {
      type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    })
    const click = mock(() => undefined)
    const remove = mock(() => undefined)
    const anchor = {
      href: '',
      download: '',
      rel: '',
      style: { display: '' },
      click,
      remove,
    }
    const createObjectURL = mock(() => 'blob:playground-export')
    const revokeObjectURL = mock(() => undefined)
    let scheduledCallback: (() => void) | null = null
    const setTimeout = mock((callback: () => void, delayMs: number) => {
      expect(delayMs).toBe(60_000)
      scheduledCallback = callback
      return 1
    })
    const appendChild = mock(() => anchor as never)
    Object.defineProperty(globalThis, 'document', {
      configurable: true,
      value: {
        body: {
          appendChild,
        },
      },
    })

    triggerPlaygroundExport(
      blob,
      'playground-records.xlsx',
      {
        createObjectURL,
        revokeObjectURL,
        setTimeout,
      },
      anchor
    )

    expect(createObjectURL).toHaveBeenCalledWith(blob)
    expect(appendChild).toHaveBeenCalledWith(anchor as never)
    expect(click).toHaveBeenCalledTimes(1)
    expect(remove).toHaveBeenCalledTimes(1)
    expect(revokeObjectURL).not.toHaveBeenCalled()
    expect(anchor.href).toBe('blob:playground-export')
    expect(anchor.download).toBe('playground-records.xlsx')
    expect(anchor.rel).toBe('noopener')
    expect(anchor.style.display).toBe('none')

    scheduledCallback?.()
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:playground-export')
  })

  test('bubbles object URL creation failures without touching the anchor', () => {
    const click = mock(() => undefined)
    const remove = mock(() => undefined)
    const anchor = {
      href: '',
      download: '',
      rel: '',
      style: { display: '' },
      click,
      remove,
    }
    const createObjectURL = mock(() => {
      throw new Error('boom')
    })
    const revokeObjectURL = mock(() => undefined)
    const setTimeout = mock(() => 1)
    const appendChild = mock(() => anchor as never)
    Object.defineProperty(globalThis, 'document', {
      configurable: true,
      value: {
        body: {
          appendChild,
        },
      },
    })

    expect(() =>
      triggerPlaygroundExport(
        new Blob(['xlsx-bytes']),
        'playground-records.xlsx',
        {
          createObjectURL,
          revokeObjectURL,
          setTimeout,
        },
        anchor
      )
    ).toThrow('boom')

    expect(appendChild).not.toHaveBeenCalled()
    expect(click).not.toHaveBeenCalled()
    expect(remove).not.toHaveBeenCalled()
    expect(revokeObjectURL).not.toHaveBeenCalled()
    expect(setTimeout).not.toHaveBeenCalled()
  })

  test('still schedules delayed revocation when appending the anchor fails', () => {
    const remove = mock(() => undefined)
    const anchor = {
      href: '',
      download: '',
      rel: '',
      style: { display: '' },
      click: mock(() => undefined),
      remove,
    }
    const createObjectURL = mock(() => 'blob:playground-export')
    const revokeObjectURL = mock(() => undefined)
    const setTimeout = mock(() => 1)
    const appendChild = mock(() => {
      throw new Error('append failed')
    })
    Object.defineProperty(globalThis, 'document', {
      configurable: true,
      value: {
        body: {
          appendChild,
        },
      },
    })

    expect(() =>
      triggerPlaygroundExport(
        new Blob(['xlsx-bytes']),
        'playground-records.xlsx',
        {
          createObjectURL,
          revokeObjectURL,
          setTimeout,
        },
        anchor
      )
    ).toThrow('append failed')

    expect(createObjectURL).toHaveBeenCalledTimes(1)
    expect(appendChild).toHaveBeenCalledWith(anchor as never)
    expect(remove).toHaveBeenCalledTimes(1)
    expect(setTimeout).toHaveBeenCalledTimes(1)
    expect(revokeObjectURL).not.toHaveBeenCalled()
  })
})
