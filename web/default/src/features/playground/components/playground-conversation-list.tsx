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
  ChevronLeft,
  ChevronRight,
  ListChecks,
  Plus,
  Trash2,
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
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  deletePlaygroundConversations,
  listPlaygroundConversations,
} from '../api'
import type { PlaygroundConversationSummary } from '../types'

interface PlaygroundConversationListProps {
  currentConversationId: string
  disabled?: boolean
  refreshKey?: number
  draftConversation?: PlaygroundConversationSummary | null
  onNew: () => void
}

const conversationsQueryKey = ['playground-conversations']

function PlaygroundConversationListContent(
  props: PlaygroundConversationListProps
) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
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

  return (
    <>
      <aside
        className={cn(
          'bg-sidebar border-sidebar-border flex h-full shrink-0 flex-col border-r transition-[width] duration-200',
          isCollapsed ? 'w-14' : 'w-52'
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
            className='text-muted-foreground rounded-lg'
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
        <nav
          className='flex flex-1 flex-col gap-1 px-2'
          aria-label={t('Conversations')}
        >
          {isCollapsed ? (
            <>
              <Button
                variant='ghost'
                size='icon'
                className='rounded-lg'
                onClick={props.onNew}
                disabled={props.disabled}
                aria-label={t('New')}
              >
                <Plus aria-hidden='true' />
              </Button>
              <Button
                variant='ghost'
                size='icon'
                className='text-muted-foreground hover:text-destructive rounded-lg'
                onClick={requestDeleteCurrent}
                disabled={
                  deleteMutation.isPending ||
                  !conversations.some(
                    (conversation) =>
                      conversation.conversation_id ===
                      props.currentConversationId
                  )
                }
                aria-label={t('Delete this conversation?')}
              >
                <Trash2 aria-hidden='true' />
              </Button>
              <Button
                variant='ghost'
                size='icon'
                className='text-muted-foreground rounded-lg'
                onClick={() => setIsManageOpen(true)}
                disabled={conversations.length === 0}
                aria-label={t('Delete selected conversations')}
              >
                <ListChecks aria-hidden='true' />
              </Button>
            </>
          ) : (
            <>
              <Button
                variant='ghost'
                className='h-10 w-full justify-start rounded-lg px-3'
                onClick={props.onNew}
                disabled={props.disabled}
              >
                <Plus data-icon='inline-start' aria-hidden='true' />
                {t('New')}
              </Button>
              <Button
                variant='ghost'
                className='text-muted-foreground hover:text-destructive h-10 w-full justify-start rounded-lg px-3'
                onClick={requestDeleteCurrent}
                disabled={
                  deleteMutation.isPending ||
                  !conversations.some(
                    (conversation) =>
                      conversation.conversation_id ===
                      props.currentConversationId
                  )
                }
              >
                <Trash2 data-icon='inline-start' aria-hidden='true' />
                {t('Delete')}
              </Button>
              <Button
                variant='ghost'
                className='text-muted-foreground h-10 w-full justify-start rounded-lg px-3'
                onClick={() => setIsManageOpen(true)}
                disabled={conversations.length === 0}
              >
                <ListChecks data-icon='inline-start' aria-hidden='true' />
                {t('Delete selected conversations')}
              </Button>
            </>
          )}
        </nav>
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
