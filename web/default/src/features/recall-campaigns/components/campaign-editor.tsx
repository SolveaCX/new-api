import { useEffect, useState } from 'react'
import { useFieldArray, useForm, type FieldPath } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { useRecallCampaignMutations } from '../api'
import { audienceTemplateDescriptionKeys } from '../copy'
import {
  formatRecallMinorAmount,
  normalizeRecallCouponSource,
  normalizeRecallDiscountType,
  parseRecallMajorAmount,
  prepareRecallCampaignSubmitDraft,
  recallFixedCurrencies,
  removeRecallEmailStage,
  setRecallCampaignGroupMode,
} from '../helpers'
import {
  recallCampaignActivatedUpdateSchema,
  recallCampaignDraftSchema,
} from '../schemas'
import type {
  RecallCampaignDraft,
  RecallCampaignStatus,
  RecallDiscountConfig,
  RecallFixedCurrency,
} from '../types'
import { CampaignProductSelector } from './campaign-product-selector'

type RecallFixedAmountInputs = Record<RecallFixedCurrency, string>

const recallFixedAmountPaths: Record<
  RecallFixedCurrency,
  FieldPath<RecallCampaignDraft>
> = {
  USD: 'discount_config.amount_off',
  INR: 'discount_config.currency_options.inr',
  BRL: 'discount_config.currency_options.brl',
  JPY: 'discount_config.currency_options.jpy',
}

function getRecallFixedMinorAmount(
  discount: RecallDiscountConfig,
  currency: RecallFixedCurrency
): number {
  if (currency === 'USD') return discount.amount_off
  return discount.currency_options[currency.toLowerCase()] ?? 0
}

function createRecallFixedAmountInputs(
  discount: RecallDiscountConfig
): RecallFixedAmountInputs {
  return Object.fromEntries(
    recallFixedCurrencies.map((currency) => [
      currency,
      formatRecallMinorAmount(
        currency,
        getRecallFixedMinorAmount(discount, currency)
      ),
    ])
  ) as RecallFixedAmountInputs
}

function createRecallCampaignFormDraft(
  draft: RecallCampaignDraft
): RecallCampaignDraft {
  const normalizedDraft =
    draft.coupon_source === 'automatic' &&
    draft.discount_config.type === 'fixed'
      ? normalizeRecallDiscountType(draft, 'fixed')
      : draft
  return prepareRecallCampaignSubmitDraft(normalizedDraft)
}

const audienceFields: Record<
  RecallCampaignDraft['audience_template'],
  { name: FieldPath<RecallCampaignDraft>; label: string; step?: string }[]
> = {
  first_purchase: [
    {
      name: 'audience_config.registration_age_days',
      label: 'Registration age days',
    },
    {
      name: 'audience_config.min_request_count',
      label: 'Minimum request count',
    },
    { name: 'audience_config.max_quota', label: 'Maximum quota' },
    {
      name: 'audience_config.last_api_call_age_days',
      label: 'Last API call age days',
    },
  ],
  lapsed_payer: [
    {
      name: 'audience_config.min_paid_amount',
      label: 'Minimum paid amount',
      step: '0.01',
    },
    {
      name: 'audience_config.last_api_call_age_days',
      label: 'Last API call age days',
    },
    {
      name: 'audience_config.last_payment_age_days',
      label: 'Last payment age days',
    },
    { name: 'audience_config.max_quota', label: 'Maximum quota' },
  ],
  expired_subscription: [
    {
      name: 'audience_config.subscription_expired_days',
      label: 'Subscription expired days',
    },
    {
      name: 'audience_config.min_subscription_amount',
      label: 'Minimum subscription amount',
      step: '0.01',
    },
    {
      name: 'audience_config.min_subscription_count',
      label: 'Minimum subscription count',
    },
    {
      name: 'audience_config.last_api_call_age_days',
      label: 'Last API call age days',
    },
  ],
}

function createRecallCampaignDefaults(): RecallCampaignDraft {
  return {
    name: '',
    audience_template: 'first_purchase',
    audience_config: {
      registration_age_days: 30,
      min_request_count: 1,
      max_quota: 0,
      min_paid_amount: 0,
      last_api_call_age_days: 30,
      last_payment_age_days: 30,
      subscription_expired_days: 30,
      min_subscription_amount: 0,
      min_subscription_count: 1,
      payment_providers: [],
      groups: [],
      group_mode: '',
      require_verified_email: true,
    },
    execution_mode: 'manual',
    schedule: {
      scheduled_at: 0,
      timezone: 'UTC',
      frequency: 'daily',
      weekday: 1,
      hour: 9,
      minute: 0,
    },
    coupon_source: 'automatic',
    existing_coupon_id: '',
    discount_config: {
      type: 'percent',
      percent_off: 20,
      amount_off: 0,
      currency: '',
      currency_options: {},
      minimum_amount: 0,
      minimum_amount_currency: '',
      coupon_redeem_by: 0,
    },
    product_scope: { topup_price_ids: [], subscription_price_ids: [] },
    promotion_valid_seconds: 604800,
    enrollment_limit: 1000,
    worker_concurrency: 5,
    email_sequence: [
      {
        stage_no: 1,
        delay_seconds: 0,
        template_version: 1,
        templates: { en: { subject: '', body_text: '' } },
      },
    ],
  }
}

interface CampaignEditorProps {
  campaignId?: number
  initialDraft?: RecallCampaignDraft
  status?: RecallCampaignStatus
  onSaved?: (campaignId: number) => void
}

export function CampaignEditor(props: CampaignEditorProps) {
  const { t } = useTranslation()
  const mutations = useRecallCampaignMutations(props.campaignId)
  const updateSchema =
    props.status && props.status !== 'draft'
      ? recallCampaignActivatedUpdateSchema
      : recallCampaignDraftSchema
  const defaultValues = createRecallCampaignFormDraft(
    props.initialDraft ?? createRecallCampaignDefaults()
  )
  const form = useForm<RecallCampaignDraft>({
    resolver: zodResolver(updateSchema),
    defaultValues,
  })
  const [fixedAmountInputs, setFixedAmountInputs] =
    useState<RecallFixedAmountInputs>(() =>
      createRecallFixedAmountInputs(defaultValues.discount_config)
    )
  const stages = useFieldArray({
    control: form.control,
    name: 'email_sequence',
  })
  const audienceTemplate = form.watch('audience_template')
  const couponSource = form.watch('coupon_source')
  const discountType = form.watch('discount_config.type')
  const executionMode = form.watch('execution_mode')
  const groups = form.watch('audience_config.groups')
  const groupMode = form.watch('audience_config.group_mode')
  const providers = form.watch('audience_config.payment_providers')
  const topUpPrices = form.watch('product_scope.topup_price_ids')
  const subscriptionPrices = form.watch('product_scope.subscription_price_ids')
  const immutable = Boolean(props.status && props.status !== 'draft')
  const automaticFixed =
    couponSource === 'automatic' && discountType === 'fixed'
  const terminal = props.status === 'cancelled' || props.status === 'completed'
  const isSaving = mutations.create.isPending || mutations.update.isPending

  useEffect(() => {
    if (props.initialDraft) {
      const draft = createRecallCampaignFormDraft(props.initialDraft)
      form.reset(draft)
      setFixedAmountInputs(createRecallFixedAmountInputs(draft.discount_config))
    }
  }, [form, props.initialDraft])

  const setCsv = (
    path: 'audience_config.groups' | 'audience_config.payment_providers',
    value: string
  ) => {
    form.setValue(
      path,
      value
        .split(',')
        .map((item) => item.trim())
        .filter(Boolean),
      { shouldDirty: true, shouldValidate: true }
    )
  }

  const setGroupMode = (
    mode: RecallCampaignDraft['audience_config']['group_mode']
  ) => {
    void setRecallCampaignGroupMode(form, mode).catch(() => {
      toast.error(t('Something went wrong!'))
    })
  }

  const onSubmit = async (draft: RecallCampaignDraft) => {
    const normalizedDraft = prepareRecallCampaignSubmitDraft(draft)
    const response = props.campaignId
      ? await mutations.update.mutateAsync(normalizedDraft)
      : await mutations.create.mutateAsync(normalizedDraft)
    if (!response.success || !response.data) return
    toast.success(
      props.campaignId ? t('Campaign updated') : t('Campaign created')
    )
    props.onSaved?.(response.data.id)
  }

  const setCouponSource = (value: RecallCampaignDraft['coupon_source']) => {
    const normalized = normalizeRecallCouponSource(form.getValues(), value)
    form.setValue('coupon_source', normalized.coupon_source, {
      shouldDirty: true,
      shouldValidate: true,
    })
    form.setValue('existing_coupon_id', normalized.existing_coupon_id, {
      shouldDirty: true,
      shouldValidate: true,
    })
    form.setValue('discount_config', normalized.discount_config, {
      shouldDirty: true,
      shouldValidate: true,
    })
    setFixedAmountInputs(
      createRecallFixedAmountInputs(normalized.discount_config)
    )
  }

  const setDiscountType = (
    value: RecallCampaignDraft['discount_config']['type']
  ) => {
    const normalized = normalizeRecallDiscountType(form.getValues(), value)
    form.setValue('discount_config', normalized.discount_config, {
      shouldDirty: true,
      shouldValidate: true,
    })
    setFixedAmountInputs(
      createRecallFixedAmountInputs(normalized.discount_config)
    )
  }

  const setFixedAmount = (currency: RecallFixedCurrency, value: string) => {
    setFixedAmountInputs((current) => ({ ...current, [currency]: value }))
    form.setValue(
      recallFixedAmountPaths[currency],
      parseRecallMajorAmount(currency, value) ?? 0,
      { shouldDirty: true, shouldValidate: true }
    )
  }

  return (
    <form className='space-y-4' onSubmit={form.handleSubmit(onSubmit)}>
      <Card>
        <CardHeader>
          <CardTitle>{t('1. Campaign and audience')}</CardTitle>
        </CardHeader>
        <CardContent className='grid gap-4 md:grid-cols-2'>
          <div className='space-y-2'>
            <Label htmlFor='recall-name'>{t('Campaign name')}</Label>
            <Input
              id='recall-name'
              disabled={immutable}
              {...form.register('name')}
            />
          </div>
          <div className='space-y-2'>
            <Label>{t('Audience template')}</Label>
            <Select
              disabled={immutable}
              value={audienceTemplate}
              onValueChange={(value) =>
                value &&
                form.setValue(
                  'audience_template',
                  value as RecallCampaignDraft['audience_template']
                )
              }
              items={[
                { value: 'first_purchase', label: t('First purchase') },
                { value: 'lapsed_payer', label: t('Lapsed payer') },
                {
                  value: 'expired_subscription',
                  label: t('Expired subscription'),
                },
              ]}
            >
              <SelectTrigger
                aria-describedby='recall-audience-help'
                className='w-full'
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectItem value='first_purchase'>
                    {t('First purchase')}
                  </SelectItem>
                  <SelectItem value='lapsed_payer'>
                    {t('Lapsed payer')}
                  </SelectItem>
                  <SelectItem value='expired_subscription'>
                    {t('Expired subscription')}
                  </SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
          <div
            id='recall-audience-help'
            className='bg-muted/50 text-muted-foreground space-y-1 rounded-md p-3 text-sm md:col-span-2'
          >
            <p>
              {t(
                'Audience templates define the base audience. The rules shown below narrow it further, and built-in eligibility filters also apply. Preview the audience before activation.'
              )}
            </p>
            <p aria-live='polite' className='text-foreground'>
              {t(audienceTemplateDescriptionKeys[audienceTemplate])}
            </p>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('2. Audience rules')}</CardTitle>
        </CardHeader>
        <CardContent className='grid gap-4 md:grid-cols-3'>
          {audienceFields[audienceTemplate].map((field) => (
            <div className='space-y-2' key={field.name}>
              <Label>{t(field.label)}</Label>
              <Input
                type='number'
                min={0}
                step={field.step ?? '1'}
                disabled={immutable}
                {...form.register(field.name, { valueAsNumber: true })}
              />
            </div>
          ))}
          <div className='space-y-2'>
            <Label htmlFor='recall-groups'>
              {t('Groups (comma separated)')}
            </Label>
            <Input
              id='recall-groups'
              disabled={immutable || groupMode === ''}
              value={groups.join(', ')}
              onChange={(event) =>
                setCsv('audience_config.groups', event.target.value)
              }
            />
          </div>
          <div className='space-y-2'>
            <Label>{t('Group mode')}</Label>
            <Select
              disabled={immutable}
              value={groupMode}
              onValueChange={(value) =>
                setGroupMode(
                  (value ??
                    '') as RecallCampaignDraft['audience_config']['group_mode']
                )
              }
              items={[
                { value: '', label: t('No group filter') },
                { value: 'allow', label: t('Allow groups') },
                { value: 'block', label: t('Block groups') },
              ]}
            >
              <SelectTrigger className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectItem value=''>{t('No group filter')}</SelectItem>
                  <SelectItem value='allow'>{t('Allow groups')}</SelectItem>
                  <SelectItem value='block'>{t('Block groups')}</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
          {audienceTemplate !== 'first_purchase' && (
            <div className='space-y-2'>
              <Label>{t('Payment providers (comma separated)')}</Label>
              <Input
                disabled={immutable}
                value={providers.join(', ')}
                onChange={(event) =>
                  setCsv(
                    'audience_config.payment_providers',
                    event.target.value
                  )
                }
              />
            </div>
          )}
          <label className='flex items-center gap-2 md:col-span-3'>
            <input
              type='checkbox'
              disabled={immutable}
              {...form.register('audience_config.require_verified_email')}
            />
            {t('Require verified email')}
          </label>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('3. Stripe Coupon')}</CardTitle>
        </CardHeader>
        <CardContent className='grid gap-4 md:grid-cols-3'>
          <div className='space-y-2'>
            <Label>{t('Coupon source')}</Label>
            <Select
              disabled={immutable}
              value={couponSource}
              onValueChange={(value) =>
                value &&
                setCouponSource(value as RecallCampaignDraft['coupon_source'])
              }
              items={[
                { value: 'automatic', label: t('Automatic Coupon') },
                { value: 'existing', label: t('Existing Coupon') },
              ]}
            >
              <SelectTrigger className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectItem value='automatic'>
                    {t('Automatic Coupon')}
                  </SelectItem>
                  <SelectItem value='existing'>
                    {t('Existing Coupon')}
                  </SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
          {couponSource === 'existing' ? (
            <div className='space-y-2 md:col-span-2'>
              <Label>{t('Existing Coupon ID')}</Label>
              <Input
                disabled={immutable}
                {...form.register('existing_coupon_id')}
              />
            </div>
          ) : null}
          <div className='space-y-2'>
            <Label>{t('Discount type')}</Label>
            <Select
              disabled={immutable}
              value={discountType}
              onValueChange={(value) =>
                value &&
                setDiscountType(
                  value as RecallCampaignDraft['discount_config']['type']
                )
              }
              items={[
                { value: 'percent', label: t('Percent') },
                { value: 'fixed', label: t('Fixed amount') },
              ]}
            >
              <SelectTrigger className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectItem value='percent'>{t('Percent')}</SelectItem>
                  <SelectItem value='fixed'>{t('Fixed amount')}</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
          {discountType === 'percent' ? (
            <div className='space-y-2'>
              <Label>{t('Percent off')}</Label>
              <Input
                type='number'
                min={0.01}
                max={100}
                step='0.01'
                disabled={immutable}
                {...form.register('discount_config.percent_off', {
                  valueAsNumber: true,
                })}
              />
            </div>
          ) : automaticFixed ? (
            <>
              <p className='text-muted-foreground text-sm md:col-span-3'>
                {t(
                  'Stripe does not convert fixed Coupon amounts automatically. Configure each checkout currency explicitly.'
                )}
              </p>
              {recallFixedCurrencies.map((currency) => (
                <div className='space-y-2' key={currency}>
                  <Label>{t('{{currency}} amount off', { currency })}</Label>
                  <Input
                    type='number'
                    min={currency === 'JPY' ? 1 : 0.01}
                    step={currency === 'JPY' ? '1' : '0.01'}
                    disabled={immutable}
                    value={fixedAmountInputs[currency]}
                    onChange={(event) =>
                      setFixedAmount(currency, event.target.value)
                    }
                  />
                </div>
              ))}
            </>
          ) : (
            <>
              <div className='space-y-2'>
                <Label>{t('Amount off')}</Label>
                <Input
                  type='number'
                  min={1}
                  disabled={immutable}
                  {...form.register('discount_config.amount_off', {
                    valueAsNumber: true,
                  })}
                />
              </div>
              <div className='space-y-2'>
                <Label>{t('Currency')}</Label>
                <Input
                  maxLength={3}
                  placeholder='USD'
                  disabled={immutable}
                  {...form.register('discount_config.currency')}
                />
              </div>
            </>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('4. Products, minimum, and validity')}</CardTitle>
        </CardHeader>
        <CardContent className='grid gap-4 md:grid-cols-2'>
          <CampaignProductSelector
            topUpPriceIDs={topUpPrices}
            subscriptionPriceIDs={subscriptionPrices}
            onTopUpChange={(value) =>
              form.setValue('product_scope.topup_price_ids', value, {
                shouldDirty: true,
                shouldValidate: true,
              })
            }
            onSubscriptionChange={(value) =>
              form.setValue('product_scope.subscription_price_ids', value, {
                shouldDirty: true,
                shouldValidate: true,
              })
            }
            immutable={immutable}
          />
          {!automaticFixed ? (
            <>
              <div className='space-y-2'>
                <Label>{t('Minimum amount')}</Label>
                <Input
                  type='number'
                  min={0}
                  disabled={immutable}
                  {...form.register('discount_config.minimum_amount', {
                    valueAsNumber: true,
                  })}
                />
              </div>
              <div className='space-y-2'>
                <Label>{t('Minimum amount currency')}</Label>
                <Input
                  maxLength={3}
                  placeholder='USD'
                  disabled={immutable}
                  {...form.register('discount_config.minimum_amount_currency')}
                />
              </div>
            </>
          ) : null}
          <div className='space-y-2'>
            <Label>{t('Coupon redeem-by timestamp')}</Label>
            <Input
              type='number'
              min={0}
              disabled={immutable}
              {...form.register('discount_config.coupon_redeem_by', {
                valueAsNumber: true,
              })}
            />
          </div>
          <div className='space-y-2'>
            <Label>{t('Promotion validity seconds')}</Label>
            <Input
              type='number'
              min={1}
              disabled={immutable}
              {...form.register('promotion_valid_seconds', {
                valueAsNumber: true,
              })}
            />
          </div>
          <div className='space-y-2'>
            <Label>{t('Enrollment limit')}</Label>
            <Input
              type='number'
              min={1}
              max={100000}
              disabled={immutable}
              {...form.register('enrollment_limit', { valueAsNumber: true })}
            />
          </div>
          <div className='space-y-2'>
            <Label>{t('Worker concurrency')}</Label>
            <Input
              type='number'
              min={1}
              max={20}
              disabled={immutable}
              {...form.register('worker_concurrency', { valueAsNumber: true })}
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('5. Execution schedule')}</CardTitle>
        </CardHeader>
        <CardContent className='grid gap-4 md:grid-cols-3'>
          <div className='space-y-2'>
            <Label>{t('Execution mode')}</Label>
            <Select
              disabled={immutable}
              value={executionMode}
              onValueChange={(value) =>
                value &&
                form.setValue(
                  'execution_mode',
                  value as RecallCampaignDraft['execution_mode']
                )
              }
              items={[
                { value: 'manual', label: t('Manual') },
                { value: 'scheduled_once', label: t('Scheduled once') },
                { value: 'recurring', label: t('Recurring') },
              ]}
            >
              <SelectTrigger className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectItem value='manual'>{t('Manual')}</SelectItem>
                  <SelectItem value='scheduled_once'>
                    {t('Scheduled once')}
                  </SelectItem>
                  <SelectItem value='recurring'>{t('Recurring')}</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
          {executionMode === 'scheduled_once' ? (
            <div className='space-y-2'>
              <Label>{t('Scheduled Unix timestamp')}</Label>
              <Input
                type='number'
                disabled={immutable}
                {...form.register('schedule.scheduled_at', {
                  valueAsNumber: true,
                })}
              />
            </div>
          ) : null}
          {executionMode === 'recurring' ? (
            <>
              <div className='space-y-2'>
                <Label>{t('IANA timezone')}</Label>
                <Input
                  placeholder='America/New_York'
                  disabled={immutable}
                  {...form.register('schedule.timezone')}
                />
              </div>
              <div className='space-y-2'>
                <Label>{t('Frequency')}</Label>
                <Select
                  disabled={immutable}
                  value={form.watch('schedule.frequency')}
                  onValueChange={(value) =>
                    value &&
                    form.setValue(
                      'schedule.frequency',
                      value as 'daily' | 'weekly'
                    )
                  }
                  items={[
                    { value: 'daily', label: t('Daily') },
                    { value: 'weekly', label: t('Weekly') },
                  ]}
                >
                  <SelectTrigger className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value='daily'>{t('Daily')}</SelectItem>
                      <SelectItem value='weekly'>{t('Weekly')}</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </div>
              {form.watch('schedule.frequency') === 'weekly' ? (
                <div className='space-y-2'>
                  <Label>{t('Weekday (0-6)')}</Label>
                  <Input
                    type='number'
                    min={0}
                    max={6}
                    disabled={immutable}
                    {...form.register('schedule.weekday', {
                      valueAsNumber: true,
                    })}
                  />
                </div>
              ) : null}
              <div className='space-y-2'>
                <Label>{t('Hour (0-23)')}</Label>
                <Input
                  type='number'
                  min={0}
                  max={23}
                  disabled={immutable}
                  {...form.register('schedule.hour', { valueAsNumber: true })}
                />
              </div>
              <div className='space-y-2'>
                <Label>{t('Minute (0-59)')}</Label>
                <Input
                  type='number'
                  min={0}
                  max={59}
                  disabled={immutable}
                  {...form.register('schedule.minute', { valueAsNumber: true })}
                />
              </div>
            </>
          ) : null}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('6. Email sequence')}</CardTitle>
        </CardHeader>
        <CardContent className='space-y-4'>
          <p className='text-muted-foreground text-sm'>
            {t(
              "Email content is translated automatically when saved, sent in each user's language, and falls back to English when unavailable."
            )}
          </p>
          {stages.fields.map((stage, index) => {
            const subjectPath =
              `email_sequence.${index}.templates.en.subject` as FieldPath<RecallCampaignDraft>
            const bodyPath =
              `email_sequence.${index}.templates.en.body_text` as FieldPath<RecallCampaignDraft>
            const subjectId = `recall-email-${index}-subject`
            const subjectErrorId = `${subjectId}-error`
            const subjectError = form.getFieldState(
              subjectPath,
              form.formState
            ).error
            const bodyId = `recall-email-${index}-body-text`
            const bodyErrorId = `${bodyId}-error`
            const bodyError = form.getFieldState(bodyPath, form.formState).error
            return (
              <div className='space-y-3 rounded-lg border p-3' key={stage.id}>
                <div className='flex flex-wrap items-center justify-between gap-2'>
                  <strong>
                    {t('Email stage {{stage}}', { stage: index + 1 })}
                  </strong>
                  <span className='text-muted-foreground text-xs'>
                    {t('TemplateVersion')}:{' '}
                    {form.watch(`email_sequence.${index}.template_version`)}
                  </span>
                </div>
                <div className='grid gap-3 md:grid-cols-2'>
                  <div className='space-y-2'>
                    <Label>{t('Delay seconds')}</Label>
                    <Input
                      type='number'
                      min={0}
                      disabled={immutable}
                      {...form.register(
                        `email_sequence.${index}.delay_seconds`,
                        { valueAsNumber: true }
                      )}
                    />
                  </div>
                  <div className='space-y-2'>
                    <Label htmlFor={subjectId}>{t('Subject')}</Label>
                    <Input
                      id={subjectId}
                      disabled={terminal}
                      aria-invalid={Boolean(subjectError)}
                      aria-describedby={
                        subjectError ? subjectErrorId : undefined
                      }
                      {...form.register(subjectPath)}
                    />
                    {subjectError ? (
                      <p
                        id={subjectErrorId}
                        role='alert'
                        className='text-destructive text-sm'
                      >
                        {t(String(subjectError.message))}
                      </p>
                    ) : null}
                  </div>
                </div>
                <div className='space-y-2'>
                  <Label htmlFor={bodyId}>{t('Body text')}</Label>
                  <Textarea
                    id={bodyId}
                    rows={5}
                    disabled={terminal}
                    aria-invalid={Boolean(bodyError)}
                    aria-describedby={bodyError ? bodyErrorId : undefined}
                    {...form.register(bodyPath)}
                  />
                  {bodyError ? (
                    <p
                      id={bodyErrorId}
                      role='alert'
                      className='text-destructive text-sm'
                    >
                      {t(String(bodyError.message))}
                    </p>
                  ) : null}
                </div>
                {stages.fields.length > 1 && !immutable ? (
                  <Button
                    type='button'
                    variant='outline'
                    onClick={() =>
                      stages.replace(
                        removeRecallEmailStage(
                          form.getValues('email_sequence'),
                          index
                        )
                      )
                    }
                  >
                    {t('Remove stage')}
                  </Button>
                ) : null}
              </div>
            )
          })}
          {stages.fields.length < 3 && !immutable ? (
            <Button
              type='button'
              variant='outline'
              onClick={() =>
                stages.append({
                  stage_no: stages.fields.length + 1,
                  delay_seconds: stages.fields.length * 86400,
                  template_version: 1,
                  templates: { en: { subject: '', body_text: '' } },
                })
              }
            >
              {t('Add email stage')}
            </Button>
          ) : null}
        </CardContent>
      </Card>

      {Object.keys(form.formState.errors).length > 0 ? (
        <p className='text-destructive text-sm'>
          {t('Please correct the highlighted fields.')}
        </p>
      ) : null}
      <Button type='submit' disabled={isSaving || terminal}>
        {isSaving ? t('Saving') : t('Save campaign')}
      </Button>
    </form>
  )
}
