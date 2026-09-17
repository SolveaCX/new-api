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
import { Textarea } from '@/components/ui/textarea'
import { getSupplierProfile, upsertSupplierProfile } from '../api'
import { supplierQueryKey } from '../keys'
import { REGIONS, formatPrice, splitList } from '../market'
import type { ComputeBid, ComputeSupplier } from '../types'

export function SupplierProfile() {
  const q = useQuery({
    queryKey: supplierQueryKey,
    queryFn: getSupplierProfile,
  })
  if (q.isLoading) return null
  return (
    <SupplierProfileForm
      key={
        q.data?.supplier
          ? `${q.data.supplier.id}:${q.data.supplier.updated_time}`
          : 'new'
      }
      supplier={q.data?.supplier ?? null}
      alias={q.data?.alias ?? '#S-…'}
      bids={q.data?.bids ?? []}
    />
  )
}

function SupplierProfileForm(props: {
  supplier: ComputeSupplier | null
  alias: string
  bids: ComputeBid[]
}) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const s = props.supplier
  const [company, setCompany] = useState(s?.company ?? '')
  const [contact, setContact] = useState(s?.contact ?? '')
  const [regions, setRegions] = useState<string[]>(() =>
    splitList(s?.regions ?? '')
  )
  const [inventory, setInventory] = useState(s?.inventory ?? '')

  const save = useMutation({
    mutationFn: () =>
      upsertSupplierProfile({ company, contact, regions, inventory }),
    onSuccess: () => {
      toast.success(t('Supplier profile saved'))
      void qc.invalidateQueries({ queryKey: supplierQueryKey })
    },
    onError: (e) =>
      toast.error(e instanceof Error ? e.message : t('Could not save profile')),
  })

  const level = s?.level ?? -1
  const levelLabel =
    level === 2
      ? t('L2 · preferred')
      : level === 1
        ? t('L1 · verified')
        : level === 0
          ? t('L0 · registered, verification pending')
          : t('Not registered')

  return (
    <div className='grid gap-4 lg:grid-cols-[1.4fr_1fr]'>
      <Card>
        <CardHeader>
          <CardTitle>{t('Supplier profile')}</CardTitle>
          <CardDescription>
            {t(
              'Company and contact are visible to flatkey only. Buyers see you as {{alias}}.',
              { alias: props.alias }
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className='grid gap-4 md:grid-cols-2'>
          <div>
            <Label htmlFor='sp-company' className='mb-2'>
              {t('Company')}
            </Label>
            <Input
              id='sp-company'
              value={company}
              onChange={(e) => setCompany(e.target.value)}
            />
          </div>
          <div>
            <Label htmlFor='sp-contact' className='mb-2'>
              {t('Contact (email or phone, flatkey only)')}
            </Label>
            <Input
              id='sp-contact'
              value={contact}
              onChange={(e) => setContact(e.target.value)}
            />
          </div>
          <div className='md:col-span-2'>
            <Label className='mb-2'>{t('Regions with capacity')}</Label>
            <div className='flex flex-wrap gap-2'>
              {REGIONS.map((r) => (
                <button
                  key={r}
                  type='button'
                  onClick={() =>
                    setRegions((rs) =>
                      rs.includes(r) ? rs.filter((x) => x !== r) : [...rs, r]
                    )
                  }
                  className={cn(
                    'rounded-full border px-3 py-1 text-sm font-semibold',
                    regions.includes(r)
                      ? 'border-foreground bg-foreground text-background'
                      : 'hover:bg-muted'
                  )}
                >
                  {r}
                </button>
              ))}
            </div>
          </div>
          <div className='md:col-span-2'>
            <Label htmlFor='sp-inv' className='mb-2'>
              {t('Inventory (GPU model × nodes × region · available from)')}
            </Label>
            <Textarea
              id='sp-inv'
              rows={4}
              value={inventory}
              onChange={(e) => setInventory(e.target.value)}
              placeholder='24 × B300 (8/node) · Tokyo Tier 3 · from 2026-11-01'
            />
          </div>
          <div className='md:col-span-2'>
            <Button
              disabled={
                !company.trim() || regions.length === 0 || save.isPending
              }
              onClick={() => save.mutate()}
            >
              {s ? t('Save profile') : t('Register as supplier')}
            </Button>
          </div>
        </CardContent>
      </Card>
      <div className='flex flex-col gap-4'>
        <Card>
          <CardHeader>
            <CardTitle>{t('Verification level')}</CardTitle>
          </CardHeader>
          <CardContent className='flex flex-col gap-2 text-sm'>
            <Badge
              variant={level >= 1 ? 'default' : 'secondary'}
              className='w-fit'
            >
              {levelLabel}
            </Badge>
            <p className='text-muted-foreground'>
              {t(
                'L0 can browse requests. L1 (one node verified through the flatkey agent + deposit) can bid. L2 (3 contracts + certification) is marked preferred on every bid.'
              )}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>{t('Your bids')}</CardTitle>
          </CardHeader>
          <CardContent className='text-sm'>
            {props.bids.length === 0 ? (
              <p className='text-muted-foreground'>{t('No bids yet')}</p>
            ) : (
              <ul className='flex flex-col gap-2'>
                {props.bids.map((b) => (
                  <li key={b.id} className='flex justify-between'>
                    <span>RFQ-{b.rfq_id}</span>
                    <span className='font-semibold'>
                      {formatPrice(b.price_per_gpu_hour)} · {b.status}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
