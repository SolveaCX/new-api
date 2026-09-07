import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Check, Loader2, Save } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { officialWebsiteUrl } from '@/lib/origins'
import { getModels } from '@/features/models/api'
import type { Model } from '@/features/models/types'
import { SettingsSection } from '../components/settings-section'
import { SettingsSwitchField } from '../components/settings-form-layout'
import { useUpdateOption } from '../hooks/use-update-option'
import {
  modelLogoPath,
  modelOptions,
  normalizeWelcomePromo,
  serializeWelcomePromo,
  type WelcomePromoModel,
} from './welcome-promo-config'

type WelcomePromoSectionProps = {
  enabled: boolean
  data: string
}

async function getAllModels(): Promise<Model[]> {
  const first = await getModels({ p: 1, page_size: 100 })
  if (!first.success || !first.data) return []
  const pageCount = Math.ceil(first.data.total / first.data.page_size)
  if (pageCount <= 1) return first.data.items
  const remaining = await Promise.all(
    Array.from({ length: pageCount - 1 }, (_, index) =>
      getModels({ p: index + 2, page_size: first.data?.page_size ?? 100 }),
    ),
  )
  return [
    ...first.data.items,
    ...remaining.flatMap((response) =>
      response.success && response.data ? response.data.items : [],
    ),
  ]
}

export function WelcomePromoSection(props: WelcomePromoSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [models, setModels] = useState<WelcomePromoModel[]>(() =>
    normalizeWelcomePromo(props.data),
  )
  const [savedValue, setSavedValue] = useState(() =>
    serializeWelcomePromo(normalizeWelcomePromo(props.data)),
  )
  const [isEnabled, setIsEnabled] = useState(props.enabled)

  const modelQuery = useQuery({
    queryKey: ['models', 'welcome-promo-options'],
    queryFn: getAllModels,
    staleTime: 5 * 60 * 1000,
  })

  useEffect(() => {
    const next = normalizeWelcomePromo(props.data)
    // Settings are controlled by the parent query and must reset local edits when refreshed.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setModels(next)
    setSavedValue(serializeWelcomePromo(next))
  }, [props.data])

  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => setIsEnabled(props.enabled), [props.enabled])

  const availableModelNames = useMemo(
    () => modelOptions(modelQuery.data ?? []),
    [modelQuery.data],
  )

  const updateSlot = (
    index: number,
    field: keyof Omit<WelcomePromoModel, 'model_name'> | 'model_name',
    value: string,
  ) => {
    setModels((current) =>
      current.map((item, itemIndex) =>
        itemIndex === index ? { ...item, [field]: value } : item,
      ),
    )
  }

  const handleSave = async () => {
    if (models.length !== 3 || models.some((item) => !item.model_name.trim())) {
      toast.error(t('Please choose a model for each slot'))
      return
    }
    if (models.some((item) => !item.description.trim() || !item.offer.trim())) {
      toast.error(t('Description and offer are required'))
      return
    }
    if (new Set(models.map((item) => item.model_name)).size !== models.length) {
      toast.error(t('Each slot must use a different model'))
      return
    }
    const value = serializeWelcomePromo(models)
    const response = await updateOption.mutateAsync({
      key: 'console_setting.welcome_promo',
      value,
    })
    if (response.success) setSavedValue(value)
  }

  const handleToggleEnabled = async (checked: boolean) => {
    const response = await updateOption.mutateAsync({
      key: 'console_setting.welcome_promo_enabled',
      value: checked,
    })
    if (response.success) setIsEnabled(checked)
  }

  const isDirty = serializeWelcomePromo(models) !== savedValue

  return (
    <SettingsSection title={t('Welcome Promo')}>
      <div className='space-y-4'>
        <SettingsSwitchField
          checked={isEnabled}
          onCheckedChange={handleToggleEnabled}
          label={t('Enabled')}
          className='border-b-0 py-0'
        />
        <div className='text-muted-foreground text-sm'>
          {t(
            'Configure the three model cards shown in the homepage welcome popup.'
          )}
          <div className='mt-1'>
            {t('Logo is mapped automatically from the model name.')}
          </div>
        </div>

        {modelQuery.isLoading ? (
          <div className='text-muted-foreground flex items-center gap-2 text-sm'>
            <Loader2 className='h-4 w-4 animate-spin' />
            {t('Loading')}
          </div>
        ) : null}
        {modelQuery.isError ? (
          <p className='text-destructive text-sm'>{t('No models available')}</p>
        ) : null}

        <div className='space-y-3'>
          {models.map((item, index) => {
            const logo = modelLogoPath(item.model_name)
            const names = Array.from(
              new Set([...availableModelNames, item.model_name].filter(Boolean)),
            ).sort((a, b) => a.localeCompare(b))
            return (
              <div
                key={index}
                className='grid gap-3 rounded-lg border p-3 md:grid-cols-[minmax(11rem,1fr)_minmax(12rem,1.2fr)_minmax(10rem,0.8fr)]'
              >
                <div className='space-y-2'>
                  <div className='text-muted-foreground text-xs font-medium'>
                    #{index + 1}
                  </div>
                  <Select
                    value={item.model_name}
                    onValueChange={(value) =>
                      updateSlot(index, 'model_name', value ?? '')
                    }
                  >
                    <SelectTrigger className='w-full' aria-label={t('Model')}>
                      <SelectValue placeholder={t('Select or enter model name')} />
                    </SelectTrigger>
                    <SelectContent>
                      {names.map((name) => (
                        <SelectItem key={name} value={name}>
                          {name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  {logo ? (
                    <div className='text-muted-foreground flex items-center gap-2 text-xs'>
                      <img
                        src={officialWebsiteUrl(logo)}
                        alt=''
                        className='h-5 w-5 object-contain'
                      />
                      {t('Logo')}
                    </div>
                  ) : null}
                </div>
                <label className='space-y-2'>
                  <span className='text-muted-foreground text-xs font-medium'>
                    {t('Description')}
                  </span>
                  <Input
                    value={item.description}
                    maxLength={200}
                    onChange={(event) =>
                      updateSlot(index, 'description', event.target.value)
                    }
                    placeholder={t('Description')}
                  />
                </label>
                <label className='space-y-2'>
                  <span className='text-muted-foreground text-xs font-medium'>
                    {t('Offer')}
                  </span>
                  <Input
                    value={item.offer}
                    maxLength={100}
                    onChange={(event) =>
                      updateSlot(index, 'offer', event.target.value)
                    }
                    placeholder={t('Offer')}
                  />
                </label>
              </div>
            )
          })}
        </div>

        <Button onClick={handleSave} disabled={!isDirty || updateOption.isPending}>
          {updateOption.isPending ? (
            <Loader2 className='h-4 w-4 animate-spin' />
          ) : (
            <Save className='h-4 w-4' />
          )}
          {updateOption.isPending ? t('Saving') : t('Save')}
          {!updateOption.isPending && !isDirty ? (
            <Check className='h-4 w-4' />
          ) : null}
        </Button>
      </div>
    </SettingsSection>
  )
}
