import { type DemandPool, poolProgress } from "@/lib/compute-pools";

export type PoolStripCopy = {
  title: string;
  body: string;
  pooled: string;
  requests: string;
  next: string;
  top: string;
  tierLabel: string;
  pooling: string;
};

function fill(template: string, vars: Record<string, string | number>): string {
  return template.replace(/\{(\w+)\}/g, (_, k: string) => String(vars[k] ?? ""));
}

export function ComputePoolStrip({ pools, copy, compact = false }: { pools: DemandPool[]; copy: PoolStripCopy; compact?: boolean }) {
  const shown = compact ? pools.slice(0, 4) : pools.slice(0, 6);
  return (
    <section className={compact ? "cq-pools cq-pools-compact" : "cq-pools"} aria-label={copy.title}>
      {!compact ? (
        <header>
          <h2 className="cm-h2">{copy.title}</h2>
          <p className="cm-note">{copy.body}</p>
        </header>
      ) : null}
      <div className="cq-pools-grid">
        {shown.map((p) => {
          const pct = Math.round(poolProgress(p.gpus, p.next_tier_gpus) * 100);
          return (
            <article className="cq-pool-card" key={p.gpu_model}>
              <div className="cq-pool-head">
                <b>{p.gpu_model}</b>
                {p.discount_pct > 0 ? (
                  <span className="cq-tier">{fill(copy.tierLabel, { pct: p.discount_pct })}</span>
                ) : (
                  <span className="cq-tier cq-tier-open">{copy.pooling}</span>
                )}
              </div>
              <div className="cq-pool-num">
                {p.gpus} <small>{copy.pooled}</small>
              </div>
              <div className="cq-bar">
                <i style={{ width: `${pct}%` }} />
              </div>
              <div className="cq-pool-foot">
                <span>{fill(copy.requests, { n: p.requests + p.leads })}</span>
                <span>
                  {p.next_tier_gpus > 0
                    ? fill(copy.next, { need: Math.max(0, p.next_tier_gpus - p.gpus) })
                    : copy.top}
                </span>
              </div>
            </article>
          );
        })}
      </div>
    </section>
  );
}
