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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link2, PlugZap } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { formatTimestampToDate } from '@/lib/format'
import {
  getConnectedApps,
  mcpOAuthQueryKeys,
  revokeConnectedApp,
} from './api'
import { formatConnectedAppTimestamp } from './lib'
import type { ConnectedApp } from './types'

function splitScopes(scopes: string): string[] {
  return scopes.split(/\s+/).filter(Boolean)
}

function getQueryErrorMessage(
  error: Error | null,
  response: { success: boolean; message?: string } | undefined
): string | undefined {
  if (error) return error.message
  if (response && !response.success) return response.message
  return undefined
}

export function ConnectedAppsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [revokeTarget, setRevokeTarget] = useState<ConnectedApp | null>(null)

  const appsQuery = useQuery({
    queryKey: mcpOAuthQueryKeys.connectedApps,
    queryFn: getConnectedApps,
  })

  const revokeMutation = useMutation({
    mutationFn: (grantPublicId: string) => revokeConnectedApp(grantPublicId),
    onSuccess: async (response) => {
      if (!response.success) return
      toast.success(t('Connected app revoked'))
      setRevokeTarget(null)
      await queryClient.invalidateQueries({
        queryKey: mcpOAuthQueryKeys.connectedApps,
      })
    },
  })

  const apps = useMemo(() => appsQuery.data?.data ?? [], [appsQuery.data?.data])
  const errorMessage = getQueryErrorMessage(appsQuery.error, appsQuery.data)

  return (
    <div className='space-y-6 p-4 md:p-6'>
      <div>
        <h1 className='text-2xl font-semibold tracking-tight'>
          {t('Connected Apps')}
        </h1>
        <p className='text-muted-foreground mt-1 text-sm'>
          {t('Review and revoke apps connected to your account.')}
        </p>
      </div>

      {appsQuery.isLoading ? (
        <div className='grid gap-3'>
          <Skeleton className='h-32 w-full' />
          <Skeleton className='h-32 w-full' />
        </div>
      ) : errorMessage ? (
        <Card>
          <CardHeader>
            <CardTitle>{t('Connected apps unavailable')}</CardTitle>
            <CardDescription>{errorMessage}</CardDescription>
          </CardHeader>
        </Card>
      ) : apps.length === 0 ? (
        <Card>
          <CardContent className='flex min-h-48 flex-col items-center justify-center text-center'>
            <div className='bg-muted mb-3 flex h-10 w-10 items-center justify-center rounded-lg'>
              <PlugZap className='h-5 w-5' aria-hidden='true' />
            </div>
            <p className='font-medium'>{t('No connected apps')}</p>
            <p className='text-muted-foreground mt-1 text-sm'>
              {t('Approved apps will appear here.')}
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className='grid gap-3'>
          {apps.map((app) => {
            const isActive = app.status === 'active'
            const appName = app.display_name || app.client_id
            return (
              <Card key={app.grant_public_id}>
                <CardHeader className='gap-3 sm:flex-row sm:items-start sm:justify-between'>
                  <div className='flex min-w-0 gap-3'>
                    <div className='bg-muted flex h-10 w-10 shrink-0 items-center justify-center rounded-lg'>
                      <Link2 className='h-5 w-5' aria-hidden='true' />
                    </div>
                    <div className='min-w-0'>
                      <CardTitle className='truncate text-base'>
                        {appName}
                      </CardTitle>
                    </div>
                  </div>
                  <div className='flex items-center gap-2'>
                    <Badge variant={isActive ? 'default' : 'secondary'}>
                      {isActive ? t('Active') : t('Revoked')}
                    </Badge>
                    {isActive ? (
                      <Button
                        type='button'
                        variant='destructive'
                        size='sm'
                        aria-label={t('Revoke {{name}}', { name: appName })}
                        onClick={() => setRevokeTarget(app)}
                      >
                        {t('Revoke')}
                      </Button>
                    ) : null}
                  </div>
                </CardHeader>
                <CardContent className='space-y-4'>
                  <div className='flex flex-wrap gap-2'>
                    {splitScopes(app.scopes).map((scope) => (
                      <Badge key={scope} variant='outline'>
                        {scope}
                      </Badge>
                    ))}
                  </div>
                  <dl className='grid gap-3 text-sm sm:grid-cols-2'>
                    <div>
                      <dt className='text-muted-foreground'>{t('Created')}</dt>
                      <dd>{formatTimestampToDate(app.created_time)}</dd>
                    </div>
                    <div>
                      <dt className='text-muted-foreground'>
                        {t('Last used')}
                      </dt>
                      <dd>
                        {formatConnectedAppTimestamp(
                          app.last_used_at,
                          formatTimestampToDate,
                          t('Never')
                        )}
                      </dd>
                    </div>
                  </dl>
                </CardContent>
              </Card>
            )
          })}
        </div>
      )}

      <ConfirmDialog
        open={Boolean(revokeTarget)}
        onOpenChange={(open) => {
          if (!open) setRevokeTarget(null)
        }}
        title={t('Revoke connected app')}
        desc={t(
          'Revoke access for {{name}}? This app will no longer be able to use your authorization grant.',
          { name: revokeTarget?.display_name || revokeTarget?.client_id || '' }
        )}
        destructive
        confirmText={t('Revoke')}
        isLoading={revokeMutation.isPending}
        handleConfirm={() => {
          if (revokeTarget) {
            revokeMutation.mutate(revokeTarget.grant_public_id)
          }
        }}
      />
    </div>
  )
}
