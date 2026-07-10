/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useTranslation } from 'react-i18next'
import { formatQuota } from '@/lib/format'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { TitledCard } from '@/components/ui/titled-card'
import type { InvitationSummary } from '../types'

interface InvitationFaqProps {
  summary: InvitationSummary | null
}

export function InvitationFaq({ summary }: InvitationFaqProps) {
  const { t } = useTranslation()
  const inviterReward = formatQuota(summary?.inviter_reward_quota ?? 0)
  const inviteeReward = formatQuota(summary?.invitee_reward_quota ?? 0)
  const limit = summary?.inviter_reward_max_count ?? 0
  const items = [
    {
      question: t('When are referral rewards granted?'),
      answer: t(
        'Referral rewards are granted only after your friend completes their first successful top-up. Registration, creating an API key, and making an API call do not grant a reward.'
      ),
    },
    {
      question: t('What are the current referral rewards?'),
      answer: t(
        "The current configured rewards are {{inviterReward}} for you and {{inviteeReward}} for your friend. Rewards are processed after your friend's first successful top-up.",
        { inviterReward, inviteeReward }
      ),
    },
    {
      question: t('Is there a referral reward limit?'),
      answer:
        limit === 0
          ? t(
              'There is currently no limit on the number of referral rewards you can earn.'
            )
          : t(
              'The maximum number of successful referrals you can earn rewards for is {{count}}. Friends invited after that can still receive their reward.',
              { count: limit }
            ),
    },
    {
      question: t('How do I use my referral rewards?'),
      answer: t(
        'Transfer available referral rewards to your main balance, then use them for API requests.'
      ),
    },
    {
      question: t('Which referrals appear here?'),
      answer: t(
        'This list shows active accounts registered through your referral link. Deleted accounts may not appear, so the rewards shown here may not add up to your lifetime earnings.'
      ),
    },
    {
      question: t('What behavior is prohibited?'),
      answer: t(
        'Self-referrals, duplicate accounts, and other abuse are prohibited. Rewards may be withheld or revoked.'
      ),
    },
  ]

  return (
    <TitledCard title={t('FAQ')}>
      <Accordion>
        {items.map((item, index) => (
          <AccordionItem key={item.question} value={`item-${index}`}>
            <AccordionTrigger>{item.question}</AccordionTrigger>
            <AccordionContent className='text-muted-foreground'>
              {item.answer}
            </AccordionContent>
          </AccordionItem>
        ))}
      </Accordion>
    </TitledCard>
  )
}
