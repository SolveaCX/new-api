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
import { useMemo } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useLocation } from '@tanstack/react-router'
import { AlertCircle, ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'
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
import { Skeleton } from '@/components/ui/skeleton'
import {
  getOAuthAuthorizationDetails,
  mcpOAuthQueryKeys,
  submitOAuthAuthorization,
} from './api'
import { buildAuthorizePayload, normalizeScopes } from './lib'
import type { OAuthAuthorizeAction } from './types'

function getQueryErrorMessage(
  error: Error | null,
  response: { success: boolean; message?: string } | undefined
): string | undefined {
  if (error) return error.message
  if (response && !response.success) return response.message
  return undefined
}

export function AuthorizePage() {
  const { t } = useTranslation()
  const location = useLocation()
  const search = location.searchStr

  const detailsQuery = useQuery({
    queryKey: mcpOAuthQueryKeys.authorizationDetails(search),
    queryFn: () => getOAuthAuthorizationDetails(search),
  })

  const authorizationMutation = useMutation({
    mutationFn: (action: OAuthAuthorizeAction) =>
      submitOAuthAuthorization(buildAuthorizePayload(search, action)),
    onSuccess: (response) => {
      const redirectUrl = response.data?.redirect_url
      if (response.success && redirectUrl) {
        window.location.assign(redirectUrl)
      }
    },
  })

  const details = detailsQuery.data?.data
  const scopes = useMemo(
    () => normalizeScopes(details?.scopes ?? ''),
    [details?.scopes]
  )
  const errorMessage = getQueryErrorMessage(
    detailsQuery.error,
    detailsQuery.data
  )
  const mutationErrorMessage = getQueryErrorMessage(
    authorizationMutation.error,
    authorizationMutation.data
  )
  const isSubmitting = authorizationMutation.isPending

  return (
    <main className='bg-muted/30 flex min-h-svh items-center justify-center p-4'>
      <Card className='w-full max-w-xl'>
        <CardHeader>
          <div className='bg-primary/10 text-primary mb-2 flex h-10 w-10 items-center justify-center rounded-lg'>
            <ShieldCheck className='h-5 w-5' aria-hidden='true' />
          </div>
          <CardTitle>{t('Authorize MCP access')}</CardTitle>
          <CardDescription>
            {t('Review the app and permissions before continuing.')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-5'>
          {detailsQuery.isLoading ? (
            <div className='space-y-3'>
              <Skeleton className='h-12 w-full' />
              <Skeleton className='h-20 w-full' />
              <Skeleton className='h-8 w-2/3' />
            </div>
          ) : errorMessage ? (
            <div className='border-destructive/30 bg-destructive/10 text-destructive flex gap-3 rounded-lg border p-3 text-sm'>
              <AlertCircle className='mt-0.5 h-4 w-4 shrink-0' />
              <div>
                <p className='font-medium'>{t('Authorization unavailable')}</p>
                <p>{errorMessage}</p>
              </div>
            </div>
          ) : (
            <>
              <section className='space-y-1'>
                <p className='text-muted-foreground text-sm'>
                  {t('Application')}
                </p>
                <p className='text-lg font-semibold'>
                  {details?.client_name || t('Unknown application')}
                </p>
              </section>
              <section className='space-y-1'>
                <p className='text-muted-foreground text-sm'>
                  {t('Flatkey Tools resource')}
                </p>
                <p className='font-mono text-sm break-all'>
                  {details?.resource || t('No resource provided')}
                </p>
              </section>
              <section className='space-y-2'>
                <p className='text-muted-foreground text-sm'>
                  {t('Requested scopes')}
                </p>
                <div className='flex flex-wrap gap-2'>
                  {scopes.length > 0 ? (
                    scopes.map((scope) => (
                      <Badge key={scope} variant='secondary'>
                        {scope}
                      </Badge>
                    ))
                  ) : (
                    <span className='text-muted-foreground text-sm'>
                      {t('No scopes requested')}
                    </span>
                  )}
                </div>
              </section>
            </>
          )}
          {mutationErrorMessage ? (
            <p className='text-destructive text-sm'>{mutationErrorMessage}</p>
          ) : null}
        </CardContent>
        <CardFooter className='flex flex-col-reverse gap-2 sm:flex-row sm:justify-end'>
          <Button
            type='button'
            variant='outline'
            disabled={
              detailsQuery.isLoading || Boolean(errorMessage) || isSubmitting
            }
            onClick={() => authorizationMutation.mutate('deny')}
          >
            {t('Deny')}
          </Button>
          <Button
            type='button'
            disabled={
              detailsQuery.isLoading || Boolean(errorMessage) || isSubmitting
            }
            onClick={() => authorizationMutation.mutate('approve')}
          >
            {isSubmitting ? t('Submitting...') : t('Approve')}
          </Button>
        </CardFooter>
      </Card>
    </main>
  )
}
