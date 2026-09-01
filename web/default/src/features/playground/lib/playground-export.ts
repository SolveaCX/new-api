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
export interface PlaygroundExportBrowser {
  createObjectURL: (blob: Blob) => string
  revokeObjectURL: (url: string) => void
  setTimeout: (callback: () => void, delayMs: number) => number
}

interface PlaygroundExportAnchor {
  href: string
  download: string
  rel: string
  style: {
    display: string
  }
  click: () => void
  remove: () => void
}

const DEFAULT_REVOKE_DELAY_MS = 60_000

function createDefaultBrowser(): PlaygroundExportBrowser {
  return {
    createObjectURL: (blob) => globalThis.URL.createObjectURL(blob),
    revokeObjectURL: (url) => globalThis.URL.revokeObjectURL(url),
    setTimeout: (callback, delayMs) => globalThis.setTimeout(callback, delayMs),
  }
}

function createDefaultAnchor(): PlaygroundExportAnchor {
  const doc = globalThis.document
  if (!doc?.createElement) {
    throw new Error('Playground export requires a document')
  }

  return doc.createElement('a') as PlaygroundExportAnchor
}

export function triggerPlaygroundExport(
  blob: Blob,
  filename: string,
  browser: PlaygroundExportBrowser = createDefaultBrowser(),
  anchor: PlaygroundExportAnchor = createDefaultAnchor()
): void {
  const objectUrl = browser.createObjectURL(blob)
  anchor.href = objectUrl
  anchor.download = filename
  anchor.rel = 'noopener'
  anchor.style.display = 'none'

  const doc = globalThis.document
  try {
    doc?.body?.appendChild(anchor as unknown as Node)
    anchor.click()
  } finally {
    anchor.remove()
    browser.setTimeout(() => {
      browser.revokeObjectURL(objectUrl)
    }, DEFAULT_REVOKE_DELAY_MS)
  }
}

export const triggerPlaygroundRecordsDownload = triggerPlaygroundExport
