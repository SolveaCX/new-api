import { SiteShell } from "@/components/site-shell";
import { ComputeDemandBoard } from "@/components/compute-demand-board";
import { type Locale } from "@/lib/locales";
import { consoleUrl } from "@/lib/origins";
import { DEMAND_INDEX, generateDemandSnapshot } from "@/lib/compute-demand";
import { getComputeMarketCopy } from "@/lib/compute-market-copy";
import { staticFeaturePages } from "@/lib/static-feature-pages";

export const COMPUTE_MARKET_POST_URL = consoleUrl("/compute/market", "?tab=post");
export const COMPUTE_MARKET_SUPPLY_URL = consoleUrl("/compute/market", "?tab=supply");
export const COMPUTE_MARKET_SUPPLIER_URL = consoleUrl("/compute/market", "?tab=supplier");

export function ComputeMarketPage({ locale }: { locale: Locale }) {
  const copy = getComputeMarketCopy(locale);
  const rows = generateDemandSnapshot(40, 7);
  return (
    <SiteShell locale={locale} pathname={staticFeaturePages.compute.pathname}>
      <main className="cm-page">
        <section className="cm-hero">
          <p className="cm-kick">{copy.kicker}</p>
          <h1>{copy.title}</h1>
          <p className="cm-sub">{copy.subtitle}</p>
          <div className="cm-ctas">
            <a className="cm-btn cm-btn-dark" href={COMPUTE_MARKET_POST_URL}>
              {copy.ctaPost}
            </a>
            <a className="cm-btn cm-btn-light" href={COMPUTE_MARKET_SUPPLIER_URL}>
              {copy.ctaSupply}
            </a>
          </div>
          <div className="cm-index">
            {DEMAND_INDEX.map(([gpu, price]) => (
              <span key={gpu}>
                {gpu} <b>${price.toFixed(2)}</b>
              </span>
            ))}
            <span className="cm-pill">
              <i />
              {copy.indexNote}
            </span>
          </div>
        </section>
        <section className="cm-main">
          <div>
            <h2 className="cm-h2">{copy.boardTitle}</h2>
            <ComputeDemandBoard
              rows={rows}
              variant="table"
              copy={copy.board}
              bidHref={COMPUTE_MARKET_SUPPLY_URL}
              joinHref={COMPUTE_MARKET_SUPPLIER_URL}
              liveIntervalMs={6000}
              seed={7}
              maxRows={18}
              allHref={COMPUTE_MARKET_SUPPLY_URL}
              allLabel={copy.board.allLabel}
            />
            <p className="cm-note" style={{ marginTop: 12 }}>
              {copy.footNote}
            </p>
          </div>
          <aside className="cm-side">
            <div className="cm-card cm-card-dark">
              <h3>{copy.supplyTitle}</h3>
              <p>{copy.supplyBody}</p>
              <a className="cm-btn cm-btn-lime" href={COMPUTE_MARKET_SUPPLIER_URL}>
                {copy.supplyCta}
              </a>
            </div>
            <div className="cm-card">
              <h3>{copy.demandTitle}</h3>
              <p>{copy.demandBody}</p>
              <a className="cm-btn cm-btn-dark" href={COMPUTE_MARKET_POST_URL}>
                {copy.demandCta}
              </a>
            </div>
            <div className="cm-card">
              <h3>{copy.whyTitle}</h3>
              <ul>
                {copy.why.map((line) => (
                  <li key={line}>{line}</li>
                ))}
              </ul>
            </div>
          </aside>
        </section>
      </main>
    </SiteShell>
  );
}
