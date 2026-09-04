const DEFAULT_TALLY_FORM_ID = '1A6gM4'
const TALLY_FORM_IDS: Record<string, string> = {
  zh: 'BzRylN',
  ja: 'vG9Z1Q',
}
const TALLY_EMBED_OPTIONS =
  'alignLeft=1&hideTitle=1&transparentBackground=1&dynamicHeight=1'

function normalizeLanguage(language?: string): string {
  return language?.trim().toLowerCase().replace(/_/g, '-') ?? ''
}

export function getTallyFormId(language?: string): string {
  const normalized = normalizeLanguage(language)
  if (normalized === 'zh' || normalized.startsWith('zh-')) {
    return TALLY_FORM_IDS.zh
  }
  if (normalized === 'ja' || normalized.startsWith('ja-')) {
    return TALLY_FORM_IDS.ja
  }
  return DEFAULT_TALLY_FORM_ID
}

export function getTallyFormUrl(language?: string): string {
  return `https://tally.so/r/${getTallyFormId(language)}`
}

export function getTallyEmbedUrl(
  language?: string,
  originPage?: string
): string {
  const originPageQuery = originPage
    ? `&originPage=${encodeURIComponent(originPage)}`
    : ''
  return `https://tally.so/embed/${getTallyFormId(language)}?${TALLY_EMBED_OPTIONS}${originPageQuery}`
}
