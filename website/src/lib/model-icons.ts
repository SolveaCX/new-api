// Verified against @lobehub/icons-static-svg 1.95.0. Keep the CDN version and
// aliases shared by pricing, rankings, and model cards.
const VERIFIED_ICON_NAMES = new Set([
  "openai",
  "anthropic",
  "claude-color",
  "google-color",
  "gemini-color",
  "deepseek-color",
  "qwen-color",
  "alibabacloud-color",
  "mistral-color",
  "xai",
  "grok",
  "meta-color",
  "moonshot",
  "kimi-color",
  "bytedance-color",
  "minimax-color",
  "kuaishou-color",
  "venice",
  "together-color",
  "vertexai-color",
  "vercel",
  "opencode",
  "zai",
  "zhipu-color",
  "vidu",
  "spark-color",
  "hunyuan-color",
  "kling-color",
  "cohere-color",
  "wenxin-color",
  "doubao-color",
  "jimeng-color",
  "ollama",
  "yi-color",
]);

export function getLobeStaticSvgUrl(iconKey?: string): string | null {
  if (!iconKey) return null;
  const directKey = normalizeIconKey(iconKey);
  if (directKey) return `https://cdn.jsdelivr.net/npm/@lobehub/icons-static-svg@1.95.0/icons/${directKey}.svg`;
  return null;
}

export function normalizeIconKey(iconKey: string): string | null {
  const known: Record<string, string> = {
    openai: "openai",
    "open-ai": "openai",
    anthropic: "anthropic",
    claude: "claude-color",
    google: "google-color",
    gemini: "gemini-color",
    deepseek: "deepseek-color",
    "deep-seek": "deepseek-color",
    "deep-seek-color": "deepseek-color",
    "venice-ai": "venice",
    "together-ai-color": "together-color",
    "together-ai": "together-color",
    "moonshot-color": "moonshot",
    "vertex-ai-color": "vertexai-color",
    "vertex-ai": "vertexai-color",
    "byte-dance": "bytedance-color",
    "byte-dance-color": "bytedance-color",
    bytedance: "bytedance-color",
    seedance: "bytedance-color",
    "vercel-color": "vercel",
    "open-code": "opencode",
    minimax: "minimax-color",
    kuaishou: "kuaishou-color",
    qwen: "qwen-color",
    alibaba: "alibabacloud-color",
    "alibaba-cloud": "alibabacloud-color",
    mistral: "mistral-color",
    xai: "xai",
    grok: "grok",
    meta: "meta-color",
    llama: "meta-color",
    moonshot: "moonshot",
    kimi: "kimi-color",
  };
  const normalized = iconKey
    .split(".")
    .filter((segment) => segment && !segment.includes("="))
    .join("-")
    .replace(/([a-z0-9])([A-Z])/g, "$1-$2")
    .toLowerCase()
    .replace(/[^a-z0-9-]/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "");

  if (!normalized) return null;
  // Unknown catalog metadata (for example "ai") is not a CDN filename.
  const resolved = known[normalized] ?? normalized;
  return VERIFIED_ICON_NAMES.has(resolved) ? resolved : null;
}

export function getLocalLogoUrl(iconKey?: string): string | null {
  if (!iconKey) return null;
  const normalized = normalizeIconKey(iconKey);
  if (!normalized) return null;
  const localByIcon: Record<string, string> = {
    venice: "venice",
    "together-color": "together",
    "vertexai-color": "vertexai",
    vercel: "vercel",
    opencode: "opencode",
    zai: "zai",
    "zhipu-color": "zai",
    ollama: "ollama",
    openai: "openai",
    anthropic: "claude",
    "claude-color": "claude",
    "google-color": "googlegemini",
    "gemini-color": "googlegemini",
    "deepseek-color": "deepseek",
    qwen: "qwen",
    "qwen-color": "qwen",
    alibabacloud: "alibabacloud",
    "alibabacloud-color": "alibabacloud",
    "mistral-color": "mistralai",
    xai: "xai",
    grok: "xai",
    "meta-color": "meta",
    moonshot: "moonshotai",
    "kimi-color": "moonshotai",
    "bytedance-color": "bytedance",
    minimax: "minimax",
    "minimax-color": "minimax",
    kuaishou: "kuaishou",
    "kuaishou-color": "kuaishou",
  };
  const localName = localByIcon[normalized];
  return localName ? `/assets/logos/${localName}.svg` : null;
}
