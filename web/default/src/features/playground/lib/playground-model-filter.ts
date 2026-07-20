/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
const NON_CHAT_MODEL_PATTERNS = [
  /(^|[-_/])(?:dall[ -]?e|gpt-image|imagen|flux|stable-diffusion|sdxl|midjourney|jimeng|qwen-image|z-image)(?:$|[-_/])/,
  /(^|[-_/])(?:image|video|seedance|sora|kling|veo|wan|hailuo|runway|pika|luma)(?:$|[-_/])/,
  /(^|[-_/])(?:tts|whisper|transcribe|speech|audio-preview|audio)(?:$|[-_/])/,
  /(^|[-_/])(?:embedding|embeddings|rerank|reranker|moderation|suno|music|lyrics)(?:$|[-_/])/,
  /^mj_/,
]

// Allowlist of image models that are actually driven through the normal chat
// completions endpoint: they take a text prompt and return the generated image
// as a markdown data-URI (`![](data:image/png;base64,...)`) inside the assistant
// message, which the Playground renders inline via <Response>/Streamdown. These
// belong in the Playground even though their names contain an `-image` segment
// that the NON_CHAT patterns above would otherwise exclude.
//
// Scoped deliberately to Google's chat-capable image models (nano-banana and the
// Gemini flash/pro *image* variants, with or without a `google/` prefix or a
// `-preview` suffix). Do NOT widen this to gpt-image / dall-e / imagen / flux /
// etc. — those use the dedicated images endpoint and would not render here.
const CHAT_CAPABLE_IMAGE_PATTERNS = [
  /(^|[-_/])nano-banana(?:$|[-_/])/,
  /(^|\/)gemini[a-z0-9.-]*-(?:flash|pro)(?:-lite)?-image(?:-preview)?$/,
]

// Allowlist of video-generation models that ARE supported end-to-end by the
// Playground. Unlike chat/image models they do NOT run through chat completions:
// the send path detects them (see `isVideoGenModelName`) and drives the async
// `/v1/videos` submit → poll → content flow, rendering an inline `<video>` in the
// assistant bubble. They still need to be selectable in the model picker, so they
// are allowlisted here even though their name matches the `veo`/`video` NON_CHAT
// pattern above.
//
// Scoped deliberately to Google's veo *generate* models (veo-3.1 / veo-3.0, fast
// or standard, with or without a `google/` prefix or a `-preview` suffix). Do NOT
// widen this to seedance / kling / sora / other video families — those are not
// wired to the video flow and would silently fail.
const VIDEO_GEN_PATTERNS = [/(^|\/)veo[-a-z0-9.]*generate[-a-z0-9.]*$/]

export function isPlaygroundChatModelName(model: unknown): model is string {
  if (typeof model !== 'string') return false
  const normalized = model.trim().toLowerCase()
  if (!normalized) return false
  // Allowlist wins: a chat-capable image model — or a supported veo video-gen
  // model — stays visible even though it also matches an `-image` / `veo` /
  // `video` non-chat pattern below.
  if (VIDEO_GEN_PATTERNS.some((pattern) => pattern.test(normalized))) return true
  if (CHAT_CAPABLE_IMAGE_PATTERNS.some((pattern) => pattern.test(normalized)))
    return true
  return !NON_CHAT_MODEL_PATTERNS.some((pattern) => pattern.test(normalized))
}

/**
 * True when `model` is a supported video-generation model (veo *generate*).
 * The Playground send path uses this to route to the async `/v1/videos` flow
 * instead of chat completions. Mirrors the `VIDEO_GEN_PATTERNS` allowlist so the
 * picker (which shows these models via `isPlaygroundChatModelName`) and the send
 * path never disagree about what counts as a video model.
 */
export function isVideoGenModelName(model: unknown): model is string {
  if (typeof model !== 'string') return false
  const normalized = model.trim().toLowerCase()
  if (!normalized) return false
  return VIDEO_GEN_PATTERNS.some((pattern) => pattern.test(normalized))
}
