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
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { GPU_INSTANCES, type GpuInstance } from '../catalog'

function InstanceSpec(props: { label: string; value: string }) {
  return (
    <div className='text-muted-foreground flex justify-between'>
      <span>{props.label}</span>
      <span className='text-foreground font-medium'>{props.value}</span>
    </div>
  )
}

function InstanceCard(props: { gpu: GpuInstance }) {
  const { t } = useTranslation()
  const { gpu } = props
  return (
    <div className='bg-card flex flex-col rounded-lg border p-4'>
      <div className='mb-2 flex items-center gap-2'>
        <Badge variant='secondary'>NVIDIA</Badge>
        <Badge variant='outline'>SECURE</Badge>
      </div>
      <div className='flex items-center justify-between'>
        <span className='font-semibold'>{gpu.name}</span>
        <Badge
          variant='secondary'
          className={gpu.availability === 'high' ? 'text-success' : 'text-warning'}
        >
          {gpu.availability === 'high' ? t('In stock') : t('Limited')}
        </Badge>
      </div>
      <div className='text-muted-foreground mt-0.5 text-xs'>
        Max CUDA {gpu.maxCuda}
      </div>
      <div className='mt-3 grid grid-cols-2 gap-x-4 gap-y-1 text-xs'>
        <InstanceSpec label='VRAM' value={gpu.vram} />
        <InstanceSpec label='vCPU' value={gpu.vcpu} />
        <InstanceSpec label='RAM' value={gpu.ram} />
      </div>
      <div className='mt-3 flex items-end justify-between border-t pt-3'>
        <div>
          <div className='text-lg font-bold'>
            {gpu.onDemand}
            <span className='text-muted-foreground text-xs font-normal'>/hr</span>
          </div>
          {gpu.spot && (
            <div className='text-success text-xs font-medium'>
              {t('Spot')} {gpu.spot}/hr
            </div>
          )}
        </div>
        <Button size='sm' disabled>
          {t('Coming soon')}
        </Button>
      </div>
    </div>
  )
}

export function GpuInstances() {
  const { t } = useTranslation()
  return (
    <div className='flex flex-col gap-4'>
      <div className='bg-primary/5 border-primary/20 rounded-md border p-3 text-sm'>
        {t(
          'Rent a whole GPU by the hour — SSH or Jupyter in and run your own environment. Launching soon; specs and pricing below are a preview.'
        )}
      </div>
      <div className='grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3'>
        {GPU_INSTANCES.map((g) => (
          <InstanceCard key={g.id} gpu={g} />
        ))}
      </div>
      <div className='bg-warning/10 border-warning/30 rounded-md border p-3 text-xs'>
        {t(
          'Illustrative pricing, pending calibration. Final rate is set from real upstream cost plus a thin margin.'
        )}
      </div>
    </div>
  )
}
