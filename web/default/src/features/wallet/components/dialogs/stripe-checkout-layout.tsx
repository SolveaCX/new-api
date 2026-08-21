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
import type { ReactNode } from 'react'
import { Gift, Loader2, LockKeyhole, ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import type {
  StripeCheckoutSummaryLineKey,
  StripeCheckoutViewModel,
} from '../../lib/stripe-checkout-view-model'

interface StripeCheckoutLayoutProps {
  title: string
  description: string
  viewModel: StripeCheckoutViewModel | null
  onPaymentContainer: (element: HTMLDivElement | null) => void
  onCurrencyContainer: (element: HTMLDivElement | null) => void
  showCurrencySelector: boolean
  mounting: boolean
  submitting: boolean
  error: string | null
  onConfirm: () => void
  promotionCode: string
  promotionCodeApplied: boolean
  promotionCodeError: string | null
  promotionCodeSubmitting: boolean
  onPromotionCodeChange: (value: string) => void
  onApplyPromotionCode: () => void
  onRemovePromotionCode: () => void
  closeControl?: ReactNode
}

function getSummaryLabel(
  key: StripeCheckoutSummaryLineKey,
  t: (key: string) => string
): string {
  if (key === 'subtotal') return t('Subtotal')
  if (key === 'discount') return t('Discount')
  if (key === 'tax') return t('Tax')
  return t('Surcharge')
}

function getSummaryAmountPrefix(key: StripeCheckoutSummaryLineKey): string {
  return key === 'discount' ? '−' : ''
}

export function StripeCheckoutLayout({
  title,
  description,
  viewModel,
  onPaymentContainer,
  onCurrencyContainer,
  showCurrencySelector,
  mounting,
  submitting,
  error,
  onConfirm,
  promotionCode,
  promotionCodeApplied,
  promotionCodeError,
  promotionCodeSubmitting,
  onPromotionCodeChange,
  onApplyPromotionCode,
  onRemovePromotionCode,
  closeControl,
}: StripeCheckoutLayoutProps) {
  const { t } = useTranslation()
  const canSubmit = Boolean(viewModel?.canConfirm && !mounting && !submitting)
  let buttonLabel = t('Continue')
  if (submitting) {
    buttonLabel = t('Processing payment...')
  }

  return (
    <div className='relative min-h-[690px] w-full overflow-hidden bg-white max-[900px]:min-h-0'>
      {closeControl}
      <div className='grid min-h-[690px] grid-cols-[minmax(0,1.16fr)_minmax(360px,0.84fr)] max-[900px]:min-h-0 max-[900px]:grid-cols-1'>
        <section
          data-slot='stripe-checkout-form-pane'
          aria-label={t('Payment details')}
          className='min-w-0 border-r border-[#dfe3e8] p-14 max-[900px]:border-r-0 max-[900px]:border-b max-[900px]:px-6 max-[900px]:py-9 max-[520px]:px-[18px] max-[520px]:py-7'
        >
          <div className='mb-9 flex items-start gap-3.5 pr-14'>
            <span
              aria-hidden='true'
              className='mt-2.5 size-3 shrink-0 rounded-full bg-[#31c3b1] shadow-[0_0_0_6px_rgba(49,195,177,0.14)]'
            />
            <div>
              <p className='mb-2 text-base font-semibold text-[#5e6470] sm:text-lg'>
                {t('Stripe secure checkout')}
              </p>
              <h1 className='text-[31px] leading-[1.08] font-bold tracking-tight text-[#20242a] sm:text-[42px]'>
                {title}
              </h1>
            </div>
          </div>

          <label className='mb-10 grid min-h-[72px] grid-cols-[140px_minmax(0,1fr)] items-center rounded-xl border border-[#dfe3e8] bg-[#fbfbfc] px-[22px] shadow-[0_2px_7px_rgba(25,31,40,0.10)] max-[900px]:grid-cols-1 max-[900px]:gap-2 max-[900px]:p-4'>
            <span className='text-sm font-semibold text-[#5c6066] sm:text-base'>
              {t('Email')}
            </span>
            <input
              type='email'
              value={viewModel?.email ?? ''}
              readOnly
              aria-readonly='true'
              className='min-w-0 border-0 bg-transparent text-base text-[#20242a] outline-none sm:text-lg'
            />
          </label>

          <div className='mb-[22px] flex min-w-0 items-center justify-between gap-4'>
            <h2 className='text-[23px] leading-tight font-bold text-[#20242a] sm:text-[26px]'>
              {t('Payment method')}
            </h2>
            <span className='text-sm font-bold tracking-wide text-[#646a73] uppercase'>
              {viewModel?.currency ?? ''}
            </span>
          </div>

          <div className='relative min-h-[250px] rounded-[14px] border border-[#dfe3e8] bg-white p-5 shadow-[0_8px_22px_rgba(22,30,46,0.06)] sm:p-7'>
            <div
              ref={onCurrencyContainer}
              className={cn('mb-5', !showCurrencySelector && 'hidden')}
              data-slot='stripe-currency-selector'
            />
            <div ref={onPaymentContainer} data-slot='stripe-payment-element' />
            {mounting ? (
              <div
                role='status'
                className='absolute inset-0 flex items-center justify-center rounded-[14px] bg-white/90'
              >
                <Loader2
                  aria-hidden='true'
                  className='size-7 animate-spin text-[#646a73]'
                />
                <span className='sr-only'>{t('Loading payment form...')}</span>
              </div>
            ) : null}
          </div>

          <div className='mt-5 rounded-[14px] border border-[#dfe3e8] bg-[#fbfbfc] p-4 shadow-[0_2px_7px_rgba(25,31,40,0.08)] sm:p-5'>
            <label
              htmlFor='stripe-promotion-code'
              className='mb-2 block text-sm font-semibold text-[#5c6066]'
            >
              {t('Promotion code')}
            </label>
            <div className='flex gap-2 max-[520px]:flex-col'>
              <input
                id='stripe-promotion-code'
                type='text'
                value={promotionCode}
                onChange={(event) => onPromotionCodeChange(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    event.preventDefault()
                    if (!promotionCodeApplied) onApplyPromotionCode()
                  }
                }}
                placeholder={t('Enter promotion code')}
                autoComplete='off'
                disabled={promotionCodeApplied || promotionCodeSubmitting}
                className='min-w-0 flex-1 rounded-lg border border-[#cfd5dc] bg-white px-3 py-2.5 text-sm text-[#20242a] transition outline-none placeholder:text-[#8b919a] focus:border-[#0576d7] focus:ring-4 focus:ring-blue-100 disabled:cursor-not-allowed disabled:bg-[#f1f2f4]'
              />
              {promotionCodeApplied ? (
                <button
                  type='button'
                  onClick={onRemovePromotionCode}
                  disabled={promotionCodeSubmitting}
                  className='rounded-lg border border-[#cfd5dc] bg-white px-4 py-2.5 text-sm font-semibold text-[#4b5057] transition hover:bg-[#f1f2f4] disabled:cursor-not-allowed disabled:opacity-50'
                >
                  {t('Remove')}
                </button>
              ) : (
                <button
                  type='button'
                  onClick={onApplyPromotionCode}
                  disabled={
                    !promotionCode.trim() ||
                    promotionCodeSubmitting ||
                    mounting ||
                    submitting
                  }
                  className='rounded-lg bg-[#20242a] px-4 py-2.5 text-sm font-semibold text-white transition hover:bg-[#3b4149] disabled:cursor-not-allowed disabled:opacity-50'
                >
                  {promotionCodeSubmitting ? t('Applying...') : t('Apply')}
                </button>
              )}
            </div>
            {promotionCodeError ? (
              <p role='alert' className='mt-2 text-sm text-red-700'>
                {promotionCodeError}
              </p>
            ) : null}
          </div>

          {error ? (
            <p
              role='alert'
              className='mt-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700'
            >
              {error}
            </p>
          ) : null}
        </section>

        <aside
          data-slot='stripe-checkout-summary-pane'
          aria-label={t('Order summary')}
          className='flex min-w-0 flex-col gap-6 bg-[linear-gradient(180deg,rgba(247,248,250,0.90),rgba(255,255,255,0.96))] p-14 max-[900px]:gap-5 max-[900px]:px-6 max-[900px]:py-9 max-[520px]:px-[18px] max-[520px]:py-7'
        >
          <div className='pr-14 max-[900px]:pr-0'>
            <p className='mb-3 text-base font-bold text-[#626872] sm:text-lg'>
              {viewModel?.productName ?? 'Flatkey'}
            </p>
            <p className='text-[44px] leading-[0.98] font-extrabold tracking-tight break-words text-[#2a2e33] sm:text-[56px]'>
              {viewModel?.primaryAmount ?? '—'}
            </p>
            <p className='mt-3 text-base font-medium text-[#646a73] sm:text-xl'>
              {viewModel?.productDescription || description}
            </p>
          </div>

          {viewModel?.topupSummary ? (
            <div className='flex items-center gap-3 rounded-xl bg-violet-50 px-4 py-3 text-violet-900'>
              <Gift aria-hidden='true' className='size-5 shrink-0' />
              <span className='text-sm font-semibold'>
                {t('Pay ${{pay}}, get ${{total}} — includes ${{bonus}} bonus', {
                  pay: viewModel.topupSummary.pay_amount,
                  total: viewModel.topupSummary.credit_amount,
                  bonus: viewModel.topupSummary.bonus_amount,
                })}
              </span>
            </div>
          ) : null}

          <div className='rounded-[14px] border border-[#dfe3e8] bg-white p-6 shadow-[0_8px_22px_rgba(22,30,46,0.06)]'>
            {viewModel?.summaryLines.map((line) => (
              <div
                key={line.key}
                className='mb-4 flex justify-between gap-4 text-base text-[#525861] last:mb-0'
              >
                <span>{getSummaryLabel(line.key, t)}</span>
                <strong className='font-extrabold text-[#4b5057]'>
                  {getSummaryAmountPrefix(line.key)}
                  {line.amount}
                </strong>
              </div>
            ))}
            <div className='my-5 h-px bg-[#dfe3e8]' />
            <div className='flex justify-between gap-4 text-xl font-extrabold text-[#20242a]'>
              <span>{t('Total due')}</span>
              <strong>{viewModel?.totalAmount ?? '—'}</strong>
            </div>
          </div>

          <button
            type='button'
            disabled={!canSubmit}
            onClick={onConfirm}
            className='inline-flex min-h-[66px] w-full items-center justify-center gap-3 rounded-lg bg-[#0576d7] px-5 text-lg font-bold text-white shadow-[0_8px_16px_rgba(5,118,215,0.26)] transition hover:-translate-y-px hover:bg-[#0469c0] hover:shadow-[0_10px_20px_rgba(5,118,215,0.30)] focus-visible:ring-4 focus-visible:ring-blue-200 focus-visible:outline-none disabled:translate-y-0 disabled:cursor-not-allowed disabled:opacity-50 sm:text-[22px]'
          >
            {submitting ? (
              <Loader2 aria-hidden='true' className='size-5 animate-spin' />
            ) : (
              <LockKeyhole aria-hidden='true' className='size-5' />
            )}
            {buttonLabel}
          </button>

          <div className='flex items-center gap-2.5 text-sm text-[#5e6570]'>
            <ShieldCheck
              aria-hidden='true'
              className='size-5 shrink-0 text-[#26a899]'
            />
            <span>{t('Encrypted payment powered by Stripe')}</span>
          </div>

          <footer className='mt-auto flex flex-wrap items-center justify-center gap-x-[18px] gap-y-2 pt-5 text-sm text-[#4d535b] sm:text-base'>
            <span>{t('Powered by Stripe')}</span>
            <a className='hover:text-foreground' href='/terms'>
              {t('Terms')}
            </a>
            <a className='hover:text-foreground' href='/privacy'>
              {t('Privacy')}
            </a>
          </footer>
        </aside>
      </div>
    </div>
  )
}
