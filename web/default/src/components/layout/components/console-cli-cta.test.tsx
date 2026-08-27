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
import { renderToStaticMarkup } from 'react-dom/server'
import { ConsoleCliCta } from './console-cli-cta'

describe('ConsoleCliCta', () => {
  test('opens the official CLI landing page with responsive labels', () => {
    const html = renderToStaticMarkup(<ConsoleCliCta />)

    expect(html).toContain('href="/cli"')
    expect(html).toContain('target="_blank"')
    expect(html).toContain('noopener')
    expect(html).toContain('noreferrer')
    expect(html).toContain('Flatkey CLI')
    expect(html).toContain('sm:hidden')
    expect(html).toContain('aria-hidden="true"')
  })

  test('keeps the brand gradient high contrast through hover', () => {
    const html = renderToStaticMarkup(<ConsoleCliCta />)

    expect(html).toContain('from-violet-700')
    expect(html).toContain('to-fuchsia-700')
    expect(html).toContain('hover:from-violet-600')
    expect(html).toContain('hover:to-fuchsia-600')
  })
})
