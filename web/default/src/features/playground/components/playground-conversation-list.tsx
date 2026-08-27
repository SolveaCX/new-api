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
  ListChecks,
  Pencil,
  SquarePen,
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
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
  const [isManageOpen, setIsManageOpen] = useState(false)
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
      setIsManageOpen(false)
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
    if (ids.length === 0) return
    deleteMutation.mutate(ids)
  }

  const requestDeleteCurrent = () => {
    const isSaved = conversations.some(
      (conversation) =>
        conversation.conversation_id === props.currentConversationId
    )
    if (!isSaved) return
    requestDelete([props.currentConversationId])
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
          'bg-background border-border flex h-full shrink-0 flex-col border-r transition-[width] duration-200',
          isCollapsed ? 'w-10' : 'w-52'
        )}
      >
        <div
          className={cn(
            'flex h-14 items-center px-2',
            isCollapsed ? 'justify-center' : 'justify-end'
          )}
        >
          <Button
            variant='ghost'
            size='icon'
            className='text-muted-foreground hover:bg-muted rounded-md'
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
        {!isCollapsed && (
          <nav
            className='flex flex-col gap-0.5 px-2'
            aria-label={t('Conversations')}
          >
            <Button
              variant='ghost'
              className='h-9 w-full justify-start rounded-md px-2.5 text-sm'
              onClick={props.onNew}
              disabled={props.disabled}
            >
              <SquarePen data-icon='inline-start' aria-hidden='true' />
              {t('New')}
            </Button>
            <Button
              variant='ghost'
              className='text-muted-foreground hover:bg-muted hover:text-destructive h-9 w-full justify-start rounded-md px-2.5 text-sm'
              onClick={requestDeleteCurrent}
              disabled={
                deleteMutation.isPending ||
                !conversations.some(
                  (conversation) =>
                    conversation.conversation_id === props.currentConversationId
                )
              }
            >
              <Trash2 data-icon='inline-start' aria-hidden='true' />
              {t('Delete')}
            </Button>
            <Button
              variant='ghost'
              className='text-muted-foreground hover:bg-muted h-9 w-full justify-start rounded-md px-2.5 text-sm'
              onClick={() => setIsManageOpen(true)}
              disabled={conversations.length === 0}
            >
              <ListChecks data-icon='inline-start' aria-hidden='true' />
              {t('Delete selected conversations')}
            </Button>
          </nav>
        )}
        {!isCollapsed && (
          <section className='border-border/70 mt-4 flex min-h-0 flex-1 flex-col gap-2 border-t px-2 pt-4'>
            <h2 className='text-muted-foreground px-2 text-xs font-medium'>
              {t('Conversations')}
            </h2>
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
                          ? 'bg-muted text-foreground'
                          : 'hover:bg-muted/70'
                      )}
                    >
                      <Checkbox
                        checked={selectedIds.has(conversation.conversation_id)}
                        onCheckedChange={() =>
                          toggleSelected(conversation.conversation_id)
                        }
                        aria-label={t('Select conversation')}
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
                            onClick={submitRename}
                            disabled={renameMutation.isPending}
                            aria-label={t('Save')}
                          >
                            <Check aria-hidden='true' />
                          </Button>
                          <Button
                            size='icon-xs'
                            variant='ghost'
                            onClick={() => setEditingId(null)}
                            aria-label={t('Cancel')}
                          >
                            <X aria-hidden='true' />
                          </Button>
                        </div>
                      ) : (
                        <button
                          type='button'
                          className='min-w-0 flex-1 truncate text-left text-sm'
                          onClick={() => props.onSelect(conversation)}
                          disabled={props.disabled}
                        >
                          {conversation.name}
                        </button>
                      )}
                      {!isEditing && (
                        <div className='flex shrink-0 opacity-0 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100'>
                          <Button
                            size='icon-xs'
                            variant='ghost'
                            onClick={() => startRename(conversation)}
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
      <Dialog open={isManageOpen} onOpenChange={setIsManageOpen}>
        <DialogContent className='sm:max-w-md'>
          <DialogHeader>
            <DialogTitle>{t('Delete selected conversations')}</DialogTitle>
            <DialogDescription>
              {t('Deleted conversations cannot be recovered.')}
            </DialogDescription>
          </DialogHeader>
          <div className='bg-muted/40 flex items-center rounded-lg border px-3 py-2'>
            <label className='flex items-center gap-2 text-sm'>
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
          </div>
          <ScrollArea className='max-h-72'>
            <div className='flex flex-col gap-1 pr-3'>
              {conversationsQuery.isLoading && (
                <p className='text-muted-foreground py-6 text-center text-sm'>
                  {t('Loading conversations...')}
                </p>
              )}
              {!conversationsQuery.isLoading && conversations.length === 0 && (
                <p className='text-muted-foreground py-6 text-center text-sm'>
                  {t('No conversations yet')}
                </p>
              )}
              {conversations.map((conversation) => (
                <label
                  key={conversation.conversation_id}
                  className='hover:bg-muted flex items-center gap-3 rounded-lg px-3 py-2.5'
                >
                  <Checkbox
                    checked={selectedIds.has(conversation.conversation_id)}
                    onCheckedChange={() =>
                      toggleSelected(conversation.conversation_id)
                    }
                    aria-label={t('Select conversation')}
                  />
                  <span className='min-w-0 flex-1 truncate text-sm'>
                    {conversation.name}
                  </span>
                </label>
              ))}
            </div>
          </ScrollArea>
          <DialogFooter>
            <Button variant='outline' onClick={() => setIsManageOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button
              variant='destructive'
              onClick={deleteSelected}
              disabled={selectedIds.size === 0 || deleteMutation.isPending}
            >
              <Trash2 data-icon='inline-start' aria-hidden='true' />
              {t('Delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
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
