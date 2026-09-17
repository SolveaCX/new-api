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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { placeBid } from '../api'
import {
  annualValue,
  formatPrice,
  formatUsd,
  hardRequirementsFor,
  suggestedBidPrice,
} from '../market'
import type { ComputeBid, ComputeRFQ } from '../types'

export function BidDialog(props: {
  rfq: ComputeRFQ
  lowest: number
  myBid: ComputeBid | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const reqs = hardRequirementsFor(props.rfq)
  const [price, setPrice] = useState(() =>
    String(
      suggestedBidPrice(
        props.rfq,
        props.lowest,
        props.myBid?.price_per_gpu_hour
      )
    )
  )
  const [deliverDate, setDeliverDate] = useState(props.rfq.start_date)
  const [sla, setSla] = useState('99.5')
  const [hard, setHard] = useState<Record<string, boolean>>(() =>
    Object.fromEntries(reqs.map((r) => [r, true]))
  )
  const [deviations, setDeviations] = useState('')

  const m = useMutation({
    mutationFn: () =>
      placeBid(props.rfq.id, {
        price_per_gpu_hour: Number(price),
        deliver_date: deliverDate,
        sla_pct: Number(sla),
        hard_reqs: hard,
        deviations,
      }),
    onSuccess: () => {
      toast.success(t('Bid placed — it is binding until the buyer decides'))
      void qc.invalidateQueries({ queryKey: ['compute-market'] })
      props.onOpenChange(false)
    },
    onError: (e) =>
      toast.error(e instanceof Error ? e.message : t('Could not place bid')),
  })

  const p = Number(price)
  const eligible = reqs.every((r) => hard[r])
  const valid =
    p > 0 &&
    p < props.rfq.price_ceiling &&
    (!props.lowest || p < props.lowest || Boolean(props.myBid))

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-w-xl'>
        <DialogHeader>
          <DialogTitle>
            {t('Bid on {{code}}', { code: props.rfq.code })}
          </DialogTitle>
          <DialogDescription>
            {props.rfq.gpu_model} × {props.rfq.nodes} ×{' '}
            {props.rfq.gpus_per_node} · {props.rfq.term_months} {t('mo')} ·{' '}
            {props.rfq.regions} · {t('ceiling')}{' '}
            {formatPrice(props.rfq.price_ceiling)} · {t('current lowest')}{' '}
            {formatPrice(props.lowest)}
          </DialogDescription>
        </DialogHeader>
        <div className='grid gap-4 md:grid-cols-2'>
          <div>
            <Label htmlFor='bid-price' className='mb-2'>
              {t('Your price (USD / GPU-hour)')}
            </Label>
            <Input
              id='bid-price'
              type='number'
              step='0.05'
              value={price}
              onChange={(e) => setPrice(e.target.value)}
            />
            <p className='text-muted-foreground mt-1 text-xs'>
              {t(
                'Annual value {{value}} · 3% fee · you receive {{net}}/GPU-hr',
                {
                  value: formatUsd(annualValue(props.rfq, p || 0)),
                  net: formatPrice((p || 0) * 0.97),
                }
              )}
            </p>
          </div>
          <div>
            <Label htmlFor='bid-date' className='mb-2'>
              {t('Deliverable from')}
            </Label>
            <Input
              id='bid-date'
              type='date'
              value={deliverDate}
              onChange={(e) => setDeliverDate(e.target.value)}
            />
            <p className='text-muted-foreground mt-1 text-xs'>
              {t('Buyer asked for {{date}}', { date: props.rfq.start_date })}
            </p>
          </div>
          <div>
            <Label htmlFor='bid-sla' className='mb-2'>
              {t('Committed SLA (%)')}
            </Label>
            <Input
              id='bid-sla'
              type='number'
              step='0.1'
              value={sla}
              onChange={(e) => setSla(e.target.value)}
            />
          </div>
          <div className='md:col-span-2'>
            <Label className='mb-2'>
              {t('Hard requirements — untick anything you cannot meet')}
            </Label>
            <div className='flex flex-col gap-2 rounded-md border p-3'>
              {reqs.map((r) => (
                <label key={r} className='flex items-center gap-2 text-sm'>
                  <Checkbox
                    checked={Boolean(hard[r])}
                    onCheckedChange={(v) =>
                      setHard((h) => ({ ...h, [r]: Boolean(v) }))
                    }
                  />
                  <span>{r}</span>
                </label>
              ))}
            </div>
            {!eligible && (
              <p className='mt-1 text-xs text-amber-600'>
                {t('Your bid will be visible but not eligible for selection.')}
              </p>
            )}
          </div>
          <div className='md:col-span-2'>
            <Label htmlFor='bid-dev' className='mb-2'>
              {t('Soft deviations (shown to the buyer)')}
            </Label>
            <Textarea
              id='bid-dev'
              rows={2}
              value={deviations}
              onChange={(e) => setDeviations(e.target.value)}
              placeholder='IB 200G instead of 400G'
            />
          </div>
        </div>
        <DialogFooter>
          <p className='text-muted-foreground mr-auto text-xs'>
            {t(
              'Binding: if the buyer accepts and you do not sign within 72h, your deposit is forfeited.'
            )}
          </p>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button disabled={!valid || m.isPending} onClick={() => m.mutate()}>
            {props.myBid
              ? t('Lower bid to {{price}}', { price: formatPrice(p || 0) })
              : t('Submit bid {{price}}', { price: formatPrice(p || 0) })}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
