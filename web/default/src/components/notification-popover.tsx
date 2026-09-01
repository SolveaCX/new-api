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
import type { TFunction } from 'i18next'
import { useEffect, useRef, useState } from 'react'
import { Bell, ExternalLink, Megaphone } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatDateTimeObject } from '@/lib/time'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Markdown } from '@/components/ui/markdown'
import {
  Popover,
  PopoverContent,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'

interface AnnouncementItem {
  key?: string
  source?: 'notice' | 'announcement'
  type?: string
  content?: string
  extra?: string
  publishDate?: string | Date
  link?: string
}

interface NotificationPopoverProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  unreadCount: number
  timeline: AnnouncementItem[]
  loading: boolean
  className?: string
}

const announcementMarkdownClassName =
  '[&_h1]:mt-0 [&_h1]:mb-4 [&_h2]:mt-4 [&_h2]:mb-3 [&_h3]:mt-3 [&_h3]:mb-2 ' +
  '[&_p]:my-3 [&_ul]:my-3 [&_ul]:list-disc [&_ul]:pl-6 ' +
  '[&_ol]:my-3 [&_ol]:list-decimal [&_ol]:pl-6 [&_li]:my-1'

function ExpandableMarkdown({ content }: { content: string }) {
  const contentRef = useRef<HTMLDivElement>(null)
  const [open, setOpen] = useState(false)
  const [canExpand, setCanExpand] = useState(false)
  const { t } = useTranslation()

  useEffect(() => {
    const element = contentRef.current
    if (!element) return

    const measure = () => {
      setCanExpand(element.scrollHeight > 72 + 1)
    }

    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(element)
    return () => observer.disconnect()
  }, [content])

  const handleClick = (event: React.MouseEvent<HTMLDivElement>) => {
    if (!canExpand || (event.target as HTMLElement).closest('a')) return
    event.preventDefault()
    event.stopPropagation()
    setOpen(true)
  }

  const handleKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (!canExpand || (event.target as HTMLElement).closest('a')) return
    if (event.key !== 'Enter' && event.key !== ' ') return
    event.preventDefault()
    setOpen(true)
  }

  return (
    <div
      ref={contentRef}
      role={canExpand ? 'button' : undefined}
      tabIndex={canExpand ? 0 : undefined}
      aria-haspopup={canExpand ? 'dialog' : undefined}
      onClick={handleClick}
      onKeyDown={handleKeyDown}
      className={
        canExpand
          ? "relative max-h-[4.5rem] cursor-pointer overflow-hidden after:absolute after:right-0 after:bottom-0 after:bg-popover after:px-1 after:font-medium after:content-['...']"
          : undefined
      }
    >
      <Markdown className={announcementMarkdownClassName}>
        {content}
      </Markdown>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className='max-h-[min(80vh,40rem)] overflow-y-auto sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>{t('System Announcements')}</DialogTitle>
          </DialogHeader>
          <Markdown className={announcementMarkdownClassName}>
            {content}
          </Markdown>
        </DialogContent>
      </Dialog>
    </div>
  )
}

/**
 * Get relative time string from a date
 */
function getRelativeTime(publishDate: string | Date, t: TFunction): string {
  if (!publishDate) return ''

  const now = new Date()
  const pubDate = new Date(publishDate)

  // If invalid date, return original string
  if (isNaN(pubDate.getTime()))
    return typeof publishDate === 'string' ? publishDate : ''

  const diffMs = now.getTime() - pubDate.getTime()
  const diffSeconds = Math.floor(diffMs / 1000)
  const diffMinutes = Math.floor(diffSeconds / 60)
  const diffHours = Math.floor(diffMinutes / 60)
  const diffDays = Math.floor(diffHours / 24)
  const diffWeeks = Math.floor(diffDays / 7)
  const diffMonths = Math.floor(diffDays / 30)
  const diffYears = Math.floor(diffDays / 365)

  // If future time, show specific date
  if (diffMs < 0) return formatDateTimeObject(pubDate)

  // Return relative time based on difference
  if (diffSeconds < 60) return t('Just now')
  if (diffMinutes < 60)
    return diffMinutes === 1
      ? t('1 minute ago')
      : t('{{count}} minutes ago', { count: diffMinutes })
  if (diffHours < 24)
    return diffHours === 1
      ? t('1 hour ago')
      : t('{{count}} hours ago', { count: diffHours })
  if (diffDays < 7)
    return diffDays === 1
      ? t('1 day ago')
      : t('{{count}} days ago', { count: diffDays })
  if (diffWeeks < 4)
    return diffWeeks === 1
      ? t('1 week ago')
      : t('{{count}} weeks ago', { count: diffWeeks })
  if (diffMonths < 12)
    return diffMonths === 1
      ? t('1 month ago')
      : t('{{count}} months ago', { count: diffMonths })
  if (diffYears < 2) return t('1 year ago')

  // Over 2 years, show specific date
  return formatDateTimeObject(pubDate)
}

/**
 * Empty state component
 */
function EmptyState({
  icon,
  title,
  description,
}: {
  icon: React.ReactNode
  title: string
  description?: string
}) {
  return (
    <Empty className='min-h-48 border-0 p-4'>
      <EmptyHeader>
        <EmptyMedia variant='icon'>{icon}</EmptyMedia>
        <EmptyTitle>{title}</EmptyTitle>
        {description ? (
          <EmptyDescription>{description}</EmptyDescription>
        ) : null}
      </EmptyHeader>
    </Empty>
  )
}

function AnnouncementsContent({
  timeline,
  loading,
  t,
}: {
  timeline: AnnouncementItem[]
  loading: boolean
  t: TFunction
}) {
  if (loading) {
    return (
      <EmptyState
        icon={<Megaphone />}
        title={t('Loading...')}
        description={t('Latest platform updates and notices')}
      />
    )
  }

  if (timeline.length === 0) {
    return <EmptyState icon={<Bell />} title={t('No announcements at this time')} />
  }

  return (
    <ScrollArea className='h-[min(52vh,28rem)] pr-3'>
      <div className='flex flex-col'>
        {timeline.map((item, idx) => {
          const publishDate = item.publishDate
            ? new Date(item.publishDate)
            : null
          const relativeTime = publishDate
            ? getRelativeTime(publishDate, t)
            : ''
          const absoluteTime = publishDate
            ? formatDateTimeObject(publishDate)
            : ''

          const content = (
            <div className='py-3'>
              <div className='flex items-start gap-3'>
                <div className='flex min-w-0 flex-1 flex-col gap-2'>
                  <div className='text-sm'>
                    <ExpandableMarkdown content={item.content || ''} />
                  </div>

                  {item.extra ? (
                    <div className='text-muted-foreground text-xs'>
                      <ExpandableMarkdown content={item.extra} />
                    </div>
                  ) : null}

                  {absoluteTime ? (
                    <div className='text-muted-foreground text-xs'>
                      {relativeTime ? `${relativeTime} • ` : null}
                      {absoluteTime}
                    </div>
                  ) : null}
                </div>
                {item.link ? (
                  <ExternalLink className='text-muted-foreground mt-1 size-3.5 shrink-0' />
                ) : null}
              </div>
            </div>
          )

          return (
            <div key={item.key || idx}>
              {item.link ? (
                <a href={item.link} target='_blank' rel='noopener noreferrer' className='hover:bg-muted/40 block transition-colors'>
                  {content}
                </a>
              ) : (
                content
              )}
              {idx < timeline.length - 1 ? <Separator /> : null}
            </div>
          )
        })}
      </div>
    </ScrollArea>
  )
}

/**
 * Notification popover with a unified notification timeline
 */
export function NotificationPopover({
  open,
  onOpenChange,
  unreadCount,
  timeline,
  loading,
  className,
}: NotificationPopoverProps) {
  const { t } = useTranslation()
  return (
    <Popover open={open} onOpenChange={onOpenChange}>
      <PopoverTrigger
        render={
          <Button
            variant='ghost'
            size='icon'
            className={cn('relative size-9', className)}
            aria-label={t('Notifications')}
          />
        }
      >
        <Bell className='size-[1.2rem]' />
        {unreadCount > 0 ? (
          <Badge
            variant='destructive'
            className='absolute -top-1 -right-1 flex h-5 min-w-5 items-center justify-center px-1 text-[10px] font-semibold tabular-nums'
          >
            {unreadCount > 99 ? '99+' : unreadCount}
          </Badge>
        ) : null}
      </PopoverTrigger>

      <PopoverContent
        align='end'
        sideOffset={8}
        className='w-[min(26rem,calc(100vw-1rem))] gap-3 p-3'
      >
        <PopoverHeader className='gap-1 px-1'>
          <PopoverTitle>{t('System Announcements')}</PopoverTitle>
          <p className='text-muted-foreground text-xs'>
            {t('Latest platform updates and notices')}
          </p>
        </PopoverHeader>

        <AnnouncementsContent timeline={timeline} loading={loading} t={t} />

        <div className='flex justify-end'>
          <Button size='sm' onClick={() => onOpenChange(false)}>
            {t('Close')}
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}
