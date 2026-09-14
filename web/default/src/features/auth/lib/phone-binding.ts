import type { AuthUser } from '@/stores/auth-store'
import type { PhoneVerificationStatus } from '../types'

type PhoneBindingUser = Pick<AuthUser, 'phone_verification_required' | 'role'>

/**
 * Whether the blocking phone-binding dialog must be shown.
 *
 * The decision is made by the backend (`/api/user/self` →
 * `phone_verification_required`): only PLG accounts created at/after the
 * rollout start that have not verified a phone yet. Older accounts are
 * exempt. Keeping the rule server-side guarantees the console dialog and the
 * API gate can never disagree. A missing field is an unknown state and never
 * opens the dialog.
 */
export function shouldRequirePhoneBinding(
  user: PhoneBindingUser | null | undefined,
  smsVerificationEnabled = true
): boolean {
  return smsVerificationEnabled && user?.phone_verification_required === true
}

/**
 * Whether an optional phone-binding prompt should be shown. Unlike the
 * blocking gate, this applies to every unbound non-admin account.
 */
export function shouldSuggestPhoneBinding(
  user: PhoneBindingUser | null | undefined,
  status: PhoneVerificationStatus | null | undefined
): boolean {
  return (
    typeof user?.role === 'number' &&
    user.role < 10 &&
    status?.sms_verification_enabled === true &&
    status.phone_bound === false
  )
}
