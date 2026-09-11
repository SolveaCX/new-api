import { useState } from 'react'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog } from '@/components/dialog'
import { Turnstile } from '@/components/turnstile'
import { bindPhone, sendPhoneVerification } from '../api'
import { useTurnstile } from '../hooks/use-turnstile'
import {
  buildPhoneNumber,
  PHONE_COUNTRIES,
  SMS_VERIFICATION_COUNTDOWN,
} from '../sign-up/lib/phone-verification'

interface PhoneBindingDialogProps {
  open: boolean
  required?: boolean
  onOpenChange: (open: boolean) => void
  onSuccess: (phoneNumber: string, phoneVerifiedAt: number) => void
}

export function PhoneBindingDialog(props: PhoneBindingDialogProps) {
  const { t } = useTranslation()
  const [countryCode, setCountryCode] = useState('+86')
  const [phoneNumber, setPhoneNumber] = useState('')
  const [verificationCode, setVerificationCode] = useState('')
  const [secondsLeft, setSecondsLeft] = useState(0)
  const [sendingCode, setSendingCode] = useState(false)
  const [binding, setBinding] = useState(false)
  const {
    isTurnstileEnabled,
    turnstileSiteKey,
    turnstileToken,
    setTurnstileToken,
    validateTurnstile,
  } = useTurnstile()

  const handleSendCode = async () => {
    let fullPhone: string
    try {
      fullPhone = buildPhoneNumber(countryCode, phoneNumber)
    } catch {
      toast.error(t('Please enter a valid phone number'))
      return
    }
    if (!validateTurnstile()) return

    setSendingCode(true)
    try {
      const response = await sendPhoneVerification(fullPhone, turnstileToken)
      if (!response.success) {
        toast.error(
          response.message || t('Failed to send SMS verification code')
        )
        return
      }
      setSecondsLeft(SMS_VERIFICATION_COUNTDOWN)
      const timer = window.setInterval(() => {
        setSecondsLeft((current) => {
          if (current <= 1) {
            window.clearInterval(timer)
            return 0
          }
          return current - 1
        })
      }, 1000)
      toast.success(t('SMS verification code sent'))
    } catch {
      // The shared API interceptor presents the server error.
    } finally {
      setSendingCode(false)
    }
  }

  const handleBind = async () => {
    let fullPhone: string
    try {
      fullPhone = buildPhoneNumber(countryCode, phoneNumber)
    } catch {
      toast.error(t('Please enter a valid phone number'))
      return
    }
    if (!verificationCode.trim()) {
      toast.error(t('Please enter the SMS verification code'))
      return
    }

    setBinding(true)
    try {
      const response = await bindPhone(fullPhone, verificationCode.trim())
      if (!response.success || !response.data) {
        toast.error(response.message || t('Failed to bind phone number'))
        return
      }
      toast.success(t('Phone number bound successfully'))
      props.onSuccess(
        response.data.phone_number,
        response.data.phone_verified_at
      )
      props.onOpenChange(false)
      setPhoneNumber('')
      setVerificationCode('')
      setSecondsLeft(0)
    } catch {
      // The shared API interceptor presents the server error.
    } finally {
      setBinding(false)
    }
  }

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen && props.required) return
    if (!nextOpen && !binding && !sendingCode) {
      setPhoneNumber('')
      setVerificationCode('')
      setSecondsLeft(0)
    }
    props.onOpenChange(nextOpen)
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={handleOpenChange}
      title={t('Phone number verification required')}
      description={t(
        'Please bind and verify your phone number before using the console and API.'
      )}
      showCloseButton={!props.required}
      contentClassName='sm:max-w-md'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          {!props.required && (
            <Button
              type='button'
              variant='outline'
              onClick={() => handleOpenChange(false)}
              disabled={binding || sendingCode}
            >
              {t('Cancel')}
            </Button>
          )}
          <Button
            type='button'
            onClick={handleBind}
            disabled={
              binding || sendingCode || !phoneNumber || !verificationCode
            }
          >
            {binding && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
            {binding ? t('Binding...') : t('Bind phone number')}
          </Button>
        </>
      }
    >
      <div className='space-y-4 py-4'>
        <div className='space-y-2'>
          <Label htmlFor='phone-number'>{t('Phone number')}</Label>
          <div className='flex gap-2'>
            <select
              id='phone-country-code'
              aria-label={t('Country code')}
              className='border-input bg-background h-9 w-28 rounded-md border px-2 text-sm'
              value={countryCode}
              onChange={(event) => setCountryCode(event.target.value)}
              disabled={binding || sendingCode}
            >
              {PHONE_COUNTRIES.map(([country, code, flag]) => (
                <option key={`${country}-${code}`} value={code}>
                  {flag} {code}
                </option>
              ))}
            </select>
            <Input
              id='phone-number'
              value={phoneNumber}
              onChange={(event) => setPhoneNumber(event.target.value)}
              placeholder={t('Enter phone number')}
              disabled={binding || sendingCode}
            />
          </div>
        </div>

        <div className='space-y-2'>
          <Label htmlFor='phone-verification-code'>
            {t('SMS verification code')}
          </Label>
          <div className='flex gap-2'>
            <Input
              id='phone-verification-code'
              value={verificationCode}
              inputMode='numeric'
              autoComplete='one-time-code'
              maxLength={6}
              onChange={(event) =>
                setVerificationCode(event.target.value.replace(/\D/g, ''))
              }
              placeholder={t('Please enter the SMS verification code')}
              disabled={binding}
            />
            <Button
              type='button'
              variant='outline'
              className='shrink-0'
              onClick={handleSendCode}
              disabled={
                binding || sendingCode || secondsLeft > 0 || !phoneNumber
              }
            >
              {secondsLeft > 0 ? `${secondsLeft}s` : t('Send code')}
            </Button>
          </div>
        </div>

        {isTurnstileEnabled && (
          <Turnstile
            siteKey={turnstileSiteKey}
            onVerify={setTurnstileToken}
            onExpire={() => setTurnstileToken('')}
          />
        )}
      </div>
    </Dialog>
  )
}
