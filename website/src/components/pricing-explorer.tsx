"use client";

import {
  ArrowDownAZ,
  Boxes,
  Check,
  ChevronDown,
  LayoutGrid,
  List,
  RotateCcw,
  Search,
} from "lucide-react";
import Link from "next/link";
import { useEffect, useMemo, useRef, useState } from "react";
import { HomeModelLogo } from "@/components/home-model-logo";
import { ModelsDirectoryTable, type ModelsDirectoryTableCopy } from "@/components/models-directory-table";
import { getHomeCopy } from "@/lib/home-copy";
import { buildRowsForModels, type HomePricedModel } from "@/lib/home-models";
import {
  filterPricingModels,
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
type SortMode = "featured" | "provider" | "price-low" | "price-high";
type ViewMode = "list" | "cards";
type PurposeFilter = "text" | "image" | "file" | "audio" | "video";
type PurposeOption = {
  value: PurposeFilter;
  copyKey: "textPurpose" | "imagePurpose" | "filePurpose" | "audioPurpose" | "videoPurpose";
};
type SortOption = {
  value: SortMode;
  copyKey: "recommended" | "providerAsc" | "lowestInput" | "highestInput";
};
type ProviderOption = {
  name: string;
  iconKey?: string;
};
type DirectoryTableLabels = Pick<
  ModelsDirectoryTableCopy,
  "colProvider" | "colInput" | "colOutput" | "colCapabilities" | "colBilling" | "colHealth" | "noCapabilities" | "latencyLabel"
>;

const PURPOSE_FILTERS: PurposeOption[] = [
  { value: "text", copyKey: "textPurpose" },
  { value: "image", copyKey: "imagePurpose" },
  { value: "file", copyKey: "filePurpose" },
  { value: "audio", copyKey: "audioPurpose" },
  { value: "video", copyKey: "videoPurpose" },
];

const SORT_OPTIONS: SortOption[] = [
  { value: "featured", copyKey: "recommended" },
  { value: "provider", copyKey: "providerAsc" },
  { value: "price-low", copyKey: "lowestInput" },
  { value: "price-high", copyKey: "highestInput" },
];

const DIRECTORY_TABLE_LABELS: Record<Locale, DirectoryTableLabels> = {
  en: {
    colProvider: "Provider",
    colInput: "Input",
    colOutput: "Output",
    colCapabilities: "Capabilities",
    colBilling: "Billing",
    colHealth: "Health",
    noCapabilities: "API",
    latencyLabel: "TTFT",
  },
  zh: {
    colProvider: "供应商",
    colInput: "输入",
    colOutput: "输出",
    colCapabilities: "能力",
    colBilling: "计费",
    colHealth: "健康度",
    noCapabilities: "API",
    latencyLabel: "TTFT",
  },
  es: {
    colProvider: "Proveedor",
    colInput: "Entrada",
    colOutput: "Salida",
    colCapabilities: "Capacidades",
    colBilling: "Cobro",
    colHealth: "Salud",
    noCapabilities: "API",
    latencyLabel: "TTFT",
  },
  fr: {
    colProvider: "Fournisseur",
    colInput: "Entrée",
    colOutput: "Sortie",
    colCapabilities: "Fonctions",
    colBilling: "Tarif",
    colHealth: "Santé",
    noCapabilities: "API",
    latencyLabel: "TTFT",
  },
  pt: {
    colProvider: "Provedor",
    colInput: "Entrada",
    colOutput: "Saída",
    colCapabilities: "Capacidades",
    colBilling: "Cobrança",
    colHealth: "Saúde",
    noCapabilities: "API",
    latencyLabel: "TTFT",
  },
  ru: {
    colProvider: "Провайдер",
    colInput: "Вход",
    colOutput: "Выход",
    colCapabilities: "Возможности",
    colBilling: "Биллинг",
    colHealth: "Здоровье",
    noCapabilities: "API",
    latencyLabel: "TTFT",
  },
  ja: {
    colProvider: "プロバイダー",
    colInput: "入力",
    colOutput: "出力",
    colCapabilities: "機能",
    colBilling: "課金",
    colHealth: "ヘルス",
    noCapabilities: "API",
    latencyLabel: "TTFT",
  },
  vi: {
    colProvider: "Nhà cung cấp",
    colInput: "Đầu vào",
    colOutput: "Đầu ra",
    colCapabilities: "Khả năng",
    colBilling: "Tính phí",
    colHealth: "Sức khỏe",
    noCapabilities: "API",
    latencyLabel: "TTFT",
  },
  de: {
    colProvider: "Anbieter",
    colInput: "Eingabe",
    colOutput: "Ausgabe",
    colCapabilities: "Funktionen",
    colBilling: "Abrechnung",
    colHealth: "Status",
    noCapabilities: "API",
    latencyLabel: "TTFT",
  },
  id: {
    colProvider: "Penyedia",
    colInput: "Input",
    colOutput: "Output",
    colCapabilities: "Kapabilitas",
    colBilling: "Billing",
    colHealth: "Kesehatan",
    noCapabilities: "API",
    latencyLabel: "TTFT",
  },
};

export function PricingExplorer(props: PricingExplorerProps) {
  const [query, setQuery] = useState("");
  const [routeFilters, setRouteFilters] = useState(() => normalizePricingRouteSearch(props.initialSearch));
  const [sort, setSort] = useState<SortMode>("featured");
  const [viewMode, setViewMode] = useState<ViewMode>("list");
  useEffect(() => {
    const handlePopState = () => {
      setRouteFilters(normalizePricingRouteSearch(readModelsLocationSearch()));
    };

    window.addEventListener("popstate", handlePopState);
    return () => window.removeEventListener("popstate", handlePopState);
  }, []);

  const activeFilters = useMemo(
    () =>
      normalizePricingSearch({
        q: query,
        vendor: routeFilters.vendor,
        pricing: routeFilters.pricing,
        purpose: routeFilters.purpose,
      }),
    [query, routeFilters.pricing, routeFilters.purpose, routeFilters.vendor]
  );

  const filteredModels = useMemo(() => {
    const base = filterPricingModels(props.models, {
      q: activeFilters.q,
      vendor: activeFilters.vendor,
      pricing: activeFilters.pricing,
    });
    const purpose = activeFilters.purpose;

    if (!isPurposeFilter(purpose)) return base;
    return base.filter((model) => modelMatchesPurpose(model, purpose));
  }, [activeFilters.pricing, activeFilters.purpose, activeFilters.q, activeFilters.vendor, props.models]);

  const orderedModels = useMemo(
    () => (sort === "featured" ? sortPricingModelsBySeries(filteredModels) : filteredModels),
    [filteredModels, sort]
  );

  const directoryRows = useMemo(
    () => sortDirectoryRows(buildRowsForModels(orderedModels, props.vendors, props.groupRatio), sort),
    [orderedModels, props.groupRatio, props.vendors, sort]
  );
  const visibleRows = directoryRows.slice(0, 120);
  const topVendors = useMemo(() => getTopVendors(props.models, 6), [props.models]);
  const providerNames = useMemo(() => {
    const names = [...topVendors];
    if (activeFilters.vendor && activeFilters.vendor !== ALL && !names.includes(activeFilters.vendor)) {
      names.push(activeFilters.vendor);
    }
    return names;
  }, [activeFilters.vendor, topVendors]);
  const providerOptions = useMemo<ProviderOption[]>(
    () =>
      providerNames.map((vendorName) => ({
        name: vendorName,
        iconKey: props.vendors.find((item) => item.name === vendorName)?.icon,
      })),
    [props.vendors, providerNames]
  );
  const hasActiveFilters =
    activeFilters.vendor !== ALL || activeFilters.pricing !== ALL || activeFilters.purpose !== ALL || Boolean(activeFilters.q) || sort !== "featured";

  const resetFilters = () => {
    setSort("featured");
    navigateWithFilters({});
  };

  const navigateWithFilters = (nextSearch: PricingSearch) => {
    const normalized = normalizePricingSearch(nextSearch);
    setQuery(normalized.q ?? "");
    setRouteFilters(normalizePricingRouteSearch(normalized));
    window.history.pushState(null, "", modelsHref(props.locale, normalized));
  };

  return (
    <>
      <section
        className="mb-6 rounded-2xl border border-[#E6E0F0]/80 bg-white/88 p-4 shadow-[0_18px_60px_-48px_rgba(78,54,150,0.55)] backdrop-blur-md dark:border-white/10 dark:bg-white/[0.055] sm:p-5"
        data-local-models-toolbar="true"
      >
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0">
            <h2 className="inline-flex items-center gap-2.5 text-[1.35rem] font-black tracking-tight text-slate-950 sm:text-2xl dark:text-white">
              <span className="inline-flex size-8 shrink-0 items-center justify-center rounded-lg border border-[#DED6EC] bg-[#F8F5FF] text-[#6B46C1] dark:border-white/10 dark:bg-violet-300/10 dark:text-violet-200">
                <Boxes className="size-4" aria-hidden="true" />
              </span>
              {copy(props.locale, "enabledModels")}
            </h2>
          </div>
          <button
            type="button"
            onClick={resetFilters}
            disabled={!hasActiveFilters}
            data-local-reset-filters="true"
            data-local-reset-header="true"
            className="inline-flex h-9 items-center justify-center gap-2 self-start rounded-lg border border-[#CDBDFF] bg-[#F6F2FF] px-3.5 text-sm font-black text-[#5B21B6] transition-colors hover:border-[#A78BFA] hover:bg-[#EFE7FF] disabled:pointer-events-none disabled:border-[#DDD7E7] disabled:bg-white/75 disabled:text-slate-500 dark:border-violet-200/25 dark:bg-violet-300/12 dark:text-violet-100 dark:hover:border-violet-200/40 dark:hover:bg-violet-300/18 dark:disabled:border-white/10 dark:disabled:bg-white/[0.04] dark:disabled:text-slate-400"
          >
            <RotateCcw className="size-4" />
            {copy(props.locale, "reset")}
          </button>
        </div>

        <div className="mt-4 space-y-3">
          <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_auto_auto_auto]" data-local-controls-row="true">
            <label className="relative min-w-0">
              <Search className="absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-slate-400" />
              <input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder={copy(props.locale, "searchPlaceholder")}
                className="h-11 w-full rounded-xl border border-[#DED8E8] bg-white px-4 pl-10 text-sm font-semibold text-slate-900 outline-none transition-colors placeholder:text-slate-400 focus:border-[#8B5CF6] focus:ring-3 focus:ring-violet-400/12 dark:border-white/10 dark:bg-white/[0.045] dark:text-white dark:placeholder:text-slate-500"
                type="search"
              />
            </label>

            <SortControl locale={props.locale} value={sort} onChange={setSort} />
            <ViewModeToggle locale={props.locale} value={viewMode} onChange={setViewMode} />
          </div>

          <div className="space-y-3" data-local-filter-panel="true">
            <ProviderFilterGroup
              activeFilters={activeFilters}
              activeVendor={activeFilters.vendor ?? ALL}
              allLabel={copy(props.locale, "allVendors")}
              locale={props.locale}
              onSelect={(vendor) => navigateWithFilters({ ...activeFilters, vendor })}
              title={copy(props.locale, "provider")}
              vendors={providerOptions}
            />

            <div className="space-y-3" data-local-filter-groups="true">
              <FilterGroup id="pricing" title={copy(props.locale, "pricingType")}>
                <FilterChip href={modelsHref(props.locale, { ...activeFilters, pricing: ALL })} label={copy(props.locale, "allModels")} active={activeFilters.pricing === ALL} onClick={() => navigateWithFilters({ ...activeFilters, pricing: ALL })} />
                <FilterChip href={modelsHref(props.locale, { ...activeFilters, pricing: "token" })} label={copy(props.locale, "tokenBased")} active={activeFilters.pricing === "token"} onClick={() => navigateWithFilters({ ...activeFilters, pricing: "token" })} />
                <FilterChip href={modelsHref(props.locale, { ...activeFilters, pricing: "request" })} label={copy(props.locale, "perRequest")} active={activeFilters.pricing === "request"} onClick={() => navigateWithFilters({ ...activeFilters, pricing: "request" })} />
              </FilterGroup>

              <FilterGroup id="purpose" title={copy(props.locale, "purpose")}>
                <FilterChip
                  href={modelsHref(props.locale, { ...activeFilters, purpose: ALL })}
                  label={copy(props.locale, "allPurposes")}
                  active={activeFilters.purpose === ALL}
                  onClick={() => navigateWithFilters({ ...activeFilters, purpose: ALL })}
                />
                {PURPOSE_FILTERS.map((purpose) => {
                  return (
                    <FilterChip
                      key={purpose.value}
                      href={modelsHref(props.locale, { ...activeFilters, purpose: purpose.value })}
                      label={copy(props.locale, purpose.copyKey)}
                      active={activeFilters.purpose === purpose.value}
                      onClick={() => navigateWithFilters({ ...activeFilters, purpose: purpose.value })}
                    />
                  );
                })}
              </FilterGroup>
            </div>

          </div>
        </div>
      </section>

      <section className="relative min-h-64">
        {visibleRows.length > 0 ? (
          <ModelsDirectoryTable copy={getModelsDirectoryTableCopy(props.locale)} rows={visibleRows} locale={props.locale} view={viewMode} />
        ) : (
          <div className="flex min-h-64 flex-col items-center justify-center rounded-2xl border border-slate-200/80 bg-white/82 px-6 py-14 text-center shadow-lg dark:border-white/10 dark:bg-[#0a1020]/78">
            <Boxes className="size-10 text-slate-400" />
            <h2 className="mt-4 text-lg font-semibold text-slate-950 dark:text-white">{copy(props.locale, "noModels")}</h2>
            <p className="mt-2 max-w-md text-sm text-slate-500 dark:text-slate-400">{copy(props.locale, "noModelsHint")}</p>
          </div>
        )}
      </section>
    </>
  );
}

export function getModelsDirectoryTableCopy(locale: Locale): ModelsDirectoryTableCopy {
  return {
    ...getHomeCopy(locale).table,
    ...DIRECTORY_TABLE_LABELS[locale],
  };
}

function FilterGroup(props: { id: string; title: string; children: React.ReactNode }) {
  return (
    <section className="min-w-0" data-local-filter-section={props.id}>
      <h3 className="mb-1.5 text-[10px] font-black tracking-[0.08em] text-slate-500 uppercase dark:text-slate-400">{props.title}</h3>
      <div className="flex flex-wrap gap-1.5">{props.children}</div>
    </section>
  );
}

function ProviderFilterGroup(props: {
  activeFilters: PricingSearch;
  activeVendor: string;
  allLabel: string;
  locale: Locale;
  onSelect: (vendor: string) => void;
  title: string;
  vendors: ProviderOption[];
}) {
  return (
    <section
      className="min-w-0"
      data-local-provider-selector="true"
      data-local-filter-section="provider"
    >
      <h3 className="mb-1.5 text-[10px] font-black tracking-[0.08em] text-slate-500 uppercase dark:text-slate-400">{props.title}</h3>
      <div className="flex flex-wrap gap-1.5">
        <ProviderFilterLink
          active={props.activeVendor === ALL}
          href={modelsHref(props.locale, { ...props.activeFilters, vendor: ALL })}
          label={props.allLabel}
          onClick={() => props.onSelect(ALL)}
          vendor={ALL}
        />
        {props.vendors.map((vendor) => (
          <ProviderFilterLink
            key={vendor.name}
            active={props.activeVendor === vendor.name}
            href={modelsHref(props.locale, { ...props.activeFilters, vendor: vendor.name })}
            iconKey={vendor.iconKey}
            label={vendor.name}
            onClick={() => props.onSelect(vendor.name)}
            vendor={vendor.name}
          />
        ))}
      </div>
    </section>
  );
}

function ProviderFilterLink(props: {
  active: boolean;
  href: string;
  iconKey?: string;
  label: string;
  onClick: () => void;
  vendor: string;
}) {
  const className = cn(
    "group inline-flex h-7 max-w-[9.5rem] shrink-0 items-center gap-1.5 rounded-lg px-2 text-left text-[11px] font-bold transition-colors",
    props.active
      ? "bg-[#F5F0FF] text-[#4C1D95] shadow-sm dark:bg-violet-300/14 dark:text-violet-100"
      : "bg-white/85 text-slate-700 hover:bg-[#FAF8FF] hover:text-[#4C1D95] dark:bg-white/[0.045] dark:text-slate-300 dark:hover:bg-violet-300/10 dark:hover:text-white"
  );
  const logoClassName = props.active
    ? "bg-white shadow-sm"
    : "bg-white shadow-sm dark:bg-white/8";

  return (
    <Link
      href={props.href}
      onClick={(event) => {
        if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
        event.preventDefault();
        props.onClick();
      }}
      data-local-models-filter="true"
      data-local-provider-filter={props.vendor}
      className={className}
      title={props.label}
    >
      {props.vendor === ALL ? (
        <span
          className={cn(
            "grid size-3.5 shrink-0 place-items-center rounded-md",
            props.active ? "bg-white text-[#6B46C1]" : "bg-slate-50 text-slate-500 dark:bg-white/8 dark:text-slate-300"
          )}
          aria-hidden="true"
        >
          <Boxes className="size-2.5" />
        </span>
      ) : (
        <HomeModelLogo
          className={logoClassName}
          iconKey={props.iconKey}
          modelName={props.label}
          vendor={props.label}
          fallback={props.label.charAt(0)}
          surfaceSize={18}
          imageSize={11}
        />
      )}
      <span className="min-w-0 truncate">{props.label}</span>
    </Link>
  );
}

function ViewModeToggle(props: { locale: Locale; value: ViewMode; onChange: (value: ViewMode) => void }) {
  const options = [
    { value: "list" as const, label: copy(props.locale, "listView"), icon: <List className="size-4" aria-hidden="true" /> },
    { value: "cards" as const, label: copy(props.locale, "cardView"), icon: <LayoutGrid className="size-4" aria-hidden="true" /> },
  ];

  return (
    <div
      className="inline-flex h-10 items-center rounded-xl border border-[#DED8E8] bg-white p-1 dark:border-white/10 dark:bg-white/[0.045]"
      data-local-view-toggle="true"
    >
      {options.map((option) => {
        const active = props.value === option.value;
        return (
            <button
              key={option.value}
              type="button"
              onClick={() => props.onChange(option.value)}
              className={cn(
              "grid size-8 place-items-center rounded-lg transition-colors",
              active
                ? "bg-[#F1EAFF] text-[#5B21B6] dark:bg-violet-300/16 dark:text-violet-100"
                : "text-slate-500 hover:bg-[#FAF8FF] hover:text-[#5B21B6] dark:text-slate-300 dark:hover:bg-violet-300/10 dark:hover:text-white"
            )}
            aria-label={option.label}
            aria-pressed={active}
            title={option.label}
          >
            {option.icon}
          </button>
        );
      })}
    </div>
  );
}

function SortControl(props: { locale: Locale; value: SortMode; onChange: (value: SortMode) => void }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const activeLabel = copy(props.locale, SORT_OPTIONS.find((option) => option.value === props.value)?.copyKey ?? "recommended");

  useEffect(() => {
    if (!open) return;
    const handlePointerDown = (event: PointerEvent) => {
      if (!ref.current?.contains(event.target as Node)) setOpen(false);
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    document.addEventListener("pointerdown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [open]);

  return (
    <div ref={ref} className="relative min-w-[214px]" data-local-sort-dropdown="true">
      <button
        type="button"
        className="inline-flex h-10 w-full items-center justify-between gap-3 rounded-xl border border-[#DDD7E7] bg-white px-3.5 text-sm font-bold text-slate-800 transition-colors hover:border-[#BCA7FF] hover:bg-[#FAF8FF] focus:border-violet-400 focus:ring-3 focus:ring-violet-400/12 focus:outline-none dark:border-white/10 dark:bg-white/[0.045] dark:text-white dark:hover:border-violet-300/35 dark:hover:bg-violet-300/10"
        onClick={() => setOpen((current) => !current)}
        aria-expanded={open}
        aria-haspopup="menu"
      >
        <span className="inline-flex min-w-0 items-center gap-2">
          <ArrowDownAZ className="size-4 shrink-0 text-violet-500" aria-hidden="true" />
          <span className="text-sm font-bold text-slate-500 dark:text-slate-400">{copy(props.locale, "sort")}</span>
          <span className="truncate">{activeLabel}</span>
        </span>
        <ChevronDown className={cn("size-4 shrink-0 text-slate-400 transition-transform", open ? "rotate-180" : "")} aria-hidden="true" />
      </button>

      {open ? (
        <div
          className="absolute right-0 z-20 mt-2 w-72 overflow-hidden rounded-xl border border-[#DDD7E7] bg-white p-1.5 shadow-xl dark:border-white/10 dark:bg-[#0b1020]"
          role="menu"
        >
          {SORT_OPTIONS.map((option) => {
            const active = props.value === option.value;
            return (
              <button
                key={option.value}
                type="button"
                onClick={() => {
                  props.onChange(option.value);
                  setOpen(false);
                }}
                className={cn(
                  "flex h-10 w-full items-center justify-between rounded-md px-3 text-left text-sm font-semibold transition-colors",
                  active
                    ? "bg-violet-50 text-violet-800 dark:bg-violet-300/12 dark:text-violet-100"
                    : "text-slate-600 hover:bg-slate-50 hover:text-slate-950 dark:text-slate-300 dark:hover:bg-white/[0.06] dark:hover:text-white"
                )}
                role="menuitemradio"
                aria-checked={active}
              >
                {copy(props.locale, option.copyKey)}
                {active ? <Check className="size-4 text-violet-600 dark:text-violet-300" aria-hidden="true" /> : null}
              </button>
            );
          })}
        </div>
      ) : null}
    </div>
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
    "group inline-flex h-7 max-w-full items-center gap-1.5 rounded-lg px-2 text-[11px] font-bold transition-colors",
    props.active
      ? "bg-[#F5F0FF] text-[#4C1D95] shadow-sm dark:bg-violet-300/14 dark:text-violet-100"
      : "bg-white/85 text-slate-700 hover:bg-[#FAF8FF] hover:text-[#4C1D95] dark:bg-white/[0.045] dark:text-slate-300 dark:hover:bg-violet-300/10 dark:hover:text-white"
  );
  const content = (
    <>
      {props.icon ? <span className="shrink-0">{props.icon}</span> : null}
      <span className="truncate">{props.label}</span>
      {props.count != null ? (
        <span
          className={cn(
            "rounded-md px-1.5 py-0.5 text-[10px]",
            props.active
              ? "bg-white/80 text-violet-900 dark:bg-white/15 dark:text-violet-100"
              : "bg-slate-100 text-slate-500 dark:bg-white/10 dark:text-slate-300"
          )}
        >
          {props.count}
        </span>
      ) : null}
    </>
  );

  if (props.href) {
    return (
      <Link
        href={props.href}
        onClick={(event) => {
          if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
          event.preventDefault();
          props.onClick();
        }}
        data-local-models-filter="true"
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

function sortDirectoryRows(rows: HomePricedModel[], sort: SortMode): HomePricedModel[] {
  const byModelName = (a: HomePricedModel, b: HomePricedModel) => a.name.localeCompare(b.name, "en", { numeric: true });
  if (sort === "featured") return rows;
  return [...rows].sort((a, b) => {
    if (sort === "provider") {
      return a.vendor.localeCompare(b.vendor, "en", { numeric: true }) || byModelName(a, b);
    }
    if (sort === "price-low") {
      return a.discountedUsd - b.discountedUsd || byModelName(a, b);
    }
    return b.discountedUsd - a.discountedUsd || byModelName(a, b);
  });
}

function normalizeFilterValue(value: string | undefined): string | undefined {
  const normalized = value?.trim();
  if (!normalized || normalized.toLowerCase() === ALL) return undefined;
  return normalized;
}

export function normalizePricingSearch(search: PricingSearch = {}): PricingSearch {
  const q = normalizeFilterValue(search.q);
  const vendor = normalizeFilterValue(search.vendor);
  const pricing = normalizeFilterValue(search.pricing ?? search.quota);
  const purpose = normalizeFilterValue(search.purpose);
  return {
    ...(q ? { q } : {}),
    vendor: vendor ?? ALL,
    pricing: pricing ?? ALL,
    endpoint: ALL,
    purpose: isPurposeFilter(purpose) ? purpose : ALL,
  };
}

function normalizePricingRouteSearch(search: PricingSearch = {}): PricingSearch {
  const normalized = normalizePricingSearch(search);
  return {
    vendor: normalized.vendor ?? ALL,
    pricing: normalized.pricing ?? ALL,
    purpose: normalized.purpose ?? ALL,
  };
}

function readModelsLocationSearch(): PricingSearch {
  const params = new URLSearchParams(window.location.search);
  return {
    vendor: params.get("vendor") ?? undefined,
    pricing: params.get("pricing") ?? params.get("quota") ?? undefined,
    purpose: params.get("purpose") ?? undefined,
  };
}

export function modelsHref(locale: Locale, search: PricingSearch = {}) {
  const normalized = normalizePricingSearch(search);
  const params = new URLSearchParams();
  if (normalized.vendor && normalized.vendor !== ALL) params.set("vendor", normalized.vendor);
  if (normalized.pricing && normalized.pricing !== ALL) params.set("pricing", normalized.pricing);
  if (normalized.purpose && normalized.purpose !== ALL) params.set("purpose", normalized.purpose);
  const query = params.toString();
  return `${localizePath("/models", locale)}${query ? `?${query}` : ""}`;
}

function isPurposeFilter(value: string | undefined): value is PurposeFilter {
  return PURPOSE_FILTERS.some((item) => item.value === value);
}

function modelMatchesPurpose(model: PricingModel, purpose: PurposeFilter): boolean {
  const haystack = modelSearchText(model);
  if (purpose === "image") return hasAny(haystack, ["image", "images", "vision", "imagen", "dall e", "dalle", "gpt image"]) || hasPriceDimension(model, "image");
  if (purpose === "video") return hasAny(haystack, ["video", "videos", "sora", "veo", "kling", "seedance"]);
  if (purpose === "audio") return hasAny(haystack, ["audio", "voice", "speech", "tts", "stt", "transcribe", "realtime"]) || hasPriceDimension(model, "audio");
  if (purpose === "file") return hasAny(haystack, ["file", "files", "document", "documents", "pdf", "ocr", "embedding", "embeddings", "rerank"]);
  return (
    hasAny(haystack, ["text", "chat", "completion", "responses", "reason", "reasoning", "tool", "tools", "code", "json"]) ||
    !hasAny(haystack, ["image", "video", "audio", "voice", "speech", "tts", "file", "document", "pdf", "ocr"])
  );
}

function modelSearchText(model: PricingModel): string {
  return [
    model.model_name,
    model.description,
    model.vendor_name,
    model.tags,
    model.display_pricing?.billing_kind,
    ...Object.keys(model.display_pricing?.prices ?? {}),
    ...(model.supported_endpoint_types ?? []),
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase()
    .replace(/[_./:-]+/g, " ");
}

function hasAny(value: string, needles: string[]): boolean {
  return needles.some((needle) => value.includes(needle));
}

function hasPriceDimension(model: PricingModel, prefix: string): boolean {
  return Object.keys(model.display_pricing?.prices ?? {}).some((dimension) => dimension.startsWith(prefix));
}

const COPY: Record<string, Record<string, string>> = {
  en: {
    enabledModels: "Browse 100+ models",
    searchPlaceholder: "Search model name, provider, or purpose...",
    filter: "Filter",
    reset: "Reset",
    filtersActive: "Live directory",
    provider: "Provider",
    pricingType: "Pricing Type",
    purpose: "Purpose",
    allVendors: "All Vendors",
    allModels: "All Models",
    tokenBased: "Token-based",
    perRequest: "Per Request",
    allPurposes: "All",
    textPurpose: "Text",
    imagePurpose: "Image",
    filePurpose: "File",
    audioPurpose: "Audio",
    videoPurpose: "Video",
    noModels: "No Models Found",
    noModelsHint: "No models match your current filters.",
    loadingModels: "Loading models",
    sort: "Sort",
    listView: "List view",
    cardView: "Card view",
    recommended: "Recommended",
    providerAsc: "Provider A-Z",
    lowestInput: "Lowest input",
    highestInput: "Highest input",
  },
  zh: {
    enabledModels: "浏览 100+ 个模型",
    searchPlaceholder: "搜索模型名称、供应商或用途...",
    filter: "筛选",
    reset: "重置",
    filtersActive: "实时目录",
    provider: "供应商",
    pricingType: "计费类型",
    purpose: "用途",
    allVendors: "全部供应商",
    allModels: "全部模型",
    tokenBased: "按 Token 计费",
    perRequest: "按请求计费",
    allPurposes: "全部",
    textPurpose: "文本",
    imagePurpose: "图像",
    filePurpose: "文件",
    audioPurpose: "音频",
    videoPurpose: "视频",
    noModels: "未找到模型",
    noModelsHint: "没有模型匹配当前筛选条件。",
    loadingModels: "正在加载模型",
    sort: "排序",
    listView: "列表视图",
    cardView: "卡片视图",
    recommended: "推荐",
    providerAsc: "供应商 A-Z",
    lowestInput: "输入价从低到高",
    highestInput: "输入价从高到低",
  },
  es: {
    enabledModels: "Explorar 100+ modelos",
    searchPlaceholder: "Buscar modelo, proveedor o uso...",
    filter: "Filtrar",
    reset: "Restablecer",
    filtersActive: "Directorio en vivo",
    provider: "Proveedor",
    pricingType: "Tipo de precio",
    purpose: "Uso",
    allVendors: "Todos los proveedores",
    allModels: "Todos los modelos",
    tokenBased: "Por token",
    perRequest: "Por solicitud",
    allPurposes: "Todos",
    textPurpose: "Texto",
    imagePurpose: "Imagen",
    filePurpose: "Archivo",
    audioPurpose: "Audio",
    videoPurpose: "Vídeo",
    noModels: "No se encontraron modelos",
    noModelsHint: "Ningún modelo coincide con los filtros actuales.",
    loadingModels: "Cargando modelos",
    sort: "Ordenar",
    listView: "Vista de lista",
    cardView: "Vista de tarjetas",
    recommended: "Recomendado",
    providerAsc: "Proveedor A-Z",
    lowestInput: "Entrada más baja",
    highestInput: "Entrada más alta",
  },
  fr: {
    enabledModels: "Explorer 100+ modèles",
    searchPlaceholder: "Rechercher un modèle, fournisseur ou usage...",
    filter: "Filtrer",
    reset: "Réinitialiser",
    filtersActive: "Répertoire live",
    provider: "Fournisseur",
    pricingType: "Type de tarif",
    purpose: "Usage",
    allVendors: "Tous les fournisseurs",
    allModels: "Tous les modèles",
    tokenBased: "Au token",
    perRequest: "Par requête",
    allPurposes: "Tous",
    textPurpose: "Texte",
    imagePurpose: "Image",
    filePurpose: "Fichier",
    audioPurpose: "Audio",
    videoPurpose: "Vidéo",
    noModels: "Aucun modèle trouvé",
    noModelsHint: "Aucun modèle ne correspond aux filtres actuels.",
    loadingModels: "Chargement des modèles",
    sort: "Trier",
    listView: "Vue liste",
    cardView: "Vue cartes",
    recommended: "Recommandé",
    providerAsc: "Fournisseur A-Z",
    lowestInput: "Entrée la moins chère",
    highestInput: "Entrée la plus chère",
  },
  pt: {
    enabledModels: "Explorar 100+ modelos",
    searchPlaceholder: "Pesquisar modelo, provedor ou uso...",
    filter: "Filtrar",
    reset: "Redefinir",
    filtersActive: "Diretório ao vivo",
    provider: "Provedor",
    pricingType: "Tipo de preço",
    purpose: "Uso",
    allVendors: "Todos os provedores",
    allModels: "Todos os modelos",
    tokenBased: "Por token",
    perRequest: "Por requisição",
    allPurposes: "Todos",
    textPurpose: "Texto",
    imagePurpose: "Imagem",
    filePurpose: "Arquivo",
    audioPurpose: "Áudio",
    videoPurpose: "Vídeo",
    noModels: "Nenhum modelo encontrado",
    noModelsHint: "Nenhum modelo corresponde aos filtros atuais.",
    loadingModels: "Carregando modelos",
    sort: "Ordenar",
    listView: "Vista de lista",
    cardView: "Vista de cartões",
    recommended: "Recomendado",
    providerAsc: "Provedor A-Z",
    lowestInput: "Menor entrada",
    highestInput: "Maior entrada",
  },
  ru: {
    enabledModels: "Просмотр 100+ моделей",
    searchPlaceholder: "Поиск по модели, провайдеру или назначению...",
    filter: "Фильтр",
    reset: "Сбросить",
    filtersActive: "Живой каталог",
    provider: "Провайдер",
    pricingType: "Тип цены",
    purpose: "Назначение",
    allVendors: "Все провайдеры",
    allModels: "Все модели",
    tokenBased: "По токенам",
    perRequest: "За запрос",
    allPurposes: "Все",
    textPurpose: "Текст",
    imagePurpose: "Изображения",
    filePurpose: "Файлы",
    audioPurpose: "Аудио",
    videoPurpose: "Видео",
    noModels: "Модели не найдены",
    noModelsHint: "Ни одна модель не соответствует текущим фильтрам.",
    loadingModels: "Загрузка моделей",
    sort: "Сортировка",
    listView: "Вид списком",
    cardView: "Вид карточками",
    recommended: "Рекомендовано",
    providerAsc: "Провайдер A-Z",
    lowestInput: "Минимальный вход",
    highestInput: "Максимальный вход",
  },
  ja: {
    enabledModels: "100+ 個のモデルを閲覧",
    searchPlaceholder: "モデル名、プロバイダー、用途を検索...",
    filter: "フィルター",
    reset: "リセット",
    filtersActive: "ライブディレクトリ",
    provider: "プロバイダー",
    pricingType: "料金タイプ",
    purpose: "用途",
    allVendors: "すべてのプロバイダー",
    allModels: "すべてのモデル",
    tokenBased: "Token ベース",
    perRequest: "リクエスト単位",
    allPurposes: "すべて",
    textPurpose: "テキスト",
    imagePurpose: "画像",
    filePurpose: "ファイル",
    audioPurpose: "音声",
    videoPurpose: "動画",
    noModels: "モデルが見つかりません",
    noModelsHint: "現在のフィルターに一致するモデルはありません。",
    loadingModels: "モデルを読み込み中",
    sort: "並び替え",
    listView: "リスト表示",
    cardView: "カード表示",
    recommended: "おすすめ",
    providerAsc: "プロバイダー A-Z",
    lowestInput: "入力料金が低い順",
    highestInput: "入力料金が高い順",
  },
  vi: {
    enabledModels: "Duyệt 100+ mô hình",
    searchPlaceholder: "Tìm mô hình, nhà cung cấp hoặc mục đích...",
    filter: "Lọc",
    reset: "Đặt lại",
    filtersActive: "Danh mục trực tiếp",
    provider: "Nhà cung cấp",
    pricingType: "Loại giá",
    purpose: "Mục đích",
    allVendors: "Tất cả nhà cung cấp",
    allModels: "Tất cả mô hình",
    tokenBased: "Theo token",
    perRequest: "Theo request",
    allPurposes: "Tất cả",
    textPurpose: "Văn bản",
    imagePurpose: "Hình ảnh",
    filePurpose: "Tệp",
    audioPurpose: "Âm thanh",
    videoPurpose: "Video",
    noModels: "Không tìm thấy mô hình",
    noModelsHint: "Không có mô hình nào khớp với bộ lọc hiện tại.",
    loadingModels: "Đang tải mô hình",
    sort: "Sắp xếp",
    listView: "Xem dạng danh sách",
    cardView: "Xem dạng thẻ",
    recommended: "Đề xuất",
    providerAsc: "Nhà cung cấp A-Z",
    lowestInput: "Đầu vào thấp nhất",
    highestInput: "Đầu vào cao nhất",
  },
  de: {
    enabledModels: "100+ Modelle durchsuchen",
    searchPlaceholder: "Modellname, Anbieter oder Zweck suchen...",
    filter: "Filtern",
    reset: "Zuruecksetzen",
    filtersActive: "Live-Verzeichnis",
    provider: "Anbieter",
    pricingType: "Preistyp",
    purpose: "Zweck",
    allVendors: "Alle Anbieter",
    allModels: "Alle Modelle",
    tokenBased: "Token-basiert",
    perRequest: "Pro Anfrage",
    allPurposes: "Alle",
    textPurpose: "Text",
    imagePurpose: "Bild",
    filePurpose: "Datei",
    audioPurpose: "Audio",
    videoPurpose: "Video",
    noModels: "Keine Modelle gefunden",
    noModelsHint: "Keine Modelle passen zu den aktuellen Filtern.",
    loadingModels: "Modelle werden geladen",
    sort: "Sortieren",
    listView: "Listenansicht",
    cardView: "Kartenansicht",
    recommended: "Empfohlen",
    providerAsc: "Anbieter A-Z",
    lowestInput: "Niedrigste Eingabe",
    highestInput: "Hoechste Eingabe",
  },
  id: {
    enabledModels: "Jelajahi 100+ model",
    searchPlaceholder: "Cari model, penyedia, atau tujuan...",
    filter: "Filter",
    reset: "Atur ulang",
    filtersActive: "Direktori live",
    provider: "Penyedia",
    pricingType: "Tipe harga",
    purpose: "Tujuan",
    allVendors: "Semua penyedia",
    allModels: "Semua model",
    tokenBased: "Berbasis token",
    perRequest: "Per permintaan",
    allPurposes: "Semua",
    textPurpose: "Teks",
    imagePurpose: "Gambar",
    filePurpose: "File",
    audioPurpose: "Audio",
    videoPurpose: "Video",
    noModels: "Model tidak ditemukan",
    noModelsHint: "Tidak ada model yang cocok dengan filter saat ini.",
    loadingModels: "Memuat model",
    sort: "Urutkan",
    listView: "Tampilan daftar",
    cardView: "Tampilan kartu",
    recommended: "Direkomendasikan",
    providerAsc: "Penyedia A-Z",
    lowestInput: "Input terendah",
    highestInput: "Input tertinggi",
  },
};

function copy(locale: Locale, key: string, values?: Record<string, string>) {
  let text = COPY[locale]?.[key] ?? COPY.en[key] ?? key;
  for (const [name, value] of Object.entries(values ?? {})) text = text.replace(`{{${name}}}`, value);
  return text;
}
