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
import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Check, Copy, Loader2, Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import type { ApiKey } from '@/features/keys/types'
import { CreateApiKeyDialog } from './create-api-key-dialog'

function ApiKeyCopyButton(props: {
  keyId: number
  fullKey?: string
  loading: boolean
  resolveKey: (id: number) => Promise<string | null>
}) {
  const { t } = useTranslation()
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })
  const [copying, setCopying] = useState(false)
  const busy = props.loading || copying
  const copied = props.fullKey !== undefined && copiedText === props.fullKey

  const handleCopy = async () => {
    if (busy) return

    setCopying(true)
    try {
      const value = props.fullKey ?? (await props.resolveKey(props.keyId))
      if (!value) return
      await copyToClipboard(value)
    } finally {
      setCopying(false)
    }
  }

  let tooltip: string
  if (busy) {
    tooltip = t('Loading...')
  } else if (copied) {
    tooltip = t('Copied!')
  } else {
    tooltip = t('Copy API key')
  }

  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            type='button'
            variant='ghost'
            size='icon'
            className='size-6 shrink-0'
            onClick={() => void handleCopy()}
            disabled={busy}
            // Keep the accessible name stable while the value is resolving;
            // callers can still discover every row by its copy affordance.
            aria-label={t('Copy API key')}
            aria-busy={busy || undefined}
          />
        }
      >
        {busy ? (
          <Loader2 className='size-3.5 animate-spin' aria-hidden='true' />
        ) : copied ? (
          <Check className='text-success size-3.5' aria-hidden='true' />
        ) : (
          <Copy className='size-3.5' aria-hidden='true' />
        )}
      </TooltipTrigger>
      <TooltipContent>{tooltip}</TooltipContent>
    </Tooltip>
  )
}

export function ApiKeyPicker(props: {
  keys: ApiKey[]
  loading: boolean
  selectedKeyId: number | null
  resolvedKeys: Record<number, string>
  loadingKeys: Record<number, boolean>
  resolveKey: (id: number) => Promise<string | null>
  onSelect: (keyId: number) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)

  const handleCreated = async (keyId: number) => {
    await queryClient.invalidateQueries({
      queryKey: ['dashboard', 'overview', 'api-keys'],
    })
    if (keyId) props.onSelect(keyId)
  }

  return (
    <div className='bg-muted/30 flex flex-col gap-3 rounded-xl border p-4'>
      <div className='flex flex-wrap items-start justify-between gap-2'>
        <div className='flex min-w-0 flex-col gap-0.5'>
          <span className='text-sm font-semibold'>{t('API keys')}</span>
          <span className='text-muted-foreground text-xs'>
            {t(
              'Create a key to authenticate requests, or choose one of your recent keys.'
            )}
          </span>
        </div>
        <Button
          size='sm'
          className='shrink-0'
          onClick={() => setCreateOpen(true)}
        >
          <Plus data-icon='inline-start' />
          {t('Create API key')}
        </Button>
      </div>

      <CreateApiKeyDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={handleCreated}
      />

      {props.loading ? (
        <div className='flex flex-col gap-2'>
          <Skeleton className='h-11 w-full rounded-lg' />
          <Skeleton className='h-11 w-full rounded-lg' />
        </div>
      ) : props.keys.length === 0 ? (
        <p className='text-muted-foreground py-2 text-sm'>
          {t('No API key yet. Create one to get a ready-to-run example.')}
        </p>
      ) : (
        <div className='flex flex-col gap-1.5'>
          {props.keys.map((key) => {
            const selected = key.id === props.selectedKeyId
            const fullKey = props.resolvedKeys[key.id]

            return (
              <div
                key={key.id}
                className={cn(
                  'bg-background flex items-center gap-2 rounded-lg border px-3 py-2',
                  selected && 'border-primary ring-primary/30 ring-1'
                )}
              >
                {/* The copy affordance sits directly against the key it
                    copies, so the key text is sized to its content rather
                    than stretched; the name and date take the slack. Only
                    the key itself selects the row — the metadata beside it
                    is not an interactive target. */}
                <button
                  type='button'
                  onClick={() => props.onSelect(key.id)}
                  className='min-w-0 shrink truncate text-left font-mono text-xs'
                  aria-pressed={selected}
                >
                  {`sk-${key.key}`}
                </button>
                <ApiKeyCopyButton
                  keyId={key.id}
                  fullKey={fullKey}
                  loading={Boolean(props.loadingKeys[key.id])}
                  resolveKey={props.resolveKey}
                />
                <div className='ml-1 hidden min-w-0 flex-1 items-center gap-3 sm:flex'>
                  <span className='text-muted-foreground min-w-0 flex-1 truncate text-xs'>
                    {key.name}
                  </span>
                  <span className='text-muted-foreground hidden shrink-0 text-xs md:block'>
                    {formatTimestampToDate(key.created_time)}
                  </span>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
