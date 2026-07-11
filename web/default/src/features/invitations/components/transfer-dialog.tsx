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
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog } from '@/components/dialog'
import { formatInvitationUSD } from '../lib/usd'

interface TransferDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: (amount: number) => Promise<boolean>
  availableUSD: number
  transferring: boolean
}

const MINIMUM_TRANSFER_USD = 1

// eslint-disable-next-line react-refresh/only-export-components
export function getTransferAmountInputConstraints(availableUSD: number) {
  return {
    min: MINIMUM_TRANSFER_USD,
    max: availableUSD,
    step: 'any' as const,
  }
}

// eslint-disable-next-line react-refresh/only-export-components
export function clampTransferAmount(
  amount: number,
  availableUSD: number
): number {
  if (!Number.isFinite(amount) || !Number.isFinite(availableUSD)) return amount
  return Math.min(amount, availableUSD)
}

// eslint-disable-next-line react-refresh/only-export-components
export function isValidTransferAmount(
  amount: number,
  availableUSD: number
): boolean {
  return (
    Number.isFinite(amount) &&
    Number.isFinite(availableUSD) &&
    amount >= MINIMUM_TRANSFER_USD &&
    amount <= availableUSD
  )
}

export function TransferDialog({
  open,
  onOpenChange,
  onConfirm,
  availableUSD,
  transferring,
}: TransferDialogProps) {
  const { t } = useTranslation()
  const [amount, setAmount] = useState(
    Math.min(MINIMUM_TRANSFER_USD, availableUSD)
  )
  const valid = isValidTransferAmount(amount, availableUSD)

  useEffect(() => {
    if (open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setAmount(Math.min(MINIMUM_TRANSFER_USD, availableUSD))
    }
  }, [availableUSD, open])

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
            {formatInvitationUSD(availableUSD)}
          </p>
        </div>
        <div className='space-y-2'>
          <Label htmlFor='invitation-transfer-amount'>
            {t('Transfer Amount')}
          </Label>
          <div className='relative'>
            <span
              aria-hidden='true'
              className='text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 -translate-y-1/2 font-mono text-lg'
            >
              $
            </span>
            <Input
              id='invitation-transfer-amount'
              type='number'
              value={amount}
              onChange={(event) =>
                setAmount(
                  clampTransferAmount(
                    Number(event.currentTarget.value),
                    availableUSD
                  )
                )
              }
              {...getTransferAmountInputConstraints(availableUSD)}
              aria-invalid={!valid}
              aria-describedby='invitation-transfer-amount-help'
              className='pl-7 font-mono text-lg'
            />
          </div>
          <p
            id='invitation-transfer-amount-help'
            className='text-muted-foreground text-xs'
          >
            {t('Minimum:')} {formatInvitationUSD(MINIMUM_TRANSFER_USD)}
          </p>
        </div>
      </div>
    </Dialog>
  )
}
