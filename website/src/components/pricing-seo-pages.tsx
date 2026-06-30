import Link from "next/link";
import { ArrowRight, Boxes, CheckCircle2, DollarSign } from "lucide-react";
import { SiteShell } from "@/components/site-shell";
import { PricingModelBrowser } from "@/components/pricing-model-browser";
import {
  formatModelPrice,
  getPricingData,
  isTokenBasedModel,
  type PricingModel,
} from "@/lib/pricing";
import {
  buildPricingSeoIndex,
  findModelSeoEntry,
  findVendorSeoEntry,
  pricingSeoCopy,
  vendorPricingHref,
  type PricingSeoModel,
  type PricingSeoVendor,
} from "@/lib/pricing-seo";
import { localizePath, type Locale } from "@/lib/locales";
import { consoleUrl } from "@/lib/origins";

type SeoPageResult<T> =
  | { found: true; entry: T; index: ReturnType<typeof buildPricingSeoIndex> }
  | { found: false };

export async function getVendorSeoPageData(slug: string): Promise<SeoPageResult<PricingSeoVendor>> {
  const pricing = await getPricingData();
  const index = buildPricingSeoIndex(pricing);
  const entry = findVendorSeoEntry(index, slug);
  if (!entry) return { found: false };
  return { found: true, entry, index };
}

export async function getModelSeoPageData(slug: string): Promise<SeoPageResult<PricingSeoModel>> {
  const pricing = await getPricingData();
  const index = buildPricingSeoIndex(pricing);
  const entry = findModelSeoEntry(index, slug);
  if (!entry) return { found: false };
  return { found: true, entry, index };
}

export function VendorSeoPage(props: { locale: Locale; entry: PricingSeoVendor }) {
  const copy = pricingSeoCopy(props.locale);
  const pricingPath = vendorPricingHref(localizePath("/pricing", props.locale), props.entry.slug);
  const modelCount = props.entry.models.length.toLocaleString();

  return (
    <SiteShell locale={props.locale} pathname={`/vendors/${props.entry.slug}`}>
      <div className="model-square-page relative min-h-screen overflow-x-hidden bg-[linear-gradient(180deg,#f4f0ff_0%,#fbfaff_32%,#ffffff_62%,#f4f1ff_100%)] dark:bg-[linear-gradient(180deg,#050712_0%,#080b18_36%,#070712_72%,#03040b_100%)]">
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 bg-[linear-gradient(to_right,rgba(124,58,237,0.08)_1px,transparent_1px),linear-gradient(to_bottom,rgba(124,58,237,0.08)_1px,transparent_1px)] bg-[size:4.5rem_4.5rem] opacity-70 dark:bg-[linear-gradient(to_right,rgba(148,163,184,0.055)_1px,transparent_1px),linear-gradient(to_bottom,rgba(148,163,184,0.045)_1px,transparent_1px)] dark:opacity-45"
        />
        <div
          aria-hidden
          className="pointer-events-none absolute inset-x-0 top-0 h-[640px] opacity-75"
          style={{
            background: [
              "radial-gradient(ellipse 56% 46% at 22% 8%, rgba(168,85,247,0.30) 0%, transparent 68%)",
              "radial-gradient(ellipse 46% 36% at 78% 6%, rgba(99,102,241,0.28) 0%, transparent 70%)",
              "radial-gradient(ellipse 48% 34% at 50% 46%, rgba(217,70,239,0.18) 0%, transparent 72%)",
            ].join(", "),
            maskImage: "linear-gradient(to bottom, black 40%, transparent 100%)",
            WebkitMaskImage: "linear-gradient(to bottom, black 40%, transparent 100%)",
          }}
        />
        <div className="relative mx-auto w-full max-w-[1800px] px-3 pt-16 pb-8 sm:px-6 sm:pt-20 sm:pb-10 xl:px-8">
          <header className="mx-auto mb-6 max-w-3xl pt-5 text-center sm:mb-10 sm:pt-10">
            <p className="mx-auto mb-4 inline-flex items-center gap-2 rounded-full border border-violet-400/35 bg-violet-500/10 px-4 py-1.5 text-xs font-semibold tracking-[0.18em] text-violet-700 uppercase shadow-[0_0_28px_rgba(168,85,247,0.14)] dark:border-violet-300/25 dark:bg-violet-300/10 dark:text-violet-200">
              <span className="size-1.5 rounded-full bg-violet-500 shadow-[0_0_12px_rgba(168,85,247,0.9)] dark:bg-violet-300" />
              {copy.vendorEyebrow}
            </p>
            <h1 className="bg-[linear-gradient(90deg,#171321_0%,#7c3aed_46%,#2563eb_100%)] bg-clip-text text-[clamp(2.6rem,7vw,5rem)] leading-[0.98] font-black tracking-tight text-transparent dark:bg-[linear-gradient(90deg,#ffffff_0%,#c4b5fd_48%,#93c5fd_100%)] dark:bg-clip-text">
              {props.entry.displayName} API models
            </h1>
            <p className="mx-auto mt-5 max-w-2xl text-sm leading-relaxed text-slate-600 dark:text-slate-300 sm:text-base">
              Compare {props.entry.displayName} models on flatkey.ai, route them through one API key, and keep usage,
              billing, provider access, and model pricing in one console.
            </p>
            <div className="mt-6 flex flex-wrap justify-center gap-3">
              <Link
                href={pricingPath}
                className="flatkey-primary-cta inline-flex h-10 items-center justify-center rounded-lg px-4 text-sm font-medium shadow-[0_16px_34px_-18px_rgba(15,23,42,0.55)] transition-opacity hover:opacity-90"
              >
                {copy.vendorPricingCta}
                <ArrowRight className="size-4" />
              </Link>
              <a
                href={consoleUrl("/sign-up")}
                className="inline-flex h-10 items-center rounded-lg border border-violet-300/30 bg-white/65 px-4 text-sm font-semibold text-slate-700 transition-colors hover:border-violet-400/45 hover:bg-white dark:border-violet-300/20 dark:bg-white/[0.06] dark:text-slate-200"
              >
                Get an API key
              </a>
            </div>
          </header>

          <section className="mb-4 rounded-3xl border border-violet-500/14 bg-white/72 p-5 shadow-[0_18px_64px_-56px_rgba(91,33,182,0.62)] backdrop-blur-sm dark:border-white/10 dark:bg-white/[0.055] dark:shadow-[0_18px_64px_-56px_rgba(124,58,237,0.95)] sm:p-6">
            <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
              <div className="min-w-0">
                <h2 className="text-foreground inline-flex items-center gap-3 text-xl font-bold tracking-tight sm:text-2xl">
                  <span className="bg-muted/70 border-border text-foreground/80 inline-flex size-9 shrink-0 items-center justify-center rounded-full border">
                    <Boxes className="size-5" aria-hidden="true" />
                  </span>
                  {copy.vendorModelsTitle}: {modelCount}
                </h2>
                <p className="mt-3 max-w-3xl text-sm leading-7 text-slate-600 dark:text-slate-300">
                  The stable provider URL is <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs dark:bg-white/10">/pricing?vendor={props.entry.slug}</code>.
                  Use it to share and crawl {props.entry.displayName} pricing without relying on localized provider names.
                </p>
              </div>
              <Link href={pricingPath} className="inline-flex h-9 shrink-0 items-center gap-2 rounded-full border border-violet-500/16 bg-violet-500/8 px-3 text-sm font-semibold text-violet-700 transition-colors hover:border-violet-500/25 hover:bg-violet-500/12 hover:text-violet-600 dark:border-violet-300/20 dark:bg-violet-300/10 dark:text-violet-200">
                {copy.vendorPricingCta}
                <ArrowRight className="size-4" aria-hidden="true" />
              </Link>
            </div>
          </section>

          <PricingModelBrowser
            locale={props.locale}
            models={props.entry.models}
            groupRatio={mergeGroupRatio(props.entry.models)}
            usableGroup={{}}
            endpointMap={{}}
            autoGroups={[]}
          />
        </div>
      </div>
    </SiteShell>
  );
}

export function ModelSeoPage(props: { locale: Locale; entry: PricingSeoModel }) {
  const copy = pricingSeoCopy(props.locale);
  const model = props.entry.model;
  const vendor = props.entry.vendor;
  const pricingPath = vendorPricingHref(localizePath("/pricing", props.locale), model.vendor_slug ?? vendor?.slug ?? "all");
  const relatedModels = vendor?.models.filter((candidate) => candidate.model_name !== model.model_name).slice(0, 6) ?? [];

  return (
    <SiteShell locale={props.locale} pathname={`/models/${props.entry.slug}`}>
      <div className="model-square-page relative min-h-screen overflow-x-hidden bg-[linear-gradient(180deg,#f4f0ff_0%,#fbfaff_32%,#ffffff_62%,#f4f1ff_100%)] dark:bg-[linear-gradient(180deg,#050712_0%,#080b18_36%,#070712_72%,#03040b_100%)]">
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 bg-[linear-gradient(to_right,rgba(124,58,237,0.08)_1px,transparent_1px),linear-gradient(to_bottom,rgba(124,58,237,0.08)_1px,transparent_1px)] bg-[size:4.5rem_4.5rem] opacity-70 dark:bg-[linear-gradient(to_right,rgba(148,163,184,0.055)_1px,transparent_1px),linear-gradient(to_bottom,rgba(148,163,184,0.045)_1px,transparent_1px)] dark:opacity-45"
        />
        <div
          aria-hidden
          className="pointer-events-none absolute inset-x-0 top-0 h-[640px] opacity-75"
          style={{
            background: [
              "radial-gradient(ellipse 56% 46% at 22% 8%, rgba(168,85,247,0.30) 0%, transparent 68%)",
              "radial-gradient(ellipse 46% 36% at 78% 6%, rgba(99,102,241,0.28) 0%, transparent 70%)",
              "radial-gradient(ellipse 48% 34% at 50% 46%, rgba(217,70,239,0.18) 0%, transparent 72%)",
            ].join(", "),
            maskImage: "linear-gradient(to bottom, black 40%, transparent 100%)",
            WebkitMaskImage: "linear-gradient(to bottom, black 40%, transparent 100%)",
          }}
        />
        <div className="relative mx-auto w-full max-w-[1800px] px-3 pt-16 pb-8 sm:px-6 sm:pt-20 sm:pb-10 xl:px-8">
          <header className="mx-auto mb-6 max-w-3xl pt-5 text-center sm:mb-10 sm:pt-10">
            <p className="mx-auto mb-4 inline-flex items-center gap-2 rounded-full border border-violet-400/35 bg-violet-500/10 px-4 py-1.5 text-xs font-semibold tracking-[0.18em] text-violet-700 uppercase shadow-[0_0_28px_rgba(168,85,247,0.14)] dark:border-violet-300/25 dark:bg-violet-300/10 dark:text-violet-200">
              <span className="size-1.5 rounded-full bg-violet-500 shadow-[0_0_12px_rgba(168,85,247,0.9)] dark:bg-violet-300" />
              {copy.modelEyebrow}
            </p>
            <h1 className="break-words bg-[linear-gradient(90deg,#171321_0%,#7c3aed_46%,#2563eb_100%)] bg-clip-text text-[clamp(2.4rem,6vw,4.75rem)] leading-[0.98] font-black tracking-tight text-transparent dark:bg-[linear-gradient(90deg,#ffffff_0%,#c4b5fd_48%,#93c5fd_100%)] dark:bg-clip-text">
              {model.model_name} API pricing
            </h1>
            <p className="mx-auto mt-5 max-w-2xl text-sm leading-relaxed text-slate-600 dark:text-slate-300 sm:text-base">
              Use {model.model_name}
              {vendor ? ` from ${vendor.displayName}` : ""} through flatkey.ai with transparent prepaid billing,
              production routing, and OpenAI-compatible access.
            </p>
            <div className="mt-6 flex flex-wrap justify-center gap-3">
              <Link
                href={pricingPath}
                className="flatkey-primary-cta inline-flex h-10 items-center justify-center rounded-lg px-4 text-sm font-medium shadow-[0_16px_34px_-18px_rgba(15,23,42,0.55)] transition-opacity hover:opacity-90"
              >
                {copy.modelPricingCta}
                <ArrowRight className="size-4" />
              </Link>
              {vendor ? (
                <Link
                  href={localizePath(`/vendors/${vendor.slug}`, props.locale)}
                  className="inline-flex h-10 items-center rounded-lg border border-violet-300/30 bg-white/65 px-4 text-sm font-semibold text-slate-700 transition-colors hover:border-violet-400/45 hover:bg-white dark:border-violet-300/20 dark:bg-white/[0.06] dark:text-slate-200"
                >
                  {vendor.displayName} models
                </Link>
              ) : null}
            </div>
          </header>

          <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_380px]">
            <section className="min-w-0 space-y-4">
              <div className="rounded-3xl border border-violet-300/35 bg-white/60 p-4 shadow-[0_20px_70px_rgba(91,33,182,0.10)] backdrop-blur-xl dark:border-white/10 dark:bg-white/[0.055] dark:shadow-[0_20px_70px_-56px_rgba(124,58,237,0.95)] sm:p-5">
                <h2 className="text-foreground inline-flex items-center gap-3 text-xl font-bold tracking-tight sm:text-2xl">
                  <span className="bg-muted/70 border-border text-foreground/80 inline-flex size-9 shrink-0 items-center justify-center rounded-full border">
                    <DollarSign className="size-5" aria-hidden="true" />
                  </span>
                  Model details
                </h2>
                <p className="mt-3 text-sm leading-7 text-slate-600 dark:text-slate-300">
                  {model.description || model.vendor_description || `${model.model_name} is available through flatkey.ai for metered API usage.`}
                </p>
                <dl className="mt-5 grid gap-3 sm:grid-cols-2">
                  <DetailItem label="Billing" value={isTokenBasedModel(model) ? "Token-based" : "Per request"} />
                  <DetailItem label="Vendor" value={vendor?.displayName ?? model.vendor_name ?? "AI provider"} />
                  <DetailItem label="Model ID" value={model.model_name} mono />
                  <DetailItem label="Pricing filter" value={`vendor=${model.vendor_slug ?? vendor?.slug ?? "all"}`} mono />
                </dl>
              </div>

              <PricingModelBrowser
                locale={props.locale}
                models={[model]}
                groupRatio={model.group_ratio ?? {}}
                usableGroup={{}}
                endpointMap={{}}
                autoGroups={[]}
              />
            </section>

            <aside className="space-y-4">
              <section className="rounded-3xl border border-violet-300/35 bg-white/60 p-4 shadow-[0_20px_70px_rgba(91,33,182,0.10)] backdrop-blur-xl dark:border-white/10 dark:bg-white/[0.055] sm:p-5">
                <h2 className="text-sm font-black text-slate-950 dark:text-white">Pricing summary</h2>
                <div className="mt-4 grid gap-3">
                  <SummaryLine label="Input" value={priceLabel(model, "input")} />
                  <SummaryLine label="Output" value={priceLabel(model, "output")} />
                  <SummaryLine label="Endpoints" value={(model.supported_endpoint_types ?? []).slice(0, 3).join(", ") || "OpenAI-compatible"} />
                </div>
                <Link href={pricingPath} className="mt-4 inline-flex h-9 items-center gap-2 rounded-full border border-violet-500/16 bg-violet-500/8 px-3 text-sm font-semibold text-violet-700 transition-colors hover:border-violet-500/25 hover:bg-violet-500/12 hover:text-violet-600 dark:border-violet-300/20 dark:bg-violet-300/10 dark:text-violet-200">
                  {copy.modelPricingCta}
                  <ArrowRight className="size-4" aria-hidden="true" />
                </Link>
              </section>

              <section className="rounded-3xl border border-violet-300/35 bg-white/60 p-4 shadow-[0_20px_70px_rgba(91,33,182,0.10)] backdrop-blur-xl dark:border-white/10 dark:bg-white/[0.055] sm:p-5">
                <h2 className="text-sm font-black text-slate-950 dark:text-white">{copy.relatedModelsTitle}</h2>
                <div className="mt-4 grid gap-2">
                  {relatedModels.length > 0 ? (
                    relatedModels.map((related) => (
                      <Link
                        key={related.model_name}
                        href={localizePath(`/models/${related.model_slug ?? ""}`, props.locale)}
                        className="flex min-w-0 items-start justify-between gap-3 rounded-xl border border-violet-300/25 bg-white/55 px-3 py-2 text-sm font-semibold text-slate-700 transition-colors hover:border-violet-400/45 hover:bg-white hover:text-slate-950 dark:border-white/10 dark:bg-white/[0.045] dark:text-slate-300 dark:hover:border-violet-300/30 dark:hover:bg-violet-300/10 dark:hover:text-white"
                      >
                        <span className="min-w-0 break-all">{related.model_name}</span>
                        <span className="shrink-0 text-xs text-slate-500 dark:text-slate-400">
                          {isTokenBasedModel(related) ? `${formatModelPrice(related, "input")}/1M` : `${formatModelPrice(related)}/request`}
                        </span>
                      </Link>
                    ))
                  ) : (
                    <Link href={pricingPath} className="rounded-xl border border-violet-300/25 bg-white/55 px-3 py-2 text-sm font-semibold text-violet-700 dark:border-white/10 dark:bg-white/[0.045] dark:text-violet-200">
                      {copy.modelPricingCta}
                    </Link>
                  )}
                </div>
              </section>
            </aside>
          </div>
        </div>
      </div>
    </SiteShell>
  );
}

function DetailItem(props: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex min-w-0 items-start gap-2 rounded-xl border border-violet-300/25 bg-white/55 px-3 py-2 dark:border-white/10 dark:bg-white/[0.045]">
      <CheckCircle2 className="mt-0.5 size-3.5 shrink-0 text-violet-700 dark:text-violet-200" aria-hidden="true" />
      <div className="min-w-0">
        <dt className="text-[10px] font-medium tracking-wider text-muted-foreground uppercase">{props.label}</dt>
        <dd className={`mt-1 break-words text-sm font-semibold text-slate-900 dark:text-slate-100 ${props.mono ? "font-mono" : ""}`}>{props.value}</dd>
      </div>
    </div>
  );
}

function SummaryLine(props: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 rounded-xl border border-violet-300/25 bg-white/55 px-3 py-2 dark:border-white/10 dark:bg-white/[0.045]">
      <span className="text-xs font-semibold tracking-wider text-muted-foreground uppercase">{props.label}</span>
      <span className="text-right font-mono text-sm font-semibold text-slate-950 dark:text-slate-100">{props.value}</span>
    </div>
  );
}

function priceLabel(model: PricingModel, kind: "input" | "output") {
  if (!isTokenBasedModel(model)) return `${formatModelPrice(model)} / request`;
  return `${formatModelPrice(model, kind)} / 1M tokens`;
}

function mergeGroupRatio(models: PricingModel[]) {
  return models.reduce<Record<string, number>>((accumulator, model) => {
    for (const [group, ratio] of Object.entries(model.group_ratio ?? {})) accumulator[group] = ratio;
    return accumulator;
  }, {});
}
