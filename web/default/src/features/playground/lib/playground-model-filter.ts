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

export function isPlaygroundChatModelName(model: unknown): model is string {
  if (typeof model !== 'string') return false
  const normalized = model.trim().toLowerCase()
  if (!normalized) return false
  // Allowlist wins: a chat-capable image model stays visible even though it also
  // matches an `-image` non-chat pattern below.
  if (CHAT_CAPABLE_IMAGE_PATTERNS.some((pattern) => pattern.test(normalized)))
    return true
  return !NON_CHAT_MODEL_PATTERNS.some((pattern) => pattern.test(normalized))
}
