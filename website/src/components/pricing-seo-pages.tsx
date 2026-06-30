import Link from "next/link";
import { ArrowRight, Boxes, DollarSign, Gauge, Layers, Route } from "lucide-react";
import { SiteShell } from "@/components/site-shell";
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

  return (
    <SiteShell locale={props.locale} pathname={`/vendors/${props.entry.slug}`}>
      <div className="relative overflow-x-hidden bg-[linear-gradient(180deg,#f7f4ff_0%,#ffffff_44%,#f3f8ff_100%)]">
        <section className="mx-auto grid max-w-6xl gap-8 px-5 pt-24 pb-12 md:grid-cols-[minmax(0,1fr)_320px] md:px-8 md:pt-28">
          <div>
            <p className="mb-4 inline-flex items-center gap-2 rounded-full border border-violet-500/20 bg-white/70 px-3 py-1 text-xs font-semibold tracking-widest text-violet-700 uppercase">
              <Boxes className="size-3.5" />
              {copy.vendorEyebrow}
            </p>
            <h1 className="max-w-3xl text-[clamp(2.2rem,5vw,4.5rem)] leading-none font-black tracking-tight text-slate-950">
              {props.entry.displayName} API models
            </h1>
            <p className="mt-5 max-w-2xl text-base leading-8 text-slate-600">
              Compare {props.entry.displayName} models on flatkey.ai, route them through one API key, and keep usage,
              billing, and provider access in one console.
            </p>
            <div className="mt-7 flex flex-wrap gap-3">
              <Link
                href={pricingPath}
                className="inline-flex h-11 items-center gap-2 rounded-lg bg-violet-600 px-4 text-sm font-semibold text-white shadow-[0_18px_36px_-22px_rgba(91,33,182,0.8)]"
              >
                {copy.vendorPricingCta}
                <ArrowRight className="size-4" />
              </Link>
              <a
                href={consoleUrl("/sign-up")}
                className="inline-flex h-11 items-center rounded-lg border border-slate-200 bg-white px-4 text-sm font-semibold text-slate-800"
              >
                Get an API key
              </a>
            </div>
          </div>

          <aside className="grid content-start gap-3 rounded-2xl border border-violet-500/14 bg-white/72 p-5 shadow-[0_24px_70px_-54px_rgba(91,33,182,0.7)]">
            <Stat icon={Layers} label="Models" value={String(props.entry.models.length)} />
            <Stat icon={Route} label="Pricing URL" value={`/pricing?vendor=${props.entry.slug}`} />
            <Stat icon={Gauge} label="Access" value="One OpenAI-compatible key" />
          </aside>
        </section>

        <section className="mx-auto max-w-6xl px-5 pb-20 md:px-8">
          <div className="mb-4 flex items-center justify-between gap-4">
            <h2 className="text-xl font-bold tracking-tight text-slate-950">{copy.vendorModelsTitle}</h2>
            <Link href={pricingPath} className="text-sm font-semibold text-violet-700">
              {copy.vendorPricingCta}
            </Link>
          </div>
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {props.entry.models.slice(0, 60).map((model) => (
              <ModelSeoCard key={model.model_name} model={model} locale={props.locale} />
            ))}
          </div>
        </section>
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
      <div className="relative overflow-x-hidden bg-[linear-gradient(180deg,#f7f4ff_0%,#ffffff_44%,#f3f8ff_100%)]">
        <section className="mx-auto grid max-w-6xl gap-8 px-5 pt-24 pb-12 md:grid-cols-[minmax(0,1fr)_340px] md:px-8 md:pt-28">
          <div>
            <p className="mb-4 inline-flex items-center gap-2 rounded-full border border-violet-500/20 bg-white/70 px-3 py-1 text-xs font-semibold tracking-widest text-violet-700 uppercase">
              <DollarSign className="size-3.5" />
              {copy.modelEyebrow}
            </p>
            <h1 className="max-w-3xl break-words text-[clamp(2rem,4.5vw,4rem)] leading-none font-black tracking-tight text-slate-950">
              {model.model_name} API pricing
            </h1>
            <p className="mt-5 max-w-2xl text-base leading-8 text-slate-600">
              Use {model.model_name}
              {vendor ? ` from ${vendor.displayName}` : ""} through flatkey.ai with transparent prepaid billing,
              production routing, and OpenAI-compatible access.
            </p>
            <div className="mt-7 flex flex-wrap gap-3">
              <Link
                href={pricingPath}
                className="inline-flex h-11 items-center gap-2 rounded-lg bg-violet-600 px-4 text-sm font-semibold text-white shadow-[0_18px_36px_-22px_rgba(91,33,182,0.8)]"
              >
                {copy.modelPricingCta}
                <ArrowRight className="size-4" />
              </Link>
              {vendor ? (
                <Link
                  href={localizePath(`/vendors/${vendor.slug}`, props.locale)}
                  className="inline-flex h-11 items-center rounded-lg border border-slate-200 bg-white px-4 text-sm font-semibold text-slate-800"
                >
                  {vendor.displayName} models
                </Link>
              ) : null}
            </div>
          </div>

          <aside className="grid content-start gap-3 rounded-2xl border border-violet-500/14 bg-white/72 p-5 shadow-[0_24px_70px_-54px_rgba(91,33,182,0.7)]">
            <Stat icon={DollarSign} label="Input price" value={priceLabel(model, "input")} />
            <Stat icon={DollarSign} label="Output price" value={priceLabel(model, "output")} />
            <Stat icon={Route} label="Endpoints" value={(model.supported_endpoint_types ?? []).slice(0, 3).join(", ") || "OpenAI-compatible"} />
          </aside>
        </section>

        <section className="mx-auto max-w-6xl px-5 pb-20 md:px-8">
          <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(280px,360px)]">
            <article className="rounded-2xl border border-violet-500/14 bg-white/74 p-5 shadow-[0_24px_70px_-56px_rgba(91,33,182,0.54)]">
              <h2 className="text-xl font-bold tracking-tight text-slate-950">Model details</h2>
              <p className="mt-3 text-sm leading-7 text-slate-600">
                {model.description || model.vendor_description || `${model.model_name} is available through flatkey.ai for metered API usage.`}
              </p>
              <dl className="mt-5 grid gap-3 sm:grid-cols-2">
                <Info label="Billing" value={isTokenBasedModel(model) ? "Token-based" : "Per request"} />
                <Info label="Vendor" value={vendor?.displayName ?? model.vendor_name ?? "AI provider"} />
                <Info label="Model ID" value={model.model_name} mono />
                <Info label="Pricing filter" value={`vendor=${model.vendor_slug ?? vendor?.slug ?? "all"}`} mono />
              </dl>
            </article>

            <aside className="rounded-2xl border border-violet-500/14 bg-white/74 p-5 shadow-[0_24px_70px_-56px_rgba(91,33,182,0.54)]">
              <h2 className="text-base font-bold tracking-tight text-slate-950">{copy.relatedModelsTitle}</h2>
              <div className="mt-4 grid gap-2">
                {relatedModels.length > 0 ? (
                  relatedModels.map((related) => <ModelSeoCard key={related.model_name} model={related} locale={props.locale} compact />)
                ) : (
                  <Link href={pricingPath} className="rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-violet-700">
                    {copy.modelPricingCta}
                  </Link>
                )}
              </div>
            </aside>
          </div>
        </section>
      </div>
    </SiteShell>
  );
}

function ModelSeoCard(props: { model: PricingModel; locale: Locale; compact?: boolean }) {
  const href = localizePath(`/models/${props.model.model_slug ?? ""}`, props.locale);
  return (
    <Link
      href={href}
      className="block rounded-2xl border border-violet-500/12 bg-white/76 p-4 shadow-[0_18px_54px_-48px_rgba(91,33,182,0.68)] transition-colors hover:border-violet-500/28 hover:bg-white"
    >
      <h3 className="break-all text-sm font-black text-slate-950">{props.model.model_name}</h3>
      {props.compact ? null : (
        <p className="mt-2 line-clamp-2 min-h-10 text-xs leading-5 text-slate-500">
          {props.model.description || props.model.vendor_description || "Transparent usage pricing through flatkey.ai."}
        </p>
      )}
      <div className="mt-3 flex flex-wrap items-center gap-2 text-xs text-slate-500">
        <span>{isTokenBasedModel(props.model) ? `${formatModelPrice(props.model, "input")}/1M input` : `${formatModelPrice(props.model)}/request`}</span>
        {props.model.vendor_name ? <span>{props.model.vendor_name}</span> : null}
      </div>
    </Link>
  );
}

function Stat(props: { icon: React.ComponentType<{ className?: string }>; label: string; value: string }) {
  const Icon = props.icon;
  return (
    <div className="flex min-w-0 gap-3 rounded-xl border border-violet-500/10 bg-white/68 p-3">
      <span className="inline-flex size-8 shrink-0 items-center justify-center rounded-lg bg-violet-500/10 text-violet-700">
        <Icon className="size-4" />
      </span>
      <div className="min-w-0">
        <p className="text-xs font-semibold tracking-widest text-slate-400 uppercase">{props.label}</p>
        <p className="mt-1 truncate text-sm font-bold text-slate-950">{props.value}</p>
      </div>
    </div>
  );
}

function Info(props: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-white/70 p-3">
      <dt className="text-xs font-semibold tracking-widest text-slate-400 uppercase">{props.label}</dt>
      <dd className={`mt-1 break-words text-sm font-semibold text-slate-900 ${props.mono ? "font-mono" : ""}`}>{props.value}</dd>
    </div>
  );
}

function priceLabel(model: PricingModel, kind: "input" | "output") {
  if (!isTokenBasedModel(model)) return `${formatModelPrice(model)} / request`;
  return `${formatModelPrice(model, kind)} / 1M tokens`;
}
