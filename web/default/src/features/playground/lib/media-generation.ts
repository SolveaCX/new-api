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
import type { PlaygroundAttachment } from '../types'
import { isSafeAttachmentURL } from './message-utils'

export type PlaygroundModelKind =
  | 'chat'
  | 'image'
  | 'video'
  | 'audio'
  | 'unsupported'

export type MediaGenerationFamily =
  | 'gpt-image'
  | 'gemini-image-pro'
  | 'gemini-image-flash'
  | 'grok-image'
  | 'grok-video'
  | 'veo-3.0'
  | 'veo-3.1'
  | 'seedance-2.0'
  | 'seedance-2.5'
  | 'sonilo-video-to-music'
  | 'tts'

export type MediaParameterKey =
  | 'count'
  | 'size'
  | 'quality'
  | 'outputFormat'
  | 'background'
  | 'compression'
  | 'resolution'
  | 'aspectRatio'
  | 'duration'
  | 'responseFormat'
  | 'voice'
  | 'speed'
  | 'generateAudio'
  | 'seed'
  | 'preserveSpeech'
  | 'ducking'

export type MediaParameterValue = string | number | boolean

export type MediaGenerationSettings = Partial<
  Record<MediaParameterKey, MediaParameterValue>
>

export interface MediaParameterOption {
  value: string
  labelKey?: string
}

interface MediaParameterFieldBase {
  key: MediaParameterKey
  labelKey: string
}

export interface MediaSelectParameterField extends MediaParameterFieldBase {
  control: 'select'
  options: MediaParameterOption[]
}

export interface MediaNumberParameterField extends MediaParameterFieldBase {
  control: 'number'
  min: number
  max: number
  step?: number
  unitKey?: string
  visibleWhen?: {
    key: MediaParameterKey
    values: MediaParameterValue[]
  }
}

export interface MediaSwitchParameterField extends MediaParameterFieldBase {
  control: 'switch'
}

export type MediaParameterField =
  | MediaSelectParameterField
  | MediaNumberParameterField
  | MediaSwitchParameterField

export interface MediaGenerationProfile {
  kind: 'image' | 'video' | 'audio'
  family: MediaGenerationFamily
  inputKind?: 'image' | 'video'
  requiresAttachment?: boolean
  fields: MediaParameterField[]
  defaults: MediaGenerationSettings
  noteKey?: string
}

export type MediaGenerationRequest =
  | {
      kind: 'image'
      endpoint:
        | '/pg/chat/completions'
        | '/pg/images/generations'
        | '/pg/images/edits'
      payload: Record<string, unknown>
    }
  | {
      kind: 'audio'
      endpoint: '/pg/audio/speech'
      payload: Record<string, unknown>
    }
  | {
      kind: 'video'
      endpoint: '/pg/videos'
      payload: Record<string, unknown>
    }
  | {
      kind: 'video-to-music'
      endpoint: '/pg/video-to-music'
      payload: FormData
    }

export function validateMediaGenerationAttachments(
  model: unknown,
  attachments: PlaygroundAttachment[]
): string | undefined {
  const profile = resolveMediaGenerationProfile(model)
  if (attachments.length === 0) {
    return profile?.requiresAttachment
      ? 'Upload one video to use this model'
      : undefined
  }
  // Ordinary chat models already accept multimodal attachments through the
  // chat-completions path. Video is different: a chat model may accept the
  // request while still being unable to inspect video frames. Reject it
  // before upload/dispatch unless the model is known to expose video input.
  if (!profile) {
    if (
      attachments.some((attachment) => attachment.kind === 'audio') &&
      !supportsPlaygroundAudioInput(model)
    ) {
      return 'This chat model does not support audio attachments'
    }
    if (
      attachments.some((attachment) => attachment.kind === 'video') &&
      !supportsPlaygroundVideoInput(model)
    ) {
      return 'This chat model does not support video attachments'
    }
    return undefined
  }
  if (profile.kind === 'image') {
    if (profile.family === 'gpt-image') {
      return attachments.some((attachment) => attachment.kind !== 'image')
        ? 'Image editing accepts image references only'
        : undefined
    }
    return 'This image model does not support attachments in Playground'
  }
  if (profile.family === 'sonilo-video-to-music') {
    return attachments.length === 1 && attachments[0]?.kind === 'video'
      ? undefined
      : 'Upload one video to use this model'
  }
  if (profile.kind === 'audio') {
    return 'Text-to-speech models do not support attachments in Playground'
  }
  if (profile.kind !== 'video') {
    return 'Attachments are supported only for chat models'
  }
  if (
    attachments.some(
      (attachment) => attachment.kind !== 'image' && attachment.kind !== 'video'
    )
  ) {
    return 'Video generation accepts image or video references only'
  }
  if (
    (profile.family === 'veo-3.0' || profile.family === 'veo-3.1') &&
    attachments.some((attachment) => attachment.kind === 'video')
  ) {
    return 'Veo supports image-to-video, not video-to-video'
  }
  if (
    profile.family === 'grok-video' &&
    attachments.some((attachment) => attachment.kind === 'video')
  ) {
    return 'Grok video editing is not available in Playground yet'
  }
  return undefined
}

/**
 * Return whether an ordinary chat model is known to consume video input.
 *
 * The Playground model endpoint currently returns names only, so this is a
 * deliberately conservative capability gate. Gemini chat models are the
 * only family we have verified end-to-end in this UI; dedicated video
 * generation profiles are handled separately by resolveMediaGenerationProfile.
 */
export function supportsPlaygroundVideoInput(model: unknown): boolean {
  const normalized = normalizeModelName(model)
  if (!normalized || !normalized.includes('gemini')) return false

  // Image/audio/speech specialisations share the Gemini name but do not
  // provide the video-understanding chat contract used by this input.
  return !/(?:image|tts|embedding|embed|audio|speech|transcrib)/.test(
    normalized
  )
}

/**
 * Return whether an ordinary chat model is known to consume audio input.
 *
 * The Playground model endpoint currently returns names only. Keep this gate
 * conservative and limited to model families whose upstream contracts expose
 * audio content parts: Gemini multimodal chat models and the dedicated GPT
 * Audio preview aliases. TTS/realtime/image specialisations are intentionally
 * excluded because they use different endpoints or output-only contracts.
 */
export function supportsPlaygroundAudioInput(model: unknown): boolean {
  const normalized = normalizeModelName(model)
  if (!normalized) return false

  if (
    /(^|\/)gpt-(?:4o(?:-mini)?-audio-preview|audio(?:-mini)?)(?:$|[-_/])/.test(
      normalized
    )
  ) {
    return true
  }

  if (!normalized.includes('gemini')) return false
  if (
    /(?:image|tts|embedding|embed|speech|transcrib|robotics)/.test(normalized)
  ) {
    return false
  }
  return /(?:flash|pro)/.test(normalized)
}

const imageRatios = ['1:1', '3:2', '2:3', '4:3', '3:4', '16:9', '9:16', '21:9']

const seedance20Ratios = [
  'adaptive',
  '16:9',
  '4:3',
  '1:1',
  '3:4',
  '9:16',
  '21:9',
]

const seedance25Ratios = ['adaptive', '16:9', '4:3', '1:1', '3:4', '9:16']

const GPT_IMAGE_SIZE = '1024x1024'

function selectField(
  key: MediaParameterKey,
  labelKey: string,
  values: Array<string | MediaParameterOption>
): MediaSelectParameterField {
  return {
    key,
    labelKey,
    control: 'select',
    options: values.map((value) => {
      if (typeof value === 'string') return { value }
      return value
    }),
  }
}

function getPlayableAttachmentURL(
  attachment: PlaygroundAttachment
): string | undefined {
  if (attachment.kind !== 'image' && attachment.kind !== 'video') {
    return undefined
  }
  return isSafeAttachmentURL(attachment.url, attachment.kind, attachment)
    ? attachment.url.trim()
    : undefined
}

const gptImageProfile: MediaGenerationProfile = {
  kind: 'image',
  family: 'gpt-image',
  defaults: {
    count: 1,
    size: GPT_IMAGE_SIZE,
    quality: 'auto',
    outputFormat: 'png',
    background: 'auto',
    compression: 90,
  },
  fields: [
    selectField('quality', 'Quality', [
      { value: 'auto', labelKey: 'Auto' },
      { value: 'low', labelKey: 'Low' },
      { value: 'medium', labelKey: 'Medium' },
      { value: 'high', labelKey: 'High' },
    ]),
    selectField('outputFormat', 'Output format', ['png', 'jpeg']),
    selectField('background', 'Background', [
      { value: 'auto', labelKey: 'Auto' },
      { value: 'opaque', labelKey: 'Opaque' },
    ]),
    {
      key: 'compression',
      labelKey: 'Compression',
      control: 'number',
      min: 0,
      max: 100,
      step: 1,
      unitKey: '%',
      visibleWhen: {
        key: 'outputFormat',
        values: ['jpeg'],
      },
    },
  ],
}

const geminiImageProProfile: MediaGenerationProfile = {
  kind: 'image',
  family: 'gemini-image-pro',
  defaults: {
    resolution: '2K',
    aspectRatio: '1:1',
  },
  fields: [
    selectField('resolution', 'Resolution', ['1K', '2K', '4K']),
    selectField('aspectRatio', 'Aspect ratio', imageRatios),
  ],
}

const geminiImageFlashProfile: MediaGenerationProfile = {
  kind: 'image',
  family: 'gemini-image-flash',
  defaults: {
    resolution: '1K',
    aspectRatio: '1:1',
  },
  fields: [
    selectField('resolution', 'Resolution', ['1K']),
    selectField('aspectRatio', 'Aspect ratio', imageRatios),
  ],
}

const grokImageProfile: MediaGenerationProfile = {
  kind: 'image',
  family: 'grok-image',
  defaults: {
    count: 1,
    responseFormat: 'url',
  },
  fields: [
    {
      key: 'count',
      labelKey: 'Image count',
      control: 'number',
      min: 1,
      max: 10,
      step: 1,
    },
    selectField('responseFormat', 'Response format', [
      { value: 'url', labelKey: 'URL' },
      { value: 'b64_json', labelKey: 'Base64' },
    ]),
  ],
}

function createGrokVideoProfile(resolutions: string[]): MediaGenerationProfile {
  return {
    kind: 'video',
    family: 'grok-video',
    defaults: {
      resolution: '720p',
      duration: 5,
      aspectRatio: '16:9',
    },
    fields: [
      selectField('resolution', 'Resolution', resolutions),
      {
        key: 'duration',
        labelKey: 'Duration',
        control: 'number',
        min: 1,
        max: 15,
        step: 1,
        unitKey: 'seconds',
      },
      selectField('aspectRatio', 'Aspect ratio', [
        '1:1',
        '16:9',
        '9:16',
        '4:3',
        '3:4',
        '3:2',
        '2:3',
      ]),
    ],
  }
}

const grokVideoProfile = createGrokVideoProfile(['480p', '720p'])
const grokVideo15Profile = createGrokVideoProfile(['480p', '720p', '1080p'])

const veoProfile: MediaGenerationProfile = {
  kind: 'video',
  family: 'veo-3.1',
  defaults: {
    resolution: '720p',
    duration: 8,
    aspectRatio: '16:9',
  },
  fields: [
    selectField('resolution', 'Resolution', ['720p', '1080p', '4k']),
    selectField('duration', 'Duration', [
      { value: '4', labelKey: '4 seconds' },
      { value: '6', labelKey: '6 seconds' },
      { value: '8', labelKey: '8 seconds' },
    ]),
    selectField('aspectRatio', 'Aspect ratio', ['16:9', '9:16']),
  ],
  noteKey: 'Veo 1080p and 4K output requires an 8-second duration.',
}

const veo30Profile: MediaGenerationProfile = {
  kind: 'video',
  family: 'veo-3.0',
  defaults: {
    resolution: '720p',
    duration: 8,
    aspectRatio: '16:9',
  },
  fields: [
    selectField('resolution', 'Resolution', ['720p']),
    selectField('duration', 'Duration', [
      { value: '4', labelKey: '4 seconds' },
      { value: '6', labelKey: '6 seconds' },
      { value: '8', labelKey: '8 seconds' },
    ]),
    selectField('aspectRatio', 'Aspect ratio', ['16:9', '9:16']),
  ],
}

function createSeedance20Profile(
  resolutions: string[]
): MediaGenerationProfile {
  return {
    kind: 'video',
    family: 'seedance-2.0',
    defaults: {
      resolution: '720p',
      duration: 5,
      aspectRatio: 'adaptive',
      generateAudio: false,
    },
    fields: [
      selectField('resolution', 'Resolution', resolutions),
      selectField('aspectRatio', 'Aspect ratio', seedance20Ratios),
      {
        key: 'duration',
        labelKey: 'Duration',
        control: 'number',
        min: 4,
        max: 15,
        step: 1,
        unitKey: 'seconds',
      },
      {
        key: 'generateAudio',
        labelKey: 'Generate audio',
        control: 'switch',
      },
    ],
  }
}

const seedance20FullProfile = createSeedance20Profile([
  '480p',
  '720p',
  '1080p',
  '4k',
])

const seedance20EconomyProfile = createSeedance20Profile(['480p', '720p'])

const seedance25Profile: MediaGenerationProfile = {
  kind: 'video',
  family: 'seedance-2.5',
  defaults: {
    resolution: '720p',
    duration: 5,
    aspectRatio: 'adaptive',
    generateAudio: false,
  },
  fields: [
    selectField('resolution', 'Resolution', ['480p', '720p']),
    selectField('aspectRatio', 'Aspect ratio', seedance25Ratios),
    {
      key: 'duration',
      labelKey: 'Duration',
      control: 'number',
      min: 4,
      max: 30,
      step: 1,
      unitKey: 'seconds',
    },
    {
      key: 'generateAudio',
      labelKey: 'Generate audio',
      control: 'switch',
    },
  ],
}

const ttsVoiceValues = [
  'alloy',
  'ash',
  'coral',
  'echo',
  'fable',
  'nova',
  'onyx',
  'sage',
  'shimmer',
  'Kore',
  'Puck',
  'Aoede',
]

const geminiTTSVoiceValues = [
  'Zephyr',
  'Puck',
  'Charon',
  'Kore',
  'Fenrir',
  'Leda',
  'Orus',
  'Aoede',
  'Callirrhoe',
  'Autonoe',
  'Enceladus',
  'Iapetus',
  'Umbriel',
  'Algieba',
  'Despina',
  'Erinome',
  'Algenib',
  'Rasalgethi',
  'Laomedeia',
  'Achernar',
  'Alnilam',
  'Schedar',
  'Gacrux',
  'Pulcherrima',
  'Achird',
  'Zubenelgenubi',
  'Vindemiatrix',
  'Sadachbia',
  'Sadaltager',
  'Sulafat',
]

function ttsVoiceField(): MediaSelectParameterField {
  return selectField('voice', 'Voice', ttsVoiceValues)
}

function geminiTTSVoiceField(): MediaSelectParameterField {
  return selectField('voice', 'Voice', geminiTTSVoiceValues)
}

const ttsOutputFields: MediaParameterField[] = [
  selectField('responseFormat', 'Response format', [
    'mp3',
    'wav',
    'opus',
    'aac',
    'flac',
    'pcm',
  ]),
  {
    key: 'speed',
    labelKey: 'Speed',
    control: 'number',
    min: 0.25,
    max: 4,
    step: 0.05,
  },
]

const soniloVideoToMusicProfile: MediaGenerationProfile = {
  kind: 'audio',
  family: 'sonilo-video-to-music',
  inputKind: 'video',
  requiresAttachment: true,
  defaults: {
    duration: 10,
    outputFormat: 'mp3',
    preserveSpeech: false,
    ducking: false,
  },
  fields: [
    {
      key: 'duration',
      labelKey: 'Video duration',
      control: 'number',
      min: 1,
      max: 3600,
      step: 0.1,
      unitKey: 'seconds',
    },
    selectField('outputFormat', 'Output format', ['mp3', 'm4a', 'wav']),
    {
      key: 'preserveSpeech',
      labelKey: 'Preserve speech',
      control: 'switch',
    },
    {
      key: 'ducking',
      labelKey: 'Ducking',
      control: 'switch',
    },
  ],
}

const ttsProfile: MediaGenerationProfile = {
  kind: 'audio',
  family: 'tts',
  defaults: {
    voice: 'alloy',
    responseFormat: 'mp3',
    speed: 1,
  },
  fields: [ttsVoiceField(), ...ttsOutputFields],
}

// Gemini's legacy generateContent TTS contract accepts a voice and returns
// raw PCM; it has no OpenAI-style response_format or numeric speed controls.
// Keep those controls out of the UI rather than silently pretending they are
// applied by the adapter.
const geminiTTSProfile: MediaGenerationProfile = {
  kind: 'audio',
  family: 'tts',
  defaults: { voice: 'Kore' },
  fields: [geminiTTSVoiceField()],
}

const unsupportedPatterns = [
  /(^|[-_/])(?:dall[ -]?e|imagen|flux|stable-diffusion|sdxl|midjourney|jimeng|qwen-image|z-image)(?:$|[-_/])/,
  /(^|\/)nano-banana(?:$|[-_/])/,
  /(^|\/)minimax-h3(?:$|[-_/])/,
  /(^|[-_/])(?:image|video|seedance|sora|kling|veo|wan|hailuo|runway|pika|luma)(?:$|[-_/])/,
  /(^|[-_/])(?:whisper|transcribe|audio-preview|audio|tts|speech|cosyvoice|fish(?:-speech)?)(?:$|[-_/])/,
  /(^|[-_/])(?:embedding|embeddings|rerank|reranker|moderation|suno|music|lyrics)(?:$|[-_/])/,
  /(^|[-_/])(?:eleven(?:labs)?|sonilo)(?:$|[-_/])|^eleven[_-]/,
  /^mj_/,
]

const unsupportedAudioModelPattern =
  /(?:^|[-_/])(?:eleven(?:labs)?)(?:$|[-_/])|^eleven[_-]/i

const supportedTTSModelPattern =
  /(?:^|\/)(?:gemini-[^/]*-tts[^/]*|gpt-4o-mini-tts[^/]*|tts-1(?:-[^/]*)?|speech-(?:2\.5-(?:hd|turbo)-preview|(?:01|02)-(?:hd|turbo)))(?:$|\/)/i

function isPlaygroundTTSModel(model: string): boolean {
  if (!model || unsupportedAudioModelPattern.test(model)) return false
  return supportedTTSModelPattern.test(model)
}

function normalizeModelName(model: unknown): string {
  return typeof model === 'string' ? model.trim().toLowerCase() : ''
}

function cloneProfile(profile: MediaGenerationProfile): MediaGenerationProfile {
  return {
    ...profile,
    defaults: { ...profile.defaults },
    fields: profile.fields.map((field) => {
      if (field.control === 'select') {
        return {
          ...field,
          options: field.options.map((option) => ({ ...option })),
        }
      }
      if (field.control === 'number' && field.visibleWhen) {
        return {
          ...field,
          visibleWhen: {
            ...field.visibleWhen,
            values: [...field.visibleWhen.values],
          },
        }
      }
      return { ...field }
    }),
  }
}

export function resolveMediaGenerationProfile(
  model: unknown
): MediaGenerationProfile | undefined {
  const normalized = normalizeModelName(model)
  if (!normalized) return undefined

  if (normalized === 'sonilo-video-to-music') {
    return cloneProfile(soniloVideoToMusicProfile)
  }

  if (
    /(^|\/)gpt-image-2(?:$|[-_\/])/.test(normalized) ||
    /(^|\/)gpt-image-2\.5-(?:flare|sunburst)(?:$|[-_\/])/.test(normalized)
  ) {
    return cloneProfile(gptImageProfile)
  }
  if (normalized.includes('grok-imagine-image')) {
    return cloneProfile(grokImageProfile)
  }
  if (/(^|\/)grok-imagine-video-1\.5$/.test(normalized)) {
    return cloneProfile(grokVideo15Profile)
  }
  if (/(^|\/)grok-imagine-video$/.test(normalized)) {
    return cloneProfile(grokVideoProfile)
  }
  if (/(^|\/)gemini-3\.1-flash-image-preview(?:$|[-_/])/.test(normalized)) {
    return cloneProfile(geminiImageFlashProfile)
  }
  if (
    normalized.includes('gemini-3-pro-image') ||
    /(^|\/)nano-banana-pro-preview(?:$|[-_/])/.test(normalized)
  ) {
    return cloneProfile(geminiImageProProfile)
  }
  if (/(^|\/)veo-3(?:\.|-)1(?:$|[-_/])/.test(normalized)) {
    return cloneProfile(veoProfile)
  }
  if (/(^|\/)veo-3(?:\.|-)0(?:$|[-_/])/.test(normalized)) {
    return cloneProfile(veo30Profile)
  }
  if (normalized.includes('seedance')) {
    if (/2(?:[.-]|-)?5/.test(normalized)) {
      return cloneProfile(seedance25Profile)
    }
    if (/2(?:[.-]|-)?0/.test(normalized)) {
      const isEconomyVariant = /(?:^|[-_/])(?:fast|mini)(?:$|[-_/])/.test(
        normalized
      )
      return cloneProfile(
        isEconomyVariant ? seedance20EconomyProfile : seedance20FullProfile
      )
    }
  }
  if (isPlaygroundTTSModel(normalized)) {
    return cloneProfile(
      normalized.includes('gemini') ? geminiTTSProfile : ttsProfile
    )
  }
  return undefined
}

export function resolvePlaygroundModelKind(
  model: unknown
): PlaygroundModelKind {
  const normalized = normalizeModelName(model)
  if (!normalized) return 'unsupported'

  const mediaProfile = resolveMediaGenerationProfile(normalized)
  if (mediaProfile) return mediaProfile.kind

  // Dedicated audio-understanding aliases are chat-completions models even
  // though their names contain the generic `audio` marker used by the task
  // model deny-list below.
  if (supportsPlaygroundAudioInput(normalized)) return 'chat'

  if (unsupportedPatterns.some((pattern) => pattern.test(normalized))) {
    return 'unsupported'
  }
  return 'chat'
}

function normalizedNumber(
  value: MediaParameterValue | undefined,
  fallback: number,
  min: number,
  max: number,
  step?: number
): number {
  const parsed = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(parsed)) return fallback
  const clamped = Math.min(max, Math.max(min, parsed))
  if (!step || step <= 0) return clamped
  const snapped = min + Math.round((clamped - min) / step) * step
  return Math.min(max, Math.max(min, snapped))
}

export function normalizeMediaGenerationSettings(
  profile: MediaGenerationProfile,
  settings: MediaGenerationSettings
): MediaGenerationSettings {
  const normalized: MediaGenerationSettings = {
    ...profile.defaults,
    ...settings,
  }

  profile.fields.forEach((field) => {
    if (field.control === 'number') {
      const fallback = Number(profile.defaults[field.key] ?? field.min)
      normalized[field.key] = normalizedNumber(
        normalized[field.key],
        fallback,
        field.min,
        field.max,
        field.step
      )
      return
    }
    if (field.control === 'select') {
      const value = String(normalized[field.key] ?? '')
      const supported = field.options.some((option) => option.value === value)
      if (!supported) normalized[field.key] = profile.defaults[field.key]
      return
    }
    if (field.control === 'switch') {
      if (typeof normalized[field.key] !== 'boolean') {
        const fallback = profile.defaults[field.key]
        normalized[field.key] = typeof fallback === 'boolean' ? fallback : false
      }
    }
  })

  if (profile.family === 'gpt-image') {
    normalized.count = 1
    normalized.size = GPT_IMAGE_SIZE
  }

  if (
    profile.family === 'veo-3.1' &&
    (normalized.resolution === '1080p' || normalized.resolution === '4k')
  ) {
    normalized.duration = 8
  }
  return normalized
}

function buildGptImagePayload(
  prompt: string,
  model: string,
  group: string,
  settings: MediaGenerationSettings,
  attachments: PlaygroundAttachment[] = []
): Record<string, unknown> {
  const outputFormat = settings.outputFormat === 'jpeg' ? 'jpeg' : 'png'
  const background = settings.background === 'opaque' ? 'opaque' : 'auto'
  const payload: Record<string, unknown> = {
    model,
    group,
    prompt,
    n: 1,
    size: GPT_IMAGE_SIZE,
    quality: settings.quality,
    response_format: 'b64_json',
    output_format: outputFormat,
    background,
  }
  if (outputFormat === 'jpeg') {
    payload.output_compression = settings.compression
  }
  const images = attachments
    .filter(
      (attachment) =>
        attachment.kind === 'image' && !!getPlayableAttachmentURL(attachment)
    )
    .map((attachment) => getPlayableAttachmentURL(attachment)!)
  if (images.length > 0) payload.images = images
  return payload
}

function buildGeminiImagePayload(
  prompt: string,
  model: string,
  group: string,
  settings: MediaGenerationSettings
): Record<string, unknown> {
  return {
    model,
    group,
    messages: [{ role: 'user', content: prompt }],
    stream: false,
    extra_body: {
      google: {
        image_config: {
          image_size: settings.resolution,
          aspect_ratio: settings.aspectRatio,
        },
      },
    },
  }
}

function buildVideoPayload(
  prompt: string,
  model: string,
  group: string,
  family: MediaGenerationFamily,
  settings: MediaGenerationSettings,
  attachments: PlaygroundAttachment[] = []
): Record<string, unknown> {
  const media = attachments.filter(
    (attachment) =>
      (attachment.kind === 'image' || attachment.kind === 'video') &&
      !!getPlayableAttachmentURL(attachment)
  )

  if (family === 'seedance-2.0' || family === 'seedance-2.5') {
    const imageCount = media.filter(
      (attachment) => attachment.kind === 'image'
    ).length
    const content = [
      ...(prompt ? [{ type: 'text', text: prompt }] : []),
      ...media.map((attachment) =>
        attachment.kind === 'image'
          ? {
              type: 'image_url',
              image_url: { url: getPlayableAttachmentURL(attachment)! },
              ...(imageCount > 1 ? { role: 'reference_image' } : {}),
            }
          : {
              type: 'video_url',
              video_url: { url: getPlayableAttachmentURL(attachment)! },
              role: 'reference_video',
            }
      ),
    ]
    return {
      model,
      group,
      prompt,
      content,
      resolution: settings.resolution,
      ratio: settings.aspectRatio,
      duration: settings.duration,
      generate_audio: settings.generateAudio,
    }
  }

  if (family === 'veo-3.1' || family === 'veo-3.0') {
    return {
      model,
      group,
      prompt,
      ...(media[0]?.kind === 'image' ? { images: [media[0].url!.trim()] } : {}),
      duration: settings.duration,
      metadata: {
        resolution: settings.resolution,
        aspectRatio: settings.aspectRatio,
      },
    }
  }

  if (family === 'grok-video') {
    return {
      model,
      group,
      prompt,
      ...(media[0]?.kind === 'image' ? { images: [media[0].url!.trim()] } : {}),
      resolution: settings.resolution,
      aspect_ratio: settings.aspectRatio,
      duration: settings.duration,
    }
  }

  return {
    model,
    group,
    prompt,
    resolution: settings.resolution,
    ratio: settings.aspectRatio,
    duration: settings.duration,
  }
}

function buildSoniloVideoToMusicFormData(
  prompt: string,
  model: string,
  group: string,
  settings: MediaGenerationSettings,
  profile: MediaGenerationProfile,
  attachments: PlaygroundAttachment[]
): FormData | undefined {
  const video =
    attachments.length === 1 && attachments[0]?.kind === 'video'
      ? attachments[0]
      : undefined
  const videoURL = video ? getPlayableAttachmentURL(video) : undefined
  if (!video || !videoURL) return undefined

  const normalizedSettings = normalizeMediaGenerationSettings(profile, settings)
  const metadataDuration = video.durationSeconds
  const configuredDuration = settings.duration
  const duration =
    metadataDuration === undefined
      ? configuredDuration === undefined
        ? Number(normalizedSettings.duration)
        : Number(configuredDuration)
      : Number(metadataDuration)
  if (!Number.isFinite(duration) || duration < 1 || duration > 3600) {
    return undefined
  }

  const outputFormat = ['mp3', 'm4a', 'wav'].includes(
    String(normalizedSettings.outputFormat)
  )
    ? String(normalizedSettings.outputFormat)
    : 'mp3'
  const formData = new FormData()
  // The backend deliberately accepts only this exact model name. The model
  // selector normally supplies it verbatim, but canonicalizing here keeps a
  // handoff with surrounding whitespace/case from becoming an upstream 400.
  formData.set('model', normalizeModelName(model))
  formData.set('group', group.trim())
  formData.set('video_url', videoURL)
  const trimmedPrompt = prompt.trim()
  if (trimmedPrompt) formData.set('prompt', trimmedPrompt)
  formData.set('duration_seconds', String(Math.round(duration * 10) / 10))
  formData.set('output_format', outputFormat)
  formData.set('mode', 'async')
  formData.set(
    'preserve_speech',
    String(normalizedSettings.preserveSpeech === true)
  )
  formData.set('ducking', String(normalizedSettings.ducking === true))
  formData.set('variants_num', '1')
  return formData
}

export function buildMediaGenerationRequest(
  prompt: string,
  model: string,
  group: string,
  settings: MediaGenerationSettings,
  attachments: PlaygroundAttachment[] = []
): MediaGenerationRequest | undefined {
  const profile = resolveMediaGenerationProfile(model)
  if (!profile) return undefined

  // The Playground state normalizes settings when the user edits them. Keep
  // request construction serialization-only so the submitted values always
  // match the values visible in the parameter panel.

  if (profile.family === 'sonilo-video-to-music') {
    const payload = buildSoniloVideoToMusicFormData(
      prompt,
      model,
      group,
      settings,
      profile,
      attachments
    )
    if (!payload) return undefined
    return {
      kind: 'video-to-music',
      endpoint: '/pg/video-to-music',
      payload,
    }
  }

  if (profile.family === 'tts') {
    const voice = String(settings.voice ?? profile.defaults.voice ?? 'alloy')
    const normalizedModel = normalizeModelName(model)
    const payload: Record<string, unknown> = {
      model,
      group,
      input: prompt,
      voice,
    }
    if (!normalizedModel.includes('gemini')) {
      const responseFormat = String(
        settings.responseFormat ?? profile.defaults.responseFormat ?? 'mp3'
      )
      const speed = Number(settings.speed ?? profile.defaults.speed ?? 1)
      payload.response_format = responseFormat
      if (Number.isFinite(speed)) payload.speed = speed
    }
    return {
      kind: 'audio',
      endpoint: '/pg/audio/speech',
      payload,
    }
  }

  if (profile.family === 'gpt-image') {
    const hasImageAttachments = attachments.some(
      (attachment) =>
        attachment.kind === 'image' && !!getPlayableAttachmentURL(attachment)
    )
    return {
      kind: 'image',
      endpoint: hasImageAttachments
        ? '/pg/images/edits'
        : '/pg/images/generations',
      payload: buildGptImagePayload(
        prompt,
        model,
        group,
        settings,
        attachments
      ),
    }
  }
  if (profile.family === 'grok-image') {
    return {
      kind: 'image',
      endpoint: '/pg/images/generations',
      payload: {
        model,
        group,
        prompt,
        n: settings.count,
        response_format: settings.responseFormat,
      },
    }
  }
  if (
    profile.family === 'gemini-image-pro' ||
    profile.family === 'gemini-image-flash'
  ) {
    return {
      kind: 'image',
      endpoint: '/pg/chat/completions',
      payload: buildGeminiImagePayload(prompt, model, group, settings),
    }
  }
  return {
    kind: 'video',
    endpoint: '/pg/videos',
    payload: buildVideoPayload(
      prompt,
      model,
      group,
      profile.family,
      settings,
      attachments
    ),
  }
}
