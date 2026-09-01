# Playground Record Export UI Implementation Plan

> For agentic workers: REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

Goal: Add an administrator-only “Batch export” action above “New” in the default Playground conversation rail and download all persisted Playground records as an Excel workbook.

Architecture: Start from origin/main, which already contains the production conversation rail and the AdminAuth-protected Excel endpoint. Keep the HTTP request in the Playground API module, keep Blob/DOM download mechanics in a focused browser utility, and keep the rail responsible only for role visibility, pending state, and notifications. No backend or database changes are part of this plan.

Tech stack: React 19, TypeScript, Axios, Lucide icons, i18next, Bun test, TypeScript compiler, ESLint, Prettier.

---

## File map and ownership

| File | Responsibility |
| --- | --- |
| web/default/src/features/playground/constants.ts | Export endpoint constant. |
| web/default/src/features/playground/api.ts | Authenticated binary request, filename extraction, Blob error decoding. |
| web/default/src/features/playground/lib/playground-export.ts | Browser download trigger and delayed object-URL release. |
| web/default/src/features/playground/components/playground-conversation-list.tsx | Admin-only rail button, pending state, Toast, action disabling. |
| web/default/src/features/playground/api.test.ts | Request/response/error contract tests. |
| web/default/src/features/playground/lib/playground-export.test.ts | DOM download lifecycle tests. |
| web/default/src/features/playground/components/playground-conversation-list.test.tsx | Visibility, order, accessibility, and pending behavior tests. |
| web/default/src/i18n/locales/{en,zh,fr,ru,ja,vi,es,pt}.json | Translated button and status strings. |
| web/default/src/i18n/static-keys.ts | Registration for dynamically passed export labels. |

The implementation worktree must be based on origin/main. Unrelated dirty files in the existing worktree must remain untouched.

### Task 1: Create a clean implementation worktree

Files:
- Create: E:/workspace/_na_playground_record_export_ui/ (git worktree)

- [ ] Step 1: Verify the base contains both existing features.

Run from E:/workspace/_na_main_5d83b8b01:

~~~powershell
git show-ref --verify refs/remotes/origin/main
git cat-file -e origin/main:web/default/src/features/playground/components/playground-conversation-list.tsx
git cat-file -e origin/main:controller/playground_record_export_excel.go
~~~

Expected: all commands exit successfully.

- [ ] Step 2: Create an isolated branch from origin/main.

~~~powershell
git worktree add -b feat/playground-record-export-ui E:/workspace/_na_playground_record_export_ui origin/main
~~~

Expected: the new worktree is clean and reports feat/playground-record-export-ui.

- [ ] Step 3: Carry the reviewed design specification into the implementation branch.

~~~powershell
git -C E:/workspace/_na_playground_record_export_ui cherry-pick 7fb50c849ce023277695c0b6ab28e0f8f8ba2d4a
git -C E:/workspace/_na_playground_record_export_ui status --short
~~~

Expected: the design document is present and no unrelated files are modified.

### Task 2: Add the binary API contract (TDD)

Files:
- Modify: web/default/src/features/playground/constants.ts
- Modify: web/default/src/features/playground/api.ts
- Test: web/default/src/features/playground/api.test.ts

- [ ] Step 1: Write failing API tests.

Import downloadPlaygroundRecords and append this suite to api.test.ts:

~~~ts
describe('Playground record export API', () => {
  beforeEach(() => get.mockClear())

  test('requests an xlsx blob without GET deduplication', async () => {
    const blob = new Blob(['xlsx'], {
      type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    })
    get.mockResolvedValueOnce({
      data: blob,
      headers: {
        'content-disposition':
          'attachment; filename="playground-records-20260901T010203Z.xlsx"',
      },
    } as never)

    await expect(downloadPlaygroundRecords()).resolves.toMatchObject({
      blob,
      filename: 'playground-records-20260901T010203Z.xlsx',
    })
    expect(get).toHaveBeenCalledWith('/api/playground/records/export', {
      params: { format: 'xlsx' },
      responseType: 'blob',
      disableDuplicate: true,
      skipErrorHandler: true,
    })
  })

  test('uses a safe fallback when the filename header is absent', async () => {
    get.mockResolvedValueOnce({ data: new Blob(['xlsx']) } as never)

    await expect(downloadPlaygroundRecords()).resolves.toMatchObject({
      filename: expect.stringMatching(
        /^playground-records-\d{8}-\d{6}\.xlsx$/
      ),
    })
  })

  test('surfaces a JSON error message returned as a blob', async () => {
    const error = Object.assign(new Error('request failed'), {
      response: {
        status: 403,
        data: new Blob(
          [JSON.stringify({ message: 'administrator access required' })],
          { type: 'application/json' }
        ),
      },
    })
    get.mockRejectedValueOnce(error)

    await expect(downloadPlaygroundRecords()).rejects.toMatchObject({
      message: 'administrator access required',
      status: 403,
    })
  })
})
~~~

- [ ] Step 2: Run the focused test and confirm it fails.

~~~powershell
bun test src/features/playground/api.test.ts -t "Playground record export API"
~~~

Expected: FAIL because the helper and endpoint constant do not exist.

- [ ] Step 3: Implement the endpoint and helper.

Add PLAYGROUND_RECORDS_EXPORT with value /api/playground/records/export beside the existing
record constants. Add a typed PlaygroundRecordsExport result and PlaygroundExportError carrying
an optional HTTP status. The request must use the existing api instance with these exact options:

~~~ts
{
  params: { format: 'xlsx' },
  responseType: 'blob',
  disableDuplicate: true,
  skipErrorHandler: true,
}
~~~

Implement these rules:
1. Accept a Blob response and return it with a filename.
2. Parse content-disposition filename, accepting only a safe word/dot/dash xlsx name.
3. If the header is absent or unsafe, return a fallback named playground-records-YYYYMMDD-HHmmss.xlsx.
4. When a rejected response contains a Blob, await its text and parse a JSON message.
5. Preserve the response status on PlaygroundExportError so the rail can distinguish 403.
6. Reject a non-Blob success response with the generic export error.

- [ ] Step 4: Run the API suite and verify it passes.

~~~powershell
bun test src/features/playground/api.test.ts -t "Playground record export API"
~~~

Expected: PASS for request options, filename fallback, and Blob error decoding.

### Task 3: Isolate and test the browser download lifecycle

Files:
- Create: web/default/src/features/playground/lib/playground-export.ts
- Test: web/default/src/features/playground/lib/playground-export.test.ts

- [ ] Step 1: Write failing lifecycle tests.

Mock the document anchor, URL methods, and timer. Assert the anchor is clicked/removed and revocation
is scheduled rather than immediate:

~~~ts
test('starts a download and delays object URL revocation', () => {
  const click = mock(() => undefined)
  const remove = mock(() => undefined)
  const anchor = { href: '', download: '', rel: '', style: {}, click, remove }
  const createObjectURL = mock(() => 'blob:playground-export')
  const revokeObjectURL = mock(() => undefined)
  const setTimeout = mock((callback: () => void) => {
    callback()
    return 1
  })

  triggerPlaygroundExport(
    new Blob(['xlsx']),
    'playground-records.xlsx',
    { createObjectURL, revokeObjectURL, setTimeout },
    anchor as never
  )

  expect(anchor.href).toBe('blob:playground-export')
  expect(anchor.download).toBe('playground-records.xlsx')
  expect(click).toHaveBeenCalledTimes(1)
  expect(remove).toHaveBeenCalledTimes(1)
  expect(setTimeout).toHaveBeenCalledTimes(1)
  expect(revokeObjectURL).toHaveBeenCalledWith('blob:playground-export')
})
~~~

- [ ] Step 2: Run the new test and confirm it fails.

~~~powershell
bun test src/features/playground/lib/playground-export.test.ts
~~~

Expected: FAIL because triggerPlaygroundExport is not defined.

- [ ] Step 3: Implement the utility.

Use an injected browser surface and a 60-second default release delay:

~~~ts
export interface PlaygroundExportBrowser {
  createObjectURL: (blob: Blob) => string
  revokeObjectURL: (url: string) => void
  setTimeout: (callback: () => void, delayMs: number) => number
}

const DEFAULT_REVOKE_DELAY_MS = 60_000

export function triggerPlaygroundExport(
  blob: Blob,
  filename: string,
  browser: PlaygroundExportBrowser = {
    createObjectURL: (value) => URL.createObjectURL(value),
    revokeObjectURL: (value) => URL.revokeObjectURL(value),
    setTimeout: (callback, delayMs) => window.setTimeout(callback, delayMs),
  },
  anchor = document.createElement('a')
): void {
  const objectUrl = browser.createObjectURL(blob)
  anchor.href = objectUrl
  anchor.download = filename
  anchor.rel = 'noopener'
  anchor.style.display = 'none'
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  browser.setTimeout(
    () => browser.revokeObjectURL(objectUrl),
    DEFAULT_REVOKE_DELAY_MS
  )
}
~~~

Do not revoke the URL immediately after click; this is the regression guard for the reported Chrome
resource error.

- [ ] Step 4: Run the lifecycle test and verify it passes.

~~~powershell
bun test src/features/playground/lib/playground-export.test.ts
~~~

Expected: PASS, including delayed revocation.

### Task 4: Add the administrator-only rail action

Files:
- Modify: web/default/src/features/playground/components/playground-conversation-list.tsx
- Create or modify: web/default/src/features/playground/components/playground-conversation-list.test.tsx

- [ ] Step 1: Write failing visibility and pending-state tests.

Mock useIsAdmin, downloadPlaygroundRecords, and triggerPlaygroundExport. Use the existing i18next
provider and createRoot/act style. Assert admin order, non-admin omission, aria-busy, and one request:

~~~tsx
test('renders Batch export above New only for administrators', () => {
  mockIsAdmin.mockReturnValue(true)
  const html = renderRail()
  expect(html.indexOf('Batch export')).toBeGreaterThanOrEqual(0)
  expect(html.indexOf('Batch export')).toBeLessThan(html.indexOf('New'))

  mockIsAdmin.mockReturnValue(false)
  expect(renderRail()).not.toContain('Batch export')
})

test('marks the export action busy while the request is pending', async () => {
  mockIsAdmin.mockReturnValue(true)
  const pending = deferred<PlaygroundRecordsExport>()
  mockDownloadPlaygroundRecords.mockReturnValue(pending.promise)
  const { button } = renderRailIntoDom()

  await act(async () => button.click())
  expect(button.getAttribute('aria-busy')).toBe('true')
  expect(button.hasAttribute('disabled')).toBe(true)
  expect(mockDownloadPlaygroundRecords).toHaveBeenCalledTimes(1)

  pending.resolve({ blob: new Blob(['xlsx']), filename: 'records.xlsx' })
  await act(async () => pending.promise)
  expect(mockTriggerPlaygroundExport).toHaveBeenCalledWith(
    expect.any(Blob),
    'records.xlsx'
  )
})
~~~

- [ ] Step 2: Run the component test and confirm it fails.

~~~powershell
bun test src/features/playground/components/playground-conversation-list.test.tsx
~~~

Expected: FAIL because the rail has no export action.

- [ ] Step 3: Implement the minimal rail integration.

Import useIsAdmin, Download, downloadPlaygroundRecords, PlaygroundExportError, and
triggerPlaygroundExport. Add local state and handler:

~~~tsx
const isAdmin = useIsAdmin()
const [isExporting, setIsExporting] = useState(false)
const actionsDisabled =
  props.disabled || loadingConversationId !== null || isExporting

const handleExport = async () => {
  if (isExporting) return
  setIsExporting(true)
  try {
    const result = await downloadPlaygroundRecords()
    triggerPlaygroundExport(result.blob, result.filename)
    toast.success(t('Playground records export started'))
  } catch (error) {
    if (error instanceof PlaygroundExportError && error.status === 403) {
      toast.error(t('You do not have permission to export Playground records'))
    } else {
      toast.error(
        error instanceof Error && error.message.trim()
          ? error.message
          : t('Failed to export Playground records')
      )
    }
  } finally {
    setIsExporting(false)
  }
}
~~~

Render this block immediately before the existing New button and only in the expanded nav:

~~~tsx
{isAdmin && (
  <Button
    type='button'
    variant='ghost'
    className='h-9 w-full justify-start rounded-md px-2.5 text-sm'
    onClick={() => void handleExport()}
    disabled={actionsDisabled}
    aria-busy={isExporting}
  >
    {isExporting ? (
      <span
        className='border-primary size-4 animate-spin rounded-full border-2 border-t-transparent'
        aria-hidden='true'
      />
    ) : (
      <Download data-icon='inline-start' aria-hidden='true' />
    )}
    {isExporting ? t('Exporting Playground records...') : t('Batch export')}
  </Button>
)}
~~~

Use actionsDisabled for New, Batch operations, conversation selection, rename, and delete while
the request is pending. Keep collapsed/mobile behavior unchanged.

- [ ] Step 4: Run component tests and verify they pass.

~~~powershell
bun test src/features/playground/components/playground-conversation-list.test.tsx
~~~

Expected: PASS for admin visibility, ordering, pending state, and single-request protection.

### Task 5: Add translated strings

Files:
- Modify: web/default/src/i18n/locales/en.json
- Modify: web/default/src/i18n/locales/zh.json
- Modify: web/default/src/i18n/locales/fr.json
- Modify: web/default/src/i18n/locales/ru.json
- Modify: web/default/src/i18n/locales/ja.json
- Modify: web/default/src/i18n/locales/vi.json
- Modify: web/default/src/i18n/locales/es.json
- Modify: web/default/src/i18n/locales/pt.json
- Modify: web/default/src/i18n/static-keys.ts

- [ ] Step 1: Add the exact five keys to every locale.

Use this concrete translation table; keep the English values unchanged in en:

| Key | en | zh | fr | ru | ja | vi | es | pt |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Batch export | Batch export | 批量导出 | Exportation groupée | Пакетный экспорт | 一括エクスポート | Xuất hàng loạt | Exportación por lotes | Exportação em lote |
| Exporting Playground records... | Exporting Playground records... | 正在导出 Playground 记录... | Exportation des enregistrements Playground... | Экспорт записей Playground... | Playground の記録をエクスポート中… | Đang xuất bản ghi Playground... | Exportando registros de Playground... | Exportando registros do Playground... |
| Playground records export started | Playground records export started | Playground 记录导出已开始 | L’exportation des enregistrements Playground a commencé | Экспорт записей Playground начат | Playground の記録のエクスポートを開始しました | Đã bắt đầu xuất bản ghi Playground | La exportación de registros de Playground ha comenzado | A exportação dos registros do Playground foi iniciada |
| Failed to export Playground records | Failed to export Playground records | Playground 记录导出失败 | Échec de l’exportation des enregistrements Playground | Не удалось экспортировать записи Playground | Playground の記録をエクスポートできませんでした | Không thể xuất bản ghi Playground | No se pudieron exportar los registros de Playground | Falha ao exportar os registros do Playground |
| You do not have permission to export Playground records | You do not have permission to export Playground records | 你没有导出 Playground 记录的权限 | Vous n’avez pas l’autorisation d’exporter les enregistrements Playground | У вас нет разрешения на экспорт записей Playground | Playground の記録をエクスポートする権限がありません | Bạn không có quyền xuất bản ghi Playground | No tienes permiso para exportar los registros de Playground | Você não tem permissão para exportar os registros do Playground |
~~~

Register the five keys in static-keys.ts because the component passes them through a conditional
expression; do not add filter-related strings.

- [ ] Step 2: Run locale checks.

~~~powershell
bun run i18n:status
bun run i18n:lint
~~~

Expected: all supported locales parse and contain the five keys.

### Task 6: Run the smallest complete verification set

Files:
- Verify only files listed above; do not edit generated dist assets.

- [ ] Step 1: Run focused tests.

From E:/workspace/_na_playground_record_export_ui/web/default:

~~~powershell
bun test src/features/playground/api.test.ts src/features/playground/lib/playground-export.test.ts src/features/playground/components/playground-conversation-list.test.tsx
~~~

Expected: PASS with zero failures.

- [ ] Step 2: Run type checking.

~~~powershell
bun run typecheck
~~~

Expected: TypeScript exits 0. Do not run a production build unless type checking exposes a bundler-only issue.

- [ ] Step 3: Run targeted lint, formatting, and whitespace checks.

~~~powershell
bunx eslint src/features/playground/api.ts src/features/playground/api.test.ts src/features/playground/constants.ts src/features/playground/lib/playground-export.ts src/features/playground/lib/playground-export.test.ts src/features/playground/components/playground-conversation-list.tsx src/features/playground/components/playground-conversation-list.test.tsx
bunx prettier --check src/features/playground/api.ts src/features/playground/api.test.ts src/features/playground/constants.ts src/features/playground/lib/playground-export.ts src/features/playground/lib/playground-export.test.ts src/features/playground/components/playground-conversation-list.tsx src/features/playground/components/playground-conversation-list.test.tsx
git diff --check
~~~

Expected: all commands exit 0.

### Task 7: Commit the implementation

Files: all implementation, test, locale, spec, and plan files in this plan.

- [ ] Step 1: Review the diff.

~~~powershell
git status --short
git diff --stat origin/main...HEAD
git diff --check
~~~

Expected: only the reviewed Playground export UI files are listed.

- [ ] Step 2: Commit with Lore trailers.

Use an intent-focused message and include Constraint, Rejected, Confidence, Scope-risk, Directive, Tested, and
Not-tested trailers. Record the focused test, typecheck, targeted lint, Prettier, and diff-check results
in Tested; record the absence of a production build and authenticated production browser click in
Not-tested.
