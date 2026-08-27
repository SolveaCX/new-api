<!-- Parent: ../AGENTS.md -->

# relay/channel/oxalpha

OxAlpha is an OpenAI-compatible preview channel. The default base URL is
https://oxalpha.run/api; the adapter appends /v1/chat/completions and
normalizes a user-entered /v1 suffix. Authentication uses the channel key as
Authorization: Bearer <key>.

Only Chat Completions is supported. The adapter preserves the request model and
supported OpenAI fields so channel model mappings remain configurable, while
removing the undocumented `stream_options` field before forwarding. It reuses
the shared OpenAI response/SSE handlers. Embeddings, images, audio, rerank,
Responses, Claude, and Gemini conversions return explicit unsupported errors.

The public preview documentation does not specify stream_options, so this
channel is intentionally absent from relay/common/relay_info.go's
streamSupportedChannels map. Do not add credentials to source or tests.

<!-- MANUAL: -->
