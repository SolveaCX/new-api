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
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { listMyRFQs } from '../api'
import { myRFQsQueryKey } from '../keys'
import {
  formatPrice,
  formatRemaining,
  formatUsd,
  isOpenStatus,
  monthlyValue,
} from '../market'
import { useNowSeconds } from '../use-now'
import { RFQStatusBadge } from './status-badge'

export function MyRequests({ onPost }: { onPost: () => void }) {
  const { t } = useTranslation()
  const q = useQuery({
    queryKey: myRFQsQueryKey,
    queryFn: listMyRFQs,
    refetchInterval: 15000,
  })
  const now = useNowSeconds()

  if (q.isLoading) {
    return (
      <div className='flex flex-col gap-3'>
        <Skeleton className='h-24 w-full' />
        <Skeleton className='h-24 w-full' />
      </div>
    )
  }
  const items = q.data?.items ?? []
  if (items.length === 0) {
    return (
      <Card>
        <CardContent className='flex flex-col items-start gap-3 py-10'>
          <p className='text-lg font-bold'>{t('No compute requests yet')}</p>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Post what you need — GPU, nodes, term, region, ceiling price — and verified suppliers bid within 24 hours. You stay anonymous.'
            )}
          </p>
          <Button onClick={onPost}>{t('Post a request')}</Button>
        </CardContent>
      </Card>
    )
  }
  return (
    <div className='flex flex-col gap-3'>
      {items.map(
        ({ rfq, bid_count, eligible_count, lowest_price, accepted_bid }) => (
          <Card
            key={rfq.id}
            className={
              isOpenStatus(rfq.status) ? 'border-foreground' : undefined
            }
          >
            <CardContent className='grid items-center gap-4 pt-6 md:grid-cols-[1.4fr_1fr_1fr_1fr_auto]'>
              <div>
                <p className='text-muted-foreground font-mono text-xs'>
                  {rfq.code}
                </p>
                <p className='text-lg font-extrabold'>
                  {rfq.gpu_model} × {rfq.nodes} · {rfq.term_months} {t('mo')} ·{' '}
                  {rfq.regions}
                </p>
                <p className='text-muted-foreground text-xs'>
                  {rfq.delivery} · {rfq.interconnect || '—'} · {t('ceiling')}{' '}
                  {formatPrice(rfq.price_ceiling)}
                </p>
              </div>
              <div>
                <RFQStatusBadge status={rfq.status} />
                <p className='text-muted-foreground mt-1 text-xs'>
                  {rfq.status === 'matching' &&
                    t('{{bids}} bids · {{remaining}} left', {
                      bids: bid_count,
                      remaining: formatRemaining(rfq.matching_deadline, now),
                    })}
                  {rfq.status === 'choosing' &&
                    t('{{count}} eligible · pick one or extend', {
                      count: eligible_count,
                    })}
                  {rfq.status === 'matched' &&
                    accepted_bid &&
                    t('You accepted {{alias}} at {{price}}', {
                      alias: accepted_bid.supplier_alias,
                      price: formatPrice(accepted_bid.price_per_gpu_hour),
                    })}
                  {rfq.status === 'cancelled' && t('Withdrawn')}
                </p>
              </div>
              <div>
                <p className='text-xl font-extrabold'>
                  {accepted_bid
                    ? formatPrice(accepted_bid.price_per_gpu_hour)
                    : formatPrice(lowest_price)}
                </p>
                <p className='text-muted-foreground text-xs'>
                  {accepted_bid ? t('matched price') : t('lowest eligible bid')}
                </p>
              </div>
              <div>
                <p className='text-xl font-extrabold'>
                  {accepted_bid
                    ? formatUsd(
                        monthlyValue(rfq, accepted_bid.price_per_gpu_hour) * 2
                      )
                    : String(eligible_count)}
                </p>
                <p className='text-muted-foreground text-xs'>
                  {accepted_bid ? t('first escrow') : t('bids you can accept')}
                </p>
              </div>
              <Button
                variant={isOpenStatus(rfq.status) ? 'default' : 'outline'}
                render={
                  <Link
                    to='/compute/market/$rfqId'
                    params={{ rfqId: String(rfq.id) }}
                  />
                }
              >
                {isOpenStatus(rfq.status) ? t('View bids') : t('Open')}
              </Button>
            </CardContent>
          </Card>
        )
      )}
    </div>
  )
}
