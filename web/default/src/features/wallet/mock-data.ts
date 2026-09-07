import type { LifecyclePlanRecord } from './lib/subscription-plan-lifecycle'
import type { PresetAmount, TopupInfo, UserWalletData } from './types'

/**
 * Development-only fixtures used by /wallet?mock=1 so the layout can be
 * reviewed without a running API or authenticated session.
 */
export const MOCK_WALLET_USER: UserWalletData = {
  id: 1001,
  username: 'design-preview',
  quota: 498_000,
  used_quota: 0,
  request_count: 0,
  aff_quota: 0,
  aff_history_quota: 0,
  aff_count: 0,
  group: 'plg',
}

export const MOCK_PLANS: LifecyclePlanRecord[] = [
  {
    plan: {
      id: 1,
      title: 'Go',
      subtitle: 'For individuals and light everyday use',
      price_amount: 10,
      currency: 'USD',
      duration_unit: 'month',
      duration_value: 1,
      quota_reset_period: 'monthly',
      enabled: true,
      sort_order: 1,
      allow_balance_pay: true,
      max_purchase_per_user: 0,
      total_amount: 45_000_000,
      window_5h_amount: 10_000_000,
      window_week_amount: 18_000_000,
      model_count: 100,
      rpm: 60,
      concurrency: 5,
      feature_lines: '100+ models',
      payment_modes: ['stripe_recurring', 'balance_one_period'],
    },
  },
  {
    plan: {
      id: 2,
      title: 'Pro',
      subtitle: 'For daily development and frequent requests',
      price_amount: 30,
      currency: 'USD',
      duration_unit: 'month',
      duration_value: 1,
      quota_reset_period: 'monthly',
      enabled: true,
      sort_order: 2,
      allow_balance_pay: true,
      max_purchase_per_user: 0,
      total_amount: 90_000_000,
      window_5h_amount: 30_000_000,
      window_week_amount: 60_000_000,
      model_count: 100,
      rpm: 120,
      concurrency: 10,
      feature_lines: '100+ models',
      payment_modes: ['stripe_recurring', 'balance_one_period'],
    },
  },
  {
    plan: {
      id: 3,
      title: 'Max',
      subtitle: 'For teams and high-intensity workloads',
      price_amount: 100,
      currency: 'USD',
      duration_unit: 'month',
      duration_value: 1,
      quota_reset_period: 'monthly',
      enabled: true,
      sort_order: 3,
      allow_balance_pay: true,
      max_purchase_per_user: 0,
      total_amount: 300_000_000,
      window_5h_amount: 80_000_000,
      window_week_amount: 240_000_000,
      model_count: 100,
      rpm: 240,
      concurrency: 20,
      feature_lines: '100+ models',
      payment_modes: ['stripe_recurring', 'balance_one_period'],
    },
  },
]

export const MOCK_TOPUP_INFO: TopupInfo = {
  enable_online_topup: false,
  enable_stripe_topup: true,
  pay_methods: [],
  min_topup: 1,
  stripe_min_topup: 1,
  amount_options: [10, 20, 50, 100, 200],
  stripe_currency_prices: {
    USD: { 10: 1000, 20: 2000, 50: 5000, 100: 10000, 200: 20000 },
  },
  discount: {},
  bonus: {},
  client_region: 'US',
}

export const MOCK_PRESET_AMOUNTS: PresetAmount[] = [
  { value: 10 },
  { value: 20 },
  { value: 50 },
  { value: 100 },
  { value: 200 },
]
