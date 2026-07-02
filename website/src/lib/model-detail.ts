import { localizePath, type Locale } from "./locales";
import { isTokenBasedModel, type PricingModel } from "./pricing";

type ModelDetailMetadataCopy = {
  title: string;
  description: string;
};

const MODEL_DETAIL_METADATA: Record<Locale, { titleSuffix: string; description: (modelName: string) => string }> = {
  en: {
    titleSuffix: "API pricing and availability",
    description: (modelName) => `Live flatkey pricing, endpoint support, availability, and OpenAI-compatible setup for ${modelName}.`,
  },
  zh: {
    titleSuffix: "API 价格与可用性",
    description: (modelName) => `查看 ${modelName} 的 flatkey 实时价格、端点支持、可用性和 OpenAI 兼容接入方式。`,
  },
  es: {
    titleSuffix: "precios API y disponibilidad",
    description: (modelName) => `Precios flatkey en vivo, endpoints, disponibilidad y configuración compatible con OpenAI para ${modelName}.`,
  },
  fr: {
    titleSuffix: "tarifs API et disponibilité",
    description: (modelName) => `Tarifs flatkey en direct, endpoints, disponibilité et configuration compatible OpenAI pour ${modelName}.`,
  },
  pt: {
    titleSuffix: "preço API e disponibilidade",
    description: (modelName) => `Preço flatkey ao vivo, endpoints, disponibilidade e configuração compatível com OpenAI para ${modelName}.`,
  },
  ru: {
    titleSuffix: "цены API и доступность",
    description: (modelName) => `Актуальная цена flatkey, endpoints, доступность и OpenAI-совместимое подключение для ${modelName}.`,
  },
  ja: {
    titleSuffix: "API 料金と可用性",
    description: (modelName) => `${modelName} の flatkey ライブ料金、エンドポイント、可用性、OpenAI 互換の接続方法です。`,
  },
  vi: {
    titleSuffix: "giá API và độ sẵn sàng",
    description: (modelName) => `Giá flatkey trực tiếp, endpoint, độ sẵn sàng và cách cấu hình tương thích OpenAI cho ${modelName}.`,
  },
  de: {
    titleSuffix: "API-Preise und Verfügbarkeit",
    description: (modelName) => `Live-flatkey-Preise, Endpoint-Support, Verfügbarkeit und OpenAI-kompatible Einrichtung für ${modelName}.`,
  },
};

export function modelSlugFromName(modelName: string): string {
  const slug = modelName
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
  return slug || "model";
}

export function getModelDetailPathname(model: Pick<PricingModel, "model_name">): string {
  return `/models/${modelSlugFromName(model.model_name)}`;
}

export function getModelDetailPath(model: Pick<PricingModel, "model_name">, locale: Locale): string {
  return localizePath(getModelDetailPathname(model), locale);
}

export function resolveModelBySlug(models: PricingModel[], slug: string): PricingModel | null {
  const matches = models.filter((model) => modelSlugFromName(model.model_name) === slug);
  return matches.length === 1 ? matches[0] : null;
}

export function getModelSlugCounts(models: Pick<PricingModel, "model_name">[]): Map<string, number> {
  const counts = new Map<string, number>();
  for (const model of models) {
    const slug = modelSlugFromName(model.model_name);
    counts.set(slug, (counts.get(slug) ?? 0) + 1);
  }
  return counts;
}

export function isModelDetailRoutable(model: PricingModel, models: PricingModel[]): boolean {
  const slugCounts = getModelSlugCounts(models);
  if ((slugCounts.get(modelSlugFromName(model.model_name)) ?? 0) !== 1) return false;
  if (!model.model_name.trim()) return false;
  if (!model.vendor_name?.trim()) return false;
  return true;
}

export function isModelDetailIndexable(model: PricingModel, models: PricingModel[]): boolean {
  if (!isModelDetailRoutable(model, models)) return false;
  if (!hasIndexablePrice(model)) return false;
  return Boolean(model.availability_status || (model.supported_endpoint_types ?? []).length > 0);
}

export function getRoutableModelDetailPathnames(models: PricingModel[]): string[] {
  return models
    .filter((model) => isModelDetailRoutable(model, models))
    .map((model) => getModelDetailPathname(model));
}

export function getIndexableModelDetailPathnames(models: PricingModel[]): string[] {
  return models
    .filter((model) => isModelDetailIndexable(model, models))
    .map((model) => getModelDetailPathname(model));
}

export function getIndexableRelatedModels(model: PricingModel, models: PricingModel[]): PricingModel[] {
  return models.filter((item) =>
    item.model_name !== model.model_name &&
    item.vendor_name === model.vendor_name &&
    isModelDetailIndexable(item, models)
  );
}

export function modelDetailMetadataCopy(locale: Locale, model: Pick<PricingModel, "model_name">): ModelDetailMetadataCopy {
  const copy = MODEL_DETAIL_METADATA[locale] ?? MODEL_DETAIL_METADATA.en;
  return {
    title: `${model.model_name} ${copy.titleSuffix}`,
    description: copy.description(model.model_name),
  };
}

export function buildModelQuickStart(model: PricingModel, routerOrigin: string): string {
  const endpoints = model.supported_endpoint_types ?? [];
  if (endpoints.includes("embeddings")) {
    return `from openai import OpenAI

client = OpenAI(
  base_url="${routerOrigin}/v1",
  api_key="sk-flatkey-..."
)

response = client.embeddings.create(
  model="${model.model_name}",
  input="Search query text"
)`;
  }
  if (endpoints.includes("image-generation")) {
    return `from openai import OpenAI

client = OpenAI(
  base_url="${routerOrigin}/v1",
  api_key="sk-flatkey-..."
)

response = client.images.generate(
  model="${model.model_name}",
  prompt="A product photo on a clean studio background"
)`;
  }
  if (endpoints.includes("openai-response")) {
    return `from openai import OpenAI

client = OpenAI(
  base_url="${routerOrigin}/v1",
  api_key="sk-flatkey-..."
)

response = client.responses.create(
  model="${model.model_name}",
  input="Hello"
)`;
  }
  if (endpoints.includes("anthropic")) {
    return `import anthropic

client = anthropic.Anthropic(
  base_url="${routerOrigin}",
  api_key="sk-flatkey-..."
)

message = client.messages.create(
  model="${model.model_name}",
  max_tokens=1024,
  messages=[{"role": "user", "content": "Hello"}]
)`;
  }
  if (endpoints.length > 0 && !endpoints.includes("openai")) {
    return `curl ${routerOrigin}/v1/models \\
  -H "Authorization: Bearer sk-flatkey-..."

# Confirm the supported ${endpoints[0]} request shape in the dashboard before production use.`;
  }
  return `from openai import OpenAI

client = OpenAI(
  base_url="${routerOrigin}/v1",
  api_key="sk-flatkey-..."
)

response = client.chat.completions.create(
  model="${model.model_name}",
  messages=[{"role": "user", "content": "Hello"}]
)`;
}

function hasIndexablePrice(model: PricingModel): boolean {
  if (isTokenBasedModel(model)) {
    return Number.isFinite(model.model_ratio) && model.model_ratio >= 0 && Number.isFinite(model.completion_ratio) && model.completion_ratio >= 0;
  }
  return Number.isFinite(model.model_price) && Number(model.model_price) >= 0;
}
