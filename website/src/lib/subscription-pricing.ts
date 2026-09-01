/**
 * Public USD contract for the three standard subscription plans.
 *
 * The backend stores quota amounts in internal units. Keeping the published
 * contract in dollars gives the website a stable source for customer-facing
 * copy without coupling it to the installation's quota-unit setting.
 */
export const STANDARD_SUBSCRIPTION_LIMITS = {
  go: { priceUsd: 10, fiveHourUsd: 8, sevenDayUsd: 12, monthlyUsd: 25 },
  pro: { priceUsd: 30, fiveHourUsd: 18, sevenDayUsd: 45, monthlyUsd: 90 },
  max: { priceUsd: 100, fiveHourUsd: 78, sevenDayUsd: 220, monthlyUsd: 450 },
} as const;

export function formatUsd(amount: number): string {
  return `$${amount.toLocaleString("en-US", { maximumFractionDigits: 2 })}`;
}
