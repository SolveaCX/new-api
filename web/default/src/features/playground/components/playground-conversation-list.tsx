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
import { useEffect, useMemo, useState } from 'react'
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
  Pencil,
  Plus,
  Trash2,
  X,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
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
  deletePlaygroundConversations,
  listPlaygroundConversations,
  renamePlaygroundConversation,
} from '../api'
import type { PlaygroundConversationSummary } from '../types'

interface PlaygroundConversationListProps {
  currentConversationId: string
  disabled?: boolean
  refreshKey?: number
  draftConversation?: PlaygroundConversationSummary | null
  onNew: () => void
  onSelect: (conversation: PlaygroundConversationSummary) => void
}

const conversationsQueryKey = ['playground-conversations']

function PlaygroundConversationListContent(
  props: PlaygroundConversationListProps
) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editingName, setEditingName] = useState('')
  const [isCollapsed, setIsCollapsed] = useState(false)
  const [pendingDeleteIds, setPendingDeleteIds] = useState<string[] | null>(
    null
  )
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
  const allSelected = useMemo(
    () =>
      conversations.length > 0 &&
      conversations.every((item) => selectedIds.has(item.conversation_id)),
    [conversations, selectedIds]
  )

  const toggleSelected = (conversationId: string) => {
    setSelectedIds((current) => {
      const next = new Set(current)
      if (next.has(conversationId)) next.delete(conversationId)
      else next.add(conversationId)
      return next
    })
  }

  const requestDelete = (ids: string[]) => {
    if (ids.length > 0) setPendingDeleteIds(ids)
  }

  const confirmDelete = () => {
    if (!pendingDeleteIds) return
    deleteMutation.mutate(pendingDeleteIds)
    setPendingDeleteIds(null)
  }

  const deleteSelected = () => {
    const ids = [...selectedIds]
    requestDelete(ids)
  }

  const startRename = (conversation: PlaygroundConversationSummary) => {
    setEditingId(conversation.conversation_id)
    setEditingName(conversation.name)
  }

  const submitRename = () => {
    const name = editingName.trim()
    if (!editingId || !name) return
    renameMutation.mutate({ conversationId: editingId, name })
  }

  return (
    <>
      <aside
        className={cn(
          'bg-background flex h-full shrink-0 flex-col border-r shadow-sm transition-[width] duration-200',
          isCollapsed ? 'w-14' : 'w-72'
        )}
      >
        <div
          className={cn(
            'bg-muted/20 flex h-16 items-center gap-2 border-b px-3',
            isCollapsed ? 'justify-center' : 'justify-between'
          )}
        >
          {!isCollapsed && (
            <div className='min-w-0'>
              <h2 className='truncate text-base font-semibold'>
                {t('Conversations')}
              </h2>
              <p className='text-muted-foreground text-xs'>
                {conversations.length} {t('Conversations').toLowerCase()}
              </p>
            </div>
          )}
          <Button
            variant='ghost'
            size='icon'
            className='rounded-lg'
            onClick={() => setIsCollapsed((collapsed) => !collapsed)}
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
        </div>
        {isCollapsed ? (
          <div className='flex flex-1 flex-col items-center gap-2 p-2'>
            <Button
              variant='outline'
              size='icon'
              className='rounded-lg'
              onClick={props.onNew}
              disabled={props.disabled}
              aria-label={t('New')}
            >
              <Plus aria-hidden='true' />
            </Button>
          </div>
        ) : (
          <div className='flex min-h-0 flex-1 flex-col gap-3 p-3'>
            <Button
              variant='default'
              className='w-full justify-start rounded-lg'
              onClick={props.onNew}
              disabled={props.disabled}
            >
              <Plus data-icon='inline-start' aria-hidden='true' />
              {t('New')}
            </Button>
            <div className='bg-muted/40 flex items-center justify-between rounded-lg border px-2.5 py-1.5'>
              <label className='text-muted-foreground flex items-center gap-2 text-xs'>
                <Checkbox
                  checked={allSelected}
                  onCheckedChange={(checked) =>
                    setSelectedIds(
                      checked
                        ? new Set(
                            conversations.map((item) => item.conversation_id)
                          )
                        : new Set()
                    )
                  }
                  aria-label={t('Select all')}
                />
                {t('Select all')}
              </label>
              <Button
                variant='ghost'
                size='icon-xs'
                className='text-muted-foreground hover:text-destructive rounded-md'
                onClick={deleteSelected}
                disabled={selectedIds.size === 0 || deleteMutation.isPending}
                aria-label={t('Delete selected conversations')}
              >
                <Trash2 aria-hidden='true' />
              </Button>
            </div>
            <ScrollArea className='min-h-0 flex-1'>
              <div className='flex flex-col gap-1.5'>
                {conversationsQuery.isLoading && (
                  <p className='text-muted-foreground px-2 py-4 text-center text-xs'>
                    {t('Loading conversations...')}
                  </p>
                )}
                {!conversationsQuery.isLoading &&
                  conversations.length === 0 && (
                    <p className='text-muted-foreground px-2 py-8 text-center text-xs'>
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
                        'group flex items-start gap-2 rounded-xl border px-2.5 py-2.5 transition-colors',
                        isActive
                          ? 'border-primary/20 bg-primary/10 shadow-sm'
                          : 'hover:border-border hover:bg-muted/50 border-transparent'
                      )}
                    >
                      <Checkbox
                        checked={selectedIds.has(conversation.conversation_id)}
                        onCheckedChange={() =>
                          toggleSelected(conversation.conversation_id)
                        }
                        aria-label={t('Select conversation')}
                        className='mt-1'
                      />
                      {isEditing ? (
                        <div className='flex min-w-0 flex-1 items-center gap-1'>
                          <Input
                            autoFocus
                            value={editingName}
                            maxLength={120}
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
                            className='rounded-md'
                            onClick={submitRename}
                            disabled={renameMutation.isPending}
                            aria-label={t('Save')}
                          >
                            <Check aria-hidden='true' />
                          </Button>
                          <Button
                            size='icon-xs'
                            variant='ghost'
                            className='rounded-md'
                            onClick={() => setEditingId(null)}
                            aria-label={t('Cancel')}
                          >
                            <X aria-hidden='true' />
                          </Button>
                        </div>
                      ) : (
                        <button
                          type='button'
                          className='min-w-0 flex-1 text-left'
                          onClick={() => props.onSelect(conversation)}
                          disabled={props.disabled}
                        >
                          <span className='block truncate text-sm font-medium'>
                            {conversation.name}
                          </span>
                          <span className='text-muted-foreground block truncate text-xs'>
                            {conversation.preview}
                          </span>
                        </button>
                      )}
                      {!isEditing && (
                        <div className='flex shrink-0 opacity-100 transition-opacity md:opacity-0 md:group-focus-within:opacity-100 md:group-hover:opacity-100'>
                          <Button
                            size='icon-xs'
                            variant='ghost'
                            className='rounded-md'
                            onClick={() => startRename(conversation)}
                            aria-label={t('Rename')}
                          >
                            <Pencil aria-hidden='true' />
                          </Button>
                          <Button
                            size='icon-xs'
                            variant='ghost'
                            className='hover:text-destructive rounded-md'
                            onClick={() => {
                              requestDelete([conversation.conversation_id])
                            }}
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
          </div>
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
              disabled={deleteMutation.isPending}
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
