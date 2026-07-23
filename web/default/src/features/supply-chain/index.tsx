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
import { ArrowLeft01Icon, ArrowRight01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { SectionPageLayout } from '@/components/layout'
import { ChannelBindingManagement } from './components/channel-binding-management'
import { ContractManagement } from './components/contract-management'
import { DailyReportSection } from './components/daily-report-section'
import { ExclusionManagement } from './components/exclusion-management'
import { ReportBreakdownTable } from './components/report-breakdown-table'
import { ReportChannelTable } from './components/report-channel-table'
import { ReportContractTable } from './components/report-contract-table'
import { ReportFreshnessAlert } from './components/report-freshness-alert'
import { ReportOverview } from './components/report-overview'
import { ReportTrend } from './components/report-trend'
import { SupplierManagement } from './components/supplier-management'
import {
  SUPPLY_CHAIN_TABS,
  type SupplyChainManagementProps,
  type SupplyChainTab,
} from './contracts'
import {
  useSupplyChainReportBreakdown,
  useSupplyChainReportChannels,
  useSupplyChainReportContracts,
  useSupplyChainReportFreshness,
  useSupplyChainReportOverview,
  useSupplyChainReportTrend,
} from './hooks/use-supply-chain-report'
import { shiftNaturalMonth } from './lib/time'

export function SupplyChain(props: SupplyChainManagementProps) {
  const { t } = useTranslation()
  const reportQuery = {
    month: props.search.month,
    supplierIds: props.search.supplierId
      ? [props.search.supplierId]
      : undefined,
    contractIds: props.search.contractId
      ? [props.search.contractId]
      : undefined,
    channelIds: props.search.channelId ? [props.search.channelId] : undefined,
  } as const
  const reportPageQuery = {
    ...reportQuery,
    limit: props.search.pageSize,
  }
  const reportEnabled = props.search.tab === 'report'
  const overview = useSupplyChainReportOverview(reportQuery, reportEnabled)
  const trend = useSupplyChainReportTrend(reportQuery, reportEnabled)
  const contracts = useSupplyChainReportContracts(
    reportPageQuery,
    reportEnabled
  )
  const channels = useSupplyChainReportChannels(reportPageQuery, reportEnabled)
  const breakdown = useSupplyChainReportBreakdown(
    reportPageQuery,
    reportEnabled
  )
  const freshness = useSupplyChainReportFreshness(reportEnabled)
  const reportFailed =
    overview.isError ||
    trend.isError ||
    contracts.isError ||
    channels.isError ||
    breakdown.isError ||
    freshness.isError

  function changeMonth(offset: number) {
    const nextMonth = shiftNaturalMonth(props.search.month, offset)
    if (nextMonth) props.onSearchChange({ month: nextMonth, page: 1 })
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Supply Chain')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <div className='flex items-center gap-1'>
          <Button
            type='button'
            variant='outline'
            size='icon-sm'
            aria-label={t('Previous month')}
            onClick={() => changeMonth(-1)}
          >
            <HugeiconsIcon
              icon={ArrowLeft01Icon}
              strokeWidth={2}
              data-icon='inline-start'
            />
          </Button>
          <div className='min-w-24 text-center text-sm font-medium tabular-nums'>
            {props.search.month}
          </div>
          <Button
            type='button'
            variant='outline'
            size='icon-sm'
            aria-label={t('Next month')}
            onClick={() => changeMonth(1)}
          >
            <HugeiconsIcon
              icon={ArrowRight01Icon}
              strokeWidth={2}
              data-icon='inline-end'
            />
          </Button>
        </div>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <Tabs
          value={props.search.tab}
          onValueChange={(value) => {
            if (SUPPLY_CHAIN_TABS.includes(value as SupplyChainTab)) {
              props.onSearchChange({ tab: value as SupplyChainTab, page: 1 })
            }
          }}
          className='min-w-0'
        >
          <TabsList
            variant='line'
            className='max-w-full justify-start overflow-x-auto'
            aria-label={t('Supply chain sections')}
          >
            <TabsTrigger value='report'>{t('Reports')}</TabsTrigger>
            <TabsTrigger value='suppliers'>{t('Suppliers')}</TabsTrigger>
            <TabsTrigger value='contracts'>{t('Contracts')}</TabsTrigger>
            <TabsTrigger value='exclusions'>
              {t('Excluded accounts')}
            </TabsTrigger>
            <TabsTrigger value='channel-bindings'>
              {t('Channel bindings')}
            </TabsTrigger>
          </TabsList>

          <TabsContent value='report' className='flex flex-col gap-4 pt-2'>
            {reportFailed ? (
              <Alert variant='destructive'>
                <AlertTitle>
                  {t('Unable to load supply chain report')}
                </AlertTitle>
                <AlertDescription>
                  {t(
                    'Some report sections could not be loaded. Do not treat the visible totals as complete.'
                  )}
                </AlertDescription>
              </Alert>
            ) : null}
            {freshness.data ? (
              <ReportFreshnessAlert freshness={freshness.data} />
            ) : null}
            <DailyReportSection query={reportQuery} enabled={reportEnabled} />
            <ReportOverview
              data={overview.data}
              isLoading={overview.isLoading}
            />
            <ReportTrend data={trend.data} isLoading={trend.isLoading} />
            <ReportContractTable
              data={contracts.data}
              isLoading={contracts.isLoading}
              hasMore={contracts.hasNextPage}
              isLoadingMore={contracts.isFetchingNextPage}
              onLoadMore={() => void contracts.fetchNextPage()}
            />
            <ReportChannelTable
              data={channels.data}
              isLoading={channels.isLoading}
              hasMore={channels.hasNextPage}
              isLoadingMore={channels.isFetchingNextPage}
              onLoadMore={() => void channels.fetchNextPage()}
            />
            <ReportBreakdownTable
              data={breakdown.data}
              isLoading={breakdown.isLoading}
              hasMore={breakdown.hasNextPage}
              isLoadingMore={breakdown.isFetchingNextPage}
              onLoadMore={() => void breakdown.fetchNextPage()}
            />
          </TabsContent>

          <TabsContent value='suppliers' className='pt-2'>
            <SupplierManagement {...props} />
          </TabsContent>
          <TabsContent value='contracts' className='pt-2'>
            <ContractManagement {...props} />
          </TabsContent>
          <TabsContent value='exclusions' className='pt-2'>
            <ExclusionManagement {...props} />
          </TabsContent>
          <TabsContent value='channel-bindings' className='pt-2'>
            <ChannelBindingManagement {...props} />
          </TabsContent>
        </Tabs>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
