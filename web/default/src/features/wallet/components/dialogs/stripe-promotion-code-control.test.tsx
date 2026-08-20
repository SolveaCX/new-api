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
import { describe, expect, mock, test } from 'bun:test'
import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider } from 'react-i18next'

mock.restore()
mock.module('lucide-react', () => ({
  Loader2: () => null,
  Tag: () => null,
  X: () => null,
}))

const { StripePromotionCodeControl } = await import(
  './stripe-promotion-code-control.tsx'
)

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        Apply: 'Apply',
        'Applying promotion code...': 'Applying promotion code...',
        'Enter promotion code': 'Enter promotion code',
        'Promotion code': 'Promotion code',
        'Remove promotion code': 'Remove promotion code',
        'Restoring previous discount...': 'Restoring previous discount...',
      },
    },
  },
})

function renderPromotionControl(
  busyAction: 'apply' | 'restore',
  value = 'SAVE30'
) {
  return renderToStaticMarkup(
    <I18nextProvider i18n={i18n}>
      <StripePromotionCodeControl
        value={value}
        discountState={{
          source: 'manual',
          display_name: 'SAVE20',
          promotion_code_masked: 'SAVE20',
        }}
        busy
        busyAction={busyAction}
        message={null}
        onValueChange={() => undefined}
        onApply={() => undefined}
        onRemove={() => undefined}
      />
    </I18nextProvider>
  )
}

describe('StripePromotionCodeControl', () => {
  test('shows apply loading copy while replacing an existing manual promotion code', () => {
    const html = renderPromotionControl('apply')

    expect(html).toContain('Applying promotion code...')
    expect(html).not.toContain('Restoring previous discount...')
  })

  test('shows restore loading copy only while restoring the previous discount', () => {
    const html = renderPromotionControl('restore')

    expect(html).toContain('Restoring previous discount...')
    expect(html).not.toContain('Applying promotion code...')
  })
})
