/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { SectionPageLayout } from '@/components/layout'
import { InvitationFaq } from './components/invitation-faq'
import { InvitationRecordsCard } from './components/invitation-records-card'
import { InvitationStats } from './components/invitation-stats'
import { ReferralLinkCard } from './components/referral-link-card'
import { RewardStepsCard } from './components/reward-steps-card'
import { RewardTransferCard } from './components/reward-transfer-card'
import { TransferDialog } from './components/transfer-dialog'
import { useInvitations } from './hooks/use-invitations'
import { buildAffiliateLink } from './lib/share'
import type { InvitationPageData } from './types'

export interface InvitationViewProps {
  data: InvitationPageData | null
  affiliateLink: string
  loading: boolean
  recordsLoading: boolean
  affiliateLoading: boolean
  affiliateError: boolean
  error: boolean
  transferring: boolean
  page: number
  onPageChange: (page: number) => void
  onRetry: () => void
  onTransfer: (quota: number) => Promise<boolean>
}

export function InvitationView({
  data,
  affiliateLink,
  loading,
  recordsLoading,
  affiliateLoading,
  affiliateError,
  error,
  transferring,
  page,
  onPageChange,
  onRetry,
  onTransfer,
}: InvitationViewProps) {
  const [transferOpen, setTransferOpen] = useState(false)
  const summary = data?.summary ?? null

  return (
    <div className='mx-auto flex w-full max-w-7xl flex-col gap-4 sm:gap-5'>
      <InvitationStats summary={summary} loading={loading} />
      <ReferralLinkCard
        affiliateLink={affiliateLink}
        loading={affiliateLoading}
        error={affiliateError}
      />
      <RewardStepsCard />
      <RewardTransferCard
        summary={summary}
        loading={loading}
        onOpen={() => setTransferOpen(true)}
      />
      <InvitationRecordsCard
        data={data}
        loading={loading || recordsLoading}
        error={error}
        page={page}
        onRetry={onRetry}
        onPageChange={onPageChange}
      />
      <InvitationFaq summary={summary} />
      <TransferDialog
        open={transferOpen}
        onOpenChange={setTransferOpen}
        onConfirm={onTransfer}
        availableQuota={summary?.transferable_quota ?? 0}
        transferring={transferring}
      />
    </div>
  )
}

export function Invitations() {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const { invitationsQuery, codeQuery, transferMutation } = useInvitations(page)
  const data = invitationsQuery.data?.data ?? null
  const code = codeQuery.data?.data ?? ''
  const affiliateLink = buildAffiliateLink(code)

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Invite & Earn')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <InvitationView
          data={data}
          affiliateLink={affiliateLink}
          loading={invitationsQuery.isLoading}
          recordsLoading={invitationsQuery.isPlaceholderData}
          affiliateLoading={codeQuery.isLoading}
          affiliateError={codeQuery.isError}
          error={invitationsQuery.isError}
          transferring={transferMutation.isPending}
          page={page}
          onPageChange={setPage}
          onRetry={() => void invitationsQuery.refetch()}
          onTransfer={async (quota) => {
            try {
              await transferMutation.mutateAsync(quota)
              return true
            } catch {
              return false
            }
          }}
        />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
