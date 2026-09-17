# Phone Verification API Reminder Design

## Goal

Legacy PLG accounts (created before the phone-verification rollout start, see
`model.PhoneVerificationRolloutStart`) are exempt from the phone gate and many
of them never open the console, so they never see the binding dialog. This
feature reminds them through the API itself: at most once every 24 hours, the
first eligible text-generation request is answered by the gateway with a
well-formed, successful (HTTP 200) "model reply" that asks the user to bind a
phone number. The request is not forwarded upstream and is not billed. Every
other request in the window is relayed normally.

Accounts that the existing gate already blocks (new PLG accounts) keep getting
the existing 403 and are never reminded. Non-PLG accounts and accounts with a
verified phone are never reminded.

## Eligibility rule

The rule lives next to the gate rule so the two cannot drift apart:

`model.PhoneVerificationReminderEligibleForAccount(group, phoneVerifiedAt, createdAt)`
is true when, and only when:

- `SMS_VERIFICATION_ENABLED` is on (a user cannot bind a phone otherwise);
- the account group is `plg`;
- the account has no verified phone (`phone_verified_at == 0`);
- the account is **not** subject to the gate, i.e.
  `PhoneVerificationRequiredForAccount` is false for it. In practice this means
  `created_at` is 0 (legacy row) or earlier than the rollout start.

`UserBase.PhoneVerificationReminderEligible()` wraps it for the token-auth
middleware, which only has the cached user.

## Operator toggle

New module `operation_setting.PhoneVerificationSetting`, registered as
`phone_verification_setting`, with a single field
`api_reminder_enabled` (default `false`). It is an ordinary option, so an
administrator flips it through `PUT /api/option` and every node picks it up
without a redeploy. `operation_setting.PhoneVerificationAPIReminderEnabled()`
returns `common.SMSVerificationEnabled && setting.APIReminderEnabled`.

The feature ships off. Turning it on is a separate, explicit operator action.
Turning it off is the rollback.

## Where the decision is made

1. **`middleware.TokenAuth`** already holds the cached user and already runs the
   gate. Right after the gate passes, it sets the context key
   `constant.ContextKeyPhoneVerificationReminderEligible` to `true` when the
   toggle is on and the account is eligible. No Redis access here.
2. **`controller.Relay`** calls `maybeServePhoneVerificationReminder` right after
   `GenRelayInfo` and the "model officially unsupported" check, and before the
   sensitive-word check, token counting and pre-consume billing. At that point
   the request is parsed, the relay format, streaming flag, model name and user
   id are known, and nothing has been charged.

   The function returns `true` when it wrote the reminder response; `Relay`
   then releases any channel-concurrency lease the distributor may already
   hold and returns. It returns `false` in every other case and the request
   continues untouched.

The Playground runs `Relay` under `UserAuth`, not `TokenAuth`, so the context
key is never set there and the Playground is never intercepted.

## 24-hour deduplication

`service.ClaimPhoneVerificationReminder(userId)` performs one Redis
`SET NX EX 86400` on `phone_reminder:api:{userId}`. Exactly one concurrent
request per user wins the claim; the key expires on its own. When Redis is not
enabled the claim falls back to a process-local map with the same TTL (the
same degradation the notification limiter uses). Any Redis error is returned
to the caller, which treats it as "not claimed".

The claim is attempted only after the request has been found eligible for
synthesis (see next section). Requests the gateway cannot answer (embeddings,
images, structured output, ...) never consume the user's daily slot.

## Which requests get the synthesized reply

Only text-generation requests whose response shape the gateway can reproduce
faithfully, in both streaming and non-streaming form:

| Entry point | Format | Request type |
|---|---|---|
| `POST /v1/chat/completions` | `openai` (RelayMode chat completions) | `dto.GeneralOpenAIRequest` |
| `POST /v1/messages` | `claude` | `dto.ClaudeRequest` |
| `POST /v1/responses` | `openai_responses` | `dto.OpenAIResponsesRequest` |
| `POST /v1beta/models/{m}:generateContent` and `:streamGenerateContent` (also under `/v1/models/`) | `gemini` | `dto.GeminiChatRequest` |

Everything else is relayed normally and does not consume the slot: legacy
`/v1/completions`, `/v1/responses/compact`, embeddings, images, audio,
ElevenLabs, rerank, realtime, moderations, Gemini embeddings, task and
Midjourney routes.

A request is also skipped (relayed normally, slot untouched) when the caller
asked for machine-readable output, because a prose reminder would break such a
pipeline on the spot:

- OpenAI chat: `response_format.type` is `json_object` or `json_schema`, or
  `tool_choice` is `"required"` or an object naming a tool.
- Claude: `tool_choice.type` is `any` or `tool`.
- Responses: `text.format.type` is `json_object` or `json_schema`, or
  `tool_choice` is `"required"` or an object.
- Gemini: `generationConfig.responseMimeType` is `application/json`,
  `responseSchema` or `responseJsonSchema` is set, or
  `toolConfig.functionCallingConfig.mode` is `ANY`.

Requests that merely *offer* tools (`tool_choice` auto/none) are reminded like
plain chat; the model may or may not have called a tool anyway.

## Synthesized responses

All bodies use the model name from the request, a freshly generated id, the
current timestamp, zero usage and a normal end-of-turn finish reason. The
reminder text is the only content.

- **OpenAI non-stream**: `chat.completion` object with one `assistant` choice,
  `finish_reason: "stop"`.
- **OpenAI stream**: SSE with the standard headers; one `chat.completion.chunk`
  carrying `role` + full `content`, one chunk with `finish_reason: "stop"`, then
  `data: [DONE]`.
- **Claude non-stream**: `type: "message"`, one `text` content block,
  `stop_reason: "end_turn"`.
- **Claude stream**: `message_start`, `content_block_start` (text),
  `content_block_delta` (`text_delta`), `content_block_stop`, `message_delta`
  (`end_turn`, zero usage), `message_stop`, each as `event:` + `data:` lines.
- **Responses non-stream**: `object: "response"`, `status: "completed"`, one
  `message` output item with one `output_text` content part.
- **Responses stream**: `response.created`, `response.output_item.added`,
  `response.content_part.added`, `response.output_text.delta`,
  `response.output_text.done`, `response.content_part.done`,
  `response.output_item.done`, `response.completed`, each as `event:` + `data:`
  lines.
- **Gemini non-stream**: one candidate with role `model`, one text part,
  `finishReason: "STOP"`, zero `usageMetadata`.
- **Gemini stream**: the same candidate object as a single SSE `data:` line.

Every synthesized response carries the header
`X-Flatkey-Notice: phone_verification_reminder` so integrations can recognise
it deterministically. A `logger.LogInfo` line records the user id, model and
format. No consume log or quota mutation is produced.

## Reminder text

Backend i18n key `notify.phone_verification_reminder_for_api`, resolved with
`i18n.GetLangFromContext` (user language setting first, then Accept-Language,
then English). Template data: `SystemName` and `Link`. Written in every
backend locale that already carries the sibling key
`notify.phone_verification_required_for_api` (en, zh-CN, zh-TW, pt, es, fr,
ru, ja, vi), with real translations.

English:

> [{{.SystemName}} notice] Your account has not bound a phone number yet.
> Please sign in to the console and bind one: {{.Link}} . This request was not
> forwarded to the model and was not billed; simply send it again. This notice
> appears at most once every 24 hours.

`Link` is the console origin (`system_setting.ResolveConsoleOrigin`, falling
back to the server address) joined with `common.ThemeAwarePath("/console/personal")`,
which is `/profile` under the default theme. That page hosts the account
bindings tab and, for PLG accounts without a phone, the dismissible binding
dialog.

## Failure behaviour

The reminder must never cost a paying customer a request. Every failure path
falls through to normal relaying: toggle unreadable, Redis error, claim not
won, unknown request type, unsupported format, structured-output request,
marshal error while rendering. The only observable effect of a failure is a
system log line.

## Known trade-off

An automated client that sends plain chat requests receives, once per 24
hours, a 200 whose content is not a model answer. This is inherent to the
"synthesized model reply" channel the product chose over a 403 or an
out-of-band notification. The structured-output skip and the notice header
are the mitigations.

## Tests

- `model`: eligibility matrix (legacy plg, created_at 0, new plg is *not*
  eligible, verified, enterprise, feature flag off).
- `setting/operation_setting`: toggle requires both the feature flag and the
  setting.
- `middleware`: the eligibility helper honours toggle and rule; the context key
  is set only for eligible accounts (wired through `TokenAuth`).
- `service`: claim wins once per user per window on miniredis, key TTL is 24h,
  second claim loses, Redis error propagates; memory fallback wins once and
  expires.
- `controller`: request eligibility matrix for the four formats including every
  structured-output skip; golden bodies for the eight synthesized shapes;
  `maybeServePhoneVerificationReminder` is a no-op without the context key or
  when the claim is lost, writes a 200 with the notice header when eligible,
  and never writes anything when it returns false.

## Rollout

Ship behind the default-off option. After deployment an operator enables
`phone_verification_setting.api_reminder_enabled`. Disabling it stops all
reminders immediately; the Redis keys expire on their own.
