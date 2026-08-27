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
import { Plus, Save, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { SettingsPageActionsPortal } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import {
  calculateModel,
  calculateSummary,
  getCashInputs,
  getDiscountForModel,
} from './cost-calculation'
import { COST_CALCULATION_OPTION_KEY } from './cost-calculation-defaults'
import type { CostCalculationConfig } from './types'

type CostCalculationSectionProps = {
  defaultValue: CostCalculationConfig
}

function currency(value: number | null) {
  return value === null ? '-' : `$${value.toFixed(6)}`
}

function percent(value: number | null) {
  return value === null ? '-' : `${(value * 100).toFixed(1)}%`
}

function normalizeConfig(config: CostCalculationConfig): CostCalculationConfig {
  return {
    ...config,
    fields: config.fields ?? [],
    models: config.models.map((model) => ({
      ...model,
      customFields: model.customFields ?? {},
    })),
  }
}

export function CostCalculationSection(props: CostCalculationSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [config, setConfig] = useState(() =>
    normalizeConfig(props.defaultValue)
  )
  const [filter, setFilter] = useState('')
  const [newField, setNewField] = useState('')
  const [newModel, setNewModel] = useState({
    name: '',
    vendor: '',
    billingUnit: 'USD/1M tokens',
    officialInput: '0',
    officialOutput: '0',
    officialNonToken: '0',
    costDiscount: '',
    note: '',
  })

  const cashInputs = useMemo(
    () => getCashInputs(config.assumptions),
    [config.assumptions]
  )
  const summary = useMemo(
    () => calculateSummary(config.models, config.assumptions, config.discounts),
    [config]
  )
  const models = useMemo(
    () =>
      config.models.filter((model) =>
        `${model.name} ${model.vendor}`
          .toLowerCase()
          .includes(filter.toLowerCase())
      ),
    [config.models, filter]
  )

  const updateAssumption = (
    key: keyof typeof config.assumptions,
    value: string
  ) => {
    const parsed = Number(value)
    setConfig((current) => ({
      ...current,
      assumptions: {
        ...current.assumptions,
        [key]: Number.isFinite(parsed) ? parsed : 0,
      },
    }))
  }

  const updateModel = (
    name: string,
    key:
      | 'name'
      | 'vendor'
      | 'billingUnit'
      | 'officialInput'
      | 'officialOutput'
      | 'officialNonToken'
      | 'costDiscount'
      | 'note'
      | `custom:${string}`,
    value: string
  ) => {
    const isNumeric =
      key === 'officialInput' ||
      key === 'officialOutput' ||
      key === 'officialNonToken' ||
      key === 'costDiscount'
    const parsed = value.trim() === '' ? null : Number(value)
    setConfig((current) => ({
      ...current,
      models: current.models.map((model) =>
        model.name === name
          ? {
              ...model,
              ...(key.startsWith('custom:')
                ? {
                    customFields: {
                      ...(model.customFields ?? {}),
                      [key.slice('custom:'.length)]: value,
                    },
                  }
                : {
                    [key]: isNumeric
                      ? parsed !== null && Number.isFinite(parsed)
                        ? parsed
                        : 0
                      : value,
                  }),
            }
          : model
      ),
    }))
  }

  const addField = () => {
    const field = newField.trim()
    if (!field) {
      toast.error(t('Field name is required'))
      return
    }
    if (
      config.fields.some((item) => item.toLowerCase() === field.toLowerCase())
    ) {
      toast.error(t('Field already exists'))
      return
    }
    setConfig((current) => ({
      ...current,
      fields: [...current.fields, field],
      models: current.models.map((model) => ({
        ...model,
        customFields: { ...(model.customFields ?? {}), [field]: '' },
      })),
    }))
    setNewField('')
  }

  const removeField = (field: string) => {
    setConfig((current) => ({
      ...current,
      fields: current.fields.filter((item) => item !== field),
      models: current.models.map((model) => {
        const customFields = { ...(model.customFields ?? {}) }
        delete customFields[field]
        return { ...model, customFields }
      }),
    }))
  }

  const addModel = () => {
    const name = newModel.name.trim()
    if (!name) {
      toast.error(t('Model name is required'))
      return
    }
    if (config.models.some((model) => model.name === name)) {
      toast.error(t('Model already exists'))
      return
    }
    const toNumber = (value: string) => {
      const parsed = Number(value)
      return Number.isFinite(parsed) ? parsed : 0
    }
    setConfig((current) => ({
      ...current,
      models: [
        ...current.models,
        {
          name,
          vendor: newModel.vendor.trim(),
          billingUnit: newModel.billingUnit.trim() || 'USD/1M tokens',
          officialInput: toNumber(newModel.officialInput),
          officialOutput: toNumber(newModel.officialOutput),
          officialNonToken: toNumber(newModel.officialNonToken),
          costDiscount:
            newModel.costDiscount.trim() === ''
              ? null
              : toNumber(newModel.costDiscount),
          note: newModel.note.trim(),
          customFields: Object.fromEntries(
            config.fields.map((field) => [field, ''])
          ),
        },
      ],
    }))
    setNewModel({
      name: '',
      vendor: '',
      billingUnit: 'USD/1M tokens',
      officialInput: '0',
      officialOutput: '0',
      officialNonToken: '0',
      costDiscount: '',
      note: '',
    })
  }

  const removeModel = (name: string) => {
    if (!window.confirm(t('Remove model confirmation'))) return
    setConfig((current) => ({
      ...current,
      models: current.models.filter((model) => model.name !== name),
    }))
  }

  const save = () => {
    updateOption.mutate({
      key: COST_CALCULATION_OPTION_KEY,
      value: JSON.stringify(config),
    })
  }

  return (
    <div className='space-y-6'>
      <SettingsPageActionsPortal>
        <Button
          type='button'
          size='sm'
          onClick={save}
          disabled={updateOption.isPending}
        >
          <Save className='mr-2 h-4 w-4' />
          {updateOption.isPending ? t('Saving...') : t('Save cost calculation')}
        </Button>
      </SettingsPageActionsPortal>

      <SettingsSection title={t('Cost calculation assumptions')}>
        <div className='grid gap-4 rounded-lg border p-4 sm:grid-cols-2 lg:grid-cols-4'>
          {(
            [
              ['listPrice', 'List price'],
              ['bonus', 'Bonus quota'],
              ['coupon', 'Coupon cash reduction'],
              ['seedanceSellFactor', 'Seedance selling factor'],
              ['otherSellFactor', 'Other model selling factor'],
              ['inputTokens', 'Input tokens per call'],
              ['outputTokens', 'Output tokens per call'],
              ['nonTokenUsage', 'Non-token usage per call'],
            ] as const
          ).map(([key, label]) => (
            <label key={key} className='space-y-1 text-sm'>
              <span className='text-muted-foreground'>{t(label)}</span>
              <Input
                type='number'
                step='any'
                value={config.assumptions[key]}
                onChange={(event) => updateAssumption(key, event.target.value)}
              />
            </label>
          ))}
          <div className='bg-muted/40 rounded-md p-3 text-sm'>
            <div>
              {t('Actual cash payment')}: {currency(cashInputs.actualCash)}
            </div>
            <div>
              {t('Total available quota')}: {currency(cashInputs.totalQuota)}
            </div>
            <div>
              {t('Cash income factor')}: {percent(cashInputs.cashFactor)}
            </div>
          </div>
        </div>
      </SettingsSection>

      <SettingsSection title={t('Cost discounts')}>
        <div className='grid gap-4 rounded-lg border p-4 sm:grid-cols-2 lg:grid-cols-3'>
          {Object.entries(config.discounts).map(([key, value]) => (
            <label key={key} className='space-y-1 text-sm'>
              <span className='text-muted-foreground'>{t(key)}</span>
              <Input
                type='number'
                step='0.01'
                placeholder={t('Leave empty when unknown')}
                value={value ?? ''}
                onChange={(event) => {
                  const parsed =
                    event.target.value.trim() === ''
                      ? null
                      : Number(event.target.value)
                  setConfig((current) => ({
                    ...current,
                    discounts: { ...current.discounts, [key]: parsed },
                  }))
                }}
              />
            </label>
          ))}
        </div>
      </SettingsSection>

      <SettingsSection title={t('Model cost table')}>
        <div className='flex flex-wrap items-center justify-between gap-3'>
          <Input
            value={filter}
            onChange={(event) => setFilter(event.target.value)}
            placeholder={t('Search model or vendor')}
            className='max-w-sm'
          />
          <Button type='button' size='sm' variant='outline' onClick={addModel}>
            <Plus className='mr-2 h-4 w-4' />
            {t('Add model')}
          </Button>
        </div>
        <div className='grid gap-3 rounded-lg border border-dashed p-4 sm:grid-cols-2 lg:grid-cols-4'>
          {(
            [
              ['name', 'Model'],
              ['vendor', 'Vendor'],
              ['billingUnit', 'Billing unit'],
              ['officialInput', 'Official input'],
              ['officialOutput', 'Official output'],
              ['officialNonToken', 'Non-token price'],
              ['costDiscount', 'Model discount override'],
              ['note', 'Note'],
            ] as const
          ).map(([key, label]) => (
            <label key={key} className='space-y-1 text-sm'>
              <span className='text-muted-foreground'>{t(label)}</span>
              <Input
                type={
                  key === 'name' ||
                  key === 'vendor' ||
                  key === 'billingUnit' ||
                  key === 'note'
                    ? 'text'
                    : 'number'
                }
                step='any'
                value={newModel[key]}
                onChange={(event) =>
                  setNewModel((current) => ({
                    ...current,
                    [key]: event.target.value,
                  }))
                }
              />
            </label>
          ))}
          <div className='flex items-end gap-2'>
            <label className='min-w-0 flex-1 space-y-1 text-sm'>
              <span className='text-muted-foreground'>{t('New field')}</span>
              <Input
                value={newField}
                onChange={(event) => setNewField(event.target.value)}
                placeholder={t('e.g. Region')}
              />
            </label>
            <Button
              type='button'
              size='sm'
              variant='outline'
              onClick={addField}
            >
              <Plus className='mr-2 h-4 w-4' />
              {t('Add field')}
            </Button>
          </div>
        </div>
        <div className='overflow-x-auto rounded-lg border'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Model')}</TableHead>
                <TableHead>{t('Vendor')}</TableHead>
                <TableHead>{t('Billing unit')}</TableHead>
                <TableHead>{t('Official input')}</TableHead>
                <TableHead>{t('Official output')}</TableHead>
                <TableHead>{t('Non-token price')}</TableHead>
                <TableHead>{t('Model discount override')}</TableHead>
                <TableHead>{t('Note')}</TableHead>
                {config.fields.map((field) => (
                  <TableHead key={field} className='whitespace-nowrap'>
                    <span className='mr-2'>{field}</span>
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon'
                      title={t('Remove field')}
                      aria-label={t('Remove field')}
                      onClick={() => removeField(field)}
                    >
                      <Trash2 className='h-3 w-3' />
                    </Button>
                  </TableHead>
                ))}
                <TableHead>{t('Official fee')}</TableHead>
                <TableHead>{t('Flatkey price')}</TableHead>
                <TableHead>{t('Cash income')}</TableHead>
                <TableHead>{t('Upstream cost')}</TableHead>
                <TableHead>{t('Profit')}</TableHead>
                <TableHead>{t('Margin')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {models.map((model) => {
                const result = calculateModel(
                  model,
                  config.assumptions,
                  config.discounts
                )
                const status =
                  result.status === 'profit'
                    ? t('Profit')
                    : result.status === 'loss'
                      ? t('Loss')
                      : t('Missing discount')
                return (
                  <TableRow key={model.name}>
                    {(['name', 'vendor', 'billingUnit'] as const).map((key) => (
                      <TableCell key={key}>
                        <Input
                          className='h-8 min-w-32'
                          value={model[key]}
                          onChange={(event) =>
                            updateModel(model.name, key, event.target.value)
                          }
                        />
                      </TableCell>
                    ))}
                    {(
                      [
                        'officialInput',
                        'officialOutput',
                        'officialNonToken',
                      ] as const
                    ).map((key) => (
                      <TableCell key={key}>
                        <Input
                          className='h-8 min-w-24'
                          type='number'
                          step='any'
                          value={model[key] ?? ''}
                          onChange={(event) =>
                            updateModel(model.name, key, event.target.value)
                          }
                        />
                      </TableCell>
                    ))}
                    <TableCell>
                      <Input
                        className='h-8 min-w-24'
                        type='number'
                        step='any'
                        placeholder={percent(
                          getDiscountForModel(
                            { ...model, costDiscount: null },
                            config.discounts
                          )
                        )}
                        value={model.costDiscount ?? ''}
                        onChange={(event) =>
                          updateModel(
                            model.name,
                            'costDiscount',
                            event.target.value
                          )
                        }
                      />
                    </TableCell>
                    <TableCell>
                      <Input
                        className='h-8 min-w-40'
                        value={model.note}
                        onChange={(event) =>
                          updateModel(model.name, 'note', event.target.value)
                        }
                      />
                    </TableCell>
                    {config.fields.map((field) => (
                      <TableCell key={field}>
                        <Input
                          className='h-8 min-w-32'
                          value={model.customFields?.[field] ?? ''}
                          onChange={(event) =>
                            updateModel(
                              model.name,
                              `custom:${field}`,
                              event.target.value
                            )
                          }
                        />
                      </TableCell>
                    ))}
                    <TableCell>{currency(result.officialFee)}</TableCell>
                    <TableCell>{currency(result.flatkeyPrice)}</TableCell>
                    <TableCell>{currency(result.cashIncome)}</TableCell>
                    <TableCell>{currency(result.upstreamCost)}</TableCell>
                    <TableCell
                      className={
                        result.profit !== null && result.profit < 0
                          ? 'text-destructive'
                          : 'text-emerald-600'
                      }
                    >
                      {currency(result.profit)}
                    </TableCell>
                    <TableCell>{percent(result.margin)}</TableCell>
                    <TableCell className='whitespace-nowrap'>
                      {status}
                    </TableCell>
                    <TableCell className='text-right'>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon'
                        title={t('Remove model')}
                        aria-label={t('Remove model')}
                        onClick={() => removeModel(model.name)}
                      >
                        <Trash2 className='h-4 w-4' />
                      </Button>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </div>
        <div className='text-muted-foreground text-sm'>
          {t('Models')}: {summary.priced} · {t('Profit')}: {summary.profit} ·{' '}
          {t('Loss')}: {summary.loss} · {t('Missing discount')}:{' '}
          {summary.missingDiscount} · {t('Total profit')}:{' '}
          {currency(summary.totalProfit)} · {t('Margin')}:{' '}
          {percent(summary.margin)}
        </div>
      </SettingsSection>
    </div>
  )
}
