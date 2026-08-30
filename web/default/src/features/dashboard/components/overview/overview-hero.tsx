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
import { type ComponentType } from 'react'
import {
  Claude,
  DeepSeek,
  Doubao,
  Gemini,
  Grok,
  Minimax,
  OpenAI,
} from '@lobehub/icons'
import { useTranslation } from 'react-i18next'

type ModelLogo = ComponentType<{
  'aria-hidden'?: boolean
  className?: string
  size?: number
}>

const featuredModels: Array<{ label: string; logo: ModelLogo }> = [
  { label: 'Seedance', logo: Doubao.Color },
  { label: 'GPT', logo: OpenAI },
  { label: 'Claude', logo: Claude.Color },
  { label: 'Gemini', logo: Gemini.Color },
  { label: 'Grok', logo: Grok },
  { label: 'DeepSeek', logo: DeepSeek.Color },
  { label: 'MiniMax', logo: Minimax.Color },
]

const WELCOME_LOGO_URL =
  'https://cdn.shulex-voc.com/flatkey/console/overview-welcome-logo.png'

function WelcomeLogo() {
  return (
    <img
      src={WELCOME_LOGO_URL}
      alt=''
      width={68}
      height={68}
      aria-hidden='true'
      decoding='async'
      className='size-[68px] shrink-0'
    />
  )
}

export function OverviewHero() {
  const { t } = useTranslation()

  return (
    <section className='flex flex-col items-center gap-6 text-center sm:gap-8'>
      <div className='flex flex-col items-center gap-4'>
        <WelcomeLogo />
        <div className='flex flex-col items-center gap-2'>
          <span className='sr-only'>{t('Your AI gateway')}</span>
          <h1 className='text-3xl font-medium tracking-[-0.02em] sm:text-4xl'>
            {t('Welcome to Flatkey')}
          </h1>
          <p className='text-foreground max-w-[min(100%,30rem)] text-base tracking-[-0.02em]'>
            {t(
              'One connection, All models. Start building with your free credits.'
            )}
          </p>
        </div>
      </div>

      <div className='bg-card flex max-w-full flex-wrap items-center justify-center gap-2 rounded-full border px-2 py-1.5 shadow-xs sm:flex-nowrap sm:px-1.5'>
        <div className='flex items-center pl-0.5'>
          {featuredModels.slice(0, 5).map((model) => {
            const Logo = model.logo
            return (
              <span
                className='bg-card -ml-2 flex size-7 items-center justify-center rounded-full border first:ml-0'
                key={model.label}
              >
                <Logo aria-hidden size={18} className='size-[18px]' />
              </span>
            )
          })}
          <span className='bg-muted -ml-2 flex size-7 items-center justify-center rounded-full border text-xs font-semibold'>
            …
          </span>
        </div>
        <span className='text-muted-foreground max-w-[15rem] pr-1 text-[13px] sm:max-w-none sm:pr-2'>
          {t('One key connects you to the models shaping AI:')}
        </span>
      </div>
    </section>
  )
}
