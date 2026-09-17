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
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { getSupplierProfile, listOpenRFQs } from '../api'
import { openRFQsQueryKey, supplierQueryKey } from '../keys'
import { formatPrice, formatRemaining } from '../market'
import type { OpenRFQItem } from '../types'
import { useNowSeconds } from '../use-now'
import { BidDialog } from './bid-dialog'
import { RFQStatusBadge } from './status-badge'

export function SupplierBoard({ onRegister }: { onRegister: () => void }) {
  const { t } = useTranslation()
  const q = useQuery({
    queryKey: openRFQsQueryKey,
    queryFn: listOpenRFQs,
    refetchInterval: 15000,
  })
  const profile = useQuery({
    queryKey: supplierQueryKey,
    queryFn: getSupplierProfile,
  })
  const [target, setTarget] = useState<OpenRFQItem | null>(null)
  const now = useNowSeconds()
  const canBid = profile.data?.can_bid ?? false

  return (
    <div className='flex flex-col gap-4'>
      {!profile.isLoading && !canBid && (
        <Card className='border-amber-400 bg-amber-50 dark:bg-amber-950/20'>
          <CardContent className='flex flex-wrap items-center gap-3 pt-6 text-sm'>
            <span>
              {profile.data?.supplier
                ? t(
                    'Your supplier profile is registered. flatkey verifies capacity before you can bid — we will be in touch shortly.'
                  )
                : t(
                    'Register as a supplier to bid. Registration is free; flatkey verifies one node before your first bid.'
                  )}
            </span>
            {!profile.data?.supplier && (
              <Button size='sm' onClick={onRegister}>
                {t('Register as supplier')}
              </Button>
            )}
          </CardContent>
        </Card>
      )}
      <p className='text-muted-foreground text-sm'>
        {t(
          'Open requests. Buyers are anonymous; bids are public to suppliers and only go down. The buyer may accept any eligible bid at any time.'
        )}
      </p>
      {q.isLoading ? (
        <Skeleton className='h-40 w-full' />
      ) : (
        <div className='rounded-lg border'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Request')}</TableHead>
                <TableHead>{t('Spec')}</TableHead>
                <TableHead>{t('Term · region')}</TableHead>
                <TableHead>{t('Ceiling')}</TableHead>
                <TableHead>{t('Lowest')}</TableHead>
                <TableHead>{t('Remaining')}</TableHead>
                <TableHead>{t('Your bid')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(q.data?.items ?? []).length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={8}
                    className='text-muted-foreground py-8 text-center'
                  >
                    {t('No open requests right now')}
                  </TableCell>
                </TableRow>
              ) : (
                (q.data?.items ?? []).map((item) => (
                  <TableRow key={item.rfq.id}>
                    <TableCell>
                      <div className='font-semibold'>{item.rfq.code}</div>
                      <div className='text-muted-foreground font-mono text-xs'>
                        {item.rfq.buyer_alias}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className='font-semibold'>
                        {item.rfq.gpu_model} × {item.rfq.nodes} {t('nodes')} ={' '}
                        {item.rfq.gpus_per_node * item.rfq.nodes} GPU
                      </div>
                      <div className='text-muted-foreground text-xs'>
                        {item.rfq.delivery} · {item.rfq.interconnect || '—'}
                        {item.rfq.storage_tb
                          ? ` · ${item.rfq.storage_tb} TB`
                          : ''}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div>
                        {item.rfq.term_months} {t('mo')} · {t('from')}{' '}
                        {item.rfq.start_date}
                      </div>
                      <div className='text-muted-foreground text-xs'>
                        {item.rfq.regions}
                      </div>
                    </TableCell>
                    <TableCell>{formatPrice(item.rfq.price_ceiling)}</TableCell>
                    <TableCell>
                      <span className='font-semibold'>
                        {formatPrice(item.lowest_price)}
                      </span>
                      <span className='text-muted-foreground ml-1 text-xs'>
                        ({item.bid_count})
                      </span>
                    </TableCell>
                    <TableCell>
                      <RFQStatusBadge status={item.rfq.status} />
                      <div className='text-muted-foreground mt-1 text-xs'>
                        {formatRemaining(item.rfq.matching_deadline, now)}
                      </div>
                    </TableCell>
                    <TableCell>
                      {item.my_bid ? (
                        <Badge variant='secondary'>
                          {formatPrice(item.my_bid.price_per_gpu_hour)}
                          {item.my_bid.rank ? ` · #${item.my_bid.rank}` : ''}
                        </Badge>
                      ) : (
                        <span className='text-muted-foreground text-xs'>
                          {t('none')}
                        </span>
                      )}
                    </TableCell>
                    <TableCell className='text-right'>
                      <div className='flex justify-end gap-2'>
                        <Button
                          variant='outline'
                          size='sm'
                          render={
                            <Link
                              to='/compute/market/$rfqId'
                              params={{ rfqId: String(item.rfq.id) }}
                            />
                          }
                        >
                          {t('Details')}
                        </Button>
                        <Button
                          size='sm'
                          disabled={!canBid}
                          onClick={() => setTarget(item)}
                        >
                          {item.my_bid ? t('Lower') : t('Bid')}
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      )}
      {target && (
        <BidDialog
          rfq={target.rfq}
          lowest={target.lowest_price}
          myBid={target.my_bid}
          open={Boolean(target)}
          onOpenChange={(o) => !o && setTarget(null)}
        />
      )}
    </div>
  )
}
