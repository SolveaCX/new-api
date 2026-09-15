/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import en from '@/i18n/locales/en.json'
import { describe, expect, test } from 'bun:test'
import { createInstance } from 'i18next'
import { localizeSystemRewardContent, localizeTopupLogContent } from './format'

describe('localizeSystemRewardContent', () => {
  test.each([
    ['Redemption code top-up: ＄10.000000 (ID: 42)', '＄10.000000'],
    ['通过兑换码充值 ＄10.000000 额度，兑换码ID 42', '＄10.000000'],
  ])('extracts redemption metadata from %s', (content, expectedAmount) => {
    const calls: Array<{
      key: string
      options?: Record<string, unknown>
    }> = []
    const result = localizeSystemRewardContent(content, (key, options) => {
      calls.push({ key, options })
      return 'localized redemption log'
    })

    expect(result).toBe('localized redemption log')
    expect(calls).toEqual([
      {
        key: 'Redemption code top-up: {{amount}} (ID: {{id}})',
        options: { amount: expectedAmount, id: '42' },
      },
    ])
  })
})

describe('localizeTopupLogContent', () => {
  test.each([
    [
      '管理员补单成功，充值金额: ＄10.000000，支付金额：10.00',
      'Admin top-up: {{amount}}, payment amount: {{payment}}',
      '＄10.000000',
      '10.00',
      undefined,
    ],
    [
      '使用在线充值成功，充值金额: ＄20.000000，支付金额：20.00',
      'Online top-up: {{amount}}, payment amount: {{payment}}',
      '＄20.000000',
      '20.00',
      undefined,
    ],
    [
      '使用Creem充值成功，充值额度: ＄30.000000，支付金额：30.00',
      'Creem top-up: {{amount}}, payment amount: {{payment}}',
      '＄30.000000',
      '30.00',
      undefined,
    ],
    [
      'Stripe充值成功，充值额度: ＄35.000000，支付金额：35.00',
      'Stripe top-up: {{amount}}, payment amount: {{payment}}',
      '＄35.000000',
      '35.00',
      undefined,
    ],
    [
      'Waffo充值成功，充值额度: ＄37.000000，支付金额：37.00',
      'Waffo top-up: {{amount}}, payment amount: {{payment}}',
      '＄37.000000',
      '37.00',
      undefined,
    ],
    [
      'Waffo Pancake充值成功，充值额度: ＄40.000000，支付金额: 40.00',
      'Waffo Pancake top-up: {{amount}}, payment amount: {{payment}}',
      '＄40.000000',
      '40.00',
      undefined,
    ],
    [
      'Paddle充值成功，充值额度: ＄50.000000，支付金额: 50.00',
      'Paddle top-up: {{amount}}, payment amount: {{payment}}',
      '＄50.000000',
      '50.00',
      undefined,
    ],
    [
      '自动扣费充值成功，充值金额: ＄60.000000，支付金额：60.00',
      'Automatic top-up: {{amount}}, payment amount: {{payment}}',
      '＄60.000000',
      '60.00',
      undefined,
    ],
    [
      '订阅购买成功，套餐: Pro，支付金额: 9.99，支付方式: Stripe',
      'Subscription purchase: {{plan}}, payment amount: {{amount}}, payment method: {{method}}',
      'Pro',
      '9.99',
      'Stripe',
    ],
    [
      '使用余额购买订阅成功，套餐: Pro，支付金额: 9.99，扣除额度: 100000',
      'Balance subscription purchase: {{plan}}, payment amount: {{amount}}, deducted quota: {{quota}}',
      'Pro',
      '9.99',
      '100000',
    ],
  ])('localizes %s', (content, key, first, second, third) => {
    const calls: Array<{ key: string; options?: Record<string, unknown> }> = []
    const result = localizeTopupLogContent(
      content,
      (translationKey, options) => {
        calls.push({ key: translationKey, options })
        return 'localized top-up log'
      }
    )

    expect(result).toBe('localized top-up log')
    expect(calls[0]?.key).toBe(key)
    if (
      typeof third === 'string' &&
      (key.startsWith('Subscription') || key.startsWith('Balance'))
    ) {
      expect(calls[0]?.options).toEqual({
        plan: first,
        amount: second,
        ...(key.startsWith('Balance') ? { quota: third } : { method: third }),
      })
    } else {
      expect(calls[0]?.options).toEqual({ amount: first, payment: second })
    }
  })

  test('returns null for unrelated content', () => {
    expect(localizeTopupLogContent('普通日志', () => 'localized')).toBeNull()
  })
})

describe('localizeSystemRewardContent stable system audit logs', () => {
  test.each([
    [
      '新用户获得 Pro 免费套餐（$10 等值额度，1 个月有效）',
      'New user free plan: {{plan}}, {{amount}} quota, valid for 1 month',
      { plan: 'Pro', amount: '$10' },
    ],
    [
      '邀请好友订阅成功，奖励 ＄5.000000 额度 已进入套餐抵扣账户',
      'Subscription referral reward: {{amount}} credited to package discount balance',
      { amount: '＄5.000000' },
    ],
    [
      '查看渠道密钥信息 (渠道ID: 42)',
      'Viewed channel key information (channel ID: {{id}})',
      { id: '42' },
    ],
    [
      '通用安全验证成功 (验证方式: 2FA)',
      'Security verification succeeded (method: {{method}})',
      { method: '2FA' },
    ],
    ['开始设置两步验证', 'Started 2FA setup', undefined],
    ['成功启用两步验证', '2FA enabled successfully', undefined],
    ['禁用两步验证', '2FA disabled', undefined],
    ['重新生成两步验证备用码', '2FA backup codes regenerated', undefined],
    [
      '自动扣费失败：尝试为您的绑定卡扣款 $10 失败（card_declined），请检查或更新您的支付方式以免影响使用。',
      'Automatic charge failed: attempted to charge {{amount}} ({{reason}}). Please check or update your payment method.',
      { amount: '$10', reason: 'card_declined' },
    ],
    [
      '自动扣费已成功扣款 $10，但额度入账失败（支付单号 pi_123），我们将尽快为您处理，如未到账请联系客服。',
      'Automatic charge of {{amount}} succeeded but quota credit failed (payment ID {{id}}). We will process it shortly; contact support if the quota does not arrive.',
      { amount: '$10', id: 'pi_123' },
    ],
  ])('localizes %s', (content, key, options) => {
    const calls: Array<{ key: string; options?: Record<string, unknown> }> = []
    const result = localizeSystemRewardContent(
      content,
      (translationKey, opts) => {
        calls.push({ key: translationKey, options: opts })
        return 'localized system log'
      }
    )
    expect(result).toBe('localized system log')
    expect(calls[0]).toEqual({ key, ...(options ? { options } : {}) })
  })
})

describe('historical log content with English translations', () => {
  const i18n = createInstance()
  i18n.init({
    resources: { en },
    lng: 'en',
    nsSeparator: false,
    interpolation: { escapeValue: false },
  })

  test.each([
    [
      '管理员补单成功，充值金额: ＄10.000000 额度，支付金额：10.00',
      'Admin top-up: ＄10.000000, payment amount: 10.00',
    ],
    [
      '使用在线充值成功，充值金额: ¥70.000000 额度，支付金额：10.00',
      'Online top-up: ¥70.000000, payment amount: 10.00',
    ],
    [
      '使用Creem充值成功，充值额度: 5000000 点额度，支付金额：10.00',
      'Creem top-up: 5000000, payment amount: 10.00',
    ],
    [
      '使用余额购买订阅成功，套餐: Pro，支付金额: 9.99，扣除额度: ＄9.990000 额度',
      'Balance subscription purchase: Pro, payment amount: 9.99, deducted quota: ＄9.990000',
    ],
    [
      '邀请好友订阅成功，奖励 ＄5.000000 额度 已进入套餐抵扣账户',
      'Subscription referral reward: ＄5.000000 credited to package discount balance',
    ],
    [
      '已达到邀请奖励上限，本次邀请不再获得奖励',
      'Referral reward limit reached; no reward granted for this invitation',
    ],
    [
      '自动扣费已成功扣款 $10，但额度入账失败（支付单号 pi_123），我们将尽快为您处理，如未到账请联系客服。',
      'Automatic charge of $10 succeeded but quota credit failed (payment ID pi_123). We will process it shortly; contact support if the quota does not arrive.',
    ],
    [
      '管理员增加用户额度 ＄10.000000 额度',
      'Admin increased user quota by ＄10.000000',
    ],
    [
      'rented compute node worker (east) (RTX 4090) for 3 hours, charged ＄10.000000 额度',
      'Rented compute node worker (east) (RTX 4090) for 3 hours, charged ＄10.000000',
    ],
    [
      'Video async task failed task_123, refund 500000 点额度',
      'Video async task task_123 failed, refund 500000',
    ],
    [
      '管理员减少用户额度 500000 点额度',
      'Admin decreased user quota by 500000',
    ],
    [
      '管理员覆盖用户额度从 ¥70.000000 额度 为 ¥140.000000 额度',
      'Admin changed user quota from ¥70.000000 to ¥140.000000',
    ],
    [
      '管理员强制禁用了用户的两步验证',
      "Admin forcibly disabled the user's 2FA",
    ],
    [
      '自动扣费失败：尝试为您的绑定卡扣款 $10 失败（未找到可用的支付方式），请检查或更新您的支付方式以免影响使用。',
      'Automatic charge failed: attempted to charge $10 (No available payment method found). Please check or update your payment method.',
    ],
    [
      '自动扣费失败：尝试为您的绑定卡扣款 $10 失败（扣款被拒绝或需要验证），请检查或更新您的支付方式以免影响使用。',
      'Automatic charge failed: attempted to charge $10 (Payment declined or authentication required). Please check or update your payment method.',
    ],
    [
      '自动扣费失败：尝试为您的绑定卡扣款 $10 失败（扣款未完成），请检查或更新您的支付方式以免影响使用。',
      'Automatic charge failed: attempted to charge $10 (Payment not completed). Please check or update your payment method.',
    ],
  ])(
    'preserves values without Chinese quota units: %s',
    (content, expected) => {
      const translate = (key: string, options?: Record<string, unknown>) =>
        i18n.t(key, options)
      const actual =
        localizeTopupLogContent(content, translate) ??
        localizeSystemRewardContent(content, translate)

      expect(actual).toBe(expected)
    }
  )

  test.each([
    '管理员补单成功，充值金额: ＄10.000000',
    '订阅购买成功，套餐: Pro',
    'Custom payment provider message',
  ])(
    'keeps unmatched content available for the raw fallback: %s',
    (content) => {
      expect(localizeTopupLogContent(content, () => 'translated')).toBeNull()
      expect(
        localizeSystemRewardContent(content, () => 'translated')
      ).toBeNull()
    }
  )
})
