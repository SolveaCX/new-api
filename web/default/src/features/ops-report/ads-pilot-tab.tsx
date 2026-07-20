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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  ackAdsPilotInsight,
  decideAdsPilotProposal,
  opsReportQueryKeys,
} from './api'
import type {
  AdsPilotInsight,
  AdsPilotProposal,
  AdsPilotReport,
} from './types'

const usd = (v: number): string => `$${v.toFixed(v >= 100 ? 0 : 2)}`
const pct = (num: number, den: number): string =>
  den > 0 ? `${((num / den) * 100).toFixed(0)}%` : '-'

// Same Pacific rendering as the rest of the ops report.
const formatTs = (timestamp: number): string => {
  if (!timestamp) return '-'
  return new Date(timestamp * 1000).toLocaleString(undefined, {
    timeZone: 'America/Los_Angeles',
    timeZoneName: 'short',
  })
}

function FreshnessBanner({ report }: { report: AdsPilotReport }) {
  const { t } = useTranslation()
  const meta = report.meta
  const items = [
    { label: t('Ads sync'), ts: meta?.last_sync_at ?? 0 },
    { label: t('Data push'), ts: meta?.last_push_at ?? 0 },
    { label: t('Conversion upload'), ts: meta?.conv_upload_fresh_at ?? 0 },
  ]
  return (
    <div className='space-y-2'>
      {meta?.last_error ? (
        <div className='rounded-md border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-300'>
          {t('Pipeline error')}: {meta.last_error}
        </div>
      ) : null}
      {report.stale ? (
        <div className='rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-700 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-300'>
          {t(
            'Ads data is stale — the ops machine has not pushed results recently.'
          )}
        </div>
      ) : null}
      <p className='text-muted-foreground text-sm'>
        {items.map((it, i) => (
          <span key={it.label}>
            {i > 0 ? ' · ' : ''}
            {it.label}: {formatTs(it.ts)}
          </span>
        ))}
        {meta?.kill_switch ? (
          <Badge variant='destructive' className='ml-2'>
            {t('Kill switch on')}
          </Badge>
        ) : null}
      </p>
    </div>
  )
}

function KpiCards({ report }: { report: AdsPilotReport }) {
  const { t } = useTranslation()
  const rows = report.campaigns ?? []
  const cost = rows.reduce((s, r) => s + r.cost_usd, 0)
  const signups = rows.reduce((s, r) => s + r.signups, 0)
  const paid = rows.reduce((s, r) => s + r.paid_count, 0)
  const paidUsd = rows.reduce((s, r) => s + r.paid_usd, 0)
  const waste = rows.reduce((s, r) => s + r.waste_usd, 0)
  const cards = [
    { label: t('Ad Spend'), value: usd(cost) },
    {
      label: t('Signup CPA'),
      value: signups > 0 ? usd(cost / signups) : '-',
    },
    { label: t('Paid CAC'), value: paid > 0 ? usd(cost / paid) : '-' },
    { label: t('Paid Amount'), value: usd(paidUsd) },
    { label: t('Wasted Spend Share'), value: pct(waste, cost) },
  ]
  return (
    <div className='grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5'>
      {cards.map((c) => (
        <Card key={c.label}>
          <CardContent className='pt-4'>
            <div className='text-muted-foreground text-xs'>{c.label}</div>
            <div className='text-xl font-semibold'>{c.value}</div>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}

function CampaignSummaryTable({ report }: { report: AdsPilotReport }) {
  const { t } = useTranslation()
  const rows = report.campaigns ?? []
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t('Campaign')}</TableHead>
          <TableHead className='text-right'>{t('Ad Spend')}</TableHead>
          <TableHead className='text-right'>{t('Ad Clicks')}</TableHead>
          <TableHead className='text-right'>{t('Registrations')}</TableHead>
          <TableHead className='text-right'>{t('Payment Intent')}</TableHead>
          <TableHead className='text-right'>{t('Paid Users')}</TableHead>
          <TableHead className='text-right'>{t('Paid Amount')}</TableHead>
          <TableHead className='text-right'>{t('Paid CAC')}</TableHead>
          <TableHead className='text-right'>{t('Wasted Spend')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.map((r) => (
          <TableRow key={r.campaign_id}>
            <TableCell>{r.campaign_name || r.campaign_id}</TableCell>
            <TableCell className='text-right'>{usd(r.cost_usd)}</TableCell>
            <TableCell className='text-right'>{r.clicks}</TableCell>
            <TableCell className='text-right'>{r.signups}</TableCell>
            <TableCell className='text-right'>{r.intents}</TableCell>
            <TableCell className='text-right'>{r.paid_count}</TableCell>
            <TableCell className='text-right'>{usd(r.paid_usd)}</TableCell>
            <TableCell className='text-right'>
              {r.paid_count > 0 ? usd(r.cost_usd / r.paid_count) : '-'}
            </TableCell>
            <TableCell className='text-right'>
              {usd(r.waste_usd)}{' '}
              <span className='text-muted-foreground'>
                ({pct(r.waste_usd, r.cost_usd)})
              </span>
            </TableCell>
          </TableRow>
        ))}
        {rows.length === 0 ? (
          <TableRow>
            <TableCell colSpan={9} className='text-muted-foreground text-center'>
              {t('No ads data yet — waiting for the first pipeline push.')}
            </TableCell>
          </TableRow>
        ) : null}
      </TableBody>
    </Table>
  )
}

const severityVariant: Record<
  AdsPilotInsight['severity'],
  'default' | 'secondary' | 'destructive'
> = { info: 'secondary', warn: 'default', alert: 'destructive' }

const proposalStatusVariant: Record<
  AdsPilotProposal['status'],
  'default' | 'secondary' | 'destructive' | 'outline'
> = {
  pending: 'default',
  approved: 'secondary',
  executed: 'secondary',
  rejected: 'outline',
  failed: 'destructive',
}

function ProposalsCard({
  report,
  days,
}: {
  report: AdsPilotReport
  days: number
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: opsReportQueryKeys.ads(days) })
  const decideMutation = useMutation({
    mutationFn: ({
      id,
      decision,
    }: {
      id: number
      decision: 'approve' | 'reject'
    }) => decideAdsPilotProposal(id, decision),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Decision saved'))
      } else {
        toast.error(res.message || t('Failed to save decision'))
      }
      void invalidate()
    },
    onError: () => {
      toast.error(t('Failed to save decision'))
      void invalidate()
    },
  })
  const proposals = report.proposals ?? []
  const pendingCount = proposals.filter((p) => p.status === 'pending').length
  return (
    <Card>
      <CardHeader>
        <CardTitle>
          {t('Proposals')}{' '}
          {pendingCount > 0 ? (
            <Badge className='ml-1'>{pendingCount}</Badge>
          ) : null}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Created')}</TableHead>
              <TableHead>{t('Rule')}</TableHead>
              <TableHead>{t('Campaign')}</TableHead>
              <TableHead>{t('Proposal')}</TableHead>
              <TableHead>{t('Expected Impact')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead className='text-right'>{t('Decision')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {proposals.map((p) => (
              <TableRow key={p.id}>
                <TableCell className='whitespace-nowrap'>
                  {formatTs(p.created_at)}
                </TableCell>
                <TableCell>
                  <Badge variant='outline'>{p.rule}</Badge>
                </TableCell>
                <TableCell>{p.campaign_name || p.campaign_id || '-'}</TableCell>
                <TableCell>
                  <div className='font-medium'>{p.title}</div>
                  <div className='text-muted-foreground max-w-md text-xs whitespace-pre-wrap'>
                    {p.detail}
                  </div>
                  {p.result ? (
                    <div className='text-muted-foreground text-xs'>
                      {t('Result')}: {p.result}
                    </div>
                  ) : null}
                </TableCell>
                <TableCell className='max-w-xs text-xs'>
                  {p.expected_impact || '-'}
                </TableCell>
                <TableCell>
                  <Badge variant={proposalStatusVariant[p.status]}>
                    {t(`adspilot_status_${p.status}`)}
                  </Badge>
                </TableCell>
                <TableCell className='text-right whitespace-nowrap'>
                  {p.status === 'pending' ? (
                    <div className='flex justify-end gap-2'>
                      <Button
                        size='sm'
                        disabled={decideMutation.isPending}
                        onClick={() =>
                          decideMutation.mutate({
                            id: p.id,
                            decision: 'approve',
                          })
                        }
                      >
                        {t('Approve')}
                      </Button>
                      <Button
                        size='sm'
                        variant='outline'
                        disabled={decideMutation.isPending}
                        onClick={() =>
                          decideMutation.mutate({ id: p.id, decision: 'reject' })
                        }
                      >
                        {t('Reject')}
                      </Button>
                    </div>
                  ) : (
                    '-'
                  )}
                </TableCell>
              </TableRow>
            ))}
            {proposals.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={7}
                  className='text-muted-foreground text-center'
                >
                  {t('No proposals yet.')}
                </TableCell>
              </TableRow>
            ) : null}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}

function InsightsCard({
  report,
  days,
}: {
  report: AdsPilotReport
  days: number
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const ackMutation = useMutation({
    mutationFn: (id: number) => ackAdsPilotInsight(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: opsReportQueryKeys.ads(days),
      })
    },
    onError: () => toast.error(t('Failed to acknowledge')),
  })
  const insights = report.insights ?? []
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Open Alerts')}</CardTitle>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Created')}</TableHead>
              <TableHead>{t('Severity')}</TableHead>
              <TableHead>{t('Rule')}</TableHead>
              <TableHead>{t('Campaign')}</TableHead>
              <TableHead>{t('Finding')}</TableHead>
              <TableHead className='text-right' />
            </TableRow>
          </TableHeader>
          <TableBody>
            {insights.map((row) => (
              <TableRow key={row.id}>
                <TableCell className='whitespace-nowrap'>
                  {formatTs(row.created_at)}
                </TableCell>
                <TableCell>
                  <Badge variant={severityVariant[row.severity]}>
                    {t(`adspilot_severity_${row.severity}`)}
                  </Badge>
                </TableCell>
                <TableCell>
                  <Badge variant='outline'>{row.rule}</Badge>
                </TableCell>
                <TableCell>
                  {row.campaign_name || row.campaign_id || '-'}
                </TableCell>
                <TableCell>
                  <div className='font-medium'>{row.title}</div>
                  <div className='text-muted-foreground max-w-md text-xs whitespace-pre-wrap'>
                    {row.detail}
                  </div>
                </TableCell>
                <TableCell className='text-right'>
                  <Button
                    size='sm'
                    variant='outline'
                    disabled={ackMutation.isPending}
                    onClick={() => ackMutation.mutate(row.id)}
                  >
                    {t('Acknowledge')}
                  </Button>
                </TableCell>
              </TableRow>
            ))}
            {insights.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={6}
                  className='text-muted-foreground text-center'
                >
                  {t('No open alerts.')}
                </TableCell>
              </TableRow>
            ) : null}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}

function ActionsCard({ report }: { report: AdsPilotReport }) {
  const { t } = useTranslation()
  const actions = report.actions ?? []
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Automation Log')}</CardTitle>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Time')}</TableHead>
              <TableHead>{t('Rule')}</TableHead>
              <TableHead>{t('Action')}</TableHead>
              <TableHead>{t('Campaign')}</TableHead>
              <TableHead>{t('Target')}</TableHead>
              <TableHead>{t('Mode')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {actions.map((a) => (
              <TableRow key={a.id}>
                <TableCell className='whitespace-nowrap'>
                  {formatTs(a.created_at)}
                </TableCell>
                <TableCell>
                  <Badge variant='outline'>{a.rule}</Badge>
                </TableCell>
                <TableCell>{a.action_type}</TableCell>
                <TableCell>{a.campaign_name || a.campaign_id || '-'}</TableCell>
                <TableCell className='max-w-md text-xs whitespace-pre-wrap'>
                  {a.target}
                </TableCell>
                <TableCell>{t(`adspilot_mode_${a.mode}`)}</TableCell>
                <TableCell>
                  <Badge
                    variant={a.status === 'failed' ? 'destructive' : 'secondary'}
                  >
                    {t(`adspilot_action_${a.status}`)}
                  </Badge>
                </TableCell>
              </TableRow>
            ))}
            {actions.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={7}
                  className='text-muted-foreground text-center'
                >
                  {t('No automated actions yet.')}
                </TableCell>
              </TableRow>
            ) : null}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}

export function AdsPilotTab({
  report,
  days,
}: {
  report: AdsPilotReport
  days: number
}) {
  const { t } = useTranslation()
  return (
    <div className='space-y-4'>
      <FreshnessBanner report={report} />
      <KpiCards report={report} />
      <ProposalsCard report={report} days={days} />
      <Card>
        <CardHeader>
          <CardTitle>{t('Campaign Performance')}</CardTitle>
        </CardHeader>
        <CardContent>
          <CampaignSummaryTable report={report} />
        </CardContent>
      </Card>
      <InsightsCard report={report} days={days} />
      <ActionsCard report={report} />
    </div>
  )
}
