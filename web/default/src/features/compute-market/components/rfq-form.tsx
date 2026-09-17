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
import { useMutation } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
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
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Textarea } from '@/components/ui/textarea'
import { createRFQ, parseRFQText } from '../api'
import {
  COMPLIANCE_OPTIONS,
  DELIVERY_OPTIONS,
  GPU_MODELS,
  REGIONS,
  annualValue,
  applyDraft,
  emptyRFQForm,
  formatUsd,
  monthlyValue,
} from '../market'
import type { RFQFormValues } from '../types'

function Chip(props: {
  active: boolean
  onClick: () => void
  children: React.ReactNode
  warn?: boolean
}) {
  return (
    <button
      type='button'
      onClick={props.onClick}
      className={cn(
        'rounded-full border px-3 py-1 text-sm font-semibold transition-colors',
        props.active
          ? 'border-foreground bg-foreground text-background'
          : 'border-border bg-background hover:bg-muted',
        props.warn && !props.active && 'border-dashed border-amber-500'
      )}
    >
      {props.children}
    </button>
  )
}

function AITag({ on }: { on: boolean }) {
  const { t } = useTranslation()
  if (!on) return null
  return (
    <Badge
      variant='secondary'
      className='ml-2 h-4 px-1.5 text-[10px] leading-none'
    >
      {t('AI')}
    </Badge>
  )
}

export function RFQForm() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [form, setForm] = useState<RFQFormValues>(emptyRFQForm)
  const [pasted, setPasted] = useState('')
  const [filled, setFilled] = useState<Set<string>>(new Set())
  const [missing, setMissing] = useState<string[]>([])
  const [source, setSource] = useState<string>('')

  const parse = useMutation({
    mutationFn: () => parseRFQText(pasted),
    onSuccess: (draft) => {
      setForm((f) => applyDraft(f, draft))
      setFilled(new Set(draft.filled ?? []))
      setMissing(draft.missing ?? [])
      setSource(draft.source)
      toast.success(
        t('Filled {{count}} fields from your text', {
          count: draft.filled.length,
        })
      )
    },
    onError: (e) =>
      toast.error(e instanceof Error ? e.message : t('Could not parse text')),
  })

  const create = useMutation({
    mutationFn: () => createRFQ(form),
    onSuccess: ({ rfq }) => {
      toast.success(t('Request {{code}} is now matching', { code: rfq.code }))
      void navigate({
        to: '/compute/market/$rfqId',
        params: { rfqId: String(rfq.id) },
      })
    },
    onError: (e) =>
      toast.error(e instanceof Error ? e.message : t('Could not post request')),
  })

  const set = <K extends keyof RFQFormValues>(
    key: K,
    value: RFQFormValues[K]
  ) => setForm((f) => ({ ...f, [key]: value }))
  const toggle = (key: 'regions' | 'compliance', value: string) =>
    setForm((f) => ({
      ...f,
      [key]: f[key].includes(value)
        ? f[key].filter((v) => v !== value)
        : [...f[key], value],
    }))
  const num = (v: string) => (v === '' ? 0 : Number(v))
  const isFilled = (k: string) => filled.has(k)
  const isMissing = (k: string) => missing.includes(k)
  const ready =
    form.gpu_model &&
    form.nodes > 0 &&
    form.term_months > 0 &&
    form.start_date &&
    form.price_ceiling > 0 &&
    form.regions.length > 0

  return (
    <div className='grid gap-4 lg:grid-cols-[1.6fr_1fr]'>
      <div className='flex flex-col gap-4'>
        <Card className='border-primary/40'>
          <CardHeader>
            <CardTitle className='flex items-center gap-2'>
              <Sparkles className='size-4' />{' '}
              {t('Paste a paragraph, auto-fill the form')}
            </CardTitle>
            <CardDescription>
              {t(
                'An email, a chat message or a procurement memo works. Chinese, English and Japanese are fine. Every field stays editable.'
              )}
            </CardDescription>
          </CardHeader>
          <CardContent className='flex flex-col gap-3'>
            <Textarea
              rows={4}
              value={pasted}
              onChange={(e) => setPasted(e.target.value)}
              placeholder={t(
                'e.g. We need 20 B300 nodes in Japan from Nov 1 for one year, budget $3.5–5 per GPU-hour, bare metal with IB 400G, SOC 2, data must stay in Japan.'
              )}
            />
            <div className='flex flex-wrap items-center gap-2'>
              <Button
                onClick={() => parse.mutate()}
                disabled={!pasted.trim() || parse.isPending}
              >
                <Sparkles className='size-4' />{' '}
                {parse.isPending ? t('Parsing…') : t('Parse and fill')}
              </Button>
              {source && (
                <Badge variant='outline'>
                  {t('Filled {{count}} fields', { count: filled.size })} ·{' '}
                  {source === 'ai' ? t('AI') : t('rules')}
                </Badge>
              )}
              {missing.length > 0 && (
                <Badge variant='secondary'>
                  {t('Please confirm: {{fields}}', {
                    fields: missing.join(', '),
                  })}
                </Badge>
              )}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t('Hardware and scale')}</CardTitle>
            <CardDescription>
              {t(
                'Hard requirements — suppliers must confirm each one to be eligible.'
              )}
            </CardDescription>
          </CardHeader>
          <CardContent className='grid gap-4 md:grid-cols-2'>
            <div className='md:col-span-2'>
              <Label className='mb-2 flex items-center'>
                {t('GPU model')}
                <AITag on={isFilled('gpu_model')} />
              </Label>
              <div className='flex flex-wrap gap-2'>
                {GPU_MODELS.map((g) => (
                  <Chip
                    key={g}
                    active={form.gpu_model === g}
                    onClick={() => set('gpu_model', g)}
                  >
                    {g}
                  </Chip>
                ))}
              </div>
            </div>
            <div>
              <Label className='mb-2 flex items-center' htmlFor='gpus_per_node'>
                {t('GPUs per node')}
                <AITag on={isFilled('gpus_per_node')} />
              </Label>
              <Input
                id='gpus_per_node'
                type='number'
                min={1}
                max={64}
                value={form.gpus_per_node || ''}
                onChange={(e) => set('gpus_per_node', num(e.target.value))}
              />
            </div>
            <div>
              <Label className='mb-2 flex items-center' htmlFor='nodes'>
                {t('Nodes')}
                <AITag on={isFilled('nodes')} />
              </Label>
              <Input
                id='nodes'
                type='number'
                min={1}
                value={form.nodes || ''}
                onChange={(e) => set('nodes', num(e.target.value))}
                className={cn(isMissing('nodes') && 'border-amber-500')}
              />
              <p className='text-muted-foreground mt-1 text-xs'>
                = {form.gpus_per_node * form.nodes} GPU
              </p>
            </div>
            <div>
              <Label className='mb-2 flex items-center' htmlFor='term'>
                {t('Term (months)')}
                <AITag on={isFilled('term_months')} />
              </Label>
              <Input
                id='term'
                type='number'
                min={1}
                max={60}
                value={form.term_months || ''}
                onChange={(e) => set('term_months', num(e.target.value))}
                className={cn(isMissing('term_months') && 'border-amber-500')}
              />
            </div>
            <div>
              <Label className='mb-2 flex items-center' htmlFor='start'>
                {t('Start date')}
                <AITag on={isFilled('start_date')} />
              </Label>
              <Input
                id='start'
                type='date'
                value={form.start_date}
                onChange={(e) => set('start_date', e.target.value)}
                className={cn(isMissing('start_date') && 'border-amber-500')}
              />
            </div>
            <div className='md:col-span-2'>
              <Label className='mb-2 flex items-center'>
                {t('Regions')}
                <AITag on={isFilled('regions')} />
              </Label>
              <div className='flex flex-wrap gap-2'>
                {REGIONS.map((r) => (
                  <Chip
                    key={r}
                    active={form.regions.includes(r)}
                    onClick={() => toggle('regions', r)}
                    warn={isMissing('regions')}
                  >
                    {r}
                  </Chip>
                ))}
              </div>
              <p className='text-muted-foreground mt-1 text-xs'>
                {t(
                  'Suppliers outside the selected regions are visible but not eligible.'
                )}
              </p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t('Price')}</CardTitle>
            <CardDescription>
              {t(
                'Only the ceiling is public. Your target band is visible to flatkey only.'
              )}
            </CardDescription>
          </CardHeader>
          <CardContent className='grid gap-4 md:grid-cols-3'>
            <div>
              <Label className='mb-2 flex items-center' htmlFor='ceiling'>
                {t('Price ceiling (public)')}
                <AITag on={isFilled('price_ceiling')} />
              </Label>
              <Input
                id='ceiling'
                type='number'
                step='0.05'
                min={0}
                value={form.price_ceiling || ''}
                onChange={(e) => set('price_ceiling', num(e.target.value))}
                className={cn(isMissing('price_ceiling') && 'border-amber-500')}
              />
              <p className='text-muted-foreground mt-1 text-xs'>
                {t(
                  'USD per GPU-hour · this is the starting price of the auction'
                )}
              </p>
            </div>
            <div>
              <Label className='mb-2' htmlFor='tmin'>
                {t('Target min (private)')}
              </Label>
              <Input
                id='tmin'
                type='number'
                step='0.05'
                min={0}
                value={form.target_price_min || ''}
                onChange={(e) => set('target_price_min', num(e.target.value))}
              />
            </div>
            <div>
              <Label className='mb-2' htmlFor='tmax'>
                {t('Target max (private)')}
              </Label>
              <Input
                id='tmax'
                type='number'
                step='0.05'
                min={0}
                value={form.target_price_max || ''}
                onChange={(e) => set('target_price_max', num(e.target.value))}
              />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t('Delivery and compliance')}</CardTitle>
            <CardDescription>
              {t(
                'Soft deviations are allowed — suppliers declare them and you see them on every bid.'
              )}
            </CardDescription>
          </CardHeader>
          <CardContent className='grid gap-4 md:grid-cols-3'>
            <div>
              <Label className='mb-2 flex items-center' htmlFor='delivery'>
                {t('Delivery')}
                <AITag on={isFilled('delivery')} />
              </Label>
              <NativeSelect
                id='delivery'
                value={form.delivery}
                onChange={(e) => set('delivery', e.target.value)}
              >
                {DELIVERY_OPTIONS.map((d) => (
                  <NativeSelectOption key={d} value={d}>
                    {d === 'bare_metal'
                      ? t('Bare metal')
                      : d === 'vm'
                        ? t('VM')
                        : d === 'slurm'
                          ? 'Slurm'
                          : 'Kubernetes'}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </div>
            <div>
              <Label className='mb-2 flex items-center' htmlFor='ic'>
                {t('Interconnect')}
                <AITag on={isFilled('interconnect')} />
              </Label>
              <Input
                id='ic'
                value={form.interconnect}
                placeholder='IB 400G'
                onChange={(e) => set('interconnect', e.target.value)}
              />
            </div>
            <div>
              <Label className='mb-2 flex items-center' htmlFor='storage'>
                {t('Storage (TB)')}
                <AITag on={isFilled('storage_tb')} />
              </Label>
              <Input
                id='storage'
                type='number'
                min={0}
                value={form.storage_tb || ''}
                onChange={(e) => set('storage_tb', num(e.target.value))}
              />
            </div>
            <div className='md:col-span-3'>
              <Label className='mb-2 flex items-center'>
                {t('Compliance')}
                <AITag on={isFilled('compliance')} />
              </Label>
              <div className='flex flex-wrap gap-2'>
                {COMPLIANCE_OPTIONS.map((c) => (
                  <Chip
                    key={c}
                    active={form.compliance.includes(c)}
                    onClick={() => toggle('compliance', c)}
                  >
                    {c}
                  </Chip>
                ))}
              </div>
            </div>
            <div className='md:col-span-3'>
              <Label className='mb-2' htmlFor='notes'>
                {t('Notes for suppliers')}
              </Label>
              <Textarea
                id='notes'
                rows={3}
                value={form.notes}
                onChange={(e) => set('notes', e.target.value)}
              />
              <p className='text-muted-foreground mt-1 text-xs'>
                {t(
                  'Emails, phone numbers and links are removed automatically. All communication stays on flatkey.'
                )}
              </p>
            </div>
          </CardContent>
        </Card>

        <div className='flex flex-wrap items-center gap-3'>
          <Button
            size='lg'
            disabled={!ready || create.isPending}
            onClick={() => create.mutate()}
          >
            {create.isPending
              ? t('Posting…')
              : t('Post request and start matching')}
          </Button>
          <span className='text-muted-foreground text-sm'>
            {t(
              'Your request is listed anonymously as a buyer alias. Suppliers bid for 24 hours; you can accept any eligible bid at any time.'
            )}
          </span>
        </div>
      </div>

      <div className='flex flex-col gap-4'>
        <Card className='bg-foreground text-background'>
          <CardContent className='flex flex-col gap-3 pt-6'>
            <div>
              <p className='text-xs opacity-70'>
                {t('Annual value at ceiling')}
              </p>
              <p className='text-3xl font-extrabold tracking-tight'>
                {formatUsd(annualValue(form, form.price_ceiling))}
              </p>
              <p className='text-xs opacity-70'>
                {form.gpus_per_node * form.nodes} GPU × $
                {form.price_ceiling || 0} × 8,760h ·{' '}
                {t('auctions typically settle at 75–85% of the ceiling')}
              </p>
            </div>
            <div className='border-t border-white/15 pt-3'>
              <p className='text-xs opacity-70'>
                {t('First escrow (first month + 1 month deposit)')}
              </p>
              <p className='text-2xl font-extrabold tracking-tight'>
                ≈ {formatUsd(monthlyValue(form, form.price_ceiling) * 2)}
              </p>
              <p className='text-xs opacity-70'>
                {t(
                  'released to the supplier only after acceptance and monthly SLA'
                )}
              </p>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>{t('What happens after you post')}</CardTitle>
          </CardHeader>
          <CardContent className='text-sm'>
            <ul className='flex flex-col gap-2'>
              <li>
                ·{' '}
                {t(
                  'Listed anonymously to verified suppliers matching your regions and GPU'
                )}
              </li>
              <li>
                ·{' '}
                {t(
                  '24h public reverse auction — bids only go down; last-15-minute bids extend the window'
                )}
              </li>
              <li>
                ·{' '}
                {t(
                  'You see every bid live: price, delivery date, credit, deviations'
                )}
              </li>
              <li>
                ·{' '}
                {t(
                  'Accept any eligible bid at any time, or pick from the lowest 3 after the close'
                )}
              </li>
              <li>
                ·{' '}
                {t(
                  'One contract with flatkey; escrow released after acceptance tests'
                )}
              </li>
            </ul>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
