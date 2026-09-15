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
import { useQuery } from '@tanstack/react-query'
import { LogOut } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { getCookie } from '@/lib/cookies'
import { cn } from '@/lib/utils'
import { LayoutProvider } from '@/context/layout-provider'
import { useStatus } from '@/hooks/use-status'
import { Button } from '@/components/ui/button'
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar'
import { AnimatedOutlet } from '@/components/page-transition'
import { SkipToMain } from '@/components/skip-to-main'
import { getPhoneVerificationStatus } from '@/features/auth/api'
import { PhoneBindingDialog } from '@/features/auth/components/phone-binding-dialog'
import {
  shouldRequirePhoneBinding,
  shouldSuggestPhoneBinding,
} from '@/features/auth/lib/phone-binding'
import {
  hasDismissedPhoneBindingSuggestion,
  markPhoneBindingSuggestionDismissed,
} from '@/features/auth/lib/phone-binding-dismissal'
import { Onboarding } from '@/features/onboarding'
import { exitImpersonation as exitImpersonationRequest } from '@/features/users/api'
import { AppHeader } from './app-header'
import { AppSidebar } from './app-sidebar'

type AuthenticatedLayoutProps = {
  children?: React.ReactNode
}

export function AuthenticatedLayout(props: AuthenticatedLayoutProps) {
  const { t } = useTranslation()
  const defaultOpen = getCookie('sidebar_state') !== 'false'
  const user = useAuthStore((state) => state.auth.user)
  const setUser = useAuthStore((state) => state.auth.setUser)
  const { status } = useStatus()
  const phoneBindingRequired = shouldRequirePhoneBinding(
    user,
    status?.sms_verification === true
  )
  const phoneStatusQuery = useQuery({
    queryKey: ['auth', 'phone-verification-status', user?.id],
    queryFn: getPhoneVerificationStatus,
    enabled:
      Boolean(user) &&
      (user?.role ?? 10) < 10 &&
      status?.sms_verification === true,
    staleTime: 30_000,
    retry: false,
  })
  const phoneStatus = phoneStatusQuery.data?.success
    ? phoneStatusQuery.data.data
    : undefined
  // The stored dismissal is keyed by user id, so it is re-read whenever the
  // account changes (login, logout, admin impersonation) rather than kept as a
  // stale mount-time snapshot. Resetting state during render is React's
  // documented alternative to a reset effect, and avoids a frame where the
  // previous account's dismissal hides the new account's prompt.
  const [dismissalState, setDismissalState] = useState(() => ({
    userId: user?.id,
    dismissed: hasDismissedPhoneBindingSuggestion(user?.id),
  }))
  if (dismissalState.userId !== user?.id) {
    setDismissalState({
      userId: user?.id,
      dismissed: hasDismissedPhoneBindingSuggestion(user?.id),
    })
  }
  const phoneBindingDismissed =
    dismissalState.userId === user?.id && dismissalState.dismissed
  const phoneBindingSuggested = shouldSuggestPhoneBinding(
    user,
    phoneStatus,
    phoneBindingDismissed
  )
  // Only the backend gate blocks. The suggestion opens the same dialog in its
  // dismissible form, so exempt accounts can close it and keep working.
  const showPhoneBinding = phoneBindingRequired || phoneBindingSuggested
  const exitImpersonation = async () => {
    const result = await exitImpersonationRequest()
    if (!result.success || !result.data) return
    // The restored admin ID must be persisted before navigating, for the same
    // New-Api-User/session consistency check used when entering user view.
    setUser(result.data)
    window.location.assign('/users')
  }

  return (
    <LayoutProvider>
      <SidebarProvider defaultOpen={defaultOpen} className='flex-col'>
        {user?.impersonating && (
          <div className='flex h-10 shrink-0 items-center justify-center gap-3 bg-amber-500 px-4 text-sm font-medium text-black'>
            <span>
              {t(
                'Currently viewing as {{username}} (administrator {{administrator}})',
                {
                  username: user.username,
                  administrator: user.impersonator_username,
                }
              )}
            </span>
            <Button
              size='sm'
              variant='outline'
              className='h-7 border-black/30 bg-transparent text-black hover:bg-black/10'
              onClick={exitImpersonation}
            >
              <LogOut className='mr-1 h-3.5 w-3.5' />
              {t('Return to administrator')}
            </Button>
          </div>
        )}
        <SkipToMain />
        <AppHeader />
        <div className='flex min-h-0 w-full flex-1'>
          <AppSidebar />
          <SidebarInset
            className={cn(
              '@container/content',
              'h-[calc(100svh-var(--app-header-height,0px))]',
              'min-h-0 overflow-hidden',
              'peer-data-[variant=inset]:h-[calc(100svh-var(--app-header-height,0px)-(var(--spacing)*4))]'
            )}
          >
            {/* Scroll container for the routed page; min-h-0 keeps the inner Main's
                  flex-1/overflow working so the page (not the layout) owns scrolling. */}
            <div className='flex min-h-0 flex-1 flex-col overflow-hidden'>
              {props.children ?? <AnimatedOutlet />}
            </div>
          </SidebarInset>
        </div>
      </SidebarProvider>
      <Onboarding />
      <PhoneBindingDialog
        open={showPhoneBinding}
        required={phoneBindingRequired}
        onOpenChange={(open) => {
          // Persist the dismissal so the suggestion stays closed across
          // reloads. Accounts the backend requires to bind are never
          // dismissible, so their close requests are ignored here just as the
          // dialog itself ignores them.
          if (open || phoneBindingRequired) return
          markPhoneBindingSuggestionDismissed(user?.id)
          setDismissalState({ userId: user?.id, dismissed: true })
        }}
        onSuccess={(phoneNumber, phoneVerifiedAt) => {
          void phoneStatusQuery.refetch()
          setUser((currentUser) =>
            currentUser
              ? {
                  ...currentUser,
                  phone_number: phoneNumber,
                  phone_verified_at: phoneVerifiedAt,
                  phone_verification_required: false,
                }
              : currentUser
          )
        }}
      />
    </LayoutProvider>
  )
}
