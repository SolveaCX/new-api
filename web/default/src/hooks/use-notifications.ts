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
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useNotificationStore } from '@/stores/notification-store'
import { getNotice } from '@/lib/api'
import { useStatus } from '@/hooks/use-status'

function hashString(input: string): string {
  let hash = 0
  if (!input) return '0'

  for (let i = 0; i < input.length; i += 1) {
    const chr = input.charCodeAt(i)
    hash = (hash << 5) - hash + chr
    hash |= 0
  }

  return hash.toString(36)
}

/**
 * Generate a unique key for an announcement
 * Prefer backend id, fall back to a content hash so edits register
 */
function getAnnouncementKey(item: Record<string, unknown>): string {
  if (!item) return ''

  if (item.id !== undefined && item.id !== null) {
    return `id:${item.id}`
  }

  const fingerprint = JSON.stringify({
    publishDate: (item?.publishDate as string) || '',
    content: ((item?.content as string) || '').trim(),
    extra: ((item?.extra as string) || '').trim(),
    type: (item?.type as string) || '',
    title: ((item?.title as string) || '').trim(),
    link: ((item?.link as string) || '').trim(),
  })
  return `hash:${hashString(fingerprint)}`
}

export interface NotificationTimelineItem {
  key: string
  source: 'notice' | 'announcement'
  type?: string
  content?: string
  extra?: string
  publishDate?: string | Date
  link?: string
}

/**
 * Hook to manage notifications (Notice + Announcements)
 * Provides unread counts and read status management
 */
export function useNotifications() {
  const [popoverOpen, setPopoverOpen] = useState(false)
  const { i18n } = useTranslation()

  // Fetch Notice from API
  const {
    data: noticeResponse,
    isLoading: noticeLoading,
    refetch: refetchNotice,
  } = useQuery({
    queryKey: ['notice', i18n.language],
    queryFn: getNotice,
    staleTime: 1000 * 60 * 5, // 5 minutes
  })

  // Fetch Announcements from status
  const { status, loading: statusLoading } = useStatus()
  const announcementsEnabled = status?.announcements_enabled ?? false
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const announcements: Record<string, unknown>[] = announcementsEnabled
    ? ((status?.announcements || []) as Record<string, unknown>[]).slice(0, 20)
    : []

  // Notification store
  const {
    lastReadNotice,
    markNoticeRead,
    markAnnouncementsRead,
    isAnnouncementRead,
  } = useNotificationStore()

  // Extract notice content
  const noticeContent = noticeResponse?.success
    ? (noticeResponse.data || '').trim()
    : ''

  const timeline = useMemo<NotificationTimelineItem[]>(() => {
    const items: NotificationTimelineItem[] = announcements.map((item) => ({
      key: getAnnouncementKey(item),
      source: 'announcement' as const,
      type: typeof item.type === 'string' ? item.type : undefined,
      content: typeof item.content === 'string' ? item.content : undefined,
      extra: typeof item.extra === 'string' ? item.extra : undefined,
      publishDate:
        typeof item.publishDate === 'string' || item.publishDate instanceof Date
          ? item.publishDate
          : undefined,
      link: typeof item.link === 'string' ? item.link : undefined,
    }))

    if (noticeContent) {
      items.push({
        content: noticeContent,
        key: `notice:${noticeContent}`,
        source: 'notice' as const,
        type: 'default',
      })
    }

    return items.sort((a, b) => {
      if (a.source === 'notice') return -1
      if (b.source === 'notice') return 1
      return (
        new Date(String(b.publishDate || 0)).getTime() -
        new Date(String(a.publishDate || 0)).getTime()
      )
    })
  }, [announcements, noticeContent])

  // Calculate unread counts
  const unreadCounts = useMemo(() => {
    const noticeUnread = noticeContent && noticeContent !== lastReadNotice ? 1 : 0
    const announcementsUnread = announcements.filter((item) =>
      !isAnnouncementRead(getAnnouncementKey(item))
    ).length

    return {
      notice: noticeUnread,
      announcements: announcementsUnread,
      total: noticeUnread + announcementsUnread,
    }
  }, [noticeContent, lastReadNotice, announcements, isAnnouncementRead])

  const markAnnouncementsAsRead = () => {
    if (announcements.length > 0) {
      const allKeys = announcements.map((item: Record<string, unknown>) =>
        getAnnouncementKey(item)
      )
      markAnnouncementsRead(allKeys)
    }
  }

  // Handle popover open
  const handleOpenPopover = () => {
    if (noticeContent) {
      markNoticeRead(noticeContent)
    }
    markAnnouncementsAsRead()
    setPopoverOpen(true)
  }

  const handlePopoverOpenChange = (open: boolean) => {
    if (open) {
      handleOpenPopover()
      return
    }

    setPopoverOpen(false)
  }

  return {
    // Data
    notice: noticeContent,
    timeline,
    loading: noticeLoading || statusLoading,

    // Unread counts
    unreadCount: unreadCounts.total,
    unreadNoticeCount: unreadCounts.notice,
    unreadAnnouncementsCount: unreadCounts.announcements,

    // Popover state
    popoverOpen,
    setPopoverOpen: handlePopoverOpenChange,

    // Actions
    openPopover: handleOpenPopover,
    closePopover: () => setPopoverOpen(false),
    refetchNotice,
  }
}
