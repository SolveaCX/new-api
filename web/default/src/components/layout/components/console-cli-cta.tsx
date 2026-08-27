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
import { ExternalLink, Terminal } from 'lucide-react'
import { officialWebsiteUrl } from '@/lib/origins'
import { Button } from '@/components/ui/button'

export function ConsoleCliCta() {
  return (
    <Button
      size='sm'
      className='h-9 gap-1.5 border-violet-400/30 bg-gradient-to-r from-violet-700 to-fuchsia-700 px-2 text-xs font-semibold text-white shadow-[0_10px_24px_-14px_rgba(124,58,237,0.85)] hover:from-violet-600 hover:to-fuchsia-600 focus-visible:ring-violet-500/40 dark:border-violet-300/30 dark:text-white'
      render={
        <a
          href={officialWebsiteUrl('/cli')}
          target='_blank'
          rel='noopener noreferrer'
        />
      }
    >
      <Terminal aria-hidden='true' />
      <span className='hidden sm:inline'>Flatkey CLI</span>
      <span className='sm:hidden'>CLI</span>
      <ExternalLink className='hidden sm:block' aria-hidden='true' />
    </Button>
  )
}
