"use client";

import { useEffect, useMemo, useState } from "react";
import { type DemandPool, POOL_GPU_MODELS, poolProgress } from "@/lib/compute-pools";

export type QuickRequestCopy = {
  title: string;
  hint: string;
  pasteLabel: string;
  pastePlaceholder: string;
  gpuLabel: string;
  gpusLabel: string;
  termLabel: string;
  termUnit: string;
  contactLabel: string;
  contactPlaceholder: string;
  contactNote: string;
  submit: string;
  submitting: string;
  refBanner: string;
  doneTitle: string;
  doneBody: string;
  poolLine: string;
  poolNext: string;
  poolTop: string;
  shareTitle: string;
  shareBody: string;
  shareCopy: string;
  shareCopied: string;
  shareX: string;
  shareText: string;
  referrals: string;
  another: string;
  error: string;
};

type Result = {
  code: string;
  share_url: string;
  referrals: number;
  pool: DemandPool;
};

const TERMS = [1, 3, 6, 12, 24, 36];
const STORAGE_KEY = "fk-compute-lead";

function fill(template: string, vars: Record<string, string | number>): string {
  return template.replace(/\{(\w+)\}/g, (_, k: string) => String(vars[k] ?? ""));
}

function guessFromText(text: string): { gpu?: string; gpus?: number; term?: number } {
  const out: { gpu?: string; gpus?: number; term?: number } = {};
  const t = text.toUpperCase();
  for (const g of POOL_GPU_MODELS) {
    if (t.includes(g.toUpperCase())) {
      out.gpu = g;
      break;
    }
  }
  const before = out.gpu ? t.match(new RegExp(`(\\d{1,5})\\s*(?:X|×|张|卡|个)?\\s*${out.gpu.toUpperCase().replace(" ", "\\s*")}`)) : null;
  const cards = t.match(/(\d{1,5})\s*(?:X|×|张|卡|GPUS?|CARDS?)/);
  const nodes = t.match(/(\d{1,4})\s*(?:台|NODES?|SERVERS?|机)/);
  if (cards) out.gpus = Number(cards[1]);
  else if (before) out.gpus = Number(before[1]);
  else if (nodes) out.gpus = Number(nodes[1]) * 8;
  const months = t.match(/(\d{1,3})\s*(?:MONTHS?|MO|个月|月)/);
  const years = t.match(/(\d{1,2})\s*(?:YEARS?|YR|年)/);
  if (months) out.term = Number(months[1]);
  else if (years) out.term = Number(years[1]) * 12;
  if (out.term && !TERMS.includes(out.term)) {
    out.term = TERMS.reduce((a, b) => (Math.abs(b - (out.term as number)) < Math.abs(a - (out.term as number)) ? b : a));
  }
  return out;
}

export function ComputeQuickRequest({ copy, source = "website" }: { copy: QuickRequestCopy; source?: string }) {
  const [text, setText] = useState("");
  const [gpu, setGpu] = useState<string>("H200");
  const [gpus, setGpus] = useState<string>("16");
  const [term, setTerm] = useState<number>(6);
  const [contact, setContact] = useState("");
  const [website, setWebsite] = useState(""); // honeypot
  const [ref, setRef] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<Result | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    // Restore the share screen / referral code once the page is hydrated.
    const restore = window.setTimeout(() => {
      try {
        const params = new URLSearchParams(window.location.search);
        const r = (params.get("ref") ?? "").trim().toUpperCase();
        if (/^[A-Z0-9]{4,16}$/.test(r)) setRef(r);
        const saved = window.localStorage.getItem(STORAGE_KEY);
        if (saved) {
          const parsed = JSON.parse(saved) as Result;
          if (parsed?.code) setResult(parsed);
        }
      } catch {
        /* ignore */
      }
    }, 0);
    return () => window.clearTimeout(restore);
  }, []);

  function onTextChange(value: string) {
    setText(value);
    if (!value.trim()) return;
    const g = guessFromText(value);
    if (g.gpu) setGpu(g.gpu);
    if (g.gpus) setGpus(String(g.gpus));
    if (g.term) setTerm(g.term);
  }

  const gpuCount = Number(gpus) || 0;
  const canSubmit = !busy && gpuCount > 0 && contact.trim().length >= 5;

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;
    setBusy(true);
    setError("");
    try {
      const res = await fetch("/api/compute/leads", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          text: text.trim(),
          gpu_model: gpu,
          gpus: gpuCount,
          term_months: term,
          contact: contact.trim(),
          ref,
          source,
          website,
        }),
      });
      const json = (await res.json()) as { success?: boolean; message?: string; data?: Result };
      if (!res.ok || !json.success || !json.data?.code) {
        setError(json.message || copy.error);
        return;
      }
      setResult(json.data);
      try {
        window.localStorage.setItem(STORAGE_KEY, JSON.stringify(json.data));
      } catch {
        /* ignore */
      }
    } catch {
      setError(copy.error);
    } finally {
      setBusy(false);
    }
  }

  const shareText = useMemo(
    () => (result ? fill(copy.shareText, { gpu: result.pool.gpu_model, gpus: result.pool.gpus, url: result.share_url }) : ""),
    [result, copy.shareText],
  );

  async function copyLink() {
    if (!result) return;
    try {
      await navigator.clipboard.writeText(result.share_url);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      window.prompt(copy.shareCopy, result.share_url);
    }
  }

  function reset() {
    setResult(null);
    setText("");
    setContact("");
    try {
      window.localStorage.removeItem(STORAGE_KEY);
    } catch {
      /* ignore */
    }
  }

  if (result) {
    const p = result.pool;
    const pct = Math.round(poolProgress(p.gpus, p.next_tier_gpus) * 100);
    return (
      <div className="cq cq-done" data-testid="quick-request-done">
        <div className="cq-badge">#{result.code}</div>
        <h3>{copy.doneTitle}</h3>
        <p className="cq-sub">{copy.doneBody}</p>
        <div className="cq-pool">
          <div className="cq-pool-line">{fill(copy.poolLine, { gpu: p.gpu_model, gpus: p.gpus, n: p.requests + p.leads })}</div>
          <div className="cq-bar">
            <i style={{ width: `${pct}%` }} />
          </div>
          <div className="cq-pool-next">
            {p.next_tier_gpus > 0
              ? fill(copy.poolNext, { need: Math.max(0, p.next_tier_gpus - p.gpus), pct: p.discount_pct + 5 })
              : fill(copy.poolTop, { pct: p.discount_pct })}
          </div>
        </div>
        <div className="cq-share">
          <h4>{copy.shareTitle}</h4>
          <p>{copy.shareBody}</p>
          <div className="cq-link">
            <code>{result.share_url}</code>
            <button type="button" className="cm-btn cm-btn-dark" onClick={copyLink}>
              {copied ? copy.shareCopied : copy.shareCopy}
            </button>
          </div>
          <div className="cq-share-row">
            <a
              className="cm-btn cm-btn-light"
              href={`https://x.com/intent/post?text=${encodeURIComponent(shareText)}`}
              target="_blank"
              rel="noreferrer"
            >
              {copy.shareX}
            </a>
            <span className="cq-refs">{fill(copy.referrals, { n: result.referrals })}</span>
          </div>
        </div>
        <button type="button" className="cq-again" onClick={reset}>
          {copy.another}
        </button>
      </div>
    );
  }

  return (
    <form className="cq" onSubmit={submit} data-testid="quick-request">
      <h3>{copy.title}</h3>
      <p className="cq-sub">{copy.hint}</p>
      {ref ? <div className="cq-ref">{fill(copy.refBanner, { code: ref })}</div> : null}
      <label className="cq-field">
        <span>{copy.pasteLabel}</span>
        <textarea
          rows={2}
          value={text}
          placeholder={copy.pastePlaceholder}
          onChange={(e) => onTextChange(e.target.value)}
        />
      </label>
      <div className="cq-grid">
        <label className="cq-field">
          <span>{copy.gpuLabel}</span>
          <select value={gpu} onChange={(e) => setGpu(e.target.value)}>
            {POOL_GPU_MODELS.map((g) => (
              <option key={g} value={g}>
                {g}
              </option>
            ))}
          </select>
        </label>
        <label className="cq-field">
          <span>{copy.gpusLabel}</span>
          <input
            type="number"
            min={1}
            max={100000}
            inputMode="numeric"
            value={gpus}
            onChange={(e) => setGpus(e.target.value)}
          />
        </label>
        <label className="cq-field">
          <span>{copy.termLabel}</span>
          <select value={term} onChange={(e) => setTerm(Number(e.target.value))}>
            {TERMS.map((m) => (
              <option key={m} value={m}>
                {m} {copy.termUnit}
              </option>
            ))}
          </select>
        </label>
      </div>
      <label className="cq-field">
        <span>{copy.contactLabel}</span>
        <input
          type="text"
          autoComplete="email"
          value={contact}
          placeholder={copy.contactPlaceholder}
          onChange={(e) => setContact(e.target.value)}
        />
        <small>{copy.contactNote}</small>
      </label>
      <input
        className="cq-hp"
        tabIndex={-1}
        autoComplete="off"
        name="website"
        value={website}
        onChange={(e) => setWebsite(e.target.value)}
        aria-hidden="true"
      />
      {error ? <div className="cq-error">{error}</div> : null}
      <button type="submit" className="cm-btn cm-btn-dark cq-submit" disabled={!canSubmit}>
        {busy ? copy.submitting : copy.submit}
      </button>
    </form>
  );
}
