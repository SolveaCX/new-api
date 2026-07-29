/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useState, type ReactNode } from 'react'
import { Search01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import type { SupplyChainManagementProps } from '../contracts'
import type { SupplierStatus } from '../types'

export function ManagementToolbar(props: {
  search: SupplyChainManagementProps['search']
  onSearchChange: SupplyChainManagementProps['onSearchChange']
  actions?: ReactNode
  filters?: ReactNode
}) {
  const { t } = useTranslation()
  const [keyword, setKeyword] = useState(props.search.filter)

  return (
    <div className='bg-card flex flex-col gap-3 rounded-xl border p-3 sm:flex-row sm:items-center'>
      <form
        className='flex min-w-0 flex-1 gap-2'
        onSubmit={(event) => {
          event.preventDefault()
          props.onSearchChange({ filter: keyword.trim(), page: 1 })
        }}
      >
        <Input
          value={keyword}
          onChange={(event) => setKeyword(event.target.value)}
          placeholder={t('Search')}
          aria-label={t('Search')}
          className='min-w-0'
        />
        <Button type='submit' variant='outline'>
          <HugeiconsIcon icon={Search01Icon} strokeWidth={2} />
          {t('Search')}
        </Button>
      </form>
      {props.filters}
      {props.actions}
    </div>
  )
}

export function ManagementStatus(props: {
  isLoading: boolean
  isError: boolean
  isEmpty: boolean
  children: ReactNode
}) {
  const { t } = useTranslation()
  if (props.isLoading) {
    return (
      <div className='flex min-h-52 items-center justify-center rounded-xl border'>
        <Spinner className='size-5' />
        <span className='sr-only'>{t('Loading')}</span>
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
      <Empty className='min-h-52 border'>
        <EmptyHeader>
          <EmptyTitle>{t('No data')}</EmptyTitle>
          <EmptyDescription>
            {t('Adjust the filters or create the first record.')}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }
  return props.children
}

export function ManagementPagination(props: {
  page: number
  pageSize: number
  total: number
  onSearchChange: SupplyChainManagementProps['onSearchChange']
}) {
  const { t } = useTranslation()
  const lastPage = Math.max(1, Math.ceil(props.total / props.pageSize))
  return (
    <div className='text-muted-foreground flex flex-wrap items-center justify-between gap-2 text-sm'>
      <span>
        {t('{{count}} records', { count: props.total })} · {props.page}/
        {lastPage}
      </span>
      <div className='flex gap-2'>
        <Button
          type='button'
          size='sm'
          variant='outline'
          disabled={props.page <= 1}
          onClick={() => props.onSearchChange({ page: props.page - 1 })}
        >
          {t('Previous')}
        </Button>
        <Button
          type='button'
          size='sm'
          variant='outline'
          disabled={props.page >= lastPage}
          onClick={() => props.onSearchChange({ page: props.page + 1 })}
        >
          {t('Next')}
        </Button>
      </div>
    </div>
  )
}

export function StatusBadge(props: { status: SupplierStatus }) {
  const { t } = useTranslation()
  return (
    <Badge variant={props.status === 'active' ? 'default' : 'secondary'}>
      {props.status === 'active' ? t('Active') : t('Inactive')}
    </Badge>
  )
}

export function ConfirmAction(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description: ReactNode
  confirmLabel: string
  pending: boolean
  destructive?: boolean
  onConfirm: () => void
}) {
  const { t } = useTranslation()
  return (
    <AlertDialog open={props.open} onOpenChange={props.onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{props.title}</AlertDialogTitle>
          <AlertDialogDescription>{props.description}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={props.pending}>
            {t('Cancel')}
          </AlertDialogCancel>
          <AlertDialogAction
            variant={props.destructive ? 'destructive' : 'default'}
            disabled={props.pending}
            onClick={props.onConfirm}
          >
            {props.pending ? <Spinner /> : null}
            {props.confirmLabel}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
