# Phone Collection Abuse-Prevention Disclosure Design

## Goal

Update the public Terms and Privacy pages so users are clearly told that flatkey.ai may collect phone numbers for account verification and abuse prevention.

## Scope

- Update the English default Terms and Privacy documents.
- Update the existing localized legal documents for Chinese, Spanish, French, Portuguese, Japanese, Russian, and Vietnamese.
- German and Indonesian legal routes currently fall back to the English legal documents; they inherit the updated English disclosure without changing locale routing.
- Do not describe marketing, advertising, or unrelated profiling purposes.

## Content

Privacy will identify phone number as account information and explain that it may be used for account verification, fraud prevention, bulk-registration detection, abuse prevention, security investigations, and policy enforcement. It will state that the number and related verification signals may be shared with authentication, verification, anti-fraud, security, and infrastructure providers when needed, and retained only as long as needed for service, security, legal, audit, and dispute purposes.

Terms will require truthful contact information, allow phone verification when reasonably needed for account security or abuse prevention, prohibit bypassing or falsifying verification, and clarify that abnormal or abusive accounts may be restricted or suspended.

## Implementation

Edit only the legal document string sources under `website/src/content/legal/`. Preserve existing section numbering and markdown syntax. Update each document's last-updated date to September 14, 2026.

## Validation

Run website lint, typecheck, tests, and production build. Verify rendered `/terms`, `/privacy`, `/zh/terms`, and `/ja/privacy` contain the new disclosure and that all legal locale documents remain non-empty.
