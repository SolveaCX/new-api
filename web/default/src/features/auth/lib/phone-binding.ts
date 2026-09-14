import type { AuthUser } from '@/stores/auth-store'

type PhoneBindingUser = Pick<AuthUser, 'phone_verification_required'>

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
