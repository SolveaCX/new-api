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
/**
 * Per-user persistence for dismissing the optional phone-binding prompt.
 *
 * The prompt shown to accounts outside the backend rollout gate is a
 * suggestion, not a gate, so closing it has to stick: component state alone
 * would bring the dialog back on the next reload or route remount, which is
 * indistinguishable from not being dismissible at all.
 *
 * Only the *suggestion* is dismissible. Accounts the backend actually
 * requires to bind (`phone_verification_required`) are unaffected by this
 * flag — see `shouldRequirePhoneBinding`.
 *
 * The key is namespaced by user id so a shared browser cannot leak one
 * account's dismissal to the next, matching
 * `dashboard/overview/welcome-notice-persistence.ts`. All access is guarded
 * (`typeof window` + try/catch) so private mode, disabled storage and quota
 * errors can never crash the layout.
 */

const DISMISSED_KEY_PREFIX = 'phone_binding_suggestion_dismissed:'

const FLAG_VALUE = '1'

/** A user id the flag can be keyed on. Falsy ids skip persistence. */
type UserId = number | string | null | undefined

function isStorageAvailable(): boolean {
  return typeof window !== 'undefined' && !!window.localStorage
}

function normalizeUserId(userId: UserId): string | null {
  if (userId === null || userId === undefined || userId === '') return null
  return String(userId)
}

function dismissedKey(userId: string): string {
  return `${DISMISSED_KEY_PREFIX}${userId}`
}

/** True once this user has closed the optional phone-binding prompt. */
export function hasDismissedPhoneBindingSuggestion(userId: UserId): boolean {
  const id = normalizeUserId(userId)
  if (!id || !isStorageAvailable()) return false
  try {
    return window.localStorage.getItem(dismissedKey(id)) === FLAG_VALUE
  } catch {
    // Storage is unreadable; treat the prompt as not yet dismissed so the
    // suggestion still reaches the user. It then lives for this session only.
    return false
  }
}

/** Record that the prompt was dismissed, so later visits skip it. */
export function markPhoneBindingSuggestionDismissed(userId: UserId): void {
  const id = normalizeUserId(userId)
  if (!id || !isStorageAvailable()) return
  try {
    window.localStorage.setItem(dismissedKey(id), FLAG_VALUE)
  } catch {
    // Storage is unavailable; the prompt reappears next visit, which is a
    // better failure than crashing the layout.
  }
}
