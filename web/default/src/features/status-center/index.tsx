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
import { useTranslation } from 'react-i18next'
import { useIsRoot } from '@/hooks/use-admin'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { SectionPageLayout } from '@/components/layout'
import {
  SecureVerificationDialog,
  useSecureVerification,
} from '@/features/auth/secure-verification'
import { IncidentsPanel } from './components/incidents'
import { MaintenancePanel } from './components/maintenance'
import {
  SubscribersPanel,
  DeliveriesPanel,
  AuditPanel,
} from './components/operations'
import { OverviewPanel } from './components/overview'
import { SettingsPanel } from './components/settings'
import { getStatusCenterPermissions } from './types'

type StatusCenterTab =
  | 'overview'
  | 'incidents'
  | 'maintenance'
  | 'subscribers'
  | 'deliveries'
  | 'settings'
  | 'audit'

const tabs: StatusCenterTab[] = [
  'overview',
  'incidents',
  'maintenance',
  'subscribers',
  'deliveries',
  'settings',
  'audit',
]

export function StatusCenter() {
  const { t } = useTranslation()
  const isRoot = useIsRoot()
  const [activeTab, setActiveTab] = useState<StatusCenterTab>('overview')
  const role = isRoot ? 'root' : 'admin'
  const permissions = getStatusCenterPermissions(role, false)
  const verification = useSecureVerification()

  const runSensitiveAction = async (action: () => Promise<unknown>) => {
    await verification.startVerification(action, {
      title: t('statusCenter.verification.title'),
      description: t('statusCenter.verification.description'),
    })
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('statusCenter.title')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <p className='text-muted-foreground text-sm'>
          {t('statusCenter.description')}
        </p>
        <Tabs
          value={activeTab}
          onValueChange={(value) => setActiveTab(value as StatusCenterTab)}
          className='gap-4'
        >
          <div className='overflow-x-auto pb-1'>
            <TabsList aria-label={t('statusCenter.tabs.label')}>
              {tabs.map((tab) => (
                <TabsTrigger key={tab} value={tab}>
                  {t(`statusCenter.tabs.${tab}`)}
                </TabsTrigger>
              ))}
            </TabsList>
          </div>

          <TabsContent value='overview'>
            <OverviewPanel
              active={activeTab === 'overview'}
              role={role}
              runSensitiveAction={runSensitiveAction}
            />
          </TabsContent>
          <TabsContent value='incidents'>
            <IncidentsPanel active={activeTab === 'incidents'} />
          </TabsContent>
          <TabsContent value='maintenance'>
            <MaintenancePanel active={activeTab === 'maintenance'} />
          </TabsContent>
          <TabsContent value='subscribers'>
            <SubscribersPanel active={activeTab === 'subscribers'} />
          </TabsContent>
          <TabsContent value='deliveries'>
            <DeliveriesPanel
              active={activeTab === 'deliveries'}
              isRoot={permissions.canViewRootControls}
              runSensitiveAction={runSensitiveAction}
            />
          </TabsContent>
          <TabsContent value='settings'>
            <SettingsPanel
              active={activeTab === 'settings'}
              isRoot={permissions.canViewRootControls}
              runSensitiveAction={runSensitiveAction}
            />
          </TabsContent>
          <TabsContent value='audit'>
            <AuditPanel active={activeTab === 'audit'} />
          </TabsContent>
        </Tabs>
      </SectionPageLayout.Content>

      <SecureVerificationDialog
        open={verification.open}
        onOpenChange={verification.setOpen}
        methods={verification.methods}
        state={verification.state}
        onVerify={(method, code) => {
          void verification.executeVerification(method, code)
        }}
        onCancel={verification.cancel}
        onCodeChange={verification.setCode}
        onMethodChange={verification.switchMethod}
      />
    </SectionPageLayout>
  )
}
