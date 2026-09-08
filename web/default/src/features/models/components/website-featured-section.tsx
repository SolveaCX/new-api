/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  AlertTriangle,
  ArrowDown,
  ArrowUp,
  Loader2,
  Plus,
  Search,
  Trash2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import {
  getWebsiteFeaturedModels,
  updateWebsiteFeaturedModels,
  uploadWebsiteFeaturedBackgroundImage,
} from '../api'
import { modelsQueryKeys, parseModelTags } from '../lib'
import type { WebsiteFeaturedCandidate } from '../types'
import {
  filterWebsiteFeaturedCandidates,
  moveWebsiteFeaturedModel,
  type WebsiteFeaturedListItem,
} from './website-featured-utils'

function normalizeFeaturedItems(
  items: WebsiteFeaturedListItem[]
): WebsiteFeaturedListItem[] {
  return items.map((item, sortOrder) => ({ ...item, sort_order: sortOrder }))
}

function ModelTags(props: { value?: string }) {
  const tags = parseModelTags(props.value)
  if (tags.length === 0) return null
  return (
    <div className='mt-1 flex flex-wrap gap-1'>
      {tags.map((tag) => (
        <Badge key={tag} variant='secondary'>
          {tag}
        </Badge>
      ))}
    </div>
  )
}

function featuredConfigPayload(item: WebsiteFeaturedListItem) {
  return {
    model_name: item.model_name,
    display_name: item.display_name ?? '',
    description: item.description ?? '',
    tags: item.tags ?? '',
    background_image_url: item.background_image_url ?? '',
    background_image: item.background_image ?? '',
    fallback_background_image: item.fallback_background_image ?? '',
    video: item.video ?? '',
  }
}

export function WebsiteFeaturedSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [draftFeatured, setDraftFeatured] = useState<
    WebsiteFeaturedListItem[] | null
  >(null)
  const [candidateQuery, setCandidateQuery] = useState('')
  const [uploadingModelNames, setUploadingModelNames] = useState<Set<string>>(
    () => new Set()
  )
  const [pendingUploadedModelNames, setPendingUploadedModelNames] = useState<
    Set<string>
  >(() => new Set())

  const { data, isLoading, isError } = useQuery({
    queryKey: modelsQueryKeys.websiteFeatured(),
    queryFn: getWebsiteFeaturedModels,
  })

  const serverFeatured = useMemo(
    () => normalizeFeaturedItems(data?.data?.featured ?? []),
    [data?.data?.featured]
  )
  const featured = draftFeatured ?? serverFeatured
  const updateFeatured = (
    updater: (current: WebsiteFeaturedListItem[]) => WebsiteFeaturedListItem[]
  ) => {
    setDraftFeatured((current) => updater(current ?? serverFeatured))
  }

  const candidates = useMemo(
    () => data?.data?.candidates ?? [],
    [data?.data?.candidates]
  )
  const filteredCandidates = useMemo(
    () => filterWebsiteFeaturedCandidates(candidates, featured, candidateQuery),
    [candidateQuery, candidates, featured]
  )
  const savedItems = data?.data?.featured ?? []
  const isDirty =
    JSON.stringify(savedItems.map(featuredConfigPayload)) !==
    JSON.stringify(featured.map(featuredConfigPayload))

  const saveMutation = useMutation({
    mutationFn: async () => {
      const response = await updateWebsiteFeaturedModels(featured)
      if (!response.success) {
        throw new Error(response.message || t('Failed to save featured models'))
      }
      return response
    },
    onSuccess: () => {
      setDraftFeatured(null)
      setPendingUploadedModelNames(new Set())
      toast.success(t('Featured models saved successfully'))
      void queryClient.invalidateQueries({
        queryKey: modelsQueryKeys.websiteFeatured(),
      })
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to save featured models'))
    },
  })

  const addCandidate = (candidate: WebsiteFeaturedCandidate) => {
    updateFeatured((current) =>
      normalizeFeaturedItems([
        ...current,
        {
          ...candidate,
          sort_order: current.length,
        },
      ])
    )
  }

  const removeFeatured = (index: number) => {
    updateFeatured((current) =>
      normalizeFeaturedItems(
        current.filter((_, itemIndex) => itemIndex !== index)
      )
    )
  }

  const updateFeaturedField = (
    index: number,
    field: keyof Omit<
      WebsiteFeaturedListItem,
      'model_name' | 'sort_order' | 'available'
    >,
    value: string
  ) => {
    updateFeatured((current) =>
      current.map((item, itemIndex) =>
        itemIndex === index ? { ...item, [field]: value } : item
      )
    )
  }

  const uploadBackgroundImage = async (modelName: string, file?: File) => {
    if (!file) return
    if (
      !['image/png', 'image/jpeg', 'image/webp', 'image/gif'].includes(
        file.type
      )
    ) {
      toast.error(t('Please select an image file'))
      return
    }
    if (file.size > 8 * 1024 * 1024) {
      toast.error(t('Background image must be smaller than 8 MB'))
      return
    }

    setUploadingModelNames((current) => new Set(current).add(modelName))
    try {
      const response = await uploadWebsiteFeaturedBackgroundImage(file)
      const uploadedURL = response.data?.url
      if (!response.success || !uploadedURL) {
        throw new Error('upload failed')
      }

      updateFeatured((current) =>
        current.map((item) =>
          item.model_name === modelName
            ? {
                ...item,
                background_image_url: uploadedURL,
                background_image: '',
              }
            : item
        )
      )
      setPendingUploadedModelNames((current) => new Set(current).add(modelName))
      toast.success(
        t(
          'Background image uploaded. Save the featured configuration to publish it.'
        )
      )
    } catch {
      toast.error(t('Failed to upload background image'))
    } finally {
      setUploadingModelNames((current) => {
        const next = new Set(current)
        next.delete(modelName)
        return next
      })
    }
  }

  if (isLoading) {
    return (
      <div className='text-muted-foreground flex items-center justify-center gap-2 py-12'>
        <Loader2 className='h-5 w-5 animate-spin' />
        {t('Loading featured models')}
      </div>
    )
  }

  if (isError || !data?.success) {
    return (
      <Alert variant='destructive'>
        <AlertTriangle className='h-4 w-4' />
        <AlertTitle>{t('Unable to load featured models')}</AlertTitle>
        <AlertDescription>
          {t('Please retry later or check your administrator permissions.')}
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <div className='space-y-6'>
      <div className='text-muted-foreground text-sm'>
        {t(
          'Choose one or more models and arrange their order. Featured models appear first on the public model directory.'
        )}
      </div>

      <section className='space-y-3'>
        <div className='flex items-center justify-between gap-3'>
          <div>
            <h2 className='font-medium'>{t('Featured model order')}</h2>
            <p className='text-muted-foreground text-sm'>
              {t('Models not listed here keep the normal automatic ordering.')}
            </p>
          </div>
          <Button
            onClick={() => saveMutation.mutate()}
            disabled={
              !isDirty || saveMutation.isPending || uploadingModelNames.size > 0
            }
            size='sm'
          >
            {saveMutation.isPending && <Loader2 className='animate-spin' />}
            {t('Save featured configuration')}
          </Button>
        </div>

        {featured.length === 0 ? (
          <div className='border-border text-muted-foreground rounded-lg border border-dashed px-4 py-8 text-center text-sm'>
            {t(
              'No featured models configured. Add models from the list below.'
            )}
          </div>
        ) : (
          <div className='space-y-2'>
            {featured.map((item, index) => (
              <div
                className='border-border bg-card flex flex-wrap items-center gap-3 rounded-lg border p-3'
                key={`${item.model_name}-${index}`}
              >
                <span className='text-muted-foreground w-6 text-center text-sm'>
                  {index + 1}
                </span>
                <div className='min-w-0 flex-1'>
                  <div className='truncate font-medium'>{item.model_name}</div>
                  {item.vendor_name && (
                    <div className='text-muted-foreground truncate text-xs'>
                      {item.vendor_name}
                    </div>
                  )}
                  <ModelTags value={item.tags} />
                </div>
                {!item.available && (
                  <Badge variant='outline'>{t('No longer available')}</Badge>
                )}
                <div className='flex items-center gap-1'>
                  <Button
                    variant='ghost'
                    size='icon-xs'
                    onClick={() =>
                      updateFeatured((current) =>
                        moveWebsiteFeaturedModel(current, index, -1)
                      )
                    }
                    disabled={
                      index === 0 || uploadingModelNames.has(item.model_name)
                    }
                    aria-label={t('Move featured model up')}
                  >
                    <ArrowUp />
                  </Button>
                  <Button
                    variant='ghost'
                    size='icon-xs'
                    onClick={() =>
                      updateFeatured((current) =>
                        moveWebsiteFeaturedModel(current, index, 1)
                      )
                    }
                    disabled={
                      index === featured.length - 1 ||
                      uploadingModelNames.has(item.model_name)
                    }
                    aria-label={t('Move featured model down')}
                  >
                    <ArrowDown />
                  </Button>
                  <Button
                    variant='ghost'
                    size='icon-xs'
                    onClick={() => removeFeatured(index)}
                    disabled={uploadingModelNames.has(item.model_name)}
                    aria-label={t('Remove featured model')}
                  >
                    <Trash2 />
                  </Button>
                </div>
                <div className='basis-full border-t pt-3' />
                <div className='grid basis-full gap-3 md:grid-cols-2'>
                  <label className='grid gap-1 text-sm'>
                    <span className='text-muted-foreground'>
                      {t('Video URL')}
                    </span>
                    <Input
                      value={item.video ?? ''}
                      onChange={(event) =>
                        updateFeaturedField(index, 'video', event.target.value)
                      }
                      placeholder='https://... or /assets/...'
                    />
                  </label>
                  <label className='grid gap-1 text-sm'>
                    <span className='text-muted-foreground'>
                      {t('Display name')}
                    </span>
                    <Input
                      value={item.display_name ?? ''}
                      onChange={(event) =>
                        updateFeaturedField(
                          index,
                          'display_name',
                          event.target.value
                        )
                      }
                      placeholder={item.model_name}
                    />
                  </label>
                  <label className='grid gap-1 text-sm'>
                    <span className='text-muted-foreground'>{t('Tags')}</span>
                    <Input
                      value={item.tags ?? ''}
                      onChange={(event) =>
                        updateFeaturedField(index, 'tags', event.target.value)
                      }
                      placeholder='Coding, Agents'
                    />
                  </label>
                  <label className='grid gap-1 text-sm md:col-span-2'>
                    <span className='text-muted-foreground'>
                      {t('Description')}
                    </span>
                    <Textarea
                      value={item.description ?? ''}
                      onChange={(event) =>
                        updateFeaturedField(
                          index,
                          'description',
                          event.target.value
                        )
                      }
                      rows={2}
                    />
                  </label>
                  <label className='grid gap-1 text-sm'>
                    <span className='text-muted-foreground'>
                      {t('Background image URL')}
                    </span>
                    <Input
                      value={item.background_image_url ?? ''}
                      onChange={(event) =>
                        updateFeaturedField(
                          index,
                          'background_image_url',
                          event.target.value
                        )
                      }
                      placeholder='https://...'
                    />
                  </label>
                  <label className='grid gap-1 text-sm'>
                    <span className='text-muted-foreground'>
                      {t('Fallback background image URL')}
                    </span>
                    <Input
                      value={item.fallback_background_image ?? ''}
                      onChange={(event) =>
                        updateFeaturedField(
                          index,
                          'fallback_background_image',
                          event.target.value
                        )
                      }
                      placeholder='https://...'
                    />
                  </label>
                  <label className='grid gap-1 text-sm md:col-span-2'>
                    <span className='text-muted-foreground'>
                      {t('Upload background image')}
                    </span>
                    <Input
                      type='file'
                      accept='image/png,image/jpeg,image/webp,image/gif'
                      disabled={uploadingModelNames.has(item.model_name)}
                      onChange={(event) => {
                        const file = event.target.files?.[0]
                        event.currentTarget.value = ''
                        void uploadBackgroundImage(item.model_name, file)
                      }}
                    />
                    {uploadingModelNames.has(item.model_name) && (
                      <span className='text-muted-foreground flex items-center gap-1 text-xs'>
                        <Loader2 className='h-3 w-3 animate-spin' />
                        {t('Uploading background image...')}
                      </span>
                    )}
                    {pendingUploadedModelNames.has(item.model_name) && (
                      <span className='text-muted-foreground text-xs'>
                        {t(
                          'Background image uploaded. Save the featured configuration to publish it.'
                        )}
                      </span>
                    )}
                    {item.background_image && (
                      <span className='text-muted-foreground text-xs'>
                        {t(
                          'An uploaded image is configured and will be used before the URL.'
                        )}
                      </span>
                    )}
                  </label>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>

      <section className='space-y-3'>
        <div>
          <h2 className='font-medium'>{t('Available public models')}</h2>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Only models currently shown on the public model directory can be selected.'
            )}
          </p>
        </div>
        <div className='relative max-w-md'>
          <Search className='text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 h-4 w-4 -translate-y-1/2' />
          <Input
            value={candidateQuery}
            onChange={(event) => setCandidateQuery(event.target.value)}
            placeholder={t('Search public models')}
            className='pl-8'
            aria-label={t('Search public models')}
          />
        </div>
        <div className='grid gap-2 md:grid-cols-2'>
          {filteredCandidates.map((candidate) => (
            <div
              className='border-border flex items-center gap-3 rounded-lg border p-3'
              key={candidate.model_name}
            >
              <div className='min-w-0 flex-1'>
                <div className='truncate text-sm font-medium'>
                  {candidate.model_name}
                </div>
                {candidate.vendor_name && (
                  <div className='text-muted-foreground truncate text-xs'>
                    {candidate.vendor_name}
                  </div>
                )}
                <ModelTags value={candidate.tags} />
              </div>
              <Button
                variant='outline'
                size='sm'
                onClick={() => addCandidate(candidate)}
              >
                <Plus />
                {t('Feature')}
              </Button>
            </div>
          ))}
        </div>
        {filteredCandidates.length === 0 && (
          <div className='text-muted-foreground py-6 text-center text-sm'>
            {t('No additional public models found')}
          </div>
        )}
      </section>
    </div>
  )
}
