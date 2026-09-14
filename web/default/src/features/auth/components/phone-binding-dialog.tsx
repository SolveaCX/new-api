import { useState } from 'react'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from '@/components/ui/input-group'
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select'
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
      title={
        props.required
          ? t('Phone number verification required')
          : t('Bind phone number')
      }
      description={
        props.required
          ? t(
              'Please bind and verify your phone number before using the console and API.'
            )
          : t(
              'Bind and verify your phone number to improve account security.'
            )
      }
      showCloseButton={!props.required}
      contentClassName='rounded-2xl sm:max-w-[480px]'
      contentHeight='auto'
      headerClassName='gap-1.5'
      titleClassName='text-xl'
      descriptionClassName='max-w-sm leading-5'
      bodyClassName='flex flex-col gap-5'
      footerClassName='border-border/70 border-t bg-muted/30'
      footer={
        <>
          {!props.required && (
            <Button
              type='button'
              variant='outline'
              className='min-w-24 rounded-lg'
              onClick={() => handleOpenChange(false)}
              disabled={binding || sendingCode}
            >
              {t('Cancel')}
            </Button>
          )}
          <Button
            type='button'
            className='min-w-40 rounded-lg'
            onClick={handleBind}
            disabled={
              binding || sendingCode || !phoneNumber || !verificationCode
            }
          >
            {binding && <Loader2 data-icon='inline-start' className='animate-spin' />}
            {binding ? t('Binding...') : t('Bind phone number')}
          </Button>
        </>
      }
    >
      <FieldGroup className='gap-4 py-2'>
        <Field>
          <FieldLabel htmlFor='phone-number'>{t('Phone number')}</FieldLabel>
          <div className='flex gap-2'>
            <NativeSelect
              id='phone-country-code'
              aria-label={t('Country code')}
              className='w-28 shrink-0 [&_[data-slot=native-select]]:h-10'
              value={countryCode}
              onChange={(event) => setCountryCode(event.target.value)}
              disabled={binding || sendingCode}
            >
              {PHONE_COUNTRIES.map(([country, code, flag]) => (
                <NativeSelectOption key={`${country}-${code}`} value={code}>
                  {flag} {code}
                </NativeSelectOption>
              ))}
            </NativeSelect>
            <Input
              id='phone-number'
              className='h-10'
              inputMode='tel'
              autoComplete='tel-national'
              value={phoneNumber}
              onChange={(event) => setPhoneNumber(event.target.value)}
              placeholder={t('Enter phone number')}
              disabled={binding || sendingCode}
            />
          </div>
        </Field>

        <Field>
          <FieldLabel htmlFor='phone-verification-code'>
            {t('SMS verification code')}
          </FieldLabel>
          <InputGroup className='h-10'>
            <InputGroupInput
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
            <InputGroupAddon align='inline-end'>
              <InputGroupButton
                variant='secondary'
                size='sm'
                onClick={handleSendCode}
                disabled={
                  binding || sendingCode || secondsLeft > 0 || !phoneNumber
                }
              >
                {secondsLeft > 0 ? `${secondsLeft}s` : t('Send code')}
              </InputGroupButton>
            </InputGroupAddon>
          </InputGroup>
        </Field>

        {isTurnstileEnabled && (
          <div className='bg-muted/30 flex min-h-[76px] items-center justify-center overflow-hidden rounded-xl border p-1.5'>
            <Turnstile
              siteKey={turnstileSiteKey}
              onVerify={setTurnstileToken}
              onExpire={() => setTurnstileToken('')}
            />
          </div>
        )}
      </FieldGroup>
    </Dialog>
  )
}
