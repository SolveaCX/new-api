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
import { ArrowLeft } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useCountdown } from '@/hooks/use-countdown'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog } from '@/components/dialog'
import { maskPhoneNumber } from '../../lib/format'

interface PhoneBindDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentPhone?: string
  onPreviewChange: (phone: string) => void
}

type PhoneBindStep = 'verify-current' | 'enter-new'

export function PhoneBindDialog(props: PhoneBindDialogProps) {
  const { t } = useTranslation()
  const [step, setStep] = useState<PhoneBindStep>(() =>
    props.currentPhone ? 'verify-current' : 'enter-new'
  )
  const [currentCode, setCurrentCode] = useState('')
  const [phone, setPhone] = useState('')
  const [newCode, setNewCode] = useState('')
  const currentCountdown = useCountdown({ initialSeconds: 60 })
  const newCountdown = useCountdown({ initialSeconds: 60 })

  const handleOpenChange = (open: boolean) => {
    props.onOpenChange(open)
  }

  const sendPrototypeCode = (target: 'current' | 'new') => {
    toast.info(t('Prototype only: no verification code was sent.'))
    if (target === 'current') {
      currentCountdown.start()
      return
    }
    newCountdown.start()
  }

  const handleVerifyCurrent = () => {
    if (!/^\d{6}$/.test(currentCode)) {
      toast.error(t('Verification code must be 6 digits'))
      return
    }
    setStep('enter-new')
  }

  const handleComplete = () => {
    const normalizedPhone = phone.replace(/[\s-]/g, '')
    if (!/^\+?\d{7,15}$/.test(normalizedPhone)) {
      toast.error(t('Please enter a valid phone number'))
      return
    }
    if (!/^\d{6}$/.test(newCode)) {
      toast.error(t('Verification code must be 6 digits'))
      return
    }

    props.onPreviewChange(normalizedPhone)
    toast.success(t('Phone binding preview updated.'))
    handleOpenChange(false)
  }

  const isChanging = Boolean(props.currentPhone)
  const title = isChanging ? t('Change Phone') : t('Bind Phone')

  return (
    <Dialog
      open={props.open}
      onOpenChange={handleOpenChange}
      title={title}
      description={
        isChanging
          ? t('Verify your current phone before binding a new one.')
          : t('Bind a phone number to your account.')
      }
      contentClassName='sm:max-w-md'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => handleOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          {step === 'verify-current' ? (
            <Button
              type='button'
              onClick={handleVerifyCurrent}
              disabled={currentCode.length !== 6}
            >
              {t('Next')}
            </Button>
          ) : (
            <Button
              type='button'
              onClick={handleComplete}
              disabled={!phone || newCode.length !== 6}
            >
              {isChanging ? t('Change Phone') : t('Bind Phone')}
            </Button>
          )}
        </>
      }
    >
      <div className='space-y-4 py-4'>
        <div className='bg-muted/60 text-muted-foreground rounded-lg px-3 py-2.5 text-xs leading-relaxed'>
          {t(
            'Prototype only: verification codes are not sent and changes are not saved.'
          )}
        </div>

        {step === 'verify-current' ? (
          <div className='space-y-2'>
            <Label htmlFor='current-phone-code'>
              {t('Verify Current Phone')}
            </Label>
            <p className='text-muted-foreground text-sm'>
              {t('Enter the code sent to {{phone}}', {
                phone: maskPhoneNumber(props.currentPhone),
              })}
            </p>
            <div className='flex gap-2'>
              <Input
                id='current-phone-code'
                inputMode='numeric'
                autoComplete='one-time-code'
                value={currentCode}
                onChange={(event) =>
                  setCurrentCode(
                    event.target.value.replace(/\D/g, '').slice(0, 6)
                  )
                }
                placeholder={t('Enter code')}
                maxLength={6}
              />
              <Button
                type='button'
                variant='outline'
                onClick={() => sendPrototypeCode('current')}
                disabled={currentCountdown.isActive}
              >
                {currentCountdown.isActive
                  ? `${currentCountdown.secondsLeft}s`
                  : t('Send')}
              </Button>
            </div>
          </div>
        ) : (
          <>
            {isChanging && (
              <Button
                type='button'
                variant='ghost'
                size='sm'
                className='-ml-2 h-7 px-2 text-xs'
                onClick={() => setStep('verify-current')}
              >
                <ArrowLeft className='mr-1 h-3.5 w-3.5' />
                {t('Back to verification')}
              </Button>
            )}
            <div className='space-y-2'>
              <Label htmlFor='phone'>{t('Phone Number')}</Label>
              <Input
                id='phone'
                type='tel'
                inputMode='tel'
                autoComplete='tel'
                value={phone}
                onChange={(event) => setPhone(event.target.value)}
                placeholder={t('Enter your phone number')}
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='new-phone-code'>{t('Verification Code')}</Label>
              <div className='flex gap-2'>
                <Input
                  id='new-phone-code'
                  inputMode='numeric'
                  autoComplete='one-time-code'
                  value={newCode}
                  onChange={(event) =>
                    setNewCode(
                      event.target.value.replace(/\D/g, '').slice(0, 6)
                    )
                  }
                  placeholder={t('Enter code')}
                  maxLength={6}
                />
                <Button
                  type='button'
                  variant='outline'
                  onClick={() => sendPrototypeCode('new')}
                  disabled={!phone || newCountdown.isActive}
                >
                  {newCountdown.isActive
                    ? `${newCountdown.secondsLeft}s`
                    : t('Send')}
                </Button>
              </div>
            </div>
          </>
        )}
      </div>
    </Dialog>
  )
}
