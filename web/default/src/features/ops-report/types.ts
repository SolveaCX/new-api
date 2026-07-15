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

export interface ApiResponse<T = unknown> {
  success: boolean
  message: string
  data: T
}

export interface OpsFunnelRow {
  key: string
  registrations: number
  real_browse: number
  manual_keys: number
  key_users: number
  pay_intent: number
  paid: number
  paid_usd: number
  cost_usd: number
}

export interface OpsDailyRow extends OpsFunnelRow {
  ads_cost_usd: number
  ads_clicks: number
}

export interface OpsNameCount {
  name: string
  count: number
}

export interface OpsCampaignRow extends OpsFunnelRow {
  keywords: string[] | null
  languages: string[] | null
  landing_pages: OpsNameCount[] | null
  match_types: OpsNameCount[] | null
  trend: number[] | null
}

export interface OpsKeywordRow extends OpsFunnelRow {
  campaigns: string[] | null
}

export interface OpsDauRow {
  date: string
  active_users: number
  requests: number
  quota_usd: number
}

export interface OpsPayerRow {
  user_id: number
  username: string
  display_name: string
  email: string
  paid_usd: number
  orders: number
  first_paid_at: number
  registered_at: number
  campaign: string
  keyword: string
  lng: string
  browser_lang: string
  landing: string
  signup_method: string
  currencies: string[] | null
  last_ip: string
  ip_country: string
  pay_country: string
  balance_usd: number
  consumed_usd: number
  requests: number
  last_active_at: number
  top_models: string[] | null
}

export interface OpsPaymentRow {
  key: string
  intent: number
  unpaid: number
  first: number
  first_usd: number
  repeat: number
  repeat_usd: number
}

export interface OpsReportData {
  generated_at: number
  days: number
  daily: OpsDailyRow[]
  weekly_funnel: OpsFunnelRow[]
  campaign_funnel: OpsCampaignRow[]
  keyword_funnel: OpsKeywordRow[] | null
  payment_weekly: OpsPaymentRow[]
  dau: OpsDauRow[]
  total_paid_users: number
  total_paid_usd: number
  top_payers: OpsPayerRow[] | null
  registered_users: OpsRegisteredUserRow[] | null
}

export interface OpsRegisteredUserRow {
  user_id: number
  username: string
  display_name: string
  email: string
  signup_method: string
  registered_at: number
  campaign: string
  keyword: string
  lng: string
  browser_lang: string
  landing: string
  last_ip: string
  ip_country: string
  pay_country: string
  balance_usd: number
  consumed_usd: number
  requests: number
  paid_usd: number
  last_active_at: number
}

export type OpsStripePersonStatus = 'paid' | 'failed' | 'no_action' | 'setup'

export interface OpsStripePersonRow {
  user_id: number
  email: string
  display_name: string
  billing_names: string[] | null
  locales: string[] | null
  landing: string
  referrer: string
  registered_at: number
  balance_usd: number
  last_ip: string
  ip_country: string
  campaign: string
  keyword: string
  lng: string
  browser_lang: string
  signup_method: string
  requests: number
  consumed_usd: number
  first_at: number
  last_at: number
  sessions: number
  completed: number
  attempts: number
  succeeded: number
  amounts: OpsNameCount[] | null
  methods: string[] | null
  card_country: string[] | null
  card_brands: string[] | null
  billing_cc: string[] | null
  fail_reasons: OpsNameCount[] | null
  status: OpsStripePersonStatus
}

export interface OpsStripeReport {
  generated_at: number
  days: number
  sessions_created: number
  sessions_completed: number
  sessions_expired: number
  charges_succeeded: number
  charges_failed: number
  charges_blocked: number
  persons: OpsStripePersonRow[] | null
  unmatched_sessions: number
  capped: boolean
}

// --- AdPilot (广告投放) board ---

export interface AdsPilotCampaignDaily {
  date: string
  campaign_id: string
  campaign_name: string
  cost_usd: number
  clicks: number
  impressions: number
  conversions: number
  signups: number
  intents: number
  paid_count: number
  paid_usd: number
  waste_usd: number
  updated_at: number
}

export interface AdsPilotCampaignSummary {
  campaign_id: string
  campaign_name: string
  cost_usd: number
  clicks: number
  impressions: number
  conversions: number
  signups: number
  intents: number
  paid_count: number
  paid_usd: number
  waste_usd: number
}

export type AdsPilotSeverity = 'info' | 'warn' | 'alert'

export interface AdsPilotInsight {
  id: number
  created_at: number
  severity: AdsPilotSeverity
  rule: string
  campaign_id: string
  campaign_name: string
  title: string
  detail: string
  dedup_key: string
  status: 'open' | 'acked'
  acked_by: number
  acked_at: number
}

export interface AdsPilotAction {
  id: number
  created_at: number
  rule: string
  action_type: string
  campaign_id: string
  campaign_name: string
  target: string
  params: string
  mode: 'auto' | 'approved'
  status: 'done' | 'failed' | 'reverted'
  revert_info: string
}

export type AdsPilotProposalStatus =
  | 'pending'
  | 'approved'
  | 'rejected'
  | 'executed'
  | 'failed'

export interface AdsPilotProposal {
  id: number
  created_at: number
  rule: string
  kind: 'budget' | 'bidding' | 'keyword' | 'copy'
  campaign_id: string
  campaign_name: string
  title: string
  detail: string
  expected_impact: string
  dedup_key: string
  status: AdsPilotProposalStatus
  decided_by: number
  decided_at: number
  executed_at: number
  result: string
}

export interface AdsPilotMeta {
  id: number
  last_sync_at: number
  last_push_at: number
  last_error: string
  conv_upload_fresh_at: number
  kill_switch: boolean
}

export interface AdsPilotReport {
  generated_at: number
  days: number
  meta: AdsPilotMeta | null
  stale: boolean
  campaigns: AdsPilotCampaignSummary[] | null
  daily: AdsPilotCampaignDaily[] | null
  insights: AdsPilotInsight[] | null
  actions: AdsPilotAction[] | null
  proposals: AdsPilotProposal[] | null
}
