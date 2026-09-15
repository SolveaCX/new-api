# Phone Collection Abuse-Prevention Disclosure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Tell users on flatkey.ai Terms and Privacy pages that phone numbers may be collected and used for account security and abuse prevention.

**Architecture:** Keep the existing static legal-document source. Update the English default document and each existing localized document; German and Indonesian routes continue using the English fallback already defined by `getDefaultLegalDocument`.

**Tech Stack:** Next.js 16, TypeScript template strings, Bun, Vitest/bun:test.

---

### Task 1: Update Terms disclosures

**Files:**
- Modify: `website/src/content/legal/default-documents.ts`
- Modify: `website/src/content/legal/localized-default-documents.ts`
- Modify: `website/src/content/legal/localized-default-documents-es.ts`
- Modify: `website/src/content/legal/localized-default-documents-pt.ts`

- [ ] Add a phone-verification sentence to the account/contact section in every Terms translation. State that verification may be required for account security or abuse prevention, and that falsifying or bypassing it is prohibited.
- [ ] Update each Terms last-updated date to September 14, 2026.

### Task 2: Update Privacy disclosures

**Files:** Same four legal source files.

- [ ] Add phone number to information collected in every Privacy translation.
- [ ] Add purposes: account verification, fraud prevention, bulk-registration detection, abuse prevention, security investigation, and policy enforcement.
- [ ] Add limited sharing with authentication, verification, anti-fraud, security, and infrastructure providers when needed.
- [ ] Add retention language tied to service, security, legal, audit, and dispute needs.
- [ ] Update each Privacy last-updated date to September 14, 2026.

### Task 3: Validate

**Files:** No new test files; use existing website tests.

- [ ] Run `bun run lint` from `website/`.
- [ ] Run `bun run typecheck` from `website/`.
- [ ] Run `bun test` from `website/`.
- [ ] Run `bun run build` from `website/`.
- [ ] Confirm generated page content for `/terms`, `/privacy`, `/zh/terms`, and `/ja/privacy` includes phone and abuse-prevention language.

### Task 4: Commit

- [ ] Commit design, plan, and legal copy changes with message `docs/legal: disclose phone collection for abuse prevention`.
