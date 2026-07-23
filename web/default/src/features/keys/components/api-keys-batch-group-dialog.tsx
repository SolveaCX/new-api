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
import { isAxiosError } from 'axios'
import type { Table } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Spinner } from '@/components/ui/spinner'
import { batchUpdateApiKeyGroup } from '../api'
import { coordinateBatchGroupUpdate } from '../lib/api-key-batch-group'
import type { ApiKey } from '../types'
import {
  ApiKeyGroupCombobox,
  type ApiKeyGroupOption,
} from './api-key-group-combobox'
import { useApiKeys } from './api-keys-provider'

type ApiKeysBatchGroupDialogProps<TData> = {
  open: boolean
  onOpenChange: (open: boolean) => void
  options: ApiKeyGroupOption[]
  table: Table<TData>
}

function getServerErrorMessage(error: unknown): string | undefined {
  if (!isAxiosError<{ message?: unknown }>(error)) return undefined

  const message = error.response?.data?.message
  return typeof message === 'string' ? message : undefined
}

export function ApiKeysBatchGroupDialog<TData>(
  props: ApiKeysBatchGroupDialogProps<TData>
) {
  const { t } = useTranslation()
  const { triggerRefresh } = useApiKeys()
  const [group, setGroup] = useState<string>()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const selectedRows = props.table.getFilteredSelectedRowModel().rows

  const handleOpenChange = (open: boolean) => {
    if (isSubmitting) return
    if (!open) setGroup(undefined)
    props.onOpenChange(open)
  }

  const handleConfirm = async () => {
    if (!group) return

    setIsSubmitting(true)
    try {
      const ids = selectedRows.map((row) => (row.original as ApiKey).id)
      const result = await coordinateBatchGroupUpdate({
        request: batchUpdateApiKeyGroup,
        ids,
        group,
        successEffects: {
          resetSelection: () => props.table.resetRowSelection(),
          refresh: triggerRefresh,
          clearGroup: () => setGroup(undefined),
          closeDialog: () => props.onOpenChange(false),
        },
      })
      if (!result.success) {
        toast.error(
          result.message || t('Failed to update selected API key groups')
        )
        return
      }

      toast.success(
        t('Updated the group for {{count}} API key(s)', {
          count: result.count,
        })
      )
    } catch (error) {
      toast.error(
        getServerErrorMessage(error) ||
          t('Failed to update selected API key groups')
      )
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog open={props.open} onOpenChange={handleOpenChange}>
      <DialogContent className='sm:max-w-md' showCloseButton={!isSubmitting}>
        <DialogHeader>
          <DialogTitle>
            {t('Update group for {{count}} API key(s)', {
              count: selectedRows.length,
            })}
          </DialogTitle>
          <DialogDescription>
            {t('Choose a group for the selected API keys.')}
          </DialogDescription>
        </DialogHeader>

        <FieldGroup className='py-2'>
          <Field data-disabled={isSubmitting || undefined}>
            <FieldLabel>{t('Group')}</FieldLabel>
            <ApiKeyGroupCombobox
              options={props.options}
              value={group}
              onValueChange={setGroup}
              placeholder={t('Select a group')}
              ariaLabel={t('Select a group')}
              disabled={isSubmitting}
            />
          </Field>
        </FieldGroup>

        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => handleOpenChange(false)}
            disabled={isSubmitting}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            onClick={() => void handleConfirm()}
            disabled={!group || isSubmitting}
          >
            {isSubmitting && <Spinner data-icon='inline-start' />}
            {t('Update group')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
