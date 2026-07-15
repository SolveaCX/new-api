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
import { useEffect, useRef, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { ArrowLeft, CircleAlert, CircleCheck, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { exchangeRegistrationEmailToken } from '../../api'
import { AuthLayout } from '../../auth-layout'
import {
  getRegistrationEmailToken,
  isRegistrationEmailVerified,
} from '../../lib/registration-email-verification'

export type EmailVerificationScreenState =
  | 'verifying'
  | 'verified'
  | 'unavailable'

type ResolveRegistrationEmailVerificationDependencies = {
  clearFragment: () => void
  exchangeToken: (token: string) => Promise<unknown>
}

type EmailVerificationStatusContentProps = {
  state: EmailVerificationScreenState
}

export async function resolveRegistrationEmailVerification(
  hash: string,
  dependencies: ResolveRegistrationEmailVerificationDependencies
): Promise<EmailVerificationScreenState> {
  const token = getRegistrationEmailToken(hash)
  if (!token) return 'unavailable'

  dependencies.clearFragment()
  try {
    const response = await dependencies.exchangeToken(token)
    return isRegistrationEmailVerified(response) ? 'verified' : 'unavailable'
  } catch {
    return 'unavailable'
  }
}

export function startRegistrationEmailVerificationEffect(
  hash: string,
  dependencies: ResolveRegistrationEmailVerificationDependencies,
  onResolved: (state: EmailVerificationScreenState) => void
): () => void {
  let active = true
  void resolveRegistrationEmailVerification(hash, dependencies).then(
    (nextState) => {
      if (active) onResolved(nextState)
    }
  )
  return () => {
    active = false
  }
}

function clearRegistrationEmailVerificationFragment() {
  if (typeof window === 'undefined') return
  const cleanUrl = `${window.location.pathname}${window.location.search}`
  window.history.replaceState(window.history.state, '', cleanUrl)
}

export function EmailVerificationStatusContent(
  props: EmailVerificationStatusContentProps
) {
  const { t } = useTranslation()

  if (props.state === 'verifying') {
    return (
      <div aria-live='polite' className='space-y-3 text-center'>
        <Loader2
          aria-hidden='true'
          className='mx-auto size-12 animate-spin text-violet-600 dark:text-violet-300'
        />
        <h2 className='text-xl font-semibold tracking-normal'>
          {t('Verifying your email')}
        </h2>
        <p className='text-muted-foreground text-sm leading-6'>
          {t('Please wait while we confirm this verification link.')}
        </p>
      </div>
    )
  }

  if (props.state === 'verified') {
    return (
      <div aria-live='polite' className='space-y-3 text-center'>
        <CircleCheck
          aria-hidden='true'
          className='mx-auto size-12 text-emerald-600 dark:text-emerald-400'
        />
        <h2 className='text-xl font-semibold tracking-normal'>
          {t('Email verified')}
        </h2>
        <p className='text-muted-foreground text-sm leading-6'>
          {t('Your email is verified. Return to registration to continue.')}
        </p>
      </div>
    )
  }

  return (
    <div aria-live='polite' className='space-y-3 text-center'>
      <CircleAlert
        aria-hidden='true'
        className='text-destructive mx-auto size-12'
      />
      <h2 className='text-xl font-semibold tracking-normal'>
        {t('Verification link unavailable')}
      </h2>
      <p className='text-muted-foreground text-sm leading-6'>
        {t(
          'This verification link is invalid or expired. Use the code in your email or request a new message.'
        )}
      </p>
    </div>
  )
}

export function EmailVerificationScreen() {
  const { t } = useTranslation()
  const initialHash = useRef(
    typeof window === 'undefined' ? '' : window.location.hash
  )
  const [state, setState] = useState<EmailVerificationScreenState>(() =>
    getRegistrationEmailToken(initialHash.current) ? 'verifying' : 'unavailable'
  )

  useEffect(() => {
    return startRegistrationEmailVerificationEffect(
      initialHash.current,
      {
        clearFragment: clearRegistrationEmailVerificationFragment,
        exchangeToken: exchangeRegistrationEmailToken,
      },
      setState
    )
  }, [])

  return (
    <AuthLayout>
      <main className='mx-auto flex min-h-[300px] w-full max-w-sm flex-col items-center justify-center gap-7 py-6'>
        <EmailVerificationStatusContent state={state} />
        {state !== 'verifying' && (
          <Button
            size='lg'
            className='w-full sm:w-auto'
            render={<Link to='/sign-up' />}
          >
            <ArrowLeft aria-hidden='true' />
            {t('Back to registration')}
          </Button>
        )}
      </main>
    </AuthLayout>
  )
}
