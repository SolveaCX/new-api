/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import { Spinner } from '@/components/ui/spinner'

interface ProgressiveListProps {
  isLoading: boolean
  isError: boolean
  isEmpty: boolean
  hasMore?: boolean
  isLoadingMore?: boolean
  onLoadMore?: () => void
  children: ReactNode
}

export function ProgressiveLoadMoreButton(props: {
  visible: boolean
  isLoading?: boolean
  onClick?: () => void
}) {
  const { t } = useTranslation()
  if (!props.visible) return null
  return (
    <Button
      type='button'
      size='sm'
      variant='outline'
      disabled={props.isLoading}
      onClick={props.onClick}
    >
      {props.isLoading ? <Spinner /> : null}
      {t('Load more')}
    </Button>
  )
}

export function ProgressiveList(props: ProgressiveListProps) {
  const { t } = useTranslation()
  if (props.isLoading) {
    return (
      <div className='text-muted-foreground flex min-h-20 items-center justify-center gap-2 text-sm'>
        <Spinner />
        <span>{t('Loading')}</span>
      </div>
    )
  }
  if (props.isError) {
    return (
      <Alert variant='destructive'>
        <AlertTitle>{t('Unable to load data')}</AlertTitle>
        <AlertDescription>{t('Please try again later.')}</AlertDescription>
      </Alert>
    )
  }
  if (props.isEmpty) {
    return (
      <Empty className='min-h-20 border'>
        <EmptyHeader>
          <EmptyTitle>{t('No data')}</EmptyTitle>
          <EmptyDescription>{t('Please try again later.')}</EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }
  return (
    <div className='flex flex-col gap-2'>
      {props.children}
      <ProgressiveLoadMoreButton
        visible={Boolean(props.hasMore)}
        isLoading={props.isLoadingMore}
        onClick={props.onLoadMore}
      />
    </div>
  )
}
