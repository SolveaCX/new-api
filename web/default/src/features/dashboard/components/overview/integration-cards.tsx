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
import { ArrowRight, Braces, Bot, Terminal, TerminalSquare } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  CardStaggerContainer,
  CardStaggerItem,
} from '@/components/page-transition'
import type { IntegrationId } from './integration-snippets'

interface IntegrationCard {
  id: IntegrationId
  title: string
  description: string
  icon: typeof Braces
  tone: 'neutral' | 'blue' | 'amber' | 'teal'
  badge?: string
}

function getCardBackground(tone: IntegrationCard['tone']): string {
  if (tone === 'blue') return 'bg-linear-to-r from-white via-white to-[#f0f5ff]'
  if (tone === 'amber')
    return 'bg-linear-to-r from-white via-white to-[#fffaf1]'
  if (tone === 'teal') return 'bg-linear-to-r from-white via-white to-[#effcf8]'
  return 'bg-linear-to-r from-white via-white to-[#f9f9fa]'
}

function getIconTone(tone: IntegrationCard['tone']): string {
  if (tone === 'blue') return 'bg-[#e6eeff] text-[#386fe5]'
  if (tone === 'amber') return 'bg-[#fff2dc] text-[#b87921]'
  if (tone === 'teal') return 'bg-[#d8f5ec] text-[#1d9d79]'
  return 'bg-[#f4f1f8] text-foreground'
}

// eslint-disable-next-line react-refresh/only-export-components
export function useIntegrationCards(): IntegrationCard[] {
  const { t } = useTranslation()

  return [
    {
      id: 'api',
      title: t('API for developers'),
      description: t(
        'Call any model with an OpenAI-compatible API. Copy a ready-to-run example for your model and language.'
      ),
      icon: TerminalSquare,
      tone: 'neutral',
    },
    {
      id: 'sdk',
      title: t('SDKs for developers'),
      description: t(
        'Use the OpenAI SDK you already know — with Flatkey as the gateway.'
      ),
      icon: Braces,
      tone: 'blue',
    },
    {
      id: 'cli',
      title: t('Flatkey CLI'),
      description: t(
        'Generate images and videos from your terminal. Let your AI assistant drive the workflow.'
      ),
      icon: Terminal,
      tone: 'amber',
      badge: t('Fast'),
    },
    {
      id: 'agent',
      title: t('Codex & Claude Code'),
      description: t(
        'Connect your coding agent with one command, then use Flatkey from your existing projects.'
      ),
      icon: Bot,
      tone: 'teal',
      badge: t('Simple'),
    },
  ]
}

export function IntegrationCards(props: {
  onSelect: (id: IntegrationId) => void
}) {
  const { t } = useTranslation()
  const cards = useIntegrationCards()

  return (
    <section className='flex flex-col gap-5'>
      <div className='flex flex-col gap-1 px-1 sm:px-5'>
        <h2 className='text-xl font-medium tracking-[-0.02em]'>
          {t("Choose how you'll use Flatkey")}
        </h2>
        <p className='text-sm tracking-[-0.01em] text-[#454545]'>
          {t('All four options use the same account and model catalog.')}
        </p>
      </div>

      <CardStaggerContainer className='bg-card grid gap-3 rounded-[20px] border border-black/[0.04] p-4 shadow-[0_1px_8px_rgba(0,0,0,0.04)] sm:gap-5 sm:p-6 md:grid-cols-2 dark:border-white/10'>
        {cards.map((card) => {
          const Icon = card.icon

          return (
            <CardStaggerItem key={card.id}>
              <button
                type='button'
                onClick={() => props.onSelect(card.id)}
                className={`group focus-visible:ring-ring relative flex h-full min-h-[170px] w-full flex-col items-start gap-5 rounded-2xl border border-black/[0.1] p-5 text-left transition-all hover:-translate-y-0.5 hover:shadow-md focus-visible:ring-2 focus-visible:outline-none sm:min-h-[184px] sm:p-6 ${getCardBackground(card.tone)}`}
              >
                <span
                  className={`flex size-9 items-center justify-center rounded-lg ${getIconTone(card.tone)}`}
                >
                  <Icon className='size-[18px]' aria-hidden='true' />
                </span>
                {card.id === 'agent' && (
                  <span className='absolute top-6 right-6 inline-flex items-center gap-1 text-sm font-medium text-[#386fe5]'>
                    {t('Preview')}
                    <ArrowRight className='size-4' aria-hidden='true' />
                  </span>
                )}
                <span className='flex flex-col gap-2'>
                  <span className='flex items-center gap-2 text-xl font-medium tracking-[-0.02em]'>
                    {card.title}
                    {card.badge && (
                      <span
                        className={`rounded-md px-2 py-0.5 text-xs font-medium ${
                          card.tone === 'teal'
                            ? 'bg-[#d8f5ec] text-[#1d9d79]'
                            : 'bg-[#e9efff] text-[#386fe5]'
                        }`}
                      >
                        {card.badge}
                      </span>
                    )}
                  </span>
                  <span className='text-sm leading-relaxed text-[#454545]'>
                    {card.description}
                  </span>
                </span>
              </button>
            </CardStaggerItem>
          )
        })}
      </CardStaggerContainer>
    </section>
  )
}
