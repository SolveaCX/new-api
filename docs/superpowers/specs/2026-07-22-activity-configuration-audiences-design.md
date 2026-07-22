# Activity Configuration Audience Design

## Goal

Rename the administrator-facing recall campaign area to **Activity Configuration** and add two explicit audience templates for operational use:

1. **Registered only** selects users who have registered, have never paid, and have never made an API request within an operator-configured registration time range.
2. **Specified users** selects an exact union of user IDs and email addresses so operators can run controlled tests.

The existing recall delivery, coupon, email, activation, and audit behavior remains unchanged.

## Product terminology

- Replace the user-facing recall campaign navigation label, list heading, empty state, create/edit titles, detail return label, and related action copy with Activity Configuration terminology in every supported Console locale.
- Chinese uses `活动配置` as the primary area name.
- Internal route paths, API paths, Go type names, database table names, and existing stored campaign records keep their current recall campaign names for compatibility.

## Audience templates

### Existing templates

`first_purchase`, `lapsed_payer`, and `expired_subscription` keep their current contracts and selection results. In particular, `first_purchase` is not redefined as registered-only.

### Registered only

Add the stored audience template value `registered_only`.

A user is eligible only when all of the following are true:

- the account is enabled;
- the stored email is valid;
- the user has not opted out of recall marketing;
- the email is verified when the campaign's existing `require_verified_email` option is enabled;
- the user passes the existing optional user-group allow/block filter;
- the user has no successful payment history;
- `request_count` is exactly `0`;
- `created_at` is greater than or equal to `registration_start_at` and less than or equal to `registration_end_at`.

The template exposes two required `datetime-local` controls. The browser converts the selected local timestamps to Unix seconds before submission. Both boundaries are inclusive. Validation rejects missing values, non-positive timestamps, or an end earlier than the start.

This template does not expose minimum request count, maximum quota, last API call age, payment provider, or payment-age fields because they do not define the requested audience.

### Specified users

Add the stored audience template value `specified_users`.

The editor exposes two separate controls:

- a searchable, multi-select user picker backed by the existing admin user-search capability; and
- a multiline email input accepting comma- or newline-separated email addresses.

Operators do not need to discover or type numeric user IDs. The user picker searches by the identifiers supported by the existing admin user search, displays enough account context to distinguish results, and stores the selected user IDs in `specified_user_ids`. The email input is normalized into `specified_emails`.

The backend trims values, lowercases email addresses for matching, removes duplicates, and treats the two lists as a union. A user selected in the picker and also matched by a manually entered email appears once. At least one selected user or valid email is required, and the combined normalized list is limited to 500 identifiers per campaign.

Specified users bypass the behavioral audience thresholds and registration-time filters, but they do not bypass the global delivery safety rules. The account must still be enabled, the stored email must be valid, recall marketing opt-out must be false, and verified-email enforcement follows the existing campaign switch. Optional user-group allow/block filtering is hidden for this template so an exact test list cannot be silently narrowed by unrelated audience configuration.

Unknown IDs and emails are ignored during preview and snapshot. The preview reports the normal eligible total and samples only resolved eligible users; it does not reveal whether a missing email belongs to an account.

## Data contract

Extend the existing JSON-backed audience configuration without a database migration:

```json
{
  "registration_start_at": 0,
  "registration_end_at": 0,
  "specified_user_ids": [],
  "specified_emails": []
}
```

The fields are inactive for all other templates. Existing campaign JSON continues to decode with zero values and retains its current behavior.

Eligibility snapshots continue to store the selected template and resolved user facts. Specified-user campaigns store only resolved user facts in recipient snapshots, not the operator's full identifier input.

## Backend flow

1. Validate the template-specific audience configuration before preview, create, update, or activation.
2. Build a bounded candidate query:
   - `registered_only` applies the inclusive `created_at` range, no-payment condition, and `request_count = 0` at the database boundary where possible.
   - `specified_users` resolves the normalized ID/email union in bounded batches and deduplicates by user ID.
3. Apply shared account, email, opt-out, verification, and template-specific checks in the existing selector.
4. Reuse the current preview, snapshot, recipient, email, audit, and attribution pipelines.

No new dependency or database migration is required.

## Frontend flow

- Add both templates to the audience-template selector with localized descriptions.
- `registered_only` shows registration start and end datetime controls plus the existing verified-email switch.
- `specified_users` shows a debounced searchable multi-select user picker, a separate multiline email input, their combined normalized count, and the existing verified-email switch.
- Selected users remain visible as removable chips even when the current search query changes. Loading an existing draft resolves saved user IDs to display labels; unresolved historical IDs remain visible as unavailable selections instead of being silently removed.
- Validation errors are attached to the active controls and prevent preview or save.
- Loading an existing draft reconstructs the datetime-local values, selected-user chips, and multiline email value without mutating stored identifiers.
- Changing templates preserves inactive configuration values in the draft but submits them only as inert JSON fields; the backend chooses behavior solely from `audience_template`.

## Error handling and safety

- Reject malformed email tokens, invalid selected user IDs received by the API, more than 500 combined identifiers, reversed registration ranges, and missing template-required fields.
- Do not fall back from an empty or invalid specified-user list to a broad audience.
- Do not expose exact-match misses as account enumeration details.
- Preserve admin-only access for viewing, editing, previewing, saving, and activating Activity Configuration.

## Verification

Backend tests must cover:

- exact registered-only boundaries;
- exclusion of users with any payment;
- exclusion of users with `request_count > 0`;
- specified IDs, specified emails, union deduplication, unknown identifiers, case-insensitive email matching, and the 500-identifier limit;
- shared disabled, invalid-email, opt-out, and verified-email exclusions;
- compatibility of every existing audience template and stored draft.

Frontend tests must cover:

- template options and conditional fields;
- datetime conversion and range validation;
- debounced user search, multi-selection, preserved selections across searches, manual email parsing, union deduplication, invalid-email errors, and the maximum count;
- submit payloads and legacy draft loading;
- Activity Configuration terminology in all supported locales.

Browser smoke verification must confirm an administrator can preview both new audiences, save and reopen each configuration, and that a normal user cannot open the page or call its preview endpoints.
