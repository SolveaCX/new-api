/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'
import { formatQuota } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog } from '@/components/dialog'

interface TransferDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: (amount: number) => Promise<boolean>
  availableQuota: number
  transferring: boolean
}

export type TransferAmountValidationError =
  | 'invalid'
  | 'below-minimum'
  | 'exceeds-available'
  | null

// Exported for the focused boundary test; this remains pure UI-domain logic.
// eslint-disable-next-line react-refresh/only-export-components
export function getTransferAmountValidationError(
  amount: number,
  minimum: number,
  maximum: number
): TransferAmountValidationError {
  if (
    !Number.isFinite(amount) ||
    !Number.isInteger(amount) ||
    !Number.isFinite(minimum) ||
    !Number.isFinite(maximum) ||
    minimum <= 0
  ) {
    return 'invalid'
  }
  if (amount < minimum) return 'below-minimum'
  if (amount > maximum) return 'exceeds-available'
  return null
}

// Exported for the focused boundary test; this remains pure UI-domain logic.
// eslint-disable-next-line react-refresh/only-export-components
export function isValidTransferAmount(
  amount: number,
  minimum: number,
  maximum: number
): boolean {
  return getTransferAmountValidationError(amount, minimum, maximum) === null
}

export function TransferDialog({
  open,
  onOpenChange,
  onConfirm,
  availableQuota,
  transferring,
}: TransferDialogProps) {
  const { t } = useTranslation()
  const configuredQuotaPerUnit = useSystemConfigStore(
    (state) => state.config.currency.quotaPerUnit
  )
  const minimum =
    Number.isFinite(configuredQuotaPerUnit) && configuredQuotaPerUnit > 0
      ? configuredQuotaPerUnit
      : DEFAULT_CURRENCY_CONFIG.quotaPerUnit
  const [amount, setAmount] = useState(minimum)
  const validationError = getTransferAmountValidationError(
    amount,
    minimum,
    availableQuota
  )
  const valid = validationError === null
  const exceedsAvailable = validationError === 'exceeds-available'

  useEffect(() => {
    if (open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setAmount(minimum)
    }
  }, [minimum, open])

  const handleConfirm = async () => {
    if (!valid) return
    const success = await onConfirm(amount)
    if (success) onOpenChange(false)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Transfer Rewards')}
      description={t('Move available referral rewards to your main balance.')}
      contentClassName='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'
      footerClassName='grid grid-cols-2 gap-2 sm:flex'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            variant='outline'
            onClick={() => onOpenChange(false)}
            disabled={transferring}
          >
            {t('Cancel')}
          </Button>
          <Button onClick={handleConfirm} disabled={transferring || !valid}>
            {transferring ? (
              <Loader2 className='animate-spin' aria-hidden='true' />
            ) : null}
            {t('Transfer')}
          </Button>
        </>
      }
    >
      <div className='space-y-4 py-3'>
        <div className='space-y-2'>
          <Label className='text-muted-foreground text-xs font-medium uppercase'>
            {t('Available Rewards')}
          </Label>
          <p className='text-2xl font-semibold tabular-nums'>
            {formatQuota(availableQuota)}
          </p>
        </div>
        <div className='space-y-2'>
          <Label htmlFor='invitation-transfer-amount'>
            {t('Transfer Amount')}
          </Label>
          <Input
            id='invitation-transfer-amount'
            type='number'
            value={amount}
            onChange={(event) => setAmount(Number(event.currentTarget.value))}
            min={minimum}
            max={availableQuota}
            step={minimum}
            aria-invalid={!valid}
            aria-describedby={
              exceedsAvailable
                ? 'invitation-transfer-amount-error'
                : 'invitation-transfer-amount-help'
            }
            className='font-mono text-lg'
          />
          {exceedsAvailable ? (
            <p
              id='invitation-transfer-amount-error'
              role='alert'
              className='text-destructive text-xs'
            >
              {t('Transfer amount cannot exceed available rewards.')}
            </p>
          ) : (
            <p
              id='invitation-transfer-amount-help'
              className='text-muted-foreground text-xs'
            >
              {t('Minimum:')} {formatQuota(minimum)}
            </p>
          )}
        </div>
      </div>
    </Dialog>
  )
}
