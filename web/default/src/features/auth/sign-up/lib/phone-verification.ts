export const SMS_VERIFICATION_COUNTDOWN = 60

export const PHONE_COUNTRIES = [
  ['US', '+1', '🇺🇸'],
  ['CA', '+1', '🇨🇦'],
  ['GB', '+44', '🇬🇧'],
  ['CN', '+86', '🇨🇳'],
  ['HK', '+852', '🇭🇰'],
  ['MO', '+853', '🇲🇴'],
  ['TW', '+886', '🇹🇼'],
  ['JP', '+81', '🇯🇵'],
  ['KR', '+82', '🇰🇷'],
  ['SG', '+65', '🇸🇬'],
  ['MY', '+60', '🇲🇾'],
  ['TH', '+66', '🇹🇭'],
  ['VN', '+84', '🇻🇳'],
  ['PH', '+63', '🇵🇭'],
  ['ID', '+62', '🇮🇩'],
  ['IN', '+91', '🇮🇳'],
  ['AU', '+61', '🇦🇺'],
  ['NZ', '+64', '🇳🇿'],
  ['DE', '+49', '🇩🇪'],
  ['FR', '+33', '🇫🇷'],
  ['IT', '+39', '🇮🇹'],
  ['ES', '+34', '🇪🇸'],
  ['NL', '+31', '🇳🇱'],
  ['SE', '+46', '🇸🇪'],
  ['CH', '+41', '🇨🇭'],
  ['AT', '+43', '🇦🇹'],
  ['BE', '+32', '🇧🇪'],
  ['IE', '+353', '🇮🇪'],
  ['PT', '+351', '🇵🇹'],
  ['PL', '+48', '🇵🇱'],
  ['TR', '+90', '🇹🇷'],
  ['IL', '+972', '🇮🇱'],
  ['AE', '+971', '🇦🇪'],
  ['SA', '+966', '🇸🇦'],
  ['ZA', '+27', '🇿🇦'],
  ['BR', '+55', '🇧🇷'],
  ['MX', '+52', '🇲🇽'],
  ['AR', '+54', '🇦🇷'],
  ['CL', '+56', '🇨🇱'],
  ['CO', '+57', '🇨🇴'],
  ['PE', '+51', '🇵🇪'],
  ['RU', '+7', '🇷🇺'],
  ['UA', '+380', '🇺🇦'],
] as const

export function buildPhoneNumber(countryCode: string, localNumber: string): string {
  const countryDigits = countryCode.replace(/\D/g, '')
  const localDigits = localNumber.replace(/\D/g, '')
  const digits = `${countryDigits}${localDigits}`
  if (!countryDigits || localDigits.length < 4 || digits.length < 8 || digits.length > 15) {
    throw new Error('Invalid phone number')
  }
  return `+${digits}`
}
