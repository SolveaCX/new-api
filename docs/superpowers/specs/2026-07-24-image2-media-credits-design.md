# gpt-image-2 Subscription Media Credits Design

## Goal

Charge `gpt-image-2*` subscription requests against `UserSubscription.MediaCreditsUsed` instead of `AmountUsed`, while preserving wallet fallback and request-id idempotency.

## Billing contract

- A media credit represents `$0.01`.
- Convert the final internal quota to credits with `ceil(quota / QuotaPerUnit * 100)`; with the current `QuotaPerUnit=500000`, quota `3161` costs one credit.
- Recognize `gpt-image-2`, provider-qualified `*/gpt-image-2`, and `gpt-image-2-*` aliases as media subscription traffic.
- Subscription media traffic does not consume the general subscription pool or the 5-hour/7-day windows.
- If no active subscription has enough media credits, `subscription_first` falls back to wallet billing. Database and system errors remain 5xx and never masquerade as insufficient balance.

## Data and transaction design

Extend `SubscriptionPreConsumeRecord` with a pool discriminator whose default is the existing general quota pool. Pre-consume selects and row-locks the active entitlement, checks the selected pool, writes the request-id record, and increments the selected used column in one database transaction.

Media settlement records the absolute target credit amount on the same request-id record and applies only the difference to `media_credits_used`. Repeating the same settlement is therefore a no-op. Refund locks the same record, reverses its current amount from the recorded pool, and marks it refunded; repeating refund is also a no-op. The implementation remains correct across multiple router nodes because no correctness decision depends on process-local state.

## Runtime flow

`NewBillingSession` chooses media mode from `RelayInfo.OriginModelName`. `SubscriptionFunding` keeps raw quota for token-key accounting but translates subscription funding operations to absolute media credits. Media mode bypasses subscription window reservation. General subscription and wallet behavior are unchanged.

`RelayInfo` records the subscription pool type so usage logs can distinguish media credits from general quota and subscription low-balance email logic can avoid formatting credits as dollar quota.

## Failure behavior

- Media pool insufficient: return the dedicated model sentinel, map it to `ErrorCodeInsufficientUserQuota`, then fall back to wallet.
- Storage/migration/transaction failure: return the existing data error path; do not fall back.
- Upstream request failure before settlement: refund the request-id record, including any absolute media reservation already applied.
- Settlement retry: set the same absolute credit target and make no second deduction.

## Validation

Automated tests cover conversion rounding, media-only pre-consume, absolute settlement idempotency, refund idempotency, no general-window consumption, and wallet fallback. Build and focused `model`, `service`, and Codex image tests must pass before deployment. Production verification checks wallet unchanged, `amount_used` unchanged, and `media_credits_used` incremented by one for a low image request.
