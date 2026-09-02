import type { Locale } from "./locales";

export type ModelPromotion = "free" | "limited" | "hot" | "new";

const PROMOTION_PRIORITY: Record<ModelPromotion, number> = {
  free: 0,
  limited: 1,
  hot: 2,
  new: Number.POSITIVE_INFINITY,
};

// Fable 5.1 leads the launch banner and the default model order.
const PINNED_MODEL_PATTERN = /(^|[/])claude[-_.]?fable[-_.]?5[-_.]?1(?:[-_.]|$)/;

const LABELS: Record<Locale, Record<ModelPromotion, string>> = {
  en: { free: "Free", limited: "Limited discount", hot: "HOT", new: "New release" },
  zh: { free: "免费", limited: "限时折扣", hot: "热门", new: "新发布" },
  es: { free: "Gratis", limited: "Descuento por tiempo limitado", hot: "Popular", new: "Nuevo lanzamiento" },
  fr: { free: "Gratuit", limited: "Remise à durée limitée", hot: "Tendance", new: "Nouvelle sortie" },
  pt: { free: "Grátis", limited: "Desconto por tempo limitado", hot: "Em alta", new: "Novo lançamento" },
  ru: { free: "Бесплатно", limited: "Временная скидка", hot: "Популярно", new: "Новый релиз" },
  ja: { free: "無料", limited: "期間限定割引", hot: "人気", new: "新リリース" },
  vi: { free: "Miễn phí", limited: "Giảm giá có thời hạn", hot: "Nổi bật", new: "Phát hành mới" },
  de: { free: "Kostenlos", limited: "Zeitlich begrenzter Rabatt", hot: "Beliebt", new: "Neu veröffentlicht" },
  id: { free: "Gratis", limited: "Diskon terbatas", hot: "Populer", new: "Rilis baru" },
};

export function getModelPromotions(modelName: string): ModelPromotion[] {
  const name = modelName.toLowerCase();
  const promotions: ModelPromotion[] = [];
  if (/(^|[/])deepseek[-_.]?v4[-_.]?flash$/.test(name)) {
    promotions.push("free");
  }
  if (
    /(^|[/])glm[-_.]?5[-_.]?3[-_.]?flash$/.test(name) ||
    /(^|[/])deepseek[-_.]?v4[-_.]?pro$/.test(name)
  ) {
    promotions.push("limited");
  }
  if (
    /(^|[/_-])seedance[-_.]?2[-_.]?5(?:[-_.]|$)/.test(name) ||
    /(^|[/])gpt[-_.]?5[-_.]?6[-_.]?sol(?:[-_.]|$)/.test(name) ||
    /(^|[/])claude[-_.]?(?:opus[-_.]?(?:4[-_.]?8|5)|sonnet[-_.]?(?:4[-_.]?6|5)|haiku[-_.]?4[-_.]?5(?:[-_.]?20251001)?)(?:[-_.]|$)/.test(name)
  ) promotions.push("hot");
  if (/(^|[/])glm[-_.]?5[-_.]?3(?:[-_.]?flash)?$/.test(name) || PINNED_MODEL_PATTERN.test(name)) {
    promotions.push("new");
  }
  return promotions;
}

export function modelPromotionLabel(locale: Locale, promotion: ModelPromotion): string {
  return LABELS[locale]?.[promotion] ?? LABELS.en[promotion];
}

export function modelPromotionPriority(modelName: string): number {
  if (PINNED_MODEL_PATTERN.test(modelName.toLowerCase())) return -1;
  const promotions = getModelPromotions(modelName);
  return promotions.length === 0 ? Number.POSITIVE_INFINITY : Math.min(...promotions.map((promotion) => PROMOTION_PRIORITY[promotion]));
}

export function sortModelsByPromotion<T extends { model_name: string }>(models: readonly T[]): T[] {
  return models
    .map((model, index) => ({ model, index, priority: modelPromotionPriority(model.model_name) }))
    .sort((a, b) => a.priority - b.priority || a.index - b.index)
    .map(({ model }) => model);
}
