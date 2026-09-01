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
*/
import { useEffect, useMemo, useRef, useState } from 'react'
import {
  QueryClient,
  QueryClientProvider,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import {
  Check,
  ChevronLeft,
  ChevronRight,
  Download,
  ListChecks,
  Pencil,
  SquarePen,
  Trash2,
  X,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { useIsAdmin } from '@/hooks/use-admin'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  downloadPlaygroundRecords,
  deletePlaygroundConversations,
  listPlaygroundConversations,
  PlaygroundRecordExportError,
  renamePlaygroundConversation,
} from '../api'
import { triggerPlaygroundExport } from '../lib/playground-export'
import type { PlaygroundConversationSummary } from '../types'

interface PlaygroundConversationListProps {
  currentConversationId: string
  disabled?: boolean
  refreshKey?: number
  draftConversation?: PlaygroundConversationSummary | null
  onNew: () => void
  onSelect: (
    conversation: PlaygroundConversationSummary
  ) => Promise<void> | void
}

const conversationsQueryKey = ['playground-conversations']

function getDefaultCollapsedState() {
  if (typeof window === 'undefined' || !window.matchMedia) return true
  return !window.matchMedia('(min-width: 768px)').matches
}

function PlaygroundConversationListContent(
  props: PlaygroundConversationListProps
) {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const queryClient = useQueryClient()
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editingName, setEditingName] = useState('')
  const [isCollapsed, setIsCollapsed] = useState(getDefaultCollapsedState)
  const [isBatchMode, setIsBatchMode] = useState(false)
  const [isExporting, setIsExporting] = useState(false)
  const [loadingConversationId, setLoadingConversationId] = useState<
    string | null
  >(null)
  const [pendingDeleteIds, setPendingDeleteIds] = useState<string[] | null>(
    null
  )
  const isExportingRef = useRef(false)
  const conversationsQuery = useQuery({
    queryKey: conversationsQueryKey,
    queryFn: listPlaygroundConversations,
    refetchInterval: 10000,
  })
  useEffect(() => {
    void queryClient.invalidateQueries({ queryKey: conversationsQueryKey })
  }, [props.refreshKey, queryClient])
  const renameMutation = useMutation({
    mutationFn: ({
      conversationId,
      name,
    }: {
      conversationId: string
      name: string
    }) => renamePlaygroundConversation(conversationId, name),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: conversationsQueryKey })
      setEditingId(null)
      toast.success(t('Conversation renamed'))
    },
    onError: (error) =>
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to rename conversation')
      ),
  })
  const deleteMutation = useMutation({
    mutationFn: deletePlaygroundConversations,
    onSuccess: (_data, deletedIds) => {
      setSelectedIds(new Set())
      setIsBatchMode(false)
      void queryClient.invalidateQueries({ queryKey: conversationsQueryKey })
      toast.success(t('Conversations deleted'))
      if (deletedIds.includes(props.currentConversationId)) props.onNew()
    },
    onError: (error) =>
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to delete conversations')
      ),
  })
  const conversations = useMemo(() => {
    const loaded = conversationsQuery.data ?? []
    const draft = props.draftConversation
    if (
      !draft ||
      loaded.some((item) => item.conversation_id === draft.conversation_id)
    ) {
      return loaded
    }
    return [draft, ...loaded]
  }, [conversationsQuery.data, props.draftConversation])
  const actionsDisabled =
    props.disabled || loadingConversationId !== null || isExporting
  const allSelected = useMemo(
    () =>
      conversations.length > 0 &&
      conversations.every((item) => selectedIds.has(item.conversation_id)),
    [conversations, selectedIds]
  )

  const toggleSelected = (conversationId: string) => {
    if (actionsDisabled) return
    setSelectedIds((current) => {
      const next = new Set(current)
      if (next.has(conversationId)) next.delete(conversationId)
      else next.add(conversationId)
      return next
    })
  }

  const requestDelete = (ids: string[]) => {
    if (!actionsDisabled && ids.length > 0) setPendingDeleteIds(ids)
  }

  const confirmDelete = () => {
    if (actionsDisabled || !pendingDeleteIds) return
    deleteMutation.mutate(pendingDeleteIds)
    setPendingDeleteIds(null)
  }

  const deleteSelected = () => {
    if (actionsDisabled) return
    const ids = [...selectedIds]
    if (ids.length > 0) requestDelete(ids)
  }

  const startRename = (conversation: PlaygroundConversationSummary) => {
    if (actionsDisabled) return
    setEditingId(conversation.conversation_id)
    setEditingName(conversation.name)
  }

  const submitRename = () => {
    if (actionsDisabled) return
    const name = editingName.trim()
    if (!editingId || !name) return
    renameMutation.mutate({ conversationId: editingId, name })
  }

  const handleExport = async () => {
    if (!isAdmin || isExportingRef.current) return
    isExportingRef.current = true
    setIsExporting(true)
    try {
      const { blob, filename } = await downloadPlaygroundRecords()
      triggerPlaygroundExport(blob, filename)
      toast.success(t('Playground records export started'))
    } catch (error) {
      if (
        error instanceof PlaygroundRecordExportError &&
        error.status === 403
      ) {
        toast.error(
          t('You do not have permission to export Playground records')
        )
      } else {
        toast.error(
          error instanceof Error && error.message.trim()
            ? error.message
            : t('Failed to export Playground records')
        )
      }
    } finally {
      isExportingRef.current = false
      setIsExporting(false)
    }
  }

  const selectConversation = async (
    conversation: PlaygroundConversationSummary
  ) => {
    if (
      actionsDisabled ||
      conversation.conversation_id === props.currentConversationId
    )
      return
    setLoadingConversationId(conversation.conversation_id)
    try {
      await props.onSelect(conversation)
    } finally {
      setLoadingConversationId(null)
    }
  }

  return (
    <>
      <aside
        className={cn(
          'bg-sidebar text-sidebar-foreground border-sidebar-border relative flex h-full shrink-0 flex-col overflow-visible transition-[width] duration-200',
          isCollapsed ? 'w-0 border-r-0' : 'w-52 border-r'
        )}
      >
        <Button
          variant='ghost'
          size='icon'
          className='text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground border-sidebar-border bg-sidebar pointer-events-auto absolute top-3 left-full z-20 ml-3 size-8 rounded-full border shadow-sm'
          onClick={() => setIsCollapsed((collapsed) => !collapsed)}
          disabled={actionsDisabled}
          aria-expanded={!isCollapsed}
          aria-label={
            isCollapsed
              ? t('Expand conversations')
              : t('Collapse conversations')
          }
        >
          {isCollapsed ? (
            <ChevronRight aria-hidden='true' />
          ) : (
            <ChevronLeft aria-hidden='true' />
          )}
        </Button>
        {loadingConversationId && (
          <div
            className='bg-background text-muted-foreground absolute top-14 left-full z-20 ml-3 flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs shadow-sm'
            role='status'
            aria-live='polite'
          >
            <span className='border-primary size-3.5 animate-spin rounded-full border-2 border-t-transparent' />
            {t('Loading conversations...')}
          </div>
        )}
        {!isCollapsed && (
          <nav
            className='flex flex-col gap-0.5 px-2 pt-3'
            aria-label={t('Conversations')}
          >
            {isAdmin && (
              <Button
                type='button'
                variant='ghost'
                className='h-9 w-full justify-start rounded-md px-2.5 text-sm'
                onClick={() => void handleExport()}
                disabled={actionsDisabled}
                aria-busy={isExporting}
                title={
                  isExporting
                    ? t('Exporting Playground records...')
                    : t('Batch export')
                }
              >
                {isExporting ? (
                  <span
                    className='border-primary size-4 animate-spin rounded-full border-2 border-t-transparent'
                    aria-hidden='true'
                  />
                ) : (
                  <Download data-icon='inline-start' aria-hidden='true' />
                )}
                {isExporting
                  ? t('Exporting Playground records...')
                  : t('Batch export')}
              </Button>
            )}
            <Button
              variant='ghost'
              className='h-9 w-full justify-start rounded-md px-2.5 text-sm'
              onClick={props.onNew}
              disabled={actionsDisabled}
            >
              <SquarePen data-icon='inline-start' aria-hidden='true' />
              {t('New')}
            </Button>
            <Button
              variant='ghost'
              className='text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground h-9 w-full justify-start rounded-md px-2.5 text-sm'
              onClick={() => {
                setIsBatchMode((mode) => !mode)
                setSelectedIds(new Set())
              }}
              disabled={conversations.length === 0 || actionsDisabled}
              aria-pressed={isBatchMode}
            >
              <ListChecks data-icon='inline-start' aria-hidden='true' />
              {isBatchMode ? t('Done') : t('Batch operations')}
            </Button>
          </nav>
        )}
        {!isCollapsed && (
          <section className='border-sidebar-border mt-4 flex min-h-0 flex-1 flex-col gap-2 border-t px-2 pt-4'>
            <div className='flex items-center justify-between px-2'>
              <h2 className='text-sidebar-foreground/60 text-xs font-medium'>
                {t('Conversations')}
              </h2>
              {isBatchMode && (
                <Button
                  className='h-6 rounded-md px-2 text-xs'
                  onClick={() =>
                    setSelectedIds(
                      allSelected
                        ? new Set()
                        : new Set(
                            conversations.map(
                              (conversation) => conversation.conversation_id
                            )
                          )
                    )
                  }
                  variant='ghost'
                  disabled={actionsDisabled}
                >
                  {allSelected ? t('Deselect all') : t('Select all')}
                </Button>
              )}
            </div>
            {isBatchMode && (
              <div className='flex items-center gap-2 px-2'>
                <Button
                  className='h-7 flex-1 text-xs'
                  disabled={
                    selectedIds.size === 0 ||
                    deleteMutation.isPending ||
                    actionsDisabled
                  }
                  onClick={deleteSelected}
                  variant='destructive'
                >
                  <Trash2 data-icon='inline-start' aria-hidden='true' />
                  {t('Delete')}
                  {selectedIds.size > 0 ? ` (${selectedIds.size})` : ''}
                </Button>
              </div>
            )}
            <ScrollArea className='min-h-0 flex-1'>
              <div className='flex flex-col gap-0.5'>
                {conversationsQuery.isLoading && (
                  <p className='text-muted-foreground px-2 py-4 text-center text-xs'>
                    {t('Loading conversations...')}
                  </p>
                )}
                {!conversationsQuery.isLoading &&
                  conversations.length === 0 && (
                    <p className='text-muted-foreground px-2 py-4 text-center text-xs'>
                      {t('No conversations yet')}
                    </p>
                  )}
                {conversations.map((conversation) => {
                  const isEditing = editingId === conversation.conversation_id
                  const isActive =
                    props.currentConversationId === conversation.conversation_id
                  return (
                    <div
                      key={conversation.conversation_id}
                      className={cn(
                        'group flex min-h-9 items-center gap-2 rounded-lg px-2',
                        isActive
                          ? 'bg-sidebar-accent text-sidebar-accent-foreground'
                          : 'hover:bg-sidebar-accent/70'
                      )}
                    >
                      {isBatchMode && (
                        <Checkbox
                          checked={selectedIds.has(
                            conversation.conversation_id
                          )}
                          onCheckedChange={() =>
                            toggleSelected(conversation.conversation_id)
                          }
                          disabled={actionsDisabled}
                          aria-label={t('Select conversation')}
                        />
                      )}
                      {isEditing ? (
                        <div className='flex min-w-0 flex-1 items-center gap-1'>
                          <Input
                            autoFocus
                            value={editingName}
                            maxLength={120}
                            disabled={actionsDisabled}
                            onChange={(event) =>
                              setEditingName(event.target.value)
                            }
                            onKeyDown={(event) => {
                              if (event.key === 'Enter') submitRename()
                              if (event.key === 'Escape') setEditingId(null)
                            }}
                            className='h-7'
                          />
                          <Button
                            size='icon-xs'
                            variant='ghost'
                            onClick={submitRename}
                            disabled={
                              actionsDisabled || renameMutation.isPending
                            }
                            aria-label={t('Save')}
                          >
                            <Check aria-hidden='true' />
                          </Button>
                          <Button
                            size='icon-xs'
                            variant='ghost'
                            onClick={() => setEditingId(null)}
                            disabled={actionsDisabled}
                            aria-label={t('Cancel')}
                          >
                            <X aria-hidden='true' />
                          </Button>
                        </div>
                      ) : (
                        <button
                          type='button'
                          className='min-w-0 flex-1 truncate text-left text-sm'
                          onClick={() =>
                            isBatchMode
                              ? toggleSelected(conversation.conversation_id)
                              : void selectConversation(conversation)
                          }
                          disabled={actionsDisabled}
                        >
                          {conversation.name}
                        </button>
                      )}
                      {!isEditing && !isBatchMode && (
                        <div className='flex shrink-0 opacity-0 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100'>
                          <Button
                            size='icon-xs'
                            variant='ghost'
                            onClick={() => startRename(conversation)}
                            disabled={actionsDisabled}
                            aria-label={t('Rename')}
                          >
                            <Pencil aria-hidden='true' />
                          </Button>
                          <Button
                            size='icon-xs'
                            variant='ghost'
                            className='hover:text-destructive'
                            onClick={() =>
                              requestDelete([conversation.conversation_id])
                            }
                            disabled={actionsDisabled}
                            aria-label={t('Delete')}
                          >
                            <Trash2 aria-hidden='true' />
                          </Button>
                        </div>
                      )}
                    </div>
                  )
                })}
              </div>
            </ScrollArea>
          </section>
        )}
      </aside>
      <AlertDialog
        open={pendingDeleteIds !== null}
        onOpenChange={(open) => {
          if (!open) setPendingDeleteIds(null)
        }}
      >
        <AlertDialogContent size='sm'>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {pendingDeleteIds && pendingDeleteIds.length > 1
                ? t('Delete selected conversations?')
                : t('Delete this conversation?')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t('Deleted conversations cannot be recovered.')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              onClick={confirmDelete}
              disabled={actionsDisabled || deleteMutation.isPending}
            >
              <Trash2 data-icon='inline-start' aria-hidden='true' />
              {t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

export function PlaygroundConversationList(
  props: PlaygroundConversationListProps
) {
  const [queryClient] = useState(() => new QueryClient())
  return (
    <QueryClientProvider client={queryClient}>
      <PlaygroundConversationListContent {...props} />
    </QueryClientProvider>
  )
}
