import { isPlgUser, type AuthUser } from '@/stores/auth-store'

type PhoneBindingUser = Pick<
  AuthUser,
  'group' | 'phone_number' | 'phone_verified_at' | 'role'
>

const ADMIN_ROLE = 10

export function shouldRequirePhoneBinding(
  user: PhoneBindingUser | null | undefined,
  smsVerificationEnabled = true
): boolean {
  return (
    smsVerificationEnabled &&
    (user?.role ?? ADMIN_ROLE) < ADMIN_ROLE &&
    isPlgUser(user?.group) &&
    !(user?.phone_verified_at ?? 0)
  )
}
