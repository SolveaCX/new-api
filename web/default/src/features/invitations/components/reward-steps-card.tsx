/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useTranslation } from 'react-i18next'
import { TitledCard } from '@/components/ui/titled-card'

export function RewardStepsCard() {
  const { t } = useTranslation()
  const steps = [
    {
      title: t('Share your referral link'),
      description: t('Send your unique referral link to a friend.'),
    },
    {
      title: t('Your friend signs up'),
      description: t('They create their account using your referral link.'),
    },
    {
      title: t('Your friend completes their first successful top-up'),
      description: t(
        'NewAPI then processes the configured rewards for both accounts, subject to the referral reward limit.'
      ),
    },
  ]

  return (
    <TitledCard title={t('How it works')}>
      <ol className='grid gap-4 md:grid-cols-3'>
        {steps.map((step, index) => (
          <li key={step.title} className='flex min-w-0 gap-3'>
            <span className='bg-primary text-primary-foreground flex size-7 shrink-0 items-center justify-center rounded-full text-xs font-semibold tabular-nums'>
              {index + 1}
            </span>
            <div className='min-w-0'>
              <h3 className='font-medium'>{step.title}</h3>
              <p className='text-muted-foreground mt-1 text-sm'>
                {step.description}
              </p>
            </div>
          </li>
        ))}
      </ol>
    </TitledCard>
  )
}
