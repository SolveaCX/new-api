/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import { CopyButton } from '@/components/copy-button'
import { buildInvitationShareLinks } from '../lib/share'

interface ReferralLinkCardProps {
  affiliateLink: string
  loading: boolean
  error: boolean
}

export function ReferralLinkCard({
  affiliateLink,
  loading,
  error,
}: ReferralLinkCardProps) {
  const { t } = useTranslation()
  const links = buildInvitationShareLinks(
    affiliateLink,
    t(
      'Join NewAPI with my referral link. Referral rewards are processed after your first successful top-up.'
    )
  )

  return (
    <TitledCard
      title={t('Your Referral Link')}
      description={t(
        'Share your referral link with friends. Referral rewards are processed after their first successful top-up.'
      )}
      contentClassName='space-y-3'
    >
      {loading ? (
        <>
          <Skeleton className='h-9 w-full' />
          <Skeleton className='h-8 w-64 max-w-full' />
        </>
      ) : (
        <>
          <div className='flex min-w-0 gap-2'>
            <Input
              value={affiliateLink}
              readOnly
              aria-label={t('Your Referral Link')}
              className='min-w-0 flex-1 font-mono text-xs'
            />
            {affiliateLink ? (
              <CopyButton
                value={affiliateLink}
                variant='outline'
                tooltip={t('Copy referral link')}
                aria-label={t('Copy referral link')}
              >
                <span className='hidden sm:inline'>{t('Copy')}</span>
              </CopyButton>
            ) : null}
          </div>

          {error ? (
            <p className='text-muted-foreground text-sm'>
              {t("We couldn't load your referrals.")}
            </p>
          ) : null}

          {affiliateLink ? (
            <div className='flex flex-wrap gap-2'>
              <Button
                variant='outline'
                size='sm'
                render={<a href={links.email} />}
              >
                {t('Share by email')}
              </Button>
              <Button
                variant='outline'
                size='sm'
                render={
                  <a href={links.x} target='_blank' rel='noreferrer noopener' />
                }
              >
                {t('Share on X')}
              </Button>
              <Button
                variant='outline'
                size='sm'
                render={
                  <a
                    href={links.linkedin}
                    target='_blank'
                    rel='noreferrer noopener'
                  />
                }
              >
                {t('Share on LinkedIn')}
              </Button>
            </div>
          ) : null}
        </>
      )}
    </TitledCard>
  )
}
