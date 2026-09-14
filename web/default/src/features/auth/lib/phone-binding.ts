import { isPlgUser, type AuthUser } from '@/stores/auth-store'

type PhoneBindingUser = Pick<
  AuthUser,
  'group' | 'phone_number' | 'phone_verified_at'
>

export function shouldRequirePhoneBinding(
  user: PhoneBindingUser | null | undefined,
  smsVerificationEnabled = true
): boolean {
  return (
    smsVerificationEnabled &&
    isPlgUser(user?.group) &&
    !(user?.phone_verified_at ?? 0)
  )
}
