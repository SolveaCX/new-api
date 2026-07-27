import { useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { PlusSignIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import {
  Progress,
  ProgressLabel,
  ProgressValue,
} from '@/components/ui/progress'
import { Spinner } from '@/components/ui/spinner'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { createHistoricalImport, listAllBoundChannelBindings } from '../api'
import type { SupplyChainManagementProps } from '../contracts'
import { useIdempotentIntent } from '../hooks/use-idempotent-intent'
import { useSupplyChainAdminMutation } from '../hooks/use-supply-chain-admin'
import {
  useCompletedHistoricalSeries,
  useHistoricalImportList,
} from '../hooks/use-supply-chain-historical-imports'
import { formatMicroUsd, formatPpmPercent } from '../lib/format'
import {
  historicalImportProgress,
  rollupHistoricalSeries,
} from '../lib/historical-import'
import {
  buildHistoricalMappings,
  parseHistoricalMappings,
} from '../lib/historical-mapping'
import {
  historicalImportFormSchema,
  type HistoricalImportFormValues,
} from '../lib/schemas'
import { formatTime } from '../lib/time'
import { supplyChainQueryKeys } from '../query-keys'
import type {
  SupplierChannelBinding,
  SupplierHistoricalChannelMapping,
  SupplierHistoricalImport,
  SupplierHistoricalImportCommand,
} from '../types'
import { ManagementPagination, ManagementStatus } from './management-common'

const DEFAULT_FORM: HistoricalImportFormValues = {
  start_date: '',
  end_date: '',
  quota_per_unit: '500000',
  excluded_user_ids_json: '[]',
  channel_mappings_json: '[]',
  reason: '',
}

const ASSUMPTION_LABELS: Record<string, string> = {
  sales_equals_quota_divided_by_frozen_quota_per_unit:
    'Sales amount equals quota divided by the frozen quota-per-unit value.',
  official_list_requires_valid_logged_group_ratio:
    'Official list amount requires a valid group ratio saved on each source log.',
  procurement_cost_requires_explicit_channel_mapping:
    'Procurement cost requires an explicit frozen channel mapping.',
  authoritative_reports_and_inventory_are_unchanged:
    'Authoritative reports and inventory are not changed by this estimate.',
}

function historicalStatusVariant(
  status: SupplierHistoricalImport['status']
): 'destructive' | 'default' | 'secondary' {
  if (status === 'failed') return 'destructive'
  if (status === 'completed') return 'default'
  return 'secondary'
}

function HistoricalStatusBadge(props: {
  status: SupplierHistoricalImport['status']
}) {
  const { t } = useTranslation()
  const labels = {
    pending: t('Pending'),
    running: t('Running'),
    completed: t('Completed'),
    failed: t('Failed'),
  }
  return (
    <Badge variant={historicalStatusVariant(props.status)}>
      {labels[props.status]}
    </Badge>
  )
}

function HistoricalMappingTable(props: {
  mappings: SupplierHistoricalChannelMapping[]
  bindings: SupplierChannelBinding[]
}) {
  const { t } = useTranslation()
  const bindingByChannel = useMemo(
    () =>
      new Map(props.bindings.map((binding) => [binding.channel_id, binding])),
    [props.bindings]
  )

  if (props.mappings.length === 0) {
    return (
      <Alert>
        <AlertTitle>{t('No channel mappings configured')}</AlertTitle>
        <AlertDescription>
          {t(
            'Unmapped channels are still counted, but procurement cost and gross profit remain unknown.'
          )}
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <div className='overflow-hidden rounded-xl border'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Channel')}</TableHead>
            <TableHead>{t('Supplier')}</TableHead>
            <TableHead>{t('Contract')}</TableHead>
            <TableHead>{t('Rate version')}</TableHead>
            <TableHead>{t('Procurement rate')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.mappings.map((mapping) => {
            const source = bindingByChannel.get(mapping.channel_id)
            const binding =
              source?.supplier_id === mapping.supplier_id &&
              source.supplier_contract_id === mapping.contract_id &&
              source.current_rate_version_id === mapping.rate_version_id &&
              source.current_procurement_multiplier_ppm ===
                mapping.procurement_multiplier_ppm
                ? source
                : undefined
            return (
              <TableRow key={mapping.channel_id}>
                <TableCell>
                  <div className='font-medium'>
                    {binding?.channel_name ?? `#${mapping.channel_id}`}
                  </div>
                  {binding ? (
                    <div className='text-muted-foreground'>
                      #{mapping.channel_id}
                    </div>
                  ) : null}
                </TableCell>
                <TableCell>
                  <div className='font-medium'>
                    {binding?.supplier_name ?? `#${mapping.supplier_id}`}
                  </div>
                  {binding ? (
                    <div className='text-muted-foreground'>
                      #{mapping.supplier_id}
                    </div>
                  ) : null}
                </TableCell>
                <TableCell>
                  <div className='font-medium'>
                    {binding?.contract_name ?? `#${mapping.contract_id}`}
                  </div>
                  {binding ? (
                    <div className='text-muted-foreground'>
                      {binding.contract_no
                        ? `${binding.contract_no} · #${mapping.contract_id}`
                        : `#${mapping.contract_id}`}
                    </div>
                  ) : null}
                </TableCell>
                <TableCell>#{mapping.rate_version_id}</TableCell>
                <TableCell>
                  {formatPpmPercent(
                    mapping.procurement_multiplier_ppm,
                    t('Unknown')
                  )}
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}

function HistoricalImportForm() {
  const { t } = useTranslation()
  const intent = useIdempotentIntent()
  const [mappingSources, setMappingSources] = useState<
    SupplierChannelBinding[]
  >([])
  const [generatingMappings, setGeneratingMappings] = useState(false)
  const form = useForm<HistoricalImportFormValues>({
    resolver: zodResolver(historicalImportFormSchema),
    defaultValues: DEFAULT_FORM,
  })
  const mutation = useSupplyChainAdminMutation<{
    command: SupplierHistoricalImportCommand
    key: string
  }>({
    mutationFn: ({ command, key }) =>
      createHistoricalImport({ data: command, idempotencyKey: key }),
    invalidate: [supplyChainQueryKeys.historicalImports.all()],
  })
  const mappingsJSON = form.watch('channel_mappings_json')
  const mappings = useMemo(
    () => parseHistoricalMappings(mappingsJSON),
    [mappingsJSON]
  )

  async function generateMappings(): Promise<void> {
    setGeneratingMappings(true)
    try {
      const bindings = await listAllBoundChannelBindings()
      const generated = buildHistoricalMappings(bindings)
      setMappingSources(bindings)
      form.setValue(
        'channel_mappings_json',
        JSON.stringify(generated, null, 2),
        {
          shouldDirty: true,
          shouldValidate: true,
        }
      )
      toast.success(
        t('Generated {{count}} channel mappings from current bindings', {
          count: generated.length,
        })
      )
    } catch {
      toast.error(t('Unable to load current channel bindings'))
    } finally {
      setGeneratingMappings(false)
    }
  }

  async function submit(values: HistoricalImportFormValues): Promise<void> {
    const command: SupplierHistoricalImportCommand = {
      start_date: values.start_date,
      end_date: values.end_date,
      quota_per_unit: values.quota_per_unit.trim(),
      excluded_user_ids: JSON.parse(values.excluded_user_ids_json),
      channel_mappings: JSON.parse(values.channel_mappings_json),
      reason: values.reason.trim(),
    }
    const result = await intent.run({
      execute: (key) => mutation.mutateAsync({ command, key }),
    })
    if (result === 'success') {
      toast.success(t('Historical estimate import created'))
      form.reset(DEFAULT_FORM)
    } else if (result !== 'blocked') {
      toast.error(t('Unable to create historical estimate import'))
    }
  }

  const pending = mutation.isPending || intent.isSubmitting
  return (
    <form onSubmit={form.handleSubmit(submit)}>
      <Card>
        <CardHeader>
          <CardTitle>{t('Create historical estimate import')}</CardTitle>
          <CardDescription>
            {t(
              'The end date is exclusive. All inputs are frozen into an immutable estimate command.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <FieldGroup>
            <Field data-invalid={Boolean(form.formState.errors.start_date)}>
              <FieldLabel htmlFor='historical-start-date'>
                {t('Start date')}
              </FieldLabel>
              <Input
                id='historical-start-date'
                type='date'
                aria-invalid={Boolean(form.formState.errors.start_date)}
                {...form.register('start_date')}
              />
              <FieldError>
                {form.formState.errors.start_date
                  ? t(form.formState.errors.start_date.message ?? '')
                  : null}
              </FieldError>
            </Field>
            <Field data-invalid={Boolean(form.formState.errors.end_date)}>
              <FieldLabel htmlFor='historical-end-date'>
                {t('End date (exclusive)')}
              </FieldLabel>
              <Input
                id='historical-end-date'
                type='date'
                aria-invalid={Boolean(form.formState.errors.end_date)}
                {...form.register('end_date')}
              />
              <FieldError>
                {form.formState.errors.end_date
                  ? t(form.formState.errors.end_date.message ?? '')
                  : null}
              </FieldError>
            </Field>
            <Field data-invalid={Boolean(form.formState.errors.quota_per_unit)}>
              <FieldLabel htmlFor='historical-quota-per-unit'>
                {t('Quota per unit')}
              </FieldLabel>
              <Input
                id='historical-quota-per-unit'
                inputMode='decimal'
                aria-invalid={Boolean(form.formState.errors.quota_per_unit)}
                {...form.register('quota_per_unit')}
              />
              <FieldDescription>
                {t(
                  'Use the quota-per-unit value that was active for the source logs being estimated.'
                )}
              </FieldDescription>
              <FieldError>
                {form.formState.errors.quota_per_unit
                  ? t(form.formState.errors.quota_per_unit.message ?? '')
                  : null}
              </FieldError>
            </Field>
            <Field
              data-invalid={Boolean(
                form.formState.errors.excluded_user_ids_json
              )}
            >
              <FieldLabel htmlFor='historical-excluded-users'>
                {t('Excluded user IDs (JSON)')}
              </FieldLabel>
              <Textarea
                id='historical-excluded-users'
                rows={3}
                aria-invalid={Boolean(
                  form.formState.errors.excluded_user_ids_json
                )}
                {...form.register('excluded_user_ids_json')}
              />
              <FieldDescription>
                {t('Example')}: <code>[1, 7, 23]</code>
              </FieldDescription>
              <FieldError>
                {form.formState.errors.excluded_user_ids_json
                  ? t(
                      form.formState.errors.excluded_user_ids_json.message ?? ''
                    )
                  : null}
              </FieldError>
            </Field>
            <Field
              data-invalid={Boolean(
                form.formState.errors.channel_mappings_json
              )}
            >
              <div className='flex flex-wrap items-center justify-between gap-2'>
                <FieldLabel htmlFor='historical-channel-mappings'>
                  {t('Channel mappings')}
                </FieldLabel>
                <Button
                  type='button'
                  size='sm'
                  variant='outline'
                  disabled={generatingMappings}
                  onClick={() => void generateMappings()}
                >
                  {generatingMappings ? (
                    <Spinner data-icon='inline-start' />
                  ) : null}
                  {t('Generate from current channel bindings')}
                </Button>
              </div>
              <FieldDescription>
                {t(
                  'Each mapping freezes a channel to its supplier, contract, and procurement rate version. Later binding or rate changes do not alter this estimate.'
                )}
              </FieldDescription>
              <HistoricalMappingTable
                mappings={mappings}
                bindings={mappingSources}
              />
              <Accordion className='rounded-lg border px-3'>
                <AccordionItem value='field-reference'>
                  <AccordionTrigger>
                    {t('Field descriptions and data sources')}
                  </AccordionTrigger>
                  <AccordionContent>
                    <dl className='text-muted-foreground flex flex-col gap-3'>
                      <div>
                        <dt className='text-foreground font-medium'>
                          <code>channel_id</code>
                        </dt>
                        <dd>
                          {t(
                            'Channel ID from the current channel binding list and historical consume logs.'
                          )}
                        </dd>
                      </div>
                      <div>
                        <dt className='text-foreground font-medium'>
                          <code>supplier_id</code>
                        </dt>
                        <dd>
                          {t('Supplier ID attached to the channel binding.')}
                        </dd>
                      </div>
                      <div>
                        <dt className='text-foreground font-medium'>
                          <code>contract_id</code>
                        </dt>
                        <dd>
                          {t(
                            'Contract ID from supplier_contract_id on the channel binding.'
                          )}
                        </dd>
                      </div>
                      <div>
                        <dt className='text-foreground font-medium'>
                          <code>rate_version_id</code>
                        </dt>
                        <dd>
                          {t(
                            'Procurement rate version ID. The generator uses current_rate_version_id; historical versions come from contract history.'
                          )}
                        </dd>
                      </div>
                      <div>
                        <dt className='text-foreground font-medium'>
                          <code>procurement_multiplier_ppm</code>
                        </dt>
                        <dd>
                          {t(
                            'Procurement multiplier in parts per million. For example, 600000 means 60%.'
                          )}
                        </dd>
                      </div>
                    </dl>
                  </AccordionContent>
                </AccordionItem>
                <AccordionItem value='advanced-json'>
                  <AccordionTrigger>
                    {t('Advanced JSON editor')}
                  </AccordionTrigger>
                  <AccordionContent>
                    <Textarea
                      id='historical-channel-mappings'
                      rows={10}
                      aria-label={t('Channel mappings JSON')}
                      aria-invalid={Boolean(
                        form.formState.errors.channel_mappings_json
                      )}
                      {...form.register('channel_mappings_json')}
                    />
                  </AccordionContent>
                </AccordionItem>
              </Accordion>
              <Alert>
                <AlertTitle>{t('Check historical rate changes')}</AlertTitle>
                <AlertDescription>
                  {t(
                    'The generator uses current bindings. If a binding or procurement rate changed during the selected period, split the import at its effective time and select the corresponding historical rate version.'
                  )}
                </AlertDescription>
              </Alert>
              <FieldError>
                {form.formState.errors.channel_mappings_json
                  ? t(form.formState.errors.channel_mappings_json.message ?? '')
                  : null}
              </FieldError>
            </Field>
            <Field data-invalid={Boolean(form.formState.errors.reason)}>
              <FieldLabel htmlFor='historical-reason'>{t('Reason')}</FieldLabel>
              <Textarea
                id='historical-reason'
                rows={3}
                aria-invalid={Boolean(form.formState.errors.reason)}
                {...form.register('reason')}
              />
              <FieldError>
                {form.formState.errors.reason
                  ? t(form.formState.errors.reason.message ?? '')
                  : null}
              </FieldError>
            </Field>
          </FieldGroup>
        </CardContent>
        <CardFooter className='justify-end'>
          <Button type='submit' disabled={pending}>
            {pending ? (
              <Spinner data-icon='inline-start' />
            ) : (
              <HugeiconsIcon
                icon={PlusSignIcon}
                strokeWidth={2}
                data-icon='inline-start'
              />
            )}
            {t('Create estimate import')}
          </Button>
        </CardFooter>
      </Card>
    </form>
  )
}

function HistoricalImportDetails(props: {
  item: SupplierHistoricalImport | undefined
}) {
  const { t } = useTranslation()
  const series = useCompletedHistoricalSeries(props.item)
  const points = useMemo(
    () => series.data?.pages.flatMap((page) => page.items) ?? [],
    [series.data]
  )
  const rollups = useMemo(() => rollupHistoricalSeries(points), [points])

  if (!props.item) return null
  const item = props.item
  return (
    <Card>
      <CardHeader>
        <CardTitle>
          {t('Import #{{id}}', { id: item.id })}{' '}
          <HistoricalStatusBadge status={item.status} />
        </CardTitle>
        <CardDescription>
          {item.start_date} → {item.end_date} · {item.reason}
        </CardDescription>
      </CardHeader>
      <CardContent className='flex flex-col gap-4'>
        <Alert>
          <AlertTitle>{t('Estimate coverage')}</AlertTitle>
          <AlertDescription>
            {t(
              'This import covers final successful consume logs in the frozen LOG_DB range. It does not write authoritative accounting facts.'
            )}
          </AlertDescription>
        </Alert>
        <div className='flex flex-col gap-2'>
          <div className='font-medium'>{t('Frozen assumptions')}</div>
          <ul className='text-muted-foreground list-disc ps-5 text-sm'>
            {item.assumptions.map((assumption) => (
              <li key={assumption}>
                {t(ASSUMPTION_LABELS[assumption] ?? assumption)}
              </li>
            ))}
          </ul>
        </div>
        {item.status === 'failed' ? (
          <Alert variant='destructive'>
            <AlertTitle>{t('Historical import failed')}</AlertTitle>
            <AlertDescription>
              {item.error_message || t('No error details are available.')}
            </AlertDescription>
          </Alert>
        ) : null}
        {item.status !== 'completed' ? (
          <Alert>
            <AlertTitle>
              {t('Financial estimates are not published yet')}
            </AlertTitle>
            <AlertDescription>
              {t(
                'Amounts are shown only after the frozen source count is fully verified. Partial running or failed totals remain hidden.'
              )}
            </AlertDescription>
          </Alert>
        ) : (
          <ManagementStatus
            isLoading={series.isLoading}
            isError={series.isError}
            isEmpty={!rollups.length}
          >
            <div className='flex flex-col gap-3'>
              {series.hasNextPage ? (
                <Alert>
                  <AlertTitle>
                    {t('Load all pages before viewing complete daily totals.')}
                  </AlertTitle>
                </Alert>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Date')}</TableHead>
                      <TableHead>{t('Requests')}</TableHead>
                      <TableHead>{t('Unknown')}</TableHead>
                      <TableHead>{t('Unassigned')}</TableHead>
                      <TableHead>{t('Sales')}</TableHead>
                      <TableHead>{t('Procurement cost')}</TableHead>
                      <TableHead>{t('Gross profit')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {rollups.map((row) => (
                      <TableRow key={row.date}>
                        <TableCell>{row.date}</TableCell>
                        <TableCell>
                          {row.sourceCount.toLocaleString()}
                        </TableCell>
                        <TableCell>
                          {row.unknownCount.toLocaleString()}
                        </TableCell>
                        <TableCell>
                          {row.unassignedCount.toLocaleString()}
                        </TableCell>
                        <TableCell>
                          {formatMicroUsd(row.salesMicroUsd, t('Unknown'))}
                          {row.salesUnknownCount > 0 ? (
                            <div className='text-muted-foreground text-xs'>
                              {t('Unknown')}:{' '}
                              {row.salesUnknownCount.toLocaleString()}
                            </div>
                          ) : null}
                        </TableCell>
                        <TableCell>
                          {formatMicroUsd(row.costMicroUsd, t('Unknown'))}
                          {row.costUnknownCount > 0 ? (
                            <div className='text-muted-foreground text-xs'>
                              {t('Unknown')}:{' '}
                              {row.costUnknownCount.toLocaleString()}
                            </div>
                          ) : null}
                        </TableCell>
                        <TableCell>
                          {formatMicroUsd(row.grossMicroUsd, t('Unknown'))}
                          {row.grossUnknownCount > 0 ? (
                            <div className='text-muted-foreground text-xs'>
                              {t('Unknown')}:{' '}
                              {row.grossUnknownCount.toLocaleString()}
                            </div>
                          ) : null}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
              {series.hasNextPage ? (
                <Button
                  type='button'
                  variant='outline'
                  disabled={series.isFetchingNextPage}
                  onClick={() => void series.fetchNextPage()}
                >
                  {series.isFetchingNextPage ? <Spinner /> : null}
                  {t('Load more')}
                </Button>
              ) : null}
            </div>
          </ManagementStatus>
        )}
      </CardContent>
      <CardFooter className='text-muted-foreground flex-wrap gap-3 text-xs'>
        <span>
          {t('Coverage scope')}: {item.coverage_scope}
        </span>
        <span>
          {t('Quota per unit')}: {item.quota_per_unit}
        </span>
      </CardFooter>
    </Card>
  )
}

export function HistoricalImportManagement(props: SupplyChainManagementProps) {
  const { t } = useTranslation()
  const [selectedId, setSelectedId] = useState<number>()
  const query = useHistoricalImportList(
    props.search.page,
    props.search.pageSize
  )
  const items = query.data?.items ?? []
  const selected = items.find((item) => item.id === selectedId) ?? items[0]
  return (
    <div className='flex flex-col gap-4'>
      <Alert variant='destructive'>
        <AlertTitle>
          {t('Historical estimates are non-authoritative')}
        </AlertTitle>
        <AlertDescription>
          {t(
            'They are isolated from authoritative profit reports and inventory. Use them only for explicitly reviewed historical analysis.'
          )}
        </AlertDescription>
      </Alert>
      <HistoricalImportForm />
      <Card>
        <CardHeader>
          <CardTitle>{t('Historical estimate imports')}</CardTitle>
          <CardDescription>
            {t(
              'Pending and running imports refresh automatically every 10 seconds.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <ManagementStatus
            isLoading={query.isLoading}
            isError={query.isError}
            isEmpty={!items.length}
          >
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Import')}</TableHead>
                  <TableHead>{t('Range')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead>{t('Progress')}</TableHead>
                  <TableHead>{t('Created at')}</TableHead>
                  <TableHead>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((item) => {
                  const progress =
                    item.status === 'completed'
                      ? 100
                      : historicalImportProgress(
                          item.processed_count,
                          item.candidate_count
                        )
                  return (
                    <TableRow key={item.id}>
                      <TableCell>#{item.id}</TableCell>
                      <TableCell>
                        {item.start_date} → {item.end_date}
                      </TableCell>
                      <TableCell>
                        <HistoricalStatusBadge status={item.status} />
                      </TableCell>
                      <TableCell className='min-w-48'>
                        <Progress value={progress}>
                          <ProgressLabel>
                            {item.processed_count.toLocaleString()} /{' '}
                            {item.candidate_count.toLocaleString()}
                          </ProgressLabel>
                          <ProgressValue>
                            {() => `${Math.round(progress)}%`}
                          </ProgressValue>
                        </Progress>
                      </TableCell>
                      <TableCell>{formatTime(item.created_at)}</TableCell>
                      <TableCell>
                        <Button
                          type='button'
                          size='sm'
                          variant={
                            item.id === selected?.id ? 'secondary' : 'outline'
                          }
                          onClick={() => setSelectedId(item.id)}
                        >
                          {t('View')}
                        </Button>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </ManagementStatus>
        </CardContent>
        <CardFooter>
          <ManagementPagination
            page={props.search.page}
            pageSize={props.search.pageSize}
            total={query.data?.total ?? 0}
            onSearchChange={props.onSearchChange}
          />
        </CardFooter>
      </Card>
      <HistoricalImportDetails item={selected} />
    </div>
  )
}
