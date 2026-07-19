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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { KeyRound, Send } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  configureStatusDiscord,
  getStatusSettings,
  statusCenterQueryKeys,
  testStatusDiscord,
  updateStatusSetting,
} from '../api'
import { formatStatusTimestamp } from '../format'
import {
  getStatusSettingLabel,
  resolveStatusMutationError,
  type StatusSetting,
} from '../types'
import { EmptyState, ErrorState, ForbiddenState, LoadingState } from './common'

const discordSettingKey = 'status.discord.webhook_endpoint'
const discordSettingPrefix = 'status.discord.'

type SettingsPanelProps = {
  active: boolean
  isRoot: boolean
  runSensitiveAction: (action: () => Promise<unknown>) => Promise<void>
}

export function SettingsPanel(props: SettingsPanelProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [replacements, setReplacements] = useState<Record<string, string>>({})
  const [discordEndpoint, setDiscordEndpoint] = useState('')

  const settingsQuery = useQuery({
    queryKey: statusCenterQueryKeys.settings(),
    queryFn: getStatusSettings,
    enabled: props.active && props.isRoot,
  })
  const reload = async () => {
    await queryClient.invalidateQueries({
      queryKey: statusCenterQueryKeys.settings(),
    })
  }
  const onError = async (error: unknown) => {
    const resolution = await resolveStatusMutationError(error, reload)
    toast.error(t(resolution.messageKey))
  }

  const settingMutation = useMutation({
    mutationFn: (input: { setting: StatusSetting; value: string }) =>
      updateStatusSetting(input.setting.key, {
        value: input.value,
        expected_version: input.setting.version,
      }),
    onSuccess: (setting) => {
      setReplacements((current) => ({ ...current, [setting.key]: '' }))
      toast.success(t('statusCenter.settings.saved'))
      void reload()
    },
    onError,
  })
  const discordMutation = useMutation({
    mutationFn: (expectedVersion: number) =>
      configureStatusDiscord({
        endpoint: discordEndpoint.trim(),
        expected_version: expectedVersion,
      }),
    onSuccess: () => {
      setDiscordEndpoint('')
      toast.success(t('statusCenter.settings.discordSaved'))
      void reload()
    },
    onError,
  })
  const discordTestMutation = useMutation({
    mutationFn: testStatusDiscord,
    onSuccess: () => toast.success(t('statusCenter.settings.discordTestSent')),
    onError,
  })

  if (!props.isRoot) return <ForbiddenState />
  if (settingsQuery.isLoading) return <LoadingState />
  if (settingsQuery.isError) {
    return <ErrorState onRetry={() => void settingsQuery.refetch()} />
  }

  const settings = settingsQuery.data ?? []
  const discordSetting = settings.find(
    (setting) => setting.key === discordSettingKey
  )
  const readOnlySettings = settings.filter(
    (setting) =>
      setting.key !== discordSettingKey &&
      setting.key.startsWith(discordSettingPrefix)
  )
  const ordinarySettings = settings.filter(
    (setting) =>
      setting.key !== discordSettingKey &&
      !setting.key.startsWith(discordSettingPrefix)
  )

  const saveSetting = (setting: StatusSetting) => {
    const replacement = replacements[setting.key]?.trim() ?? ''
    if (!replacement) return
    void props.runSensitiveAction(() =>
      settingMutation.mutateAsync({ setting, value: replacement })
    )
  }

  return (
    <div className='space-y-4'>
      <Card>
        <CardHeader>
          <CardTitle>{t('statusCenter.settings.title')}</CardTitle>
          <CardDescription>
            {t('statusCenter.settings.description')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-3'>
          {readOnlySettings.map((setting) => {
            const settingLabel = getStatusSettingLabel(setting.key)
            return (
              <div
                key={setting.key}
                className='flex flex-wrap items-center justify-between gap-3 rounded-lg border p-3'
              >
                <div className='min-w-0'>
                  <div className='truncate font-medium'>
                    {t(settingLabel.key, settingLabel.values)}
                  </div>
                  <code className='text-muted-foreground block truncate text-xs'>
                    {setting.key}
                  </code>
                  <div className='text-muted-foreground text-xs'>
                    {setting.sensitive
                      ? t('statusCenter.settings.secretNeverShown')
                      : setting.value || t('statusCenter.notConfigured')}
                    {' · '}
                    {formatStatusTimestamp(setting.updated_at)}
                  </div>
                </div>
                <Badge variant='outline'>
                  {t('statusCenter.settings.readOnly')}
                </Badge>
              </div>
            )
          })}
          {ordinarySettings.length === 0 && readOnlySettings.length === 0 ? (
            <EmptyState descriptionKey='statusCenter.empty.settings' />
          ) : (
            ordinarySettings.map((setting) => {
              const settingLabel = getStatusSettingLabel(setting.key)
              return (
                <div
                  key={setting.key}
                  className='grid gap-3 rounded-lg border p-3 lg:grid-cols-[1fr_1fr_auto] lg:items-end'
                >
                  <div className='min-w-0'>
                    <div className='truncate font-medium'>
                      {t(settingLabel.key, settingLabel.values)}
                    </div>
                    <code className='text-muted-foreground block truncate text-xs'>
                      {setting.key}
                    </code>
                    <div className='text-muted-foreground text-xs'>
                      {setting.sensitive
                        ? t('statusCenter.settings.secretNeverShown')
                        : setting.value || t('statusCenter.notConfigured')}
                      {' · '}
                      {formatStatusTimestamp(setting.updated_at)}
                    </div>
                  </div>
                  <div className='space-y-2'>
                    <Label htmlFor={`setting-${setting.key}`}>
                      {t('statusCenter.settings.replacementValue')}
                    </Label>
                    <Input
                      id={`setting-${setting.key}`}
                      type={setting.sensitive ? 'password' : 'text'}
                      autoComplete='new-password'
                      value={replacements[setting.key] ?? ''}
                      onChange={(event) =>
                        setReplacements((current) => ({
                          ...current,
                          [setting.key]: event.target.value,
                        }))
                      }
                      placeholder={t(
                        'statusCenter.settings.replacementPlaceholder'
                      )}
                    />
                  </div>
                  <Button
                    type='button'
                    disabled={
                      !(replacements[setting.key] ?? '').trim() ||
                      settingMutation.isPending
                    }
                    onClick={() => saveSetting(setting)}
                  >
                    <KeyRound aria-hidden='true' />
                    {t('statusCenter.settings.verifyAndSave')}
                  </Button>
                </div>
              )
            })
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('statusCenter.settings.discordTitle')}</CardTitle>
          <CardDescription>
            {t('statusCenter.settings.discordDescription')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div>
            <div className='font-medium'>
              {t(getStatusSettingLabel(discordSettingKey).key)}
            </div>
            <code className='text-muted-foreground text-xs'>
              {discordSettingKey}
            </code>
          </div>
          <div className='flex flex-wrap items-center gap-2'>
            <Badge
              variant={discordSetting?.configured ? 'secondary' : 'outline'}
            >
              {discordSetting?.configured
                ? t('statusCenter.configured')
                : t('statusCenter.notConfigured')}
            </Badge>
            <span className='text-muted-foreground text-sm'>
              {t('statusCenter.settings.secretNeverShown')}
            </span>
          </div>
          <div className='space-y-2'>
            <Label htmlFor='status-discord-endpoint'>
              {t('statusCenter.settings.discordReplacement')}
            </Label>
            <Input
              id='status-discord-endpoint'
              type='password'
              autoComplete='new-password'
              value={discordEndpoint}
              onChange={(event) => setDiscordEndpoint(event.target.value)}
              placeholder={t('statusCenter.settings.discordPlaceholder')}
            />
          </div>
          <div className='flex flex-wrap gap-2'>
            <Button
              type='button'
              disabled={!discordEndpoint.trim() || discordMutation.isPending}
              onClick={() =>
                void props.runSensitiveAction(() =>
                  discordMutation.mutateAsync(discordSetting?.version ?? 0)
                )
              }
            >
              <KeyRound aria-hidden='true' />
              {t('statusCenter.settings.verifyAndSave')}
            </Button>
            <Button
              type='button'
              variant='outline'
              disabled={
                !discordSetting?.configured || discordTestMutation.isPending
              }
              onClick={() =>
                void props.runSensitiveAction(() =>
                  discordTestMutation.mutateAsync()
                )
              }
            >
              <Send aria-hidden='true' />
              {t('statusCenter.settings.verifyAndTest')}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
