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

function WelcomeLogo() {
  return (
    <svg
      width='68'
      height='68'
      viewBox='0 0 68 68'
      fill='none'
      xmlns='http://www.w3.org/2000/svg'
      aria-hidden='true'
    >
      <rect width='68' height='68' rx='34' fill='#F6EFFF' />
      <rect
        x='12.0225'
        y='18.6762'
        width='32'
        height='32'
        rx='8'
        transform='rotate(-12 12.0225 18.6762)'
        fill='url(#welcome-logo-bg)'
      />
      <g clipPath='url(#welcome-logo-clip)'>
        <rect
          x='18'
          y='18'
          width='32'
          height='32'
          rx='8'
          fill='white'
          fillOpacity='0.32'
        />
        <path
          d='M34 25.7188C36.1875 26.6562 38.375 27.2188 40.4062 27.5C41.1875 27.625 41.6562 28.1875 41.6562 28.9688V33.2188C41.6562 37.75 38.375 40.7188 34 42.2812C29.625 40.7188 26.3438 37.75 26.3438 33.2188V28.9688C26.3438 28.1875 26.8125 27.625 27.5938 27.5C29.625 27.2188 31.8125 26.6562 34 25.7188Z'
          fill='url(#welcome-logo-shield)'
        />
        <path
          d='M34 26.043C36.1722 26.9608 38.341 27.515 40.3584 27.7949V27.7959C40.6871 27.8485 40.9319 27.9894 41.0947 28.1816C41.2578 28.3744 41.3564 28.6393 41.3564 28.9688V33.2188C41.3564 37.5339 38.2653 40.4101 34 41.9609C29.7347 40.4101 26.6436 37.5339 26.6436 33.2188V28.9688C26.6436 28.6393 26.7422 28.3744 26.9053 28.1816C27.0681 27.9894 27.3129 27.8485 27.6416 27.7959L27.6406 27.7949C29.6583 27.5151 31.8274 26.9609 34 26.043Z'
          stroke='white'
          strokeOpacity='0.6'
          strokeWidth='0.6'
        />
        <path
          d='M34 33.1562C35.1736 33.1562 36.125 32.2049 36.125 31.0312C36.125 29.8576 35.1736 28.9062 34 28.9062C32.8264 28.9062 31.875 29.8576 31.875 31.0312C31.875 32.2049 32.8264 33.1562 34 33.1562Z'
          fill='white'
        />
        <path
          d='M34 31.875C34.466 31.875 34.8438 31.4972 34.8438 31.0313C34.8438 30.5653 34.466 30.1875 34 30.1875C33.534 30.1875 33.1562 30.5653 33.1562 31.0313C33.1562 31.4972 33.534 31.875 34 31.875Z'
          fill='#6D28D9'
        />
        <path
          d='M34.75 32.875C34.75 32.4608 34.4142 32.125 34 32.125C33.5858 32.125 33.25 32.4608 33.25 32.875V37.625C33.25 38.0392 33.5858 38.375 34 38.375C34.4142 38.375 34.75 38.0392 34.75 37.625V32.875Z'
          fill='white'
        />
        <path
          d='M36.1875 35.4062H34.9375C34.6268 35.4062 34.375 35.6581 34.375 35.9687C34.375 36.2794 34.6268 36.5312 34.9375 36.5312H36.1875C36.4982 36.5312 36.75 36.2794 36.75 35.9687C36.75 35.6581 36.4982 35.4062 36.1875 35.4062Z'
          fill='white'
        />
        <path
          d='M35.625 37.125H34.9375C34.6268 37.125 34.375 37.3768 34.375 37.6875C34.375 37.9982 34.6268 38.25 34.9375 38.25H35.625C35.9357 38.25 36.1875 37.9982 36.1875 37.6875C36.1875 37.3768 35.9357 37.125 35.625 37.125Z'
          fill='white'
        />
      </g>
      <defs>
        <linearGradient
          id='welcome-logo-bg'
          x1='11.9262'
          y1='35.8554'
          x2='44.6823'
          y2='35.6615'
          gradientUnits='userSpaceOnUse'
        >
          <stop stopColor='#9F6EF3' />
          <stop offset='1' stopColor='#A97EF3' />
        </linearGradient>
        <linearGradient
          id='welcome-logo-shield'
          x1='26.3438'
          y1='25.7188'
          x2='42.8554'
          y2='40.9842'
          gradientUnits='userSpaceOnUse'
        >
          <stop stopColor='#A855F7' />
          <stop offset='1' stopColor='#6D28D9' />
        </linearGradient>
        <clipPath id='welcome-logo-clip'>
          <rect
            width='32'
            height='32'
            fill='white'
            transform='translate(18 18)'
          />
        </clipPath>
      </defs>
    </svg>
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
