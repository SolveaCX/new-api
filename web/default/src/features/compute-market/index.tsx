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
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { SectionPageLayout } from '@/components/layout'
import { Marketplace } from './components/marketplace'
import { MyRequests } from './components/my-requests'
import { RFQForm } from './components/rfq-form'
import { SupplierProfile } from './components/supplier-profile'
import type { ComputeMarketTab } from './keys'

export function ComputeMarket({ tab }: { tab: ComputeMarketTab }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const go = (next: ComputeMarketTab) =>
    void navigate({ to: '/compute/market', search: { tab: next } })
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Compute Market')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <Tabs
          value={tab}
          onValueChange={(v) => go(v as ComputeMarketTab)}
          className='gap-4'
        >
          <TabsList>
            <TabsTrigger value='market'>{t('Marketplace')}</TabsTrigger>
            <TabsTrigger value='requests'>{t('My requests')}</TabsTrigger>
            <TabsTrigger value='post'>{t('Post a request')}</TabsTrigger>
            <TabsTrigger value='supplier'>{t('Supplier profile')}</TabsTrigger>
          </TabsList>
          <TabsContent value='market'>
            <Marketplace onRegister={() => go('supplier')} />
          </TabsContent>
          <TabsContent value='requests'>
            <MyRequests onPost={() => go('post')} />
          </TabsContent>
          <TabsContent value='post'>
            <RFQForm />
          </TabsContent>
          <TabsContent value='supplier'>
            <SupplierProfile />
          </TabsContent>
        </Tabs>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
