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
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { StripeCheckoutSession } from '@stripe/stripe-js'
import { X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from '@/components/ui/dialog'
import { updateStripeCheckoutDiscount } from '../../api'
import {
  mountStripeCheckoutElements,
  type MountedStripeCheckoutElements,
} from '../../lib/stripe-checkout-elements'
import { recoverStripeCheckoutMountFailure } from '../../lib/stripe-checkout-recovery'
import { buildStripeCheckoutViewModel } from '../../lib/stripe-checkout-view-model'
import type {
  ApiResponse,
  StripeCheckoutDiscountState,
  StripeCheckoutRevisionData,
  StripeTopupSummary,
} from '../../types'
import { StripeCheckoutLayout } from './stripe-checkout-layout'
import {
  StripePromotionCodeControl,
  type StripePromotionCodeBusyAction,
} from './stripe-promotion-code-control'

export interface StripeCheckoutDialogSession {
  clientSecret: string
  publishableKey: string
  summary: StripeTopupSummary | null
  title?: string
  description?: string
  fallbackUrl?: string
  checkoutContext?: string
  checkoutRevision?: number
  discountState?: StripeCheckoutDiscountState
}

interface StripeCheckoutDialogProps {
  session: StripeCheckoutDialogSession | null
  onOpenChange: (open: boolean) => void
}

export function StripeCheckoutDialog(props: StripeCheckoutDialogProps) {
  const { t } = useTranslation()
  const title = props.session?.title ?? t('Confirm Payment')
  const description =
    props.session?.description ?? t('Payment is processed securely by Stripe.')

  return (
    <Dialog open={Boolean(props.session)} onOpenChange={props.onOpenChange}>
      <DialogContent
        showCloseButton={false}
        className='max-h-[calc(100dvh-2rem)] w-[calc(100vw-2rem)] max-w-[1120px] gap-0 overflow-y-auto rounded-[24px] p-0 ring-1 ring-[#dfe3e8] max-[520px]:top-0 max-[520px]:left-0 max-[520px]:h-dvh max-[520px]:max-h-dvh max-[520px]:w-screen max-[520px]:max-w-none max-[520px]:translate-x-0 max-[520px]:translate-y-0 max-[520px]:rounded-none sm:max-w-[1120px]'
      >
        <DialogTitle className='sr-only'>{title}</DialogTitle>
        <DialogDescription className='sr-only'>{description}</DialogDescription>
        {props.session ? (
          <StripeCheckoutFrame
            key={props.session.clientSecret}
            session={props.session}
            title={title}
            description={description}
            onOpenChange={props.onOpenChange}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  )
}

function StripeCheckoutFrame(props: {
  session: StripeCheckoutDialogSession
  title: string
  description: string
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const paymentContainerRef = useRef<HTMLDivElement | null>(null)
  const currencyContainerRef = useRef<HTMLDivElement | null>(null)
  const mountedRef = useRef<MountedStripeCheckoutElements | null>(null)
  const requestGenerationRef = useRef(0)
  const mutationInFlightRef = useRef(false)
  const [current, setCurrent] = useState(() => props.session)
  const [checkoutSession, setCheckoutSession] =
    useState<StripeCheckoutSession | null>(null)
  const [promotionCode, setPromotionCode] = useState('')
  const [promotionMessage, setPromotionMessage] = useState<{
    kind: 'success' | 'error'
    text: string
  } | null>(null)
  const [mounting, setMounting] = useState(true)
  const [switching, setSwitching] = useState(false)
  const [busyAction, setBusyAction] =
    useState<StripePromotionCodeBusyAction>(null)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const sessionClientSecret = current.clientSecret
  const sessionPublishableKey = current.publishableKey
  const sessionFallbackUrl = current.fallbackUrl
  const onOpenChange = props.onOpenChange
  const discountState = current.discountState ?? { source: 'none' as const }
  const hasRevisionContract = Boolean(
    current.checkoutContext &&
    typeof current.checkoutRevision === 'number' &&
    current.checkoutRevision > 0 &&
    current.discountState
  )
  const setPaymentContainer = useCallback((element: HTMLDivElement | null) => {
    paymentContainerRef.current = element
  }, [])
  const setCurrencyContainer = useCallback((element: HTMLDivElement | null) => {
    currencyContainerRef.current = element
  }, [])

  useEffect(() => {
    let cancelled = false
    let mounted: MountedStripeCheckoutElements | null = null

    const mount = async () => {
      if (!paymentContainerRef.current) return
      setMounting(true)
      setCheckoutSession(null)
      try {
        mounted = await mountStripeCheckoutElements({
          clientSecret: sessionClientSecret,
          publishableKey: sessionPublishableKey,
          paymentContainer: paymentContainerRef.current,
          currencyContainer: currencyContainerRef.current,
          onSessionChange: (nextSession) => {
            if (!cancelled) setCheckoutSession(nextSession)
          },
        })
        if (cancelled) {
          mounted.destroy()
          return
        }
        mountedRef.current = mounted
        setSwitching(false)
        setBusyAction(null)
      } catch (_mountError) {
        if (cancelled) return
        setSwitching(false)
        setBusyAction(null)
        recoverStripeCheckoutMountFailure({
          fallbackUrl: sessionFallbackUrl,
          navigate: (url) => {
            window.location.assign(url)
            toast.success(t('Redirecting to payment page...'))
          },
          notifyFailure: () =>
            toast.error(t('Failed to load the payment form, please try again')),
          close: () => onOpenChange(false),
        })
      } finally {
        if (!cancelled) setMounting(false)
      }
    }

    void mount()
    return () => {
      cancelled = true
      mountedRef.current = null
      mounted?.destroy()
    }
  }, [
    onOpenChange,
    sessionClientSecret,
    sessionFallbackUrl,
    sessionPublishableKey,
    t,
  ])

  const viewModel = useMemo(
    () =>
      checkoutSession
        ? buildStripeCheckoutViewModel(checkoutSession, current.summary)
        : null,
    [checkoutSession, current.summary]
  )

  const handleConfirm = useCallback(async () => {
    const mounted = mountedRef.current
    if (!mounted || submitting || switching) return
    setSubmitting(true)
    setError(null)
    try {
      const result = await mounted.confirm()
      if (result.type === 'error') {
        setError(result.error.message)
        toast.error(result.error.message)
        return
      }
      setCheckoutSession(result.session)
    } catch (_confirmError) {
      const message = t('Payment confirmation failed, please try again')
      setError(message)
      toast.error(message)
    } finally {
      setSubmitting(false)
    }
  }, [submitting, switching, t])

  const installRevision = useCallback(
    (
      data: StripeCheckoutRevisionData,
      message: { kind: 'success' | 'error'; text: string }
    ) => {
      if (!isCompleteRevisionData(data)) {
        setPromotionMessage({
          kind: 'error',
          text: t('Unable to update the checkout. Please try again.'),
        })
        setSwitching(false)
        setBusyAction(null)
        return
      }
      setSwitching(true)
      setPromotionCode('')
      setPromotionMessage(message)
      setCurrent((previous) => ({
        ...previous,
        clientSecret: data.client_secret,
        publishableKey: data.publishable_key,
        fallbackUrl: data.fallback_url,
        checkoutContext: data.checkout_context,
        checkoutRevision: data.checkout_revision,
        discountState: data.discount_state,
        summary: data.topup_summary ?? null,
      }))
    },
    [t]
  )

  const mutateDiscount = useCallback(
    async (action: 'apply' | 'restore') => {
      if (
        !hasRevisionContract ||
        !current.checkoutContext ||
        !current.checkoutRevision ||
        mutationInFlightRef.current
      ) {
        return
      }
      const trimmedCode = promotionCode.trim()
      if (action === 'apply' && !trimmedCode) return

      const generation = requestGenerationRef.current + 1
      requestGenerationRef.current = generation
      mutationInFlightRef.current = true
      setSwitching(true)
      setBusyAction(action)
      setPromotionMessage({
        kind: 'success',
        text:
          action === 'apply'
            ? t('Applying promotion code...')
            : t('Restoring previous discount...'),
      })

      try {
        const response = await updateStripeCheckoutDiscount(
          action === 'apply'
            ? {
                action: 'apply',
                checkout_context: current.checkoutContext,
                expected_revision: current.checkoutRevision,
                promotion_code: trimmedCode,
                request_id: createCheckoutMutationRequestId(),
              }
            : {
                action: 'restore',
                checkout_context: current.checkoutContext,
                expected_revision: current.checkoutRevision,
                request_id: createCheckoutMutationRequestId(),
              }
        )
        if (generation !== requestGenerationRef.current) return

        if (response.success && response.data) {
          installRevision(response.data, {
            kind: 'success',
            text: getSuccessMessage(action, response.data.discount_state, t),
          })
          return
        }

        if (
          response.message === 'checkout_revision_conflict' &&
          response.data &&
          isCompleteRevisionData(response.data)
        ) {
          installRevision(response.data, {
            kind: 'error',
            text: t(
              'Checkout changed in another request. The latest checkout was restored.'
            ),
          })
          return
        }

        setPromotionMessage({
          kind: 'error',
          text: getDiscountErrorMessage(response, t),
        })
        setSwitching(false)
        setBusyAction(null)
      } catch (_error) {
        if (generation !== requestGenerationRef.current) return
        setPromotionMessage({
          kind: 'error',
          text: t('Unable to update the checkout. Please try again.'),
        })
        setSwitching(false)
        setBusyAction(null)
      } finally {
        if (generation === requestGenerationRef.current) {
          mutationInFlightRef.current = false
        }
      }
    },
    [
      current.checkoutContext,
      current.checkoutRevision,
      hasRevisionContract,
      installRevision,
      promotionCode,
      t,
    ]
  )

  return (
    <StripeCheckoutLayout
      title={props.title}
      description={props.description}
      viewModel={viewModel}
      onPaymentContainer={setPaymentContainer}
      onCurrencyContainer={setCurrencyContainer}
      showCurrencySelector={(checkoutSession?.currencyOptions?.length ?? 0) > 1}
      mounting={mounting}
      submitting={submitting || switching}
      error={error}
      onConfirm={() => void handleConfirm()}
      promotionControl={
        hasRevisionContract ? (
          <StripePromotionCodeControl
            value={promotionCode}
            discountState={discountState}
            busy={switching}
            busyAction={busyAction}
            message={promotionMessage}
            onValueChange={setPromotionCode}
            onApply={() => void mutateDiscount('apply')}
            onRemove={() => void mutateDiscount('restore')}
          />
        ) : undefined
      }
      closeControl={
        <DialogClose
          render={
            <button
              type='button'
              aria-label={t('Close payment')}
              className='absolute top-[22px] right-[22px] z-10 grid size-[52px] place-items-center rounded-full border-[3px] border-violet-400 bg-[#fbfbff] text-[#05070a] shadow-[0_0_0_6px_rgba(156,101,255,0.20)] transition hover:scale-105 focus-visible:ring-4 focus-visible:ring-violet-200 focus-visible:outline-none max-[900px]:top-4 max-[900px]:right-4 max-[900px]:size-[46px]'
            />
          }
        >
          <X aria-hidden='true' className='size-7 stroke-[2.5]' />
        </DialogClose>
      }
    />
  )
}

function isCompleteRevisionData(
  data: StripeCheckoutRevisionData
): data is Required<
  Pick<
    StripeCheckoutRevisionData,
    | 'client_secret'
    | 'publishable_key'
    | 'checkout_context'
    | 'checkout_revision'
    | 'discount_state'
  >
> &
  StripeCheckoutRevisionData {
  return Boolean(
    data.client_secret &&
    data.publishable_key &&
    data.checkout_context &&
    typeof data.checkout_revision === 'number' &&
    data.checkout_revision > 0 &&
    data.discount_state
  )
}

function createCheckoutMutationRequestId(): string {
  if (
    typeof crypto !== 'undefined' &&
    typeof crypto.randomUUID === 'function'
  ) {
    return crypto.randomUUID()
  }
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`
}

function getSuccessMessage(
  action: 'apply' | 'restore',
  discountState: StripeCheckoutDiscountState | undefined,
  t: (key: string) => string
): string {
  if (action === 'restore') return t('Previous discount restored.')
  if (discountState?.source === 'manual' && discountState.replaced_source) {
    return t('Promotion code applied. Previous discount replaced.')
  }
  return t('Promotion code applied.')
}

function getDiscountErrorMessage(
  response: ApiResponse<StripeCheckoutRevisionData>,
  t: (key: string) => string
): string {
  if (response.message === 'promotion_code_invalid') {
    return t('This promotion code is invalid.')
  }
  if (response.message === 'promotion_code_ineligible') {
    return t('This promotion code is not eligible for this purchase.')
  }
  if (response.message === 'promotion_code_ambiguous') {
    return t('Multiple promotion codes match. Contact support.')
  }
  if (response.message === 'checkout_revision_conflict') {
    return t(
      'Checkout changed in another request. The latest checkout was restored.'
    )
  }
  return t('Unable to update the checkout. Please try again.')
}
