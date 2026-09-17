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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { SectionPageLayout } from '@/components/layout'
import {
  acceptBid,
  cancelRFQ,
  extendRFQ,
  getRFQ,
  getSupplierProfile,
  raiseCeiling,
  withdrawBid,
} from './api'
import { BidDialog } from './components/bid-dialog'
import { RFQStatusBadge } from './components/status-badge'
import { rfqQueryKey, supplierQueryKey } from './keys'
import {
  annualValue,
  canAcceptBid,
  formatPrice,
  formatRemaining,
  formatUsd,
  isOpenStatus,
  monthlyValue,
} from './market'
import type { ComputeBid, RFQDetail as RFQDetailData } from './types'

function BidCard(props: {
  bid: ComputeBid
  rfq: RFQDetailData['rfq']
  acceptable: boolean
  onAccept: () => void
  pending: boolean
}) {
  const { t } = useTranslation()
  const { bid, rfq } = props
  const muted =
    !bid.eligible || bid.status === 'rejected' || bid.status === 'withdrawn'
  return (
    <Card
      className={cn(
        bid.rank === 1 && !muted && 'border-foreground shadow-md',
        muted && 'opacity-60',
        bid.status === 'accepted' && 'border-primary'
      )}
    >
      <CardContent className='grid items-center gap-4 pt-6 md:grid-cols-[1.4fr_1fr_1fr_1fr_auto]'>
        <div>
          <div className='flex flex-wrap items-center gap-2'>
            <span className='font-bold'>{bid.supplier_alias}</span>
            {bid.supplier_level >= 2 && <Badge>{t('L2 preferred')}</Badge>}
            {bid.mine && <Badge variant='secondary'>{t('you')}</Badge>}
            {bid.rank > 0 && bid.status === 'live' && (
              <Badge variant='outline'>#{bid.rank}</Badge>
            )}
            {bid.status === 'accepted' && <Badge>{t('accepted')}</Badge>}
            {!bid.eligible && (
              <Badge variant='destructive'>{t('not eligible')}</Badge>
            )}
          </div>
          <div className='text-muted-foreground text-xs'>
            {bid.sla_pct ? `SLA ${bid.sla_pct}%` : ''}
          </div>
        </div>
        <div>
          <div className='text-2xl font-extrabold tracking-tight'>
            {formatPrice(bid.price_per_gpu_hour)}
            <span className='text-muted-foreground text-xs font-medium'>
              {' '}
              /GPU-hr
            </span>
          </div>
          <div className='text-muted-foreground text-xs'>
            {t('annual')} {formatUsd(annualValue(rfq, bid.price_per_gpu_hour))}
          </div>
        </div>
        <div>
          <div className='font-semibold'>{bid.deliver_date || '—'}</div>
          <div className='text-muted-foreground text-xs'>
            {bid.deliver_date &&
            rfq.start_date &&
            bid.deliver_date > rfq.start_date
              ? t('later than requested')
              : t('deliverable from')}
          </div>
        </div>
        <div>
          <div
            className={cn(
              'font-semibold',
              bid.deviations ? 'text-amber-600' : 'text-green-700'
            )}
          >
            {bid.deviations || t('none')}
          </div>
          <div className='text-muted-foreground text-xs'>{t('deviations')}</div>
        </div>
        <div>
          {props.acceptable && (
            <Button
              disabled={props.pending}
              onClick={props.onAccept}
              variant={bid.rank === 1 ? 'default' : 'outline'}
            >
              {t('Accept {{price}}', {
                price: formatPrice(bid.price_per_gpu_hour),
              })}
            </Button>
          )}
        </div>
      </CardContent>
    </Card>
  )
}

export function RFQDetail({ rfqId }: { rfqId: number }) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const q = useQuery({
    queryKey: rfqQueryKey(rfqId),
    queryFn: () => getRFQ(rfqId),
    refetchInterval: 10000,
  })
  const profile = useQuery({
    queryKey: supplierQueryKey,
    queryFn: getSupplierProfile,
  })
  const [confirmBid, setConfirmBid] = useState<ComputeBid | null>(null)
  const [bidOpen, setBidOpen] = useState(false)
  const [raise, setRaise] = useState('')

  const refresh = () =>
    void qc.invalidateQueries({ queryKey: ['compute-market'] })
  const onErr = (e: unknown) =>
    toast.error(e instanceof Error ? e.message : t('Request failed'))
  const accept = useMutation({
    mutationFn: (bidId: number) => acceptBid(rfqId, bidId),
    onSuccess: ({ accepted_bid }) => {
      toast.success(
        t(
          'Matched with {{alias}} at {{price}}. flatkey will prepare the contract and escrow.',
          {
            alias: accepted_bid.supplier_alias,
            price: formatPrice(accepted_bid.price_per_gpu_hour),
          }
        )
      )
      setConfirmBid(null)
      refresh()
    },
    onError: onErr,
  })
  const extend = useMutation({
    mutationFn: () => extendRFQ(rfqId),
    onSuccess: () => {
      toast.success(t('Matching extended by 24h'))
      refresh()
    },
    onError: onErr,
  })
  const doRaise = useMutation({
    mutationFn: () => raiseCeiling(rfqId, Number(raise)),
    onSuccess: () => {
      toast.success(t('Ceiling raised'))
      setRaise('')
      refresh()
    },
    onError: onErr,
  })
  const cancel = useMutation({
    mutationFn: () => cancelRFQ(rfqId),
    onSuccess: () => {
      toast.success(t('Request withdrawn'))
      refresh()
    },
    onError: onErr,
  })
  const withdraw = useMutation({
    mutationFn: (bidId: number) => withdrawBid(bidId),
    onSuccess: () => {
      toast.success(t('Bid withdrawn'))
      refresh()
    },
    onError: onErr,
  })

  if (q.isLoading || !q.data) {
    return (
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Compute Market')}</SectionPageLayout.Title>
        <SectionPageLayout.Content>
          {q.isError ? (
            <p className='text-destructive'>{t('Request not found')}</p>
          ) : (
            <Skeleton className='h-64 w-full' />
          )}
        </SectionPageLayout.Content>
      </SectionPageLayout>
    )
  }
  const d = q.data
  const { rfq, bids, top } = d
  const now = d.now
  const open = isOpenStatus(rfq.status)
  const myBid = bids.find((b) => b.mine && b.status === 'live') ?? null
  const liveCount = bids.filter((b) => b.status === 'live').length
  const eligibleCount = bids.filter(
    (b) => b.status === 'live' && b.eligible
  ).length

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <span className='flex items-center gap-3'>
          <Button
            variant='ghost'
            size='sm'
            render={<Link to='/compute/market' />}
          >
            <ArrowLeft className='size-4' /> {t('Market')}
          </Button>
          {rfq.code} <RFQStatusBadge status={rfq.status} />
        </span>
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='grid gap-4 lg:grid-cols-[1.6fr_1fr]'>
          <div className='flex flex-col gap-4'>
            <Card>
              <CardContent className='grid gap-4 pt-6 md:grid-cols-4'>
                <div className='md:col-span-4'>
                  <p className='text-2xl font-extrabold'>
                    {rfq.gpu_model} × {rfq.nodes} {t('nodes')}{' '}
                    <span className='text-muted-foreground text-base font-semibold'>
                      = {rfq.gpus_per_node * rfq.nodes} GPU
                    </span>
                  </p>
                  <p className='text-muted-foreground text-sm'>
                    {rfq.term_months} {t('mo')} · {t('from')} {rfq.start_date} ·{' '}
                    {rfq.regions} · {rfq.delivery} · {rfq.interconnect || '—'}
                    {rfq.storage_tb ? ` · ${rfq.storage_tb} TB` : ''}
                    {rfq.compliance ? ` · ${rfq.compliance}` : ''}
                  </p>
                  {rfq.notes && (
                    <p className='mt-2 text-sm whitespace-pre-wrap'>
                      {rfq.notes}
                    </p>
                  )}
                </div>
                <div>
                  <p className='text-muted-foreground text-xs uppercase'>
                    {t('Lowest eligible')}
                  </p>
                  <p className='text-3xl font-extrabold'>
                    {formatPrice(d.lowest_price)}
                  </p>
                </div>
                <div>
                  <p className='text-muted-foreground text-xs uppercase'>
                    {t('Ceiling')}
                  </p>
                  <p className='text-2xl font-bold'>
                    {formatPrice(rfq.price_ceiling)}
                  </p>
                </div>
                <div>
                  <p className='text-muted-foreground text-xs uppercase'>
                    {rfq.status === 'matching' ? t('Remaining') : t('Status')}
                  </p>
                  <p className='text-2xl font-bold'>
                    {rfq.status === 'matching'
                      ? formatRemaining(rfq.matching_deadline, now)
                      : rfq.status}
                  </p>
                </div>
                <div>
                  <p className='text-muted-foreground text-xs uppercase'>
                    {t('Eligible bids')}
                  </p>
                  <p className='text-2xl font-bold'>
                    {eligibleCount}{' '}
                    <span className='text-muted-foreground text-sm'>
                      / {liveCount}
                    </span>
                  </p>
                </div>
              </CardContent>
            </Card>

            <div className='flex items-center justify-between'>
              <h3 className='font-bold'>{t('Bids by price')}</h3>
              {d.is_owner && open && (
                <span className='text-muted-foreground text-xs'>
                  {rfq.status === 'matching'
                    ? t(
                        'Accept any eligible bid now — or wait; after the close you pick from the lowest 3.'
                      )
                    : t(
                        'Matching closed: choose one of the lowest 3 eligible bids, or extend.'
                      )}
                </span>
              )}
            </div>
            {bids.length === 0 && (
              <Card>
                <CardContent className='text-muted-foreground py-8 text-center text-sm'>
                  {t(
                    'No bids yet. Suppliers matching this request have been notified.'
                  )}
                </CardContent>
              </Card>
            )}
            {bids.map((b) => (
              <BidCard
                key={b.id}
                bid={b}
                rfq={rfq}
                acceptable={d.is_owner && canAcceptBid(rfq, b, top)}
                onAccept={() => setConfirmBid(b)}
                pending={accept.isPending}
              />
            ))}

            {d.is_owner && open && (
              <Card>
                <CardHeader>
                  <CardTitle>{t('Not enough bids?')}</CardTitle>
                  <CardDescription>
                    {t(
                      'Like adding a tip on a ride-hailing app: a higher ceiling or a longer window brings more suppliers.'
                    )}
                  </CardDescription>
                </CardHeader>
                <CardContent className='flex flex-wrap items-center gap-2'>
                  <Input
                    type='number'
                    step='0.05'
                    className='w-32'
                    placeholder={String(rfq.price_ceiling + 0.5)}
                    value={raise}
                    onChange={(e) => setRaise(e.target.value)}
                  />
                  <Button
                    variant='outline'
                    disabled={
                      !raise ||
                      Number(raise) <= rfq.price_ceiling ||
                      doRaise.isPending
                    }
                    onClick={() => doRaise.mutate()}
                  >
                    {t('Raise ceiling')}
                  </Button>
                  <Button
                    variant='outline'
                    disabled={extend.isPending || rfq.extensions >= 3}
                    onClick={() => extend.mutate()}
                  >
                    {t('Extend 24h')} ({rfq.extensions}/3)
                  </Button>
                  <Button
                    variant='destructive'
                    disabled={cancel.isPending}
                    onClick={() => cancel.mutate()}
                  >
                    {t('Withdraw request')}
                  </Button>
                </CardContent>
              </Card>
            )}
          </div>

          <div className='flex flex-col gap-4'>
            {d.is_owner ? (
              <Card className='bg-foreground text-background'>
                <CardContent className='flex flex-col gap-2 pt-6 text-sm'>
                  <p className='text-xs opacity-70'>{t('After you accept')}</p>
                  <p className='text-xl font-extrabold'>
                    {t('Lock price → sign within 72h → escrow')}
                  </p>
                  <ol className='list-decimal pl-4 opacity-90'>
                    <li>
                      {t(
                        'The bid is locked and binding on the supplier; other bids are released.'
                      )}
                    </li>
                    <li>
                      {t(
                        'You and the supplier each sign a back-to-back contract with flatkey.'
                      )}
                    </li>
                    <li>
                      {t(
                        'First month + one month deposit go to escrow; released after acceptance tests and monthly SLA.'
                      )}
                    </li>
                  </ol>
                  {d.accepted_bid && (
                    <p className='mt-2 text-xs opacity-70'>
                      {t('First escrow ≈ {{value}}', {
                        value: formatUsd(
                          monthlyValue(rfq, d.accepted_bid.price_per_gpu_hour) *
                            2
                        ),
                      })}
                    </p>
                  )}
                </CardContent>
              </Card>
            ) : (
              <Card>
                <CardHeader>
                  <CardTitle>{t('Your bid')}</CardTitle>
                </CardHeader>
                <CardContent className='flex flex-col gap-3 text-sm'>
                  {myBid ? (
                    <>
                      <p className='text-2xl font-extrabold'>
                        {formatPrice(myBid.price_per_gpu_hour)}{' '}
                        <span className='text-muted-foreground text-sm font-medium'>
                          · {myBid.rank ? `#${myBid.rank}` : t('not eligible')}
                        </span>
                      </p>
                      <div className='flex gap-2'>
                        <Button
                          size='sm'
                          disabled={!open}
                          onClick={() => setBidOpen(true)}
                        >
                          {t('Lower')}
                        </Button>
                        <Button
                          size='sm'
                          variant='outline'
                          disabled={!open || withdraw.isPending}
                          onClick={() => withdraw.mutate(myBid.id)}
                        >
                          {t('Withdraw')}
                        </Button>
                      </div>
                    </>
                  ) : (
                    <>
                      <p className='text-muted-foreground'>
                        {profile.data?.can_bid
                          ? t(
                              'Bids are public and binding. The buyer may accept yours at any time.'
                            )
                          : t('Verify your supplier profile to bid.')}
                      </p>
                      <Button
                        disabled={!open || !profile.data?.can_bid}
                        onClick={() => setBidOpen(true)}
                      >
                        {t('Bid on this request')}
                      </Button>
                    </>
                  )}
                </CardContent>
              </Card>
            )}
            <Card>
              <CardHeader>
                <CardTitle>{t('Rules')}</CardTitle>
              </CardHeader>
              <CardContent className='text-muted-foreground flex flex-col gap-1 text-xs'>
                <p>
                  ·{' '}
                  {t(
                    'Buyer and supplier see aliases only; contact details never cross the market.'
                  )}
                </p>
                <p>
                  ·{' '}
                  {t(
                    'Bids only go down (min $0.05). A bid in the last 15 minutes extends matching by 15 minutes.'
                  )}
                </p>
                <p>
                  ·{' '}
                  {t(
                    'A bid that misses a hard requirement is visible but cannot be accepted.'
                  )}
                </p>
                <p>
                  ·{' '}
                  {t(
                    'Matched then cancelled: buyer forfeits $2,000; supplier that does not sign forfeits 1%.'
                  )}
                </p>
              </CardContent>
            </Card>
          </div>
        </div>

        <AlertDialog
          open={Boolean(confirmBid)}
          onOpenChange={(o) => !o && setConfirmBid(null)}
        >
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>
                {t('Accept {{alias}} at {{price}} per GPU-hour?', {
                  alias: confirmBid?.supplier_alias,
                  price: formatPrice(confirmBid?.price_per_gpu_hour ?? 0),
                })}
              </AlertDialogTitle>
              <AlertDialogDescription>
                {t(
                  'This locks the price and closes the auction. Monthly ≈ {{monthly}}, first escrow ≈ {{escrow}}. Cancelling after this forfeits your $2,000 deposit.',
                  {
                    monthly: formatUsd(
                      monthlyValue(rfq, confirmBid?.price_per_gpu_hour ?? 0)
                    ),
                    escrow: formatUsd(
                      monthlyValue(rfq, confirmBid?.price_per_gpu_hour ?? 0) * 2
                    ),
                  }
                )}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
              <AlertDialogAction
                onClick={() => confirmBid && accept.mutate(confirmBid.id)}
              >
                {t('Accept and match')}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
        {!d.is_owner && bidOpen && (
          <BidDialog
            rfq={rfq}
            lowest={d.lowest_price}
            myBid={myBid}
            open={bidOpen}
            onOpenChange={setBidOpen}
          />
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
