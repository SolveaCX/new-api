import type { AuthUser } from '@/stores/auth-store'
import type { PhoneVerificationStatus } from '../types'

type PhoneBindingUser = Pick<
  AuthUser,
  'phone_verification_required' | 'role' | 'group'
>

/**
 * The only group the phone-verification rollout covers, mirroring
 * `plgUserGroup` in `model/user.go`. Enterprise and every other group are
 * outside it and must not be prompted at all.
 */
const PLG_GROUP = 'plg'

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
 * blocking gate, this reaches unbound PLG accounts the rollout exempts by
 * creation date, so it must stay dismissible: `dismissed` carries the per-user
 * flag from `phone-binding-dismissal.ts`.
 *
 * The group check matches the backend rule, which only ever subjects PLG
 * accounts to phone verification — enterprise and other groups are never
 * prompted, not even with a dismissible dialog. A missing group is an unknown
 * state and never prompts.
 *
 * This only ever suppresses the *suggestion*. An account the backend gates
 * still opens the blocking dialog through `shouldRequirePhoneBinding`, which
 * takes no dismissal input, so a stored flag can never unblock it.
 */
export function shouldSuggestPhoneBinding(
  user: PhoneBindingUser | null | undefined,
  status: PhoneVerificationStatus | null | undefined,
  dismissed = false
): boolean {
  return (
    !dismissed &&
    user?.group === PLG_GROUP &&
    typeof user.role === 'number' &&
    user.role < 10 &&
    status?.sms_verification_enabled === true &&
    status.phone_bound === false
  )
}
