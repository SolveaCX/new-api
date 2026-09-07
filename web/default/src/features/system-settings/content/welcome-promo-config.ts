import type { Model } from '@/features/models/types'

export const WELCOME_PROMO_SLOT_COUNT = 3

export type WelcomePromoModel = {
  model_name: string
  description: string
  offer: string
}

export const DEFAULT_WELCOME_PROMO: WelcomePromoModel[] = [
  {
    model_name: 'deepseek-v4-pro',
    description: 'DeepSeek reasoning model',
    offer: '55% off',
  },
  {
    model_name: 'glm-5.3-flash',
    description: 'GLM multimodal model',
    offer: '55% off',
  },
  {
    model_name: 'gpt-5.6-sol',
    description: 'GPT frontier model',
    offer: '55% off',
  },
]

export const DEFAULT_WELCOME_PROMO_JSON = JSON.stringify(DEFAULT_WELCOME_PROMO)

export function normalizeWelcomePromo(value: string): WelcomePromoModel[] {
  try {
    const parsed: unknown = JSON.parse(value || '[]')
    if (!Array.isArray(parsed)) return [...DEFAULT_WELCOME_PROMO]
    const normalized = parsed
      .slice(0, WELCOME_PROMO_SLOT_COUNT)
      .map((item): WelcomePromoModel | null => {
        if (!item || typeof item !== 'object') return null
        const record = item as Record<string, unknown>
        const modelName = typeof record.model_name === 'string' ? record.model_name.trim() : ''
        const description = typeof record.description === 'string' ? record.description : ''
        const offer = typeof record.offer === 'string' ? record.offer : ''
        if (!modelName) return null
        return { model_name: modelName, description, offer }
      })
      .filter((item): item is WelcomePromoModel => item !== null)
    return normalized.length > 0 ? normalized : [...DEFAULT_WELCOME_PROMO]
  } catch {
    return [...DEFAULT_WELCOME_PROMO]
  }
}

export function serializeWelcomePromo(models: WelcomePromoModel[]): string {
  return JSON.stringify(
    models.slice(0, WELCOME_PROMO_SLOT_COUNT).map((item) => ({
      model_name: item.model_name.trim(),
      description: item.description.trim(),
      offer: item.offer.trim(),
    })),
  )
}

export function modelLogoPath(modelName: string): string | null {
  const normalized = modelName.toLowerCase()
  const mappings: Array<[RegExp, string]> = [
    [/openai|gpt|dall-e|sora|codex/, '/assets/logos/openai.svg'],
    [/anthropic|claude/, '/assets/logos/claude.svg'],
    [/google|gemini|imagen|veo|gemma/, '/assets/logos/googlegemini.svg'],
    [/deepseek|deep-seek/, '/assets/logos/deepseek.svg'],
    [/kimi|moonshot/, '/assets/logos/moonshotai.svg'],
    [/qwen|alibaba|aliyun|通义/, '/assets/logos/qwen.svg'],
    [/minimax/, '/assets/logos/minimax.svg'],
    [/bytedance|doubao|seedance|seedream/, '/assets/logos/bytedance.svg'],
    [/zhipu|glm|chatglm|z\.ai|z-ai|智谱/, '/assets/logos/zai.svg'],
    [/grok|xai|x\.ai/, '/assets/logos/xai.svg'],
    [/mistral/, '/assets/logos/mistralai.svg'],
    [/meta|llama/, '/assets/logos/meta.svg'],
    [/baidu|ernie|文心/, '/logos/baidu.svg'],
  ]
  return mappings.find(([pattern]) => pattern.test(normalized))?.[1] ?? null
}

export function modelOptions(models: Model[]): string[] {
  return Array.from(new Set(models.map((model) => model.model_name.trim()).filter(Boolean))).sort((a, b) => a.localeCompare(b))
}
