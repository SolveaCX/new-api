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
import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import {
  acceptBid,
  extendRFQ,
  getRFQ,
  getSupplierProfile,
  listOpenRFQs,
  placeBid,
  withdrawBid,
} from '../api'
import { openRFQsQueryKey, rfqQueryKey, supplierQueryKey } from '../keys'
import {
  GPU_MODELS,
  type MarketplaceStatusFilter,
  annualValue,
  canAcceptBid,
  filterMarketplace,
  formatPrice,
  formatRemaining,
  formatUsd,
  hardRequirementsFor,
  isOpenStatus,
  marketplaceStats,
  monthlyValue,
  paginate,
  suggestedBidPrice,
} from '../market'
import type { ComputeBid, OpenRFQItem } from '../types'
import { useNowSeconds } from '../use-now'
import { RFQStatusBadge } from './status-badge'

const PER_PAGE = 14

function Chip(props: {
  on: boolean
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <button
      type='button'
      onClick={props.onClick}
      className={cn(
        'rounded-full border px-3 py-1 text-xs font-semibold whitespace-nowrap transition-colors',
        props.on
          ? 'border-foreground bg-foreground text-background'
          : 'border-border bg-background hover:bg-muted'
      )}
    >
      {props.children}
    </button>
  )
}

function Nickname({ alias, hint }: { alias: string; hint: string }) {
  return (
    <span className='group relative inline-flex cursor-help items-center gap-1 font-semibold whitespace-nowrap'>
      <span className='bg-primary/10 text-primary inline-flex size-4 items-center justify-center rounded-full text-[10px] font-extrabold'>
        {alias.replace('#', '')[0]}
      </span>
      <span className='border-foreground/40 border-b border-dashed'>
        {alias}
      </span>
      <span
        role='tooltip'
        className='bg-foreground text-background invisible absolute top-full left-0 z-30 mt-2 w-64 rounded-lg px-3 py-2 text-xs font-medium whitespace-normal shadow-lg group-hover:visible'
      >
        {hint}
      </span>
    </span>
  )
}

/** Right-hand detail pane for the selected request. */
function Detail(props: {
  item: OpenRFQItem
  canBid: boolean
  now: number
  onClose: () => void
  onRegister: () => void
}) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const { rfq } = props.item
  const q = useQuery({
    queryKey: rfqQueryKey(rfq.id),
    queryFn: () => getRFQ(rfq.id),
    refetchInterval: 10000,
  })
  const d = q.data
  const myBid = d?.bids.find((b) => b.mine && b.status === 'live') ?? null
  const lowest = d?.lowest_price ?? props.item.lowest_price
  const reqs = hardRequirementsFor(rfq)
  const [price, setPrice] = useState(() =>
    String(suggestedBidPrice(rfq, lowest, myBid?.price_per_gpu_hour))
  )
  const [deliverDate, setDeliverDate] = useState(rfq.start_date)
  const [hard, setHard] = useState<Record<string, boolean>>(() =>
    Object.fromEntries(reqs.map((r) => [r, true]))
  )
  const [deviations, setDeviations] = useState('')
  const refresh = () =>
    void qc.invalidateQueries({ queryKey: ['compute-market'] })
  const onErr = (e: unknown) =>
    toast.error(e instanceof Error ? e.message : t('Request failed'))
  const bid = useMutation({
    mutationFn: () =>
      placeBid(rfq.id, {
        price_per_gpu_hour: Number(price),
        deliver_date: deliverDate,
        sla_pct: 99.5,
        hard_reqs: hard,
        deviations,
      }),
    onSuccess: () => {
      toast.success(t('Bid placed — it is binding until the buyer decides'))
      refresh()
    },
    onError: onErr,
  })
  const withdraw = useMutation({
    mutationFn: (id: number) => withdrawBid(id),
    onSuccess: () => {
      toast.success(t('Bid withdrawn'))
      refresh()
    },
    onError: onErr,
  })
  const accept = useMutation({
    mutationFn: (id: number) => acceptBid(rfq.id, id),
    onSuccess: () => {
      toast.success(t('Matched. flatkey will prepare the contract and escrow.'))
      refresh()
    },
    onError: onErr,
  })
  const extend = useMutation({
    mutationFn: () => extendRFQ(rfq.id),
    onSuccess: () => {
      toast.success(t('Matching extended by 24h'))
      refresh()
    },
    onError: onErr,
  })
  const p = Number(price)
  const eligible = reqs.every((r) => hard[r])
  const valid =
    p > 0 && p < rfq.price_ceiling && (!lowest || p < lowest || Boolean(myBid))
  const open = isOpenStatus(rfq.status)

  return (
    <Card className='sticky top-4'>
      <CardContent className='flex flex-col gap-3 pt-5 text-sm'>
        <div className='flex items-center justify-between'>
          <b>{rfq.code}</b>
          <div className='flex items-center gap-2'>
            <RFQStatusBadge status={rfq.status} />
            <button
              type='button'
              className='text-muted-foreground text-xs'
              onClick={props.onClose}
            >
              ✕
            </button>
          </div>
        </div>
        <div>
          <p className='text-2xl font-extrabold tracking-tight'>
            {rfq.gpu_model} × {rfq.nodes}
          </p>
          <p className='text-muted-foreground text-xs'>
            {rfq.gpus_per_node * rfq.nodes} GPU · {rfq.term_months} {t('mo')} ·{' '}
            {t('from')} {rfq.start_date} · {rfq.regions}
          </p>
        </div>
        <dl className='grid grid-cols-[96px_1fr] gap-x-3 gap-y-1 text-xs'>
          <dt className='text-muted-foreground'>{t('Delivery')}</dt>
          <dd>
            {rfq.delivery} · {rfq.interconnect || '—'}
            {rfq.storage_tb ? ` · ${rfq.storage_tb} TB` : ''}
          </dd>
          <dt className='text-muted-foreground'>{t('Compliance')}</dt>
          <dd>{rfq.compliance || '—'}</dd>
          <dt className='text-muted-foreground'>{t('Ceiling')}</dt>
          <dd>
            <b>{formatPrice(rfq.price_ceiling)}</b> /GPU-hr
          </dd>
          <dt className='text-muted-foreground'>{t('Per year')}</dt>
          <dd>
            <b>{formatUsd(annualValue(rfq, lowest || rfq.price_ceiling))}</b>
          </dd>
          <dt className='text-muted-foreground'>{t('Buyer')}</dt>
          <dd>
            <Nickname
              alias={rfq.buyer_alias}
              hint={t(
                'Buyers are anonymous. Company and contact become visible after a match.'
              )}
            />
          </dd>
          {rfq.status === 'matching' && (
            <>
              <dt className='text-muted-foreground'>{t('Remaining')}</dt>
              <dd>{formatRemaining(rfq.matching_deadline, props.now)}</dd>
            </>
          )}
        </dl>
        {rfq.notes && (
          <p className='text-muted-foreground text-xs whitespace-pre-wrap'>
            {rfq.notes}
          </p>
        )}
        <div>
          <Label className='text-muted-foreground mb-1 text-[10px] tracking-wider uppercase'>
            {t('Bid ladder')} ·{' '}
            {d
              ? d.bids.filter((b) => b.status !== 'withdrawn').length
              : props.item.bid_count}
          </Label>
          {!d ? (
            <Skeleton className='h-12 w-full' />
          ) : d.bids.length === 0 ? (
            <p className='text-muted-foreground text-xs'>
              {t('No bids yet. The first bid is seen first.')}
            </p>
          ) : (
            <ul className='divide-y text-xs'>
              {d.bids
                .filter((b) => b.status !== 'withdrawn')
                .map((b: ComputeBid) => (
                  <li
                    key={b.id}
                    className='flex items-center justify-between py-1'
                  >
                    <span
                      className={cn(
                        !b.eligible && 'text-muted-foreground line-through'
                      )}
                    >
                      {b.rank ? `#${b.rank} · ` : ''}
                      {b.supplier_alias}
                      {b.mine && (
                        <Badge
                          variant='secondary'
                          className='ml-1 h-4 px-1 text-[10px]'
                        >
                          {t('you')}
                        </Badge>
                      )}
                      {b.deviations ? (
                        <span className='text-muted-foreground'>
                          {' '}
                          · {b.deviations}
                        </span>
                      ) : (
                        ''
                      )}
                    </span>
                    <span className='flex items-center gap-2'>
                      <b className={cn(b.rank === 1 && 'text-primary')}>
                        {formatPrice(b.price_per_gpu_hour)}
                      </b>
                      {d.is_owner && canAcceptBid(rfq, b, d.top) && (
                        <Button
                          size='sm'
                          className='h-6 px-2 text-[11px]'
                          disabled={accept.isPending}
                          onClick={() => accept.mutate(b.id)}
                        >
                          {t('Accept')}
                        </Button>
                      )}
                    </span>
                  </li>
                ))}
            </ul>
          )}
        </div>
        {open && d?.is_owner && (
          <div className='bg-foreground text-background rounded-lg p-3 text-xs'>
            <b>{t('This is your request.')}</b>{' '}
            {t(
              'Accept any eligible bid at any time; after the close you pick from the lowest 3.'
            )}
            <div className='mt-2'>
              <Button
                size='sm'
                variant='secondary'
                className='h-7'
                disabled={extend.isPending || rfq.extensions >= 3}
                onClick={() => extend.mutate()}
              >
                {t('Extend 24h')} ({rfq.extensions}/3)
              </Button>
            </div>
          </div>
        )}
        {open && !d?.is_owner && props.canBid && (
          <div className='rounded-lg border p-3'>
            <Label className='mb-1 text-xs' htmlFor='mk-price'>
              {myBid ? t('Lower your bid') : t('Your bid')} ·{' '}
              {t('must be below {{price}}', {
                price: formatPrice(lowest || rfq.price_ceiling),
              })}
            </Label>
            <div className='flex gap-2'>
              <Input
                id='mk-price'
                type='number'
                step='0.05'
                value={price}
                onChange={(e) => setPrice(e.target.value)}
              />
              <Button
                disabled={!valid || bid.isPending}
                onClick={() => bid.mutate()}
              >
                {myBid ? t('Lower') : t('Bid')}
              </Button>
            </div>
            <div className='mt-2 grid grid-cols-2 gap-2'>
              <div>
                <Label className='mb-1 text-[10px]' htmlFor='mk-date'>
                  {t('Deliverable from')}
                </Label>
                <Input
                  id='mk-date'
                  type='date'
                  value={deliverDate}
                  onChange={(e) => setDeliverDate(e.target.value)}
                />
              </div>
              <div>
                <Label className='mb-1 text-[10px]' htmlFor='mk-dev'>
                  {t('Soft deviations')}
                </Label>
                <Textarea
                  id='mk-dev'
                  rows={1}
                  value={deviations}
                  onChange={(e) => setDeviations(e.target.value)}
                />
              </div>
            </div>
            <div className='mt-2 flex flex-col gap-1'>
              {reqs.map((r) => (
                <label key={r} className='flex items-center gap-2 text-xs'>
                  <Checkbox
                    checked={Boolean(hard[r])}
                    onCheckedChange={(v) =>
                      setHard((h) => ({ ...h, [r]: Boolean(v) }))
                    }
                  />
                  {r}
                </label>
              ))}
              {!eligible && (
                <span className='text-xs text-amber-600'>
                  {t(
                    'Your bid will be visible but not eligible for selection.'
                  )}
                </span>
              )}
            </div>
            <p className='text-muted-foreground mt-2 text-[11px]'>
              {t(
                'Bids are public and binding. The buyer may accept yours at any time.'
              )}
              {myBid && (
                <>
                  {' '}
                  ·{' '}
                  <button
                    type='button'
                    className='underline'
                    onClick={() => withdraw.mutate(myBid.id)}
                  >
                    {t('Withdraw')}
                  </button>
                </>
              )}
            </p>
          </div>
        )}
        {open && !d?.is_owner && !props.canBid && (
          <div className='bg-foreground text-background rounded-lg p-3 text-xs'>
            <b>{t('Want to bid on this request?')}</b>{' '}
            {t(
              'Register as a supplier and verify one node (free). Bids are public and binding.'
            )}
            <div className='mt-2'>
              <Button
                size='sm'
                variant='secondary'
                className='h-7'
                onClick={props.onRegister}
              >
                {t('Become a supplier')} →
              </Button>
            </div>
          </div>
        )}
        {!open && props.item.accepted_bid && (
          <p className='text-muted-foreground text-xs'>
            {t('Matched at {{price}} · monthly ≈ {{monthly}}', {
              price: formatPrice(props.item.accepted_bid.price_per_gpu_hour),
              monthly: formatUsd(
                monthlyValue(rfq, props.item.accepted_bid.price_per_gpu_hour)
              ),
            })}
          </p>
        )}
      </CardContent>
    </Card>
  )
}

export function Marketplace({ onRegister }: { onRegister: () => void }) {
  const { t } = useTranslation()
  const q = useQuery({
    queryKey: [...openRFQsQueryKey, 'recent'],
    queryFn: () => listOpenRFQs(true),
    refetchInterval: 15000,
  })
  const profile = useQuery({
    queryKey: supplierQueryKey,
    queryFn: getSupplierProfile,
  })
  const now = useNowSeconds()
  const [gpu, setGpu] = useState('all')
  const [status, setStatus] = useState<MarketplaceStatusFilter>('all')
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)
  const [selected, setSelected] = useState<number | null>(null)
  const items = useMemo(() => q.data?.items ?? [], [q.data])
  const stats = useMemo(() => marketplaceStats(items, now), [items, now])
  const filtered = useMemo(
    () => filterMarketplace(items, { gpu, status, query, now }),
    [items, gpu, status, query, now]
  )
  const { pages, current, slice } = paginate(filtered, page, PER_PAGE)
  const selectedItem = items.find((i) => i.rfq.id === selected) ?? null
  const canBid = profile.data?.can_bid ?? false

  return (
    <div className='flex flex-col gap-3'>
      {!profile.isLoading && !canBid && (
        <div className='bg-foreground text-background flex flex-wrap items-center gap-3 rounded-lg px-4 py-3 text-sm'>
          <span className='flex-1'>
            {profile.data?.supplier
              ? t(
                  'Your supplier profile is registered. flatkey verifies capacity before you can bid — we will be in touch shortly.'
                )
              : t(
                  'Browse freely. To bid, register as a supplier and verify one node — free.'
                )}
          </span>
          {!profile.data?.supplier && (
            <Button size='sm' variant='secondary' onClick={onRegister}>
              {t('Become a supplier')} →
            </Button>
          )}
        </div>
      )}
      <div className='grid gap-2 md:grid-cols-4'>
        {[
          [formatUsd(stats.openValue), t('open demand · per year')],
          [String(stats.matching), t('requests matching')],
          [String(stats.bids), t('supplier bids · live')],
          [String(stats.endingSoon), t('ending within 1 hour')],
        ].map(([v, l]) => (
          <Card key={l}>
            <CardContent className='pt-4 pb-3'>
              <p className='text-2xl font-extrabold tracking-tight'>{v}</p>
              <p className='text-muted-foreground text-xs font-semibold'>{l}</p>
            </CardContent>
          </Card>
        ))}
      </div>
      <div className='flex flex-wrap items-center gap-1.5'>
        <span className='text-muted-foreground text-xs'>GPU</span>
        <Chip
          on={gpu === 'all'}
          onClick={() => {
            setGpu('all')
            setPage(1)
          }}
        >
          {t('All')}
        </Chip>
        {GPU_MODELS.map((g) => (
          <Chip
            key={g}
            on={gpu === g}
            onClick={() => {
              setGpu(g)
              setPage(1)
            }}
          >
            {g}
          </Chip>
        ))}
        <span className='text-muted-foreground ml-2 text-xs'>
          {t('Status')}
        </span>
        {(
          [
            ['all', t('All')],
            ['matching', t('Matching')],
            ['ending', t('Ending soon')],
            ['matched', t('Matched')],
            ['mine', t('Mine')],
          ] as [MarketplaceStatusFilter, string][]
        ).map(([v, l]) => (
          <Chip
            key={v}
            on={status === v}
            onClick={() => {
              setStatus(v)
              setPage(1)
            }}
          >
            {l}
          </Chip>
        ))}
        <Input
          className='ml-auto h-8 w-52'
          placeholder={t('Search GPU / region / alias')}
          value={query}
          onChange={(e) => {
            setQuery(e.target.value)
            setPage(1)
          }}
        />
      </div>
      <div
        className={cn(
          'grid gap-3',
          selectedItem ? 'lg:grid-cols-[minmax(0,1fr)_340px]' : ''
        )}
      >
        <div className='rounded-lg border'>
          {q.isLoading ? (
            <Skeleton className='h-48 w-full' />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Request')}</TableHead>
                  <TableHead>{t('Spec')}</TableHead>
                  <TableHead>{t('Term · region')}</TableHead>
                  <TableHead>{t('Ceiling / lowest')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead>{t('Buyer')}</TableHead>
                  <TableHead>{t('Per year')}</TableHead>
                  <TableHead />
                </TableRow>
              </TableHeader>
              <TableBody>
                {slice.length === 0 ? (
                  <TableRow>
                    <TableCell
                      colSpan={8}
                      className='text-muted-foreground py-8 text-center'
                    >
                      {t('No requests match these filters')}
                    </TableCell>
                  </TableRow>
                ) : (
                  slice.map((item) => {
                    const r = item.rfq
                    return (
                      <TableRow
                        key={r.id}
                        className={cn(
                          'cursor-pointer',
                          selected === r.id && 'bg-primary/5'
                        )}
                        onClick={() => setSelected(r.id)}
                      >
                        <TableCell className='whitespace-nowrap'>
                          <b>{r.code}</b>
                          <div className='text-muted-foreground text-xs'>
                            {r.buyer_alias}
                          </div>
                        </TableCell>
                        <TableCell className='whitespace-nowrap'>
                          <b>
                            {r.gpu_model} × {r.nodes}
                          </b>{' '}
                          <span className='text-muted-foreground text-xs'>
                            = {r.gpus_per_node * r.nodes} GPU
                          </span>
                          <div className='text-muted-foreground text-xs'>
                            {r.delivery} · {r.interconnect || '—'}
                          </div>
                        </TableCell>
                        <TableCell className='whitespace-nowrap'>
                          {r.term_months} {t('mo')}
                          <div className='text-muted-foreground text-xs'>
                            {r.regions}
                          </div>
                        </TableCell>
                        <TableCell className='whitespace-nowrap'>
                          <b className='text-primary'>
                            ≤ {formatPrice(r.price_ceiling)}
                          </b>
                          <div className='text-muted-foreground text-xs'>
                            {item.lowest_price
                              ? `${t('lowest')} ${formatPrice(item.lowest_price)}`
                              : item.accepted_bid
                                ? `${t('matched')} ${formatPrice(item.accepted_bid.price_per_gpu_hour)}`
                                : t('awaiting first bid')}
                          </div>
                        </TableCell>
                        <TableCell className='whitespace-nowrap'>
                          <RFQStatusBadge status={r.status} />
                          <div className='text-muted-foreground text-xs'>
                            {r.status === 'matching'
                              ? `${item.bid_count} ${t('bids')} · ${formatRemaining(r.matching_deadline, now)}`
                              : item.accepted_bid
                                ? formatPrice(
                                    item.accepted_bid.price_per_gpu_hour
                                  )
                                : ''}
                          </div>
                        </TableCell>
                        <TableCell>
                          <Nickname
                            alias={r.buyer_alias}
                            hint={t(
                              'Buyers are anonymous. Company and contact become visible after a match.'
                            )}
                          />
                        </TableCell>
                        <TableCell className='whitespace-nowrap'>
                          <b>
                            {formatUsd(
                              annualValue(
                                r,
                                item.lowest_price ||
                                  item.accepted_bid?.price_per_gpu_hour ||
                                  r.price_ceiling
                              )
                            )}
                          </b>
                        </TableCell>
                        <TableCell className='text-right'>
                          {isOpenStatus(r.status) && (
                            <Button
                              size='sm'
                              variant={item.my_bid ? 'outline' : 'default'}
                              onClick={(e) => {
                                e.stopPropagation()
                                setSelected(r.id)
                              }}
                            >
                              {item.my_bid
                                ? t('Lower')
                                : canBid
                                  ? t('Bid')
                                  : t('View')}
                            </Button>
                          )}
                        </TableCell>
                      </TableRow>
                    )
                  })
                )}
              </TableBody>
            </Table>
          )}
          <div className='text-muted-foreground flex items-center justify-between border-t px-3 py-2 text-xs'>
            <span>
              {t('{{count}} requests', { count: filtered.length })} ·{' '}
              {t('anonymous aliases')}
            </span>
            <span className='flex items-center gap-1'>
              <Button
                size='sm'
                variant='ghost'
                className='h-6 px-2'
                disabled={current <= 1}
                onClick={() => setPage(current - 1)}
              >
                ‹
              </Button>
              {current} / {pages}
              <Button
                size='sm'
                variant='ghost'
                className='h-6 px-2'
                disabled={current >= pages}
                onClick={() => setPage(current + 1)}
              >
                ›
              </Button>
            </span>
          </div>
        </div>
        {selectedItem && (
          <Detail
            key={selectedItem.rfq.id}
            item={selectedItem}
            canBid={canBid}
            now={now}
            onClose={() => setSelected(null)}
            onRegister={onRegister}
          />
        )}
      </div>
    </div>
  )
}
