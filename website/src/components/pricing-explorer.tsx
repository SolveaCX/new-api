"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { Boxes, Filter, RotateCcw, Search } from "lucide-react";
import { ModelsDirectoryTable } from "@/components/models-directory-table";
import { ModelLogo } from "@/components/pricing-model-browser";
import { getHomeCopy } from "@/lib/home-copy";
import { buildRowsForModels } from "@/lib/home-models";
import {
  filterPricingModels,
  getTopEndpoints,
  getTopVendors,
  sortPricingModelsBySeries,
  type PricingModel,
  type PricingSearch,
  type PricingVendor,
} from "@/lib/pricing";
import { localizePath, type Locale } from "@/lib/locales";
import { cn } from "@/lib/utils";

type PricingExplorerProps = {
  locale: Locale;
  models: PricingModel[];
  vendors: PricingVendor[];
  groupRatio: Record<string, number>;
  usableGroup: Record<string, string>;
  endpointMap: Record<string, unknown>;
  autoGroups: string[];
  initialSearch?: PricingSearch;
};

const ALL = "all";

export function PricingExplorer(props: PricingExplorerProps) {
  const [query, setQuery] = useState(props.initialSearch?.q ?? "");
  const [vendor, setVendor] = useState(props.initialSearch?.vendor ?? ALL);
  const [quota, setQuota] = useState(props.initialSearch?.quota ?? ALL);
  const [endpoint, setEndpoint] = useState(props.initialSearch?.endpoint ?? ALL);

  const filteredModels = useMemo(
    () =>
      sortPricingModelsBySeries(
        filterPricingModels(props.models, {
          q: query,
          vendor,
          quota,
          endpoint,
        })
      ),
    [endpoint, props.models, query, quota, vendor]
  );
  const visibleModels = filteredModels.slice(0, 120);
  const topVendors = useMemo(() => getTopVendors(props.models, 18), [props.models]);
  const topEndpoints = useMemo(() => getTopEndpoints(props.models, 10), [props.models]);
  const hasActiveFilters = vendor !== ALL || quota !== ALL || endpoint !== ALL;

  const resetFilters = () => {
    setVendor(ALL);
    setQuota(ALL);
    setEndpoint(ALL);
  };

  return (
    <>
      <section className="fk-model-toolbar mb-4 rounded-[1.25rem] border-2 border-[#101014] bg-[#FFFDF6]/92 p-4 shadow-[5px_5px_0_#101014] backdrop-blur-sm dark:border-white/24 dark:bg-[#111116]/88 dark:shadow-[5px_5px_0_rgba(255,255,255,0.16)] sm:p-5">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div className="min-w-0">
            <h2 className="inline-flex items-center gap-3 text-lg font-black tracking-normal text-[#101014] sm:text-2xl dark:text-white">
              <span className="inline-flex size-9 shrink-0 items-center justify-center rounded-full border-2 border-[#101014] bg-[#F9F871] text-[#101014] shadow-[2px_2px_0_#101014] dark:border-white/24 dark:shadow-[2px_2px_0_rgba(255,255,255,0.16)]">
                <Boxes className="size-5" aria-hidden="true" />
              </span>
              {copy(props.locale, "enabledModels", { count: props.models.length.toLocaleString() })}
            </h2>
          </div>

          <div className="relative w-full lg:max-w-xl">
            <Search className="absolute top-1/2 left-3 size-4 -translate-y-1/2 text-[#4D4D56] dark:text-white/68" />
            <input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={copy(props.locale, "searchPlaceholder")}
              className="h-11 w-full rounded-full border-2 border-[#101014] bg-white px-4 pl-10 text-sm font-semibold text-[#101014] outline-none transition-colors placeholder:text-[#6D6A72] focus:bg-[#F9F871]/24 focus:ring-3 focus:ring-[#7C3AED]/18 dark:border-white/24 dark:bg-white/8 dark:text-white dark:placeholder:text-white/52"
              type="search"
            />
          </div>
        </div>
      </section>

      <div className="grid gap-4 xl:grid-cols-[330px_minmax(0,1fr)]">
        <aside className="fk-model-filter-panel sticky top-[calc(var(--fk-header-safe-area)+1rem)] hidden max-h-[calc(100dvh-var(--fk-header-safe-area)-2rem)] self-start overflow-y-auto rounded-[1.25rem] border-2 border-[#101014] bg-[#FFFDF6]/90 p-4 shadow-[5px_5px_0_#101014] backdrop-blur-xl dark:border-white/24 dark:bg-[#111116]/88 dark:shadow-[5px_5px_0_rgba(255,255,255,0.16)] xl:block">
          <div className="mb-2.5 flex items-center justify-between gap-2">
            <div>
              <h2 className="text-sm font-black text-[#101014] dark:text-white">{copy(props.locale, "filter")}</h2>
              <p className="mt-1 text-xs leading-relaxed text-[#5C5861] dark:text-white/62">{copy(props.locale, "filterHint")}</p>
            </div>
            <button
              type="button"
              onClick={resetFilters}
              disabled={!hasActiveFilters}
              className="inline-flex h-8 items-center gap-1.5 rounded-full border border-[#101014]/15 bg-white px-2.5 text-xs font-bold text-[#101014] transition-colors hover:bg-[#F9F871] disabled:pointer-events-none disabled:opacity-40 dark:border-white/14 dark:bg-white/8 dark:text-white"
            >
              <RotateCcw className="size-3.5" />
              {copy(props.locale, "reset")}
            </button>
          </div>

          {hasActiveFilters ? <span className="mb-3 inline-flex rounded-full border border-[#7C3AED]/30 bg-[#EEE4FF] px-2.5 py-1 text-xs font-bold text-[#4C1D95] dark:bg-[#7C3AED]/20 dark:text-[#C8A8FF]">{copy(props.locale, "filtersActive")}</span> : null}

          <div className="space-y-1">
            <FilterSection title={copy(props.locale, "allVendors")}>
              <FilterChip
                href={pricingHref(props.locale)}
                label={copy(props.locale, "allVendors")}
                count={props.models.length}
                active={vendor === ALL}
                onClick={() => setVendor(ALL)}
              />
              {topVendors.map((vendorName) => {
                const vendorInfo = props.vendors.find((item) => item.name === vendorName);
                const count = props.models.filter((model) => model.vendor_name === vendorName).length;
                return (
                  <FilterChip
                    key={vendorName}
                    href={pricingHref(props.locale, { vendor: vendorName })}
                    label={vendorName}
                    count={count}
                    active={vendor === vendorName}
                    icon={vendorInfo?.icon ? <ModelLogo iconKey={vendorInfo.icon} fallback={vendorName.charAt(0)} size={14} /> : undefined}
                    onClick={() => setVendor(vendorName)}
                  />
                );
              })}
            </FilterSection>

            <FilterSection title={copy(props.locale, "pricingType")}>
              <FilterChip label={copy(props.locale, "allModels")} count={props.models.length} active={quota === ALL} onClick={() => setQuota(ALL)} />
              <FilterChip label={copy(props.locale, "tokenBased")} count={props.models.filter((model) => model.quota_type === 0).length} active={quota === "token"} onClick={() => setQuota("token")} />
              <FilterChip label={copy(props.locale, "perRequest")} count={props.models.filter((model) => model.quota_type === 1).length} active={quota === "request"} onClick={() => setQuota("request")} />
            </FilterSection>

            <FilterSection title={copy(props.locale, "endpointType")}>
              <FilterChip label={copy(props.locale, "allTypes")} count={props.models.length} active={endpoint === ALL} onClick={() => setEndpoint(ALL)} />
              {topEndpoints.map((endpointName) => (
                <FilterChip
                  key={endpointName}
                  label={endpointName}
                  count={props.models.filter((model) => model.supported_endpoint_types?.includes(endpointName)).length}
                  active={endpoint === endpointName}
                  onClick={() => setEndpoint(endpointName)}
                />
              ))}
            </FilterSection>
          </div>
        </aside>

        <section className="min-w-0 space-y-4">
          <div className="rounded-[1.1rem] border-2 border-[#101014] bg-[#FFFDF6]/90 p-3 shadow-[4px_4px_0_#101014] backdrop-blur-xl dark:border-white/24 dark:bg-[#111116]/88 dark:shadow-[4px_4px_0_rgba(255,255,255,0.16)]">
            <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
              <div className="flex items-baseline gap-1 text-sm font-semibold text-[#5C5861] dark:text-white/62">
                <span className="font-black tabular-nums text-[#101014] dark:text-white">{filteredModels.length.toLocaleString()}</span>
                <span>{filteredModels.length === 1 ? copy(props.locale, "model") : copy(props.locale, "models")}</span>
                {filteredModels.length !== props.models.length ? <span className="text-xs text-[#6D6A72] dark:text-white/48">/ {props.models.length.toLocaleString()}</span> : null}
              </div>
              <button
                type="button"
                onClick={() => {
                  const sidebar = document.querySelector<HTMLElement>("[data-pricing-mobile-filters]");
                  sidebar?.classList.toggle("hidden");
                }}
                className="inline-flex h-9 items-center gap-1.5 rounded-full border-2 border-[#101014] bg-[#F9F871] px-3 text-xs font-black text-[#101014] shadow-[2px_2px_0_#101014] dark:border-white/24 dark:shadow-[2px_2px_0_rgba(255,255,255,0.16)] xl:hidden"
              >
                <Filter className="size-4" />
                {copy(props.locale, "filter")}
              </button>
            </div>
          </div>

          <div data-pricing-mobile-filters className="hidden rounded-[1.1rem] border-2 border-[#101014] bg-[#FFFDF6]/94 p-4 shadow-[4px_4px_0_#101014] backdrop-blur-xl dark:border-white/24 dark:bg-[#111116]/90 dark:shadow-[4px_4px_0_rgba(255,255,255,0.16)] xl:hidden">
            <div className="mb-3 flex items-center justify-between">
              <span className="text-sm font-black text-[#101014] dark:text-white">{copy(props.locale, "filter")}</span>
              <button type="button" onClick={resetFilters} className="text-xs font-bold text-[#7C3AED] dark:text-[#C8A8FF]">{copy(props.locale, "reset")}</button>
            </div>
            <div className="space-y-3">
              <MobileFilterRow title={copy(props.locale, "pricingType")}>
                <FilterChip label={copy(props.locale, "allModels")} active={quota === ALL} onClick={() => setQuota(ALL)} />
                <FilterChip label={copy(props.locale, "tokenBased")} active={quota === "token"} onClick={() => setQuota("token")} />
                <FilterChip label={copy(props.locale, "perRequest")} active={quota === "request"} onClick={() => setQuota("request")} />
              </MobileFilterRow>
              <MobileFilterRow title={copy(props.locale, "allVendors")}>
                <FilterChip label={copy(props.locale, "allVendors")} active={vendor === ALL} onClick={() => setVendor(ALL)} />
                {topVendors.slice(0, 8).map((vendorName) => (
                  <FilterChip key={vendorName} label={vendorName} active={vendor === vendorName} onClick={() => setVendor(vendorName)} />
                ))}
              </MobileFilterRow>
            </div>
          </div>

          {visibleModels.length > 0 ? (
            <ModelsDirectoryTable
              copy={getHomeCopy(props.locale).table}
              rows={buildRowsForModels(visibleModels, props.vendors, props.groupRatio)}
              locale={props.locale}
            />
          ) : (
            <div className="flex min-h-64 flex-col items-center justify-center rounded-[1.25rem] border-2 border-[#101014] bg-[#FFFDF6] px-6 py-14 text-center shadow-[5px_5px_0_#101014] dark:border-white/24 dark:bg-[#111116] dark:shadow-[5px_5px_0_rgba(255,255,255,0.16)]">
              <Boxes className="size-10 text-[#6D6A72]" />
              <h2 className="mt-4 text-lg font-semibold">{copy(props.locale, "noModels")}</h2>
              <p className="mt-2 max-w-md text-sm text-[#5C5861] dark:text-white/62">{copy(props.locale, "noModelsHint")}</p>
            </div>
          )}
        </section>
      </div>
    </>
  );
}

function FilterSection(props: { title: string; children: React.ReactNode }) {
  return (
    <section className="border-b border-[#101014]/12 pb-3 last:border-b-0 dark:border-white/12">
      <h3 className="py-2.5 text-sm font-black text-[#101014] dark:text-white">{props.title}</h3>
      <div className="flex flex-wrap gap-1.5">{props.children}</div>
    </section>
  );
}

function MobileFilterRow(props: { title: string; children: React.ReactNode }) {
  return (
    <section>
      <h3 className="mb-2 text-xs font-black tracking-normal text-[#5C5861] uppercase dark:text-white/62">{props.title}</h3>
      <div className="flex flex-wrap gap-1.5">{props.children}</div>
    </section>
  );
}

function FilterChip(props: {
  label: string;
  active: boolean;
  onClick: () => void;
  count?: number;
  href?: string;
  icon?: React.ReactNode;
}) {
  const className = cn(
    "group fk-chip-motion inline-flex max-w-full items-center gap-1.5 rounded-full border-2 px-2.5 py-1.5 text-xs font-black transition-all",
    props.active
      ? "border-[#101014] bg-[#EEE4FF] text-[#101014] shadow-[2px_2px_0_#101014] dark:border-white/24 dark:bg-[#7C3AED]/24 dark:text-white dark:shadow-[2px_2px_0_rgba(255,255,255,0.16)]"
      : "border-[#101014]/18 bg-white/65 text-[#4D4D56] hover:border-[#101014] hover:bg-[#F9F871] hover:text-[#101014] dark:border-white/14 dark:bg-white/[0.055] dark:text-white/72 dark:hover:border-white/28 dark:hover:bg-white/12 dark:hover:text-white"
  );
  const content = (
    <>
      {props.icon ? <span className="shrink-0">{props.icon}</span> : null}
      <span className="truncate">{props.label}</span>
      {props.count != null ? <span className={cn("rounded-md px-1.5 py-0.5 text-[10px]", props.active ? "bg-white/80 text-[#101014] dark:bg-white/15 dark:text-white" : "bg-[#EEE4FF] text-[#4C1D95] dark:bg-white/10 dark:text-[#C8A8FF]")}>{props.count}</span> : null}
    </>
  );

  if (props.href) {
    return (
      <Link
        href={props.href}
        onClick={(event) => {
          event.preventDefault();
          props.onClick();
          window.history.replaceState(null, "", props.href);
        }}
        className={className}
        title={props.label}
      >
        {content}
      </Link>
    );
  }

  return (
    <button type="button" onClick={props.onClick} className={className} title={props.label}>
      {content}
    </button>
  );
}

function pricingHref(locale: Locale, params?: { vendor?: string }) {
  const search = new URLSearchParams();
  if (params?.vendor) search.set("vendor", params.vendor);
  const query = search.toString();
  return `${localizePath("/models", locale)}${query ? `?${query}` : ""}`;
}

const COPY: Record<string, Record<string, string>> = {
  en: {
    enabledModels: "This site currently has {{count}} models enabled",
    searchPlaceholder: "Search model name, provider, endpoint, or tag...",
    filter: "Filter",
    filterHint: "Refine models by provider, type, and endpoint.",
    reset: "Reset",
    filtersActive: "Filters active",
    allVendors: "All Vendors",
    pricingType: "Pricing Type",
    endpointType: "Endpoint Type",
    allModels: "All Models",
    tokenBased: "Token-based",
    perRequest: "Per Request",
    allTypes: "All Types",
    model: "model",
    models: "models",
    noModels: "No Models Found",
    noModelsHint: "No models match your current filters.",
  },
  zh: {
    enabledModels: "本站当前已启用 {{count}} 个模型",
    searchPlaceholder: "搜索模型名称、供应商、端点或标签...",
    filter: "筛选",
    filterHint: "按供应商、计费类型和端点筛选模型。",
    reset: "重置",
    filtersActive: "筛选已启用",
    allVendors: "全部供应商",
    pricingType: "计费类型",
    endpointType: "端点类型",
    allModels: "全部模型",
    tokenBased: "按 Token 计费",
    perRequest: "按请求计费",
    allTypes: "全部类型",
    model: "个模型",
    models: "个模型",
    noModels: "未找到模型",
    noModelsHint: "没有模型匹配当前筛选条件。",
  },
  es: {
    enabledModels: "Este sitio tiene {{count}} modelos habilitados",
    searchPlaceholder: "Buscar nombre de modelo, proveedor, endpoint o etiqueta...",
    filter: "Filtrar",
    filterHint: "Refina modelos por proveedor, tipo y endpoint.",
    reset: "Restablecer",
    filtersActive: "Filtros activos",
    allVendors: "Todos los proveedores",
    pricingType: "Tipo de precio",
    endpointType: "Tipo de endpoint",
    allModels: "Todos los modelos",
    tokenBased: "Por token",
    perRequest: "Por solicitud",
    allTypes: "Todos los tipos",
    model: "modelo",
    models: "modelos",
    noModels: "No se encontraron modelos",
    noModelsHint: "Ningún modelo coincide con los filtros actuales.",
  },
  fr: {
    enabledModels: "Ce site a actuellement {{count}} modèles activés",
    searchPlaceholder: "Rechercher un modèle, fournisseur, endpoint ou tag...",
    filter: "Filtrer",
    filterHint: "Affinez les modèles par fournisseur, type et endpoint.",
    reset: "Réinitialiser",
    filtersActive: "Filtres actifs",
    allVendors: "Tous les fournisseurs",
    pricingType: "Type de tarif",
    endpointType: "Type d'endpoint",
    allModels: "Tous les modèles",
    tokenBased: "Au token",
    perRequest: "Par requête",
    allTypes: "Tous les types",
    model: "modèle",
    models: "modèles",
    noModels: "Aucun modèle trouvé",
    noModelsHint: "Aucun modèle ne correspond aux filtres actuels.",
  },
  pt: {
    enabledModels: "Este site tem {{count}} modelos habilitados",
    searchPlaceholder: "Pesquisar nome do modelo, provedor, endpoint ou tag...",
    filter: "Filtrar",
    filterHint: "Refine modelos por provedor, tipo e endpoint.",
    reset: "Redefinir",
    filtersActive: "Filtros ativos",
    allVendors: "Todos os provedores",
    pricingType: "Tipo de preço",
    endpointType: "Tipo de endpoint",
    allModels: "Todos os modelos",
    tokenBased: "Por token",
    perRequest: "Por requisição",
    allTypes: "Todos os tipos",
    model: "modelo",
    models: "modelos",
    noModels: "Nenhum modelo encontrado",
    noModelsHint: "Nenhum modelo corresponde aos filtros atuais.",
  },
  ru: {
    enabledModels: "На сайте сейчас включено {{count}} моделей",
    searchPlaceholder: "Поиск по модели, провайдеру, endpoint или тегу...",
    filter: "Фильтр",
    filterHint: "Уточните модели по провайдеру, типу и endpoint.",
    reset: "Сбросить",
    filtersActive: "Фильтры активны",
    allVendors: "Все провайдеры",
    pricingType: "Тип цены",
    endpointType: "Тип endpoint",
    allModels: "Все модели",
    tokenBased: "По токенам",
    perRequest: "За запрос",
    allTypes: "Все типы",
    model: "модель",
    models: "моделей",
    noModels: "Модели не найдены",
    noModelsHint: "Ни одна модель не соответствует текущим фильтрам.",
  },
  ja: {
    enabledModels: "このサイトでは現在 {{count}} 個のモデルが有効です",
    searchPlaceholder: "モデル名、プロバイダー、endpoint、タグを検索...",
    filter: "フィルター",
    filterHint: "プロバイダー、種類、endpoint でモデルを絞り込みます。",
    reset: "リセット",
    filtersActive: "フィルター適用中",
    allVendors: "すべてのプロバイダー",
    pricingType: "料金タイプ",
    endpointType: "Endpoint タイプ",
    allModels: "すべてのモデル",
    tokenBased: "Token ベース",
    perRequest: "リクエスト単位",
    allTypes: "すべてのタイプ",
    model: "モデル",
    models: "モデル",
    noModels: "モデルが見つかりません",
    noModelsHint: "現在のフィルターに一致するモデルはありません。",
  },
  vi: {
    enabledModels: "Site này hiện có {{count}} mô hình được bật",
    searchPlaceholder: "Tìm tên mô hình, nhà cung cấp, endpoint hoặc tag...",
    filter: "Lọc",
    filterHint: "Lọc mô hình theo nhà cung cấp, loại và endpoint.",
    reset: "Đặt lại",
    filtersActive: "Bộ lọc đang bật",
    allVendors: "Tất cả nhà cung cấp",
    pricingType: "Loại giá",
    endpointType: "Loại endpoint",
    allModels: "Tất cả mô hình",
    tokenBased: "Theo token",
    perRequest: "Theo request",
    allTypes: "Tất cả loại",
    model: "mô hình",
    models: "mô hình",
    noModels: "Không tìm thấy mô hình",
    noModelsHint: "Không có mô hình nào khớp với bộ lọc hiện tại.",
  },
};

function copy(locale: Locale, key: string, values?: Record<string, string>) {
  let text = COPY[locale]?.[key] ?? COPY.en[key] ?? key;
  for (const [name, value] of Object.entries(values ?? {})) text = text.replace(`{{${name}}}`, value);
  return text;
}
