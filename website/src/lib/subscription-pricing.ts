/**
 * Public USD contract for the three standard subscription plans.
 *
 * The backend stores quota amounts in internal units. Keeping the published
 * contract in dollars gives the website a stable source for customer-facing
 * copy without coupling it to the installation's quota-unit setting.
 */
export const STANDARD_SUBSCRIPTION_LIMITS = {
  go: { priceUsd: 10, monthlyUsd: 13 },
  pro: { priceUsd: 30, monthlyUsd: 45 },
  max: { priceUsd: 100, monthlyUsd: 170 },
} as const;

export function formatUsd(amount: number): string {
  return `$${amount.toLocaleString("en-US", { maximumFractionDigits: 2 })}`;
}
