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
import { Loader2, Tag, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import type { StripeCheckoutDiscountState } from '../../types'

export type StripePromotionCodeBusyAction = 'apply' | 'restore' | null

interface StripePromotionCodeControlProps {
  value: string
  discountState: StripeCheckoutDiscountState
  busy: boolean
  busyAction?: StripePromotionCodeBusyAction
  message: { kind: 'success' | 'error'; text: string } | null
  onValueChange: (value: string) => void
  onApply: () => void
  onRemove: () => void
}

export function StripePromotionCodeControl({
  value,
  discountState,
  busy,
  busyAction = null,
  message,
  onValueChange,
  onApply,
  onRemove,
}: StripePromotionCodeControlProps) {
  const { t } = useTranslation()
  const hasManualDiscount = discountState.source === 'manual'
  const manualCode =
    discountState.promotion_code_masked || discountState.display_name
  const canApply = value.trim().length > 0 && !busy
  const applying = busy && busyAction === 'apply'
  const restoring = busy && busyAction === 'restore'

  return (
    <section
      data-slot='stripe-promotion-code-control'
      className='min-w-0 rounded-[14px] border border-[#dfe3e8] bg-white p-4 shadow-[0_8px_22px_rgba(22,30,46,0.06)]'
      aria-label={t('Promotion code')}
    >
      <form
        className='grid min-w-0 gap-3'
        onSubmit={(event) => {
          event.preventDefault()
          if (canApply) onApply()
        }}
      >
        <label
          htmlFor='stripe-promotion-code'
          className='flex min-w-0 items-center gap-2 text-sm font-bold text-[#4b5057]'
        >
          <Tag aria-hidden='true' className='size-4 shrink-0 text-[#0576d7]' />
          <span>{t('Promotion code')}</span>
        </label>
        <div className='grid min-w-0 grid-cols-[minmax(0,1fr)_auto] gap-2 max-[520px]:grid-cols-1'>
          <input
            id='stripe-promotion-code'
            name='promotion_code'
            value={value}
            disabled={busy}
            autoComplete='off'
            placeholder={t('Enter promotion code')}
            onChange={(event) => onValueChange(event.currentTarget.value)}
            className='min-h-11 min-w-0 rounded-lg border border-[#cfd5dc] bg-[#fbfbfc] px-3 text-base text-[#20242a] transition outline-none placeholder:text-[#8a929d] focus:border-[#0576d7] focus:ring-4 focus:ring-blue-100 disabled:cursor-not-allowed disabled:opacity-60'
          />
          <button
            type='submit'
            disabled={!canApply}
            className='inline-flex min-h-11 items-center justify-center gap-2 rounded-lg bg-[#20242a] px-4 text-sm font-bold whitespace-nowrap text-white transition hover:bg-[#111418] focus-visible:ring-4 focus-visible:ring-slate-200 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50'
          >
            {applying ? (
              <Loader2 aria-hidden='true' className='size-4 animate-spin' />
            ) : null}
            {applying ? t('Applying promotion code...') : t('Apply')}
          </button>
        </div>
      </form>

      {hasManualDiscount ? (
        <div className='mt-3 flex min-w-0 flex-wrap items-center justify-between gap-2 rounded-lg bg-blue-50 px-3 py-2 text-sm text-blue-900'>
          <span className='min-w-0 font-semibold break-words'>
            {manualCode || t('Promotion code applied.')}
          </span>
          <button
            type='button'
            disabled={busy}
            onClick={onRemove}
            className='inline-flex items-center gap-1.5 rounded-md px-2 py-1 font-bold text-blue-800 transition hover:bg-blue-100 focus-visible:ring-4 focus-visible:ring-blue-100 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50'
          >
            {restoring ? (
              <Loader2 aria-hidden='true' className='size-4 animate-spin' />
            ) : (
              <X aria-hidden='true' className='size-4' />
            )}
            {restoring
              ? t('Restoring previous discount...')
              : t('Remove promotion code')}
          </button>
        </div>
      ) : null}

      {message ? (
        <p
          role={message.kind === 'error' ? 'alert' : undefined}
          aria-live='polite'
          className={
            message.kind === 'error'
              ? 'mt-3 text-sm font-semibold text-red-700'
              : 'mt-3 text-sm font-semibold text-emerald-700'
          }
        >
          {message.text}
        </p>
      ) : null}
    </section>
  )
}
