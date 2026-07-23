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
import { useTranslation } from 'react-i18next'
import { formatQuota, formatTimestampToDate } from '@/lib/format'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import type {
  FlexiblePaymentChoice,
  PlanRecord,
  SubscriptionPaymentAvailability,
  SubscriptionPaymentQuote,
  SubscriptionPaymentQuotes,
} from '@/features/subscriptions/types'
import {
  getMatchingPaymentQuote,
  requiresLocalCurrencyQuote,
} from '../lib/subscription-plan-lifecycle'

type PlanPurchaseDialogProps = {
  open: boolean
  plan: PlanRecord | null
  currentPlanId: number
  paymentAvailability: SubscriptionPaymentAvailability | undefined
  paymentQuotes?: SubscriptionPaymentQuotes
  selectedPaymentChoice?: FlexiblePaymentChoice
  months?: number
  isLoading?: boolean
  projectedStart?: number
  projectedEnd?: number
  projectedRemainingDays?: number
  refundableNotStartedValue?: number
  isQuoteLoading?: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: (choice: FlexiblePaymentChoice, months: number) => void
  onQuoteRequest?: (choice: FlexiblePaymentChoice, months: number) => void
}

const PAYMENT_CHOICES: FlexiblePaymentChoice[] = [
  'stripe_recurring',
  'alipay',
  'pix',
  'upi',
  'balance',
]

function getPaymentChoiceLabel(
  choice: FlexiblePaymentChoice,
  t: (key: string) => string
): string {
  switch (choice) {
    case 'stripe_recurring':
      return t('Stripe automatic subscription')
    case 'alipay':
      return t('Alipay')
    case 'pix':
      return t('Pix')
    case 'upi':
      return t('UPI')
    case 'balance':
      return t('Flatkey balance')
  }
}

function getDisabledReason(
  availability: SubscriptionPaymentAvailability | undefined,
  choice: FlexiblePaymentChoice
): string {
  const item = availability?.[choice]
  if (item?.available === false)
    return item.disabled_reason || item.reason || ''
  return ''
}

function formatPlanPrice(amount: number, currency = 'USD'): string {
  if (currency === 'BRL') {
    return Intl.NumberFormat('pt-BR', {
      style: 'currency',
      currency: 'BRL',
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(amount)
  }
  if (currency === 'INR') {
    return Intl.NumberFormat('en-IN', {
      style: 'currency',
      currency: 'INR',
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(amount)
  }
  return `$${Number.isInteger(amount) ? amount.toFixed(0) : amount.toFixed(2)}`
}

function getQuoteReadinessReason(
  choice: FlexiblePaymentChoice,
  quote: SubscriptionPaymentQuote | undefined,
  isQuoteLoading: boolean,
  t: (key: string) => string
): string {
  if (!requiresLocalCurrencyQuote(choice) || quote) return ''
  if (isQuoteLoading) return t('Loading local currency quote...')
  return t('Local currency quote is unavailable.')
}

function getMonthLabel(
  count: number,
  t: (key: string, values?: { count: number }) => string
): string {
  if (count === 1) return t('{{count}} month', { count })
  return t('{{count}} months', { count })
}

export function PlanPurchaseDialog(props: PlanPurchaseDialogProps) {
  const { t } = useTranslation()
  const [choice, setChoice] = useState<FlexiblePaymentChoice>(
    props.selectedPaymentChoice ?? 'stripe_recurring'
  )
  const [months, setMonths] = useState(props.months ?? 1)
  const selectedChoice = props.selectedPaymentChoice ?? choice
  const selectedMonths = props.months ?? months
  const showMonths = selectedChoice !== 'stripe_recurring'
  const selectedQuote = getMatchingPaymentQuote(
    selectedChoice,
    props.paymentQuotes,
    selectedMonths
  )

  const totalPrice = useMemo(() => {
    if (selectedQuote) return selectedQuote.total
    const unitPrice = Number(props.plan?.plan.price_amount || 0)
    return showMonths ? unitPrice * selectedMonths : unitPrice
  }, [props.plan?.plan.price_amount, selectedQuote, selectedMonths, showMonths])
  const selectedDisabledReason = getDisabledReason(
    props.paymentAvailability,
    selectedChoice
  )
  const selectedQuoteReadinessReason = getQuoteReadinessReason(
    selectedChoice,
    selectedQuote,
    props.isQuoteLoading === true,
    t
  )
  const totalPriceLabel = selectedQuoteReadinessReason
    ? '—'
    : formatPlanPrice(totalPrice, selectedQuote?.currency)

  if (!props.open || !props.plan) return null

  return (
    <div
      role='dialog'
      aria-modal='true'
      aria-labelledby='wallet-plan-purchase-title'
      className='bg-popover text-popover-foreground ring-foreground/10 grid w-full gap-4 rounded-xl p-4 text-sm ring-1 sm:max-w-xl'
    >
      <div className='flex flex-col gap-2'>
        <h2
          id='wallet-plan-purchase-title'
          className='text-base leading-none font-medium'
        >
          {t('Purchase plan')}
        </h2>
        <p className='text-muted-foreground text-sm'>
          {t('Review the payment choice and term before continuing.')}
        </p>
      </div>

      <div className='space-y-4'>
        <div className='grid gap-2' role='radiogroup'>
          {PAYMENT_CHOICES.map((paymentChoice) => {
            const disabledReason = getDisabledReason(
              props.paymentAvailability,
              paymentChoice
            )
            const disabled = disabledReason.length > 0
            return (
              <label
                key={paymentChoice}
                className='border-input has-[:focus-visible]:ring-ring/50 flex min-h-11 items-start gap-3 rounded-lg border px-3 py-2 text-sm has-[:focus-visible]:ring-3'
              >
                <input
                  type='radio'
                  name='wallet-payment-choice'
                  value={paymentChoice}
                  checked={selectedChoice === paymentChoice}
                  disabled={disabled}
                  onChange={() => {
                    setChoice(paymentChoice)
                    props.onQuoteRequest?.(paymentChoice, selectedMonths)
                  }}
                  className='mt-1'
                />
                <span className='min-w-0'>
                  <span className='block font-medium'>
                    {getPaymentChoiceLabel(paymentChoice, t)}
                  </span>
                  {disabledReason ? (
                    <span className='text-muted-foreground block text-xs'>
                      {disabledReason}
                    </span>
                  ) : null}
                </span>
              </label>
            )
          })}
        </div>

        {showMonths ? (
          <label className='grid gap-1.5 text-sm'>
            <span className='font-medium'>{t('Months')}</span>
            <NativeSelect
              className='w-full'
              value={selectedMonths}
              onChange={(event) => {
                const nextMonths = Number(event.target.value)
                setMonths(nextMonths)
                props.onQuoteRequest?.(selectedChoice, nextMonths)
              }}
              aria-label={t('Months')}
            >
              {Array.from({ length: 12 }, (_, index) => index + 1).map(
                (month) => (
                  <NativeSelectOption key={month} value={month}>
                    {getMonthLabel(month, t)}
                  </NativeSelectOption>
                )
              )}
            </NativeSelect>
          </label>
        ) : null}

        <div className='rounded-lg border p-3 text-sm'>
          <div className='flex items-center justify-between gap-3'>
            <span className='text-muted-foreground'>{t('Total price')}</span>
            <span className='font-semibold tabular-nums'>
              {totalPriceLabel}
            </span>
          </div>
          <div className='mt-2 grid gap-1 text-xs'>
            {selectedQuote ? (
              <div className='flex items-center justify-between gap-3'>
                <span className='text-muted-foreground'>{t('Unit price')}</span>
                <span className='font-medium tabular-nums'>
                  {formatPlanPrice(
                    selectedQuote.unit_price,
                    selectedQuote.currency
                  )}
                </span>
              </div>
            ) : null}
            {selectedQuoteReadinessReason ? (
              <div className='text-muted-foreground'>
                {selectedQuoteReadinessReason}
              </div>
            ) : null}
            {props.projectedStart ? (
              <div>
                {t('Start date')}: {formatTimestampToDate(props.projectedStart)}
              </div>
            ) : null}
            {props.projectedEnd ? (
              <div>
                {t('End date')}: {formatTimestampToDate(props.projectedEnd)}
              </div>
            ) : null}
            {typeof props.projectedRemainingDays === 'number' ? (
              <div>
                {t('Remaining days')}:{' '}
                {t('{{count}} days', {
                  count: Math.max(0, props.projectedRemainingDays),
                })}
              </div>
            ) : null}
          </div>
        </div>

        {props.currentPlanId > 0 ? (
          <Alert>
            <AlertDescription>
              {t(
                'Replacement charges the full target price. No prorating or credit is applied.'
              )}{' '}
              {t(
                'The active started term is not refunded. Monthly and Image + video usage reset; 5-hour and 7-day rolling usage is retained and re-evaluated.'
              )}
              {typeof props.refundableNotStartedValue === 'number'
                ? ` ${t('Refundable not-started value: {{value}}', {
                    value: formatQuota(props.refundableNotStartedValue),
                  })}`
                : ''}
            </AlertDescription>
          </Alert>
        ) : null}
      </div>

      <div className='bg-muted/50 -mx-4 -mb-4 flex flex-col-reverse gap-2 rounded-b-xl border-t p-4 sm:flex-row sm:justify-end'>
        <Button
          className='min-h-11'
          disabled={
            props.isLoading ||
            selectedDisabledReason.length > 0 ||
            selectedQuoteReadinessReason.length > 0
          }
          onClick={() => props.onConfirm(selectedChoice, selectedMonths)}
        >
          {t('Continue')}
        </Button>
        <Button
          variant='outline'
          className='min-h-11'
          disabled={props.isLoading}
          onClick={() => props.onOpenChange(false)}
        >
          {t('Close')}
        </Button>
      </div>
    </div>
  )
}
