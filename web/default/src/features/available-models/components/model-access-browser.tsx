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
import { Link } from '@tanstack/react-router'
import { ArrowRight01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import {
  filterModelAccessModels,
  getCreateKeySearch,
  getModelAccessScopeModels,
  getModelEndpointFilters,
  getModelEndpointLabel,
  isFixedModelAccessView,
  resolveModelAccessScope,
  type ModelEndpointFilter,
} from '../lib/model-access-browser'
import type { UserModelAccess } from '../types'
import { ModelAccessList } from './model-access-list'
import { ModelAccessScopeRail } from './model-access-scope-rail'

type ModelAccessBrowserProps = {
  access: UserModelAccess
}

export function ModelAccessBrowser({ access }: ModelAccessBrowserProps) {
  const { t } = useTranslation()
  const fixedView = isFixedModelAccessView(access)
  const [selectedScopeId, setSelectedScopeId] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [endpoint, setEndpoint] = useState<ModelEndpointFilter>('all')

  const activeScopeId = resolveModelAccessScope(access, selectedScopeId)

  const scopeModels = useMemo(
    () => getModelAccessScopeModels(access, activeScopeId),
    [access, activeScopeId]
  )
  const endpointFilters = useMemo(
    () => getModelEndpointFilters(scopeModels),
    [scopeModels]
  )

  const activeEndpoint = endpointFilters.includes(endpoint) ? endpoint : 'all'

  const visibleModels = useMemo(
    () => filterModelAccessModels(scopeModels, query, activeEndpoint),
    [activeEndpoint, query, scopeModels]
  )
  const selectedScope = access.groups.find(
    (scope) => scope.id === activeScopeId
  )
  const description = fixedView
    ? t('View models and compatible endpoints available to your account.')
    : t('View models supported by each access group and compatible endpoint.')

  const clearFilters = () => {
    setQuery('')
    setEndpoint('all')
  }

  const catalog = (
    <div className='flex min-w-0 flex-col gap-3'>
      {fixedView && (
        <Card size='sm'>
          <CardHeader>
            <CardTitle>{t('Current account scope')}</CardTitle>
          </CardHeader>
          <CardContent>
            <p className='text-muted-foreground text-sm' aria-live='polite'>
              {t('{{count}} models available', { count: scopeModels.length })}
            </p>
          </CardContent>
          <CardFooter>
            <Button
              size='sm'
              className='w-full justify-between sm:w-auto'
              render={<Link to='/keys' search={getCreateKeySearch()} />}
            >
              {t('Create API Key')}
              <HugeiconsIcon
                icon={ArrowRight01Icon}
                strokeWidth={2}
                data-icon='inline-end'
                aria-hidden='true'
              />
            </Button>
          </CardFooter>
        </Card>
      )}

      {!fixedView && (
        <div className='flex flex-col gap-2 lg:hidden'>
          <NativeSelect
            className='w-full'
            value={activeScopeId ?? ''}
            aria-label={t('Access groups')}
            onChange={(event) => setSelectedScopeId(event.target.value)}
          >
            {access.groups.map((scope) => (
              <NativeSelectOption key={scope.id} value={scope.id}>
                {scope.label} ·{' '}
                {t('{{count}} models available', {
                  count: scope.model_ids.length,
                })}
              </NativeSelectOption>
            ))}
          </NativeSelect>
          {selectedScope && (
            <Card size='sm'>
              <CardHeader>
                <CardTitle>{selectedScope.label}</CardTitle>
              </CardHeader>
              <CardContent className='flex flex-col gap-2'>
                {selectedScope.description && (
                  <p className='text-muted-foreground text-xs'>
                    {selectedScope.description}
                  </p>
                )}
                <div className='flex flex-wrap gap-1.5'>
                  <Badge variant='secondary'>
                    {t('{{count}} models available', {
                      count: selectedScope.model_ids.length,
                    })}
                  </Badge>
                  {selectedScope.ratio !== null && (
                    <Badge variant='outline'>
                      {t('Ratio')} {selectedScope.ratio}×
                    </Badge>
                  )}
                </div>
              </CardContent>
              <CardFooter>
                <Button
                  size='sm'
                  variant='outline'
                  className='w-full justify-between'
                  render={
                    <Link
                      to='/keys'
                      search={getCreateKeySearch(selectedScope.id)}
                    />
                  }
                >
                  {t('Use this group to create an API key')}
                  <HugeiconsIcon
                    icon={ArrowRight01Icon}
                    strokeWidth={2}
                    data-icon='inline-end'
                    aria-hidden='true'
                  />
                </Button>
              </CardFooter>
            </Card>
          )}
        </div>
      )}

      <div className='bg-card flex flex-col gap-2.5 rounded-xl border p-3 sm:p-4'>
        <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
          <div className='min-w-0'>
            {!fixedView && (
              <h3 className='truncate text-sm font-semibold'>
                {selectedScope?.label}
              </h3>
            )}
            <p className='text-muted-foreground text-xs' aria-live='polite'>
              {t('{{count}} models available', {
                count: `${visibleModels.length} / ${scopeModels.length}`,
              })}
            </p>
          </div>
          <Input
            className='w-full sm:w-64'
            value={query}
            aria-label={t('Search models')}
            placeholder={t('Search models')}
            onChange={(event) => setQuery(event.target.value)}
          />
        </div>

        <div className='overflow-x-auto pb-0.5'>
          <ToggleGroup
            value={[activeEndpoint]}
            variant='outline'
            size='sm'
            aria-label={t('Endpoints')}
            onValueChange={(values) => {
              if (values[0]) setEndpoint(values[0])
            }}
          >
            {endpointFilters.map((filter) => (
              <ToggleGroupItem key={filter} value={filter}>
                {getModelEndpointLabel(filter, t)}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
        </div>
      </div>

      <ModelAccessList
        models={visibleModels}
        scopeIsEmpty={scopeModels.length === 0}
        onClearFilters={clearFilters}
      />
    </div>
  )

  return (
    <div className='mx-auto flex w-full max-w-7xl flex-col gap-4'>
      <p className='text-muted-foreground max-w-3xl text-sm'>{description}</p>
      {fixedView ? (
        catalog
      ) : (
        <div className='grid min-w-0 gap-4 lg:grid-cols-[18rem_minmax(0,1fr)] lg:items-start'>
          <aside className='hidden lg:sticky lg:top-0 lg:block'>
            <ModelAccessScopeRail
              scopes={access.groups}
              selectedScopeId={activeScopeId}
              onScopeChange={setSelectedScopeId}
            />
          </aside>
          {catalog}
        </div>
      )}
    </div>
  )
}
