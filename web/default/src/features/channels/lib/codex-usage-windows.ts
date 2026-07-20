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
export type CodexRateLimitWindow = {
  used_percent?: number
  reset_at?: number
  reset_after_seconds?: number
  limit_window_seconds?: number
}

export type CodexRateLimit = {
  plan_type?: string
  allowed?: boolean
  limit_reached?: boolean
  primary_window?: CodexRateLimitWindow
  secondary_window?: CodexRateLimitWindow
}

export type CodexRateLimitSource = {
  plan_type?: string
  rate_limit?: CodexRateLimit
}

export type VisibleCodexLimitWindow<
  TWindow extends CodexRateLimitWindow = CodexRateLimitWindow,
> = {
  kind: 'fiveHour' | 'weekly'
  window: TWindow
}

function classifyWindowByDuration(
  windowData?: CodexRateLimitWindow | null
): VisibleCodexLimitWindow['kind'] | null {
  const seconds = Number(windowData?.limit_window_seconds)
  if (!Number.isFinite(seconds) || seconds <= 0) return null
  if (seconds === 5 * 60 * 60) return 'fiveHour'
  if (seconds === 7 * 24 * 60 * 60) return 'weekly'
  return null
}

export function resolveCodexLimitWindows(data: CodexRateLimitSource | null): {
  fiveHourWindow: CodexRateLimitWindow | null
  weeklyWindow: CodexRateLimitWindow | null
} {
  const rateLimit = data?.rate_limit ?? {}
  const primary = rateLimit.primary_window ?? null
  const secondary = rateLimit.secondary_window ?? null
  const windows = [primary, secondary].filter(Boolean) as CodexRateLimitWindow[]
  const planType = String(data?.plan_type ?? rateLimit.plan_type ?? '')
    .trim()
    .toLowerCase()
  let fiveHourWindow: CodexRateLimitWindow | null = null
  let weeklyWindow: CodexRateLimitWindow | null = null

  for (const window of windows) {
    const kind = classifyWindowByDuration(window)
    if (kind === 'fiveHour' && !fiveHourWindow) {
      fiveHourWindow = window
      continue
    }
    if (kind === 'weekly' && !weeklyWindow) {
      weeklyWindow = window
    }
  }

  if (planType === 'free') fiveHourWindow = null

  return { fiveHourWindow, weeklyWindow }
}

export function getVisibleCodexLimitWindows<
  TWindow extends CodexRateLimitWindow = CodexRateLimitWindow,
>(
  fiveHourWindow?: TWindow | null,
  weeklyWindow?: TWindow | null
): VisibleCodexLimitWindow<TWindow>[] {
  const windows: Array<{
    kind: VisibleCodexLimitWindow['kind']
    window?: TWindow | null
  }> = [
    { kind: 'fiveHour', window: fiveHourWindow },
    { kind: 'weekly', window: weeklyWindow },
  ]

  return windows.filter(
    (item): item is VisibleCodexLimitWindow<TWindow> =>
      !!item.window && Object.keys(item.window).length > 0
  )
}
