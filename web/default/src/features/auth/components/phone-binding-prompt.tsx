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
import { Smartphone } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { useOnboardingStore } from '@/stores/onboarding-store'
import { needsPhoneBinding } from '@/lib/phone-binding'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'

export function PhoneBindingPrompt() {
  const { t, i18n } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const onboardingOpen = useOnboardingStore((state) => state.open)
  const [dismissedUser, setDismissedUser] = useState<number | null>(null)
  const preview =
    import.meta.env.DEV &&
    new URLSearchParams(window.location.search).get('phoneBindingPreview') ===
      '1'
  const identity = user?.id ?? -1
  const open =
    !onboardingOpen &&
    !user?.impersonating &&
    (preview || needsPhoneBinding(user)) &&
    dismissedUser !== identity

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) setDismissedUser(identity)
      }}
    >
      <DialogContent className='max-h-[calc(100dvh-2rem)] gap-6 overflow-y-auto p-6 sm:max-w-[440px] sm:p-8'>
        <DialogHeader>
          <span className='bg-muted text-foreground mb-2 flex size-12 items-center justify-center rounded-2xl'>
            <Smartphone className='size-6' aria-hidden='true' />
          </span>
          <DialogTitle>{t('Link your phone number')}</DialogTitle>
          <DialogDescription>
            {t('Add a phone number to help keep your account secure.')}
          </DialogDescription>
        </DialogHeader>
        <form
          className='flex flex-col gap-5'
          onSubmit={(event) => event.preventDefault()}
        >
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor='binding-phone'>
                {t('Phone number')}
              </FieldLabel>
              <div className='flex gap-2'>
                <Input
                  aria-label={t('Country calling code')}
                  type='tel'
                  autoComplete='tel-country-code'
                  defaultValue={i18n.language.startsWith('zh') ? '+86' : '+1'}
                  maxLength={4}
                  className='h-11 w-20 shrink-0'
                />
                <Input
                  id='binding-phone'
                  type='tel'
                  autoComplete='tel-national'
                  placeholder={t('Enter your phone number')}
                  className='h-11 min-w-0 flex-1'
                />
              </div>
            </Field>
            <Field>
              <FieldLabel htmlFor='binding-code'>
                {t('Verification code')}
              </FieldLabel>
              <div className='flex gap-2'>
                <Input
                  id='binding-code'
                  inputMode='numeric'
                  autoComplete='one-time-code'
                  placeholder={t('Enter verification code')}
                  maxLength={6}
                  className='h-11 min-w-0 flex-1'
                />
                <Button
                  type='button'
                  variant='outline'
                  disabled
                  aria-describedby='binding-unavailable'
                  className='h-auto min-h-11 max-w-[45%] whitespace-normal'
                >
                  {t('Send code')}
                </Button>
              </div>
            </Field>
          </FieldGroup>
          <p
            id='binding-unavailable'
            className='text-muted-foreground text-xs leading-5'
          >
            {t('Phone verification will be available soon.')}
          </p>
          <div className='flex flex-col gap-2'>
            <Button
              type='submit'
              disabled
              aria-describedby='binding-unavailable'
              className='min-h-11 w-full'
            >
              {t('Link phone number')}
            </Button>
            <DialogClose
              render={<Button variant='ghost' className='min-h-10 w-full' />}
            >
              {t('Maybe later')}
            </DialogClose>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
