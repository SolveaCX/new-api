# PLG Hidden Model Call Gate Design

## Goal

Make the pricing "hidden models" list (`pricing_visibility_setting.hidden_models`) also block API calls for PLG identities. Today the list is display-only: hidden models disappear from the pricing pages, rankings, the Console catalog and the Playground picker, but stay fully callable through every relay entry point and are still returned by `/v1/models`. After this change a PLG user can neither discover nor call a hidden model. Enterprise identities are not affected in any way.

## Identity rule

"PLG" follows the existing code definition: the user's identity group is `plg`, or empty (every non-enterprise user is served from `plg`). The token's routing group is irrelevant. The rule lives in one pure function, `service.HiddenModelBlockedForIdentity(identityGroup, modelName)`, so the middleware and the listing endpoints cannot drift apart.

## Call gate

`middleware.Distribute()` calls the gate after the token allow/deny-list checks and before channel selection. Distribute is the single channel-selection entry for `/v1/*`, `/pg/*`, `/mj`, `/suno` and `/v1beta`, so one hook covers every relay format including the Playground.

- Only requests that will select a channel are gated. Task fetches (`GET /v1/videos/{id}` etc.) resolve the model name from an already-accepted task and keep working even if the model is hidden afterwards.
- Video remix (`POST /v1/videos/{id}/remix`) is a new upstream invocation whose model is only known after the origin task is loaded, so the distributor cannot gate it. `relay.ResolveOriginTask` applies the same rule right after deriving the model and before locking the origin channel, returning a request-local 404 `model_not_found` task error (no channel penalty, no retry).
- The identity group comes from the request context, which `TokenAuth` already forces to `plg` for non-enterprise users. Playground requests carry only a session copy that can be stale, so on `/pg/` the gate reads the group from the database (same source as `resolvePlaygroundUsingGroup`). A lookup failure yields an empty group and is treated as PLG: unknown identity fails closed.
- Rejection is HTTP 404 with error code `model_not_found` and the i18n message `distributor.model_not_found` ("The model X does not exist or you do not have access to it"). The wording never reveals that the model exists and is hidden.
- Queued video-task workers are not gated. Submission already went through the gate and pre-charged the account; blocking mid-flight would require refund handling for no security gain.

## Listing endpoints

For PLG identities only:

- `/v1/models` (OpenAI, Anthropic and Gemini shapes) drops hidden models.
- `/v1/models/{model}` answers a hidden model with the same envelope as an unknown model.
- `/v1/available_models` drops hidden models.

The Console `model-access?view=available_models` and Playground catalogs already filter; they are unchanged. The token editor still lets a PLG user add a hidden model to an allowlist; the call is rejected anyway, and tightening the editor is out of scope.

## Copy

The Console setting description changes from "Hidden models remain fully callable via the API" to "PLG users cannot call hidden models through the API; enterprise users are not affected" in all eight frontend locales. The Go type comment for `PricingVisibilitySetting` documents the gate.

## Multi-node behaviour

The gate is stateless and reads the option that every node already syncs from the shared `options` table. No new cache, lock or job is introduced.

## Rollout

The gate has no feature flag: it takes effect for every model already on the hidden list the moment the router deploys. Before merging, the operator must confirm that no PLG account is currently calling a listed model, or accept that those calls start returning 404.

## Tests

- `service`: pure-function cases (PLG hit, PLG miss, empty group, enterprise never blocked, empty list inert).
- `middleware`: end-to-end `Distribute()` runs proving a PLG request is rejected before channel selection, an enterprise request with the same model succeeds, and Playground requests use the database group (stale session group in both directions, missing user fails closed).
- `controller`: PLG exclusion and enterprise retention for `/v1/models`, `/v1/models/{model}` and `/v1/available_models`.
- `relay`: remix of a hidden-model task is rejected for PLG before the channel lock, allowed for enterprise, and allowed for PLG when the model is visible.
